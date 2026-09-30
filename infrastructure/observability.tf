resource "google_monitoring_uptime_check_config" "frontend" {
  project            = var.project_id
  display_name       = "Aurea Link frontend"
  checker_type       = "STATIC_IP_CHECKERS"
  period             = "300s"
  timeout            = "10s"
  log_check_failures = true

  http_check {
    path           = "/health"
    port           = 443
    request_method = "GET"
    use_ssl        = true
    validate_ssl   = true
  }

  monitored_resource {
    type = "uptime_url"
    labels = {
      host       = var.canonical_host
      project_id = var.project_id
    }
  }

  content_matchers {
    content = "ok"
    matcher = "CONTAINS_STRING"
  }

  user_labels = {
    application = "aurea-link"
    environment = "production"
  }

  depends_on = [google_project_service.required["monitoring.googleapis.com"]]
}

resource "google_monitoring_notification_channel" "email" {
  count = var.alert_notification_email == "" ? 0 : 1

  project      = var.project_id
  display_name = "Aurea Link production alerts"
  description  = "Email notifications for production availability incidents"
  type         = "email"
  labels = {
    email_address = var.alert_notification_email
  }

  depends_on = [google_project_service.required["monitoring.googleapis.com"]]
}

resource "google_monitoring_alert_policy" "frontend_unavailable" {
  project      = var.project_id
  display_name = "Aurea Link frontend unavailable"
  combiner     = "OR"
  enabled      = true
  severity     = "CRITICAL"

  notification_channels = google_monitoring_notification_channel.email[*].name

  conditions {
    display_name = "A majority of uptime probes fail"

    condition_threshold {
      filter          = "metric.type=\"monitoring.googleapis.com/uptime_check/check_passed\" AND metric.label.check_id=\"${google_monitoring_uptime_check_config.frontend.uptime_check_id}\" AND resource.type=\"uptime_url\""
      comparison      = "COMPARISON_LT"
      threshold_value = 0.5
      duration        = "300s"

      aggregations {
        alignment_period     = "300s"
        per_series_aligner   = "ALIGN_FRACTION_TRUE"
        cross_series_reducer = "REDUCE_MEAN"
      }

      trigger {
        count = 1
      }
    }
  }

  alert_strategy {
    auto_close           = "1800s"
    notification_prompts = ["OPENED", "CLOSED"]
  }

  documentation {
    mime_type = "text/markdown"
    subject   = "Aurea Link production health check is failing"
    content   = <<-EOT
      The public frontend health endpoint failed from a majority of Google Cloud probe locations for five minutes.

      1. Run `curl --fail --show-error https://${var.canonical_host}/health` from a separate network.
      2. Inspect recent `eterealink-web` Cloud Run request and container logs, then check the active revision and instance count.
      3. If the Cloud Run URL is healthy, inspect the custom-domain mapping, DNS, and managed certificate.
      4. If failures began with a release, roll back by redeploying the last known-good commit through the production workflow.
    EOT
  }

  user_labels = {
    application = "aurea-link"
    environment = "production"
  }

  depends_on = [google_project_service.required["monitoring.googleapis.com"]]
}

