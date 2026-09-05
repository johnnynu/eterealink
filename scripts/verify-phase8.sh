#!/usr/bin/env bash

set -euo pipefail

PROJECT_ID="${PROJECT_ID:-eterealink}"
REGION="${REGION:-us-west1}"
SERVICE="${SERVICE:-eterealink-api}"

service_url="$(gcloud run services describe "${SERVICE}" \
	--project="${PROJECT_ID}" \
	--region="${REGION}" \
	--format='value(status.url)')"

curl --fail --silent --show-error "${service_url}/healthz"
echo
curl --fail --silent --show-error "${service_url}/readyz"
echo

gcloud run services describe "${SERVICE}" \
	--project="${PROJECT_ID}" \
	--region="${REGION}" \
	--format='table(metadata.name,status.latestReadyRevisionName,status.url,spec.template.spec.serviceAccountName)'
