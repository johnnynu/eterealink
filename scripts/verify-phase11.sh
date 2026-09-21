#!/usr/bin/env bash

set -euo pipefail

PROJECT_ID="${PROJECT_ID:-eterealink}"
REGION="${REGION:-us-west1}"
SERVICE="${SERVICE:-eterealink-api}"
FRONTEND_SERVICE="${FRONTEND_SERVICE:-eterealink-web}"
MIGRATION_JOB="${MIGRATION_JOB:-eterealink-migrate}"
GCS_BUCKET="${GCS_BUCKET:-eterealink-files}"
DATABASE_SECRET="${DATABASE_SECRET:-eterealink-database-url}"
FRONTEND_URL="${FRONTEND_URL:-https://aurealink.app}"
API_ACCOUNT="${API_ACCOUNT:-eterealink-api@${PROJECT_ID}.iam.gserviceaccount.com}"
FRONTEND_ACCOUNT="${FRONTEND_ACCOUNT:-eterealink-web@${PROJECT_ID}.iam.gserviceaccount.com}"
MIGRATION_ACCOUNT="${MIGRATION_ACCOUNT:-eterealink-migrate@${PROJECT_ID}.iam.gserviceaccount.com}"
SIGNER_ROLE="projects/${PROJECT_ID}/roles/aureaLinkSignedURLCreator"
OBJECT_ROLE="projects/${PROJECT_ID}/roles/aureaLinkObjectRuntime"

fail() {
	echo "$1" >&2
	exit 1
}

for command in curl gcloud jq; do
	command -v "${command}" >/dev/null 2>&1 || fail "required command is unavailable: ${command}"
done

role_json="$(gcloud iam roles describe aureaLinkSignedURLCreator --project="${PROJECT_ID}" --format=json)"
jq --exit-status '.includedPermissions == ["iam.serviceAccounts.signBlob"]' <<<"${role_json}" >/dev/null \
	|| fail "signed URL role has unexpected permissions"

role_json="$(gcloud iam roles describe aureaLinkObjectRuntime --project="${PROJECT_ID}" --format=json)"
jq --exit-status '.includedPermissions | sort == ["storage.objects.create", "storage.objects.delete", "storage.objects.get"]' <<<"${role_json}" >/dev/null \
	|| fail "object runtime role has unexpected permissions"

project_policy="$(gcloud projects get-iam-policy "${PROJECT_ID}" --format=json)"
jq --exit-status --arg api "serviceAccount:${API_ACCOUNT}" \
	'[.bindings[] | select((.members // []) | index($api))] | length == 0' <<<"${project_policy}" >/dev/null \
	|| fail "API identity still has a project-level IAM grant"

api_policy="$(gcloud iam service-accounts get-iam-policy "${API_ACCOUNT}" --project="${PROJECT_ID}" --format=json)"
jq --exit-status --arg role "${SIGNER_ROLE}" --arg api "serviceAccount:${API_ACCOUNT}" \
	'any(.bindings[]; .role == $role and ((.members // []) | index($api) != null)) and all(.bindings[]; .role != "roles/iam.serviceAccountTokenCreator" or ((.members // []) | index($api) == null))' \
	<<<"${api_policy}" >/dev/null || fail "API signing IAM is not least privilege"

bucket_policy="$(gcloud storage buckets get-iam-policy "gs://${GCS_BUCKET}" --format=json)"
jq --exit-status --arg role "${OBJECT_ROLE}" --arg api "serviceAccount:${API_ACCOUNT}" \
	'any(.bindings[]; .role == $role and ((.members // []) | index($api) != null)) and all(.bindings[]; .role != "roles/storage.objectUser" or ((.members // []) | index($api) == null))' \
	<<<"${bucket_policy}" >/dev/null || fail "bucket runtime IAM is not least privilege"

