#!/usr/bin/env bash

set -euo pipefail

PROJECT_ID="${PROJECT_ID:-eterealink}"
REGION="${REGION:-us-west1}"
SERVICE="${SERVICE:-eterealink-api}"
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

active_account="$(gcloud auth list --filter=status:ACTIVE --format='value(account)' | head -n 1)"
[[ -n "${active_account}" ]] || fail "no active gcloud account; run: gcloud auth login"

if [[ "$(gcloud config get-value project 2>/dev/null)" != "${PROJECT_ID}" ]]; then
	fail "active gcloud project must be ${PROJECT_ID}"
fi

if ! gcloud run services describe "${SERVICE}" \
	--project="${PROJECT_ID}" \
	--region="${REGION}" >/dev/null 2>&1; then
	fail "Cloud Run service ${SERVICE} is unavailable; deploy Phase 8 first"
fi

if ! gcloud run jobs describe "${MIGRATION_JOB}" \
	--project="${PROJECT_ID}" \
	--region="${REGION}" >/dev/null 2>&1; then
	fail "Cloud Run migration job ${MIGRATION_JOB} is unavailable; deploy Phase 8 first"
fi

if ! gcloud sql instances describe "${DB_INSTANCE}" \
	--project="${PROJECT_ID}" >/dev/null 2>&1; then
	fail "Cloud SQL instance ${DB_INSTANCE} is unavailable; deploy Phase 8 first"
fi

echo "Enabling Phase 9 Google Cloud APIs..."
gcloud services enable \
	compute.googleapis.com \
	servicenetworking.googleapis.com \
	--project="${PROJECT_ID}" \
	--quiet

if ! gcloud compute networks describe "${NETWORK}" \
	--project="${PROJECT_ID}" >/dev/null 2>&1; then
	echo "Creating custom VPC ${NETWORK}..."
	gcloud compute networks create "${NETWORK}" \
		--project="${PROJECT_ID}" \
		--subnet-mode=custom \
		--bgp-routing-mode=regional \
		--description="Aurea Link production network" \
		--quiet
else
	network_json="$(gcloud compute networks describe "${NETWORK}" \
		--project="${PROJECT_ID}" \
		--format=json)"
	jq --exit-status '.autoCreateSubnetworks == false and .routingConfig.routingMode == "REGIONAL"' \
		<<<"${network_json}" >/dev/null \
		|| fail "existing network ${NETWORK} is not a regional, custom-mode VPC"
fi

if ! gcloud compute networks subnets describe "${SUBNET}" \
	--project="${PROJECT_ID}" \
	--region="${REGION}" >/dev/null 2>&1; then
	echo "Creating regional subnet ${SUBNET} (${SUBNET_CIDR})..."
	gcloud compute networks subnets create "${SUBNET}" \
		--project="${PROJECT_ID}" \
		--network="${NETWORK}" \
		--region="${REGION}" \
		--range="${SUBNET_CIDR}" \
		--enable-private-ip-google-access \
		--description="Direct VPC egress for Aurea Link Cloud Run workloads" \
		--quiet
else
	subnet_json="$(gcloud compute networks subnets describe "${SUBNET}" \
		--project="${PROJECT_ID}" \
		--region="${REGION}" \
		--format=json)"
	jq --exit-status \
		--arg cidr "${SUBNET_CIDR}" \
		--arg network "${NETWORK}" \
		'.ipCidrRange == $cidr and (.network | endswith("/" + $network)) and .privateIpGoogleAccess == true' \
		<<<"${subnet_json}" >/dev/null \
		|| fail "existing subnet ${SUBNET} does not match ${NETWORK} ${SUBNET_CIDR} with Private Google Access enabled"
fi

