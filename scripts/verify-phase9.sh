#!/usr/bin/env bash

set -euo pipefail

PROJECT_ID="${PROJECT_ID:-eterealink}"
REGION="${REGION:-us-west1}"
SERVICE="${SERVICE:-eterealink-api}"
FRONTEND_SERVICE="${FRONTEND_SERVICE:-eterealink-web}"
MIGRATION_JOB="${MIGRATION_JOB:-eterealink-migrate}"
DB_INSTANCE="${DB_INSTANCE:-eterealink-db}"
DB_NAME="${DB_NAME:-eterealink}"
DATABASE_SECRET="${DATABASE_SECRET:-eterealink-database-url}"
NETWORK="${NETWORK:-eterealink}"
SUBNET="${SUBNET:-eterealink-us-west1}"
SUBNET_CIDR="${SUBNET_CIDR:-10.0.1.0/24}"
PSA_RANGE="${PSA_RANGE:-eterealink-google-managed-services}"
PSA_RANGE_CIDR="${PSA_RANGE_CIDR:-10.10.0.0/24}"
SERVICE_NETWORKING_SERVICE="servicenetworking.googleapis.com"

fail() {
	echo "$1" >&2
	exit 1
}

for command in curl gcloud jq; do
	command -v "${command}" >/dev/null 2>&1 || fail "required command is unavailable: ${command}"
done

network_json="$(gcloud compute networks describe "${NETWORK}" \
	--project="${PROJECT_ID}" \
	--format=json)"
jq --exit-status '.autoCreateSubnetworks == false and .routingConfig.routingMode == "REGIONAL"' \
	<<<"${network_json}" >/dev/null \
	|| fail "${NETWORK} is not a regional, custom-mode VPC"

subnet_json="$(gcloud compute networks subnets describe "${SUBNET}" \
	--project="${PROJECT_ID}" \
	--region="${REGION}" \
	--format=json)"
jq --exit-status \
	--arg cidr "${SUBNET_CIDR}" \
	--arg network "${NETWORK}" \
	'.ipCidrRange == $cidr and (.network | endswith("/" + $network)) and .privateIpGoogleAccess == true' \
	<<<"${subnet_json}" >/dev/null \
	|| fail "${SUBNET} does not match the expected network configuration"

psa_address="${PSA_RANGE_CIDR%/*}"
psa_prefix_length="${PSA_RANGE_CIDR#*/}"
psa_json="$(gcloud compute addresses describe "${PSA_RANGE}" \
	--project="${PROJECT_ID}" \
	--global \
	--format=json)"
jq --exit-status \
	--arg address "${psa_address}" \
	--argjson prefix_length "${psa_prefix_length}" \
	--arg network "${NETWORK}" \
	'.address == $address and .prefixLength == $prefix_length and .purpose == "VPC_PEERING" and (.network | endswith("/" + $network))' \
	<<<"${psa_json}" >/dev/null \
	|| fail "${PSA_RANGE} does not match ${PSA_RANGE_CIDR} on ${NETWORK}"

peering_json="$(gcloud services vpc-peerings list \
	--project="${PROJECT_ID}" \
	--network="${NETWORK}" \
	--service="${SERVICE_NETWORKING_SERVICE}" \
	--format=json)"
jq --exit-status --arg range "${PSA_RANGE}" \
	'any(.[]; (.reservedPeeringRanges // []) | index($range) != null)' \
	<<<"${peering_json}" >/dev/null \
	|| fail "Private Services Access is not connected through ${PSA_RANGE}"

sql_json="$(gcloud sql instances describe "${DB_INSTANCE}" \
	--project="${PROJECT_ID}" \
	--format=json)"
expected_network="projects/${PROJECT_ID}/global/networks/${NETWORK}"
jq --exit-status \
	--arg network "${expected_network}" \
	'.settings.ipConfiguration.privateNetwork == $network and .settings.ipConfiguration.ipv4Enabled == false and ((.settings.ipConfiguration.authorizedNetworks // []) | length == 0)' \
	<<<"${sql_json}" >/dev/null \
	|| fail "${DB_INSTANCE} is not private-only on ${NETWORK}"
