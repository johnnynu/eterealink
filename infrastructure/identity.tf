resource "google_service_account" "api" {
  project      = var.project_id
  account_id   = local.runtime_account_id
  display_name = "Eterealink API"
  description  = "Runtime identity for the Eterealink Go API"
}

resource "google_service_account" "frontend" {
  project      = var.project_id
  account_id   = local.frontend_account_id
  display_name = "Eterealink web"
  description  = "Runtime identity for the Eterealink Next.js frontend"
}

resource "google_service_account" "migrations" {
  project      = var.project_id
  account_id   = local.migration_account_id
  display_name = "Aurea Link migrations"
  description  = "Database-only identity for the Aurea Link migration job"
}

resource "google_project_iam_custom_role" "signed_url_creator" {
  project     = var.project_id
  role_id     = "aureaLinkSignedURLCreator"
  title       = "Aurea Link Signed URL Creator"
  description = "Allows the API identity to sign Cloud Storage URLs without token impersonation permissions."
  permissions = ["iam.serviceAccounts.signBlob"]
}

resource "google_project_iam_custom_role" "object_runtime" {
  project     = var.project_id
  role_id     = "aureaLinkObjectRuntime"
  title       = "Aurea Link Object Runtime"
  description = "Allows the API to create, read, and delete application objects without bucket administration."
  permissions = [
    "storage.objects.create",
    "storage.objects.delete",
    "storage.objects.get",
  ]
}

resource "google_service_account_iam_member" "api_self_signer" {
  service_account_id = google_service_account.api.name
  role               = google_project_iam_custom_role.signed_url_creator.id
  member             = "serviceAccount:${google_service_account.api.email}"

  lifecycle {
    create_before_destroy = true
  }
}

moved {
  from = google_service_account_iam_member.api_self_token_creator
  to   = google_service_account_iam_member.api_self_signer
}

resource "google_identity_platform_config" "authentication" {
  provider = google-beta
  project  = var.project_id

  authorized_domains = local.firebase_authorized_domains

  depends_on = [google_project_service.required["identitytoolkit.googleapis.com"]]
}
