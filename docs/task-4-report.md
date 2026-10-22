# Task 4 — Production acceptance and assessment readiness

Date: 23 October 2026 WIB (22 October UTC). Target: **https://wh.tyomaaans.cloud**. This report records actual public Chrome tests, separate isolated local regression/failure tests, and read-only production infrastructure inspection. It does not claim exhaustive browser, load, or penetration testing.

## Baseline and boundaries

Started with clean `main`, expected `origin=https://github.com/Tyomaaans/Warehouse-Assesment.git`, and HEAD `9da0948084d731fd9504716a152f7a9381418bd5`. The initial deployed implementation was `7c855aed8922c8f2d274c7954a270d878b24edbc`, release `7c855aed8922c8f2d274c7954a270d878b24edbc-39858680963-1`. Read AGENTS, README, PRD, technical specification, OpenAPI, ERD, data flow, RBAC, testing plan, Task 4 plan, deployment/3D–3E reports and AI workflow; reviewed existing feature/client/handler, browser, integration, migration and deployment tests.

Production writes were restricted to uniquely named `TASK4-20261023-*` synthetic products/documents/transactions, their generated notifications, and test authentication sessions. Baseline SQL confirmed no production business rows existed before this run. No pre-existing business record was edited. No production infrastructure fault, restart for failure testing, volume deletion, Nginx/Cloudflare/secret change, contract modification or new feature was performed. All authenticated production browser sessions created by the successful and failed harnesses were logged out during completion/cleanup; the natural-expiry session also logged out.

## Production browser matrix

Chrome **155.0.8059.39**, actual public HTTPS and backend, desktop **1440×1000**, mobile **390×844**. No intercepted/mocked business responses. Approved provisioned Operator and Supervisor credentials were read privately over the existing SSH connection into process memory; tokens/passwords/cookie values are excluded from evidence and this report.

| Area / acceptance case | Actual evidence | Result |
|---|---|---|
| Anonymous protection and invalid login | Protected UI redirects to login; required-field feedback/focus; incorrect password returns401 and generic visible feedback | Pass |
| Both-role login/restoration | Login200, reload refresh200, access lifetime900s, refresh value rotates while session ID is retained | Pass |
| Cookie/session transport | Two host-only Secure/HttpOnly/SameSite=Strict cookies at `/api/v1/auth` and `/api/v1`; no tokens in web storage | Pass |
| Natural access-token expiry and scheduled refresh | Natural signed expiry at22:57:01UTC; expired token401, one successful scheduled refresh, reload restoration and logout204 | Pass |
| Logout/revocation | Both roles logout204, cookies removed, old access rejected401; account-switch sessions also cleaned up | Pass |
| Verified roles and backend guards | Operator lacks Supervisor navigation/actions; Supervisor cannot create RCV; Operator product-create/audit/approve/correct/review403, Supervisor RCV/TRX-create403 | Pass |
| Supervisor products | Real UI creates two uniquely named products, missing fields show feedback, duplicate SKU409 is shown | Pass |
| Operator document creation | Mobile UI creates expected A10/B5, duplicate SKU option disabled, zero input rejected, initial INCOMPLETE and exact snapshots verified after reload | Pass |
| ORD and validation constraints | Duplicate ORD409; empty/duplicate/unknown SKU items, zero/negative/fractional quantities, unknown actor field and malformed ID400 with nested errors | Pass |
| Valid partial physical receipt | A4 persists PENDING_APPROVAL; no inventory row/posting before approval; second active attempt409 | Pass |
| Initial approval | Confirmation supports Cancel/Escape; Supervisor approval posts A4 once, actor/time recorded; RCV remains INCOMPLETE with B outstanding | Pass |
| Approved immutability | Repeated approval and attempted correction/review409; approved UI is immutable | Pass |
| Persisted over-receiving | Actual A20 returns201 REJECTED_BY_SYSTEM/UNRESOLVED, displayed as saved; inventory remains A4, further active attempt409 | Pass |
| Supervisor cap enforcement | Correction20→7 remains rejected because remaining A is6; stock does not move | Pass |
| Correction in place | Correction7→6 uses same TRX, RCV and retained item IDs; reason/actor/time and before/after snapshots recorded; PENDING_APPROVAL, then approval makes A10 | Pass |
| All-SKU completion rule | A fully approved with B0 still leaves RCV INCOMPLETE; pending quantities never count as approved | Pass |
| Exact int64 excess/review | Browser submits9223372036854775807 exactly for B;201 rejected; Supervisor resolves with note/actor/time; rejection retained and B stock remains0 | Pass |
| Subsequent receipt/final completion | After resolution Operator records B5; final approval alone sets COMPLETED, all remaining quantities0 and completed_at set; subsequent submission409 | Pass |
| Audit | Main document has15 records, including two immutable correction snapshots20→7→6, three approvals, review and completion, with authenticated actors/timestamps | Pass |
| Concurrent active creation | Two simultaneous requests for a dedicated A2 RCV yield201/409 and one active winner | Pass |
| Concurrent approval | Two simultaneous approvals yield200/409; further repeat409, one additional ledger posting | Pass |
| Inventory | Actual balance/ledger data and SKU/source joins; original balances A12/B5 and ledger rows A3/B1 preserved. Mobile checked again after deployment | Pass after Recovery; UI-01 closed |
| Outbox/consumer delivery | Read-only SQL finds12 fixture events, all published; all four approved TRXs have approval events;20 notifications (Operator 12 / Supervisor 8) | Pass |
| Ownership and read state | Cross-recipient PATCH404 both ways; UI marks only this run's notification read; repeated PATCH preserves first read_at | Pass |
| Real notification polling | Visible mobile view generates successful REST GET after59,687ms; no WebSocket/SSE | Pass |
| Dashboard | For each role all four displayed counts match server `meta.total_data`; previews are not used as totals | Pass |
| Account changes | Same browser changes Operator→Supervisor; delivered rows match the new recipient and exclude the old account's notification IDs | Pass |
| Mobile/accessibility | RCV/receipt/correction/approval/notifications and navigation focus pass. Original inventory390→557px defect repaired; deployed document remains390px before/after table scrolling | Pass after Recovery |
| Additional read/contract checks | Both roles pass list/detail/status/pagination/empty/mobile checks, canonical missing-ID404 and forged-role JWT401 in the Recovery rerun | Pass after Recovery |