secret_policy="$(gcloud secrets get-iam-policy "${DATABASE_SECRET}" --project="${PROJECT_ID}" --format=json)"
for account in "${API_ACCOUNT}" "${MIGRATION_ACCOUNT}"; do
	jq --exit-status --arg member "serviceAccount:${account}" \
		'any(.bindings[]; .role == "roles/secretmanager.secretAccessor" and ((.members // []) | index($member) != null))' \
		<<<"${secret_policy}" >/dev/null || fail "${account} cannot access ${DATABASE_SECRET}"
done

api_json="$(gcloud run services describe "${SERVICE}" --project="${PROJECT_ID}" --region="${REGION}" --format=json)"
frontend_json="$(gcloud run services describe "${FRONTEND_SERVICE}" --project="${PROJECT_ID}" --region="${REGION}" --format=json)"
job_json="$(gcloud run jobs describe "${MIGRATION_JOB}" --project="${PROJECT_ID}" --region="${REGION}" --format=json)"

jq --exit-status --arg account "${API_ACCOUNT}" '[.. | objects | .serviceAccountName? // empty] | any(. == $account)' <<<"${api_json}" >/dev/null \
	|| fail "API service does not use ${API_ACCOUNT}"
jq --exit-status --arg account "${FRONTEND_ACCOUNT}" '[.. | objects | .serviceAccountName? // empty] | any(. == $account)' <<<"${frontend_json}" >/dev/null \
	|| fail "frontend service does not use ${FRONTEND_ACCOUNT}"
jq --exit-status --arg account "${MIGRATION_ACCOUNT}" '[.. | objects | .serviceAccountName? // empty] | any(. == $account)' <<<"${job_json}" >/dev/null \
	|| fail "migration job does not use ${MIGRATION_ACCOUNT}"

for resource_json in "${api_json}" "${job_json}"; do
	jq --exit-status --arg secret "${DATABASE_SECRET}" \
		'[.. | objects | .secretKeyRef? // empty | select((.secret // .name // "") | endswith($secret))] | length > 0 and all(.[]; (.version // .key // "") | test("^[0-9]+$"))' \
		<<<"${resource_json}" >/dev/null || fail "Cloud Run database secret is not pinned to a numeric version"
done

for setting in \
	"SIGNED_URL_TTL=15m" \
	"ANONYMOUS_FILE_TTL=24h" \
	"MAX_ANONYMOUS_FILE_BYTES=1073741824" \
	"MAX_ANONYMOUS_TRANSFER_BYTES=1073741824" \
	"MAX_ANONYMOUS_FILES=10" \
	"ANONYMOUS_UPLOAD_RATE_LIMIT=6" \
	"ANONYMOUS_UPLOAD_RATE_WINDOW=1m"; do
	name="${setting%%=*}"
	value="${setting#*=}"
	jq --exit-status --arg name "${name}" --arg value "${value}" \
		'[.. | objects | select(.name? == $name) | .value? // empty] | any(. == $value)' \
		<<<"${api_json}" >/dev/null || fail "API setting ${setting} is missing"
done

headers="$(mktemp)"
trap 'rm -f "${headers}"' EXIT
curl --fail --silent --show-error --dump-header "${headers}" --output /dev/null "${FRONTEND_URL}/health"
for expected in \
	"strict-transport-security: max-age=31536000" \
	"x-content-type-options: nosniff" \
	"x-frame-options: DENY"; do
	grep -Fqi "${expected}" "${headers}" || fail "frontend response is missing ${expected}"
done

api_url="$(jq -r '.status.url' <<<"${api_json}")"
curl --silent --show-error --dump-header "${headers}" --output /dev/null "${api_url}/v1/shares/phase11-missing"
grep -Fqi 'cache-control: no-store' "${headers}" || fail "API response is missing Cache-Control: no-store"
grep -Fqi 'x-content-type-options: nosniff' "${headers}" || fail "API response is missing X-Content-Type-Options: nosniff"

./scripts/verify-phase10.sh
echo "Phase 11 verification passed: least-privilege IAM, pinned secrets, bounded anonymous uploads, and security headers are active"