psa_address="${PSA_RANGE_CIDR%/*}"
psa_prefix_length="${PSA_RANGE_CIDR#*/}"
if ! gcloud compute addresses describe "${PSA_RANGE}" \
	--project="${PROJECT_ID}" \
	--global >/dev/null 2>&1; then
	echo "Reserving ${PSA_RANGE_CIDR} for Private Services Access..."
	gcloud compute addresses create "${PSA_RANGE}" \
		--project="${PROJECT_ID}" \
		--global \
		--purpose=VPC_PEERING \
		--addresses="${psa_address}" \
		--prefix-length="${psa_prefix_length}" \
		--network="${NETWORK}" \
		--description="Private Services Access range for Aurea Link" \
		--quiet
else
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
		|| fail "existing address range ${PSA_RANGE} does not match ${PSA_RANGE_CIDR} on ${NETWORK}"
fi

peering_json="$(gcloud services vpc-peerings list \
	--project="${PROJECT_ID}" \
	--network="${NETWORK}" \
	--service="${SERVICE_NETWORKING_SERVICE}" \
	--format=json)"
if ! jq --exit-status --arg range "${PSA_RANGE}" \
	'any(.[]; (.reservedPeeringRanges // []) | index($range) != null)' \
	<<<"${peering_json}" >/dev/null; then
	if [[ "$(jq 'length' <<<"${peering_json}")" != "0" ]]; then
		fail "${NETWORK} already has a service networking connection that does not use ${PSA_RANGE}"
	fi
	echo "Connecting ${NETWORK} to Google managed services..."
	gcloud services vpc-peerings connect \
		--project="${PROJECT_ID}" \
		--network="${NETWORK}" \
		--service="${SERVICE_NETWORKING_SERVICE}" \
		--ranges="${PSA_RANGE}" \
		--quiet
fi

sql_json="$(gcloud sql instances describe "${DB_INSTANCE}" \
	--project="${PROJECT_ID}" \
	--format=json)"
private_address="$(jq -r '[.ipAddresses[]? | select(.type == "PRIVATE") | .ipAddress] | first // empty' <<<"${sql_json}")"
private_network="$(jq -r '.settings.ipConfiguration.privateNetwork // empty' <<<"${sql_json}")"
expected_network="projects/${PROJECT_ID}/global/networks/${NETWORK}"

if [[ -z "${private_address}" ]]; then
	echo "Adding a private address to Cloud SQL instance ${DB_INSTANCE}..."
	sql_operation="$(gcloud sql instances patch "${DB_INSTANCE}" \
		--project="${PROJECT_ID}" \
		--network="${expected_network}" \
		--async \
		--format='value(name)' \
		--quiet)"
	sql_operation="${sql_operation##*/}"
	[[ -n "${sql_operation}" ]] || fail "Cloud SQL did not return a private-network operation"
	gcloud beta sql operations wait "${sql_operation}" --project="${PROJECT_ID}"
	sql_json="$(gcloud sql instances describe "${DB_INSTANCE}" \
		--project="${PROJECT_ID}" \
		--format=json)"
	private_address="$(jq -r '[.ipAddresses[]? | select(.type == "PRIVATE") | .ipAddress] | first // empty' <<<"${sql_json}")"
	[[ -n "${private_address}" ]] || fail "Cloud SQL did not receive a private address"
elif [[ "${private_network}" != "${expected_network}" ]]; then
	fail "Cloud SQL private address belongs to ${private_network}, expected ${expected_network}"
fi

latest_secret_version="$(gcloud secrets versions list "${DATABASE_SECRET}" \
	--project="${PROJECT_ID}" \
	--filter='state=ENABLED' \
	--sort-by='~createTime' \
	--limit=1 \
	--format='value(name)')"
latest_secret_version="${latest_secret_version##*/}"
[[ -n "${latest_secret_version}" ]] || fail "${DATABASE_SECRET} has no enabled secret version"

current_database_url="$(gcloud secrets versions access "${latest_secret_version}" \
	--secret="${DATABASE_SECRET}" \
	--project="${PROJECT_ID}")"
