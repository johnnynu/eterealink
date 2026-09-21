resource "google_storage_bucket" "files" {
  project                     = var.project_id
  name                        = local.bucket_name
  location                    = var.region
  storage_class               = "STANDARD"
  uniform_bucket_level_access = true
  public_access_prevention    = "enforced"
  force_destroy               = false

  soft_delete_policy {
    retention_duration_seconds = 0
  }

  cors {
    origin = local.cors_origins
    method = ["GET", "HEAD", "POST", "PUT"]
    response_header = [
      "Content-Type",
      "Content-Length",
      "Content-Range",
      "ETag",
      "Location",
      "Range",
      "X-Goog-Resumable",
      "X-Goog-If-Generation-Match",
    ]
    max_age_seconds = 3600
  }

  lifecycle {
    prevent_destroy = true
  }

  depends_on = [google_project_service.required["storage.googleapis.com"]]
}

resource "google_storage_bucket_iam_member" "api_object_runtime" {
  bucket = google_storage_bucket.files.name
  role   = google_project_iam_custom_role.object_runtime.id
  member = "serviceAccount:${google_service_account.api.email}"

  lifecycle {
    create_before_destroy = true
  }
}

moved {
  from = google_storage_bucket_iam_member.api_object_user
  to   = google_storage_bucket_iam_member.api_object_runtime
}
