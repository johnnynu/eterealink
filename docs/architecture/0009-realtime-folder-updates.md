# ADR 0009: Realtime folder updates after Phase 5.1

## Status

Proposed for a post-preview Phase 6.5 collaboration milestone. Realtime delivery is intentionally outside Phase 5.1.

## Context

Phase 5.1 establishes folder membership, contributor permissions, invite links, uploader ownership, and attribution. A user viewing a shared folder does not yet receive changes made by another user until the folder is refreshed or reopened. Production realtime support must work when Cloud Run scales the Go API across multiple instances and must not make transient event delivery another source of truth.

## Decision

- Use authenticated Server-Sent Events because folder updates are primarily server-to-browser notifications. Continue using ordinary API requests and signed Cloud Storage URLs for mutations and file bytes.
- Add an endpoint such as `GET /v1/folders/{id}/events`. Authorize the initial connection against inherited folder membership, send heartbeat frames, and reconnect clients with exponential backoff.
- Send small invalidation events such as `folder.changed`, never complete file metadata. On receipt or reconnection, the browser refetches authoritative folder contents and preserves its local search, selection, pagination, and panel state.
- Initially use PostgreSQL `LISTEN`/`NOTIFY` through Cloud SQL. Emit notifications within the same transactions that complete, move, remove, or delete files and that change membership. Notifications are delivered only after commit, and every listening API session can receive them.
- Maintain one database listener per Cloud Run instance and fan out authorized events only to that instance's connected browsers. Do not dedicate one PostgreSQL connection per browser.
- Rotate streaming requests before Cloud Run's request timeout and have clients reconnect automatically. A reconnect always begins with a normal folder refetch so a missed notification cannot leave the UI permanently stale.
- If connection volume or notification traffic outgrows Cloud SQL signaling, introduce Memorystore for Redis Pub/Sub as the cross-instance event bus. Each Cloud Run instance subscribes and broadcasts locally.
- If events later drive durable workflows, audit history, notifications, or analytics, add a transactional PostgreSQL outbox and publish it to Google Cloud Pub/Sub. Redis/SSE remains the low-latency UI path; the outbox and Pub/Sub become the reliable processing path.

## Consequences

The first production implementation can reuse Cloud Run, Cloud SQL, Firebase Authentication, and the planned private VPC path without adding an always-on service. Realtime events remain hints, while PostgreSQL stays authoritative. Long-lived streams consume Cloud Run capacity and must tolerate forced reconnection.

Memorystore adds cost and operational surface, so it is deferred until observed scale justifies it. Pub/Sub is not required merely to refresh open folders because UI clients recover by refetching.

## References

- [Cloud Run WebSocket and cross-instance synchronization guidance](https://docs.cloud.google.com/run/docs/triggering/websockets)
- [Cloud Run request timeout configuration](https://docs.cloud.google.com/run/docs/configuring/request-timeout)
- [PostgreSQL `LISTEN`](https://www.postgresql.org/docs/current/sql-listen.html) and [`NOTIFY`](https://www.postgresql.org/docs/current/sql-notify.html)
