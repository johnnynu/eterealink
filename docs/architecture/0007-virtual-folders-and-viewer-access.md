# ADR 0007: Virtual folders and inherited viewer access

## Status

Accepted for Phase 5.

## Context

Persistent files already belong to an internal user and use opaque Cloud Storage keys. Phase 5 needs organization and authenticated collaboration without treating object-name prefixes as directories or weakening owner authorization. Folder access must extend to descendants, while a viewer must never gain mutation rights or visibility into the owner's account-wide storage usage.

## Decision

- Keep folders entirely in PostgreSQL. Moving a file changes `files.folder_id` and never copies its Cloud Storage object.
- Represent ownership with `folders.owner_id`. Store only `VIEWER` entries in `folder_members`; `OWNER` is derived rather than duplicated.
- Allow nested folders and enforce case-insensitive sibling-name uniqueness. Reject self-parenting, descendant cycles, cross-owner parents, and cross-owner file moves.
- Grant a viewer read-only access to the shared folder and all descendants. A direct membership on a nested folder does not expose its ancestors above the shared root.
- Let viewers browse folder metadata and request signed downloads. Keep create, rename, move, delete, link, upload, and membership operations owner-only.
- Add viewers by normalized email only when that email already maps to an Eterealink user. Do not create invitation or placeholder identities in this phase.
- Delete folders only when they contain no child folders or files. This prevents a folder operation from implicitly deleting or relocating user content.
- Support bulk file moves in one transaction. Bulk file deletion continues using the existing object-first deletion path for each selected file so database rows never disappear before their storage objects.
- Reserve persistent upload capacity atomically by locking the owner row and counting both pending and ready files before metadata is inserted. Default the account quota to 25 GiB and keep it configurable independently from the per-file limit.
- Continue using opaque file IDs in storage keys, so duplicate filenames are safe and do not overwrite objects.
- Execute folder-scoped filename search, active-link filtering, sorting, and pagination in PostgreSQL. Use opaque keyset cursors tied to the selected sort order so large libraries do not require loading or offset-scanning every file.

## Consequences

Owners can organize persistent files without storage rewrites and share a subtree with another authenticated user. Viewers inherit downloads through the shared hierarchy but cannot mutate it. Folder removal is intentionally explicit and conservative, and unfinished uploads consume quota until their metadata is deleted or lifecycle cleanup is introduced.
