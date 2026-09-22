#!/usr/bin/env bash

set -euo pipefail

PROJECT_ID="${PROJECT_ID:-eterealink}"
PROJECT_NUMBER="${PROJECT_NUMBER:-300331831616}"
REGION="${REGION:-us-west1}"
REPOSITORY="${REPOSITORY:-eterealink}"
SERVICE="${SERVICE:-eterealink-api}"
FRONTEND_SERVICE="${FRONTEND_SERVICE:-eterealink-web}"
MIGRATION_JOB="${MIGRATION_JOB:-eterealink-migrate}"
DEPLOY_ACCOUNT="${DEPLOY_ACCOUNT:-eterealink-deploy@${PROJECT_ID}.iam.gserviceaccount.com}"
WORKLOAD_IDENTITY_POOL="${WORKLOAD_IDENTITY_POOL:-aurealink-github}"
WORKLOAD_IDENTITY_PROVIDER="${WORKLOAD_IDENTITY_PROVIDER:-github}"
GITHUB_REPOSITORY_ID="${GITHUB_REPOSITORY_ID:-1352078479}"
IMAGE_TAG="${IMAGE_TAG:-$(git rev-parse HEAD)}"

fail() {
	echo "$1" >&2
	exit 1
}

for command in curl gcloud git jq terraform; do
	command -v "${command}" >/dev/null 2>&1 || fail "required command is unavailable: ${command}"
done

api_image="${REGION}-docker.pkg.dev/${PROJECT_ID}/${REPOSITORY}/api:${IMAGE_TAG}"
frontend_image="${REGION}-docker.pkg.dev/${PROJECT_ID}/${REPOSITORY}/frontend:${IMAGE_TAG}"

api_json="$(gcloud run services describe "${SERVICE}" --project="${PROJECT_ID}" --region="${REGION}" --format=json)"
frontend_json="$(gcloud run services describe "${FRONTEND_SERVICE}" --project="${PROJECT_ID}" --region="${REGION}" --format=json)"
job_json="$(gcloud run jobs describe "${MIGRATION_JOB}" --project="${PROJECT_ID}" --region="${REGION}" --format=json)"

jq --exit-status --arg image "${api_image}" '[.. | objects | .image? // empty] | any(. == $image)' <<<"${api_json}" >/dev/null \
	|| fail "API service does not run ${api_image}"
jq --exit-status --arg image "${frontend_image}" '[.. | objects | .image? // empty] | any(. == $image)' <<<"${frontend_json}" >/dev/null \
	|| fail "frontend service does not run ${frontend_image}"
jq --exit-status --arg image "${api_image}" '[.. | objects | .image? // empty] | any(. == $image)' <<<"${job_json}" >/dev/null \
	|| fail "migration job does not run ${api_image}"

provider_json="$(gcloud iam workload-identity-pools providers describe "${WORKLOAD_IDENTITY_PROVIDER}" \
	--project="${PROJECT_ID}" \
	--location=global \
	--workload-identity-pool="${WORKLOAD_IDENTITY_POOL}" \
	--format=json)"
jq --exit-status --arg repository_id "${GITHUB_REPOSITORY_ID}" \
	'.attributeCondition | contains("assertion.repository_id == '\''" + $repository_id + "'\''") and contains("assertion.ref == '\''refs/heads/main'\''")' \
	<<<"${provider_json}" >/dev/null || fail "GitHub OIDC provider is not restricted to the production repository and branch"

deploy_policy="$(gcloud iam service-accounts get-iam-policy "${DEPLOY_ACCOUNT}" --project="${PROJECT_ID}" --format=json)"
expected_principal="principalSet://iam.googleapis.com/projects/${PROJECT_NUMBER}/locations/global/workloadIdentityPools/${WORKLOAD_IDENTITY_POOL}/attribute.repository_id/${GITHUB_REPOSITORY_ID}"
jq --exit-status --arg principal "${expected_principal}" \
	'any(.bindings[]; .role == "roles/iam.workloadIdentityUser" and ((.members // []) | index($principal) != null))' \
	<<<"${deploy_policy}" >/dev/null || fail "deployment identity does not trust the expected GitHub repository"

user_key_count="$(gcloud iam service-accounts keys list \
	--iam-account="${DEPLOY_ACCOUNT}" \
	--project="${PROJECT_ID}" \
	--managed-by=user \
	--format='value(name)' | wc -l | tr -d ' ')"
[[ "${user_key_count}" == "0" ]] || fail "deployment identity has a user-managed service-account key"

export TF_VAR_api_image_tag="${IMAGE_TAG}"
export TF_VAR_frontend_image_tag="${IMAGE_TAG}"
export TF_CLI_ARGS_plan="${TF_CLI_ARGS_plan:-} -var=api_image_tag=${IMAGE_TAG} -var=frontend_image_tag=${IMAGE_TAG}"

./scripts/verify-phase11.sh
echo "Phase 12 verification passed: keyless GitHub CI/CD deployed and verified immutable production images"
