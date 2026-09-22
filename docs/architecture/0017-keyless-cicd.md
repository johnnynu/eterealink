# ADR 0017: Keyless GitHub CI/CD

## Status

Accepted and implemented in Phase 12.

## Context

Phase 10 made Terraform authoritative for the production platform, and Phase 11 separated and narrowed the runtime identities. Releases still required a developer workstation to build images, apply Terraform, run migrations, and verify production. Phase 12 needs repeatable push-to-deploy automation without storing a Google Cloud service-account key in GitHub or bypassing Terraform for Cloud Run changes.

## Decision

- Run reusable GitHub Actions CI for every pull request and again as a required part of a production deployment. CI checks Go formatting and tests, frontend lint/tests/builds, shell syntax, and both Terraform roots.
- Use GitHub's OIDC token with Google Cloud Workload Identity Federation. The provider admits only the immutable GitHub repository ID `1352078479` on `refs/heads/main`, and that principal may impersonate one dedicated deployment service account.
- Keep the federation pool, provider, deployment account, project grants, and state-bucket access in the separately bootstrapped Terraform root. This avoids a circular dependency in which the production root would need its own missing credentials to create its deployer.
- Build API and frontend images in parallel and tag both with the full Git commit SHA. Reuse an existing tag on a safe workflow rerun because Artifact Registry rejects tag replacement.
- Authenticate Docker with a short-lived access token. Do not create or store a service-account key.
- Use repository variables only for the Firebase browser configuration compiled into the frontend. These values are public client configuration, not credentials that authorize backend access.
- Recover the existing database password from Secret Manager at deploy time, mask it immediately, and pass it to Terraform only through the existing ephemeral write-only input.
- Make the production job create and display a saved Terraform plan, apply exactly that plan, execute the migration job, and run the cumulative Phase 12 verifier.
- Serialize production workflows and attach the apply job to GitHub's `production` environment so repository protection rules can gate the state-changing step.

## Consequences

Pull requests receive deterministic application and infrastructure feedback without Google Cloud credentials. A main-branch commit cannot reach production until the same checks pass, both immutable images exist, and any configured production-environment rules allow the apply job to proceed.

The deployment identity necessarily has broad control over the resources Terraform owns. Its short-lived credentials are limited to one immutable repository and the main branch, and the verifier rejects user-managed keys. Changes to the bootstrap trust remain a deliberate local administrator operation recorded in separate Terraform state.

The frontend build requires five GitHub repository variables. A missing value stops the build before an image is published. The first workflow run also depends on applying the updated bootstrap root once.
