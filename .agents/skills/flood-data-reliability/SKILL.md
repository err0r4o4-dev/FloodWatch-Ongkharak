---
name: flood-data-reliability
description: Use when modeling, ingesting, deriving, exposing, or displaying Flood Watch flood, water, road, alert, or AI evidence statuses.
---
# Purpose
Ensure safety-relevant information remains source-aware, time-aware, and honest about uncertainty.
# Use When
Changing status rules, freshness windows, provenance fields, aggregation, alert evidence, or degraded-data behavior.
# Do Not Use When
Working on unrelated account preferences, static layout, or infrastructure with no data semantics.
# Required Rules
- Keep `OFFICIAL`, `COMMUNITY`, and `AI` provenance distinct through storage, APIs, and presentation.
- Carry observed time, retrieved time, freshness, confidence where applicable, and explanation of derivation.
- Default missing, stale, contradictory, or unavailable evidence to `UNKNOWN`; `UNKNOWN` is never `SAFE`.
- Do not invent readings, imply certainty, or overwrite raw evidence with derived status.
# Workflow
Identify the evidence chain, define failure and stale cases first, test provenance and status transitions, then review user-facing wording.
# Checklist
- [ ] Every derived claim traces to retained source evidence.
- [ ] Degraded inputs produce visible unknown/unavailable behavior.
# Related Skills
`external-data-ingestion`, `rest-api-contract-design`, `python-fastapi-ai-service`.
