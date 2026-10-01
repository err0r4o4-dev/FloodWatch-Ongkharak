---
name: python-fastapi-ai-service
description: Use when changing Flood Watch's internal Python FastAPI AI service, model adapters, inference endpoints, evidence payloads, or service-to-service integration.
---
# Purpose
Keep AI inference internal, bounded, observable, and separate from verified flood evidence.
# Use When
Adding FastAPI routes, model invocation, image/video processing, AI result schemas, or Go-to-AI service calls.
# Do Not Use When
Building public browser APIs, final public safety statuses, or general Go API handlers.
# Required Rules
- Keep the service internal-only; the browser never calls it directly.
- Validate media and request sizes, enforce timeouts, and return typed failure states for model or dependency errors.
- Return model output with model/version, input reference, inference time, confidence, and limitations.
- Classify AI output as `AI` provenance; it never replaces `OFFICIAL` evidence or converts uncertainty into `SAFE`.
# Workflow
Define the internal contract, test valid/invalid/dependency-failure paths, then verify network exposure and resource limits.
# Checklist
- [ ] No model secret, raw private media, or internal traceback leaks in responses.
- [ ] Go callers handle unavailable AI results without fabricating status.
# Related Skills
`flood-data-reliability`, `backend-security-testing`, `docker-environment-infrastructure`.
