# ADR 0004: Model anonymous shares as transfer bundles

## Status

Accepted as a Phase 3 extension.

## Context

Anonymous senders expect to select several files and share one link. Requiring recipients to download every object separately is cumbersome, but building a large ZIP inside an HTTP request would tie request duration and memory use to transfer size. Raising the anonymous limit to 1 GiB also makes one-shot browser uploads fragile on interrupted networks.

## Decision

- Model each anonymous share as a transfer containing one to ten files.
- Enforce a 1 GiB combined transfer limit in both the browser and API. Keep the per-file limit independently configurable and no larger than the configured default of 1 GiB.
- Initiate a separate GCS resumable-upload session for every file. Upload at most three files concurrently and send 8 MiB aligned chunks.
- Mark the transfer ready only after the API verifies every object against its declared size and media type.
- Queue ZIP work through transfer state in PostgreSQL. Stream source objects into a store-only ZIP and stream the result directly into the existing GCS bucket, avoiding local disk and whole-file memory buffering.
- Run the archive worker inside the API process for local development. Preserve the worker boundary so deployment can execute the same work as a Cloud Run Job without adding a second bucket, Cloud Tasks, or Pub/Sub in the first iteration.
- Always return signed individual-file downloads. The share page polls while the archive is pending and enables “Download all as ZIP” when it becomes ready.

## Consequences

A share link becomes usable as soon as every source file is verified; ZIP preparation does not block individual downloads. ZIP output can be slightly larger than the combined source size because of archive metadata. Store-only ZIP entries avoid CPU-heavy recompression and work well for already-compressed media.

The signed URL's 15-minute lifetime limits only how long the browser has to initiate a resumable session. Once initiated, GCS owns the session URL and the upload can continue beyond that signing window. Download targets remain capped by both their short TTL and the transfer's 24-hour expiration.

The database currently provides the archive queue. A deployed worker will need retry/lease hardening and operational metrics before the system is considered production-grade at sustained traffic.
