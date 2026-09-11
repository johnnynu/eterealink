# Phase 9 private networking

Phase 9 moves PostgreSQL traffic off the transitional Cloud SQL public address. The API service and migration job use Cloud Run Direct VPC egress to reach the database's private address through a custom VPC and Private Services Access.

## Resources

| Resource | Default |
|---|---|
| Project and region | `eterealink`, `us-west1` |
| Custom VPC | `eterealink` |
| Cloud Run subnet | `eterealink-us-west1` (`10.0.1.0/24`) |
| Private Services Access allocation | `eterealink-google-managed-services` (`10.10.0.0/24`) |
| Service networking producer | `servicenetworking.googleapis.com` |
| Cloud Run API and job | `eterealink-api`, `eterealink-migrate` |
| Cloud SQL | `eterealink-db`, private TCP port `5432` |

The subnet and managed-services allocation are distinct, non-overlapping RFC 1918 ranges. A `/24` application subnet provides enough addresses for Cloud Run's Direct VPC allocation behavior and future low-volume workloads. The separate `/24` allocation gives Cloud SQL one regional service range without consuming addresses from the application subnet.

Cloud Run uses `private-ranges-only` egress. Database packets enter the VPC and cross the Private Services Access peering; public Google APIs and other public destinations retain Cloud Run's normal outbound path. This keeps Phase 9 free of an always-on connector or Cloud NAT gateway.

## Deploy

Phase 8 must already be healthy. The active `gcloud` account must have permission to manage Compute networks, Service Networking connections, Cloud SQL, Cloud Run, and Secret Manager, and the CLI project must be `eterealink`.

```bash
make phase9-deploy
```

The deployment is staged to preserve a working rollback path:

1. Create or validate the custom VPC, regional subnet, reserved managed-services range, and Private Services Access connection.
2. Add a private Cloud SQL address while retaining the existing public connector path.
3. Pin the serving API revision to its current secret version, then create a new database URL for the private address.
4. Attach the migration job to Direct VPC egress, remove its Cloud SQL Auth Proxy attachment, and execute it as a connectivity test.
5. Apply the same network and secret configuration to the API and verify `/health` plus database-backed `/readyz`.
6. Remove the Cloud SQL public address and verify readiness again.

The final database URL is an immutable Secret Manager version and uses direct TCP to the private address. The runtime keeps `roles/cloudsql.client` temporarily; Phase 11 will remove permissions that are no longer required as part of the broader least-privilege review.

Defaults can be overridden with `PROJECT_ID`, `REGION`, `NETWORK`, `SUBNET`, `SUBNET_CIDR`, `PSA_RANGE`, and `PSA_RANGE_CIDR`. Existing resources with the requested names must match those values; the script stops rather than repurposing a mismatched network range.

Adding private networking to Cloud SQL can briefly restart the small zonal instance. Cloud Run continues serving through its prior revision during the service cutover, subject to database availability during that Cloud SQL operation.

## Verify

```bash
make phase9-verify
```

The verifier checks the CIDRs and network ownership, the Private Services Access allocation, the private-only Cloud SQL configuration, both Direct VPC attachments, removal of the Cloud SQL Auth Proxy attachments, the secret's private host, and public readiness through both the API and frontend proxy.

The resulting database path is:

```text
Cloud Run API or migration job
  -> Direct VPC egress (private ranges only)
  -> eterealink-us-west1 (10.0.1.0/24)
  -> Private Services Access peering
  -> Cloud SQL private address in 10.10.0.0/24:5432
```

There is no authorized public database network and no public address to probe. Application readiness is the end-to-end connectivity check.
