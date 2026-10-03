# Existing-feature API coverage

## Scope

Complete existing feature flows using the React frontend, one Go module with separate API/worker commands, internal FastAPI, PostgreSQL/PostGIS, Redis, and private MinIO. Existing public water, flood, road, risk, report, authentication, alert, and SSE APIs remain compatible. No live external provider, route calculation engine, or CCTV integration is added.

## Boundaries

- Public flood APIs retain source identity, observation/receipt times, freshness, and effective `UNKNOWN` values. Empty or stale data never means safe.
- Authenticated users can access only their own notification inbox and read state; Push registration and inbox history are separate capabilities.
- Dashboard statistics and alert-rule reads require ADMIN. The browser uses the same in-memory access-token/refresh client as other protected APIs.
- AI processing reads only stored report media through private object storage; Python receives bounded image bytes, never an arbitrary URL or storage credential.
- No inference model is bundled. Python returns `503 MODEL_UNAVAILABLE` by default; test providers are confined to tests. Mock output must never be persisted or approved as live evidence.
- AI estimates remain separate, unverified evidence until human review. Approved output remains `AI`, never `OFFICIAL`; road observations require an explicit valid road association rather than invented map matching.
- Canonical writes and outbox changes are transactional. Realtime is a refetch hint, not authoritative resource state. Notification history and durable delivery work must survive a Push failure.

## Processing flow

```text
Browser -> Go handler -> validation/authentication -> service -> repository
                                                       |       |
                                                       |       +-> PostgreSQL/PostGIS
                                                       +-> private MinIO media

Uploaded report media -> leased AI job -> private media bytes -> internal FastAPI
  -> unavailable/retry, or non-mock evidence -> ADMIN review -> canonical AI observation

Canonical observation -> outbox -> rule evaluation -> alert
  -> per-user inbox + per-subscription delivery work -> bounded Push delivery/retry
  -> Redis stream -> public SSE -> frontend refetches REST
```

## Acceptance criteria

1. Dashboard counts derive from database records and expose errors rather than returning placeholder zeroes.
2. ADMIN can list/get enabled and disabled rules with bounded pagination; guest/USER requests receive 401/403.
3. Users can list their inbox, count unread items, mark one/all read, and never read or mutate another account's inbox.
4. Repeated alert processing does not duplicate inbox entries or delivery jobs. History remains available without VAPID configuration.
5. Private report images are not exposed through inference requests, logs, public SSE, or validation errors.
6. AI failures are retryable/observable, cannot produce mock evidence, and do not mark failed analysis as successfully completed.
7. Evidence review is transactional, disallows self-review and stale report snapshots, and keeps rejected/private evidence out of public reads.
8. Frontend uses actual API results and handles loading, empty, guest, unavailable, failure, retry, and accessible actions.
9. Canonical OpenAPI and the embedded specification agree and document all implemented feature routes.

## Verification

Implementation and integration verification are in progress. Local baseline failures and final check results are tracked in `TODO.md`. Real Google consent, SMTP delivery, Web Push delivery on a real device, and genuine model inference require separately configured providers and are not demonstrated by synthetic tests.
