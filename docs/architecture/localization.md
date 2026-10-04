# Thai and English localization

## User flow

1. Open any current public or administration route. Thai is the default.
2. Select TH or EN. System labels, navigation, validation, status explanations, dialogs created afterward, accessibility names, dates, and numbers use the selected language.
3. The selection is saved under `floodwatch.locale` in this browser. Reloads and navigation retain it; other tabs on the same origin receive storage updates. Blocked storage does not prevent switching during the session.
4. Switching does not submit or clear a form, reset map filters or route drafts, or refetch data merely to translate its labels.
5. Original reports, names, and announcement text remain unchanged. The UI identifies original content rather than claiming an automatic translation. OFFICIAL, COMMUNITY, AI, and UNKNOWN keep their existing meaning.

Current public notifications are an in-page feed available without login, with browser-local read state. This language work preserves that public/guest policy. Legacy `/login` and `/register` destinations redirect to `/admin/login`; localization does not re-enable public registration.

## Structure and copy

`apps/web/src/i18n/index.tsx` owns `LanguageProvider`, `LanguageSwitcher`, `useTranslation`, and shared date/number helpers. Catalogs are separated by locale and feature under `apps/web/src/i18n/locales/{th,en}`. English catalogs use the Thai key set as their type contract. Catalog tests verify matching keys and interpolation placeholders, plus nonempty translations.

Add both language values when adding system copy. Store validation state as message keys and parameters where it must update after switching; do not store a pretranslated error string. Do not use translated labels as API enums or data identifiers. The HTML language, title, and description track the selection. Date formatting retains Asia/Bangkok: Thai uses `th-TH`, English uses `en-GB`.

Password rules remain UTF-8 length 12–128 bytes in the client and API. The primary hint recommends a memorable phrase, explains the 12–128-character range for English letters/numbers/common symbols, and explicitly allows Thai and emoji. Short/long errors explain the corrective action without byte terminology. Precise UTF-8 detail is available in an expandable explanation; this is a copy change, not a password-policy change.

The legacy standalone `RiskPage` is not routed. `/risk` redirects into the localized map risk panel; do not restore the obsolete page as part of localization.

## Existing Web Push subscriptions

Migration 17 adds device subscription `locale`, defaulting existing records to `th` and allowing only `th` or `en`. Explicit enrollment sends the current locale. Authenticated background synchronization uses update-only `PATCH /api/v1/notifications/language` with `{endpoint, locale}`. It updates only a subscription owned by the current principal; missing and foreign subscriptions both return 404. It cannot enroll a browser, restore a removed subscription, transfer ownership, or replace encryption keys. Locale changes increment subscription version to fence stale delivery cleanup. Requests are serialized, obsolete queued choices are ignored, and mutation requests do not replay after a 401/account refresh.

The worker reads the subscription locale when claiming a delivery and sends a translated system summary with severity, provenance, and a link to original details. It does not fabricate translations of the original announcement. A delivery already claimed may still use the language captured at claim time. The service worker uses locale-aware fallback text. Sync failures offer retry rather than hiding the failure. The current public-only notification page does not expose a Push enrollment control; this path supports existing/explicitly enrolled authenticated subscriptions without changing that policy.

## Deployment and verification

Apply migration 17 before deploying the rebuilt API and worker, then deploy the web assets/service worker. Do not deploy SQL-reading code against the old schema. Roll back the API/worker first before removing the locale column; migration 17 down discards saved Push language preferences. Browser preferences are separate and remain in local storage. No migration or service restart on the application database is implied by source implementation.

Run the existing web typecheck, build, catalog/component tests, and notification Go tests. PostgreSQL tests require `TEST_DATABASE_URL`; use an isolated database for owner, missing-subscription, revision, persistence, and latest-locale checks. Render both languages on mobile and desktop, including errors, original content, draft preservation, keyboard switching, and enlarged text. Intercepted verification data is synthetic, not live flood information. Real provider delivery, physical devices, and Safari need separate verification.
