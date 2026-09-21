output "api_url" {
  description = "Public Cloud Run API URL."
  value       = google_cloud_run_v2_service.api.uri
}

output "frontend_url" {
  description = "Public Cloud Run frontend URL."
  value       = google_cloud_run_v2_service.frontend.uri
}

output "database_private_address" {
  description = "Private Cloud SQL address used by Cloud Run."
  value       = google_sql_database_instance.application.private_ip_address
}

output "artifact_registry_repository" {
  description = "Artifact Registry repository resource name."
  value       = google_artifact_registry_repository.application.name
}

output "migration_service_account" {
  description = "Least-privilege migration job identity."
  value       = google_service_account.migrations.email
}

output "domain_dns_records" {
  description = "DNS records reported by the Cloud Run domain mappings."
  value = {
    for domain, mapping in google_cloud_run_domain_mapping.frontend :
    domain => mapping.status[0].resource_records
  }
}
