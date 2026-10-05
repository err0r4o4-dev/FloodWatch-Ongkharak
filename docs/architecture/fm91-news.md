# FM91 automatic media-news publication

## Evidence-to-publication flow

```text
Opt-in FM91_NEWS Go worker (five-minute polling)
  -> bounded HTTPS GET /news
  -> deduplicated candidate article links (location OR event keyword)
  -> bounded HTTPS GET /newsarticle/{id}
  -> validated main-article text (location AND event keyword)
  -> private original HTML archive
  -> PostgreSQL immutable NEWS_REPORT / MEDIA evidence, current-news projection and health
  -> transactional external_data.updated outbox invalidation
  -> GET /api/v1/news-announcements
  -> separate FM91 panels on Announcements, Overview and Map
```

Qualifying articles publish automatically. There is no DRAFT, review queue, approval
button or admin prerequisite. `PUBLISHED` describes publication, not verification
or present-day road safety. FM91 is media evidence, not a TMD CAP official warning,
community eyewitness submission or AI inference. Any authority named in the article
remains quoted provider text; it does not change the article's MEDIA provenance.

Media remains outside the existing ten-source `/external-data/ongkharak` measurement
snapshot and `/official-announcements` TMD read model. This prevents old clients
from treating a reported depth as a station reading or a media headline as CAP.
The outbox/SSE permits MEDIA only on `external_data.updated` / `EXTERNAL_DATA`.
It cannot label water/road/flood/report events MEDIA. Existing structured alert
rules, severity thresholds and Push/email delivery are unchanged: keyword matches
publish news, not an invented hazard classification or a travel-safety guarantee.

## Collection, extraction and identity

The adapter uses the verified server-rendered `/news` listing and article main
containers, never navigation/sidebar recommendations or scripts as article text.
Candidates match Ongkharak/Nakhon Nayok/SWU, canals 14–16, Highway 305 or the
Rangsit–Nakhon Nayok road, or flood/traffic keywords. Publication requires both a
geographic match and an event match in the main headline/body. Whitespace and dash
variants are supported. These are textual relevance rules, not proof of an exact
geographic location; SWU and provincial references cannot establish an Ongkharak
coordinate by themselves.

The source article ID and canonical HTTPS URL must agree. Thai Buddhist-era dates
with an explicit publication clock are interpreted in Asia/Bangkok and stored in
UTC. A missing/unsupported date stays null. Publication time never substitutes for
an observation timestamp. The adapter embeds Go's standard timezone database so
the minimal API/worker runtime images do not depend on host timezone files.
Explicitly dated, unambiguous event text can supply an
observation time; otherwise extracted clock mentions remain literal strings.
Upcoming closures retain their scheduled time as text, not a future observation,
and do not fail the whole poll merely because the announcement concerns a future date.

Category, original text, allowed image URLs, matched locations/roads, reported
depths with units and travel advice are retained as source text. Images are not
downloaded or mirrored. No inferred coordinate, district centroid, zero water
level, station, numerical reading or SAFE road state is created. The current HTML
adapter supplies no verified coordinates, so the Map panel shows textual evidence
without creating a pin. Raw HTML and archive keys stay private.

Re-fetching candidates detects edits. Database uniqueness on
`(source_code, record_key, content_hash)` reuses unchanged evidence and stores
changed content immutably. The public read model groups revisions by article ID,
uses a deterministic source/article UUID as stable identity, and atomically updates
the current-news projection with the committed content version. Equal retrieval
timestamps never select a version by random UUID ordering. Repeated reads/polls do
not create duplicate cards. `updatedAt` is the latest content transition, including
A → B → A reversions; unchanged repeated polls leave it alone. `retrievedAt`
is the article's latest successful retrieval. Every successful ingestion emits a small
source-health/snapshot invalidation; it is not a duplicate user notification.
An empty later listing does not erase retained articles or establish an all-clear.

## Bounds and failure behavior

One source run has a 90-second deadline; HTTP timeout, bounded retries, request
pacing and response-size limits apply. Candidates, raw payloads and total archived
bytes are bounded. Exceeded limits, unsafe URLs, invalid content, network failure
or unrecognized structural markup fail visibly. No partial attempt publishes
unarchived/uncommitted evidence. Earlier committed news remains available as history
with unknown freshness when source health degrades. A failed attempt retries on
the next five-minute due poll; ingestion leases prevent overlapping source runs.

The initial collector intentionally scans only the overlapping current `/news`
listing. It does not promise full historical backfill after a long outage, and
does not use the site's token-bearing category AJAX pagination. Historical
pagination requires a separately verified bounded adapter rather than guessed
URLs, printed session tokens or unbounded crawling. Confirm site usage rights and
attribution requirements before production reuse; an unrestricted robots policy
alone is not a content license.

Source health is FRESH for at most 15 minutes after a successful poll with no
subsequent error. Article freshness separately uses actual source publication time:
at most 24 hours is FRESH; older is STALE. Missing/future publication time,
unconfirmed evidence or an unavailable source is UNKNOWN. Re-polling an old story
never makes its event new. The article remains PUBLISHED in all these cases, with
its reason and dates visible; UNKNOWN never becomes SAFE.

The public GET is read-only, has no upstream request or publication side effect,
and accepts `limit=1..50` (default 20), `offset=0..10000` (default 0). Duplicate or
invalid parameters return 400. Reads have a five-second deadline, 60/minute rate
limit and `Cache-Control: no-store`. Database/corrupt-history failures return a
sanitized 503. Indexed SQL pagination reads only up to 50 current article records,
independent of the size of retained immutable history. `historyTotal` counts unique
retained content versions, not polling attempts. Projection updates, evidence,
health and outbox events commit in one transaction. Provision archive capacity and
a reviewed retention strategy for long-running use.

## Enabling and rollback

1. Apply migration `000028_fm91_news` before deploying the matching API and worker.
2. Add `FM91_NEWS` to the existing `EXTERNAL_DATA_PROVIDERS` allowlist in both API
   and worker, retaining any already enabled provider codes.
3. Deploy/recreate the matching API and worker using the normal release procedure.
   Without configuration, public news returns NOT_CONFIGURED and performs no fetch.

The implementation does not edit `.env`, apply migrations to the application
database, restart existing services, or seed live news. Test fixtures are explicitly
synthetic; read-only live adapter checks do not persist provider data.

Disable FM91 and stop compatible writers before downgrade. Back up first: down
migration 28 removes MEDIA outbox entries and retained news/batch/raw-reference
metadata and the current-news projection, restores prior provenance constraints, and preserves official/model
evidence. Private archived HTML objects are not deleted automatically. Downgrading
does not restore deleted news or safely re-enable a source without matching code.
