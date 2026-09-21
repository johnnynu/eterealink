resource "google_sql_database_instance" "application" {
  project             = var.project_id
  name                = local.database_instance_name
  region              = var.region
  database_version    = "POSTGRES_17"
  deletion_protection = true

  settings {
    tier                        = "db-f1-micro"
    edition                     = "ENTERPRISE"
    availability_type           = "ZONAL"
    disk_type                   = "PD_HDD"
    disk_size                   = 10
    disk_autoresize             = true
    disk_autoresize_limit       = 20
    enable_dataplex_integration = true

    backup_configuration {
      enabled = false
    }

    ip_configuration {
      ipv4_enabled    = false
      private_network = google_compute_network.application.id
      ssl_mode        = "ALLOW_UNENCRYPTED_AND_ENCRYPTED"
    }
  }

  lifecycle {
    prevent_destroy = true
  }

  depends_on = [
    google_project_service.required["sqladmin.googleapis.com"],
    google_service_networking_connection.private_services,
  ]
}

resource "google_sql_database" "application" {
  project   = var.project_id
  name      = local.database_name
  instance  = google_sql_database_instance.application.name
  charset   = "UTF8"
  collation = "en_US.UTF8"
}

resource "google_sql_user" "application" {
  project             = var.project_id
  name                = local.database_user
  instance            = google_sql_database_instance.application.name
  type                = "BUILT_IN"
  password_wo         = var.database_password
  password_wo_version = var.database_credentials_version
  deletion_policy     = "ABANDON"
}

resource "google_secret_manager_secret" "database_url" {
  project   = var.project_id
  secret_id = local.database_secret_id

  replication {
    auto {}
  }

  lifecycle {
    prevent_destroy = true
  }

  depends_on = [google_project_service.required["secretmanager.googleapis.com"]]
}

resource "google_secret_manager_secret_version" "database_url" {
  secret                 = google_secret_manager_secret.database_url.id
  secret_data_wo         = "postgres://${local.database_user}:${urlencode(var.database_password)}@${google_sql_database_instance.application.private_ip_address}:5432/${local.database_name}?sslmode=disable&connect_timeout=10"
  secret_data_wo_version = var.database_credentials_version
  deletion_policy        = "ABANDON"

  lifecycle {
    create_before_destroy = true
  }

  depends_on = [google_sql_user.application]
}

resource "google_secret_manager_secret_iam_member" "api_database_url" {
  project   = var.project_id
  secret_id = google_secret_manager_secret.database_url.secret_id
  role      = "roles/secretmanager.secretAccessor"
  member    = "serviceAccount:${google_service_account.api.email}"
}

resource "google_secret_manager_secret_iam_member" "migrations_database_url" {
  project   = var.project_id
  secret_id = google_secret_manager_secret.database_url.secret_id
  role      = "roles/secretmanager.secretAccessor"
  member    = "serviceAccount:${google_service_account.migrations.email}"
}
