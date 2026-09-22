variable "project_id" {
  description = "Google Cloud project that stores Terraform state."
  type        = string
  default     = "eterealink"
}

variable "region" {
  description = "Region for the Terraform state bucket."
  type        = string
  default     = "us-west1"
}

variable "state_bucket_name" {
  description = "Globally unique bucket used by the root Terraform backend."
  type        = string
  default     = "eterealink-terraform-state"
}

variable "github_repository" {
  description = "GitHub repository allowed to deploy Aurea Link."
  type        = string
  default     = "johnnynu/eterealink"
}

variable "github_repository_id" {
  description = "Immutable GitHub repository ID allowed to deploy Aurea Link."
  type        = string
  default     = "1352078479"
}

locals {
  deploy_service_account_id = "eterealink-deploy"
  workload_identity_pool_id = "aurealink-github"
  workload_provider_id      = "github"
  production_ref            = "refs/heads/main"
  deploy_project_roles = toset([
    "roles/artifactregistry.admin",
    "roles/cloudsql.admin",
    "roles/compute.networkAdmin",
    "roles/iam.roleAdmin",
    "roles/iam.serviceAccountAdmin",
    "roles/iam.serviceAccountUser",
    "roles/identityplatform.admin",
    "roles/resourcemanager.projectIamAdmin",
    "roles/run.admin",
    "roles/secretmanager.admin",
    "roles/servicenetworking.networksAdmin",
    "roles/serviceusage.serviceUsageAdmin",
    "roles/storage.admin",
  ])
}

resource "google_project_service" "bootstrap" {
  for_each = toset([
    "iam.googleapis.com",
    "iamcredentials.googleapis.com",
    "sts.googleapis.com",
    "storage.googleapis.com",
  ])

  project            = var.project_id
  service            = each.value
  disable_on_destroy = false
}

resource "google_storage_bucket" "terraform_state" {
  project                     = var.project_id
  name                        = var.state_bucket_name
  location                    = var.region
  storage_class               = "STANDARD"
  uniform_bucket_level_access = true
  public_access_prevention    = "enforced"
  force_destroy               = false

  versioning {
    enabled = true
  }

  lifecycle {
    prevent_destroy = true
  }

  depends_on = [google_project_service.bootstrap["storage.googleapis.com"]]
}

resource "google_service_account" "deploy" {
  project      = var.project_id
  account_id   = local.deploy_service_account_id
  display_name = "Aurea Link GitHub deployment"
  description  = "Keyless production deployment identity for the Aurea Link main-branch workflow"

  depends_on = [google_project_service.bootstrap["iam.googleapis.com"]]

  lifecycle {
    prevent_destroy = true
  }
}

resource "google_project_iam_member" "deploy" {
  for_each = local.deploy_project_roles

  project = var.project_id
  role    = each.value
  member  = "serviceAccount:${google_service_account.deploy.email}"
}

resource "google_storage_bucket_iam_member" "deploy_state" {
  bucket = google_storage_bucket.terraform_state.name
  role   = "roles/storage.objectAdmin"
  member = "serviceAccount:${google_service_account.deploy.email}"
}

resource "google_iam_workload_identity_pool" "github" {
  project                   = var.project_id
  workload_identity_pool_id = local.workload_identity_pool_id
  display_name              = "Aurea Link GitHub Actions"
  description               = "GitHub OIDC identities used by the Aurea Link production workflow"

  depends_on = [google_project_service.bootstrap["iam.googleapis.com"]]

  lifecycle {
    prevent_destroy = true
  }
}

resource "google_iam_workload_identity_pool_provider" "github" {
  project                            = var.project_id
  workload_identity_pool_id          = google_iam_workload_identity_pool.github.workload_identity_pool_id
  workload_identity_pool_provider_id = local.workload_provider_id
  display_name                       = "Aurea Link GitHub repository"
  description                        = "Trusts only ${var.github_repository} main-branch workflow tokens"

  attribute_mapping = {
    "google.subject"          = "assertion.sub"
    "attribute.repository"    = "assertion.repository"
    "attribute.repository_id" = "assertion.repository_id"
    "attribute.ref"           = "assertion.ref"
  }
  attribute_condition = "assertion.repository_id == '${var.github_repository_id}' && assertion.ref == '${local.production_ref}'"

  oidc {
    issuer_uri = "https://token.actions.githubusercontent.com"
  }

  lifecycle {
    prevent_destroy = true
  }
}

resource "google_service_account_iam_member" "github_deploy" {
  service_account_id = google_service_account.deploy.name
  role               = "roles/iam.workloadIdentityUser"
  member             = "principalSet://iam.googleapis.com/${google_iam_workload_identity_pool.github.name}/attribute.repository_id/${var.github_repository_id}"
}

output "state_bucket_name" {
  value = google_storage_bucket.terraform_state.name
}

output "github_workload_identity_provider" {
  description = "Provider resource name used by google-github-actions/auth."
  value       = google_iam_workload_identity_pool_provider.github.name
}

output "github_deploy_service_account" {
  description = "Service account impersonated by the production GitHub Actions workflow."
  value       = google_service_account.deploy.email
}
