# ADR 0018: Production observability

## Status

Accepted and implemented in Phase 13.

## Context

Phase 12 can deliver a commit to production repeatably, but a successful deployment does not show whether the public application remains reachable or give an operator a focused place to investigate failures. Cloud Run already emits platform request logs and metrics, while the API emits JSON application logs. The remaining work is to turn those signals into a small, useful operating surface without adding an always-on third-party service or high-cardinality custom metrics.

## Decision

- Enable Cloud Logging and Cloud Monitoring explicitly in the production Terraform root.
- Give the keyless deployment identity a dedicated custom role limited to the Monitoring resources Terraform manages and application-log reads used by verification.
- Check `https://aurealink.app/health` over HTTPS every five minutes from Google Cloud's public uptime checkers. Validate TLS and response content, and retain failed probe details in Cloud Logging.
- Alert when fewer than half of the aligned probe results pass for five minutes. Treat the incident as critical, close it after 30 minutes without data, and include a concrete diagnosis and rollback runbook in the alert.
- Allow an optional Terraform-managed email notification channel. The alert policy exists even when an address is not supplied, so incidents remain visible in Cloud Monitoring while notification routing can be configured without changing the policy.
- Manage one production dashboard with public availability, request rate, 5xx rate, p95 request latency, Cloud Run instance count, and recent error logs for both services.
- Emit Cloud Logging-compatible structured API logs. Each non-liveness request records the method, route, status, response size, latency, and a safe request ID. When Cloud Run supplies an `X-Cloud-Trace-Context` header, promote its trace and span identifiers into Cloud Logging's recognized correlation fields.
- Suppress successful `/health` and `/healthz` application entries because Cloud Run already emits request logs for those probes. Preserve readiness and failed probe logs.
- Verify the live resources and one newly generated correlated readiness log after every production deployment, then run all prior phase checks.

## Consequences

An operator can see availability, errors, latency, scale, and recent error detail on one dashboard, and can move from an incident to the relevant request or trace. The public check also covers DNS, TLS, domain mapping, and the frontend process rather than only container health.

The five-minute check and sustained-failure threshold favor low noise and low cost over immediate paging. A notification email remains optional because its address is deployment-specific and must be verified by its recipient. Cloud Run platform logs remain authoritative for all HTTP traffic; the application log adds request IDs, trace correlation, and domain-level errors without duplicating successful liveness traffic.
