package identity

import "testing"

func TestStringClaim(t *testing.T) {
	claims := map[string]any{"email": "person@example.com", "email_verified": true}
	if got := stringClaim(claims, "email"); got != "person@example.com" {
		t.Fatalf("email = %q", got)
	}
	if got := stringClaim(claims, "email_verified"); got != "" {
		t.Fatalf("non-string claim = %q", got)
	}
}
