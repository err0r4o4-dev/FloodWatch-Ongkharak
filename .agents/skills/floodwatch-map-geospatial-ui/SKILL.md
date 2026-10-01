---
name: floodwatch-map-geospatial-ui
description: Use when implementing future Leaflet map layers, GeoJSON handling, markers, road status, or route-mode interactions.
---
# Purpose
Keep map layers independently understandable, source-aware, and safe under missing data.
# Use When
Phase 5+ introduces flood, water, risk, road, CCTV, community, safe-area, or boundary layers.
# Do Not Use When
Working on the Phase 0 shell or claiming a road is safe from absent data.
# Required Rules
- Separate `SITUATION` and `ROUTE` modes; keep layer visibility and legend synchronized.
- Validate GeoJSON coordinates and geometry at boundaries; specify CRS and spatial extents.
- Display status text, source, freshness, and an accessible non-map alternative.
- Treat `UNKNOWN` roads as unknown, not passable, even if geometry renders normally.
# Workflow
Define layer contract, test semantics and stale/unknown states, render at touch and desktop sizes.
# Checklist
- [ ] Markers and controls have keyboard/touch equivalents.
- [ ] Map never asserts route safety from incomplete evidence.
# Related Skills
`postgresql-postgis-backend`, `flood-data-reliability`, `accessible-responsive-ui`.
