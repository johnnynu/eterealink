# ADR 0002: Use keyless V4-signed Cloud Storage transfers

## Status

Accepted on September 2, 2026.

## Context

Eterealink must move file bytes directly between browsers and object storage without making the Go API a data proxy. Anonymous clients cannot receive bucket credentials, and marking a database record ready before confirming the object would allow incomplete or inconsistent shares.

## Decision

The Go API issues short-lived V4-signed `PUT` and `GET` URLs for a private Cloud Storage bucket.

- IAM Credentials `signBlob` signs URLs as the dedicated `eterealink-api` service account.
- Application Default Credentials identify the caller locally and on Cloud Run; no service-account key file is used.
- Upload URLs sign the declared `Content-Type` and an `x-goog-if-generation-match: 0` precondition, preventing reuse from replacing an existing object.
- The completion operation reads Cloud Storage metadata and requires the stored size and normalized media type to match the database record before changing `PENDING` to `READY`.
- Browser CORS is limited to known development origins and the headers and methods required by the signed request.

## Consequences

Cloud Run will handle small control-plane requests while Cloud Storage carries file bandwidth. The runtime identity needs bucket-scoped object access and permission to sign as itself. Local developers need permission to impersonate or invoke signing for that identity. Completion now depends on Cloud Storage availability, and expired abandoned uploads will require the lifecycle cleanup planned for a later phase.
