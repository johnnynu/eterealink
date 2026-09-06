#!/usr/bin/env bash

set -euo pipefail

PROJECT_ID="${PROJECT_ID:-eterealink}"
REGION="${REGION:-us-west1}"
SERVICE="${SERVICE:-eterealink-api}"
FRONTEND_SERVICE="${FRONTEND_SERVICE:-eterealink-web}"

service_url="$(gcloud run services describe "${SERVICE}" \
	--project="${PROJECT_ID}" \
	--region="${REGION}" \
	--format='value(status.url)')"
frontend_url="$(gcloud run services describe "${FRONTEND_SERVICE}" \
	--project="${PROJECT_ID}" \
	--region="${REGION}" \
	--format='value(status.url)')"

curl --fail --silent --show-error "${service_url}/health"
echo
curl --fail --silent --show-error "${service_url}/readyz"
echo
curl --fail --silent --show-error "${frontend_url}/health"
echo
curl --fail --silent --show-error "${frontend_url}/api/readyz"
echo

gcloud run services list \
	--project="${PROJECT_ID}" \
	--region="${REGION}" \
	--filter="metadata.name:(${SERVICE} OR ${FRONTEND_SERVICE})" \
	--format='table(metadata.name,status.latestReadyRevisionName,status.url,spec.template.spec.serviceAccountName)'
