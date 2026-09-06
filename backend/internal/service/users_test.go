package service

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/eterealink/eterealink/backend/internal/domain"
)

func TestProvisionUser(t *testing.T) {
	now := time.Date(2026, 9, 3, 12, 0, 0, 0, time.FixedZone("local", -7*60*60))
	store := &recordingUserStore{}
	users := NewUsers(store, func() time.Time { return now })

	user, err := users.Provision(context.Background(), AuthenticatedIdentity{
		FirebaseUID: " firebase-123 ", Email: " person@example.com ", DisplayName: " Person ",
	})
	if err != nil {
		t.Fatal(err)
	}
	if user.ID == "" || user.FirebaseUID != "firebase-123" || user.Email != "person@example.com" || user.DisplayName != "Person" {
		t.Fatalf("user = %#v", user)
	}
	if !user.CreatedAt.Equal(now.UTC()) {
		t.Fatalf("created at = %s, want %s", user.CreatedAt, now.UTC())
	}
}

func TestProvisionUserRequiresUIDAndEmail(t *testing.T) {
	users := NewUsers(&recordingUserStore{}, time.Now)
	for _, identity := range []AuthenticatedIdentity{
		{Email: "person@example.com"},
		{FirebaseUID: "firebase-123"},
	} {
		if _, err := users.Provision(context.Background(), identity); !errors.Is(err, ErrInvalidIdentity) {
			t.Fatalf("Provision(%#v) error = %v", identity, err)
		}
	}
}

func TestProvisionUserFallsBackToEmailForDisplayName(t *testing.T) {
	users := NewUsers(&recordingUserStore{}, time.Now)
	user, err := users.Provision(context.Background(), AuthenticatedIdentity{
		FirebaseUID: "firebase-123", Email: "person@example.com",
	})
	if err != nil {
		t.Fatal(err)
	}
	if user.DisplayName != "person@example.com" {
		t.Fatalf("display name = %q", user.DisplayName)
	}
}

func TestUpdateDisplayNameNormalizesWhitespaceAndAllowsClear(t *testing.T) {
	store := &recordingUserStore{}
	users := NewUsers(store, time.Now)
	value := "  Johnny   Cloud  "
	user, err := users.UpdateDisplayName(context.Background(), "user-1", &value)
	if err != nil {
		t.Fatal(err)
	}
	if user.DisplayName != "Johnny Cloud" || store.customDisplayName == nil || *store.customDisplayName != "Johnny Cloud" {
		t.Fatalf("updated user = %#v, stored name = %#v", user, store.customDisplayName)
	}
	if _, err := users.UpdateDisplayName(context.Background(), "user-1", nil); err != nil || store.customDisplayName != nil {
		t.Fatalf("clear error = %v, stored name = %#v", err, store.customDisplayName)
	}
}

func TestUpdateDisplayNameRejectsUnsafeValues(t *testing.T) {
	users := NewUsers(&recordingUserStore{}, time.Now)
	for _, value := range []string{"  ", "ab", strings.Repeat("x", 41), "bad\u0007name", "bad\tname"} {
		if _, err := users.UpdateDisplayName(context.Background(), "user-1", &value); !errors.Is(err, ErrInvalidDisplayName) {
			t.Fatalf("UpdateDisplayName(%q) error = %v", value, err)
		}
	}
}

func TestAdministratorUpdatesAndResetsStorageQuota(t *testing.T) {
	users := NewUsers(&recordingUserStore{}, time.Now, 25)
	admin := domain.User{ID: "admin", IsAdmin: true}
	target := "98000000-0000-4000-8000-000000000001"
	override := int64(100)
	updated, err := users.UpdateStorageQuota(context.Background(), admin, target, &override)
	if err != nil || updated.StorageQuotaBytes == nil || *updated.StorageQuotaBytes != 100 || updated.EffectiveQuota != 100 {
		t.Fatalf("updated quota = %#v, error = %v", updated, err)
	}
	reset, err := users.UpdateStorageQuota(context.Background(), admin, target, nil)
	if err != nil || reset.StorageQuotaBytes != nil || reset.EffectiveQuota != 25 {
		t.Fatalf("reset quota = %#v, error = %v", reset, err)
	}
}

func TestStorageQuotaUpdateRequiresAdministratorAndValidInput(t *testing.T) {
	users := NewUsers(&recordingUserStore{}, time.Now)
	validID := "98000000-0000-4000-8000-000000000001"
	positive := int64(1)
	if _, err := users.UpdateStorageQuota(context.Background(), domain.User{}, validID, &positive); !errors.Is(err, ErrForbidden) {
		t.Fatalf("non-admin error = %v", err)
	}
	admin := domain.User{IsAdmin: true}
	if _, err := users.UpdateStorageQuota(context.Background(), admin, "not-a-uuid", &positive); !errors.Is(err, ErrInvalidUserID) {
		t.Fatalf("invalid id error = %v", err)
	}
	zero := int64(0)
	if _, err := users.UpdateStorageQuota(context.Background(), admin, validID, &zero); !errors.Is(err, ErrInvalidQuota) {
		t.Fatalf("invalid quota error = %v", err)
	}
}

type recordingUserStore struct{ customDisplayName *string }

func (s *recordingUserStore) UpsertUser(_ context.Context, user domain.User) (domain.User, error) {
	return user, nil
}

func (s *recordingUserStore) UpdateCustomDisplayName(_ context.Context, userID string, displayName *string) (domain.User, error) {
	s.customDisplayName = displayName
	effective := "Google Person"
	if displayName != nil {
		effective = *displayName
	}
	return domain.User{ID: userID, Email: "person@example.com", DisplayName: effective, IdentityDisplayName: "Google Person", CustomDisplayName: displayName}, nil
}

func (s *recordingUserStore) UpdateStorageQuota(_ context.Context, userID string, quota *int64, defaultQuota int64) (domain.UserQuota, error) {
	effective := defaultQuota
	if quota != nil {
		effective = *quota
	}
	return domain.UserQuota{UserID: userID, StorageQuotaBytes: quota, EffectiveQuota: effective}, nil
}