private_address="$(jq -r '[.ipAddresses[]? | select(.type == "PRIVATE") | .ipAddress] | first // empty' <<<"${sql_json}")"
[[ -n "${private_address}" ]] || fail "${DB_INSTANCE} has no private address"

check_direct_vpc() {
	local kind="$1"
	local resource_json="$2"
	local interfaces egress

	interfaces="$(jq -r '[.. | objects | .["run.googleapis.com/network-interfaces"]? // empty] | first // empty' <<<"${resource_json}")"
	egress="$(jq -r '[.. | objects | .["run.googleapis.com/vpc-access-egress"]? // empty] | first // empty' <<<"${resource_json}")"
	[[ -n "${interfaces}" ]] || fail "${kind} does not use Direct VPC egress"
	jq --exit-status \
		--arg network "${NETWORK}" \
		--arg subnet "${SUBNET}" \
		'(if type == "string" then fromjson else . end) | length == 1 and .[0].network == $network and .[0].subnetwork == $subnet' \
		<<<"${interfaces}" >/dev/null \
		|| fail "${kind} is not attached to ${NETWORK}/${SUBNET}"
	[[ "${egress}" == "private-ranges-only" ]] \
		|| fail "${kind} VPC egress must be private-ranges-only"
	if jq --exit-status '[.. | objects | .["run.googleapis.com/cloudsql-instances"]? // empty | select(length > 0)] | length > 0' \
		<<<"${resource_json}" >/dev/null; then
		fail "${kind} still has a Cloud SQL Auth Proxy attachment"
	fi
	if jq --exit-status '[.. | objects | .cloudSqlInstance? // empty] | length > 0' \
		<<<"${resource_json}" >/dev/null; then
		fail "${kind} still has a Cloud SQL Auth Proxy volume"
	fi
}

service_json="$(gcloud run services describe "${SERVICE}" \
	--project="${PROJECT_ID}" \
	--region="${REGION}" \
	--format=json)"
job_json="$(gcloud run jobs describe "${MIGRATION_JOB}" \
	--project="${PROJECT_ID}" \
	--region="${REGION}" \
	--format=json)"
check_direct_vpc "Cloud Run service ${SERVICE}" "${service_json}"
check_direct_vpc "Cloud Run job ${MIGRATION_JOB}" "${job_json}"

latest_secret_version="$(gcloud secrets versions list "${DATABASE_SECRET}" \
	--project="${PROJECT_ID}" \
	--filter='state=ENABLED' \
	--sort-by='~createTime' \
	--limit=1 \
	--format='value(name)')"
latest_secret_version="${latest_secret_version##*/}"
database_url="$(gcloud secrets versions access "${latest_secret_version}" \
	--secret="${DATABASE_SECRET}" \
	--project="${PROJECT_ID}")"
[[ "${database_url}" == *"@${private_address}:5432/${DB_NAME}"* ]] \
	|| fail "latest ${DATABASE_SECRET} version does not target the private Cloud SQL address"
unset database_url

service_url="$(jq -r '.status.url' <<<"${service_json}")"
frontend_url="$(gcloud run services describe "${FRONTEND_SERVICE}" \
	--project="${PROJECT_ID}" \
	--region="${REGION}" \
	--format='value(status.url)')"
curl --fail --silent --show-error "${service_url}/health"
echo
curl --fail --silent --show-error "${service_url}/readyz"
echo
curl --fail --silent --show-error "${frontend_url}/api/readyz"
echo

gcloud compute networks subnets describe "${SUBNET}" \
	--project="${PROJECT_ID}" \
	--region="${REGION}" \
	--format='table(name,network.basename(),ipCidrRange,privateIpGoogleAccess)'
gcloud sql instances describe "${DB_INSTANCE}" \
	--project="${PROJECT_ID}" \
	--format='table(name,region,ipAddresses.type,ipAddresses.ipAddress,settings.ipConfiguration.privateNetwork)'
echo "Phase 9 verification passed: database traffic uses ${NETWORK}/${SUBNET} and ${private_address}:5432"
