---
name: redis-sse-realtime
description: Use when adding Flood Watch Redis streams, caches, pub-sub coordination, server-sent events, reconnect behavior, or real-time delivery.
---
# Purpose
Deliver live updates without losing ordering, recovery, or truthful degraded-state behavior.
# Use When
Implementing event publication, SSE endpoints, consumer groups, cache invalidation, or reconnect/resume logic.
# Do Not Use When
Building synchronous CRUD endpoints, persistent Postgres models, or browser visual styling.
# Required Rules
- Treat Redis as transport or cache, not the source of truth for durable flood records.
- Use event IDs, bounded payloads, explicit retention, and idempotent consumers.
- Authenticate and authorize subscriptions before stream access; scope events to the least audience.
- Support reconnect and missed-event recovery; expose disconnected or stale state rather than pretending live updates continue.
# Workflow
Define event contract and ownership, test publish/consume/reconnect paths, then verify cleanup and backpressure behavior.
# Checklist
- [ ] Consumers tolerate duplicate, delayed, and out-of-order deliveries.
- [ ] SSE clients receive heartbeats and a clear degraded state.
# Related Skills
`go-api-architecture`, `rest-api-contract-design`, `backend-security-testing`.
