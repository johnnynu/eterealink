# ADR 0015: Terraform infrastructure ownership

## Status

Accepted and implemented in Phase 10.

## Context

Phases 2, 8, and 9 created the production platform through checked-in `gcloud` scripts. Those scripts are useful deployment records, but they do not provide a persistent plan, drift detection, dependency graph, or protected shared state. Phase 10 must adopt the live resources without replacing the private database or interrupting the public application.

## Decision

- Manage the production APIs, service accounts, IAM grants, Cloud Storage bucket and CORS policy, Artifact Registry repository, VPC, subnet, Private Services Access allocation and connection, Cloud SQL resources, database secret, Cloud Run services and migration job, Firebase authorized domains, and Cloud Run domain mappings in one root Terraform configuration.
- Keep the Terraform state in a dedicated, versioned, private Cloud Storage bucket created by a small bootstrap configuration. The application object bucket does not hold infrastructure state.
- Import the resources created in earlier phases before the first plan. Retain the Phase 8 and 9 scripts as historical and emergency deployment tooling, but use Terraform as the infrastructure authority after adoption.
- Require immutable API and frontend image tags as input. Application image builds remain outside Terraform.
- Supply the database password through an ephemeral Terraform variable and use the Google provider's write-only Cloud SQL and Secret Manager arguments. The password is not persisted in Terraform configuration, plans, or state.
- Use one monotonically increasing credential version for the Cloud SQL password and database URL secret. A version change also creates new Cloud Run revisions before traffic moves to them.
- Protect the object bucket, database instance, secret container, state bucket, services, and migration job from accidental Terraform destruction. Domain mappings and the database user are abandoned rather than deleted if they leave Terraform state.
- Continue using Direct VPC egress with `PRIVATE_RANGES_ONLY`; do not introduce a connector, Cloud NAT, or public Cloud SQL address.

## Consequences

Infrastructure changes become reviewable plans and configuration drift becomes detectable. The first adoption apply adds a new database secret version and enables Terraform-side deletion protection, so it must be reviewed before execution and followed by a migration-job run and the Phase 10 verification script.

The bootstrap state has its own small local Terraform state. Operators must protect that local bootstrap state and should rarely need to run the bootstrap configuration after the backend bucket exists. DNS records remain with the external registrar; Terraform manages the Cloud Run mappings and reports the DNS records they require.
