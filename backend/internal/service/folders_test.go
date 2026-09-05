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
	if _, err := folders.AddMember(context.Background(), "user-1", "folder-1", AddFolderMemberInput{Email: " Viewer@Example.COM "}); err != nil {
		t.Fatal(err)
	}
	if store.memberEmail != "viewer@example.com" {
		t.Fatalf("member email = %q", store.memberEmail)
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

type folderStoreStub struct {
	created       domain.Folder
	scope         string
	movedIDs      []string
	movedFolderID *string
	memberEmail   string
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

func (*folderStoreStub) GetFolderContents(context.Context, string, string, time.Time, domain.FileLibraryQuery) (domain.FolderContents, bool, error) {
	return domain.FolderContents{}, false, nil
}

func (*folderStoreStub) UpdateFolder(_ context.Context, ownerID, folderID, name string, parentFolderID *string) (domain.Folder, error) {
	return domain.Folder{ID: folderID, OwnerID: ownerID, Name: name, ParentFolderID: parentFolderID}, nil
}

func (*folderStoreStub) DeleteFolder(context.Context, string, string) error { return nil }

func (*folderStoreStub) ListFolderMembers(context.Context, string, string) ([]domain.FolderMember, error) {
	return nil, nil
}

func (s *folderStoreStub) AddFolderMember(_ context.Context, _, _, email string, createdAt time.Time) (domain.FolderMember, error) {
	s.memberEmail = email
	return domain.FolderMember{Role: domain.FolderRoleViewer, CreatedAt: createdAt}, nil
}

func (*folderStoreStub) RemoveFolderMember(context.Context, string, string, string) error { return nil }

func (s *folderStoreStub) MoveOwnedFiles(_ context.Context, _ string, fileIDs []string, folderID *string) error {
	s.movedIDs = fileIDs
	s.movedFolderID = folderID
	return nil
}
