package database

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/eterealink/eterealink/backend/internal/domain"
)

func TestPostgresFolderOwnershipInvitesAndContributorAccess(t *testing.T) {
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

	member, err := database.AddFolderMember(ctx, owner.ID, root.ID, viewer.Email, domain.FolderRoleViewer, now)
	if err != nil || member.User.ID != viewer.ID {
		t.Fatalf("member = %#v, error = %v", member, err)
	}
	inheritedMembers, err := database.ListFolderMembers(ctx, owner.ID, child.ID)
	if err != nil || len(inheritedMembers) != 1 || inheritedMembers[0].User.ID != viewer.ID ||
		inheritedMembers[0].Role != domain.FolderRoleViewer || !inheritedMembers[0].Inherited ||
		inheritedMembers[0].SourceFolderID != root.ID || inheritedMembers[0].SourceFolderName != root.Name {
		t.Fatalf("inherited child members = %#v, error = %v", inheritedMembers, err)
	}
	shared, _, err := database.GetRootContents(ctx, viewer.ID, "shared", now, domain.FileLibraryQuery{Sort: "newest", Limit: 10})
	if err != nil || len(shared.Folders) != 1 || shared.Folders[0].Role != domain.FolderRoleViewer {
		t.Fatalf("shared root = %#v, error = %v", shared, err)
	}
	contents, _, err := database.GetFolderContents(ctx, viewer.ID, child.ID, now, domain.FileLibraryQuery{Sort: "newest", Limit: 10})
	if err != nil || contents.Current == nil || contents.Current.Role != domain.FolderRoleViewer || len(contents.Files) != 2 || contents.Files[0].UploaderName != owner.DisplayName {
		t.Fatalf("viewer contents = %#v, error = %v", contents, err)
	}
	firstPage, hasMore, err := database.GetFolderContents(ctx, owner.ID, child.ID, now, domain.FileLibraryQuery{Sort: "newest", Limit: 1})
	if err != nil || !hasMore || len(firstPage.Files) != 1 || firstPage.Files[0].File.ID != file.ID {
		t.Fatalf("first cursor page = %#v, more = %t, error = %v", firstPage.Files, hasMore, err)
	}
	if firstPage.Summary.FileCount != 2 || firstPage.Summary.TotalBytes != 9 || firstPage.TotalCount != 2 {
		t.Fatalf("first page totals = summary %#v, total count = %d", firstPage.Summary, firstPage.TotalCount)
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
	if searched.Summary.FileCount != 2 || searched.Summary.TotalBytes != 9 || searched.TotalCount != 1 {
		t.Fatalf("searched totals = summary %#v, total count = %d", searched.Summary, searched.TotalCount)
	}
	if _, err := database.GetAccessibleFile(ctx, viewer.ID, file.ID); err != nil {
		t.Fatalf("viewer file access: %v", err)
	}
	inviteExpiry := now.Add(7 * 24 * time.Hour)
	invite := domain.FolderInvite{
		ID: "94000000-0000-4000-8000-000000000001", FolderID: root.ID, CreatedBy: owner.ID,
		ShortCode: "phase5contributor", Role: domain.FolderRoleContributor, ExpiresAt: &inviteExpiry, CreatedAt: now,
	}
	if err := database.CreateFolderInvite(ctx, owner.ID, invite); err != nil {
		t.Fatalf("create contributor invite: %v", err)
	}
	preview, err := database.GetFolderInvitePreview(ctx, invite.ShortCode, now)
	if err != nil || preview.FolderName != root.Name || preview.OwnerName != owner.DisplayName || preview.Role != domain.FolderRoleContributor {
		t.Fatalf("invite preview = %#v, error = %v", preview, err)
	}
	invites, err := database.ListFolderInvites(ctx, owner.ID, root.ID, now)
	if err != nil || len(invites) != 1 || invites[0].Role != domain.FolderRoleContributor {
		t.Fatalf("active invites = %#v, error = %v", invites, err)
	}
	access, err := database.AcceptFolderInvite(ctx, viewer.ID, invite.ShortCode, now)
	if err != nil || access.Role != domain.FolderRoleContributor {
		t.Fatalf("accepted access = %#v, error = %v", access, err)
	}
	inheritedMembers, err = database.ListFolderMembers(ctx, owner.ID, child.ID)
	if err != nil || len(inheritedMembers) != 1 || inheritedMembers[0].Role != domain.FolderRoleContributor ||
		!inheritedMembers[0].Inherited || inheritedMembers[0].SourceFolderID != root.ID {
		t.Fatalf("upgraded inherited child members = %#v, error = %v", inheritedMembers, err)
	}
	viewerInvite := domain.FolderInvite{
		ID: "94000000-0000-4000-8000-000000000004", FolderID: root.ID, CreatedBy: owner.ID,
		ShortCode: "phase5viewer", Role: domain.FolderRoleViewer, ExpiresAt: &inviteExpiry, CreatedAt: now,
	}
	if err := database.CreateFolderInvite(ctx, owner.ID, viewerInvite); err != nil {
		t.Fatal(err)
	}
	access, err = database.AcceptFolderInvite(ctx, viewer.ID, viewerInvite.ShortCode, now)
	if err != nil || access.Role != domain.FolderRoleContributor {
		t.Fatalf("viewer invite downgraded contributor: access = %#v, error = %v", access, err)
	}
	shared, _, err = database.GetRootContents(ctx, viewer.ID, "shared", now, domain.FileLibraryQuery{Sort: "newest", Limit: 10})
	if err != nil || len(shared.Folders) != 1 || shared.Folders[0].Role != domain.FolderRoleContributor {
		t.Fatalf("contributor shared root = %#v, error = %v", shared, err)
	}

	expiredAt := now.Add(-time.Minute)
	expiredInvite := domain.FolderInvite{
		ID: "94000000-0000-4000-8000-000000000002", FolderID: root.ID, CreatedBy: owner.ID,
		ShortCode: "phase5expired", Role: domain.FolderRoleViewer, ExpiresAt: &expiredAt, CreatedAt: now.Add(-time.Hour),
	}
	if err := database.CreateFolderInvite(ctx, owner.ID, expiredInvite); err != nil {
		t.Fatal(err)
	}
	if _, err := database.AcceptFolderInvite(ctx, viewer.ID, expiredInvite.ShortCode, now); !errors.Is(err, domain.ErrExpired) {
		t.Fatalf("expired invite error = %v", err)
	}
	if _, err := database.GetFolderInvitePreview(ctx, expiredInvite.ShortCode, now); !errors.Is(err, domain.ErrExpired) {
		t.Fatalf("expired preview error = %v", err)
	}
	revokedInvite := domain.FolderInvite{
		ID: "94000000-0000-4000-8000-000000000003", FolderID: root.ID, CreatedBy: owner.ID,
		ShortCode: "phase5revoked", Role: domain.FolderRoleViewer, ExpiresAt: &inviteExpiry, CreatedAt: now,
	}
	if err := database.CreateFolderInvite(ctx, owner.ID, revokedInvite); err != nil {
		t.Fatal(err)
	}
	if err := database.RevokeFolderInvite(ctx, owner.ID, root.ID, revokedInvite.ID, now); err != nil {
		t.Fatal(err)
	}
	if _, err := database.AcceptFolderInvite(ctx, viewer.ID, revokedInvite.ShortCode, now); !errors.Is(err, domain.ErrRevoked) {
		t.Fatalf("revoked invite error = %v", err)
	}
	if _, err := database.GetFolderInvitePreview(ctx, revokedInvite.ShortCode, now); !errors.Is(err, domain.ErrRevoked) {
		t.Fatalf("revoked preview error = %v", err)
	}

	contributedFile := domain.File{
		ID: "93000000-0000-4000-8000-000000000004", OwnerID: &viewer.ID, FolderID: &childID,
		StorageKey: "phase5-test/contributed", OriginalName: "feedback.txt", MIMEType: "text/plain", SizeBytes: 4,
		Status: domain.FileStatusPending, CreatedAt: now.Add(time.Minute),
	}
	if err := database.CreateOwnedFileWithinQuota(ctx, contributedFile, 100); err != nil {
		t.Fatalf("contributor upload: %v", err)
	}
	if _, err := database.CompleteOwnedFile(ctx, viewer.ID, contributedFile.ID, now.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	ownerContents, _, err := database.GetFolderContents(ctx, owner.ID, child.ID, now.Add(time.Minute), domain.FileLibraryQuery{Sort: "newest", Limit: 10})
	if err != nil || len(ownerContents.Files) != 3 || ownerContents.Files[0].File.OwnerID == nil || *ownerContents.Files[0].File.OwnerID != viewer.ID || ownerContents.Files[0].UploaderName != viewer.DisplayName {
		t.Fatalf("mixed-owner contents = %#v, error = %v", ownerContents.Files, err)
	}
	if ownerContents.Summary.FileCount != 3 || ownerContents.Summary.TotalBytes != 13 || ownerContents.TotalCount != 3 {
		t.Fatalf("mixed-owner totals = summary %#v, total count = %d", ownerContents.Summary, ownerContents.TotalCount)
	}
	if _, err := database.GetAccessibleFile(ctx, owner.ID, contributedFile.ID); err != nil {
		t.Fatalf("folder owner accessing contributor file: %v", err)
	}
	if err := database.DeleteOwnedFile(ctx, owner.ID, contributedFile.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("owner deleting contributor file error = %v", err)
	}
	if err := database.RemoveContributedFile(ctx, owner.ID, child.ID, contributedFile.ID); err != nil {
		t.Fatalf("remove contribution: %v", err)
	}
	viewerRoot, _, err := database.GetRootContents(ctx, viewer.ID, "owned", now.Add(time.Minute), domain.FileLibraryQuery{Sort: "newest", Limit: 10})
	if err != nil || len(viewerRoot.Files) != 1 || viewerRoot.Files[0].File.ID != contributedFile.ID {
		t.Fatalf("returned contribution = %#v, error = %v", viewerRoot.Files, err)
	}
	if err := database.MoveOwnedFiles(ctx, viewer.ID, []string{contributedFile.ID}, &childID); err != nil {
		t.Fatalf("contributor move into shared child: %v", err)
	}
	if err := database.RemoveFolderMember(ctx, owner.ID, root.ID, viewer.ID); err != nil {
		t.Fatalf("remove contributor: %v", err)
	}
	viewerRoot, _, err = database.GetRootContents(ctx, viewer.ID, "owned", now.Add(time.Minute), domain.FileLibraryQuery{Sort: "newest", Limit: 10})
	if err != nil || len(viewerRoot.Files) != 1 || viewerRoot.Files[0].File.FolderID != nil {
		t.Fatalf("member removal returned files = %#v, error = %v", viewerRoot.Files, err)
	}
	if _, err := database.GetAccessibleFile(ctx, viewer.ID, file.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("removed member file access error = %v", err)
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
