package identity

import (
	"context"
	"fmt"
	"strings"

	firebase "firebase.google.com/go/v4"
	firebaseauth "firebase.google.com/go/v4/auth"
	"google.golang.org/api/option"
)

type Claims struct {
	UID         string
	Email       string
	DisplayName string
}

type Verifier interface {
	VerifyIDToken(ctx context.Context, rawToken string) (Claims, error)
}

type FirebaseVerifier struct {
	client *firebaseauth.Client
}

func NewFirebaseVerifier(ctx context.Context, projectID string) (*FirebaseVerifier, error) {
	projectID = strings.TrimSpace(projectID)
	if projectID == "" {
		return nil, fmt.Errorf("firebase project id is required")
	}
	app, err := firebase.NewApp(ctx, &firebase.Config{ProjectID: projectID}, option.WithoutAuthentication())
	if err != nil {
		return nil, fmt.Errorf("initialize firebase: %w", err)
	}
	client, err := app.Auth(ctx)
	if err != nil {
		return nil, fmt.Errorf("initialize firebase auth: %w", err)
	}
	return &FirebaseVerifier{client: client}, nil
}

func (v *FirebaseVerifier) VerifyIDToken(ctx context.Context, rawToken string) (Claims, error) {
	token, err := v.client.VerifyIDToken(ctx, rawToken)
	if err != nil {
		return Claims{}, err
	}
	return Claims{
		UID:         token.UID,
		Email:       stringClaim(token.Claims, "email"),
		DisplayName: stringClaim(token.Claims, "name"),
	}, nil
}

func stringClaim(claims map[string]any, name string) string {
	value, _ := claims[name].(string)
	return value
}
