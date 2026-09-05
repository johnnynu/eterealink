#!/usr/bin/env bash

set -euo pipefail

PROJECT_ID="${PROJECT_ID:-eterealink}"
REGION="${REGION:-us-west1}"
REPOSITORY="${REPOSITORY:-eterealink}"
SERVICE="${SERVICE:-eterealink-api}"
MIGRATION_JOB="${MIGRATION_JOB:-eterealink-migrate}"
DB_INSTANCE="${DB_INSTANCE:-eterealink-db}"
DB_NAME="${DB_NAME:-eterealink}"
DB_USER="${DB_USER:-eterealink}"
DATABASE_SECRET="${DATABASE_SECRET:-eterealink-database-url}"
RUNTIME_SERVICE_ACCOUNT="${RUNTIME_SERVICE_ACCOUNT:-eterealink-api@${PROJECT_ID}.iam.gserviceaccount.com}"
GCS_BUCKET="${GCS_BUCKET:-eterealink-files}"
FIREBASE_PROJECT_ID="${FIREBASE_PROJECT_ID:-${PROJECT_ID}}"

for command in gcloud git openssl; do
	if ! command -v "${command}" >/dev/null 2>&1; then
		echo "required command is unavailable: ${command}" >&2
		exit 1
	fi
done

active_account="$(gcloud auth list --filter=status:ACTIVE --format='value(account)' | head -n 1)"
if [[ -z "${active_account}" ]]; then
	echo "no active gcloud account; run: gcloud auth login" >&2
	exit 1
fi

if [[ "$(gcloud config get-value project 2>/dev/null)" != "${PROJECT_ID}" ]]; then
	echo "active gcloud project must be ${PROJECT_ID}" >&2
	exit 1
fi

image_tag="$(git rev-parse --short=12 HEAD)"
image_uri="${REGION}-docker.pkg.dev/${PROJECT_ID}/${REPOSITORY}/api:${image_tag}"

if ! git diff --quiet || ! git diff --cached --quiet || [[ -n "$(git ls-files --others --exclude-standard)" ]]; then
	echo "commit the deployment inputs before publishing an immutable image" >&2
	exit 1
fi

echo "Enabling Phase 8 Google Cloud APIs..."
gcloud services enable \
	artifactregistry.googleapis.com \
	cloudbuild.googleapis.com \
	run.googleapis.com \
	secretmanager.googleapis.com \
	sqladmin.googleapis.com \
	--project="${PROJECT_ID}" \
	--quiet

if ! gcloud artifacts repositories describe "${REPOSITORY}" \
	--project="${PROJECT_ID}" \
	--location="${REGION}" >/dev/null 2>&1; then
	echo "Creating Artifact Registry repository ${REPOSITORY}..."
	gcloud artifacts repositories create "${REPOSITORY}" \
		--project="${PROJECT_ID}" \
		--location="${REGION}" \
		--repository-format=docker \
		--description="Eterealink production container images" \
		--immutable-tags \
		--quiet
fi

if gcloud artifacts docker images describe "${image_uri}" \
	--project="${PROJECT_ID}" >/dev/null 2>&1; then
	echo "Reusing existing immutable API image ${image_uri}."
else
	echo "Building immutable API image ${image_uri}..."
	gcloud builds submit backend \
		--project="${PROJECT_ID}" \
		--region="${REGION}" \
		--tag="${image_uri}" \
		--quiet
fi

if ! gcloud sql instances describe "${DB_INSTANCE}" --project="${PROJECT_ID}" >/dev/null 2>&1; then
	echo "Creating the transitional Phase 8 Cloud SQL instance ${DB_INSTANCE}..."
	gcloud sql instances create "${DB_INSTANCE}" \
		--project="${PROJECT_ID}" \
		--region="${REGION}" \
		--database-version=POSTGRES_17 \
		--edition=enterprise \
		--tier=db-f1-micro \
		--availability-type=zonal \
		--storage-type=HDD \
		--storage-size=10 \
		--storage-auto-increase \
		--storage-auto-increase-limit=20 \
		--assign-ip \
		--backup \
		--quiet
fi

if ! gcloud sql databases describe "${DB_NAME}" \
	--instance="${DB_INSTANCE}" \
	--project="${PROJECT_ID}" >/dev/null 2>&1; then
	gcloud sql databases create "${DB_NAME}" \
		--instance="${DB_INSTANCE}" \
		--project="${PROJECT_ID}" \
		--quiet
fi

connection_name="$(gcloud sql instances describe "${DB_INSTANCE}" \
	--project="${PROJECT_ID}" \
	--format='value(connectionName)')"