Screenshots of production product/receiving/correction/inventory/notification states were captured outside the repository, without login fields or secret values. Mobile correction and post-repair inventory screenshots were visually inspected; tables scroll within their containers rather than widening the document.

## Durable business evidence

Successful fixture namespace: `TASK4-20261023-c26d0e`.

| Fixture | Public identifier / outcome |
|---|---|
| Product A | `NP99CtyuQA2-wrGN4PqPUw`; final balance12, three ledger rows |
| Product B | `hGjNyr58QLWV8sAVujVMNw`; final balance5, one ledger row |
| Main RCV | `BxElidH2TT-LEFVZJM4nnw`; ORD suffix `-ORD`; expected A10/B5, COMPLETED |
| Partial TRX | `4Scn_I4VSYKOs9QoGUVEJg`; A4 APPROVED |
| Corrected TRX | `cyqraEE_SVG2gPy9935dzw`; A20→7→6, same ID; APPROVED |
| Resolved excess TRX | `eYGtSYPWS1qjDlUoKn-EXw`; B max-int64, REJECTED_BY_SYSTEM/RESOLVED; zero stock |
| Final TRX | `DJk1rBNBQBC7LR7nPx8Ztg`; B5 APPROVED |
| Concurrency RCV/TRX | `5ClyBNfARC2UnTSf3n2yFQ` / `Sim2D37yTuK7a7KIR7qjHw`; A2 once, COMPLETED |

Read-only production SQL independently confirms all ledger sources are approved and unique, all 12 source events are published, four approvals have four matching approval events, no duplicate `(source_event_id, recipient_id)` pairs exist, and no TASK4 transaction remains active. Production atomicity is supported by these committed relationships; rollback at intermediate failure points is tested locally, not fault-injected into production.

A first harness attempt used the wrong table accessible name (`Expected receiving quantities` instead of `Expected and approved SKU quantities`). The temporary selector was corrected; no application defect/change was needed. Its retained namespace `TASK4-20261023-befcc1` has two products and one INCOMPLETE RCV, with no TRX/inventory. These clearly labeled test fixtures were not deleted. The core successful run passes21 grouped cases (the later supplemental inventory check reveals UI-01); failed harness attempts are not counted as passes.

## Local regression, fault injection and recovery

Node22.22.2/npm11.21.0; Go1.26.4. A fresh isolated production-like stack `wh-proof-4c5a18fb` used its own PostgreSQL/Redis/RabbitMQ volumes and private bridge. Evidence: `/tmp/warehouse-production-lzj9qot8`; API 39619 / frontend 41827; actual host TCP peer 172.30.99.1 with proxy trust empty. This is separate from the VPS and its data.

