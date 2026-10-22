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
