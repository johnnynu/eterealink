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

output "cleanup_service_account" {
  description = "Least-privilege lifecycle cleanup identity."
  value       = google_service_account.cleanup.email
}

output "cleanup_job" {
  description = "Cloud Run job that removes expired anonymous content."
  value       = google_cloud_run_v2_job.cleanup.name
}

output "cleanup_schedule" {
  description = "Hourly Cloud Scheduler trigger for lifecycle cleanup."
  value       = google_cloud_scheduler_job.cleanup.id
}

output "domain_dns_records" {
  description = "DNS records reported by the Cloud Run domain mappings."
  value = {
    for domain, mapping in google_cloud_run_domain_mapping.frontend :
    domain => mapping.status[0].resource_records
  }
}

output "monitoring_dashboard" {
  description = "Terraform-managed production monitoring dashboard resource."
  value       = google_monitoring_dashboard.application.id
}

output "frontend_uptime_check" {
  description = "Public frontend uptime check resource."
  value       = google_monitoring_uptime_check_config.frontend.name
}

output "frontend_availability_alert" {
  description = "Alert policy for sustained public frontend failures."
  value       = google_monitoring_alert_policy.frontend_unavailable.name
}
