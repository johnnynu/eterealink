# ADR 0005: Verify Firebase identity at the API boundary

## Status

Accepted for Phase 4.

## Context

Eterealink needs Google Sign-In before persistent files and folder permissions can be attached to an owner. Anonymous transfers must remain available without credentials. The browser may know that a Firebase user is signed in, but authorization decisions must never trust client-provided profile fields or an unverified user identifier.

## Decision

- Use the modular Firebase JavaScript SDK for Google Sign-In and browser session persistence.
- Send the current Firebase ID token to protected API routes as an `Authorization: Bearer` credential.
- Verify the token's signature, issuer, audience, and timestamps in the Go API with the Firebase Admin SDK.
- Derive the Firebase UID, email, and display name only from verified token claims.
- Upsert the corresponding PostgreSQL user by immutable Firebase UID when a verified identity first reaches the API. Return the existing internal UUID on subsequent requests.
- Put the provisioned domain user in request context so later protected file and folder handlers can authorize against the internal user ID.
- Keep anonymous upload, completion, and share-resolution routes outside the authentication middleware.
- Allow authentication to be unconfigured during local anonymous-flow development. In that mode the protected identity endpoint returns `503` and the browser does not show a sign-in control.

## Consequences

Firebase owns credential issuance and browser session refresh, while PostgreSQL remains the source of truth for application ownership and permissions. Protected requests perform local token verification after Firebase public signing keys have been cached. Token revocation is not checked on every request because that would require a remote lookup; short-lived ID tokens and normal Firebase refresh behavior provide the initial Phase 4 boundary.

The first authenticated endpoint is `GET /v1/me`. Phase 5 can wrap persistent upload and folder routes with the same middleware without changing anonymous behavior.
