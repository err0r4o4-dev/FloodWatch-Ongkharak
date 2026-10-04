# Guest public flow and administrator boundary

Approved October 4, 2026. Public flood information, maps, verified community reports, report submission and the notification centre do not require an account. Administrators enter through `/admin`, authenticate at `/admin/login`, and return to an allowed administration path. Backend authorization remains authoritative. Existing accounts and their records are retained; public registration is disabled.

## Display names

The public user menu retains its person icon and lets visitors edit a display name, defaulting to `Guest`. It is a browser-local preference, not a unique username, authenticated identity or ownership credential. Names are trimmed Unicode text bounded to 80 code points. Reports store the submitted name as a snapshot, so later preference changes do not rename earlier reports. Blocked browser storage falls back to the current page session. No account credentials are stored with the display name.

## Report publication and images

Anonymous submissions create `PENDING` reports with `COMMUNITY` provenance. Only administrator approval makes them `VERIFIED` and available to public report lists, map layers, images and public notifications. Rejected reports remain private. Verification does not turn community evidence into official data or certify an area or road as safe.

Images use a cryptographically random, report-scoped, short-lived upload capability returned only in the create response. The browser holds it in memory for the initial upload and failed-file retries, never in URLs or persistent browser storage. The database stores only its hash. It cannot read unpublished reports or images, change text, approve a report, or upload to another report. Uploads remain subject to type/size/count validation and the pending revision lock. Guest report editing and private tracking links are not part of this release.

Public report creation and upload are rate-limited. When behind a proxy, configure `TRUSTED_PROXY_CIDRS` to the actual trusted ingress addresses; otherwise all visitors behind that proxy share the direct-peer budget. Forwarded addresses from untrusted peers are ignored. These limits are process-local, so a deployment with multiple API replicas also needs a shared edge limit. Proxy access logs redact the upload capability header.

## Notifications

The public notification centre lists published alerts and verified community reports, retaining `OFFICIAL`, `COMMUNITY` and `AI` provenance. A manual area selection filters spatial incidents without requiring GPS; global announcements remain visible. Publication timestamps order newly approved reports. Read state is local to each browser and does not identify the visitor. Empty, unavailable or stale information never implies safe conditions.

Anonymous Web Push and report tracking tokens remain optional future work. This release provides in-web notifications, not a promise of delivery when the website is closed. Existing authenticated inbox/subscription APIs remain private compatibility interfaces and are not used for the Guest centre.

## Deployment

Apply migration `000018_guest_reports` with the matching API and frontend release after earlier migrations. It preserves existing user references and makes report ownership nullable; the display-name snapshot and hashed upload-capability fields are added. Its rollback refuses to discard guest reports. Source edits and tests do not migrate the running application's database or restart services automatically.
