variable "project_id" {
  description = "Google Cloud project that stores Terraform state."
  type        = string
  default     = "eterealink"
}

variable "region" {
  description = "Region for the Terraform state bucket."
  type        = string
  default     = "us-west1"
}

variable "state_bucket_name" {
  description = "Globally unique bucket used by the root Terraform backend."
  type        = string
  default     = "eterealink-terraform-state"
}

resource "google_storage_bucket" "terraform_state" {
  project                     = var.project_id
  name                        = var.state_bucket_name
  location                    = var.region
  storage_class               = "STANDARD"
  uniform_bucket_level_access = true
  public_access_prevention    = "enforced"
  force_destroy               = false

  versioning {
    enabled = true
  }

  lifecycle {
    prevent_destroy = true
  }
}

output "state_bucket_name" {
  value = google_storage_bucket.terraform_state.name
}