case "${current_database_url}" in
	postgres://*'@'*) ;;
	*) fail "${DATABASE_SECRET} is not a supported PostgreSQL URL" ;;
esac

if [[ "${current_database_url}" == *"@${private_address}:5432/${DB_NAME}"* ]]; then
	private_secret_version="${latest_secret_version}"
else
	# Pin the currently serving revision before creating a new latest secret version.
	# This keeps cold starts on the working public connector path during the cutover.
	echo "Pinning the current API revision to database secret version ${latest_secret_version}..."
	gcloud run services update "${SERVICE}" \
		--project="${PROJECT_ID}" \
		--region="${REGION}" \
		--update-secrets="DATABASE_URL=${DATABASE_SECRET}:${latest_secret_version}" \
		--quiet

	database_credentials="${current_database_url#postgres://}"
	database_credentials="${database_credentials%%@*}"
	private_database_url="postgres://${database_credentials}@${private_address}:5432/${DB_NAME}?sslmode=disable&connect_timeout=10"
	echo "Creating the private database connection secret version..."
	private_secret_version="$(printf '%s' "${private_database_url}" | gcloud secrets versions add "${DATABASE_SECRET}" \
		--project="${PROJECT_ID}" \
		--data-file=- \
		--format='value(name)' \
		--quiet)"
	private_secret_version="${private_secret_version##*/}"
	[[ -n "${private_secret_version}" ]] || fail "failed to create a private database secret version"
	unset database_credentials private_database_url
fi
unset current_database_url

echo "Moving the migration job to Direct VPC egress..."
gcloud run jobs update "${MIGRATION_JOB}" \
	--project="${PROJECT_ID}" \
	--region="${REGION}" \
	--network="${NETWORK}" \
	--subnet="${SUBNET}" \
	--vpc-egress=private-ranges-only \
	--clear-cloudsql-instances \
	--update-secrets="DATABASE_URL=${DATABASE_SECRET}:${private_secret_version}" \
	--quiet

echo "Testing the private database path with the migration job..."
gcloud run jobs execute "${MIGRATION_JOB}" \
	--project="${PROJECT_ID}" \
	--region="${REGION}" \
	--wait \
	--quiet

echo "Moving API service ${SERVICE} to Direct VPC egress..."
gcloud run services update "${SERVICE}" \
	--project="${PROJECT_ID}" \
	--region="${REGION}" \
	--network="${NETWORK}" \
	--subnet="${SUBNET}" \
	--vpc-egress=private-ranges-only \
	--clear-cloudsql-instances \
	--update-secrets="DATABASE_URL=${DATABASE_SECRET}:${private_secret_version}" \
	--quiet

service_url="$(gcloud run services describe "${SERVICE}" \
	--project="${PROJECT_ID}" \
	--region="${REGION}" \
	--format='value(status.url)')"

echo "Verifying the API over the private database path..."
curl --fail --silent --show-error "${service_url}/health"
echo
curl --fail --silent --show-error "${service_url}/readyz"
echo

sql_json="$(gcloud sql instances describe "${DB_INSTANCE}" \
	--project="${PROJECT_ID}" \
	--format=json)"
if jq --exit-status '.settings.ipConfiguration.ipv4Enabled == true' <<<"${sql_json}" >/dev/null; then
	echo "Private connectivity is healthy; removing the Cloud SQL public address..."
	sql_operation="$(gcloud sql instances patch "${DB_INSTANCE}" \
		--project="${PROJECT_ID}" \
		--no-assign-ip \
		--async \
		--format='value(name)' \
		--quiet)"
	sql_operation="${sql_operation##*/}"
	[[ -n "${sql_operation}" ]] || fail "Cloud SQL did not return a public-address removal operation"
	gcloud beta sql operations wait "${sql_operation}" --project="${PROJECT_ID}"
fi

curl --fail --silent --show-error "${service_url}/readyz"
echo
echo "Phase 9 private network is ready: ${NETWORK}/${SUBNET} -> ${private_address}:5432"
