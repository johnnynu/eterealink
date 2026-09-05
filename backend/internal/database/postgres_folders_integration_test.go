package database

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/eterealink/eterealink/backend/internal/domain"
)

func TestPostgresFolderOwnershipAndInheritedViewerAccess(t *testing.T) {
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		t.Skip("DATABASE_URL is not set")
	}
	ctx := context.Background()
	database, err := Open(ctx, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()

	now := time.Date(2026, time.September, 4, 19, 0, 0, 0, time.UTC)
	owner := domain.User{ID: "91000000-0000-4000-8000-000000000001", FirebaseUID: "phase5-owner", Email: "phase5-owner@example.com", DisplayName: "Folder Owner", CreatedAt: now}
	viewer := domain.User{ID: "91000000-0000-4000-8000-000000000002", FirebaseUID: "phase5-viewer", Email: "phase5-viewer@example.com", DisplayName: "Folder Viewer", CreatedAt: now}
	cleanup := func() {
		_, _ = database.pool.Exec(context.Background(), `DELETE FROM users WHERE id = ANY($1::uuid[])`, []string{owner.ID, viewer.ID})
	}
	cleanup()
	defer cleanup()
	for _, user := range []domain.User{owner, viewer} {
		if _, err := database.UpsertUser(ctx, user); err != nil {
			t.Fatal(err)
		}
	}
	root := domain.Folder{ID: "92000000-0000-4000-8000-000000000001", OwnerID: owner.ID, Name: "Project", CreatedAt: now}
	if err := database.CreateFolder(ctx, root); err != nil {
		t.Fatal(err)
	}
	childID := "92000000-0000-4000-8000-000000000002"
	child := domain.Folder{ID: childID, OwnerID: owner.ID, ParentFolderID: &root.ID, Name: "Plans", CreatedAt: now}
	if err := database.CreateFolder(ctx, child); err != nil {
		t.Fatal(err)
	}
	file := domain.File{
		ID: "93000000-0000-4000-8000-000000000001", OwnerID: &owner.ID, FolderID: &childID,
		StorageKey: "phase5-test/file-1", OriginalName: "roadmap.txt", MIMEType: "text/plain", SizeBytes: 7,
		Status: domain.FileStatusPending, CreatedAt: now,
	}
	if err := database.CreateOwnedFile(ctx, file); err != nil {
		t.Fatal(err)
	}
	if _, err := database.CompleteOwnedFile(ctx, owner.ID, file.ID, now); err != nil {
		t.Fatal(err)
	}
	olderTime := now.Add(-time.Hour)
	olderFile := domain.File{
		ID: "93000000-0000-4000-8000-000000000003", OwnerID: &owner.ID, FolderID: &childID,
		StorageKey: "phase5-test/file-older", OriginalName: "archive.txt", MIMEType: "text/plain", SizeBytes: 2,
		Status: domain.FileStatusPending, CreatedAt: olderTime,
	}
	if err := database.CreateOwnedFile(ctx, olderFile); err != nil {
		t.Fatal(err)
	}
	if _, err := database.CompleteOwnedFile(ctx, owner.ID, olderFile.ID, olderTime); err != nil {
		t.Fatal(err)
	}

	member, err := database.AddFolderMember(ctx, owner.ID, root.ID, viewer.Email, now)
	if err != nil || member.User.ID != viewer.ID {
		t.Fatalf("member = %#v, error = %v", member, err)
	}
	shared, _, err := database.GetRootContents(ctx, viewer.ID, "shared", now, domain.FileLibraryQuery{Sort: "newest", Limit: 10})
	if err != nil || len(shared.Folders) != 1 || shared.Folders[0].Role != domain.FolderRoleViewer {
		t.Fatalf("shared root = %#v, error = %v", shared, err)
	}
	contents, _, err := database.GetFolderContents(ctx, viewer.ID, child.ID, now, domain.FileLibraryQuery{Sort: "newest", Limit: 10})
	if err != nil || contents.Current == nil || contents.Current.Role != domain.FolderRoleViewer || len(contents.Files) != 2 {
		t.Fatalf("viewer contents = %#v, error = %v", contents, err)
	}
	firstPage, hasMore, err := database.GetFolderContents(ctx, owner.ID, child.ID, now, domain.FileLibraryQuery{Sort: "newest", Limit: 1})
	if err != nil || !hasMore || len(firstPage.Files) != 1 || firstPage.Files[0].File.ID != file.ID {
		t.Fatalf("first cursor page = %#v, more = %t, error = %v", firstPage.Files, hasMore, err)
	}
	secondPage, hasMore, err := database.GetFolderContents(ctx, owner.ID, child.ID, now, domain.FileLibraryQuery{
		Sort: "newest", Limit: 1, CursorID: file.ID, CursorTime: file.CreatedAt,
	})
	if err != nil || hasMore || len(secondPage.Files) != 1 || secondPage.Files[0].File.ID != olderFile.ID {
		t.Fatalf("second cursor page = %#v, more = %t, error = %v", secondPage.Files, hasMore, err)
	}
	namePage, hasMore, err := database.GetFolderContents(ctx, owner.ID, child.ID, now, domain.FileLibraryQuery{Sort: "name", Limit: 1})
	if err != nil || !hasMore || len(namePage.Files) != 1 || namePage.Files[0].File.ID != olderFile.ID {
		t.Fatalf("name cursor page = %#v, more = %t, error = %v", namePage.Files, hasMore, err)
	}
	searched, _, err := database.GetFolderContents(ctx, owner.ID, child.ID, now, domain.FileLibraryQuery{Sort: "newest", Search: "ARCHIVE", Limit: 10})
	if err != nil || len(searched.Files) != 1 || searched.Files[0].File.ID != olderFile.ID {
		t.Fatalf("searched files = %#v, error = %v", searched.Files, err)
	}
	if _, err := database.GetAccessibleFile(ctx, viewer.ID, file.ID); err != nil {
		t.Fatalf("viewer file access: %v", err)
	}
	if err := database.MoveOwnedFiles(ctx, owner.ID, []string{file.ID, olderFile.ID}, &root.ID); err != nil {
		t.Fatalf("move file: %v", err)
	}
	overQuota := domain.File{
		ID: "93000000-0000-4000-8000-000000000002", OwnerID: &owner.ID,
		StorageKey: "phase5-test/file-2", OriginalName: "extra.txt", MIMEType: "text/plain", SizeBytes: 1,
		Status: domain.FileStatusPending, CreatedAt: now,
	}
	if err := database.CreateOwnedFileWithinQuota(ctx, overQuota, 9); !errors.Is(err, domain.ErrQuotaExceeded) {
		t.Fatalf("quota error = %v", err)
	}
	if err := database.DeleteFolder(ctx, owner.ID, child.ID); err != nil {
		t.Fatalf("delete empty folder: %v", err)
	}
}
