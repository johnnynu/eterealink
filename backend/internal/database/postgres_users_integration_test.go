package database

import (
	"context"
	"errors"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/eterealink/eterealink/backend/internal/domain"
)

func TestPostgresCustomDisplayNameUniquenessAndIdentityRefresh(t *testing.T) {
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		t.Skip("DATABASE_URL is not set")
	}
	database, err := Open(context.Background(), databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()

	ctx := context.Background()
	now := time.Date(2026, time.September, 5, 12, 0, 0, 0, time.UTC)
	first := domain.User{ID: "98000000-0000-4000-8000-000000000001", FirebaseUID: "profile-first", Email: "profile-first@example.com", DisplayName: "First Google", CreatedAt: now}
	second := domain.User{ID: "98000000-0000-4000-8000-000000000002", FirebaseUID: "profile-second", Email: "profile-second@example.com", DisplayName: "Second Google", CreatedAt: now}
	cleanup := func() {
		_, _ = database.pool.Exec(context.Background(), `DELETE FROM users WHERE id = ANY($1::uuid[])`, []string{first.ID, second.ID})
	}
	cleanup()
	defer cleanup()
	if _, err := database.UpsertUser(ctx, first); err != nil {
		t.Fatal(err)
	}
	if _, err := database.UpsertUser(ctx, second); err != nil {
		t.Fatal(err)
	}

	name := "JohnnyCloud"
	updated, err := database.UpdateCustomDisplayName(ctx, first.ID, &name)
	if err != nil || updated.DisplayName != name || updated.IdentityDisplayName != first.DisplayName {
		t.Fatalf("updated = %#v, error = %v", updated, err)
	}
	casing := "JOHNNYCLOUD"
	if _, err := database.UpdateCustomDisplayName(ctx, first.ID, &casing); err != nil {
		t.Fatalf("capitalization-only update failed: %v", err)
	}
	conflict := "johnnycloud"
	if _, err := database.UpdateCustomDisplayName(ctx, second.ID, &conflict); !errors.Is(err, domain.ErrDisplayNameTaken) {
		t.Fatalf("case-insensitive conflict error = %v", err)
	}

	first.DisplayName = "Refreshed Google"
	refreshed, err := database.UpsertUser(ctx, first)
	if err != nil || refreshed.DisplayName != casing || refreshed.IdentityDisplayName != "Refreshed Google" {
		t.Fatalf("identity refresh = %#v, error = %v", refreshed, err)
	}
	cleared, err := database.UpdateCustomDisplayName(ctx, first.ID, nil)
	if err != nil || cleared.DisplayName != "Refreshed Google" || cleared.CustomDisplayName != nil {
		t.Fatalf("cleared = %#v, error = %v", cleared, err)
	}
}

func TestPostgresPerUserQuotaAndConcurrentReservations(t *testing.T) {
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		t.Skip("DATABASE_URL is not set")
	}
	database, err := Open(context.Background(), databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	ctx := context.Background()
	user := domain.User{
		ID: "99000000-0000-4000-8000-000000000001", FirebaseUID: "quota-user",
		Email: "quota-user@example.com", DisplayName: "Quota User", CreatedAt: time.Now().UTC(),
	}
	_, _ = database.pool.Exec(ctx, `DELETE FROM users WHERE id = $1`, user.ID)
	defer func() { _, _ = database.pool.Exec(context.Background(), `DELETE FROM users WHERE id = $1`, user.ID) }()
	if _, err := database.UpsertUser(ctx, user); err != nil {
		t.Fatal(err)
	}
	override := int64(10)
	quota, err := database.UpdateStorageQuota(ctx, user.ID, &override, 25)
	if err != nil || quota.StorageQuotaBytes == nil || quota.EffectiveQuota != 10 {
		t.Fatalf("override = %#v, error = %v", quota, err)
	}

	files := []domain.File{
		{ID: "99000000-0000-4000-8000-000000000011", OwnerID: &user.ID, StorageKey: "quota/file-1", OriginalName: "one.bin", MIMEType: "application/octet-stream", SizeBytes: 6, Status: domain.FileStatusPending, CreatedAt: time.Now().UTC()},
		{ID: "99000000-0000-4000-8000-000000000012", OwnerID: &user.ID, StorageKey: "quota/file-2", OriginalName: "two.bin", MIMEType: "application/octet-stream", SizeBytes: 6, Status: domain.FileStatusPending, CreatedAt: time.Now().UTC()},
	}
	start := make(chan struct{})
	results := make(chan error, len(files))
	var workers sync.WaitGroup
	for _, file := range files {
		file := file
		workers.Add(1)
		go func() {
			defer workers.Done()
			<-start
			results <- database.CreateOwnedFileWithinQuota(ctx, file, 25)
		}()
	}
	close(start)
	workers.Wait()
	close(results)
	succeeded, rejected := 0, 0
	for result := range results {
		switch {
		case result == nil:
			succeeded++
		case errors.Is(result, domain.ErrQuotaExceeded):
			rejected++
		default:
			t.Fatalf("reservation error = %v", result)
		}
	}
	if succeeded != 1 || rejected != 1 {
		t.Fatalf("concurrent results: succeeded=%d rejected=%d", succeeded, rejected)
	}
	usage, err := database.GetOwnedFileUsage(ctx, user.ID)
	if err != nil || usage.FileCount != 1 || usage.TotalBytes != 6 {
		t.Fatalf("pending usage = %#v, error = %v", usage, err)
	}

	larger := int64(12)
	if _, err := database.UpdateStorageQuota(ctx, user.ID, &larger, 25); err != nil {
		t.Fatal(err)
	}
	var missing domain.File
	for _, file := range files {
		if _, err := database.GetOwnedFile(ctx, user.ID, file.ID); errors.Is(err, domain.ErrNotFound) {
			missing = file
		}
	}
	if err := database.CreateOwnedFileWithinQuota(ctx, missing, 25); err != nil {
		t.Fatalf("larger override reservation: %v", err)
	}
	reset, err := database.UpdateStorageQuota(ctx, user.ID, nil, 25)
	if err != nil || reset.StorageQuotaBytes != nil || reset.EffectiveQuota != 25 {
		t.Fatalf("reset = %#v, error = %v", reset, err)
	}
	if err := database.DeleteOwnedFile(ctx, user.ID, missing.ID); err != nil {
		t.Fatal(err)
	}
	usage, err = database.GetOwnedFileUsage(ctx, user.ID)
	if err != nil || usage.TotalBytes != 6 {
		t.Fatalf("usage after delete = %#v, error = %v", usage, err)
	}
}
