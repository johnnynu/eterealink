package database

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/eterealink/eterealink/backend/internal/domain"
)

func TestPostgresExpiredAnonymousCleanupCandidatesAndCascades(t *testing.T) {
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
	now := time.Date(2026, time.September, 29, 12, 0, 0, 0, time.UTC)
	expired := now.Add(-time.Hour)
	future := now.Add(time.Hour)
	standalone := domain.File{
		ID: "97000000-0000-4000-8000-000000000001", StorageKey: "anonymous/expired-single",
		OriginalName: "single.txt", MIMEType: "text/plain", SizeBytes: 6,
		Status: domain.FileStatusPending, CreatedAt: expired.Add(-time.Hour), ExpiresAt: &expired,
	}
	standaloneShare := domain.ShareLink{
		ID: "97000000-0000-4000-8000-000000000002", ShortCode: "expired1", FileID: &standalone.ID,
		CreatedAt: standalone.CreatedAt, ExpiresAt: &expired,
	}
	transfer := domain.AnonymousTransfer{
		ID: "97000000-0000-4000-8000-000000000003", Status: domain.TransferStatusReady,
		ArchiveStatus: domain.ArchiveStatusReady, ArchiveStorageKey: "anonymous/expired-transfer/bundle.zip",
		ArchiveSizeBytes: pointerToInt64(10), CreatedAt: expired.Add(-time.Hour), CompletedAt: &expired, ExpiresAt: expired,
	}
	transferFile := domain.File{
		ID: "97000000-0000-4000-8000-000000000004", TransferID: &transfer.ID,
		StorageKey: "anonymous/expired-transfer/files/a", OriginalName: "a.txt", MIMEType: "text/plain",
		SizeBytes: 1, Status: domain.FileStatusReady, CreatedAt: transfer.CreatedAt, CompletedAt: &expired, ExpiresAt: &expired,
	}
	transferShare := domain.ShareLink{
		ID: "97000000-0000-4000-8000-000000000005", ShortCode: "expired2", TransferID: &transfer.ID,
		CreatedAt: transfer.CreatedAt, ExpiresAt: &expired,
	}
	futureFile := domain.File{
		ID: "97000000-0000-4000-8000-000000000006", StorageKey: "anonymous/future-single",
		OriginalName: "future.txt", MIMEType: "text/plain", SizeBytes: 6,
		Status: domain.FileStatusPending, CreatedAt: now, ExpiresAt: &future,
	}
	futureShare := domain.ShareLink{
		ID: "97000000-0000-4000-8000-000000000007", ShortCode: "future01", FileID: &futureFile.ID,
		CreatedAt: now, ExpiresAt: &future,
	}
	ids := []string{standalone.ID, transfer.ID, futureFile.ID}
	cleanup := func() {
		_, _ = database.pool.Exec(context.Background(), `DELETE FROM anonymous_transfers WHERE id = $1`, transfer.ID)
		_, _ = database.pool.Exec(context.Background(), `DELETE FROM files WHERE id = ANY($1::uuid[])`, []string{standalone.ID, futureFile.ID})
	}
	cleanup()
	defer cleanup()

	if err := database.CreateAnonymousUpload(ctx, standalone, standaloneShare); err != nil {
		t.Fatal(err)
	}
	if err := database.CreateAnonymousTransfer(ctx, transfer, []domain.File{transferFile}, transferShare); err != nil {
		t.Fatal(err)
	}
	if err := database.CreateAnonymousUpload(ctx, futureFile, futureShare); err != nil {
		t.Fatal(err)
	}

	candidates, err := database.ListExpiredAnonymousContent(ctx, now, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(candidates) != 2 {
		t.Fatalf("candidates = %#v; seeded IDs = %#v", candidates, ids)
	}
	byID := make(map[string]domain.ExpiredAnonymousContent, len(candidates))
	for _, candidate := range candidates {
		byID[candidate.ID] = candidate
	}
	if got := byID[standalone.ID]; got.Kind != domain.AnonymousCleanupFile || len(got.StorageKeys) != 1 || got.StorageKeys[0] != standalone.StorageKey {
		t.Fatalf("standalone candidate = %#v", got)
	}
	if got := byID[transfer.ID]; got.Kind != domain.AnonymousCleanupTransfer || len(got.StorageKeys) != 2 || got.StorageKeys[0] != transfer.ArchiveStorageKey || got.StorageKeys[1] != transferFile.StorageKey {
		t.Fatalf("transfer candidate = %#v", got)
	}

	for _, candidate := range candidates {
		deleted, err := database.DeleteExpiredAnonymousContent(ctx, candidate, now)
		if err != nil || !deleted {
			t.Fatalf("delete candidate %#v: deleted=%v error=%v", candidate, deleted, err)
		}
	}
	var expiredRows, expiredShares, futureRows int
	if err := database.pool.QueryRow(ctx, `SELECT count(*) FROM files WHERE id = ANY($1::uuid[])`, []string{standalone.ID, transferFile.ID}).Scan(&expiredRows); err != nil {
		t.Fatal(err)
	}
	if err := database.pool.QueryRow(ctx, `SELECT count(*) FROM share_links WHERE id = ANY($1::uuid[])`, []string{standaloneShare.ID, transferShare.ID}).Scan(&expiredShares); err != nil {
		t.Fatal(err)
	}
	if err := database.pool.QueryRow(ctx, `SELECT count(*) FROM files WHERE id = $1`, futureFile.ID).Scan(&futureRows); err != nil {
		t.Fatal(err)
	}
	if expiredRows != 0 || expiredShares != 0 || futureRows != 1 {
		t.Fatalf("remaining rows: expired files=%d expired shares=%d future files=%d", expiredRows, expiredShares, futureRows)
	}
}

func pointerToInt64(value int64) *int64 {
	return &value
}
