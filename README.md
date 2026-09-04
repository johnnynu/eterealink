<p align="center">
  <img src="./eterealink_logo.png" alt="Eterealink" width="900">
</p>

# Eterealink

Eterealink is a cloud-native file-sharing platform for quickly uploading, organizing, previewing, and sharing files through short URLs. Anonymous transfers require no account and expire after 24 hours; authenticated users will be able to retain files, organize them into folders, and manage sharing.

The project is designed as both a useful product and a practical demonstration of cloud architecture, networking, security, infrastructure as code, CI/CD, and observability on Google Cloud Platform.

## Project status

Phases 1 through 3—the local backend foundation, direct Cloud Storage transfer layer, and anonymous sharing experience—are complete.

Implemented:

- Go REST API with structured logging and graceful shutdown
- PostgreSQL schema and reversible migration runner
- Atomic creation of anonymous transfer, file, and share metadata
- Explicit per-file and whole-transfer `PENDING` to `READY` lifecycle
- Random, non-sequential short share codes
- Share expiration and revocation validation
- Configurable per-file, combined-transfer, file-count, and signed-URL lifetime limits
- V4-signed resumable Cloud Storage uploads and signed downloads using keyless IAM signing
- Stored-object size and media-type verification before uploads become `READY`
- Create-only upload preconditions that prevent a signed URL from overwriting an object
- Restricted localhost CORS configuration for browser transfers
- Unit tests plus a live signed upload/download integration test
- Responsive Next.js multi-file upload experience with drag-and-drop and combined progress
- Same-origin API proxy plus direct-to-Cloud-Storage browser upload flow
- Copyable short-link success state with exact expiration details
- One share link for up to 10 files and 1 GiB total per anonymous transfer
- Streaming background ZIP generation with individual-file download fallback
- Recipient share page with ZIP readiness polling, authorized downloads, live time remaining, and distinct expired/revoked/missing states
- Frontend lint, unit-test, and production-build checks

The complete anonymous metadata → direct GCS upload → completion → short-link resolution → signed download flow is covered by automated tests and can be exercised against the local PostgreSQL service and Phase 2 GCS bucket.

## Product goals

- Let anonymous users upload one or more files and receive a 24-hour share link without registering.
- Transfer file bytes directly between browsers and object storage rather than proxying them through the API.
- Provide short, human-shareable URLs without exposing internal IDs.
- Support previews for common images, documents, video, audio, and text.
- Give signed-in users persistent files, virtual folders, revocable links, and read-only folder sharing.
- Keep the deployed portfolio application inexpensive at low traffic.

## Architecture

```text
 +----------------------+
 |      Web Client      |
 |   Next.js / React    |
 +----------+-----------+
            |
   HTTPS / Firebase token
            |
            v
 +----------------------+
 |      Cloud Run       |
 |      Go REST API     |
 +----+------------+----+
      |            |
 Direct VPC       Signed URLs
   egress            |
      |              v
      |       +----------------+
      |       | Cloud Storage  |
      |       |  File objects  |
      |       +----------------+
      v
 +---------------+
 |  Custom VPC   |
 | Regional subnet |
 +-------+-------+
         |
 Private service path
         |
         v
 +---------------+
 |   Cloud SQL   |
 |  PostgreSQL   |
 +---------------+
```

The API owns identity, authorization, metadata, and signed-URL generation. Cloud Storage carries the file bytes directly, keeping Cloud Run stateless and avoiding unnecessary application bandwidth.

## Technology stack

| Layer | Technology |
|---|---|
| Frontend | Next.js / React |
| Backend | Go |
| Authentication | Firebase Authentication with Google Sign-In |
| Database | PostgreSQL locally; Cloud SQL for PostgreSQL in GCP |
| Object storage | Google Cloud Storage |
| Compute | Cloud Run |
| Networking | Custom VPC, Direct VPC egress, private Cloud SQL connectivity |
| Infrastructure | Terraform |
| CI/CD | GitHub Actions and Artifact Registry |
| Operations | Cloud Logging and Cloud Monitoring |

## Request flow

An anonymous transfer follows this path:

