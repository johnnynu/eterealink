<p align="center">
  <img src="./eterea-chrome-final.png" alt="Eterealink" width="900">
</p>

# Eterealink

Eterealink is a cloud-native file-sharing platform for quickly uploading, organizing, previewing, and sharing files through short URLs. Anonymous transfers require no account and expire after 24 hours; authenticated users can retain private files, organize them into folders, and collaborate through role-based shared folders.

The project is designed as both a useful product and a practical demonstration of cloud architecture, networking, security, infrastructure as code, CI/CD, and observability on Google Cloud Platform.

## Project status

Phases 1 through 6.9—the local backend foundation, direct Cloud Storage transfer layer, anonymous sharing experience, Firebase authentication, persistent-file library, virtual folders, folder collaboration, landing-page account feature introduction, safe browser previews, realtime refreshes, and profiles—are complete.

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
- Optional Firebase Google Sign-In without changing the anonymous transfer flow
- Server-side Firebase ID-token verification and idempotent local user provisioning
- Optional unique Eterealink display names with Google-name fallback, profile editing, and collaborator-facing realtime refreshes
- Live end-to-end identity verification against the Eterealink Firebase project
- Owner-scoped persistent uploads, file listing, authorized downloads, deletion, and revocable share links
- A 5 GiB per-file limit for authenticated persistent uploads while anonymous transfers remain capped at 1 GiB combined
- Persistent-library storage totals, drag-and-drop, filename search, shared-file filtering, sorting, and bounded pagination
- Signed-in `/app` workspace with a private file library and a separate 24-hour transfer flow
- Nested virtual folders with URL-backed breadcrumbs that survive refresh and browser navigation, folder-scoped uploads, renaming, and empty-folder deletion
- Inherited Viewer and Contributor folder roles with a dedicated “Shared with me” workspace
- Descendant access panels that show inherited members and link back to the folder where access was granted
- Expiring, revocable folder invite links with a first-time-user sign-in handoff, plus direct email-based access
- Contributor uploads charged to the uploader's quota, with permanent file mutations restricted to that uploader
- Owner-safe contribution removal that returns files to the uploader's private library
- Multi-select file moves and deletion, including moves back to the library root
- Persistent per-file upload queue with progress, cancel, retry, and independent failure handling
- Atomic 25 GiB account storage quota enforcement, configurable independently from the 5 GiB per-file limit
- Short-lived inline preview targets for supported images, PDFs, video, audio, and text
- Escaped text rendering, cross-origin PDF embedding, a server-side media allowlist, and generic fallback for unsupported files
- Preview selection for multi-file transfers and an authenticated preview dialog for private or shared-folder files
- Original-quality video playback with Eterealink controls for seeking, ten-second skips, volume, playback speed, picture-in-picture, fullscreen, source-resolution display, buffering and codec feedback, auto-hiding controls, and remembered preferences

The complete anonymous metadata → direct GCS upload → completion → short-link resolution → signed download flow is covered by automated tests and can be exercised against the local PostgreSQL service and Phase 2 GCS bucket.

### Post-MVP media roadmap

- WebVTT captions and track selection after uploads can associate subtitle files with a video.
- Timeline thumbnail previews after an asynchronous worker can extract and store seek-preview sprites.
- Adaptive streaming and selectable quality after a transcoding pipeline can create multiple renditions and HLS/DASH manifests.

## Product goals

