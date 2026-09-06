package service

import (
	"context"
	"errors"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/eterealink/eterealink/backend/internal/domain"
	"github.com/google/uuid"
)

var (
	ErrInvalidIdentity    = errors.New("authenticated identity is incomplete")
	ErrInvalidDisplayName = errors.New("display name must be 3 to 40 characters and cannot contain control characters")
	ErrDisplayNameTaken   = domain.ErrDisplayNameTaken
	ErrForbidden          = errors.New("administrator access is required")
	ErrInvalidUserID      = errors.New("user id must be a valid UUID")
	ErrInvalidQuota       = errors.New("storage quota must be positive or null")
)

type UserStore interface {
	UpsertUser(ctx context.Context, user domain.User) (domain.User, error)
	UpdateCustomDisplayName(ctx context.Context, userID string, displayName *string) (domain.User, error)
	UpdateStorageQuota(ctx context.Context, userID string, storageQuotaBytes *int64, defaultQuota int64) (domain.UserQuota, error)
}

type AuthenticatedIdentity struct {
	FirebaseUID string
	Email       string
	DisplayName string
}

type Users struct {
	store        UserStore
	now          Clock
	defaultQuota int64
}

func NewUsers(store UserStore, now Clock, defaultQuota ...int64) *Users {
	quota := int64(25 * 1024 * 1024 * 1024)
	if len(defaultQuota) > 0 {
		quota = defaultQuota[0]
	}
	return &Users{store: store, now: now, defaultQuota: quota}
}

func (s *Users) UpdateStorageQuota(ctx context.Context, caller domain.User, targetUserID string, storageQuotaBytes *int64) (domain.UserQuota, error) {
	if !caller.IsAdmin {
		return domain.UserQuota{}, ErrForbidden
	}
	targetUserID = strings.TrimSpace(targetUserID)
	if _, err := uuid.Parse(targetUserID); err != nil {
		return domain.UserQuota{}, ErrInvalidUserID
	}
	if storageQuotaBytes != nil && *storageQuotaBytes <= 0 {
		return domain.UserQuota{}, ErrInvalidQuota
	}
	return s.store.UpdateStorageQuota(ctx, targetUserID, storageQuotaBytes, s.defaultQuota)
}

func (s *Users) Provision(ctx context.Context, identity AuthenticatedIdentity) (domain.User, error) {
	identity.FirebaseUID = strings.TrimSpace(identity.FirebaseUID)
	identity.Email = strings.TrimSpace(identity.Email)
	identity.DisplayName = strings.TrimSpace(identity.DisplayName)
	if identity.FirebaseUID == "" || identity.Email == "" {
		return domain.User{}, ErrInvalidIdentity
	}
	if identity.DisplayName == "" {
		identity.DisplayName = identity.Email
	}

	id, err := newUUID()
	if err != nil {
		return domain.User{}, err
	}
	return s.store.UpsertUser(ctx, domain.User{
		ID:                  id,
		FirebaseUID:         identity.FirebaseUID,
		Email:               identity.Email,
		DisplayName:         identity.DisplayName,
		IdentityDisplayName: identity.DisplayName,
		CreatedAt:           s.now().UTC(),
	})
}

func (s *Users) UpdateDisplayName(ctx context.Context, userID string, value *string) (domain.User, error) {
	if value == nil {
		return s.store.UpdateCustomDisplayName(ctx, userID, nil)
	}

	for _, character := range *value {
		if unicode.IsControl(character) {
			return domain.User{}, ErrInvalidDisplayName
		}
	}
	normalized := strings.Join(strings.Fields(*value), " ")
	if utf8.RuneCountInString(normalized) < 3 || utf8.RuneCountInString(normalized) > 40 {
		return domain.User{}, ErrInvalidDisplayName
	}
	return s.store.UpdateCustomDisplayName(ctx, userID, &normalized)
}
