package api

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/eterealink/eterealink/backend/internal/domain"
	"github.com/eterealink/eterealink/backend/internal/identity"
	"github.com/eterealink/eterealink/backend/internal/service"
	"github.com/eterealink/eterealink/backend/internal/storage"
)

func TestHealthAndReadiness(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	t.Run("health does not depend on database", func(t *testing.T) {
		handler := NewHandler(nil, nil, nil, nil, nil, readiness{err: errors.New("offline")}, logger)
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/healthz", nil))
		if response.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", response.Code)
		}
	})

	t.Run("readiness reflects database", func(t *testing.T) {
		handler := NewHandler(nil, nil, nil, nil, nil, readiness{err: errors.New("offline")}, logger)
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/readyz", nil))
		if response.Code != http.StatusServiceUnavailable {
			t.Fatalf("status = %d, want 503", response.Code)
		}
	})
}

func TestInvalidUploadJSON(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	handler := NewHandler(nil, nil, nil, nil, nil, readiness{}, logger)
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/v1/uploads", strings.NewReader(`{"originalName":"x","surprise":true}`))
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", response.Code)
	}
}

func TestCurrentUserRequiresAuthentication(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	users := service.NewUsers(&userStore{}, func() time.Time { return time.Unix(100, 0) })
	handler := NewHandler(nil, nil, nil, users, tokenVerifier{}, readiness{}, logger)

	tests := []struct {
		name          string
		authorization string
		wantStatus    int
	}{
		{name: "missing token", wantStatus: http.StatusUnauthorized},
		{name: "wrong scheme", authorization: "Basic abc", wantStatus: http.StatusUnauthorized},
		{name: "invalid token", authorization: "Bearer invalid", wantStatus: http.StatusUnauthorized},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, "/v1/me", nil)
			request.Header.Set("Authorization", test.authorization)
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)
			if response.Code != test.wantStatus {
				t.Fatalf("status = %d, want %d", response.Code, test.wantStatus)
			}
			if response.Header().Get("WWW-Authenticate") == "" {
				t.Fatal("WWW-Authenticate header is missing")
			}
		})
	}
}

func TestCurrentUserVerifiesAndProvisionsIdentity(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	store := &userStore{}
	users := service.NewUsers(store, func() time.Time { return time.Unix(100, 0) })
	verifier := tokenVerifier{claims: identity.Claims{
		UID: "firebase-user", Email: "person@example.com", DisplayName: "Person",
	}}
	handler := NewHandler(nil, nil, nil, users, verifier, readiness{}, logger)
	request := httptest.NewRequest(http.MethodGet, "/v1/me", nil)
	request.Header.Set("Authorization", "bearer verified-token")
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", response.Code, response.Body.String())
	}
	if store.user.FirebaseUID != "firebase-user" || store.user.Email != "person@example.com" {
		t.Fatalf("provisioned user = %#v", store.user)
	}
	if !strings.Contains(response.Body.String(), `"displayName":"Person"`) || strings.Contains(response.Body.String(), "firebase-user") {
		t.Fatalf("response = %s", response.Body.String())
	}
}

func TestCurrentUserReportsDisabledAuthentication(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	handler := NewHandler(nil, nil, nil, nil, nil, readiness{}, logger)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/v1/me", nil))
	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", response.Code)
	}
}

func TestUpdateCurrentUserDisplayName(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	store := &userStore{}
	users := service.NewUsers(store, time.Now)
	verifier := tokenVerifier{claims: identity.Claims{UID: "firebase-user", Email: "person@example.com", DisplayName: "Google Person"}}
	handler := NewHandler(nil, nil, nil, users, verifier, readiness{}, logger)

	request := httptest.NewRequest(http.MethodPatch, "/v1/me", strings.NewReader(`{"displayName":"  Johnny   Cloud "}`))
	request.Header.Set("Authorization", "Bearer verified-token")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"displayName":"Johnny Cloud"`) || !strings.Contains(response.Body.String(), `"identityDisplayName":"Google Person"`) {
		t.Fatalf("response = %d %s", response.Code, response.Body.String())
	}

	request = httptest.NewRequest(http.MethodPatch, "/v1/me", strings.NewReader(`{"displayName":null}`))
	request.Header.Set("Authorization", "Bearer verified-token")
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"displayName":"Google Person"`) || !strings.Contains(response.Body.String(), `"customDisplayName":null`) {
		t.Fatalf("clear response = %d %s", response.Code, response.Body.String())
	}
}

