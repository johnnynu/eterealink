resource "google_cloud_run_v2_job" "migrations" {
  project             = var.project_id
  name                = local.migration_job_name
  location            = var.region
  deletion_protection = true

  template {
    template {
      service_account       = google_service_account.migrations.email
      timeout               = "600s"
      max_retries           = 1
      execution_environment = "EXECUTION_ENVIRONMENT_GEN2"

      containers {
        image   = local.api_image
        command = ["/app/migrate"]
        args    = ["up"]

        resources {
          limits = {
            cpu    = "1000m"
            memory = "512Mi"
          }
        }

        env {
          name  = "APP_ENV"
          value = "production"
        }

        env {
          name  = "DATABASE_CREDENTIALS_VERSION"
          value = tostring(var.database_credentials_version)
        }

        env {
          name = "DATABASE_URL"
          value_source {
            secret_key_ref {
              secret  = google_secret_manager_secret.database_url.secret_id
              version = google_secret_manager_secret_version.database_url.version
            }
          }
        }
      }

      vpc_access {
        egress = "PRIVATE_RANGES_ONLY"
        network_interfaces {
          network    = google_compute_network.application.name
          subnetwork = google_compute_subnetwork.cloud_run.name
        }
      }
    }
  }

  depends_on = [
    google_artifact_registry_repository.application,
    google_project_service.required["run.googleapis.com"],
    google_secret_manager_secret_iam_member.migrations_database_url,
    google_secret_manager_secret_version.database_url,
  ]

  lifecycle {
    ignore_changes = [client, client_version]
  }
}

resource "google_cloud_run_v2_job" "cleanup" {
  project             = var.project_id
  name                = local.cleanup_job_name
  location            = var.region
  deletion_protection = true

  template {
    template {
      service_account       = google_service_account.cleanup.email
      timeout               = "600s"
      max_retries           = 2
      execution_environment = "EXECUTION_ENVIRONMENT_GEN2"

      containers {
        image   = local.api_image
        command = ["/app/cleanup"]

        resources {
          limits = {
            cpu    = "1000m"
            memory = "512Mi"
          }
        }

        env {
          name  = "APP_ENV"
          value = "production"
        }

        env {
          name  = "GCS_BUCKET"
          value = google_storage_bucket.files.name
        }

        env {
          name  = "CLEANUP_BATCH_SIZE"
          value = "100"
        }

        env {
          name  = "DATABASE_CREDENTIALS_VERSION"
          value = tostring(var.database_credentials_version)
        }

        env {
          name = "DATABASE_URL"
          value_source {
            secret_key_ref {
              secret  = google_secret_manager_secret.database_url.secret_id
              version = google_secret_manager_secret_version.database_url.version
            }
          }
        }
      }

      vpc_access {
        egress = "PRIVATE_RANGES_ONLY"
        network_interfaces {
          network    = google_compute_network.application.name
          subnetwork = google_compute_subnetwork.cloud_run.name
        }
      }
    }
  }

  depends_on = [
    google_artifact_registry_repository.application,
    google_project_service.required["run.googleapis.com"],
    google_secret_manager_secret_iam_member.cleanup_database_url,
    google_secret_manager_secret_version.database_url,
    google_storage_bucket_iam_member.cleanup_object_delete,
  ]

  lifecycle {
    ignore_changes = [client, client_version]
  }
}

resource "google_cloud_run_v2_job_iam_member" "cleanup_invoker" {
  project  = var.project_id
  location = var.region
  name     = google_cloud_run_v2_job.cleanup.name
  role     = "roles/run.invoker"
  member   = "serviceAccount:${google_service_account.cleanup.email}"
}

resource "google_cloud_scheduler_job" "cleanup" {
  project          = var.project_id
  region           = var.region
  name             = local.cleanup_scheduler_name
  description      = "Delete expired anonymous Aurea Link objects and metadata."
  schedule         = "17 * * * *"
  time_zone        = "Etc/UTC"
  attempt_deadline = "180s"

  retry_config {
    retry_count          = 3
    min_backoff_duration = "30s"
    max_backoff_duration = "300s"
    max_doublings        = 3
  }

  http_target {
    http_method = "POST"
    uri         = "https://run.googleapis.com/v2/projects/${var.project_id}/locations/${var.region}/jobs/${google_cloud_run_v2_job.cleanup.name}:run"

    oauth_token {
      service_account_email = google_service_account.cleanup.email
      scope                 = "https://www.googleapis.com/auth/cloud-platform"
    }
  }

  depends_on = [
    google_cloud_run_v2_job_iam_member.cleanup_invoker,
    google_project_service.required["cloudscheduler.googleapis.com"],
  ]
}

