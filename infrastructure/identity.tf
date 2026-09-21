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

resource "google_service_account_iam_member" "api_self_token_creator" {
  service_account_id = google_service_account.api.name
  role               = "roles/iam.serviceAccountTokenCreator"
  member             = "serviceAccount:${google_service_account.api.email}"
}

resource "google_project_iam_member" "api_cloud_sql_client" {
  project = var.project_id
  role    = "roles/cloudsql.client"
  member  = "serviceAccount:${google_service_account.api.email}"
}

resource "google_identity_platform_config" "authentication" {
  provider = google-beta
  project  = var.project_id

  authorized_domains = local.firebase_authorized_domains

  depends_on = [google_project_service.required["identitytoolkit.googleapis.com"]]
}
