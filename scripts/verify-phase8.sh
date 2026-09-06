#!/usr/bin/env bash

set -euo pipefail

PROJECT_ID="${PROJECT_ID:-eterealink}"
REGION="${REGION:-us-west1}"
SERVICE="${SERVICE:-eterealink-api}"
FRONTEND_SERVICE="${FRONTEND_SERVICE:-eterealink-web}"
GCS_BUCKET="${GCS_BUCKET:-eterealink-files}"
CUSTOM_FRONTEND_URLS="${CUSTOM_FRONTEND_URLS:-https://eterealink.com https://www.eterealink.com}"
REQUEST_TIMEOUT_SECONDS="${REQUEST_TIMEOUT_SECONDS:-300}"

for command in curl gcloud grep jq; do
	if ! command -v "${command}" >/dev/null 2>&1; then
		echo "required command is unavailable: ${command}" >&2
		exit 1
	fi
done

service_url="$(gcloud run services describe "${SERVICE}" \
	--project="${PROJECT_ID}" \
	--region="${REGION}" \
	--format='value(status.url)')"
frontend_url="$(gcloud run services describe "${FRONTEND_SERVICE}" \
	--project="${PROJECT_ID}" \
	--region="${REGION}" \
	--format='value(status.url)')"
service_timeout="$(gcloud run services describe "${SERVICE}" \
	--project="${PROJECT_ID}" \
	--region="${REGION}" \
	--format='value(spec.template.spec.timeoutSeconds)')"
frontend_timeout="$(gcloud run services describe "${FRONTEND_SERVICE}" \
	--project="${PROJECT_ID}" \
	--region="${REGION}" \
	--format='value(spec.template.spec.timeoutSeconds)')"

if [[ "${service_timeout}" != "${REQUEST_TIMEOUT_SECONDS}" || "${frontend_timeout}" != "${REQUEST_TIMEOUT_SECONDS}" ]]; then
	echo "Cloud Run request timeouts must both be ${REQUEST_TIMEOUT_SECONDS}s (API=${service_timeout}s, frontend=${frontend_timeout}s)" >&2
	exit 1
fi

curl --fail --silent --show-error "${service_url}/health"
echo
curl --fail --silent --show-error "${service_url}/readyz"
echo
curl --fail --silent --show-error "${frontend_url}/health"
echo
curl --fail --silent --show-error "${frontend_url}/api/readyz"
echo

read -r -a custom_frontend_urls <<<"${CUSTOM_FRONTEND_URLS}"
firebase_access_token="$(gcloud auth print-access-token)"
firebase_config="$(curl --fail --silent --show-error \
	--header "Authorization: Bearer ${firebase_access_token}" \
	--header "x-goog-user-project: ${PROJECT_ID}" \
	"https://identitytoolkit.googleapis.com/admin/v2/projects/${PROJECT_ID}/config")"
cors_headers="$(mktemp)"
trap 'rm -f "${cors_headers}"' EXIT

for custom_frontend_url in "${custom_frontend_urls[@]}"; do
	echo "Verifying ${custom_frontend_url}..."
	curl --fail --silent --show-error "${custom_frontend_url}/health"
	echo
	curl --fail --silent --show-error "${custom_frontend_url}/api/readyz"
	echo
	curl --fail --silent --show-error "${custom_frontend_url}" | grep -Fq '<title>Eterealink — Share a file simply</title>'
	curl --fail --silent --show-error "${custom_frontend_url}/icon.svg" | grep -Fq '<svg'

	custom_frontend_hostname="${custom_frontend_url#https://}"
	if ! printf '%s' "${firebase_config}" | jq --exit-status --arg domain "${custom_frontend_hostname}" \
		'.authorizedDomains | index($domain) != null' >/dev/null; then
		echo "Firebase does not authorize ${custom_frontend_hostname}" >&2
		exit 1
	fi

	curl --fail --silent --show-error \
		--request OPTIONS \
		--header "Origin: ${custom_frontend_url}" \
		--header 'Access-Control-Request-Method: PUT' \
		--header 'Access-Control-Request-Headers: content-type,x-goog-resumable' \
		--dump-header "${cors_headers}" \
		--output /dev/null \
		"https://storage.googleapis.com/${GCS_BUCKET}/phase8-cors-check"
	if ! grep -Fqi "access-control-allow-origin: ${custom_frontend_url}" "${cors_headers}"; then
		echo "Cloud Storage CORS does not authorize ${custom_frontend_url}" >&2
		exit 1
	fi
done
unset firebase_access_token firebase_config

gcloud run services list \
	--project="${PROJECT_ID}" \
	--region="${REGION}" \
	--filter="metadata.name:(${SERVICE} OR ${FRONTEND_SERVICE})" \
	--format='table(metadata.name,status.latestReadyRevisionName,status.url,spec.template.spec.serviceAccountName)'
