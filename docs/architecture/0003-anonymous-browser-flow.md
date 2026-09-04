# ADR 0003: Keep anonymous transfers direct and same-origin

## Status

Accepted for Phase 3.

## Context

The anonymous product flow needs to coordinate three requests: create transfer metadata, upload bytes to a signed Cloud Storage URL, and confirm completion. It also needs a stable short share page that represents missing, incomplete, revoked, and expired transfers without exposing internal storage identifiers.

The frontend and API run on separate localhost ports during development. Calling the API directly from browser code would add a second CORS policy even though the only intentionally cross-origin operation is the direct object upload.

A signed download URL can remain usable without another API authorization check. If it were always issued for the full configured signed-URL lifetime, a URL created near the end of an anonymous transfer could outlive that transfer's 24-hour boundary.

## Decision

- Build the anonymous experience as a Next.js App Router application.
- Proxy browser control-plane requests under `/api/*` to the Go API. This keeps the application API same-origin while preserving direct browser-to-Cloud-Storage byte transfer.
- Treat file selection, metadata creation, byte upload, object verification, and link readiness as explicit client states.
- Use the API's `sharePath` as the canonical route rather than reconstructing short-code rules in the client.
- Resolve each recipient visit through the API and render distinct expired/revoked, missing, and temporarily unavailable states.
- Cap every signed download URL at the earlier of its configured URL lifetime, file expiration, and share expiration.
- Display expiration from server-issued timestamps; client countdowns are informational and never replace server enforcement.

## Consequences

The browser requires CORS access only to the configured private storage bucket, whose allowed local origins are maintained in `config/gcs-cors.json`. Deployments must set `API_BASE_URL` for the Next.js server and add the exact deployed frontend origin to the bucket policy.

Canceled or failed uploads can leave `PENDING` metadata and partial storage state until lifecycle cleanup is implemented in Phase 14. The API never marks those files `READY`, so their share links cannot resolve.