| Command | Actual result |
|---|---|
| `npm ci --no-audit --no-fund` |240 packages; installation passed with unchanged lockfile |
| `npm run typecheck` | Pass |
| `npm run lint` | Pass |
| `npm test` |46 tests / 5 files passed;11.55s |
| `npm run build` | Pass;1914modules; JS346.44kB, CSS31.33kB; same production asset names |
| `gofmt -l backend` | Empty output |
| `python3 scripts/verify-production.py` | Fresh API/frontend Docker builds and full isolated production gate passed |
| `go test -count=1 -v ./...` inside gate |24 top-level / 64 subtest passes; actual PostgreSQL/Redis/RabbitMQ, zero skips |
| `go build ./...` / `go vet ./...` inside gate | Pass |
| `go test -race -count=1 ./internal/notifications/... ./internal/router/... ./internal/receiving/...` | Pass;1.066s/21.860s/1.055s for tested packages, real infrastructure |
| `go test -race -count=1 -v ./internal/auth/...` | Additional actual PostgreSQL/Redis run passed, zero skips |
| `npm run test:browser -- tests/catalogue-receiving.spec.ts tests/receiving-transactions.spec.ts tests/inventory-notifications.spec.ts` |10 actual Chrome cases passed 34.2s against local production containers |
| `npm run test:browser -- tests/auth-shell.spec.ts --grep 'anonymous protection\|Operator login\|Supervisor navigation'` |3 actual Chrome cases passed 8.3s against local production containers |
| `python3 -m openapi_spec_validator docs/openapi.yml` | OK |
| `python3 /tmp/wh-check-contracts.py` |354 reference/operation/envelope/ID/traceability assertions;21operations; PRD business body unchanged |
| `python3 scripts/check-responses.py /tmp/warehouse-production-lzj9qot8` |120 actual local responses/24 payloads/563 canonical UUID occurrences across18 operations passed |
| `python3 scripts/check-responses.py /tmp/wh-task4-production-TASK4-20261023-c26d0e` |88 actual production responses/12 payloads/592 canonical UUID occurrences across16 business operations passed |
| `python3 scripts/check-deployment.py` | Production services, loopback isolation, secrets, persistence, locking/backup/rollback and host-Nginx boundary passed |
| `python3 scripts/test-deploy.py` |7 simulated deployment activation/build/pull/migration/readiness/rollback cases passed 3.850s |

Frontend commands ran in `frontend` with `PATH=/tmp/wh-task3a-toolchain/bin:$PATH`, `npm_config_cache=/tmp/wh-npm-cache`. Python validation used `PYTHONPATH=/tmp/wh-contract-validation`. Go used `GOTOOLCHAIN=local`, `GOPATH=/tmp/wh-go-path`, `GOMODCACHE=/tmp/wh-go-mod`, `GOCACHE=$PWD/.work/go-cache`, `GOTMPDIR=$PWD/.work/go-tmp` from repository root; ignored scratch files are excluded from Git. Chrome executable: `/opt/google/chrome/chrome`.

The first local gate attempt was blocked before building by the sandbox's read-only Docker Buildx activity directory. Reran with the required tool permission; the actual build and complete gate passed. No dependency/security bypass or infrastructure cleanup was used.

Actual local failure/recovery evidence:

- T01–T08 plus shared-balance and multi-SKU cases pass. Inventory-posting and late-outbox SQL failures roll back approval, balances, ledger, document status and audit/outbox together.
- Real PostgreSQL constraints/transactions enforce single active submission and exactly-once repeated/concurrent approvals. SQL immutability and expected snapshot tests run against disposable schemas.
- Redis failure fails closed; expired access/refresh/session, single-use refresh replay, role/session binding, and logout/refresh races pass. Production signing secrets were not fetched to forge an expired production token.
- Real broker mandatory-return/confirmation retry, fenced concurrent leases, database-failure nack/redelivery, atomic recipient fanout, duplicate delivery preserving ID/read state, and consumer crash-after-commit recovery pass.
- With the isolated broker stopped, approval still commits and readiness stays200; restarting that broker delivers retained events. Local worker restart preserves notification identity/read state and inventory.
- Application-container recreation preserves database snapshot `1|2|3|2|6` (products/RCVs/balance sum/ledger/notifications) and live authentication. Final seven-service health passes; normal stop gives exit0/no OOM for all seven. A later additional auth race run starts/stops only the same local PostgreSQL/Redis services. All volumes are retained.

