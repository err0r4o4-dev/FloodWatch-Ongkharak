---
name: accessible-responsive-ui
description: Use when implementing or verifying Flood Watch visible UI across keyboard, screen reader, motion, and viewport sizes.
---
# Purpose
Ensure urgent information remains perceivable and operable for diverse users.
# Use When
Shipping visible frontend changes, navigation, statuses, controls, or overlays.
# Do Not Use When
Making isolated backend-only changes with no rendered UI.
# Required Rules
- Use semantic elements, visible focus, sensible heading order, and ARIA only where necessary.
- Pair severity color with Thai text/shape; verify readable contrast and reduced motion.
- Test narrow/wide viewports, 44px mobile targets, zoom, keyboard traversal, and overlay dismissal.
# Workflow
Inspect rendered default, loading/error, and interaction states; fix failures before delivery.
# Checklist
- [ ] Keyboard reaches and identifies every action.
- [ ] No critical meaning depends on color alone.
# Related Skills
`design-system-tokens`, `cupertino-mobile-ux`, `liquid-glass-web`.
