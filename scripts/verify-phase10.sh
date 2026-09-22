#!/usr/bin/env bash

set -euo pipefail

TERRAFORM_BIN="${TERRAFORM_BIN:-terraform}"
INFRASTRUCTURE_DIR="${INFRASTRUCTURE_DIR:-infrastructure}"

fail() {
	echo "$1" >&2
	exit 1
}

command -v "${TERRAFORM_BIN}" >/dev/null 2>&1 || fail "Terraform is unavailable: ${TERRAFORM_BIN}"
[[ -n "${TF_VAR_database_password:-}" ]] || fail "TF_VAR_database_password must be set for Terraform verification"
if [[ ! -f "${INFRASTRUCTURE_DIR}/terraform.tfvars" ]]; then
	[[ -n "${TF_VAR_api_image_tag:-}" && -n "${TF_VAR_frontend_image_tag:-}" ]] \
		|| fail "infrastructure/terraform.tfvars is unavailable and Terraform image-tag variables are unset"
fi

"${TERRAFORM_BIN}" -chdir="${INFRASTRUCTURE_DIR}" fmt -check -recursive
"${TERRAFORM_BIN}" -chdir="${INFRASTRUCTURE_DIR}" validate

resource_count="$("${TERRAFORM_BIN}" -chdir="${INFRASTRUCTURE_DIR}" state list | wc -l | tr -d ' ')"
[[ "${resource_count}" -ge 30 ]] || fail "Terraform state is incomplete: found ${resource_count} resources"

set +e
"${TERRAFORM_BIN}" -chdir="${INFRASTRUCTURE_DIR}" plan -detailed-exitcode -input=false
plan_status=$?
set -e

case "${plan_status}" in
	0) ;;
	1) fail "Terraform plan failed" ;;
	2) fail "Terraform detected infrastructure drift; review and apply the plan" ;;
	*) fail "Terraform returned unexpected plan status ${plan_status}" ;;
esac

./scripts/verify-phase9.sh
echo "Phase 10 verification passed: Terraform matches the healthy private GCP deployment"
