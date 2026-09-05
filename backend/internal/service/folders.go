package service

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/eterealink/eterealink/backend/internal/domain"
)

const maxFolderNameBytes = 255

var (
	ErrInvalidFolderName   = errors.New("folder name must contain 1 to 255 characters")
	ErrFolderNotEmpty      = errors.New("folder must be empty before it can be deleted")
	ErrInvalidFolderMove   = errors.New("a folder cannot be moved into itself or one of its descendants")
	ErrInvalidMember       = errors.New("viewer must be a different existing Eterealink user")
	ErrTooManyFiles        = errors.New("choose between 1 and 100 files")
	ErrInvalidLibraryQuery = errors.New("library search or cursor is invalid")
)

type FolderStore interface {
	CreateFolder(ctx context.Context, folder domain.Folder) error
	GetRootContents(ctx context.Context, userID, scope string, now time.Time, query domain.FileLibraryQuery) (domain.FolderContents, bool, error)
	GetFolderContents(ctx context.Context, userID, folderID string, now time.Time, query domain.FileLibraryQuery) (domain.FolderContents, bool, error)
	UpdateFolder(ctx context.Context, ownerID, folderID, name string, parentFolderID *string) (domain.Folder, error)
	DeleteFolder(ctx context.Context, ownerID, folderID string) error
	ListFolderMembers(ctx context.Context, ownerID, folderID string) ([]domain.FolderMember, error)
	AddFolderMember(ctx context.Context, ownerID, folderID, email string, createdAt time.Time) (domain.FolderMember, error)
	RemoveFolderMember(ctx context.Context, ownerID, folderID, userID string) error
	MoveOwnedFiles(ctx context.Context, ownerID string, fileIDs []string, folderID *string) error
}

type Folders struct {
	store           FolderStore
	now             Clock
	maxAccountBytes int64
}

type CreateFolderInput struct {
	Name           string  `json:"name"`
	ParentFolderID *string `json:"parentFolderId"`
}

type UpdateFolderInput struct {
	Name           string  `json:"name"`
	ParentFolderID *string `json:"parentFolderId"`
}

type AddFolderMemberInput struct {
	Email string `json:"email"`
}

type MoveFilesInput struct {
	FileIDs  []string `json:"fileIds"`
	FolderID *string  `json:"folderId"`
}

type ListFolderInput struct {
	Search string
	Sort   string
	Filter string
	Limit  int
	Cursor string
}

type fileCursor struct {
	Sort      string    `json:"s"`
	ID        string    `json:"i"`
	CreatedAt time.Time `json:"t,omitempty"`
	Name      string    `json:"n,omitempty"`
	SizeBytes int64     `json:"z,omitempty"`
}

func NewFolders(store FolderStore, now Clock, maxAccountBytes ...int64) *Folders {
	service := &Folders{store: store, now: now}
	if len(maxAccountBytes) > 0 {
		service.maxAccountBytes = maxAccountBytes[0]
	}
	return service
}

func normalizeFolderName(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" || len([]byte(value)) > maxFolderNameBytes {
		return "", ErrInvalidFolderName
	}
	return value, nil
}

func normalizeOptionalID(value *string) *string {
	if value == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*value)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

func (s *Folders) Create(ctx context.Context, ownerID string, input CreateFolderInput) (domain.Folder, error) {
	name, err := normalizeFolderName(input.Name)
	if err != nil {
		return domain.Folder{}, err
	}
	id, err := newUUID()
	if err != nil {
		return domain.Folder{}, err
	}
	folder := domain.Folder{
		ID: id, OwnerID: strings.TrimSpace(ownerID), ParentFolderID: normalizeOptionalID(input.ParentFolderID),
		Name: name, CreatedAt: s.now().UTC(),
	}
	if folder.OwnerID == "" {
		return domain.Folder{}, domain.ErrNotFound
	}
	if err := s.store.CreateFolder(ctx, folder); err != nil {
		return domain.Folder{}, err
	}
	return folder, nil
}

func normalizeLibraryQuery(input ListFolderInput) (domain.FileLibraryQuery, error) {
	query := domain.FileLibraryQuery{Search: strings.TrimSpace(input.Search), SharedOnly: input.Filter == "shared", Sort: input.Sort, Limit: input.Limit}
	if len(query.Search) > 256 {
		return domain.FileLibraryQuery{}, ErrInvalidLibraryQuery
	}
	if query.Limit <= 0 {
		query.Limit = 10
	}
	if query.Limit > 100 {
		query.Limit = 100
	}
	switch query.Sort {
	case "oldest", "name", "size":
	default:
		query.Sort = "newest"
	}
	if input.Cursor == "" {
		return query, nil
	}
	encoded, err := base64.RawURLEncoding.DecodeString(input.Cursor)
	if err != nil {
		return domain.FileLibraryQuery{}, ErrInvalidLibraryQuery
	}
	var cursor fileCursor
	if err := json.Unmarshal(encoded, &cursor); err != nil || cursor.ID == "" || cursor.Sort != query.Sort {
		return domain.FileLibraryQuery{}, ErrInvalidLibraryQuery
	}
	query.CursorID, query.CursorTime, query.CursorName, query.CursorSize = cursor.ID, cursor.CreatedAt, cursor.Name, cursor.SizeBytes
	return query, nil
}

