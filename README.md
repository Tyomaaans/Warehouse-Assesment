# Warehouse Receiving & Inventory Tracking System — Specification Pack
This package contains **documentation and agent instructions**, not a completed application. It is intended as the specification-driven input for an AI coding agent.

## Documents
- `docs/prd.md` — English PRD / business source of truth.
- `docs/technical-spec.md` — architecture, database, concurrency, APIs, flows, deployment.
- `docs/erd.mmd` — Mermaid ERD.
- `docs/openapi.yml` — locked OpenAPI 1.1.0 contract, including exact envelopes, pagination and auth transport.
- `docs/conventions-audit.md` — inspected references and approved Task 1B resolutions.
- `docs/data-flow.mmd` — Mermaid business flow.
- `docs/rbac.md` — explicit role/action matrix.
- `docs/testing.md` — small critical test suite and evidence plan.
- `docs/ai-workflow.md` — reference inspection rules, 3 prompts, agent log.
- `AGENTS.md` — top-level agent guardrails.

## Agent handoff
1. Place docs in target repository.
2. Reference projects are under `/home/tyomaans/Projects`: `auth-session`, `clone-mini-erp`, `web-profile/web-profile-frontend`, and `search-feature-node-js`.
3. Prompt 1 inspected actual conventions; Task 1B resolved the differences. See `docs/conventions-audit.md`.
4. Validate the locked `docs/openapi.yml` and maintain cross-document consistency **before** implementation.
5. Execute Prompts 2 and 3 with reviews and tests.
6. Add actual application code, migrations, Dockerfiles/Compose, `.env.example`, and a real deployment README. Record tests, limitations, and Git commits.

## Assessment
Assessment asks for PRD, technical spec, ERD, API contract, data-flow, AI workflow and >=3 meaningful prompts, critical tests, working application, reproducible Docker deployment, health endpoint, and meaningful Git history. This archive addresses **documentation only**; code/deployment/test evidence must be produced by the implementing agent and verified by the developer.

## Locked implementation conventions
Canonical 22-character Base64URL UUID v4 IDs; clone-style success/nested error envelopes and management pagination; strict binding plus validator/service checks; auth-session JWT and Redis/cookie revocation; versioned SQL migrations; typed React fetch with shared refresh and one retry. Exact contracts are in `docs/openapi.yml` and `docs/technical-spec.md`. No unresolved convention conflict remains after Task 1B.
