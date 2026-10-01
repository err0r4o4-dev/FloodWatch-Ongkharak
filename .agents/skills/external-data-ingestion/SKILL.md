---
name: external-data-ingestion
description: Use when integrating external flood, weather, road, sensor, CCTV, or government data providers into Flood Watch ingestion workflows.
---
# Purpose
Bring external data in through auditable adapters that fail safely and preserve raw evidence.
# Use When
Adding provider clients, polling jobs, webhooks, normalization, deduplication, or ingestion persistence.
# Do Not Use When
Building manual admin entry, UI presentation, or unverified AI output handling.
# Required Rules
- Isolate each provider behind a fetch, validate, normalize, deduplicate, and persist adapter boundary.
- Record provider identity, retrieval time, source timestamp, payload reference, and parsing outcome.
- Apply timeouts, bounded retries, rate limits, and idempotency; never silently discard failures.
- Treat unavailable, malformed, or stale provider data as `UNKNOWN`, not a safe condition.
# Workflow
Document provider assumptions, capture representative fixtures, test success and failure normalization, then observe a dry-run result.
# Checklist
- [ ] Retries cannot create duplicate observations.
- [ ] Provider outages are visible to operators and downstream consumers.
# Related Skills
`flood-data-reliability`, `postgresql-postgis-backend`, `go-api-architecture`.
