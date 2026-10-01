---
name: cupertino-mobile-ux
description: Use when designing Flood Watch phone navigation, safe-area spacing, touch controls, or bottom sheets.
---
# Purpose
Make emergency information usable on small touch screens.
# Use When
Adding mobile navigation, drawers, map overlays, or sheets.
# Do Not Use When
Only changing desktop data contracts.
# Required Rules
- Account for safe-area insets and browser UI; do not place controls under them.
- Prefer at least 44px touch targets and keep critical actions reachable without precision taps.
- Preserve keyboard alternatives, focus order, and scroll trapping behavior in overlays.
# Workflow
Lay out 320px-first, test touch/keyboard opening and dismissal, then verify landscape/wide layout.
# Checklist
- [ ] Tap targets and close controls are usable.
- [ ] Long Thai labels wrap instead of clipping.
# Related Skills
`apple-responsive-web-ui`, `accessible-responsive-ui`.
