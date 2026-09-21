# Phase 10 Terraform

Phase 10 adopts the live Aurea Link GCP platform into Terraform. It preserves the Phase 9 private database path and adds remote state, drift detection, write-only credential handling, and deletion protection.

## Managed resources

The root configuration in `infrastructure/` manages the required project APIs, runtime identities and IAM grants, Artifact Registry, the private file bucket and CORS policy, Firebase authorized domains, private networking, Cloud SQL, Secret Manager, the API and frontend services, the migration job, and all four production domain mappings. DNS remains at the domain registrar, and image builds remain in the deployment workflow.

Terraform 1.11 or newer is required because the database password uses ephemeral and write-only values. The Google and Google Beta providers are pinned to the tested 8.3 minor series.

## Authenticate and select the project

```bash
gcloud auth application-default login
gcloud auth application-default set-quota-project eterealink
gcloud auth login
gcloud config set project eterealink
```

Terraform uses Application Default Credentials. The `gcloud` user credentials are also needed by the import and verification scripts.

## Bootstrap remote state

The bootstrap configuration has intentionally separate local state. Create the dedicated versioned state bucket once:

```bash
terraform -chdir=infrastructure/bootstrap init
terraform -chdir=infrastructure/bootstrap apply
```

Initialize the production root with that bucket:

```bash
terraform -chdir=infrastructure init \
  -backend-config='bucket=eterealink-terraform-state' \
  -backend-config='prefix=production'
```

Do not delete `infrastructure/bootstrap/terraform.tfstate`; it is ignored by Git and remains the ownership record for the backend bucket.

## Prepare inputs

Copy the example and set the tags currently deployed to Cloud Run:

```bash
cp infrastructure/terraform.tfvars.example infrastructure/terraform.tfvars
gcloud run services describe eterealink-api \
  --region=us-west1 \
  --format='value(spec.template.spec.containers[0].image)'
gcloud run services describe eterealink-web \
  --region=us-west1 \
  --format='value(spec.template.spec.containers[0].image)'
```

Only place the tag after the final colon in `terraform.tfvars`. Never store the database password in that file. For the first adoption, reuse the current password so importing does not rotate a live credential unexpectedly:

```bash
database_url="$(gcloud secrets versions access latest --secret=eterealink-database-url)"
export TF_VAR_database_password="$(printf '%s' "${database_url}" | sed -E 's#^postgres://[^:]+:([^@]+)@.*#\1#')"
unset database_url
```

The current generated password is URL-safe. For later rotations, generate another URL-safe value and increment `database_credentials_version` in `terraform.tfvars`:

```bash
export TF_VAR_database_password="$(openssl rand -hex 24)"
```

## Adopt the existing platform

The import script changes Terraform state only. It does not update GCP resources and can be rerun after an interrupted import:

```bash
make phase10-import
terraform -chdir=infrastructure plan
```

Review every planned change. The initial plan should create one new database secret version, write the current password through the provider's write-only Cloud SQL field, add a credential-version environment value to new API and migration-job revisions, and record non-destructive deletion policies. It must report `0 to destroy` and must not replace the Cloud SQL instance, bucket, network, subnet, Private Services Access connection, or domain mappings.

Apply the reviewed plan, run migrations through the newly managed job, and verify the application:

```bash
terraform -chdir=infrastructure apply
gcloud run jobs execute eterealink-migrate --region=us-west1 --wait
make phase10-verify
```

Keep `TF_VAR_database_password` set for all plan, apply, import, and verification operations. Terraform passes it only to provider write-only fields and does not retain it in plan or state.

## Normal changes

Publish immutable images first, update `api_image_tag` and/or `frontend_image_tag`, and then run `terraform plan` and `terraform apply`. Run the migration job whenever the API image changes. `make phase10-verify` requires an empty Terraform plan and then reruns the Phase 9 network and application checks.

The Phase 8 and Phase 9 scripts remain useful as deployment history and emergency tools. Do not run them as a normal deployment path after Terraform adoption because they can create drift from the Terraform configuration.
