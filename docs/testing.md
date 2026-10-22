# Testing and QA — Critical-Path Only
**Intent:** Satisfy the assessment's required automated business-rule, API validation, stock-integrity, and duplicate-operation evidence **without broad unit-test coverage**.

## Automated tests (small focused suite)
| ID | Level | Scenario | Assertion |
|---|---|---|---|
| T01 | Integration | RCV expects SKU-A 130; approve TRX 70 then TRX 60 | Stock +130 total, RCV incomplete after first approval and completed after second |
| T02 | Integration/API | RCV remaining 10, submit actual 60 | TRX saved rejected+unresolved; inventory unchanged; SPV can inspect |
| T03 | Integration | Concurrent/repeated approval of same pending TRX | Exactly one inventory ledger posting and one stock increment |
| T04 | Integration/API | Create second TRX while first pending or rejected+unresolved | Conflict; no second active TRX |
| T05 | Integration | SPV corrects rejected actual 60 to 10, then approves | Same TRX ID, audit before/after, valid pending then approved, +10 stock |
| T06 | API | Operator calls approve/correct/product-create | 403 and no state changes |
| T07 | API | Invalid SKU line/negative quantity/duplicate SKU or malformed ID | Expected validation/error envelope; no partial writes |
| T08 | Integration | Resolve confirmed physical discrepancy | Rejected remains rejected, review resolved, no stock, next TRX allowed |

If constrained, prioritize T01–T06; T07 covers the assessment's explicit API validation/error tests and must be included before submission. T08 can be demonstrated in the manual walkthrough.

## Manual end-to-end smoke
1. Login as SPV; create a product.
2. Login as Operator; create RCV with expected quantity.
3. Submit partial TRX; SPV approves; inspect inventory and incomplete status.
4. Submit remainder; SPV approves; inspect completed status and inventory ledger.
5. Create another RCV; submit over-limit attempt; verify rejection and zero inventory effect.
6. SPV reviews/corrects or resolves; inspect audit and notification display.
7. Confirm Operator cannot access SPV-only operations.

## Evidence to submit
- Locked validation errors use `{"success":false,"error":{"code":"invalid_input","message":"useful public validation message"}}`; role denials use 403/code `forbidden`. T07 includes canonical Base64URL ID validation and strict unknown-field rejection; quantities remain positive int64 integers.
- Commands used to run tests and concise pass/fail results.
- Before/after inventory balance and ledger evidence for T01–T03.
- Error status/body for validation and RBAC checks.
- Short known limitations list; do not claim full test coverage.

## Explicit exclusions
No exhaustive testing of every UI component, RabbitMQ retry permutation, styling variant, or generic utility. Focus on receiving correctness, inventory integrity, critical authorization, and API validation.

## Task 4 execution evidence

See [task-4-report.md](task-4-report.md) for the complete public HTTPS matrix, fixture identifiers, exact commands/results, deployment status and acceptance recommendation. Production Chrome covered both roles at 1440×1000 and 390×844, using only `TASK4-20261023-*` synthetic business records and their own notifications. Natural signed access-token expiry was observed after 900 seconds; the expired token returned 401, scheduled refresh succeeded and logout returned 204. No production fault injection or infrastructure restart was used for recovery testing.

Local full regression passed 46 frontend unit/component tests and actual PostgreSQL/Redis/RabbitMQ Go tests with no integration skips, plus race checks, OpenAPI/response validation and production-container Chrome flows. Actual broker outage/recovery, worker restart, application recreation/persistence, database rollback failures and notification redelivery/idempotency run only on isolated local infrastructure. Deployment rollback simulations are reported separately from real recovery evidence.

Responsive production checks found a real defect: an absolutely positioned screen-reader label in an inventory table escaped the scrolling container and widened a 390px document to 557px. The focused fix establishes that container as a positioning context. `frontend/tests/inventory-notifications.spec.ts` adds a real-browser regression asserting document width before/after table scrolling while preserving the accessible history-link name. The report records its failing pre-fix production run, corrected local results and post-deployment result; final acceptance requires the last of these.

The owner accepts the open non-blocking Cloudflare Analytics/JavaScript Detections CSP conflict. Preserve CSP/Bot Fight Mode and do not claim full validation of bot-detection integration. This finding is separate from application authentication or business correctness.

Task 4-Recovery closes the deployment/layout blocker: workflow 39869194961 attempt 2 succeeded using the previously verified artifact. The committed regression passes against public HTTPS (1 test / 5.4s); both-role supplemental checks and 9 focused production groups pass. Public assets match the repaired build, all seven services are healthy, volumes/migrations persist, and ten pre-existing business-table fingerprints remain unchanged after the isolated new-fixture smoke. Recovery captures validate 25 responses / 4 payloads / 194 canonical IDs across 11 operations. The original timeout was pre-authentication TCP connectivity; exact network attribution remains unproven. Existing full regression/natural-expiry evidence remains valid for unchanged code. Final assessment readiness is **yes**, with CF-01 and the report's stated limitations retained; no skipped or unexecuted case is counted as a pass.
