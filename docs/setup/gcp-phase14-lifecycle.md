# Phase 14 lifecycle automation

Phase 14 adds a dedicated Cloud Run job that removes expired anonymous objects before deleting their PostgreSQL metadata. Cloud Scheduler starts it at minute 17 of every hour, Cloud Run retries transient task failures twice, and Scheduler retries a failed invocation three times.

## Grant deployment access

The bootstrap root now creates and grants a custom Scheduler deployment role to the keyless deployment identity. It contains only job create, read, update, pause/enable, and delete permissions plus location discovery; it cannot manually run arbitrary Scheduler jobs. Apply this bootstrap change with administrator credentials before merging the production-root changes:

```bash
gcloud auth application-default login
gcloud auth application-default set-quota-project eterealink
terraform -chdir=infrastructure/bootstrap init
terraform -chdir=infrastructure/bootstrap plan -out=phase14-bootstrap.tfplan
terraform -chdir=infrastructure/bootstrap show phase14-bootstrap.tfplan
terraform -chdir=infrastructure/bootstrap apply phase14-bootstrap.tfplan
```

## Deployment behavior

The production Terraform apply enables the Cloud Scheduler API and creates:

- `eterealink-cleanup@eterealink.iam.gserviceaccount.com`, with access to the pinned database secret and a bucket-level custom role containing only `storage.objects.delete`, conditioned to the `anonymous/` object namespace;
- the `eterealink-cleanup` Cloud Run job, using the API image's `/app/cleanup` binary and the private database network path;
- a job-level `roles/run.invoker` grant for the cleanup identity; and
- the enabled `eterealink-cleanup-hourly` Scheduler job using OAuth to call the Cloud Run Jobs v2 `:run` endpoint.

Each execution reads at most 100 expired roots at a time and drains additional full batches. It deletes standalone object keys, or every file and ZIP key for a transfer, before deleting the root row. Missing objects count as successful cleanup; other storage errors keep the metadata and make the task fail for retry.

## Verify and operate

To verify an already deployed commit, authenticate with Google Cloud, provide the same image, database, and optional alert-email inputs used for deployment, and run:

```bash
IMAGE_TAG="$(git rev-parse HEAD)" make phase14-verify
```

The verifier checks the job image and identity, exact object-delete role, secret access, invoker grant, schedule, pinned secret version, and bounded job configuration. It then runs the cleanup job, confirms a structured completion log, and executes every Phase 13 check.

Inspect recent executions with:

```bash
gcloud run jobs executions list --job=eterealink-cleanup --region=us-west1 --project=eterealink
```

The `lifecycle cleanup complete` log reports candidates, deleted objects, already-missing objects, and deleted metadata roots. A nonzero job exit means at least one object or metadata operation failed and will be retried.
