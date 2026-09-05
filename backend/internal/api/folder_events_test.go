package api

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/eterealink/eterealink/backend/internal/domain"
	"github.com/eterealink/eterealink/backend/internal/identity"
	"github.com/eterealink/eterealink/backend/internal/realtime"
	"github.com/eterealink/eterealink/backend/internal/service"
)

func TestFolderEventStreamAuthenticatesAndStreamsAnAncestorInvalidation(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	users := service.NewUsers(&userStore{}, time.Now)
	store := &folderEventStore{contents: domain.FolderContents{
		Current: &domain.FolderAccess{Folder: domain.Folder{ID: "folder-1", OwnerID: "owner-1", Name: "Plans"}, Role: domain.FolderRoleViewer},
		Breadcrumbs: []domain.Folder{
			{ID: "ancestor-1", OwnerID: "owner-1", Name: "Project"},
			{ID: "folder-1", OwnerID: "owner-1", Name: "Plans"},
		},
		Folders: []domain.FolderAccess{}, Files: []domain.OwnedFile{},
	}}
	folders := service.NewFolders(store, time.Now)
	events := &folderEventSubscriberStub{subscribed: make(chan []string, 1), events: make(chan realtime.FolderEvent, 1)}
	verifier := tokenVerifier{claims: identity.Claims{UID: "firebase-user", Email: "person@example.com"}}
	handler := NewHandlerWithRealtime(nil, nil, nil, users, verifier, readiness{}, logger, folders, events)
	request := httptest.NewRequest(http.MethodGet, "/v1/folders/folder-1/events", nil)
	request.Header.Set("Authorization", "Bearer verified-token")
	response := httptest.NewRecorder()
	done := make(chan struct{})
	go func() {
		handler.ServeHTTP(response, request)
		close(done)
	}()

	select {
	case folderIDs := <-events.subscribed:
		if !reflect.DeepEqual(folderIDs, []string{"ancestor-1", "folder-1"}) {
			t.Fatalf("subscriptions = %#v", folderIDs)
		}
	case <-time.After(time.Second):
		t.Fatal("event stream did not subscribe")
	}
	events.events <- realtime.FolderEvent{FolderID: "ancestor-1"}
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("event stream did not finish after invalidation")
	}

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	if got := response.Header().Get("Content-Type"); got != "text/event-stream" {
		t.Fatalf("content type = %q", got)
	}
	body := response.Body.String()
	if !strings.Contains(body, ": connected\n\n") || !strings.Contains(body, "event: folder.changed\ndata: {\"folderId\":\"ancestor-1\"}\n\n") {
		t.Fatalf("stream body = %q", body)
	}
}

func TestFolderEventStreamRequiresAuthentication(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	users := service.NewUsers(&userStore{}, time.Now)
	handler := NewHandlerWithRealtime(nil, nil, nil, users, tokenVerifier{}, readiness{}, logger, service.NewFolders(&folderEventStore{}, time.Now), &folderEventSubscriberStub{})
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/v1/folders/folder-1/events", nil))
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", response.Code)
	}
}

type folderEventSubscriberStub struct {
	subscribed chan []string
	events     chan realtime.FolderEvent
}

func (s *folderEventSubscriberStub) Subscribe(folderIDs []string) (<-chan realtime.FolderEvent, func()) {
	s.subscribed <- folderIDs
	return s.events, func() {}
}

type folderEventStore struct {
	contents domain.FolderContents
}

func (*folderEventStore) CreateFolder(context.Context, domain.Folder) error { return nil }
func (*folderEventStore) GetRootContents(context.Context, string, string, time.Time, domain.FileLibraryQuery) (domain.FolderContents, bool, error) {
	return domain.FolderContents{}, false, nil
}
func (s *folderEventStore) GetFolderContents(context.Context, string, string, time.Time, domain.FileLibraryQuery) (domain.FolderContents, bool, error) {
	if s.contents.Current == nil {
		return domain.FolderContents{}, false, domain.ErrNotFound
	}
	return s.contents, false, nil
}
func (*folderEventStore) UpdateFolder(context.Context, string, string, string, *string) (domain.Folder, error) {
	return domain.Folder{}, nil
}
func (*folderEventStore) DeleteFolder(context.Context, string, string) error { return nil }
func (*folderEventStore) ListFolderMembers(context.Context, string, string) ([]domain.FolderMember, error) {
	return nil, nil
}
func (*folderEventStore) AddFolderMember(context.Context, string, string, string, domain.FolderRole, time.Time) (domain.FolderMember, error) {
	return domain.FolderMember{}, nil
}
func (*folderEventStore) RemoveFolderMember(context.Context, string, string, string) error {
	return nil
}
func (*folderEventStore) MoveOwnedFiles(context.Context, string, []string, *string) error { return nil }
func (*folderEventStore) RemoveContributedFile(context.Context, string, string, string) error {
	return nil
}
func (*folderEventStore) CreateFolderInvite(context.Context, string, domain.FolderInvite) error {
	return nil
}
func (*folderEventStore) ListFolderInvites(context.Context, string, string, time.Time) ([]domain.FolderInvite, error) {
	return nil, nil
}
func (*folderEventStore) RevokeFolderInvite(context.Context, string, string, string, time.Time) error {
	return nil
}
func (*folderEventStore) GetFolderInvitePreview(context.Context, string, time.Time) (domain.FolderInvitePreview, error) {
	return domain.FolderInvitePreview{}, nil
}
func (*folderEventStore) AcceptFolderInvite(context.Context, string, string, time.Time) (domain.FolderAccess, error) {
	return domain.FolderAccess{}, nil
}
