# Reference conventions — locked

Prompt 1 inspection: 2026-10-22. Agent Task 1B resolution: 2026-10-22. Agent: Codex. Status: **approved convention conflicts resolved; contracts locked at OpenAPI 1.1.0**. The user approved the decisions below. PRD receiving semantics are unchanged.

## References and approved decisions

All reference paths are under `/home/tyomaans/Projects`. The warehouse contract is `docs/openapi.yml`.

| Concern | Actual reference | Locked convention |
|---|---|---|
| Go architecture | `auth-session/internal/auth-session/{domain,user,session,token}` and `main.go` | Feature-first packages own handler/service/repository/DTO/storage/mapper files; domain owns entities/repository interfaces; injected NewX constructors and early error returns. No global layer buckets. |
| IDs | `auth-session/internal/auth-session/user/mapper.go:133`; `search-feature-node-js/backend/src/search-feature/product/product-validator.ts:125` | PostgreSQL UUID v4 PK/FK; public REST IDs are canonical unpadded Base64URL of 16 binary bytes (22 characters). Decode, check version/variant, re-encode exactly. Reject textual UUID inputs and padding. Runtime v4 enforcement is the warehouse adaptation required by the technical spec. |
| Binding/validation | `clone-mini-erp/backend/pkg/request.go`; auth DTO tags and service validation | Strict non-null single-object JSON, 1 MiB limit, UTF-8, unknown/trailing-value rejection, validator tags and service business validation. Useful public field/path messages, without SQL/internal details. No invented field-error map. |
| API envelopes | `clone-mini-erp/backend/pkg/error.go` | Success: `{"success":true,"message":"optional message","data":<payload>}`. Error: `{"success":false,"error":{"code":"...","message":"..."}}`. The user-approved nested error object retains the reference's outer success flag. All handlers/middleware/frontend use this format. Logout is empty HTTP 204. |
| Pagination | `clone-mini-erp/backend/internal/catalogue/product/{dto,query}.go` | Authenticated management defaults page=1, limit=10; maximum page=10000 and limit=100. data.collection with sibling data.meta: current_page, total_pages, page_size, total_data. Collections: products, receivings, transactions, balances, movements, audit_logs, notifications. No extra nesting or catalogue metadata. Empty arrays are []; total_pages=0 when empty. |
| JWT/Redis sessions | `auth-session/internal/auth-session/{token,session}/service.go`, `user/handler.go`, middleware and `.env.example` | HS256 access JWT, 15 minutes; random hex refresh tokens stored by SHA-256 hash; Redis JSON sessions, hashed session keys and per-user session sets; locked rotation and session checks on protected requests. Revocation invalidates existing access JWTs. Retain reference JWT internal UUID claims; public DTO/cookie IDs use Base64URL. Do not copy clone's different refresh-token/Redis format. |
| Cookies | Same auth handler and reference expiry example | Host-only HttpOnly refresh_token at /api/v1/auth and session_id at /api/v1; SameSite=Strict, Secure in production. Default short session 1h; remember_me session 168h, configurable. Both cookies required for refresh; bearer plus both required for logout. Rotate cookies, clear at identical paths, never put refresh token in JSON. |
| PostgreSQL/migrations | `clone-mini-erp/backend/internal/catalogue/product/repository.go`, `migrations/{001_catalogue.sql,migrate.go}` | GORM context-bound transactions, SQL constraints and SQLSTATE translation; numbered SQL with SHA-256 checksums and PostgreSQL advisory locking. No AutoMigrate. Migration bookkeeping gets a UUID PK. Warehouse RCV locking and inventory atomicity take precedence over catalogue concurrency. |
| Frontend | `web-profile/web-profile-frontend/src/projects/auth-session/{routes.tsx,utils/api.ts,components/ProtectedRoute.tsx}` | Feature-scoped React/TS, typed fetch, in-memory access token, credentials=include, shared refresh promise and single retry. Extract nested error.code/error.message; handle 204 without JSON. TanStack Query remains required by this project's spec. |

## Resolved contract corrections

- Replaced placeholder wire schemas with exact success/error and named paginated responses. Documented canonical IDs, strict requests, useful validation, and applicable 400/401/403/404/409/413/500/503 responses. Over-limit submission remains **201** with the persisted rejected transaction.
- Documented login user/role, access expiry, refresh/session cookie security and paths, rotation/concurrency, revocation and empty 204 logout. No registration, heartbeat or session-management feature was added.
- Added creation/completion, approval, latest-correction and review-resolution actor/timestamp fields. Full correction history remains immutable audit with reason and before/after data. Correction replaces items in place while retaining TRX/RCV IDs and retained item IDs.
- Ledger responses expose source item, TRX and RCV IDs via joins. Audit responses include actor/event time. Notification deduplication is **UNIQUE(source_event_id, recipient_id)**, with the outbox ID as stable event ID.
- Updated workspace paths and the API filename throughout affected documentation. RBAC still denies Supervisor RCV/TRX creation and Operator product creation/approval/correction/review. Unowned notifications return 404.
- Pending corrections and Operator drafts remain outside the baseline. RabbitMQ/outbox and TanStack Query are specification-driven integrations, not fabricated reference conventions. No chat/WebSocket or unrelated business module is introduced.

## Execution and remaining work

Task 1B changes documentation only. Validation evidence is recorded in `docs/ai-workflow.md`. No genuinely unresolved convention conflict remains. Proceed to Prompt 2 only after OpenAPI validation and cross-document checks succeed; application changes belong to that separate implementation phase. Prompt 3 owns frontend, RabbitMQ worker integration and deployment.
