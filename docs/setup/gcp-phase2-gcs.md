# Phase 2 Google Cloud Storage setup

This guide records the manually bootstrapped resources used for Eterealink's direct file-transfer phase. Terraform will adopt or replace this configuration in the infrastructure phase.

## Resource baseline

| Resource | Value |
|---|---|
| Project | `eterealink` |
| Region | `us-west1` |
| Bucket | `gs://eterealink-files` |
| Service account | `eterealink-api@eterealink.iam.gserviceaccount.com` |
| Storage class | `STANDARD` |
| Bucket access | Uniform bucket-level access |
| Public access prevention | Enforced |
| Soft delete | Disabled for temporary-transfer cost control |

The service account has `roles/storage.objectUser` on this bucket only. Signed URLs use the IAM Credentials API and `iam.serviceAccounts.signBlob`; no downloadable service-account key is created.

## Local authentication

Install and initialize the Google Cloud CLI, then create Application Default Credentials by impersonating the application service account:

```bash
gcloud config set project eterealink
gcloud auth application-default login \
  --impersonate-service-account=eterealink-api@eterealink.iam.gserviceaccount.com
```

Your Google identity must have `roles/iam.serviceAccountTokenCreator` on the application service account. The application service account also has that role on itself so the same signing implementation works when it is attached to Cloud Run.

Do not create or download a JSON service-account key.

## Backend configuration

Export these values in the shell that runs the API:

```bash
export STORAGE_BACKEND=gcs
export GCS_BUCKET=eterealink-files
export GCS_SIGNING_SERVICE_ACCOUNT=eterealink-api@eterealink.iam.gserviceaccount.com
```

The remaining values are documented in the repository's `.env.example`.

## Browser CORS policy

The checked-in [`config/gcs-cors.json`](../../config/gcs-cors.json) allows direct resumable `POST`/`PUT` uploads plus `GET` and `HEAD` requests from:

- `http://localhost:3000`
- `http://127.0.0.1:3000`

Apply changes with:

```bash
gcloud storage buckets update gs://eterealink-files \
  --cors-file=config/gcs-cors.json
```

Add the deployed frontend's exact HTTPS origin before production use. Do not replace the restricted origins with `*`.

## Verification

Run unit tests:

```bash
cd backend
go test ./...
```

Run the opt-in live test, which uploads, reads, downloads, compares, and removes one temporary object:

```bash
GCS_INTEGRATION_TEST=1 \
GCS_BUCKET=eterealink-files \
GCS_SIGNING_SERVICE_ACCOUNT=eterealink-api@eterealink.iam.gserviceaccount.com \
go test ./internal/storage -run '^TestGCSBackendIntegration$' -v -count=1
```

## Reproduction commands

The current baseline was created with the following commands. Replace `DEVELOPER_EMAIL` when reproducing it in another project.

```bash
gcloud services enable \
  storage.googleapis.com \
  iamcredentials.googleapis.com \
  --project=eterealink

gcloud iam service-accounts create eterealink-api \
  --project=eterealink \
  --display-name='Eterealink API' \
  --description='Runtime identity for the Eterealink Go API'

gcloud storage buckets create gs://eterealink-files \
  --project=eterealink \
  --location=us-west1 \
  --default-storage-class=STANDARD \
  --uniform-bucket-level-access \
  --public-access-prevention \
  --soft-delete-duration=0

gcloud storage buckets add-iam-policy-binding gs://eterealink-files \
  --member='serviceAccount:eterealink-api@eterealink.iam.gserviceaccount.com' \
  --role='roles/storage.objectUser'

gcloud iam service-accounts add-iam-policy-binding \
  eterealink-api@eterealink.iam.gserviceaccount.com \
  --project=eterealink \
  --member='user:DEVELOPER_EMAIL' \
  --role='roles/iam.serviceAccountTokenCreator'

gcloud iam service-accounts add-iam-policy-binding \
  eterealink-api@eterealink.iam.gserviceaccount.com \
  --project=eterealink \
  --member='serviceAccount:eterealink-api@eterealink.iam.gserviceaccount.com' \
  --role='roles/iam.serviceAccountTokenCreator'

gcloud storage buckets update gs://eterealink-files \
  --cors-file=config/gcs-cors.json
```
