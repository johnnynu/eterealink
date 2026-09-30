#!/usr/bin/env bash

set -euo pipefail

PROJECT_ID="${PROJECT_ID:-eterealink}"
REGION="${REGION:-us-west1}"
REPOSITORY="${REPOSITORY:-eterealink}"
CLEANUP_JOB="${CLEANUP_JOB:-eterealink-cleanup}"
CLEANUP_SCHEDULER="${CLEANUP_SCHEDULER:-eterealink-cleanup-hourly}"
CLEANUP_ACCOUNT="${CLEANUP_ACCOUNT:-eterealink-cleanup@${PROJECT_ID}.iam.gserviceaccount.com}"
DEPLOY_ACCOUNT="${DEPLOY_ACCOUNT:-eterealink-deploy@${PROJECT_ID}.iam.gserviceaccount.com}"
GCS_BUCKET="${GCS_BUCKET:-eterealink-files}"
DATABASE_SECRET="${DATABASE_SECRET:-eterealink-database-url}"
IMAGE_TAG="${IMAGE_TAG:-$(git rev-parse HEAD)}"
OBJECT_CLEANUP_ROLE="projects/${PROJECT_ID}/roles/aureaLinkObjectCleanup"
SCHEDULER_DEPLOY_ROLE="projects/${PROJECT_ID}/roles/aureaLinkSchedulerDeployer"

fail() {
	echo "$1" >&2
	exit 1
}

for command in gcloud git jq; do
	command -v "${command}" >/dev/null 2>&1 || fail "required command is unavailable: ${command}"
done

cleanup_image="${REGION}-docker.pkg.dev/${PROJECT_ID}/${REPOSITORY}/api:${IMAGE_TAG}"
cleanup_json="$(gcloud run jobs describe "${CLEANUP_JOB}" --project="${PROJECT_ID}" --region="${REGION}" --format=json)"
jq --exit-status --arg image "${cleanup_image}" '[.. | objects | .image? // empty] | any(. == $image)' <<<"${cleanup_json}" >/dev/null \
	|| fail "cleanup job does not run ${cleanup_image}"
jq --exit-status --arg account "${CLEANUP_ACCOUNT}" '[.. | objects | .serviceAccount? // .serviceAccountName? // empty] | any(. == $account)' <<<"${cleanup_json}" >/dev/null \
	|| fail "cleanup job does not use ${CLEANUP_ACCOUNT}"
