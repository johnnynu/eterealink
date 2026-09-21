variable "project_id" {
  description = "Google Cloud project that hosts Aurea Link."
  type        = string
  default     = "eterealink"
}

variable "region" {
  description = "Google Cloud region for regional resources."
  type        = string
  default     = "us-west1"
}

variable "api_image_tag" {
  description = "Immutable Artifact Registry tag for the API and migration image."
  type        = string

  validation {
    condition     = length(var.api_image_tag) > 0 && var.api_image_tag != "latest"
    error_message = "api_image_tag must identify an immutable image and cannot be latest."
  }
}

variable "frontend_image_tag" {
  description = "Immutable Artifact Registry tag for the frontend image."
  type        = string

  validation {
    condition     = length(var.frontend_image_tag) > 0 && var.frontend_image_tag != "latest"
    error_message = "frontend_image_tag must identify an immutable image and cannot be latest."
  }
}

variable "database_password" {
  description = "Password written to Cloud SQL and Secret Manager. Supply it through TF_VAR_database_password."
  type        = string
  sensitive   = true
  ephemeral   = true
}

variable "database_credentials_version" {
  description = "Monotonic version that triggers a database password and secret rotation."
  type        = number
  default     = 1

  validation {
    condition     = var.database_credentials_version >= 1 && floor(var.database_credentials_version) == var.database_credentials_version
    error_message = "database_credentials_version must be a positive integer."
  }
}

variable "custom_domains" {
  description = "Verified domains mapped to the public frontend."
  type        = set(string)
  default = [
    "aurealink.app",
    "www.aurealink.app",
    "eterealink.com",
    "www.eterealink.com",
  ]
}

variable "canonical_host" {
  description = "Canonical host used by the frontend for redirects and metadata."
  type        = string
  default     = "aurealink.app"
}

variable "legacy_hosts" {
  description = "Hosts that the frontend redirects to the canonical domain."
  type        = list(string)
  default     = ["eterealink.com", "www.eterealink.com"]
}

variable "firebase_default_domains" {
  description = "Firebase-managed and local domains retained for authentication."
  type        = set(string)
  default = [
    "127.0.0.1",
    "eterealink.firebaseapp.com",
    "eterealink.web.app",
    "localhost",
  ]
}

variable "request_timeout_seconds" {
  description = "Cloud Run request timeout; must exceed the API SSE timeout."
  type        = number
  default     = 300
}
