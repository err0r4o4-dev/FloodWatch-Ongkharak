# AI service boundary

The FastAPI service is internal-only. Neither browser nor Caddy routes to it; future Go API/worker calls should use bounded timeouts and treat AI output as evidence requiring provenance and verification, never an official observation. No model, inference, CCTV integration, or image pipeline is installed in Phase 0. `GET /health` describes process availability only, not model readiness or result accuracy.