resource "google_cloud_run_v2_service" "api" {
  project             = var.project_id
  name                = local.api_service_name
  location            = var.region
  deletion_protection = true
  ingress             = "INGRESS_TRAFFIC_ALL"

  template {
    service_account                  = google_service_account.api.email
    timeout                          = "${var.request_timeout_seconds}s"
    max_instance_request_concurrency = 40

    scaling {
      min_instance_count = 0
      max_instance_count = 3
    }

    containers {
      image = local.api_image

      ports {
        container_port = 8080
      }

      resources {
        limits = {
          cpu    = "1"
          memory = "512Mi"
        }
        cpu_idle          = true
        startup_cpu_boost = true
      }

      dynamic "env" {
        for_each = local.api_environment
        content {
          name  = env.key
          value = env.value
        }
      }

      env {
        name = "DATABASE_URL"
        value_source {
          secret_key_ref {
            secret  = google_secret_manager_secret.database_url.secret_id
            version = google_secret_manager_secret_version.database_url.version
          }
        }
      }

      startup_probe {
        failure_threshold = 12
        period_seconds    = 5
        timeout_seconds   = 3
        http_get {
          path = "/readyz"
          port = 8080
        }
      }

      liveness_probe {
        failure_threshold = 3
        period_seconds    = 10
        timeout_seconds   = 3
        http_get {
          path = "/health"
          port = 8080
        }
      }
    }

    vpc_access {
      egress = "PRIVATE_RANGES_ONLY"
      network_interfaces {
        network    = google_compute_network.application.name
        subnetwork = google_compute_subnetwork.cloud_run.name
      }
    }
  }

  traffic {
    type    = "TRAFFIC_TARGET_ALLOCATION_TYPE_LATEST"
    percent = 100
  }

  depends_on = [
    google_artifact_registry_repository.application,
    google_project_service.required["run.googleapis.com"],
    google_secret_manager_secret_iam_member.api_database_url,
    google_secret_manager_secret_version.database_url,
  ]

  lifecycle {
    ignore_changes = [client, client_version]
  }
}

resource "google_cloud_run_v2_service" "frontend" {
  project             = var.project_id
  name                = local.frontend_service_name
  location            = var.region
  deletion_protection = true
  ingress             = "INGRESS_TRAFFIC_ALL"

  template {
    service_account                  = google_service_account.frontend.email
    timeout                          = "${var.request_timeout_seconds}s"
    max_instance_request_concurrency = 80

    scaling {
      min_instance_count = 0
      max_instance_count = 3
    }

    containers {
      image = local.frontend_image

      ports {
        container_port = 3000
      }

      resources {
        limits = {
          cpu    = "1"
          memory = "512Mi"
        }
        cpu_idle          = true
        startup_cpu_boost = true
      }

      dynamic "env" {
        for_each = local.frontend_environment
        content {
          name  = env.key
          value = env.value
        }
      }

      startup_probe {
        failure_threshold = 12
        period_seconds    = 5
        timeout_seconds   = 3
        http_get {
          path = "/health"
          port = 3000
        }
      }

      liveness_probe {
        failure_threshold = 3
        period_seconds    = 10
        timeout_seconds   = 3
        http_get {
          path = "/health"
          port = 3000
        }
      }
    }
  }

  traffic {
    type    = "TRAFFIC_TARGET_ALLOCATION_TYPE_LATEST"
    percent = 100
  }

  depends_on = [
    google_artifact_registry_repository.application,
    google_project_service.required["run.googleapis.com"],
  ]

  lifecycle {
    ignore_changes = [client, client_version]
  }
}

resource "google_cloud_run_v2_service_iam_member" "api_public" {
  project  = var.project_id
  location = var.region
  name     = google_cloud_run_v2_service.api.name
  role     = "roles/run.invoker"
  member   = "allUsers"
}

resource "google_cloud_run_v2_service_iam_member" "frontend_public" {
  project  = var.project_id
  location = var.region
  name     = google_cloud_run_v2_service.frontend.name
  role     = "roles/run.invoker"
  member   = "allUsers"
}