if ! gcloud secrets describe "${DATABASE_SECRET}" --project="${PROJECT_ID}" >/dev/null 2>&1; then
	echo "Creating the application database user and Secret Manager value..."
	db_password="$(openssl rand -hex 24)"
	if gcloud sql users list \
		--instance="${DB_INSTANCE}" \
		--project="${PROJECT_ID}" \
		--filter="name=${DB_USER}" \
		--format='value(name)' | grep -qx "${DB_USER}"; then
		gcloud sql users set-password "${DB_USER}" \
			--instance="${DB_INSTANCE}" \
			--project="${PROJECT_ID}" \
			--password="${db_password}" \
			--quiet
	else
		gcloud sql users create "${DB_USER}" \
			--instance="${DB_INSTANCE}" \
			--project="${PROJECT_ID}" \
			--password="${db_password}" \
			--quiet
	fi
	database_url="postgres://${DB_USER}:${db_password}@/${DB_NAME}?host=/cloudsql/${connection_name}&sslmode=disable"
	printf '%s' "${database_url}" | gcloud secrets create "${DATABASE_SECRET}" \
		--project="${PROJECT_ID}" \
		--replication-policy=automatic \
		--data-file=- \
		--quiet
	unset database_url db_password
fi

gcloud projects add-iam-policy-binding "${PROJECT_ID}" \
	--member="serviceAccount:${RUNTIME_SERVICE_ACCOUNT}" \
	--role=roles/cloudsql.client \
	--condition=None \
	--quiet >/dev/null

gcloud secrets add-iam-policy-binding "${DATABASE_SECRET}" \
	--project="${PROJECT_ID}" \
	--member="serviceAccount:${RUNTIME_SERVICE_ACCOUNT}" \
	--role=roles/secretmanager.secretAccessor \
	--condition=None \
	--quiet >/dev/null

echo "Deploying and executing database migrations..."
gcloud run jobs deploy "${MIGRATION_JOB}" \
	--project="${PROJECT_ID}" \
	--region="${REGION}" \
	--image="${image_uri}" \
	--command=/app/migrate \
	--args=up \
	--service-account="${RUNTIME_SERVICE_ACCOUNT}" \
	--set-cloudsql-instances="${connection_name}" \
	--set-secrets="DATABASE_URL=${DATABASE_SECRET}:latest" \
	--set-env-vars=APP_ENV=production \
	--max-retries=1 \
	--task-timeout=10m \
	--quiet

gcloud run jobs execute "${MIGRATION_JOB}" \
	--project="${PROJECT_ID}" \
	--region="${REGION}" \
	--wait \
	--quiet

echo "Deploying public Cloud Run API service ${SERVICE}..."
gcloud run deploy "${SERVICE}" \
	--project="${PROJECT_ID}" \
	--region="${REGION}" \
	--image="${image_uri}" \
	--service-account="${RUNTIME_SERVICE_ACCOUNT}" \
	--set-cloudsql-instances="${connection_name}" \
	--set-secrets="DATABASE_URL=${DATABASE_SECRET}:latest" \
	--set-env-vars="APP_ENV=production,HTTP_ADDR=:8080,STORAGE_BACKEND=gcs,GCS_BUCKET=${GCS_BUCKET},GCS_SIGNING_SERVICE_ACCOUNT=${RUNTIME_SERVICE_ACCOUNT},FIREBASE_PROJECT_ID=${FIREBASE_PROJECT_ID},ANONYMOUS_FILE_TTL=24h,SIGNED_URL_TTL=15m,MAX_ANONYMOUS_FILE_BYTES=1073741824,MAX_PERSISTENT_FILE_BYTES=5368709120,MAX_PERSISTENT_STORAGE_BYTES=26843545600,MAX_ANONYMOUS_TRANSFER_BYTES=1073741824,MAX_ANONYMOUS_FILES=10" \
	--port=8080 \
	--cpu=1 \
	--memory=512Mi \
	--concurrency=40 \
	--min=0 \
	--max=3 \
	--timeout=300s \
	--startup-probe="httpGet.path=/readyz,httpGet.port=8080,timeoutSeconds=3,periodSeconds=5,failureThreshold=12" \
	--liveness-probe="httpGet.path=/healthz,httpGet.port=8080,timeoutSeconds=3,periodSeconds=10,failureThreshold=3" \
	--allow-unauthenticated \
	--quiet

service_url="$(gcloud run services describe "${SERVICE}" \
	--project="${PROJECT_ID}" \
	--region="${REGION}" \
	--format='value(status.url)')"

echo "Verifying ${service_url}..."
curl --fail --silent --show-error "${service_url}/healthz"
echo
curl --fail --silent --show-error "${service_url}/readyz"
echo
echo "Phase 8 deployment is ready: ${service_url}"
