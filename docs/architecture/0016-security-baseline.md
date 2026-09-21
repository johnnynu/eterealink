# ADR 0016: Production security baseline

## Status

Accepted and implemented in Phase 11.

## Context

The production platform already uses Firebase identity, a private Cloud SQL address, a private Cloud Storage bucket, Secret Manager, and short-lived signed URLs. The Phase 10 Terraform adoption exposed two remaining privilege gaps: the migration job reused the API identity, and the API retained Cloud SQL Client plus broad signing and object roles from the earlier connector-based deployment. Anonymous upload creation also had size limits but no request-rate control.

## Decision

- Give the API, frontend, and migration job separate user-managed service accounts. The migration identity receives only access to the database URL secret and private network connectivity.
- Remove Cloud SQL Client after the completed Direct VPC/private-IP migration. Direct PostgreSQL does not call the Cloud SQL Admin API.
- Replace Storage Object User with a custom bucket-scoped role containing only `storage.objects.create`, `storage.objects.get`, and `storage.objects.delete`.
- Replace Service Account Token Creator with a custom self-grant containing only `iam.serviceAccounts.signBlob`. Aurea Link does not need token or JWT impersonation.
- Pin Cloud Run secret references to the Terraform-managed numeric Secret Manager version instead of the mutable `latest` alias.
- Retain a 15-minute ceiling for signed URLs, a 24-hour ceiling for anonymous content, a 1 GiB anonymous transfer ceiling, and a 10-file ceiling. Startup rejects configuration that raises those limits.
- Limit anonymous upload and transfer creation to six requests per client per minute, with a second per-instance ceiling of sixty requests per minute. Return `429` and `Retry-After` when either limit is exhausted. Client addresses are stored only in memory and logged as short hashes.
- Add `no-store` and MIME-sniffing protections to API responses. Add browser framing, referrer, feature, MIME, transport, and base/object restrictions to frontend responses.

## Consequences

The API can still perform every object operation it needs and sign URLs without the ability to mint access tokens. A compromised migration container cannot access file objects or sign URLs, and a compromised frontend identity has no Google Cloud data permissions.

The application rate limiter is deliberately local to each Cloud Run instance so it adds no always-on service or database writes. Cloud Run is capped at three instances, so the global ceiling is bounded but not cluster-exact, and IP-based limits remain abuse friction rather than a substitute for an edge security product. If traffic or abuse justifies the fixed cost later, an external Application Load Balancer and Cloud Armor can provide a shared edge-enforced policy.

HSTS is emitted by the frontend on every response but browsers honor it only over HTTPS. The partial content security policy restricts framing, base URLs, and plugin content without blocking the existing Firebase, Cloud Storage, media-preview, or Next.js runtime paths.
