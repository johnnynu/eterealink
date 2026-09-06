# ADR 0006: Owner-scope persistent file operations

## Status

Accepted for the pre-Phase 5 persistent-file slice.

## Context

Firebase authentication provisions an internal PostgreSQL user, but the anonymous upload routes create expiring files without an owner. Folders and managed sharing require durable files that belong to that internal user. Hiding another user's files in the browser is not an authorization boundary; every protected operation must enforce ownership in the API and database query.

## Decision

- Add protected create, complete, list, download, delete, share-link creation, and share-link revocation routes under `/v1/files`.
- Derive the owner exclusively from the verified Firebase request context. Never accept an owner ID from request JSON or query parameters.
- Store persistent objects under `users/{ownerID}/files/{fileID}` and create file rows with `owner_id` set and `expires_at` null.
- Constrain persistent files by the uploader's remaining account quota, without a separate per-file product cap. Keep anonymous transfers at their separate 1 GiB combined limit.
- Scope completion, listing, download, and deletion queries by both file ID and owner ID. A file owned by someone else resolves as not found.
- Keep new persistent files private by default. Creating a persistent upload does not automatically create a share link.
- Allow an owner to create one active public link per ready file with a 24-hour, 7-day, 30-day, or no-expiration lifetime. Serialize link creation on the file row so concurrent requests cannot create multiple active links.
- Return the active link with the owner's file listing, and let only that owner revoke it. Reuse the existing `/s/{code}` recipient flow and its short-lived signed downloads.
- Return an owner-scoped aggregate count and byte total alongside the file listing so the workspace can display authoritative storage usage without deriving it from the visible page.
- Continue verifying stored size and media type before marking an upload ready.
- Delete the private Cloud Storage object before its metadata. Treat an already-missing object as recoverable so a retried delete can still remove the metadata.
- Preserve the anonymous transfer routes as a separate 24-hour workflow.

## Consequences

Signed-in users can maintain a durable private library and selectively expose individual files without changing anonymous sharing behavior. Phase 5 folders can attach directly to these owner-linked file rows and add folder-level membership and sharing without redesigning per-file links.
