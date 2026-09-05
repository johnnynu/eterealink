# ADR 0008: Expiring folder invites and contributor-owned files

## Status

Accepted for Phase 5.1.

## Context

Phase 5 allowed an owner to grant inherited, read-only folder access to an existing Eterealink user by email. Collaboration needs a lower-friction invitation flow and a way for members to add files without giving them control over content uploaded by someone else. Folder ownership and file ownership therefore need to remain separate concepts.

## Decision

- Keep `OWNER` derived from `folders.owner_id`, and allow `VIEWER` or `CONTRIBUTOR` rows in `folder_members`.
- Give both member roles inherited browse and download access to the shared folder and its descendants. Give Contributors the additional ability to upload and move only files they own within that shared tree.
- Show effective access in every folder's member panel, including grants inherited from ancestors. Label inherited members with the source folder and manage those grants at that source; Phase 5.1 does not introduce child-level deny overrides.
- Charge a contributed file to its uploader's account quota. Only that uploader may create its public link, move it, or permanently delete it.
- Let the folder owner remove another user's file from the current folder without deleting it. Removal returns the file to the uploader's private library root.
- When the owner removes a member, return all files that member placed anywhere in the shared subtree to the member's private library before revoking access.
- Create authenticated folder invite links with a random short code, role, optional expiration, and revocation state. Expiration is the deadline for accepting the invitation. Once accepted, membership persists until the folder owner removes it.
- Send invite links to a public `/join/{code}` landing page. A recipient without an account remains on that page through Google Sign-In; the existing identity-provisioning flow creates their internal user before the invite is accepted and the shared folder opens.
- Resolve an unauthenticated, privacy-limited invite preview containing only the folder name, owner display name, offered role, and joining deadline. Do not expose email addresses, internal IDs, folder contents, or member lists before authentication.
- Never downgrade an existing Contributor when they accept a Viewer invite.
- Keep direct email grants for users who already have Eterealink accounts, alongside invite links for easier sharing.

## Consequences

The shared folder may contain files owned by multiple accounts, so folder listings must not filter every row to the folder owner. All mutation queries continue to enforce `files.owner_id`, and the separate owner-only “remove from folder” operation changes only `folder_id`. This preserves recoverability and makes quota attribution predictable while allowing useful collaboration.

Invite links require the recipient to authenticate before acceptance, but they do not require a pre-existing Eterealink account. Revoking a link stops future joins but does not remove members who already accepted; those memberships remain visible and individually revocable in folder access management.
