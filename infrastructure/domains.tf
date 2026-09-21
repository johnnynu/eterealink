resource "google_cloud_run_domain_mapping" "frontend" {
  for_each = var.custom_domains

  project         = var.project_id
  location        = var.region
  name            = each.value
  deletion_policy = "ABANDON"

  metadata {
    namespace = var.project_id
  }

  spec {
    route_name = google_cloud_run_v2_service.frontend.name
  }

  lifecycle {
    ignore_changes = [spec[0].certificate_mode]
  }
}
