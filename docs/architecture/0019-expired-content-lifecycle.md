# ADR 0019: Expired anonymous content lifecycle

## Status

Accepted and implemented in Phase 14.

## Context

Anonymous file and multi-file transfer links stop resolving after 24 hours, but expiration checks alone leave finalized objects, abandoned uploads, generated ZIP archives, and their database rows behind. Cleanup must remove both storage and metadata, tolerate retries, avoid touching persistent account files, and stay inexpensive when the application is idle.

## Decision

- Run a dedicated Cloud Run job every hour through an authenticated Cloud Scheduler trigger. Keep cleanup out of API instances so scale-to-zero and request traffic do not determine whether it runs.
- Select only expired standalone anonymous files and expired anonymous transfers. Persistent files are excluded by ownership and transfer relationships.
- Delete every recorded Cloud Storage object before deleting its root metadata row. PostgreSQL foreign keys then cascade to the anonymous share link and, for a transfer, its file rows.
- Treat a missing object as a successful retry. Preserve metadata when any other object deletion fails so the next job execution retains the authoritative list of work.
- Process bounded batches and continue until a partial batch is reached. Cloud Run retries failed tasks, and the hourly trigger provides another recovery path.
- Give the cleanup identity only database-secret access, conditional bucket-level `storage.objects.delete` for the `anonymous/` namespace, and permission to invoke its own job. It receives no object read/create or signed-URL permissions. The worker also rejects keys outside that namespace before calling storage.
- If archive bytes are written but the worker can no longer commit the archive as ready, delete those bytes immediately. This closes the race where a transfer expires while its ZIP is being built.

## Consequences

Anonymous objects and rows disappear no more than roughly an hour after expiration during normal operation. Cleanup is idempotent across Cloud Run task retries, Scheduler retries, and overlapping manual executions. A storage failure leaves metadata available for diagnosis and retry instead of losing the key needed to remove the object.

The job does not delete authenticated users' persistent files, expired persistent share links, or accepted folder membership. Those records follow their separate product lifecycle.
