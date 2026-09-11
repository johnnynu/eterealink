# ADR 0014: Private Cloud SQL networking

## Status

Accepted and implemented in Phase 9.

## Context

Phase 8 deployed the application with Cloud Run's managed Cloud SQL integration over a transitional public database address. The instance had no authorized external networks, but the target architecture requires database traffic to stay on private addresses and expose an explicit network path without paying for an always-on connector.

## Decision

- Create a custom-mode `eterealink` VPC with a `10.0.1.0/24` regional subnet in `us-west1`.
- Reserve `10.10.0.0/24` as a global `VPC_PEERING` allocation and connect it to `servicenetworking.googleapis.com` through Private Services Access.
- Assign Cloud SQL a private address from the managed-services allocation.
- Attach the API service and migration job directly to the regional subnet with Cloud Run Direct VPC egress.
- Route only private ranges through the VPC. Public destinations retain Cloud Run's standard egress path, so this phase does not require Cloud NAT.
- Connect PostgreSQL directly over TCP port 5432. Store the private connection URL in a new immutable Secret Manager version and remove the Cloud SQL Auth Proxy volume from both Cloud Run workloads.
- Prove the path with a successful migration execution and API database readiness before removing the Cloud SQL public address.
- Retain the runtime's Cloud SQL Client IAM role until Phase 11 reviews all production permissions together.

## Consequences

PostgreSQL no longer has a public IP address. Its application traffic crosses the project's subnet and the Private Services Access peering, while browser file bytes still travel directly to Cloud Storage through signed URLs. Direct VPC egress consumes subnet addresses while revisions start and stop, so the application subnet is deliberately larger than the minimum `/26`.

The private database URL contains a stable IP assigned to this Cloud SQL instance. Replacing the instance requires publishing a new secret version and deploying both database consumers. Phase 10 will express the network, peering, instance configuration, secret wiring, and Cloud Run attachments in Terraform.
