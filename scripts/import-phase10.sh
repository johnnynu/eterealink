#!/usr/bin/env bash

set -euo pipefail

PROJECT_ID="${PROJECT_ID:-eterealink}"
REGION="${REGION:-us-west1}"
TERRAFORM_BIN="${TERRAFORM_BIN:-terraform}"
INFRASTRUCTURE_DIR="${INFRASTRUCTURE_DIR:-infrastructure}"
export GOOGLE_CLOUD_QUOTA_PROJECT="${GOOGLE_CLOUD_QUOTA_PROJECT:-${PROJECT_ID}}"

fail() {
	echo "$1" >&2
	exit 1
}

command -v "${TERRAFORM_BIN}" >/dev/null 2>&1 || fail "Terraform is unavailable: ${TERRAFORM_BIN}"
[[ -n "${TF_VAR_database_password:-}" ]] || fail "TF_VAR_database_password must be set before importing"
[[ -f "${INFRASTRUCTURE_DIR}/terraform.tfvars" ]] || fail "copy infrastructure/terraform.tfvars.example to infrastructure/terraform.tfvars and set both image tags"

import_if_missing() {
	local address="$1"
	local id="$2"

	if "${TERRAFORM_BIN}" -chdir="${INFRASTRUCTURE_DIR}" state show "${address}" >/dev/null 2>&1; then
		echo "Already imported: ${address}"
		return
	fi

	echo "Importing ${address}..."
	"${TERRAFORM_BIN}" -chdir="${INFRASTRUCTURE_DIR}" import -input=false "${address}" "${id}"
}

required_services=(
	artifactregistry.googleapis.com
	cloudbuild.googleapis.com
	compute.googleapis.com
	firebase.googleapis.com
	iamcredentials.googleapis.com
	identitytoolkit.googleapis.com
	run.googleapis.com
	secretmanager.googleapis.com
	servicenetworking.googleapis.com
	sqladmin.googleapis.com
	storage.googleapis.com
)

for service in "${required_services[@]}"; do
	import_if_missing "google_project_service.required[\"${service}\"]" "${PROJECT_ID}/${service}"
done

api_service_account="eterealink-api@${PROJECT_ID}.iam.gserviceaccount.com"
frontend_service_account="eterealink-web@${PROJECT_ID}.iam.gserviceaccount.com"

import_if_missing google_artifact_registry_repository.application "projects/${PROJECT_ID}/locations/${REGION}/repositories/eterealink"
import_if_missing google_service_account.api "projects/${PROJECT_ID}/serviceAccounts/${api_service_account}"
import_if_missing google_service_account.frontend "projects/${PROJECT_ID}/serviceAccounts/${frontend_service_account}"
import_if_missing google_service_account_iam_member.api_self_token_creator "projects/${PROJECT_ID}/serviceAccounts/${api_service_account} roles/iam.serviceAccountTokenCreator serviceAccount:${api_service_account}"
import_if_missing google_project_iam_member.api_cloud_sql_client "${PROJECT_ID} roles/cloudsql.client serviceAccount:${api_service_account}"
import_if_missing google_identity_platform_config.authentication "projects/${PROJECT_ID}/config"

import_if_missing google_storage_bucket.files "${PROJECT_ID}/eterealink-files"
import_if_missing google_storage_bucket_iam_member.api_object_user "b/eterealink-files roles/storage.objectUser serviceAccount:${api_service_account}"

import_if_missing google_compute_network.application "projects/${PROJECT_ID}/global/networks/eterealink"
import_if_missing google_compute_subnetwork.cloud_run "projects/${PROJECT_ID}/regions/${REGION}/subnetworks/eterealink-us-west1"
import_if_missing google_compute_global_address.private_services "projects/${PROJECT_ID}/global/addresses/eterealink-google-managed-services"
import_if_missing google_service_networking_connection.private_services "projects/${PROJECT_ID}/global/networks/eterealink:servicenetworking.googleapis.com"

import_if_missing google_sql_database_instance.application "projects/${PROJECT_ID}/instances/eterealink-db"
import_if_missing google_sql_database.application "projects/${PROJECT_ID}/instances/eterealink-db/databases/eterealink"
import_if_missing google_sql_user.application "${PROJECT_ID}/eterealink-db/eterealink"
import_if_missing google_secret_manager_secret.database_url "projects/${PROJECT_ID}/secrets/eterealink-database-url"
import_if_missing google_secret_manager_secret_iam_member.api_database_url "projects/${PROJECT_ID}/secrets/eterealink-database-url roles/secretmanager.secretAccessor serviceAccount:${api_service_account}"

import_if_missing google_cloud_run_v2_job.migrations "projects/${PROJECT_ID}/locations/${REGION}/jobs/eterealink-migrate"
import_if_missing google_cloud_run_v2_service.api "projects/${PROJECT_ID}/locations/${REGION}/services/eterealink-api"
import_if_missing google_cloud_run_v2_service.frontend "projects/${PROJECT_ID}/locations/${REGION}/services/eterealink-web"
import_if_missing google_cloud_run_v2_service_iam_member.api_public "projects/${PROJECT_ID}/locations/${REGION}/services/eterealink-api roles/run.invoker allUsers"
import_if_missing google_cloud_run_v2_service_iam_member.frontend_public "projects/${PROJECT_ID}/locations/${REGION}/services/eterealink-web roles/run.invoker allUsers"

for domain in aurealink.app www.aurealink.app eterealink.com www.eterealink.com; do
	import_if_missing "google_cloud_run_domain_mapping.frontend[\"${domain}\"]" "locations/${REGION}/namespaces/${PROJECT_ID}/domainmappings/${domain}"
done

echo "Phase 10 import is complete. Review terraform plan before applying any changes."
