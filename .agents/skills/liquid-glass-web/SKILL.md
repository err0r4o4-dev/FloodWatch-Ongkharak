---
name: liquid-glass-web
description: Use when adding translucent navigation, overlays, search, map controls, or bottom sheets in the Flood Watch UI.
---
# Purpose
Use subtle liquid-glass layering while preserving crucial data legibility.
# Use When
Designing a floating toolbar, navigation, sidebar, search field, or overlay.
# Do Not Use When
Presenting dense readings, alerts, tabular data, or every ordinary card.
# Required Rules
- Do NOT turn every card into glass; keep important data surfaces solid.
- Tokenize translucency, backdrop blur, border, shadow, and fallback opaque color.
- Check text contrast over both light and busy backgrounds; disable heavy effects where unsupported.
# Workflow
Select a justified overlay surface, apply shared glass tokens, then inspect content overlap and contrast.
# Checklist
- [ ] Background never obscures severity, provenance, or timestamp.
- [ ] Focus and reduced-motion behavior still works.
# Related Skills
`design-system-tokens`, `accessible-responsive-ui`.