resource "google_monitoring_dashboard" "application" {
  project = var.project_id
  dashboard_json = jsonencode({
    displayName = "Aurea Link Production"
    labels = {
      application = "aurea-link"
      managed_by  = "terraform"
    }
    mosaicLayout = {
      columns = 48
      tiles = [
        {
          xPos   = 0
          yPos   = 0
          width  = 48
          height = 4
          widget = {
            title = "Operations guide"
            text = {
              format  = "MARKDOWN"
              content = "This dashboard covers public availability, Cloud Run traffic, server errors, latency, capacity, and recent application errors. Start with the uptime and 5xx charts, then correlate a failing request by `request_id` or Cloud Trace ID in the logs panel."
            }
          }
        },
        {
          xPos   = 0
          yPos   = 4
          width  = 24
          height = 16
          widget = {
            title = "Public frontend availability"
            xyChart = {
              dataSets = [{
                plotType   = "LINE"
                targetAxis = "Y1"
                timeSeriesQuery = {
                  timeSeriesFilter = {
                    filter = "metric.type=\"monitoring.googleapis.com/uptime_check/check_passed\" AND metric.label.check_id=\"${google_monitoring_uptime_check_config.frontend.uptime_check_id}\" AND resource.type=\"uptime_url\""
                    aggregation = {
                      alignmentPeriod    = "300s"
                      perSeriesAligner   = "ALIGN_FRACTION_TRUE"
                      crossSeriesReducer = "REDUCE_MEAN"
                    }
                  }
                }
              }]
              yAxis = {
                label = "Fraction passing"
                scale = "LINEAR"
              }
            }
          }
        },
        {
          xPos   = 24
          yPos   = 4
          width  = 24
          height = 16
          widget = {
            title = "Cloud Run request rate by service"
            xyChart = {
              dataSets = [{
                plotType   = "LINE"
                targetAxis = "Y1"
                timeSeriesQuery = {
                  timeSeriesFilter = {
                    filter = "metric.type=\"run.googleapis.com/request_count\" AND resource.type=\"cloud_run_revision\" AND (resource.label.service_name=\"${local.api_service_name}\" OR resource.label.service_name=\"${local.frontend_service_name}\")"
                    aggregation = {
                      alignmentPeriod    = "60s"
                      perSeriesAligner   = "ALIGN_RATE"
                      crossSeriesReducer = "REDUCE_SUM"
                      groupByFields      = ["resource.label.service_name"]
                    }
                  }
                }
              }]
              yAxis = {
                label = "Requests / second"
                scale = "LINEAR"
              }
            }
          }
        },
        {
          xPos   = 0
          yPos   = 20
          width  = 24
          height = 16
          widget = {
            title = "Cloud Run 5xx rate by service"
            xyChart = {
              dataSets = [{
                plotType   = "LINE"
                targetAxis = "Y1"
                timeSeriesQuery = {
                  timeSeriesFilter = {
                    filter = "metric.type=\"run.googleapis.com/request_count\" AND metric.label.response_code_class=\"5xx\" AND resource.type=\"cloud_run_revision\" AND (resource.label.service_name=\"${local.api_service_name}\" OR resource.label.service_name=\"${local.frontend_service_name}\")"
                    aggregation = {
                      alignmentPeriod    = "60s"
                      perSeriesAligner   = "ALIGN_RATE"
                      crossSeriesReducer = "REDUCE_SUM"
                      groupByFields      = ["resource.label.service_name"]
                    }
                  }
                }
              }]
              yAxis = {
                label = "Errors / second"
                scale = "LINEAR"
              }
            }
          }
        },
        {
          xPos   = 24
          yPos   = 20
          width  = 24
          height = 16
          widget = {
            title = "Cloud Run p95 request latency by service"
            xyChart = {
              dataSets = [{
                plotType   = "LINE"
                targetAxis = "Y1"
                timeSeriesQuery = {
                  timeSeriesFilter = {
                    filter = "metric.type=\"run.googleapis.com/request_latencies\" AND resource.type=\"cloud_run_revision\" AND (resource.label.service_name=\"${local.api_service_name}\" OR resource.label.service_name=\"${local.frontend_service_name}\")"
                    aggregation = {
                      alignmentPeriod    = "60s"
                      perSeriesAligner   = "ALIGN_PERCENTILE_95"
                      crossSeriesReducer = "REDUCE_PERCENTILE_95"
                      groupByFields      = ["resource.label.service_name"]
                    }
                  }
                }
              }]
              yAxis = {
                label = "Milliseconds"
                scale = "LINEAR"
              }
            }
          }
        },
        {
          xPos   = 0
          yPos   = 36
          width  = 24
          height = 16
          widget = {
            title = "Cloud Run instances by service"
            xyChart = {
              dataSets = [{
                plotType   = "STACKED_AREA"
                targetAxis = "Y1"
                timeSeriesQuery = {
                  timeSeriesFilter = {
                    filter = "metric.type=\"run.googleapis.com/container/instance_count\" AND resource.type=\"cloud_run_revision\" AND (resource.label.service_name=\"${local.api_service_name}\" OR resource.label.service_name=\"${local.frontend_service_name}\")"
                    aggregation = {
                      alignmentPeriod    = "60s"
                      perSeriesAligner   = "ALIGN_MEAN"
                      crossSeriesReducer = "REDUCE_SUM"
                      groupByFields      = ["resource.label.service_name"]
                    }
                  }
                }
              }]
              yAxis = {
                label = "Instances"
                scale = "LINEAR"
              }
            }
          }
        },
        {
          xPos   = 24
          yPos   = 36
          width  = 24
          height = 16
          widget = {
            title = "Recent application errors"
            logsPanel = {
              filter        = "resource.type=\"cloud_run_revision\" AND (resource.labels.service_name=\"${local.api_service_name}\" OR resource.labels.service_name=\"${local.frontend_service_name}\") AND severity>=ERROR"
              resourceNames = ["projects/${var.project_id}"]
            }
          }
        }
      ]
    }
  })

  depends_on = [
    google_project_service.required["logging.googleapis.com"],
    google_project_service.required["monitoring.googleapis.com"],
  ]
}
