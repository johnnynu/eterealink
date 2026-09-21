data "google_project" "current" {
  project_id = var.project_id
}

locals {
  repository_name          = "eterealink"
  api_service_name         = "eterealink-api"
  frontend_service_name    = "eterealink-web"
  migration_job_name       = "eterealink-migrate"
  runtime_account_id       = "eterealink-api"
  frontend_account_id      = "eterealink-web"
  bucket_name              = "eterealink-files"
  database_instance_name   = "eterealink-db"
  database_name            = "eterealink"
  database_user            = "eterealink"
  database_secret_id       = "eterealink-database-url"
  network_name             = "eterealink"
  subnet_name              = "eterealink-us-west1"
  subnet_cidr              = "10.0.1.0/24"
  private_services_range   = "eterealink-google-managed-services"
  private_services_cidr    = "10.10.0.0/24"
  api_service_account      = "${local.runtime_account_id}@${var.project_id}.iam.gserviceaccount.com"
  frontend_service_account = "${local.frontend_account_id}@${var.project_id}.iam.gserviceaccount.com"

  api_image      = "${var.region}-docker.pkg.dev/${var.project_id}/${local.repository_name}/api:${var.api_image_tag}"
  frontend_image = "${var.region}-docker.pkg.dev/${var.project_id}/${local.repository_name}/frontend:${var.frontend_image_tag}"
  api_public_url = "https://${local.api_service_name}-${data.google_project.current.number}.${var.region}.run.app"
  web_public_url = "https://${local.frontend_service_name}-${data.google_project.current.number}.${var.region}.run.app"

  api_environment = {
    APP_ENV                      = "production"
    HTTP_ADDR                    = ":8080"
    STORAGE_BACKEND              = "gcs"
    GCS_BUCKET                   = local.bucket_name
    GCS_SIGNING_SERVICE_ACCOUNT  = local.api_service_account
    FIREBASE_PROJECT_ID          = var.project_id
    ANONYMOUS_FILE_TTL           = "24h"
    SIGNED_URL_TTL               = "15m"
    MAX_ANONYMOUS_FILE_BYTES     = "1073741824"
    MAX_PERSISTENT_STORAGE_BYTES = "26843545600"
    MAX_ANONYMOUS_TRANSFER_BYTES = "1073741824"
    MAX_ANONYMOUS_FILES          = "10"
    DATABASE_CREDENTIALS_VERSION = tostring(var.database_credentials_version)
  }

  frontend_environment = {
    API_BASE_URL   = local.api_public_url
    CANONICAL_HOST = var.canonical_host
    LEGACY_HOSTS   = join(",", var.legacy_hosts)
  }

  cors_origins = sort(tolist(setunion(
    toset([for domain in var.custom_domains : "https://${domain}"]),
    [
      "http://127.0.0.1:3000",
      "http://localhost:3000",
      local.web_public_url,
    ],
  )))

  firebase_authorized_domains = sort(tolist(setunion(
    var.firebase_default_domains,
    var.custom_domains,
    [trimprefix(local.web_public_url, "https://")],
  )))
}