1. The browser sends metadata for up to 10 files, totaling no more than 1 GiB, to the Go API.
2. The API atomically creates temporary transfer, file, and share metadata in PostgreSQL.
3. The API returns one short-lived signed resumable-upload target per file.
4. The browser uploads up to three files concurrently and resumes interrupted chunks when possible.
5. The browser confirms each file; the API verifies its stored size and media type.
6. When every file is ready, a background worker streams the objects into a ZIP stored in the existing bucket.
7. Recipients resolve one short code and can download the ZIP when ready or any file individually.
8. The transfer and share stop resolving after 24 hours. Physical object deletion remains part of the lifecycle-cleanup phase.

## Current API

| Method | Path | Behavior |
|---|---|---|
| `GET` | `/healthz` | Process liveness |
| `GET` | `/readyz` | Database-aware readiness |
| `POST` | `/v1/uploads` | Create anonymous file/share metadata and an upload target |
| `POST` | `/v1/uploads/{id}/complete` | Mark a successful direct upload ready |
| `POST` | `/v1/transfers` | Create one anonymous multi-file transfer and resumable targets |
| `POST` | `/v1/transfers/{transferID}/files/{fileID}/complete` | Verify and complete one transfer file |
| `GET` | `/v1/shares/{code}` | Resolve a usable single-file or multi-file share and short-lived download targets |

## Local development

### Prerequisites

- Go 1.25 or newer
- Node.js 24 or newer and npm
- PostgreSQL 17, or Docker with Compose

### Run the API

1. Copy the values in `.env.example` into your shell or preferred local environment manager.
2. Start PostgreSQL:

   ```bash
   docker compose up -d postgres
   ```

3. Apply the schema and start the API:

   ```bash
   make migrate-up
   make run
   ```

4. In another terminal, install and run the frontend:

   ```bash
   make frontend-install
   make frontend-run
   ```

   The web application is available at `http://localhost:3000`. It proxies API control-plane requests to `http://localhost:8080` by default. Override this with `API_BASE_URL` in `frontend/.env.local` when needed.

5. Check the service endpoints:

   ```text
   http://localhost:8080/healthz
   http://localhost:8080/readyz
   ```

Run the test suite with:

```bash
make test
```

To complete a browser upload, use the real GCS backend by following the [Phase 2 GCP setup guide](./docs/setup/gcp-phase2-gcs.md). Multi-file uploads and ZIP output use that same bucket; no second bucket is required. In local development the API process runs the archive worker. It can be separated into a Cloud Run Job when the application is deployed. The default `development` backend remains available for metadata-only work and unit tests, but intentionally cannot accept file bytes.

## Delivery roadmap

| Phase | Outcome |
|---|---|
| 1. Local backend ✅ | PostgreSQL-backed anonymous transfer metadata and lifecycle |
| 2. File transfer ✅ | Real direct uploads/downloads through Cloud Storage signed URLs |
| 3. Anonymous sharing ✅ | No-account upload-to-share experience with enforced 24-hour expiry |
| 4. Authentication | Firebase Google Sign-In and verified API identity |
| 5. Folders | Persistent user files and OWNER/VIEWER folder sharing |
| 6. Previews | Browser previews with a safe generic fallback |
| 7-10. Cloud platform | Containers, Cloud Run, private networking, and Terraform |
| 11-14. Operations | Security hardening, CI/CD, monitoring, and lifecycle cleanup |

The product MVP is reached after the preview phase. The cloud portfolio milestone adds repeatable infrastructure, private database networking, automated deployment, security controls, and operational visibility.

## Security and cost posture

- Buckets remain private; access is issued through short-lived signed URLs.
- Firebase tokens establish identity, while the API and PostgreSQL enforce authorization.
- PostgreSQL will use private Cloud SQL connectivity rather than a public database endpoint.
- A dedicated signing service account has bucket-scoped object access; Secret Manager will be added when application secrets are introduced.
- Cloud Run scales to zero, and always-on load balancers, NAT gateways, and Kubernetes are excluded unless a real requirement justifies them.
- Anonymous transfers have file-size and file-count limits plus server-enforced access expiration. Rate limiting and physical object cleanup remain part of the operations phase.

Architecture decisions are recorded under [`docs/architecture`](./docs/architecture/).
