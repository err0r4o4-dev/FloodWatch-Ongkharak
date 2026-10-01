# Flood Watch Ongkharak engineering rules

- Read relevant source, documentation, `TODO.md`, and Git status before editing; preserve other work.
- `TODO.md` defines the current execution phase. Never start a later phase automatically.
- Select only the smallest relevant set of skills in `.agents/skills/`; do not load every skill.
- Keep the React application, single Go module with API and worker commands, and internal-only Python AI service separate.
- Add focused tests for meaningful behavior and report failed or skipped checks honestly.
- Keep `OFFICIAL`, `COMMUNITY`, and `AI` provenance distinct. Stale or unavailable data is `UNKNOWN`, never implicitly `SAFE`.
- Never invent live flood data, hide errors, commit secrets, or add dependencies without a clear purpose.
- Document architecture changes and avoid destructive Git operations without explicit instruction.
