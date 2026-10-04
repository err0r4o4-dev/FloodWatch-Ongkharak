# Official TMD announcements

## Evidence-to-announcement flow

```text
Enabled TMD CAP worker (ten-minute polling)
  -> validated CAP/raw private archive
  -> immutable external_data_records and batch/raw links
  -> retained announcement lifecycle read model
  -> GET /api/v1/official-announcements
  -> separate official panel on Announcements and Overview
```

This is a public read model of stored OFFICIAL evidence, not a forecast, app-generated threshold alert, or a new delivery pipeline. It reuses migration 20 tables; no announcement migration, dependency or seeded severity/threshold is added. Existing application rule alerts and their severity filters/counts remain separate. Reading the endpoint makes no upstream HTTP request, database write, `alert.created` event, Push or email delivery.

## Identity, lifecycle and history

Records sharing a CAP identifier or Update/Cancel reference chain form one announcement. Its stable identifier is the earliest retained evidence UUID. New content under the same identifier is an immutable revision, not a duplicate announcement. Histories are ordered by source-issued time, then first recorded retrieval time and evidence UUID as deterministic tie breakers. An update-only feed can reconcile older retained references; an empty current feed does not erase prior announcements.

The latest source-issued revision determines ACTIVE, UPDATED, CANCELLED or EXPIRED. Equal-issued cancellation takes precedence over update, then initial alert; repolling cannot reverse that terminal message. Among variants of the same CAP identifier/message type with the same issue time, the most recently retrieved retained version wins; first retrieval still determines stable identity and history ordering. Re-fetching an older issued alert never supersedes a later cancellation. Shared references to an unretained predecessor connect lifecycle records but do not supply geographic applicability. Cancellation may omit title/body/area: the card retains those details from previous revisions while the cancellation history entry preserves the actual empty fields. Expiry is evaluated at request time. CAP time windows are validated per information block; when local blocks match, only those blocks determine the conservative effective/expiry envelope, not unrelated areas. No current record may be presented as effective ACTIVE/UPDATED when its source is unavailable/stale, expiry is missing, applicability is unconfirmed or it is not yet effective; effectiveStatus becomes UNKNOWN with an explanation. An empty list does not certify safety.

Only chains with retained evidence applicable to Nakhon Nayok (`TH-26`) or the adapter's matching Ongkharak polygon are listed. Provincial warnings are not street-level flood measurements. A standalone unconfirmed update/cancel cannot manufacture a local announcement. An unconfirmed update linked to an earlier applicable announcement remains visible as UNKNOWN, preserving the lifecycle rather than falsely keeping its predecessor current. Original provider text is retained, not translated or assigned an invented severity.

Equal-issued same-type updates resolve reference causality before retrieval order, so a later-arriving ancestor cannot replace its successor. Competing terminal messages or a reference cycle fail closed as UNKNOWN. Individually valid selected CAP information windows with no common intersection are retained as indeterminate evidence with null effective/expiry bounds and `CONFLICTING_TIME_WINDOWS`; raw documents preserve the original block windows. They do not fail the complete poll or bypass persistence constraints.

## API and UI

Public GET accepts `limit` (default 20, 1–50) and `offset` (default 0, 0–10000); repeated or malformed page values return 400. Maximum request time is five seconds, read rate limit 60/minute and Cache-Control is no-store. Disabled TMD returns NOT_CONFIGURED and no items. Database failures return sanitized 503; no private object keys, hashes, raw bodies or geometry are returned. Source health, latest item status and revision timestamps are distinct fields.

The endpoint reads source health and immutable evidence in one read-only repeatable-read transaction. A maximum of 10000 retained CAP evidence versions is processed; exceeding that guard returns unavailable rather than silently truncating cancellation history. Each item exposes at most twenty newest revisions and historyTotal reports the retained total. A reviewed indexed/materialized read model may be needed before a long-running deployment approaches this bound.

The bilingual official panel consumes this endpoint independently of the existing app-generated alert list. It refreshes on `external_data.updated`, every sixty seconds while visible and after explicit retry. Failed or stalled reads cannot keep a current-looking warning. Original CAP content is shown as provider text with issue/expiry dates, geographic codes, official attribution and expandable evidence history.

## Release boundary

Use existing `EXTERNAL_DATA_PROVIDERS` to include `TMD_CAP` in both API and worker; it remains disabled by default. Apply existing migration 20 before deploying matching binaries. This task does not change `.env`, migrate/restart running application services or send notifications. Concurrent report migration 21 is unrelated and must follow its own release instructions.

Next work is an explicitly approved notification bridge with recipient preferences, delivery idempotency and lifecycle-aware retractions; DDPM/ONWR integrations require separately verified provider contracts. Do not connect this read model to the existing alert-created sender implicitly.