jq --exit-status --arg bucket "${GCS_BUCKET}" '
	([.. | objects | select(.name? == "GCS_BUCKET") | .value? // empty] | any(. == $bucket)) and
	([.. | objects | select(.name? == "CLEANUP_BATCH_SIZE") | .value? // empty] | any(. == "100")) and
	([.. | objects | .command? // empty] | any(. == ["/app/cleanup"]))
' <<<"${cleanup_json}" >/dev/null || fail "cleanup job command or bounded configuration is missing"
jq --exit-status --arg secret "${DATABASE_SECRET}" \
	'[.. | objects | .secretKeyRef? // empty | select((.secret // .name // "") | endswith($secret))] | length > 0 and all(.[]; (.version // .key // "") | test("^[0-9]+$"))' \
	<<<"${cleanup_json}" >/dev/null || fail "cleanup database secret is not pinned to a numeric version"

role_json="$(gcloud iam roles describe aureaLinkObjectCleanup --project="${PROJECT_ID}" --format=json)"
jq --exit-status '.includedPermissions == ["storage.objects.delete"]' <<<"${role_json}" >/dev/null \
	|| fail "cleanup object role has unexpected permissions"

project_policy="$(gcloud projects get-iam-policy "${PROJECT_ID}" --format=json)"
jq --exit-status --arg deploy "serviceAccount:${DEPLOY_ACCOUNT}" --arg cleanup "serviceAccount:${CLEANUP_ACCOUNT}" --arg scheduler_role "${SCHEDULER_DEPLOY_ROLE}" '
	any(.bindings[]; .role == $scheduler_role and ((.members // []) | index($deploy) != null)) and
	([.bindings[] | select((.members // []) | index($cleanup) != null)] | length == 0)
' <<<"${project_policy}" >/dev/null || fail "deployment Scheduler access or cleanup identity isolation is misconfigured"

scheduler_role_json="$(gcloud iam roles describe aureaLinkSchedulerDeployer --project="${PROJECT_ID}" --format=json)"
jq --exit-status '
	.includedPermissions | sort == [
		"cloudscheduler.jobs.create",
		"cloudscheduler.jobs.delete",
		"cloudscheduler.jobs.enable",
		"cloudscheduler.jobs.fullView",
		"cloudscheduler.jobs.get",
		"cloudscheduler.jobs.list",
		"cloudscheduler.jobs.pause",
		"cloudscheduler.jobs.update",
		"cloudscheduler.locations.get",
		"cloudscheduler.locations.list"
	]
' <<<"${scheduler_role_json}" >/dev/null || fail "Scheduler deployment role has unexpected permissions"

bucket_policy="$(gcloud storage buckets get-iam-policy "gs://${GCS_BUCKET}" --format=json)"
anonymous_condition="resource.name.startsWith('projects/_/buckets/${GCS_BUCKET}/objects/anonymous/')"
jq --exit-status --arg role "${OBJECT_CLEANUP_ROLE}" --arg account "serviceAccount:${CLEANUP_ACCOUNT}" --arg condition "${anonymous_condition}" \
	'any(.bindings[]; .role == $role and ((.members // []) | index($account) != null) and .condition.expression == $condition)' \
	<<<"${bucket_policy}" >/dev/null || fail "cleanup identity cannot delete bucket objects"

secret_policy="$(gcloud secrets get-iam-policy "${DATABASE_SECRET}" --project="${PROJECT_ID}" --format=json)"
jq --exit-status --arg account "serviceAccount:${CLEANUP_ACCOUNT}" \
	'any(.bindings[]; .role == "roles/secretmanager.secretAccessor" and ((.members // []) | index($account) != null))' \
	<<<"${secret_policy}" >/dev/null || fail "cleanup identity cannot access ${DATABASE_SECRET}"

job_policy="$(gcloud run jobs get-iam-policy "${CLEANUP_JOB}" --project="${PROJECT_ID}" --region="${REGION}" --format=json)"
jq --exit-status --arg account "serviceAccount:${CLEANUP_ACCOUNT}" \
	'any(.bindings[]; .role == "roles/run.invoker" and ((.members // []) | index($account) != null))' \
	<<<"${job_policy}" >/dev/null || fail "cleanup scheduler identity cannot invoke the cleanup job"

scheduler_json="$(gcloud scheduler jobs describe "${CLEANUP_SCHEDULER}" --project="${PROJECT_ID}" --location="${REGION}" --format=json)"
expected_uri="https://run.googleapis.com/v2/projects/${PROJECT_ID}/locations/${REGION}/jobs/${CLEANUP_JOB}:run"
jq --exit-status --arg uri "${expected_uri}" --arg account "${CLEANUP_ACCOUNT}" \
	'.state == "ENABLED" and .schedule == "17 * * * *" and .timeZone == "Etc/UTC" and .httpTarget.httpMethod == "POST" and .httpTarget.uri == $uri and .httpTarget.oauthToken.serviceAccountEmail == $account' \
	<<<"${scheduler_json}" >/dev/null || fail "hourly cleanup schedule is missing, paused, or misconfigured"

gcloud run jobs execute "${CLEANUP_JOB}" --project="${PROJECT_ID}" --region="${REGION}" --wait >/dev/null
cleanup_logs="$(gcloud logging read \
	"resource.type=\"cloud_run_job\" AND resource.labels.job_name=\"${CLEANUP_JOB}\" AND jsonPayload.message=\"lifecycle cleanup complete\"" \
	--project="${PROJECT_ID}" \
	--freshness=10m \
	--limit=5 \
	--format=json)"
jq --exit-status 'any(.[]?; (.jsonPayload.candidates | type) == "number" and (.jsonPayload.metadata_deleted | type) == "number")' \
	<<<"${cleanup_logs}" >/dev/null || fail "cleanup job did not emit a structured completion summary"

./scripts/verify-phase13.sh
echo "Phase 14 verification passed: hourly least-privilege cleanup removes expired anonymous objects before cascading metadata"
