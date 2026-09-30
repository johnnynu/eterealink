#!/usr/bin/env bash

set -euo pipefail

PROJECT_ID="${PROJECT_ID:-eterealink}"
REGION="${REGION:-us-west1}"
SERVICE="${SERVICE:-eterealink-api}"
DEPLOY_ACCOUNT="${DEPLOY_ACCOUNT:-eterealink-deploy@${PROJECT_ID}.iam.gserviceaccount.com}"
CANONICAL_HOST="${CANONICAL_HOST:-aurealink.app}"
UPTIME_DISPLAY_NAME="${UPTIME_DISPLAY_NAME:-Aurea Link frontend}"
ALERT_DISPLAY_NAME="${ALERT_DISPLAY_NAME:-Aurea Link frontend unavailable}"
DASHBOARD_DISPLAY_NAME="${DASHBOARD_DISPLAY_NAME:-Aurea Link Production}"
OBSERVABILITY_ROLE="projects/${PROJECT_ID}/roles/aureaLinkObservabilityDeployer"

fail() {
	echo "$1" >&2
	exit 1
}

for command in curl gcloud jq; do
	command -v "${command}" >/dev/null 2>&1 || fail "required command is unavailable: ${command}"
done

access_token="$(gcloud auth print-access-token)"
authorization="Authorization: Bearer ${access_token}"

project_policy="$(gcloud projects get-iam-policy "${PROJECT_ID}" --format=json)"
jq --exit-status --arg role "${OBSERVABILITY_ROLE}" --arg member "serviceAccount:${DEPLOY_ACCOUNT}" \
	'any(.bindings[]?; .role == $role and ((.members // []) | index($member) != null))' \
	<<<"${project_policy}" >/dev/null || fail "deployment identity is missing the least-privilege observability role"

role_json="$(gcloud iam roles describe aureaLinkObservabilityDeployer --project="${PROJECT_ID}" --format=json)"
jq --exit-status '
	.includedPermissions as $permissions |
	["logging.logEntries.list", "monitoring.alertPolicies.create", "monitoring.dashboards.create", "monitoring.notificationChannels.create", "monitoring.uptimeCheckConfigs.create"] |
	all(. as $permission | ($permissions | index($permission) != null))
' <<<"${role_json}" >/dev/null || fail "observability deployment role is missing a required permission"

uptime_json="$(curl --fail --silent --show-error --header "${authorization}" \
	"https://monitoring.googleapis.com/v3/projects/${PROJECT_ID}/uptimeCheckConfigs")"
uptime_id="$(jq -r --arg name "${UPTIME_DISPLAY_NAME}" --arg host "${CANONICAL_HOST}" \
	'[.uptimeCheckConfigs[]? | select(.displayName == $name and .monitoredResource.labels.host == $host and .httpCheck.path == "/health" and .httpCheck.useSsl == true and .httpCheck.validateSsl == true and .period == "300s" and .logCheckFailures == true)] | first | .name // empty | split("/") | last' \
	<<<"${uptime_json}")"
[[ -n "${uptime_id}" ]] || fail "production frontend uptime check is missing or misconfigured"

alert_json="$(curl --fail --silent --show-error --header "${authorization}" \
	"https://monitoring.googleapis.com/v3/projects/${PROJECT_ID}/alertPolicies")"
jq --exit-status --arg name "${ALERT_DISPLAY_NAME}" --arg check_id "${uptime_id}" \
	'any(.alertPolicies[]?; .displayName == $name and .enabled == true and .severity == "CRITICAL" and (.documentation.content | contains("roll back")) and any(.conditions[]?; .conditionThreshold.filter | contains($check_id)))' \
	<<<"${alert_json}" >/dev/null || fail "frontend availability alert is missing, disabled, or detached from the uptime check"

dashboard_json="$(curl --fail --silent --show-error --header "${authorization}" \
	"https://monitoring.googleapis.com/v1/projects/${PROJECT_ID}/dashboards")"
jq --exit-status --arg name "${DASHBOARD_DISPLAY_NAME}" \
	'any(.dashboards[]?; .displayName == $name and (tostring | contains("run.googleapis.com/request_count")) and (tostring | contains("run.googleapis.com/request_latencies")) and (tostring | contains("run.googleapis.com/container/instance_count")) and (tostring | contains("logsPanel")))' \
	<<<"${dashboard_json}" >/dev/null || fail "production dashboard is missing required Cloud Run charts or logs panel"

api_url="$(gcloud run services describe "${SERVICE}" --project="${PROJECT_ID}" --region="${REGION}" --format='value(status.url)')"
curl --fail --silent --show-error --output /dev/null "${api_url}/readyz"

structured_logs="[]"
for _ in {1..12}; do
	structured_logs="$(gcloud logging read \
		"resource.type=\"cloud_run_revision\" AND resource.labels.service_name=\"${SERVICE}\" AND jsonPayload.message=\"request\" AND jsonPayload.path=\"/readyz\"" \
		--project="${PROJECT_ID}" \
		--freshness=10m \
		--limit=5 \
		--format=json)"
	if jq --exit-status 'any(.[]?; (.jsonPayload.request_id | length) > 0 and .jsonPayload.status == 200 and .httpRequest.status == 200 and (.trace | length) > 0)' \
		<<<"${structured_logs}" >/dev/null; then
		break
	fi
	sleep 5
done
jq --exit-status 'any(.[]?; (.jsonPayload.request_id | length) > 0 and .jsonPayload.status == 200 and .httpRequest.status == 200 and (.trace | length) > 0)' \
	<<<"${structured_logs}" >/dev/null || fail "a correlated structured API request log did not appear within 60 seconds"

./scripts/verify-phase12.sh
echo "Phase 13 verification passed: public uptime, actionable alerting, production dashboard, and correlated structured logs are active"
