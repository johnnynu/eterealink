# ADR 0012: Production container images

## Status

Accepted and implemented in Phase 7.

## Context

Eterealink needs reproducible application artifacts before its GCP infrastructure and Cloud Run services are introduced. The local development commands should remain useful, while the same application builds also need to run as unprivileged Linux containers with explicit health checks.

## Decision

- Build separate production images for the Go backend and Next.js frontend. Each Dockerfile uses a dependency/build stage and copies only runtime artifacts into the final image.
- Compile the backend API and migration runner as static binaries. Ship both binaries and the SQL migrations in one image so a deployment can run migrations as a separate job before starting the API.
- Emit the Next.js standalone server and include only its traced runtime files, static assets, and public assets in the frontend image.
- Run both application containers as dedicated, unprivileged users. Define image-level health checks for the API liveness route and frontend root route.
- Use Docker Compose to exercise PostgreSQL, one-shot migrations, the API, and the frontend as an ordered local stack. Database migrations must succeed before the API starts, and the API must become healthy before the frontend starts.
- Treat `NEXT_PUBLIC_*` values and the frontend's `API_BASE_URL` rewrite target as frontend build inputs. They are embedded by the Next.js production build, so each deployed frontend environment must build with its own values. Backend settings remain runtime environment variables.

## Consequences

The application now has deployable, non-root artifacts and a local production-like startup path. Phase 8 can reference immutable image outputs while defining the GCP foundation. The frontend image must be rebuilt when its public Firebase configuration, client limits, or backend proxy target changes.
