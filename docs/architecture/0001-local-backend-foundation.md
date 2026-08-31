# ADR 0001: Build a vertical anonymous-transfer core first

## Status

Accepted on August 30, 2026.

## Context

The design baseline has fourteen implementation phases and no existing code. Most later work depends on the same file, upload-state, expiration, and share-link rules. Building infrastructure before those rules are executable would make cloud configuration the first integration test for application behavior.

## Decision

The first implementation slice is a Go HTTP service backed by PostgreSQL that supports:

- liveness and dependency-aware readiness endpoints;
- anonymous upload metadata with a fixed 24-hour expiration;
- a `PENDING` to `READY` upload state transition;
- random 72-bit URL-safe share codes;
- share resolution that rejects incomplete, expired, and revoked records; and
- an object-storage signer interface with a clearly non-production development implementation.

The API uses Go's standard `net/http` router. PostgreSQL access uses `pgx`, the only initial runtime dependency. This keeps the dependency surface small while retaining explicit SQL and transaction control.

## Consequences

Cloud Storage signing can replace the development signer without changing the HTTP or business layers. Firebase identity can later supply an owner ID without changing anonymous-transfer behavior. The development upload URL is intentionally not a functioning byte-transfer path; real upload/download testing begins with the Cloud Storage integration.

