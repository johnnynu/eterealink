package database

import (
	"context"
	"errors"
	"os"
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
