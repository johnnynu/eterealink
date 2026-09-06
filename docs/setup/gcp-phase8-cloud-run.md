# Phase 8 Cloud Run deployment

Phase 8 publishes the Go API and Next.js frontend images to Artifact Registry, applies migrations with a Cloud Run Job, and deploys the complete public application. The checked-in deployment script also creates the smallest database bridge that lets the API work before Phase 9 adds private networking.

## Resources

| Resource | Default |
|---|---|
| Project and region | `eterealink`, `us-west1` |
| Artifact Registry repository | `eterealink` |
| API image | `api:<12-character-git-commit>` |
| Frontend image | `frontend:<12-character-git-commit>` |
| Cloud Run services | `eterealink-api`, `eterealink-web` |
| Migration job | `eterealink-migrate` |
| Cloud SQL instance | `eterealink-db` (`db-f1-micro`, zonal, PostgreSQL 17) |
| Database secret | `eterealink-database-url` |
| Runtime identity | `eterealink-api@eterealink.iam.gserviceaccount.com` |
| Frontend identity | `eterealink-web@eterealink.iam.gserviceaccount.com` |
| Public domains | `eterealink.com`, `www.eterealink.com` |

The Cloud SQL instance has a public address but no authorized networks. The API and migration job reach it through Cloud Run's managed Cloud SQL integration. Phase 9 replaces this transitional path with private IP and Direct VPC egress.

## Deploy

The active `gcloud` account must have access to the project, billing must be enabled, the CLI project must already be `eterealink`, and `frontend/.env.local` must contain the Firebase web configuration. Docker Desktop is also required as a fallback when the API Cloud Build submission is unavailable. The script does not create or download service-account keys.

```bash
gcloud auth login
gcloud config set project eterealink
make phase8-deploy
```

Run from a clean, committed worktree so both immutable image tags describe their source exactly. The script reuses images already published for that commit, preserves an existing database secret, updates the migration job, waits for migrations to succeed, deploys both services, updates Firebase and bucket CORS, and checks the API directly and through the frontend proxy. It prefers Cloud Build and falls back to a local `linux/amd64` Docker build when an API Cloud Build submission is unavailable.

Both Cloud Run services use a five-minute request timeout so the frontend proxy can carry the API's four-minute-thirty-second folder event streams without truncating them.

Defaults can be overridden with environment variables such as `PROJECT_ID`, `REGION`, `SERVICE`, `DB_INSTANCE`, and `GCS_BUCKET`.

After the quota migration is deployed, bootstrap the first administrator once through Cloud SQL:

```sql
UPDATE users
SET is_admin = true
WHERE email = 'owner@example.com';
```

Do not place an administrator email in application configuration. Future quota changes use the authenticated `PATCH /v1/admin/users/{userID}/quota` endpoint. Authenticated accounts default to 25 GiB total storage; positive per-user overrides are stored in bytes, and `NULL` restores the default.

`API_IMAGE_TAG` and `FRONTEND_IMAGE_TAG` can independently reuse a previously published immutable image when a committed change affects only one container.

## Custom domain

Domain ownership is verified once through Google Search Console. The frontend has direct Cloud Run mappings for the apex and `www` hostnames:

```bash
gcloud domains verify eterealink.com
gcloud beta run domain-mappings create \
  --service=eterealink-web \
  --domain=eterealink.com \
  --region=us-west1
gcloud beta run domain-mappings create \
  --service=eterealink-web \
  --domain=www.eterealink.com \
  --region=us-west1
```

Porkbun holds the generated apex `A` and `AAAA` records and the `www` CNAME. Google provisions and renews the certificates after those records resolve. The deployment script preserves all three production origins in Cloud Storage CORS and Firebase Authentication.

## Verify

```bash
make phase8-verify
```

Expected endpoint responses:

```json
{"status":"ok"}
{"status":"ready"}
```

The public liveness endpoint is `/health`. `/healthz` remains available inside the container and in local development, but Google reserves that path at the `run.app` frontend and returns an edge 404 before it reaches the application.

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

The complete application is available at `https://eterealink.com` and `https://www.eterealink.com`, with `https://eterealink-web-300331831616.us-west1.run.app` retained as a fallback. The frontend keeps API control-plane calls same-origin through its server-side `/api` proxy; file bytes continue to move directly between the browser and Cloud Storage.