func nextLibraryCursor(query domain.FileLibraryQuery, contents domain.FolderContents, hasMore bool) string {
	if !hasMore || len(contents.Files) == 0 {
		return ""
	}
	last := contents.Files[len(contents.Files)-1].File
	cursor := fileCursor{Sort: query.Sort, ID: last.ID, CreatedAt: last.CreatedAt, Name: strings.ToLower(last.OriginalName), SizeBytes: last.SizeBytes}
	encoded, _ := json.Marshal(cursor)
	return base64.RawURLEncoding.EncodeToString(encoded)
}

func (s *Folders) ListRoot(ctx context.Context, userID, scope string, input ListFolderInput) (domain.FolderContents, error) {
	if scope != "shared" {
		scope = "owned"
	}
	query, err := normalizeLibraryQuery(input)
	if err != nil {
		return domain.FolderContents{}, err
	}
	result, hasMore, err := s.store.GetRootContents(ctx, userID, scope, s.now().UTC(), query)
	if err == nil && scope == "owned" {
		result.Summary.QuotaBytes = s.maxAccountBytes
	}
	result.NextCursor = nextLibraryCursor(query, result, hasMore)
	return result, err
}

func (s *Folders) Contents(ctx context.Context, userID, folderID string, input ListFolderInput) (domain.FolderContents, error) {
	query, err := normalizeLibraryQuery(input)
	if err != nil {
		return domain.FolderContents{}, err
	}
	result, hasMore, err := s.store.GetFolderContents(ctx, strings.TrimSpace(userID), strings.TrimSpace(folderID), s.now().UTC(), query)
	if err == nil && result.Current != nil && result.Current.Role == domain.FolderRoleOwner {
		result.Summary.QuotaBytes = s.maxAccountBytes
	}
	result.NextCursor = nextLibraryCursor(query, result, hasMore)
	return result, err
}

func (s *Folders) Update(ctx context.Context, ownerID, folderID string, input UpdateFolderInput) (domain.Folder, error) {
	name, err := normalizeFolderName(input.Name)
	if err != nil {
		return domain.Folder{}, err
	}
	return s.store.UpdateFolder(ctx, ownerID, folderID, name, normalizeOptionalID(input.ParentFolderID))
}

func (s *Folders) Delete(ctx context.Context, ownerID, folderID string) error {
	return s.store.DeleteFolder(ctx, ownerID, folderID)
}

func (s *Folders) Members(ctx context.Context, ownerID, folderID string) ([]domain.FolderMember, error) {
	members, err := s.store.ListFolderMembers(ctx, ownerID, folderID)
	if members == nil {
		members = []domain.FolderMember{}
	}
	return members, err
}

func (s *Folders) AddMember(ctx context.Context, ownerID, folderID string, input AddFolderMemberInput) (domain.FolderMember, error) {
	email := strings.ToLower(strings.TrimSpace(input.Email))
	if email == "" {
		return domain.FolderMember{}, ErrInvalidMember
	}
	return s.store.AddFolderMember(ctx, ownerID, folderID, email, s.now().UTC())
}

func (s *Folders) RemoveMember(ctx context.Context, ownerID, folderID, userID string) error {
	return s.store.RemoveFolderMember(ctx, ownerID, folderID, userID)
}

func (s *Folders) MoveFiles(ctx context.Context, ownerID string, input MoveFilesInput) error {
	if len(input.FileIDs) == 0 || len(input.FileIDs) > 100 {
		return ErrTooManyFiles
	}
	seen := make(map[string]struct{}, len(input.FileIDs))
	ids := make([]string, 0, len(input.FileIDs))
	for _, id := range input.FileIDs {
		id = strings.TrimSpace(id)
		if id == "" {
			return ErrTooManyFiles
		}
		if _, ok := seen[id]; !ok {
			seen[id] = struct{}{}
			ids = append(ids, id)
		}
	}
	return s.store.MoveOwnedFiles(ctx, ownerID, ids, normalizeOptionalID(input.FolderID))
}
