---
name: design-system-tokens
description: Use when adding or changing Flood Watch colors, spacing, typography, surfaces, status treatments, or motion.
---
# Purpose
Keep UI values centralized and semantic across the React application.
# Use When
Changing visual primitives or mapping flood/road statuses to presentation.
# Do Not Use When
Changing domain values or persisting CSS colors in the API/database.
# Required Rules
- Define base blue palette, semantic status colors, surface/glass layers, type, spacing, radii, shadows, borders, blur, motion, z-index, and responsive thresholds in the token layer.
- Preserve `UNKNOWN` as its own neutral treatment; never style it as `PASSABLE` or safe.
- Map canonical status strings to presentation in one place; never use a color name as a domain status.
# Workflow
Inspect existing tokens, add the smallest semantic token, consume it in components, verify contrast.
# Checklist
- [ ] No unnecessary hard-coded duplicate visual value in components.
- [ ] Status label and icon/shape complement color.
# Related Skills
`apple-responsive-web-ui`, `flood-data-reliability`, `accessible-responsive-ui`.
