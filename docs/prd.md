# Product Requirements Document — Warehouse Receiving & Inventory Tracking System
**Version:** 1.0 | **Status:** Implementation baseline | **Language:** English

## 1. Problem Statement
Warehouse inbound receipts are currently recorded in spreadsheets, exposing operations to input errors, duplicate recording, inconsistent inventory balances, and poor traceability. Build a web application to register products/SKUs, define expected inbound quantities, record actual physical receipts, approve valid receipts, and track inventory and history. An external order is a reference only; this system does not manage purchasing or shipping.

## 2. Business Objectives
- Record expected and actual inbound quantities accurately, including partial deliveries.
- Prevent approval of quantities exceeding the remaining expected quantity.
- Ensure inventory increases exactly once for each approved receipt.
- Require supervisor verification for rejected over-receiving attempts.
- Provide traceable approvals, corrections, rejected attempts, and inventory movements.
- Deliver a usable web application with documented APIs, tests, and reproducible deployment.

## 3. Roles
**Warehouse Operator**
- View products, receiving documents, receiving statuses, inventory and relevant history.
- Create receiving documents with external order references and expected quantities.
- Record actual physically received quantities in receiving transactions.
- View notifications addressed to the operator.
- Cannot approve, perform supervisor corrections, or manage products.

**Warehouse Supervisor (SPV)**
- View receiving documents, transactions, inventory and history.
- Create products with unique SKUs.
- Approve valid pending receiving transactions.
- Investigate system-rejected attempts, correct input errors in unapproved transactions, record verification notes, and close confirmed physical discrepancies.
- View notifications addressed to the supervisor.
- Cannot override remaining-quantity limits.

## 4. Functional Requirements
| ID | Requirement |
|---|---|
| FR-001 | List products with unique SKUs. |
| FR-002 | Supervisor creates products; duplicate SKUs are rejected. |
| FR-003 | Operator creates a receiving document (RCV) with one external ORD reference, SKU lines, and expected quantities. |
| FR-004 | One ORD reference maps to one RCV; one RCV may contain many sequential TRXs. |
| FR-005 | List/detail RCV with items, approved totals, remaining quantities, status, and transaction history. |
| FR-006 | Operator records actual physical quantities per SKU; preserve over-limit attempts for review. |
| FR-007 | Validate each SKU against remaining expected quantity. Over-limit attempts become `REJECTED_BY_SYSTEM` and never post inventory. |
| FR-008 | Support multiple partial receipts under the same RCV, each with a new TRX identifier. |
| FR-009 | Allow only one active TRX per RCV: `PENDING_APPROVAL` or unresolved `REJECTED_BY_SYSTEM`. |
| FR-010 | Supervisor approval posts inventory atomically, records approver/time, and recalculates RCV status. |
| FR-011 | Supervisor reviews system-rejected transactions and determines input error vs real physical discrepancy. |
| FR-012 | Supervisor may correct an unapproved TRX in place, retaining TRX/RCV IDs and auditing before/after values, reason, actor, and time. Revalidate after correction. |
| FR-013 | Confirmed physical excess remains rejected, with a verification note and review marked resolved; never posts inventory. |
| FR-014 | Approved TRXs are immutable; repeated approval never posts inventory twice. |
| FR-015 | RCV stays `INCOMPLETE` until approved quantities for every SKU equal expected quantities. |
| FR-016 | RCV becomes `COMPLETED` automatically once every SKU is fully approved; completed RCVs are closed. |
| FR-017 | Show inventory balances derived from approved inbound receipts. |
| FR-018 | Record immutable inventory movement history linked to source TRX/RCV. |
| FR-019 | Audit receiving creation, attempts, rejection, review, correction, approval, and completion. |
| FR-020 | Deliver approval/rejection/review-related notifications via RabbitMQ-backed processing; persist user-visible notifications. No chat or WebSocket. |

## 5. Non-Functional Requirements
- **NFR-001 Authentication:** JWT access token lifetime 15 minutes; Redis-backed hashed refresh tokens with session revocation.
- **NFR-002 Authorization:** Backend-enforced RBAC for Operator and SPV; UI visibility is not authorization.
- **NFR-003 Data integrity:** Database constraints and transactional concurrency protection.
- **NFR-004 Atomicity:** Approval, inventory balance, inventory ledger, RCV status and outbox event must commit together.
- **NFR-005 Auditability:** Persist structured business history; separate system/security logs.
- **NFR-006 Validation:** Enforce all rules server-side independently of the UI.
- **NFR-007 Deployment:** Docker Compose, PostgreSQL, Redis, RabbitMQ, Nginx, migrations, environment configuration, health checks.
- **NFR-008 APIs:** Documented REST API and consistent request/response/error contracts aligned to reference projects.
- **NFR-009 Observability:** zerolog for application logging; structured transactional audit stored in PostgreSQL.
- **NFR-010 Usability:** Responsive, accessible, minimal UI with clear form, error, loading, empty, and status states.