func TestUpdateCurrentUserDisplayNameValidationAndConflict(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	store := &userStore{}
	users := service.NewUsers(store, time.Now)
	verifier := tokenVerifier{claims: identity.Claims{UID: "firebase-user", Email: "person@example.com", DisplayName: "Person"}}
	handler := NewHandler(nil, nil, nil, users, verifier, readiness{}, logger)

	request := httptest.NewRequest(http.MethodPatch, "/v1/me", strings.NewReader(`{"displayName":" "}`))
	request.Header.Set("Authorization", "Bearer verified-token")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusUnprocessableEntity || !strings.Contains(response.Body.String(), `"code":"validation_failed"`) {
		t.Fatalf("validation response = %d %s", response.Code, response.Body.String())
	}

	store.updateErr = service.ErrDisplayNameTaken
	request = httptest.NewRequest(http.MethodPatch, "/v1/me", strings.NewReader(`{"displayName":"Someone"}`))
	request.Header.Set("Authorization", "Bearer verified-token")
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusConflict || !strings.Contains(response.Body.String(), `"code":"display_name_taken"`) {
		t.Fatalf("conflict response = %d %s", response.Code, response.Body.String())
	}
}

func TestPersistentUploadUsesAuthenticatedOwner(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	now := time.Date(2026, time.September, 3, 12, 0, 0, 0, time.UTC)
	usersStore := &userStore{}
	users := service.NewUsers(usersStore, func() time.Time { return now })
	filesStore := &handlerFileStore{}
	files := service.NewFiles(filesStore, handlerFileBackend{}, func() time.Time { return now }, 15*time.Minute, 100)
	verifier := tokenVerifier{claims: identity.Claims{
		UID: "firebase-user", Email: "person@example.com", DisplayName: "Person",
	}}
	handler := NewHandler(nil, nil, files, users, verifier, readiness{}, logger)
	request := httptest.NewRequest(http.MethodPost, "/v1/files", strings.NewReader(`{
		"originalName":"notes.txt","mimeType":"text/plain","sizeBytes":5
	}`))
	request.Header.Set("Authorization", "Bearer verified-token")
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201: %s", response.Code, response.Body.String())
	}
	if filesStore.file.OwnerID == nil || *filesStore.file.OwnerID != usersStore.user.ID {
		t.Fatalf("file owner = %v, user = %q", filesStore.file.OwnerID, usersStore.user.ID)
	}
	if filesStore.file.ExpiresAt != nil {
		t.Fatalf("persistent file expires at %v", filesStore.file.ExpiresAt)
	}

	filesStore.file.Status = domain.FileStatusReady
	listRequest := httptest.NewRequest(http.MethodGet, "/v1/files", nil)
	listRequest.Header.Set("Authorization", "Bearer verified-token")
	listResponse := httptest.NewRecorder()
	handler.ServeHTTP(listResponse, listRequest)
	if listResponse.Code != http.StatusOK {
		t.Fatalf("list status = %d, want 200: %s", listResponse.Code, listResponse.Body.String())
	}
	if !strings.Contains(listResponse.Body.String(), `"summary":{"fileCount":1,"totalBytes":5}`) {
		t.Fatalf("list response is missing storage summary: %s", listResponse.Body.String())
	}

	downloadRequest := httptest.NewRequest(http.MethodGet, "/v1/files/"+filesStore.file.ID+"/download", nil)
	downloadRequest.Header.Set("Authorization", "Bearer verified-token")
	downloadResponse := httptest.NewRecorder()
	handler.ServeHTTP(downloadResponse, downloadRequest)
	if downloadResponse.Code != http.StatusOK || !strings.Contains(downloadResponse.Body.String(), `"preview":{"kind":"text","url":"https://preview.invalid/`) {
		t.Fatalf("download response is missing text preview: %s", downloadResponse.Body.String())
	}

	shareRequest := httptest.NewRequest(http.MethodPost, "/v1/files/"+filesStore.file.ID+"/shares", strings.NewReader(`{"expiresIn":"30d"}`))
	shareRequest.Header.Set("Authorization", "Bearer verified-token")
	shareResponse := httptest.NewRecorder()
	handler.ServeHTTP(shareResponse, shareRequest)
	if shareResponse.Code != http.StatusCreated {
		t.Fatalf("share status = %d, want 201: %s", shareResponse.Code, shareResponse.Body.String())
	}
	if filesStore.share.CreatedBy == nil || *filesStore.share.CreatedBy != usersStore.user.ID {
		t.Fatalf("share creator = %v, user = %q", filesStore.share.CreatedBy, usersStore.user.ID)
	}
	if filesStore.share.ExpiresAt == nil || !filesStore.share.ExpiresAt.Equal(now.Add(30*24*time.Hour)) {
		t.Fatalf("share expiration = %v", filesStore.share.ExpiresAt)
	}

	revokeRequest := httptest.NewRequest(http.MethodDelete, "/v1/files/"+filesStore.file.ID+"/shares/"+filesStore.share.ID, nil)
	revokeRequest.Header.Set("Authorization", "Bearer verified-token")
	revokeResponse := httptest.NewRecorder()
	handler.ServeHTTP(revokeResponse, revokeRequest)
	if revokeResponse.Code != http.StatusNoContent {
		t.Fatalf("revoke status = %d, want 204: %s", revokeResponse.Code, revokeResponse.Body.String())
	}
	if filesStore.share.RevokedAt == nil {
		t.Fatal("share was not revoked")
	}
}

