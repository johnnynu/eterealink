package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/eterealink/eterealink/backend/internal/domain"
)

func TestFoldersCreateAndMoveFiles(t *testing.T) {
	now := time.Date(2026, time.September, 4, 12, 0, 0, 0, time.UTC)
	store := &folderStoreStub{}
	folders := NewFolders(store, func() time.Time { return now })
	parentID := " parent-1 "

	created, err := folders.Create(context.Background(), "user-1", CreateFolderInput{Name: "  Project plans  ", ParentFolderID: &parentID})
	if err != nil {
		t.Fatal(err)
	}
	if created.Name != "Project plans" || created.ParentFolderID == nil || *created.ParentFolderID != "parent-1" {
		t.Fatalf("created folder = %#v", created)
	}
	if !created.CreatedAt.Equal(now) || store.created.ID != created.ID {
		t.Fatalf("stored folder = %#v", store.created)
	}

	destinationID := "destination-1"
	err = folders.MoveFiles(context.Background(), "user-1", MoveFilesInput{
		FileIDs: []string{"file-1", "file-1", "file-2"}, FolderID: &destinationID,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(store.movedIDs) != 2 || store.movedFolderID == nil || *store.movedFolderID != destinationID {
		t.Fatalf("move = %#v, %v", store.movedIDs, store.movedFolderID)
	}
}

func TestFoldersValidateNamesAndBulkSelection(t *testing.T) {
	folders := NewFolders(&folderStoreStub{}, time.Now)
	if _, err := folders.Create(context.Background(), "user-1", CreateFolderInput{Name: "  "}); !errors.Is(err, ErrInvalidFolderName) {
		t.Fatalf("empty name error = %v", err)
	}
	if err := folders.MoveFiles(context.Background(), "user-1", MoveFilesInput{}); !errors.Is(err, ErrTooManyFiles) {
		t.Fatalf("empty move error = %v", err)
	}
}

func TestFoldersRootScopeAndMemberNormalization(t *testing.T) {
	store := &folderStoreStub{}
	folders := NewFolders(store, time.Now)
	if _, err := folders.ListRoot(context.Background(), "user-1", "unexpected", ListFolderInput{}); err != nil {
		t.Fatal(err)
	}
	if store.scope != "owned" {
		t.Fatalf("scope = %q", store.scope)
	}
	if _, err := folders.AddMember(context.Background(), "user-1", "folder-1", AddFolderMemberInput{Email: " Viewer@Example.COM ", Role: "VIEWER"}); err != nil {
		t.Fatal(err)
	}
	if store.memberEmail != "viewer@example.com" {
		t.Fatalf("member email = %q", store.memberEmail)
	}
	if store.memberRole != domain.FolderRoleViewer {
		t.Fatalf("member role = %q", store.memberRole)
	}
	if _, err := folders.AddMember(context.Background(), "user-1", "folder-1", AddFolderMemberInput{Email: "editor@example.com", Role: "contributor"}); err != nil {
		t.Fatal(err)
	}
	if store.memberRole != domain.FolderRoleContributor {
		t.Fatalf("contributor role = %q", store.memberRole)
	}
	if _, err := folders.AddMember(context.Background(), "user-1", "folder-1", AddFolderMemberInput{Email: "editor@example.com", Role: "OWNER"}); !errors.Is(err, ErrInvalidFolderRole) {
		t.Fatalf("invalid role error = %v", err)
	}
}

func TestFoldersCreateAndAcceptInvite(t *testing.T) {
	now := time.Date(2026, time.September, 4, 12, 0, 0, 0, time.UTC)
	store := &folderStoreStub{}
	folders := NewFolders(store, func() time.Time { return now })

	result, err := folders.CreateInvite(context.Background(), "owner-1", "folder-1", CreateFolderInviteInput{Role: "CONTRIBUTOR", ExpiresIn: "7d"})
	if err != nil {
		t.Fatal(err)
	}
	if result.Invite.Role != domain.FolderRoleContributor || result.Invite.ExpiresAt == nil || !result.Invite.ExpiresAt.Equal(now.Add(7*24*time.Hour)) {
		t.Fatalf("invite = %#v", result.Invite)
	}
	if result.InvitePath != "/join/"+result.Invite.ShortCode || store.invite.ID != result.Invite.ID {
		t.Fatalf("invite result = %#v, stored = %#v", result, store.invite)
	}
	if _, err := folders.CreateInvite(context.Background(), "owner-1", "folder-1", CreateFolderInviteInput{Role: "OWNER", ExpiresIn: "7d"}); !errors.Is(err, ErrInvalidFolderRole) {
		t.Fatalf("invalid invite role error = %v", err)
	}
	if _, err := folders.CreateInvite(context.Background(), "owner-1", "folder-1", CreateFolderInviteInput{Role: "VIEWER", ExpiresIn: "forever"}); !errors.Is(err, ErrInvalidShareExpiration) {
		t.Fatalf("invalid invite expiration error = %v", err)
	}

	if _, err := folders.AcceptInvite(context.Background(), "user-2", "  join-code  "); err != nil {
		t.Fatal(err)
	}
	if store.acceptedCode != "join-code" || !store.acceptedAt.Equal(now) {
		t.Fatalf("accepted code = %q at %v", store.acceptedCode, store.acceptedAt)
	}
	if _, err := folders.PreviewInvite(context.Background(), "  preview-code  "); err != nil {
		t.Fatal(err)
	}
	if store.previewCode != "preview-code" || !store.previewAt.Equal(now) {
		t.Fatalf("preview code = %q at %v", store.previewCode, store.previewAt)
	}
}

func TestFoldersEncodeAndValidateCursor(t *testing.T) {
	createdAt := time.Date(2026, time.September, 4, 12, 0, 0, 0, time.UTC)
	store := &folderStoreStub{
		contents: domain.FolderContents{Files: []domain.OwnedFile{{File: domain.File{ID: "file-1", OriginalName: "Alpha.txt", CreatedAt: createdAt}}}},
		hasMore:  true,
	}
	folders := NewFolders(store, time.Now)
	result, err := folders.ListRoot(context.Background(), "user-1", "owned", ListFolderInput{Sort: "name", Limit: 1})
	if err != nil || result.NextCursor == "" {
		t.Fatalf("cursor result = %#v, error = %v", result, err)
	}
	if _, err := folders.ListRoot(context.Background(), "user-1", "owned", ListFolderInput{Sort: "name", Limit: 1, Cursor: result.NextCursor}); err != nil {
		t.Fatal(err)
	}
	if store.query.CursorID != "file-1" || store.query.CursorName != "alpha.txt" {
		t.Fatalf("decoded cursor = %#v", store.query)
	}
	if _, err := folders.ListRoot(context.Background(), "user-1", "owned", ListFolderInput{Cursor: "not-a-cursor"}); !errors.Is(err, ErrInvalidLibraryQuery) {
		t.Fatalf("invalid cursor error = %v", err)
	}
}

func TestFolderSummaryPreservesFolderUsageAndReportsAccountCapacity(t *testing.T) {
	store := &folderSummaryStore{
		folderStoreStub: &folderStoreStub{contents: domain.FolderContents{
			Summary: domain.FileLibrarySummary{FileCount: 4, TotalBytes: 384 * 1024 * 1024},
		}},
		accountUsage: domain.FileLibrarySummary{FileCount: 9, TotalBytes: 2 * 1024 * 1024 * 1024},
		quota:        1024 * 1024 * 1024 * 1024,
	}
	folders := NewFolders(store, time.Now)

	result, err := folders.Contents(context.Background(), "user-1", "folder-1", ListFolderInput{})
	if err != nil {
		t.Fatal(err)
	}
	if result.Summary.FileCount != 4 || result.Summary.TotalBytes != 384*1024*1024 {
		t.Fatalf("folder summary = %#v", result.Summary)
	}
	if result.Summary.AccountTotalBytes != 2*1024*1024*1024 || result.Summary.QuotaBytes != 1024*1024*1024*1024 {
		t.Fatalf("account capacity = %#v", result.Summary)
	}
}

type folderStoreStub struct {
	created       domain.Folder
	scope         string
	movedIDs      []string
	movedFolderID *string
	memberEmail   string
	memberRole    domain.FolderRole
	invite        domain.FolderInvite
	acceptedCode  string
	acceptedAt    time.Time
	previewCode   string
	previewAt     time.Time
	contents      domain.FolderContents
	hasMore       bool
	query         domain.FileLibraryQuery
}

func (s *folderStoreStub) CreateFolder(_ context.Context, folder domain.Folder) error {
	s.created = folder
	return nil
}

func (s *folderStoreStub) GetRootContents(_ context.Context, _ string, scope string, _ time.Time, query domain.FileLibraryQuery) (domain.FolderContents, bool, error) {
	s.scope = scope
	s.query = query
	return s.contents, s.hasMore, nil
}

func (s *folderStoreStub) GetFolderContents(context.Context, string, string, time.Time, domain.FileLibraryQuery) (domain.FolderContents, bool, error) {
	return s.contents, s.hasMore, nil
}

func (*folderStoreStub) UpdateFolder(_ context.Context, ownerID, folderID, name string, parentFolderID *string) (domain.Folder, error) {
	return domain.Folder{ID: folderID, OwnerID: ownerID, Name: name, ParentFolderID: parentFolderID}, nil
}

func (*folderStoreStub) DeleteFolder(context.Context, string, string) error { return nil }

func (*folderStoreStub) ListFolderMembers(context.Context, string, string) ([]domain.FolderMember, error) {
	return nil, nil
}

func (s *folderStoreStub) AddFolderMember(_ context.Context, _, _, email string, role domain.FolderRole, createdAt time.Time) (domain.FolderMember, error) {
	s.memberEmail = email
	s.memberRole = role
	return domain.FolderMember{Role: role, CreatedAt: createdAt}, nil
}

func (*folderStoreStub) RemoveFolderMember(context.Context, string, string, string) error { return nil }

func (s *folderStoreStub) MoveOwnedFiles(_ context.Context, _ string, fileIDs []string, folderID *string) error {
	s.movedIDs = fileIDs
	s.movedFolderID = folderID
	return nil
}

func (*folderStoreStub) RemoveContributedFile(context.Context, string, string, string) error {
	return nil
}
func (s *folderStoreStub) CreateFolderInvite(_ context.Context, _ string, invite domain.FolderInvite) error {
	s.invite = invite
	return nil
}
func (*folderStoreStub) ListFolderInvites(context.Context, string, string, time.Time) ([]domain.FolderInvite, error) {
	return nil, nil
}
func (*folderStoreStub) RevokeFolderInvite(context.Context, string, string, string, time.Time) error {
	return nil
}
func (s *folderStoreStub) GetFolderInvitePreview(_ context.Context, code string, now time.Time) (domain.FolderInvitePreview, error) {
	s.previewCode = code
	s.previewAt = now
	return domain.FolderInvitePreview{}, nil
}
func (s *folderStoreStub) AcceptFolderInvite(_ context.Context, _, code string, now time.Time) (domain.FolderAccess, error) {
	s.acceptedCode = code
	s.acceptedAt = now
	return domain.FolderAccess{}, nil
}

type folderSummaryStore struct {
	*folderStoreStub
	accountUsage domain.FileLibrarySummary
	quota        int64
}

func (s *folderSummaryStore) GetOwnedFileUsage(context.Context, string) (domain.FileLibrarySummary, error) {
	return s.accountUsage, nil
}

func (s *folderSummaryStore) GetEffectiveStorageQuota(context.Context, string, int64) (int64, error) {
	return s.quota, nil
}