## 6. User Stories & Acceptance Criteria
### US-001 Manage Products
As an SPV, I can create products with unique SKUs. **Accept:** duplicates rejected; product appears in list; Operator creation denied.
### US-002 Create RCV
As an Operator, I can create an RCV with ORD reference, SKU lines and positive expected quantities. **Accept:** unique ORD reference, no duplicate SKU lines, initial status `INCOMPLETE`.
### US-003 Partial Receiving
As an Operator, I can submit multiple sequential TRXs against an incomplete RCV. **Accept:** one active TRX at a time; each approved partial receipt updates remaining quantity; no premature completion.
### US-004 Validate Actual Quantities
As an Operator, I receive clear feedback when quantities exceed remaining. **Accept:** over-limit attempt retained as rejected, visible to SPV, no stock movement.
### US-005 Approve Receiving
As an SPV, I approve valid pending TRXs. **Accept:** inventory increases once; approver/time recorded; repeated or concurrent approval does not duplicate stock.
### US-006 Investigate and Correct
As an SPV, I can investigate rejected attempts. **Accept:** correction retains same TRX ID with before/after audit; valid correction returns to pending; confirmed excess remains rejected but review can be closed.
### US-007 Complete RCV
As a user, I see accurate RCV completion. **Accept:** only all-SKU approved equality triggers automatic `COMPLETED`; no new TRX afterward.
### US-008 Inventory History
As an authorized user, I see current inbound inventory and immutable source-linked movement history. **Accept:** pending/rejected transactions never affect inventory.
### US-009 Notifications
As an Operator or SPV, I see notifications for relevant transaction events. **Accept:** role/recipient isolation; persisted notifications; notification failure does not reverse or duplicate stock.

## 7. Business Rules & Validation
| ID | Rule |
|---|---|
| BR-001 | SKU is unique. |
| BR-002 | Expected and actual quantities are positive integers; quantities are not fractional. |
| BR-003 | One external ORD reference per RCV, unique in this application. |
| BR-004 | One RCV may contain multiple TRXs. |
| BR-005 | Approval quantity per SKU cannot exceed remaining expected quantity. |
| BR-006 | Over-limit attempts are persisted as rejected; inventory unchanged. |
| BR-007 | SPV verifies rejected attempts manually; system does not assume the source of discrepancy. |
| BR-008 | SPV correction of unapproved TRX retains IDs and produces an audit record. |
| BR-009 | Approved TRX cannot be edited/deleted. |
| BR-010 | Only SPV can approve. |
| BR-011 | Exactly one inventory posting per approved TRX item. |
| BR-012 | Pending/rejected transactions do not affect inventory. |
| BR-013 | Every SKU's approved sum must equal expected before RCV completion. |
| BR-014 | Completed RCV cannot accept TRXs. |
| BR-015 | At most one active TRX per RCV (pending or rejected with unresolved review). |
| BR-016 | A subsequent TRX requires approval of pending TRX or resolution of rejected review. |
| BR-017 | Rejected TRX does not make its RCV rejected. |
| BR-018 | External order changes do not automatically modify RCV expected quantities. |
| BR-019 | SPV cannot override expected quantity caps. |
| BR-020 | Notifications are ancillary; inventory correctness never depends on broker delivery. |

## 8. Scope
**In scope:** product listing/creation, receiving documents/items, partial/full receiving, single active TRX, rejection review/correction, SPV approval, automatic completion, inventory balances/ledger, audit history, authentication/RBAC, RabbitMQ-backed notifications, REST API, frontend, Docker deployment.

**Out of scope:** purchase order management, supplier management, shipping, finance, outbound inventory, manual stock adjustments, multiwarehouse, external order synchronization, realtime chat, WebSocket, and digital escalation workflows.

## 9. Assumptions, Constraints & Open Decisions
- One warehouse; two roles; product creation restricted to SPV.
- External ORD is a text reference, not an integrated order object; expected quantities are RCV snapshots.
- Approved quantities are the sole basis for RCV completion and inbound inventory.
- `REJECTED_BY_SYSTEM` uses a separate `review_status` to distinguish unresolved/resolved.
- A resolved rejected TRX remains rejected and cannot later be corrected unless explicitly reopened by a documented change; the baseline API does not implement reopening.
- A new RCV begins `INCOMPLETE`; receiving is recorded only via TRXs.
- No outbound module means inventory is **approved inbound balance**, not necessarily physical on-hand after external withdrawals.
- Base64 UUID serialization, API envelope, validation idioms and feature-first conventions are delegated to local reference projects, subject to the frozen OpenAPI semantic contract.
- **Open policy:** whether SPV may correct a `PENDING_APPROVAL` TRX. Baseline: correction is limited to unresolved rejected TRXs; do not expand without decision.
- **Open policy:** operator edits before submission. Baseline: transaction creation submits immediately; no draft/edit endpoint.
- Target effort 8–12 hours, maximum submission window 3 calendar days. Prioritize core workflow correctness; document limitations.

## 10. Success Criteria
- Every assessment-required core workflow is usable through the UI and API.
- Partial/full receipts, over-limit rejection, single active TRX, approval, and auto-completion follow documented rules.
- No invalid/duplicate/concurrent approval corrupts inventory.
- SPV-only operations are denied to Operator by backend.
- Critical tests, reproducible migrations, Docker startup, and setup documentation are provided.