type tokenVerifier struct {
	claims identity.Claims
	err    error
}

func (v tokenVerifier) VerifyIDToken(_ context.Context, token string) (identity.Claims, error) {
	if token == "invalid" {
		return identity.Claims{}, errors.New("invalid token")
	}
	return v.claims, v.err
}

type userStore struct {
	user      domain.User
	updateErr error
}

func (s *userStore) UpsertUser(_ context.Context, user domain.User) (domain.User, error) {
	if s.user.FirebaseUID != "" && s.user.FirebaseUID == user.FirebaseUID {
		s.user.Email = user.Email
		s.user.IdentityDisplayName = user.DisplayName
		if s.user.CustomDisplayName == nil {
			s.user.DisplayName = user.DisplayName
		}
		return s.user, nil
	}
	user.IdentityDisplayName = user.DisplayName
	s.user = user
	return user, nil
}

func (s *userStore) UpdateCustomDisplayName(_ context.Context, userID string, displayName *string) (domain.User, error) {
	if s.updateErr != nil {
		return domain.User{}, s.updateErr
	}
	s.user.CustomDisplayName = displayName
	s.user.DisplayName = s.user.IdentityDisplayName
	if displayName != nil {
		s.user.DisplayName = *displayName
	}
	return s.user, nil
}

type readiness struct{ err error }

func (r readiness) Ping(context.Context) error { return r.err }

type handlerFileStore struct {
	file  domain.File
	share domain.ShareLink
}

func (s *handlerFileStore) CreateOwnedFile(_ context.Context, file domain.File) error {
	s.file = file
	return nil
}

func (s *handlerFileStore) GetOwnedFile(_ context.Context, ownerID, fileID string) (domain.File, error) {
	if s.file.ID != fileID || s.file.OwnerID == nil || *s.file.OwnerID != ownerID {
		return domain.File{}, domain.ErrNotFound
	}
	return s.file, nil
}

