resource "google_compute_network" "application" {
  project                 = var.project_id
  name                    = local.network_name
  description             = "Eterealink production network"
  auto_create_subnetworks = false
  routing_mode            = "REGIONAL"

  depends_on = [google_project_service.required["compute.googleapis.com"]]
}

resource "google_compute_subnetwork" "cloud_run" {
  project                  = var.project_id
  name                     = local.subnet_name
  description              = "Direct VPC egress for Eterealink Cloud Run workloads"
  region                   = var.region
  network                  = google_compute_network.application.id
  ip_cidr_range            = local.subnet_cidr
  private_ip_google_access = true
}

resource "google_compute_global_address" "private_services" {
  project       = var.project_id
  name          = local.private_services_range
  description   = "Private Services Access range for Eterealink"
  purpose       = "VPC_PEERING"
  address_type  = "INTERNAL"
  address       = split("/", local.private_services_cidr)[0]
  prefix_length = tonumber(split("/", local.private_services_cidr)[1])
  network       = google_compute_network.application.id
}

resource "google_service_networking_connection" "private_services" {
  network                 = google_compute_network.application.id
  service                 = "servicenetworking.googleapis.com"
  reserved_peering_ranges = [google_compute_global_address.private_services.name]

  depends_on = [google_project_service.required["servicenetworking.googleapis.com"]]
}