The seven deployment-script failure tests are simulations, not claims that production rollback was deliberately executed. Existing browser cases requiring Vite module introspection are not counted as production tests; full frontend unit/component tests and the separate actual-public acceptance matrix above provide the stated coverage.

## Production deployment and ingress evidence

[GitHub Actions run39858680963](https://github.com/Tyomaaans/Warehouse-Assesment/actions/runs/39858680963) remains completed/success for deployed application commit7c855ae. Initial read-only VPS inspection before the repair verifies:

- Seven services healthy: API, frontend, outbox publisher, notification consumer, PostgreSQL, Redis, RabbitMQ; zero restarts/OOM at baseline. API and both workers share image `warehouse-api:<release>`, image ID `sha256:8bfee597b6bf7367bf0e2d13d52778050f91d88e13e3f827671fdbd997b38ac2`; frontend ID `sha256:33e11c9fc024838d389e57e1f6bfc47dd012e16bc856e0699702b8951e39e5f4`.
- Application ports bind only127.0.0.1:18082/18083; PostgreSQL/Redis/RabbitMQ publish no host ports. Named volumes `warehouse_postgres-data`, `warehouse_redis-data`, `warehouse_rabbitmq-data` are present. Private runtime config is0600.
- Host Nginx is active. The owner-installed direct `/api/v1/` route and Cloudflare real-IP snippet are present. Runtime `TRUSTED_PROXIES=172.30.83.1` is narrow. Public health probe request ID `D3OTw09dQoiFD3Djw5qAlw`, carrying forged forwarding headers, correlates to actual Gin TCP peer 172.30.83.1 and a client address different from forged1.2.3.4. Host ingress replaces forwarded headers.
- HTTP redirects301 to HTTPS. Public health/readiness200; SPA deep links and first-party assets load. Public certificate is Google Trust Services WE1, CNtyomaaans.cloud, valid2026-08-31 through2026-11-29; normal Chrome/curl HTTPS verification succeeds.
- Applied migrations001/002 match checked-out SHA-256 checksums. Both run atomically and repeat safely on fresh isolated infrastructure. No production migration was rerun or changed during Task4.

Historical 3D/3E reports/deployment notes accurately record the earlier operator-pending route/trust boundary. The owner subsequently configured it; the current public tests and request-ID correlation above supersede that earlier pending status. No host configuration was applied by this task.

## Open security configuration finding — CF-01 (non-blocking)

Owner verified Web Analytics active, Bot Fight Mode active, no WAF Custom Rules found and no Warehouse Security Events during the inspection. Actual Chrome confirms Analytics Beacon and the separate inline JavaScript Detections bootstrap are blocked by the unchanged CSP (`script-src 'self'`; browser effective directive `script-src-elem`). No first-party CSP block was observed in the tested workflows.

Per the owner's explicit decision: **preserve CSP, preserve Bot Fight Mode, do not modify Cloudflare or add script unsafe-inline/unsafe-eval/wildcards**. Analytics is optional and not an application dependency. JavaScript Detections remains an open, accepted non-blocking security configuration finding; bot-detection integration is **not fully validated**. No broader WAF/security guarantee or authentication bypass is inferred from the absence of inspected events/rules. Any future remediation needs separate owner approval.

## UI-01 fixed-source verification and release

The complete gate was repeated after the one-property containment repair: fresh API/frontend builds, all Go tests with actual three-service infrastructure, build/vet/race, broker failure/recovery, worker restart, application recreation and response-contract validation pass again. Evidence `/tmp/warehouse-production-q_x6s7fq`, project `wh-proof-8905e628`, API 57457 / frontend 56183, observed peer 172.30.100.1. The eleven business Chrome cases include the new regression (1.4s), all pass 45.6s; three auth cases pass 10.8s. Total 14 local browser cases pass, without skips. Captures again validate 120 responses/24 payloads/563 IDs/18 operations. Typecheck/lint, all 46 frontend tests (9.84s) and build also pass after the fix; dependency installation evidence remains valid because manifests/toolchain are unchanged.

The two source/test files were reviewed and staged explicitly; cached whitespace and complete diff checks passed. Repair commit **`04e535b0face6341a405043b9c828d1cd0ebf362`**, `fix: contain accessible labels within scrolling tables`, pushed successfully (`9da0948..04e535b`). Documentation remained unstaged until final results. [Actions run39869194961 attempt1](https://github.com/Tyomaaans/Warehouse-Assesment/actions/runs/39869194961/attempts/1) **completed with failure in deployment**. Its independent build/verification job passed every step, including the complete real production-stack integration gate. Deployment failed at `Prepare target release`: at23:08:07UTC SSH reported `Connection timed out` and exited255, before upload or deployment-script execution. Subsequent upload/start/readiness/public-warning steps were skipped; these are not counted as passed.

The pre-fix regression is a real public test failure, not a passing test or merely a harness error. It asserts390px containment and preserves the full accessible history-link name. Production release validation must repeat that same test without browser style injection, plus both-role list/detail/mobile checks and the main workflow. These checks were not rerun after CI failure because the repaired release was never deployed. Production failure injection/restart tests remain prohibited; an application cutover is performed only through the task-authorized verified repair deployment.

## Assessment deliverables and limitations

| Required deliverable | Audit outcome |
|---|---|
| PRD/business rules | Present; unchanged; acceptance cases mapped above |
| Technical specification | Present; feature-first stack, concurrency/auth/contracts documented; historical project-Nginx example is explicitly superseded by owner boundary in deployment docs |
| ERD/data flow/RBAC/OpenAPI | All present; locked documents unchanged; live payload/response/ID validation passes |
| Testing evidence | Critical automated tests, current production matrix, local rollback/recovery and exact results recorded here and in prior reports |
| AI workflow and at least 3 meaningful prompts | `docs/ai-workflow.md` includes the three substantive inspect/backend/frontend prompts plus detailed authorized task logs |
| Docker/deployment/README | Present; fresh production-stack build/start/migrations/recovery proven; host Nginx ownership and private configuration documented |
| Git history | Separate meaningful foundation/auth/product/receiving/notification/frontend/deployment commits; UI-01 repair04e535b and its regression are separate from the final documentation commit |
| Running application/core business workflows | Actual public Chrome and independent SQL evidence above |
| Original assessment attachment | No separate assessment attachment is tracked in the repository; audited stated assessment requirements captured in the locked documents and this task |

Limits: Chrome desktop/mobile viewports only; no physical-device, Firefox/Safari, load/capacity or penetration-testing claim. Destructive/failure/restart testing was local only. No production duplicate-message replay was injected; actual redelivery/idempotency was tested against the isolated RabbitMQ and production unique-pair/no-duplicate evidence was inspected read-only. Refresh-session expiry was tested locally; the production access-token expiry uses natural wall time. Test fixtures remain clearly labeled in production. Raw evidence is private/transient under `/tmp`, not an exposed production artifact or Git deliverable.

## Acceptance and delivery (updated after Recovery)

Confirmed defect UI-01 (medium, responsive acceptance blocker): at 390px an absolutely positioned `sr-only` history-link label extended beyond the unpositioned `.table-scroll` ancestor and widened the whole inventory document to 557px. Reproduced in actual public Chrome; adding `position: relative` to the scroll wrapper in browser-only diagnosis restored 390px. The source repair in04e535b changes only that positioning context; visible data, accessible names and horizontal table scrolling are retained. A new real-browser regression checks document width before/after scrolling and the full accessible link name. Its pre-fix public run fails as expected; final fixed-source/deployed results are required below.

Final acceptance after Task4-Recovery: **READY for final assessment submission within the documented scope and limitations**. Repair04e535b is deployed, the mobile regression passes against public HTTPS, the remaining supplemental checks pass for both roles, and focused post-deployment receiving/approval/inventory/notification checks pass. The 21 original production groups and natural expiry remain valid for unchanged business/authentication code. The initial failed mobile/SSH attempts remain recorded as historical failures, not passes. CF-01 is still open and non-blocking.

Complete repository inventory: `frontend/src/styles.css` (one containment property), `frontend/tests/inventory-notifications.spec.ts` (focused browser regression), new `docs/task-4-report.md`, updated `docs/testing.md`, `docs/ai-workflow.md`, `README.md`. Backend source, dependencies, migrations, Compose/workflow and locked contracts remain unchanged. Review/stage explicit paths and run `git diff --cached --check`. Deliver the verified repair separately, observe its authorized CI deployment, and finalize the documentation commit only after public regression passes. Existing workflow ignores documentation-only pushes, so the final evidence commit does not redeploy the application. Commit/push/release outcomes are recorded at handoff.


### Historical Task4 failure-stop boundary (superseded by Recovery below)

The runner could not establish SSH to its configured endpoint within the workflow's10second ConnectTimeout. Authorized local SSH to the known VPS202.10.40.123:2222 still works. This confirms a runner-to-target connectivity failure; it does not establish whether the cause is transient routing, host/provider filtering, or a differing configured endpoint. Secret values are write-only and were neither retrieved nor changed. No network/firewall/SSH/Cloudflare/Nginx change or manual deployment bypass was attempted; the failed job was not automatically retried under the owner's failure-stop gate.

Read-only confirmation after the failed workflow:

- `current` remains `7c855aed8922c8f2d274c7954a270d878b24edbc-39858680963-1`.
- All seven original container IDs are unchanged from the pre-release snapshot; all are healthy, zero restarts and no OOM.
- All ten production business-table fingerprints exactly match the pre-release snapshot. Test balances, ledger, snapshots, audit, outbox and notification read state are preserved.
- Runtime narrow trust remains172.30.83.1; public `/api/v1/ready` still returns200. No production service was restarted by Task4.

Additional exact evidence commands (temporary scripts/artifacts stay outside Git):

```sh
/tmp/wh-task3a-toolchain/bin/node /tmp/wh-task4-production.cjs
/tmp/wh-task3a-toolchain/bin/node /tmp/wh-task4-expiry.cjs
/tmp/wh-task3a-toolchain/bin/node /tmp/wh-task4-supplemental.cjs
/tmp/wh-task3a-toolchain/bin/node /tmp/wh-task4-responsive-regression.cjs before-fix
python3 /tmp/wh-task4-auth-race.py
python3 /tmp/warehouse-gh.py 39869194961
python3 /tmp/warehouse-gh.py 39869194961 logs
ssh -F /dev/null -p 2222 -i /home/tyomaans/.ssh/github_deploy_warehouse_assesment -o BatchMode=yes -o StrictHostKeyChecking=yes tyomaaans@202.10.40.123 python3 - < /tmp/wh-task4-vps-audit.py
ssh -F /dev/null -p 2222 -i /home/tyomaans/.ssh/github_deploy_warehouse_assesment -o BatchMode=yes -o StrictHostKeyChecking=yes tyomaaans@202.10.40.123 python3 - < /tmp/wh-task4-production-sql.py
```

The core public script passed; natural expiry passed; supplemental and pre-fix responsive tests failed on the real defect. The final local production gate, which contains the added regression, passed after repair. Failed and unexecuted production checks are not replaced by local passes.

At the original Task4 stop, the required next action was owner confirmation of GitHub-runner SSH reachability and authorization to resume the failed deployment job for the already verified repair. Once deployed, repeat the targeted public regression and remaining public matrix, verify release/data health, and only then finalize acceptance and the documentation commit. Do not alter CSP/Bot Fight Mode or start Task5.

Historical Git state at that stop: source repair04e535b is committed and pushed to main/origin. The four evidence files (README, testing, AI workflow, this new report) are preserved **unstaged and uncommitted** because deployment acceptance failed. No further commit/push or final documentation commit was performed. Assessment deliverables were present, but acceptance at that stop remained blocked until deployment and public verification. The Recovery result below supersedes that status. CF-01 remains separately accepted/non-blocking.


## Task4-Recovery — completed production acceptance

### SSH investigation: confirmed mechanism and limits

The confirmed original failure is an **SSH TCP connection timeout before authentication**, after the configured 10 second ConnectTimeout, exit 255. There was no authentication or host-key rejection. Read-only VPS inspection found SSH active continuously since 18 October and listening on IPv4/IPv6 port 2222; strict local key authentication succeeds. Readable daemon settings permit public-key authentication and disable passwords/root login. UFW is configured enabled, but effective UFW/nftables rules and system/auth logs require privileges unavailable to the deployment user (`sudo -n` requires a password). No privilege bypass was attempted.

GitHub metadata confirms the same five repository secrets used before the earlier successful deployment: `DEPLOY_HOST`, `DEPLOY_USER`, `DEPLOY_PORT`, `DEPLOY_SSH_KEY`, and the previously owner-approved **`DEPLOY_KNOWN_HOSTS`** plural spelling. All metadata timestamps predate that success; the production environment has no secret overrides. Values were neither retrieved nor printed. The workflow was not edited and no duplicate/singular secret was introduced.

Under the owner's explicit Recovery authorization, made **one bounded failed-job rerun** after this audit. The unchanged configuration reached SSH, created the target, uploaded the artifact and completed deployment. Thus current hosted-runner access to the intended VPS is proven by actual deployment. The original low-level cause (packet loss/routing/source-dependent filtering or another network condition) is **not conclusively established**; intermittent/source-specific connectivity is an inference. Do not label the SSH key invalid or assert that a firewall/Cloudflare setting caused the timeout. Secret hostname-versus-IP form and Cloudflare DNS settings were not exposed/inspected; the verified artifact appearing on the authorized VPS establishes the reached target. No Cloudflare, Nginx, firewall, SSH-server, secret or deployment-architecture change was necessary or made.

### Actual workflow and release

[Run39869194961 attempt2](https://github.com/Tyomaaans/Warehouse-Assesment/actions/runs/39869194961/attempts/2), **attempt2**, completed **success** on 22 October 2026 at 23:20:13 UTC. The original successful build/test job and its verified artifact were reused; only failed jobs reran. Do not count that as a second fresh build. Artifact download/extraction, strict SSH setup, target preparation, upload, background deployment, observed deploy result and loopback readiness all passed. The unchanged workflow still emits its historical operator-pending public warning; this report's actual public checks supersede that informational warning.

Actual deployed commit: **`04e535b0face6341a405043b9c828d1cd0ebf362`**. Actual release: **`04e535b0face6341a405043b9c828d1cd0ebf362-39869194961-1`**. The suffix remains1 because it comes from the artifact built in attempt1, although the recovery job is attempt2. `current` resolves to this release and `deploy.done` is0.

| Runtime | Verified image ID |
|---|---|
| API, outbox publisher, notification consumer | `sha256:2ffd1ba7c7e421218cc9935be24dda1fd7a31ba8f8539e778832f7b467cd8c0e` |
| Frontend | `sha256:7ba4faf6060c8314a9bd4224518d854996f9067e113e07d84e0636a60fd3d5f3` |

All seven services are healthy with zero restarts/OOM. PostgreSQL, Redis and RabbitMQ container IDs and named volumes are unchanged. Application/worker containers were replaced only by the authorized deployment; no production fault/restart test was performed. Migrations/checksums are unchanged. Public assets match the locally verified repair byte-for-byte: CSS `/assets/index-C7uxlfJY.css`, SHA256 `a50c1b4b420a863ae6f3e35e0400224fda6014a3bc27b511e16012d766171446`; JS `/assets/index-CLJivJSj.js`, SHA256 `c804ef4ba2c895e99782e893c0ca7afe84db6f00f204c2e754d2b9934352ab41`. The CSS rule contains the positioning fix; actual viewport checks below validate its effect.

### Public Chrome regression actually executed

Same installed Chrome 155.0.8059.39 against the public HTTPS domain, desktop 1440×1000/mobile 390×844:

| Recovery check | Actual outcome |
|---|---|
| Committed mobile regression |1 test passed in 5.4s (case 2.9s); document width≤390 before/after horizontal table scroll; accessible link name preserved |
| Remaining supplemental coverage |Both-role list/detail/history/status, pagination, empty state, canonical missing404, forged-role401 and mobile checks pass |
| Focused receiving/auth/inventory/notification smoke |9 grouped cases pass on the deployed repair |
| Both-role auth |Actual login200, reload refresh200/rotation, protected UI and logout204/revoked-access401 pass |
| Existing inventory/history |Original test SKU quantities12/5 and original four approved source-linked rows preserved |
| Fresh receiving and approval |New isolated SKU expects 2; first receipt 1 remains pending with stock 0, approval posts 1 and RCV stays INCOMPLETE; second receipt/approval completes with stock 2/two ledger rows; repeated approval 409 |
| Both-role mobile inventory/ledger |390px document remains390px after table scrolling; both actual SKU joins verified after source queries complete |
| Notifications |New event delivery, recipient ownership, cross-recipient404 and mark-as-read of this run's new notification pass |
| HTTPS/assets/API |SPA deep link/assets and health/readiness200; public fixed-asset hashes match local build |
| Runtime/CSP |Zero uncaught page errors and zero unexpected CSP violations; first-party JS/CSS200. Existing Cloudflare Beacon/inline-detection blocks remain expected; exact CSP unchanged |

The new fixture is `TASK4-RECOVERY-20261023-3b4b97`: product `J-pS_NiISrSNPjjtlRvvvA`, document `fkwdkX0dT_65aZ_VcRhO5Q`, transactions `brKXjGZaQNyuTH9AtPzkqw` and `Ypm6aznMRu-nya67RPUkSw`. It contributes one completed RCV, two approved ledger rows totaling 2, four published outbox events and six notifications. Only its newly generated notification was marked read. No pre-existing business record was modified.

Before any recovery smoke writes, all ten business-table count/fingerprints exactly matched the pre-deployment snapshot. After the isolated smoke, the same ten sets of **pre-existing rows** still match, using the recorded cutoff and parent creation times for item/balance tables. Existing product snapshots, balances, ledger, audit, outbox and notification read state are preserved. Credentials/tokens/cookies are absent from reports and retained captures; all created sessions were logged out.

### Recovery commands and honest verification boundaries

```sh
python3 /tmp/wh-task4-recovery-metadata.py
python3 /tmp/wh-task4-recovery-rerun.py
python3 /tmp/wh-task4-recovery-watch-ci.py
/tmp/wh-task3a-toolchain/bin/node /tmp/wh-task4-responsive-regression.cjs after-recovery
/tmp/wh-task3a-toolchain/bin/node /tmp/wh-task4-supplemental.cjs
/tmp/wh-task3a-toolchain/bin/node /tmp/wh-task4-recovery-browser.cjs
/tmp/wh-task3a-toolchain/bin/node /tmp/wh-task4-recovery-ledger.cjs
/tmp/wh-task3a-toolchain/bin/node /tmp/wh-task4-recovery-assets.cjs
PYTHONPATH=/tmp/wh-contract-validation python3 scripts/check-responses.py /tmp/wh-task4-production-TASK4-RECOVERY-20261023-3b4b97
PYTHONPATH=/tmp/wh-contract-validation python3 /tmp/wh-check-contracts.py
PYTHONPATH=/tmp/wh-contract-validation python3 -m openapi_spec_validator docs/openapi.yml
git diff --check
ssh -F /dev/null -p 2222 -i /home/tyomaans/.ssh/github_deploy_warehouse_assesment -o BatchMode=yes -o StrictHostKeyChecking=yes -o ConnectTimeout=10 tyomaaans@202.10.40.123 python3 - < /tmp/wh-task4-recovery-preservation.py
```

New capture validation: **25 actual responses, 4 payloads, 194 canonical UUID v4 occurrences across 11 operations pass**. Final OpenAPI validation is OK; all 356 cross-document assertions and the whitespace check pass. A final read-only audit confirms the same release and all seven healthy services with zero restarts/OOM after browser testing. Prior valid full Go/frontend/race/real-infrastructure/recovery/OpenAPI results are reused for unchanged source/toolchains; no skipped test is recounted as executed. No new application, workflow, dependency, migration or locked-contract change occurred during Recovery.

Auxiliary diagnostic failures are not hidden: generic Python urllib static-asset probes received 403/error 1010 while real Chrome succeeded; no Cloudflare change was made. A temporary byte-string assertion assumed CSS declaration order and failed despite matching asset hashes; corrected that harness to inspect the required properties irrespective of minifier order. Final actual-Chrome asset/hash/CSP and committed viewport regressions passed. These harness/client observations are distinct from the original real layout defect.

### Final readiness and remaining risks

**Task4 production acceptance is complete; Warehouse is ready for final assessment submission within this report's stated scope.** UI-01 is fixed and verified on the deployed commit. Required deliverables are present and linked from README; original full tests and current focused production regression supply the evidence. No Task5 or new business feature was started.

CF-01 remains the owner-accepted open non-blocking finding: CSP and Bot Fight Mode unchanged, no Cloudflare modification, JavaScript Detections integration not fully validated. The precise underlying network cause of the first SSH timeout remains unknown; successful recovery does not guarantee future runner connectivity. If it recurs, obtain owner-authorized privileged firewall/provider/log evidence before proposing a server-side change. Physical-device/other-browser/load/security/production-fault limitations stated above remain.

Recovery repository inventory is exactly the four preserved documentation files: `README.md`, `docs/testing.md`, `docs/ai-workflow.md`, `docs/task-4-report.md`. No additional deployment fix commit is needed. After complete explicit-path/staged review and whitespace checks, deliver `docs: finalize warehouse assessment verification`; its documentation-only push is excluded from deployment triggers. The deployed application remains 04e535b; the documentation commit/hash/push and final clean Git status are recorded in the final handoff/Git history.
