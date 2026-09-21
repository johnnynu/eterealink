# Phase 11 security hardening

Phase 11 narrows production IAM, separates the migration identity, pins database secret versions, enforces hard ceilings around anonymous sharing, rate-limits anonymous upload creation, and adds browser/API security headers.

## Review the change

The Terraform plan should:

- create `eterealink-migrate@eterealink.iam.gserviceaccount.com` and assign it to the migration job;
- create two custom roles containing three Cloud Storage object permissions and one `signBlob` permission respectively;
- grant the API those custom roles only at the bucket and API service-account resources;
- grant the database secret to the API and migration identities;
- remove the obsolete project-level Cloud SQL Client grant, Storage Object User grant, and Service Account Token Creator self-grant;
- replace `latest` database-secret references with the managed numeric version; and
- deploy API and frontend images that contain the Phase 11 application controls.

Recover the existing write-only database input before planning:

```bash
database_url="$(gcloud secrets versions access latest --secret=eterealink-database-url)"
export TF_VAR_database_password="$(printf '%s' "${database_url}" | sed -E 's#^postgres://[^:]+:([^@]+)@.*#\1#')"
unset database_url
```

Build immutable API and frontend images as described in the Phase 10 normal-change workflow, then set their tags in `infrastructure/terraform.tfvars`. The frontend must be rebuilt because its response headers are compiled from `next.config.ts`.

```bash
terraform -chdir=infrastructure plan -out=phase11.tfplan
terraform -chdir=infrastructure show phase11.tfplan
terraform -chdir=infrastructure apply phase11.tfplan
gcloud run jobs execute eterealink-migrate --region=us-west1 --wait
make phase11-verify
```

The IAM replacements can briefly overlap during apply, but Terraform does not replace the bucket, database, services, network, or secret. Keep `TF_VAR_database_password` set for verification because the inherited Phase 10 drift check requires it.

## Application controls

Production uses these fixed ceilings:

| Control | Value |
|---|---:|
| Signed URL lifetime | 15 minutes |
| Anonymous content lifetime | 24 hours |
| Anonymous file/transfer size | 1 GiB |
| Files per anonymous transfer | 10 |
| Anonymous creations per client | 6 per minute |
| Anonymous creations per API instance | 60 per minute |

Lower values are supported through environment overrides. Startup rejects overrides above the security ceilings. Rate-limit state is in memory and resets when an instance starts; the three-instance Cloud Run cap bounds the aggregate per-instance ceiling.
