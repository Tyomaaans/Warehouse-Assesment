# AI-Assisted, Specification-Driven Workflow
## Required source hierarchy
1. Technical assessment: mandatory constraints/deliverables.
2. `docs/prd.md`: business behavior.
3. `docs/technical-spec.md`, `docs/erd.mmd`, `docs/openapi.yml`, `docs/data-flow.mmd`, `docs/rbac.md`: implementation contracts.
4. Local reference projects: code style, feature-first structure, UUID/Base64 encoding, auth/session, validation, PostgreSQL/Redis, API envelopes, request/response DTOs, search/pagination patterns.
5. Agent prompts: task scope only; prompts may not silently override contracts.

## Reference-project inspection (agent's responsibility)
Read `/home/tyomaans/Projects/auth-session`, `/home/tyomaans/Projects/clone-mini-erp`, `/home/tyomaans/Projects/web-profile/web-profile-frontend`, `/home/tyomaans/Projects/search-feature-node-js` when present in the agent's environment. Inspect **actual code** before reuse. Follow relevant conventions but do not copy unrelated ERP/business modules. Record what was reused and from which project. If a reference is unavailable, report it and request direction rather than inventing its conventions.

**Critical rule:** Reference implementations determine serialization/style details only when compatible with frozen business semantics. Update OpenAPI to match the discovered envelope and Base64 format *before* coding; document any contract adjustments. Do not let reference business workflows override warehouse rules.

**Task 1B approved decisions:** See `docs/conventions-audit.md` and locked OpenAPI version 1.1.0. The user resolved the previously reported envelope, binding, pagination, identifier and notification-deduplication differences. Use those resolutions; do not stop again on historical reference differences. Task 1B edits documentation only. Once OpenAPI and consistency validation pass, proceed to Prompt 2 as a separate phase; Prompt 3 remains the frontend/notification-worker/deployment phase.

## Three meaningful agent task prompts (record execution/results)
### Prompt 1 — Inspect and lock conventions
> Read the assessment, `docs/prd.md`, `docs/technical-spec.md`, `docs/rbac.md`, `docs/openapi.yml`, and reference projects under `/home/tyomaans/Projects`. Identify feature-first layout, UUID v4 Base64 representation, validation patterns, auth/session revocation, response envelopes, and PostgreSQL/Redis conventions. Produce a short mapping of references to conventions and any conflicts. Do not implement code or invent missing reference conventions. Propose only contract adaptations necessary to preserve the PRD.

### Prompt 2 — Implement critical backend
> Using the locked PRD and technical spec, implement products, RCV/TRX, SPV review/correction/approval, inventory ledger/balances, RBAC, migrations, and REST endpoints in the established feature-first Go/Gin style. Preserve single active TRX per RCV and atomic exactly-once inventory posting. Add only critical tests T01–T07. Do not introduce order management, outbound, chat, or unrelated features. Report tests and deviations.

### Prompt 3 — Integrate frontend, notifications, deployment, verify
> Implement React/TypeScript UI for the documented flows, minimalist responsive UX, TanStack Query API integration, and role-aware navigation. Integrate RabbitMQ outbox-backed notifications without WebSocket or chat. Add Docker Compose, Nginx, PostgreSQL, Redis, RabbitMQ, migrations, health endpoint and clean-start README. Run the critical suite and a short end-to-end smoke test. Report pass/fail evidence, known limitations, and remaining work. Do not claim tests passed without executing them.

## Agent rules
- Read specs first; map each implementation change to an acceptance criterion.
- Follow user-selected stack; do not remove Redis/RabbitMQ/Nginx because they seem excessive.
- Enforce RBAC on backend, not UI alone.
- Never update inventory on pending/rejected receipts.
- Do not add business features outside the PRD.
- For ambiguity/conflict, ask or document an explicit decision; do not silently guess.
- Keep meaningful Git commits and a short agent decision log.
- User/developer reviews generated code and can explain design during technical review.

## Execution log template
| Prompt | Agent/tool | Date | Files changed | Tests executed/results | Reference patterns used | Deviations |
|---|---|---|---|---|---|---|
| 1 | Codex | 2026-10-22 | `docs/conventions-audit.md`, `docs/ai-workflow.md` | Not run; inspection only | Inspected actual auth-session, clone-mini-erp, nested web-profile frontend and search-feature-node-js code | Historical reference differences subsequently resolved by user-approved Task 1B |
| 1B | Codex | 2026-10-22 | `AGENTS.md`, `README.md`, technical spec, OpenAPI, ERD, RBAC, flow, conventions audit and workflow | OpenAPI validator 0.9.0: OK; 330 cross-document assertions pass across 21 operations; PRD business text unchanged | Applied approved feature-first/auth-session, strict binding/clone envelopes and migrations, frontend client conventions | Documentation-only contract phase; no unresolved convention choices |
| 2 | TBD | TBD | TBD | TBD | TBD | TBD |
| 3 | TBD | TBD | TBD | TBD | TBD | TBD |

**Note:** Only completed execution rows are evidence. Prompt 2/3 rows remain pending until those phases actually execute. Update commands/results after real validation.

### Task 1B validation evidence
- `PYTHONPATH=/tmp/wh-contract-validation python3 -m openapi_spec_validator docs/openapi.yml` — **OK**, using openapi-spec-validator 0.9.0 installed in a temporary tool directory.
- `python3 /tmp/wh-check-contracts.py` — **330 assertions passed**, covering local references, 21 unique operations, error responses, strict request schemas, actor/timestamp fields, ledger source links, canonical ID example, all seven pagination shapes, corrected paths/filenames and composite notification uniqueness.
- PRD body matched the original specification pack; only its pre-existing version label differs. No receiving business rule was changed.
- Task 1B created/modified documentation only. No application tests or application code belong to this phase.
