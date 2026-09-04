package service

import (
	"context"
	"errors"
	"strings"

	"github.com/eterealink/eterealink/backend/internal/domain"
)

var ErrInvalidIdentity = errors.New("authenticated identity is incomplete")

type UserStore interface {
	UpsertUser(ctx context.Context, user domain.User) (domain.User, error)
}

type AuthenticatedIdentity struct {
	FirebaseUID string
	Email       string
	DisplayName string
}

type Users struct {
	store UserStore
	now   Clock
}

func NewUsers(store UserStore, now Clock) *Users {
	return &Users{store: store, now: now}
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
		ID:          id,
		FirebaseUID: identity.FirebaseUID,
		Email:       identity.Email,
		DisplayName: identity.DisplayName,
		CreatedAt:   s.now().UTC(),
	})
}