- Let anonymous users upload one or more files and receive a 24-hour share link without registering.
- Transfer file bytes directly between browsers and object storage rather than proxying them through the API.
- Provide short, human-shareable URLs without exposing internal IDs.
- Support previews for common images, documents, video, audio, and text.
- Give signed-in users persistent files, virtual folders, revocable links, and role-based folder collaboration.
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
| `GET` | `/v1/me` | Verify a Firebase bearer token and return the provisioned user |
| `PATCH` | `/v1/me` | Set or clear the authenticated user's optional unique Eterealink display name |
| `POST` | `/v1/files` | Create owner-linked persistent file metadata and an upload target |
| `GET` | `/v1/files` | List the authenticated user's ready files and aggregate storage usage |
| `POST` | `/v1/files/{id}/complete` | Verify and complete an owned persistent upload |
| `GET` | `/v1/files/{id}/download` | Create short-lived download and optional safe preview targets for an owned file or a file inherited through folder membership |
| `POST` | `/v1/files/{id}/shares` | Create an expiring or non-expiring public link for an owned file |
| `DELETE` | `/v1/files/{id}/shares/{shareID}` | Revoke an active link for an owned file |
| `DELETE` | `/v1/files/{id}` | Delete an owned persistent file and its storage object |
| `PATCH` | `/v1/files/move` | Move up to 100 owned files to a folder or the library root |
| `POST` | `/v1/folders` | Create an owned root or nested virtual folder |
| `GET` | `/v1/folders?scope=owned\|shared` | List owned root contents or folders directly shared with the user; supports `q`, `sort`, `filter`, `limit`, and `cursor` |
| `GET` | `/v1/folders/{id}` | Browse an accessible folder with server-side search, sorting, filtering, and cursor pagination |
| `PATCH` | `/v1/folders/{id}` | Rename or move an owned folder |
| `DELETE` | `/v1/folders/{id}` | Delete an owned folder after it is empty |
| `GET` | `/v1/folders/{id}/members` | List a folder's viewers and contributors as its owner |
| `POST` | `/v1/folders/{id}/members` | Grant Viewer or Contributor access to an existing user by email |
| `DELETE` | `/v1/folders/{id}/members/{userID}` | Revoke folder access and return that member's contributions to their private library |
| `GET` | `/v1/folders/{id}/invites` | List active authenticated invite links as the folder owner |
| `POST` | `/v1/folders/{id}/invites` | Create a Viewer or Contributor invite link with a joining deadline |
| `DELETE` | `/v1/folders/{id}/invites/{inviteID}` | Revoke a folder invite link |
| `GET` | `/v1/folder-invites/{code}` | Resolve a privacy-limited invite preview containing the owner name, folder name, role, and joining deadline |
| `POST` | `/v1/folder-invites/{code}/accept` | Accept an active folder invite as the authenticated user |
| `DELETE` | `/v1/folders/{id}/files/{fileID}` | Remove another user's contribution from an owned folder without deleting their file |
| `POST` | `/v1/uploads` | Create anonymous file/share metadata and an upload target |
| `POST` | `/v1/uploads/{id}/complete` | Mark a successful direct upload ready |
| `POST` | `/v1/transfers` | Create one anonymous multi-file transfer and resumable targets |
| `POST` | `/v1/transfers/{transferID}/files/{fileID}/complete` | Verify and complete one transfer file |
| `GET` | `/v1/shares/{code}` | Resolve a usable single-file or multi-file share with short-lived download targets and optional safe previews |

## Local development

### Prerequisites

- Go 1.25 or newer
- Node.js 24 or newer and npm
- PostgreSQL 17, or Docker with Compose

### Run the API

1. Copy `.env.example` to `.env` and set the local values. Make automatically exports values from this ignored file; direct `go run` commands still require the values in your shell.
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

To enable Google Sign-In, follow the [Phase 4 Firebase setup guide](./docs/setup/firebase-phase4.md). Authentication is optional in local development: when Firebase variables are absent, anonymous transfers continue to work and the sign-in control stays hidden.

## Delivery roadmap

| Phase | Outcome |
|---|---|
| 1. Local backend ✅ | PostgreSQL-backed anonymous transfer metadata and lifecycle |
| 2. File transfer ✅ | Real direct uploads/downloads through Cloud Storage signed URLs |
| 3. Anonymous sharing ✅ | No-account upload-to-share experience with enforced 24-hour expiry |
| 4. Authentication ✅ | Firebase Google Sign-In and verified API identity |
| 4.5. Persistent files ✅ | Owner-scoped uploads, private file library, downloads, and deletion |
| 5. Folders ✅ | OWNER/VIEWER folders, bulk workflows, upload queue, quota, sharing, and cursor-based library queries |
| 5.1. Collaboration ✅ | Scalable access management, personalized expiring invites, Contributor uploads, uploader attribution, and uploader-owned file controls |
| 5.5. Landing page ✅ | Introduce account benefits alongside anonymous sharing, with a Google sign-in call to action and responsive feature section |
| 6. Previews ✅ | Browser previews with a safe generic fallback |
| 6.5. Realtime collaboration ✅ | Authenticated SSE folder invalidation, PostgreSQL notifications, and reconnect-safe refreshes |
| 6.9. Profiles ✅ | Optional unique display names, Google-name fallback, account editing, and realtime collaborator refreshes |
| 7-10. Cloud platform | Containers, Cloud Run, private networking, and Terraform |
| 11-14. Operations | Security hardening, CI/CD, monitoring, and lifecycle cleanup |

The product MVP is complete. The cloud portfolio milestone adds repeatable infrastructure, private database networking, automated deployment, security controls, and operational visibility.

## Security and cost posture

- Buckets remain private; access is issued through short-lived signed URLs.
- Firebase tokens establish identity, while the API and PostgreSQL enforce authorization.
- PostgreSQL will use private Cloud SQL connectivity rather than a public database endpoint.
- A dedicated signing service account has bucket-scoped object access; Secret Manager will be added when application secrets are introduced.
- Cloud Run scales to zero, and always-on load balancers, NAT gateways, and Kubernetes are excluded unless a real requirement justifies them.
- Anonymous transfers have file-size and file-count limits plus server-enforced access expiration. Rate limiting and physical object cleanup remain part of the operations phase.

Architecture decisions are recorded under [`docs/architecture`](./docs/architecture/).
