# Eterealink

Eterealink is a cloud-native file-sharing platform for quickly uploading, organizing, previewing, and sharing files through short URLs. Anonymous transfers require no account and expire after 24 hours; authenticated users will be able to retain files, organize them into folders, and manage sharing.

The project is designed as both a useful product and a practical demonstration of cloud architecture, networking, security, infrastructure as code, CI/CD, and observability on Google Cloud Platform.

## Project status

Phase 1—the local backend foundation—is complete.

Implemented:

- Go REST API with structured logging and graceful shutdown
- PostgreSQL schema and reversible migration runner
- Atomic creation of anonymous file and share metadata
- Explicit `PENDING` to `READY` upload lifecycle
- Random, non-sequential short share codes
- Share expiration and revocation validation
- Configurable file-size and signed-URL lifetime limits
- Unit tests for core transfer rules and HTTP health behavior

The storage interface currently uses an intentionally non-functional development signer. Phase 2 will replace it with Google Cloud Storage V4 signed URLs and verify uploaded objects before marking transfers ready.

## Product goals

- Let anonymous users upload a file and receive a 24-hour share link without registering.
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
 | Regional subnet|
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

1. The browser requests an upload from the Go API.
2. The API creates temporary file and share metadata in PostgreSQL.
3. The API returns a short-lived signed Cloud Storage upload URL.
4. The browser uploads the file directly to Cloud Storage.
5. The browser tells the API that the transfer completed.
6. The API verifies the stored object and marks the file ready.
7. Recipients resolve the short code and receive authorized preview/download metadata.
8. The file, share, and object expire after 24 hours.

## Current API

| Method | Path | Behavior |
|---|---|---|
| `GET` | `/healthz` | Process liveness |
| `GET` | `/readyz` | Database-aware readiness |
| `POST` | `/v1/uploads` | Create anonymous file/share metadata and an upload target |
| `POST` | `/v1/uploads/{id}/complete` | Mark a successful direct upload ready |
| `GET` | `/v1/shares/{code}` | Resolve a usable share and a short-lived download target |

## Local development

### Prerequisites

- Go 1.24 or newer
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

4. Check the service endpoints:

   ```text
   http://localhost:8080/healthz
   http://localhost:8080/readyz
   ```

Run the test suite with:

```bash
make test
```

## Delivery roadmap

| Phase | Outcome |
|---|---|
| 1. Local backend | PostgreSQL-backed anonymous transfer metadata and lifecycle |
| 2. File transfer | Real direct uploads/downloads through Cloud Storage signed URLs |
| 3. Anonymous sharing | Complete no-account upload-to-share experience with 24-hour expiry |
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
- Dedicated service accounts and Secret Manager will be configured with least privilege.
- Cloud Run scales to zero, and always-on load balancers, NAT gateways, and Kubernetes are excluded unless a real requirement justifies them.
- Anonymous transfers have file-size limits, abuse controls, and automated expiration.

Architecture decisions are recorded under [`docs/architecture`](./docs/architecture/).
