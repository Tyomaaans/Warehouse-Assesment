# Task 4 — Full E2E test plan (not executed)

Future target: **https://wh.tyomaaans.cloud**. This document is planning only. Task 3D/3E local acceptance and limited deployment smoke do not constitute Task 4 execution.

Prerequisites: successful CI release and public acceptance; operator-owned host Nginx API routing and measured narrow Gin trust; dedicated disposable fixture namespace/accounts; backup/recovery approval for restart tests; no deletion of existing production data. Record the deployed commit/image IDs, browsers, timestamps and isolated fixture IDs. Never capture tokens, passwords or refresh-cookie values in evidence.

| Coverage | Expected result | Verification method |
|---|---|---|
| Operator/Supervisor auth and RBAC | Login/restoration/logout work; backend denies opposite-role writes regardless of UI | Real HTTPS browser sessions plus direct authenticated API denial requests; inspect UI actions |
| Products and RCVs | Unique SKU/ORD; positive exact int64; immutable expected snapshots; correct role creation | Real UI/API creation, duplicate/invalid input and reload checks |
| Partial then full receipt | Approved partial updates inventory, RCV stays incomplete; all-SKU equality alone completes | Two-SKU fixture, UI approvals; compare document, balance and source-linked ledger after each step |
| Over-receiving | Physical excess persists HTTP 201 rejected/unresolved; zero inventory change | Capture exact request/response and stock before/after |
| Correction | Same TRX/RCV, reason/actor/time and before/after audit; revalidate into pending or remain rejected | Supervisor UI plus audit API; approve valid correction and compare stock |
| Confirmed excess review | Rejected/resolved, note/actor/time, no stock; subsequent TRX allowed | UI resolution then new Operator submission |
| One active concurrency | Concurrent submissions produce one active winner; others conflict | Coordinated separate sessions/API calls against a dedicated RCV, query final active state |
| Exactly-once posting | Repeated/concurrent approvals post one ledger source/item and increment once | Coordinated Supervisor calls; verify API/DB evidence under operator-approved read access |
| Outbox/RabbitMQ | Required events commit with business writes; broker/worker failure does not reverse stock | Observe outbox/queue/notification IDs; controlled restart/outage on dedicated approved infrastructure |
| Notification ownership/read | Only own records; cross-recipient read returns404; duplicate delivery retains IDs/read time | Two role accounts, REST polling and direct PATCH; operator-approved duplicate event replay |
| Refresh/revocation | HttpOnly host-only Secure Strict cookies with correct paths; shared refresh/single retry; revoked access rejected | Chrome network/cookie metadata, concurrent expired reads, server logout followed by protected access |
| Persistence and restart | DB, balances, sessions and broker data survive application recreation; deliveries recover safely | Before/after snapshots, authorized process/container recreation; no volume removal |
| HTTPS/Nginx/proxies | Valid public TLS; HTTP redirect; SPA deep links; `/api/v1/` reaches API directly; only actual ingress peer trusted | Browser/curl, certificate/SNI checks, request-ID-correlated TCP peer logs and spoofed-header rejection |
| Responsive/accessibility | Keyboard labels/focus/dialog behavior and no viewport overflow on supported sizes | Desktop/mobile Chrome, Firefox and Safari where available; focused keyboard/a11y review |
| Assessment and Git history | PRD/contracts unchanged, meaningful commits, reproducible commands and honest limits | Traceability review of task reports, Git log, OpenAPI and deployment documentation |

Use small fixtures covering T01–T08 in `docs/testing.md`; avoid broad artificial line-coverage goals. Distinguish actual browser/API/DB evidence from mocks or static checks. Record command, expected/actual result, timestamps, IDs and screenshots without secrets. Skipped/unavailable scenarios remain explicitly incomplete. Stop and obtain operator direction before disruptive public-host operations; never alter host Nginx automatically.