func (s *handlerFileStore) CompleteOwnedFile(_ context.Context, ownerID, fileID string, now time.Time) (domain.File, error) {
	file, err := s.GetOwnedFile(context.Background(), ownerID, fileID)
	if err != nil {
		return domain.File{}, err
	}
	file.Status = domain.FileStatusReady
	file.CompletedAt = &now
	s.file = file
	return file, nil
}

func (s *handlerFileStore) ListOwnedFiles(_ context.Context, ownerID string, _ time.Time) ([]domain.OwnedFile, error) {
	if s.file.OwnerID != nil && *s.file.OwnerID == ownerID {
		owned := domain.OwnedFile{File: s.file}
		if s.share.ID != "" {
			owned.Share = &s.share
		}
		return []domain.OwnedFile{owned}, nil
	}
	return []domain.OwnedFile{}, nil
}

func (s *handlerFileStore) GetOwnedFileUsage(_ context.Context, ownerID string) (domain.FileLibrarySummary, error) {
	if s.file.OwnerID != nil && *s.file.OwnerID == ownerID && s.file.Status == domain.FileStatusReady {
		return domain.FileLibrarySummary{FileCount: 1, TotalBytes: s.file.SizeBytes}, nil
	}
	return domain.FileLibrarySummary{}, nil
}

func (s *handlerFileStore) CreateOwnedFileShare(_ context.Context, ownerID, fileID string, share domain.ShareLink, _ time.Time) error {
	file, err := s.GetOwnedFile(context.Background(), ownerID, fileID)
	if err != nil {
		return err
	}
	if file.Status != domain.FileStatusReady || s.share.ID != "" {
		return domain.ErrConflict
	}
	s.share = share
	return nil
}

func (s *handlerFileStore) RevokeOwnedFileShare(_ context.Context, ownerID, fileID, shareID string, now time.Time) error {
	if _, err := s.GetOwnedFile(context.Background(), ownerID, fileID); err != nil {
		return err
	}
	if s.share.ID != shareID {
		return domain.ErrNotFound
	}
	s.share.RevokedAt = &now
	return nil
}

func (s *handlerFileStore) DeleteOwnedFile(_ context.Context, ownerID, fileID string) error {
	if _, err := s.GetOwnedFile(context.Background(), ownerID, fileID); err != nil {
		return err
	}
	s.file = domain.File{}
	return nil
}

type handlerFileBackend struct{}

func (handlerFileBackend) SignUpload(_ context.Context, key, _ string, expiresAt time.Time) (storage.UploadTarget, error) {
	return storage.UploadTarget{URL: "https://upload.invalid/" + key, Method: http.MethodPut, ExpiresAt: expiresAt}, nil
}

func (handlerFileBackend) SignResumableUpload(_ context.Context, key, _ string, expiresAt time.Time) (storage.UploadTarget, error) {
	return storage.UploadTarget{URL: "https://upload.invalid/" + key, Method: http.MethodPost, ExpiresAt: expiresAt}, nil
}

func (handlerFileBackend) SignDownload(_ context.Context, key, _ string, expiresAt time.Time) (storage.DownloadTarget, error) {
	return storage.DownloadTarget{URL: "https://download.invalid/" + key, ExpiresAt: expiresAt}, nil
}

func (handlerFileBackend) SignPreview(_ context.Context, key, _, _ string, expiresAt time.Time) (storage.PreviewTarget, error) {
	return storage.PreviewTarget{URL: "https://preview.invalid/" + key, ExpiresAt: expiresAt}, nil
}

func (handlerFileBackend) StatObject(context.Context, string) (storage.ObjectAttributes, error) {
	return storage.ObjectAttributes{SizeBytes: 5, MIMEType: "text/plain"}, nil
}

func (handlerFileBackend) DeleteObject(context.Context, string) error { return nil }
