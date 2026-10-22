# Warehouse Receiving & Inventory Tracking System — Specification Pack
This workspace contains **locked documentation and agent instructions**, plus unreviewed partial backend work. It is not a completed application. Backend work is stopped and preserved for separate user review; only documentation validation is authorized at the end of Task 1B.

## Documents
- `docs/prd.md` — English PRD / business source of truth.
- `docs/technical-spec.md` — architecture, database, concurrency, APIs, flows, deployment.
- `docs/erd.mmd` — Mermaid ERD.
- `docs/openapi.yml` — locked OpenAPI 1.1.0 contract, including exact envelopes, pagination and auth transport.
- `docs/conventions-audit.md` — inspected references and approved Task 1B resolutions.
- `docs/task-1b-report.md` — full change inventory, documentation validation and commit/implementation status.
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
5. Await explicit user approval before resuming Prompt 2 or committing/pushing. Prompts 2 and 3 remain later implementation phases with reviews and tests.
6. Add actual application code, migrations, Dockerfiles/Compose, `.env.example`, and a real deployment README. Record tests, limitations, and Git commits.

## Assessment
Assessment asks for PRD, technical spec, ERD, API contract, data-flow, AI workflow and >=3 meaningful prompts, critical tests, working application, reproducible Docker deployment, health endpoint, and meaningful Git history. The documentation is available; the preserved partial backend has not been built or tested. Code/deployment/test evidence remains to be produced and verified in separately authorized implementation work.

## Locked implementation conventions
Canonical 22-character Base64URL UUID v4 IDs; clone-style success/nested error envelopes and management pagination; strict binding plus validator/service checks; auth-session JWT and Redis/cookie revocation; versioned SQL migrations; typed React fetch with shared refresh and one retry. Exact contracts are in `docs/openapi.yml` and `docs/technical-spec.md`. No unresolved convention conflict remains after Task 1B.
