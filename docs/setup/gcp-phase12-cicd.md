# Phase 12 GitHub CI/CD

Phase 12 runs application and Terraform checks on pull requests, then turns each commit pushed to `main` into immutable API and frontend images and a Terraform-managed production release. Google Cloud authentication uses short-lived GitHub OIDC credentials; no service-account key is created or stored.

## Bootstrap the deployment identity

The bootstrap root owns both the versioned Terraform state bucket and the identity used to access it. Apply its Phase 12 additions once with administrator credentials:

```bash
gcloud auth application-default login
gcloud auth application-default set-quota-project eterealink
terraform -chdir=infrastructure/bootstrap init
terraform -chdir=infrastructure/bootstrap plan -out=phase12-bootstrap.tfplan
terraform -chdir=infrastructure/bootstrap show phase12-bootstrap.tfplan
terraform -chdir=infrastructure/bootstrap apply phase12-bootstrap.tfplan
```

The plan creates `eterealink-deploy@eterealink.iam.gserviceaccount.com`, a GitHub workload identity pool and provider, the deployer's project roles, and access to the state bucket. It does not replace the existing bucket. The OIDC condition accepts only repository ID `1352078479` on `refs/heads/main`, so a repository rename does not break the trust and reuse of the old repository name cannot inherit it.

Confirm the two values embedded in the production workflow:

```bash
terraform -chdir=infrastructure/bootstrap output github_workload_identity_provider
terraform -chdir=infrastructure/bootstrap output github_deploy_service_account
```

## Configure public frontend values

Create these GitHub repository variables under **Settings → Secrets and variables → Actions → Variables**:

| Variable | Source |
|---|---|
| `NEXT_PUBLIC_FIREBASE_API_KEY` | `frontend/.env.local` |
| `NEXT_PUBLIC_FIREBASE_AUTH_DOMAIN` | Production Firebase auth domain, normally `aurealink.app` |
| `NEXT_PUBLIC_FIREBASE_STORAGE_BUCKET` | `frontend/.env.local` |
| `NEXT_PUBLIC_FIREBASE_MESSAGING_SENDER_ID` | `frontend/.env.local` |
| `NEXT_PUBLIC_FIREBASE_APP_ID` | `frontend/.env.local` |

These values identify the Firebase web application and are compiled into browser JavaScript. Firebase ID tokens and server-side authorization remain the security boundary.

The equivalent GitHub CLI commands are:

```bash
gh variable set NEXT_PUBLIC_FIREBASE_API_KEY --body 'VALUE'
gh variable set NEXT_PUBLIC_FIREBASE_AUTH_DOMAIN --body 'aurealink.app'
gh variable set NEXT_PUBLIC_FIREBASE_STORAGE_BUCKET --body 'VALUE'
gh variable set NEXT_PUBLIC_FIREBASE_MESSAGING_SENDER_ID --body 'VALUE'
gh variable set NEXT_PUBLIC_FIREBASE_APP_ID --body 'VALUE'
```

## Protect production

Create a GitHub environment named `production`. Add required reviewers if releases should pause for approval after images are built and the checks pass. The workflow concurrency group permits only one production release at a time and never cancels an apply already in progress.

Configure branch protection for `main` to require the three pull-request checks:

- `Backend`
- `Frontend`
- `Infrastructure`

## Deployment behavior

`.github/workflows/ci.yml` runs on pull requests and as a reusable prerequisite of `.github/workflows/deploy.yml`. A push to `main` performs this sequence:

1. Run all CI jobs.
2. Build API and frontend Linux images in parallel and push full-commit-SHA tags to Artifact Registry. A rerun reuses tags that already exist.
3. Initialize the production GCS Terraform backend and recover the existing write-only database input from Secret Manager.
4. Save and display a Terraform plan that updates both Cloud Run image references, then apply that exact plan.
5. Execute the migration job and run `make phase12-verify`.

The verifier confirms all three Cloud Run resources use the expected immutable images, the OIDC provider is repository/branch restricted, the deployment account has no user-managed key, Terraform has no remaining drift, and every Phase 11 production check still passes.

To retry a failed release, rerun the workflow. Immutable images are detected and reused. To verify a deployed commit locally, authenticate with `gcloud`, export the database password as described in the Phase 11 guide, and run:

```bash
IMAGE_TAG="$(git rev-parse HEAD)" make phase12-verify
```
