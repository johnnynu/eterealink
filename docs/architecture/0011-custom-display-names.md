# ADR 0011: Optional custom display names

## Status

Accepted and implemented in Phase 6.9.

## Context

Firebase supplies an identity display name, but collaborators may want a distinct Eterealink name. Identity claims are refreshed during authenticated requests, so a custom value must remain separate and must not be overwritten during provisioning.

## Decision

- Keep `users.display_name` as the latest Firebase identity name and store the optional override in `users.custom_display_name`. API responses expose the effective name as `displayName`, the provider value as `identityDisplayName`, and the override as nullable `customDisplayName`. Firebase UIDs remain private.
- Normalize submitted whitespace, reject control characters, and require 3–40 Unicode code points. A partial PostgreSQL expression index enforces case-insensitive, normalized uniqueness under concurrent updates. Clearing the override restores the provider name, then email as the final fallback.
- Use the effective name in folder owners, members, invite previews, and uploader attribution. A profile update trigger publishes invalidations for folders the user owns, can access directly, or has uploaded into.
- Let the authenticated account menu edit or remove the custom name. A successful update replaces the current user in the authentication context immediately, so the menu and dashboard update without another sign-in.

## Consequences

Google names remain non-unique and continue to refresh without overwriting a custom name. Custom-name races resolve through the database constraint, and the API returns `409 display_name_taken`. Collaborator views receive the existing Phase 6.5 invalidation hint and refetch authoritative folder data.
