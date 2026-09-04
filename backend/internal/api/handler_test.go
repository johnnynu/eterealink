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
)

func TestHealthAndReadiness(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	t.Run("health does not depend on database", func(t *testing.T) {
		handler := NewHandler(nil, nil, nil, nil, readiness{err: errors.New("offline")}, logger)
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/healthz", nil))
		if response.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", response.Code)
		}
	})

	t.Run("readiness reflects database", func(t *testing.T) {
		handler := NewHandler(nil, nil, nil, nil, readiness{err: errors.New("offline")}, logger)
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/readyz", nil))
		if response.Code != http.StatusServiceUnavailable {
			t.Fatalf("status = %d, want 503", response.Code)
		}
	})
}

func TestInvalidUploadJSON(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	handler := NewHandler(nil, nil, nil, nil, readiness{}, logger)
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
	handler := NewHandler(nil, nil, users, tokenVerifier{}, readiness{}, logger)

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
	handler := NewHandler(nil, nil, users, verifier, readiness{}, logger)
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
	handler := NewHandler(nil, nil, nil, nil, readiness{}, logger)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/v1/me", nil))
	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", response.Code)
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

type userStore struct{ user domain.User }

func (s *userStore) UpsertUser(_ context.Context, user domain.User) (domain.User, error) {
	s.user = user
	return user, nil
}

type readiness struct{ err error }

func (r readiness) Ping(context.Context) error { return r.err }
