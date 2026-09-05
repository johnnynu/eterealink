# Phase 8 Cloud Run deployment

Phase 8 publishes the Go API image to Artifact Registry, applies migrations with a Cloud Run Job, and deploys the public API service. The checked-in deployment script also creates the smallest database bridge that lets the service work before Phase 9 adds private networking.

## Resources

| Resource | Default |
|---|---|
| Project and region | `eterealink`, `us-west1` |
| Artifact Registry repository | `eterealink` |
| API image | `api:<12-character-git-commit>` |
| Cloud Run service | `eterealink-api` |
| Migration job | `eterealink-migrate` |
| Cloud SQL instance | `eterealink-db` (`db-f1-micro`, zonal, PostgreSQL 17) |
| Database secret | `eterealink-database-url` |
| Runtime identity | `eterealink-api@eterealink.iam.gserviceaccount.com` |

The Cloud SQL instance has a public address but no authorized networks. The API and migration job reach it through Cloud Run's managed Cloud SQL integration. Phase 9 replaces this transitional path with private IP and Direct VPC egress.

## Deploy

The active `gcloud` account must have access to the project, billing must be enabled, and the CLI project must already be `eterealink`. The script does not create or download service-account keys.

```bash
gcloud auth login
gcloud config set project eterealink
make phase8-deploy
```

Run from a clean, committed worktree so the immutable image tag describes its source exactly. The script reuses an image already published for that commit, preserves an existing database secret, updates the migration job, waits for migrations to succeed, deploys the API revision, and checks both health endpoints.

Defaults can be overridden with environment variables such as `PROJECT_ID`, `REGION`, `SERVICE`, `DB_INSTANCE`, and `GCS_BUCKET`.

## Verify

```bash
make phase8-verify
```

Expected endpoint responses:

```json
{"status":"ok"}
{"status":"ready"}
```

Inspect the deployed image digest and latest revision when troubleshooting:

```bash
gcloud run services describe eterealink-api \
  --project=eterealink \
  --region=us-west1

gcloud run jobs executions list \
  --job=eterealink-migrate \
  --project=eterealink \
  --region=us-west1
```

Cloud Run assigns the API an HTTPS `run.app` URL. Phase 8 deploys only the API; the frontend stays local until its hosting path is selected in a later phase.
