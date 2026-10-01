---
name: apple-responsive-web-ui
description: Use when composing Flood Watch public web layouts or reviewing their responsive information hierarchy.
---
# Purpose
Create calm, readable Apple-inspired layouts without hiding disaster information behind decoration.
# Use When
Building the app shell, navigation, information panels, or responsive public pages.
# Do Not Use When
Changing backend contracts or implementing map geometry; use the dedicated skills.
# Required Rules
- Prioritize source, observation time, severity text, and action over decorative chrome.
- Use shared spacing/type/surface tokens; keep data panels opaque enough for contrast.
- Progress from narrow touch layouts to wider multi-column views without changing meaning.
# Workflow
Sketch semantic content order, choose tokenized layout, then inspect phone and desktop rendering.
# Checklist
- [ ] Primary information is readable without hover or color cues.
- [ ] Navigation and content retain usable whitespace at 320px and wide sizes.
# Related Skills
`design-system-tokens`, `accessible-responsive-ui`, `cupertino-mobile-ux`.
