# Phase 13 production observability

Phase 13 adds a public HTTPS uptime check, a sustained-availability alert, an operations dashboard, and Cloud Logging-compatible structured API request logs. Terraform owns the monitoring resources, and the deployment verifier confirms both their configuration and a live correlated request log.

## Grant the deployment identity access

The bootstrap root now creates and grants one project custom role to the keyless GitHub deployment identity. It contains only the alert-policy, dashboard, notification-channel, and uptime-check CRUD permissions that Terraform needs, plus `logging.logEntries.list` for deployment verification. Apply this bootstrap change once with administrator credentials before merging the production-root changes:

```bash
gcloud auth application-default login
gcloud auth application-default set-quota-project eterealink
terraform -chdir=infrastructure/bootstrap init
terraform -chdir=infrastructure/bootstrap plan -out=phase13-bootstrap.tfplan
terraform -chdir=infrastructure/bootstrap show phase13-bootstrap.tfplan
terraform -chdir=infrastructure/bootstrap apply phase13-bootstrap.tfplan
```

## Configure alert delivery

The critical availability policy is always created. To also deliver incidents by email, add the optional GitHub Actions repository or `production` environment variable `ALERT_NOTIFICATION_EMAIL`. The deployment workflow maps it to Terraform's `alert_notification_email` input. After the first apply, complete the verification message that Cloud Monitoring sends to that address.

For a local apply, set the input directly:

```bash
export TF_VAR_alert_notification_email='you@example.com'
```

Leaving the value empty keeps the policy and dashboard active without creating an email channel. Additional notification systems can be attached to the policy in Cloud Monitoring and then brought under Terraform management if they become necessary.

## Apply and verify

Review the production plan normally. The apply enables the Logging and Monitoring APIs and creates the uptime check, alert policy, optional notification channel, and dashboard. A main-branch deployment now runs the cumulative Phase 13 verifier.

To verify an already deployed commit locally, authenticate with Google Cloud, provide the same image, database, and optional alert-email inputs used for the deployment, and run:

```bash
IMAGE_TAG="$(git rev-parse HEAD)" make phase13-verify
```

The verifier checks the live monitoring resources, calls the API readiness endpoint, waits up to 60 seconds for the structured request entry to reach Cloud Logging, confirms request/trace correlation, and reruns every Phase 12 check.

## Use the dashboard and logs

Open **Google Cloud Console → Monitoring → Dashboards → Aurea Link Production**. The dashboard is intended to answer these questions in order:

1. Is the public domain healthy from multiple locations?
2. Did traffic, 5xx responses, or p95 latency change?
3. Did Cloud Run add instances or fail to serve traffic?
4. Which recent application error explains the change?

API request logs contain `request_id`, `method`, `path`, `status`, `response_bytes`, `duration_ms`, the structured `httpRequest` field, and Cloud Trace correlation when the request passed through Google Cloud's frontend. A focused Logs Explorer query is:

```text
resource.type="cloud_run_revision"
resource.labels.service_name="eterealink-api"
jsonPayload.request_id="REQUEST_ID"
```

Successful liveness probes are intentionally absent from application stdout logs. Cloud Run's platform request log still records them, and failed health checks continue to produce application detail.
