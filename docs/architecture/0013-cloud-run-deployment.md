# ADR 0013: Phase 8 Cloud Run deployment

## Status

Accepted and implemented in Phase 8.

## Context

Phase 7 produced deployable API, migration, and frontend artifacts in non-root container images. The API requires PostgreSQL during startup, while the private database network is deliberately scheduled for Phase 9. Phase 8 needs a complete public application rather than an API that is only usable through a local frontend.

## Decision

- Store immutable, commit-addressed API images in a regional Artifact Registry Docker repository.
- Build a separate immutable Next.js image with the public Firebase configuration and deployed API proxy target, then deploy it as the public `eterealink-web` Cloud Run service.
- Use a dedicated frontend runtime service account with no project roles. The web container only proxies HTTPS requests to the public API and does not call Google Cloud APIs directly.
- Run schema migrations from the same image as a one-shot Cloud Run Job before deploying the service revision.
- Attach the existing `eterealink-api` service account to both the migration job and API service. Continue using keyless IAM signing for Cloud Storage.
- Use the smallest zonal Cloud SQL PostgreSQL instance as a transitional Phase 8 database. Cloud Run connects through the managed Cloud SQL integration and a Unix socket. The instance has a public IP so the connector can reach it, but it has no authorized external networks.
- Store the database connection string in Secret Manager and expose it only to the runtime service account.
- Allow unauthenticated invocation because anonymous transfers and public share resolution are product requirements. Firebase bearer tokens continue to protect account endpoints.
- Scale the API to zero, cap it at three instances, and configure database-aware startup plus process liveness probes. Use `/health` for the public and Cloud Run liveness path because Google reserves `/healthz` at its frontend; retain `/healthz` for local compatibility.
- Set the Cloud Run request timeout and Go server write timeout to five minutes so the existing four-minute-thirty-second SSE rotation can finish normally.
- Authorize the deterministic frontend hostname in Firebase Authentication and add its exact origin to the Cloud Storage bucket CORS policy.

## Consequences

The full browser application is publicly deployable, and the database schema is upgraded before a new API revision serves traffic. Artifact Registry, Cloud Run, Cloud SQL, Secret Manager, Firebase authorization, bucket CORS, and IAM bindings are still manually bootstrapped in this phase; Phase 10 will replace the infrastructure portions with Terraform.

The transitional Cloud SQL public address is not the target network architecture. Phase 9 will create the custom VPC, Private Services Access allocation, private database address, and Direct VPC egress path, then remove the public database address after the private path is verified.
