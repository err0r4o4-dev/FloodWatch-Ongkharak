# Ongkharak model weather

The public application shell reads `GET /api/v1/weather/ongkharak` from Go. The API fetches Open-Meteo current model weather for the reference point 14.1 N, 101.0 E, matching the existing map center. Open-Meteo may select a nearby model grid cell. This is neither a district-wide measurement nor an observation from a local station. `source.kind=MODEL` deliberately does not claim OFFICIAL, COMMUNITY, or AI flood provenance.

## Flow and bounds

`React -> Go weather service -> bounded Open-Meteo client -> per-process cache -> React`

The adapter requests temperature, apparent temperature, humidity, precipitation, WMO weather code, wind speed/direction and day/night. Units are requested and validated explicitly: Celsius, percent, millimeters, km/h and degrees. Current timestamps use Unix seconds with UTC, keeping model time (`current.observedAt`) separate from receipt time (`retrievedAt`). Rainfall covers the provider's returned `intervalSeconds`, normally 900 seconds; it is never labeled a 24-hour total.

The existing protected ingestion HTTP client is reused with a fixed HTTPS origin and path, a 64 KiB limit, four-second attempts, two attempts, and an eight-second overall refresh budget. Redirects, proxies and private/special-use destination IPs are denied. User requests cannot choose the upstream URL or location. Missing/null values, mismatched units, invalid ranges, invalid intervals, future timestamps beyond one minute and model times more than one hour old are rejected. Additional provider metadata is ignored, not persisted or logged.

Successful responses cache for ten minutes. One in-flight refresh serves concurrent callers; a disconnected caller does not cancel that shared bounded fetch. Failure starts a one-minute cooldown. Cache coordination is per API process, not distributed: budget aggregate requests before scaling replicas. No provider call occurs at startup or while the page/API is unused.

No usable cache produces HTTP 503, `freshness=UNKNOWN` and null measurements, not zeros. A failed refresh retains prior evidence as explicitly STALE. Cached model time older than one hour is also STALE; after six hours measurements are omitted and become UNKNOWN. Successful refresh restores FRESH. HTTP responses use `Cache-Control: no-store` so browsers/proxies do not hide a changed outage state. The public route is rate-limited to 60 reads/minute per limiter identity; trusted ingress configuration still determines that identity.

This capability does not write water readings, create alerts, publish flood SSE events, modify flood thresholds, call Python AI, or add a schema migration. Weather outages do not affect API readiness or independent flood panels. The frontend shows loading, unavailable/retry and stale states independently and includes Thai/English labels, model-time/receipt-time context and Open-Meteo attribution.

## Frontend presentation

One shell-scoped `WeatherProvider` shares the request, refresh timers, retry state and age evaluation between the desktop Sidebar summary, the compact Overview summary at existing mobile/tablet breakpoints, and the native modal containing the full `WeatherPanel`. Public menu navigation keeps that provider mounted; opening details does not trigger another request. Landing and authentication pages remain outside this provider. The Sidebar summary stays within the Sidebar, below the preserved data caution, without overlaying the main content. Its minimize action becomes a keyboard-accessible reopen button; the mobile summary remains inline rather than floating above the map or bottom navigation.

The compact summary includes model provenance, freshness, model time and provider attribution. Unknown or expired measurements remain hidden. Details retain source linking, full model/receipt timestamps, interval precipitation and the flood-safety caveat. The native dialog provides modal semantics and Escape dismissal; closing returns focus to its trigger. Each instance uses unique heading/dialog IDs and shared retries update all consumers.

## Configuration and licensing

`WEATHER_ENABLED` defaults to true in development/test and false in production. Setting it false keeps the public route but returns NOT_CONFIGURED/UNKNOWN without networking. Both Compose configurations pass the flag; deployment of the changed API/frontend is still required. Existing application migrations must be handled under their own release boundaries, not by this weather feature.

The fixed free API is for non-commercial use only, subject to Open-Meteo's limits and terms. The UI attributes Open-Meteo; data is provided under CC BY 4.0. Before production use, confirm non-commercial eligibility, provider limits and attribution obligations and explicitly set `WEATHER_ENABLED=true` only when appropriate. Commercial use requires a licensed provider integration; no paid-endpoint credentials or arbitrary URL setting are implemented here.

Primary references: https://open-meteo.com/en/docs and https://open-meteo.com/en/terms. A live contract check on 2026-10-04 confirmed the requested units, Unix model timestamp and 900-second current interval. It was a provider smoke test, not validation of local conditions or official flood evidence.

## Verification

Focused Go tests cover request shape, missing/null fields, unit/range/time/grid validation, oversized/malformed responses, cache reuse, concurrent refresh coalescing, cooldown, stale/expiry/recovery, cancellation, public access, disabled state and rate limiting. `TEST_LIVE_WEATHER=true go test ./internal/weather -run TestOpenMeteoLive -count=1 -v` opts into a real public provider check; normal test runs skip networking. No credentials are needed for the non-commercial endpoint.

Focused weather/Overview/realtime frontend checks pass 35 tests. Headless Edge verification covers 320/390/768/1440 widths, TH/EN, loading, unavailable, stale, error/retry, provider-link visibility and 200% mobile text; screenshots use synthetic intercepted responses in `.tmp/weather-ui`. See the weather section of `TODO.md` for whole-workspace failures and the deployment boundary; a standalone Vite bundle is not a passing full typechecked production build.
