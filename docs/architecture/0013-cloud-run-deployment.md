# ADR 0013: Phase 8 Cloud Run deployment

## Status

Accepted and implemented in Phase 8.

## Context

Phase 7 produced deployable API and migration binaries in one non-root container image. The API requires PostgreSQL during startup, while the private database network is deliberately scheduled for Phase 9. Phase 8 still needs a working public API rather than an image-only or permanently unhealthy Cloud Run revision.

## Decision

- Store immutable, commit-addressed API images in a regional Artifact Registry Docker repository.
- Run schema migrations from the same image as a one-shot Cloud Run Job before deploying the service revision.
- Attach the existing `eterealink-api` service account to both the migration job and API service. Continue using keyless IAM signing for Cloud Storage.
- Use the smallest zonal Cloud SQL PostgreSQL instance as a transitional Phase 8 database. Cloud Run connects through the managed Cloud SQL integration and a Unix socket. The instance has a public IP so the connector can reach it, but it has no authorized external networks.
- Store the database connection string in Secret Manager and expose it only to the runtime service account.
- Allow unauthenticated invocation because anonymous transfers and public share resolution are product requirements. Firebase bearer tokens continue to protect account endpoints.
- Scale the API to zero, cap it at three instances, and configure database-aware startup plus process liveness probes.
- Set the Cloud Run request timeout and Go server write timeout to five minutes so the existing four-minute-thirty-second SSE rotation can finish normally.

## Consequences

The API is publicly deployable and its database schema is upgraded before a new revision serves traffic. Artifact Registry, Cloud Run, Cloud SQL, Secret Manager, and their IAM bindings are still manually bootstrapped in this phase; Phase 10 will replace the script with Terraform.

The transitional Cloud SQL public address is not the target network architecture. Phase 9 will create the custom VPC, Private Services Access allocation, private database address, and Direct VPC egress path, then remove the public database address after the private path is verified.
