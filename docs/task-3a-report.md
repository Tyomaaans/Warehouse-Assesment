# Task 3A — Frontend Foundation, Authentication & Shell

Date: 2026-10-23. Status: **foundation/authentication verified; authorized separate commit/push after final Git review**. Task 3A-Fix resolved the initial installation blocker and preserved all existing work. Business pages remain explicit placeholders.

## Baseline and source inspection

- Initial working tree was clean on `main`, HEAD `fccfbf3307998661801d57d0e465e16103866bf4`. Origin fetch/push URL: `https://github.com/Tyomaaans/Warehouse-Assesment.git`.
- Read `AGENTS.md`, PRD, technical specification, OpenAPI, RBAC, conventions audit, testing policy, AI workflow and README. Inspected actual backend auth routes/DTOs/cookies/JWT claims, error/response behavior, middleware and Nginx proxy configuration before integrating.
- Inspected `/home/tyomaans/Projects/web-profile/web-profile-frontend/src/projects/auth-session/utils/api.ts`, auth context, login, protected route, route tree and centralized API configuration; also reference package/Vite configuration. Reused the typed request, in-memory access token, cookie credentials, shared refresh promise, single retry and feature ownership patterns. Did not copy registration, profile, session management or heartbeat features.
- The backend has host-only HttpOnly cookies at `/api/v1/auth` and `/api/v1`, SameSite Strict, Secure in production, and no CORS middleware. The frontend uses same-origin `/api/v1` requests through a Vite development/preview proxy. No backend CORS/cookie/proxy/workflow change was made.
- Refresh returns access token/expiry without a user profile. JWT `sub`/`role`/`exp` supply frontend session display after restoration. The backend remains authoritative for authorization, including current DB role/live Redis session. No identity endpoint was invented. Email is displayed only when actually returned by login; after reload the shell uses a role/session label.

## Initial failure history

From `/home/tyomaans/Projects/wh-assesment/frontend`:

```sh
env npm_config_cache=/tmp/wh-npm-cache npm install --no-audit --no-fund
```

**Result: exit 1**, after approximately 62 seconds:

```text
TypeError: Cannot read properties of null (reading 'edgesOut')
    at [loadPeerSet] (.../@npmcli/arborist/lib/arborist/build-ideal-tree.js:1304:38)
    at async Arborist.buildIdealTree (.../build-ideal-tree.js:206:7)
    at async Arborist.reify (.../reify.js:156:5)
```

The log records Node **v22.22.1**, npm **9.2.0** and unfinished `idealTree:node_modules/vitest` peer resolution. Registry requests returned HTTP 200, so this was not observed to be a network/sandbox denial. That initial attempt did not install dependencies or generate a lockfile. The user subsequently authorized Task 3A-Fix to investigate/recover and finish verification.

Full installer log is outside the repository at `/tmp/wh-npm-cache/_logs/2026-10-22T18_36_47_379Z-debug-0.log`.

Initial tooling discovery also ran `node --version`, `npm --version` and registry version queries. Chrome was available at `/opt/google/chrome/chrome`. No browser/backend success was claimed before Task 3A-Fix.

Under the initial fail-stop instruction, stopped without staging, commit or push. Recovery described below occurred only after explicit Task 3A-Fix authorization. No `--force`, `--legacy-peer-deps`, framework replacement or peer bypass was used.

## Confirmed cause and dependency recovery

- Recorded versions were Node 22.22.1/npm 9.2.0. The manifest parsed normally and had no lockfile. Local Arborist source at the logged line dereferences `node.parent.edgesOut` without a null-parent guard. The stack and log identify the npm resolver crash during peer-tree construction.
- Copied only the original, unchanged manifest into `/tmp/wh-task3a-npm11-probe`. npm 11.21.0 resolved it and generated a lock without force/legacy flags. Its resulting graph did not auto-install the optional Vitest browser adapter involved in the older resolver's log. Later full install/ci and `npm ls --all` succeeded with no invalid/missing peers. This confirms that changing the resolver resolves this graph; the manifest was not malformed and library versions did not need to be weakened. The exact minimal optional-peer combination triggering npm 9's null-parent path was not isolated, so that narrower trigger remains an inference rather than a claimed proof.
- The probe exposed a second, confirmed mismatch: jsdom 30.1.3 and selected DOM dependencies declare Node `^22.22.2 || ^24.15.0 || >=26.0.0`. Installed Node 22.22.1 was one patch below that minimum. Raised the project's declared engine requirement to match and used **Node 22.22.2/npm 11.21.0** for all final checks. npm 12.2.1 was inspected but not selected; a compatible tested npm 11 toolchain was sufficient.
- Downloaded the official Node 22.22.2 Linux archive into `/tmp`, checked SHA-256 against Node's published `SHASUMS256.txt`, and used an isolated npm 11.21.0 CLI/cache there. No system Node/npm installation was replaced. `.nvmrc`, `packageManager`, engines and `.npmrc` engine enforcement record the tested toolchain. All React/Vite/Tailwind/Router/Query/testing library version constraints remain as originally selected.
- Successful install generated `frontend/package-lock.json`; `npm ci` installed the same 240 packages successfully. The complete installed dependency tree has no peer problems. No preserved source file was deleted or replaced.

Relevant primary evidence: [npm CLI reports of the null-parent edgesOut failure](https://github.com/npm/cli/issues/9787), the captured local log/source, registry package engine/peer metadata, and the successful controlled probe. Similar upstream reports corroborate the failure class; they do not prove this project's minimal trigger.

## Verified implementation

- React/TypeScript/Vite project configuration, Tailwind Vite plugin/design tokens, ESLint/TypeScript configuration and working Vitest/Playwright tests.
- Feature-scoped authentication and overview, shared UI components, router/providers, session-scoped query keys and cache cleanup on session changes. No business data fetches, metrics, chart values, records or notification counts are fabricated.
- Typed fetch adapts exact success/nested error envelopes and empty logout 204. Access token stays in instance memory. Requests use cookie credentials, a shared refresh promise, one request retry and a revision check preventing stale refresh completion from restoring cleared sessions. Unit and real-backend browser tests verify these boundaries.
- Auth provider/login code includes local required/email feedback, server errors, pending state, password visibility, remember-me, cookie session restoration, timed refresh, logout and client cleanup, unavailable-session retry UI. Server logout failure has an explicit unconfirmed-signout message/retry; JavaScript cannot clear HttpOnly cookies if the server is unreachable.
- Desktop sidebar/mobile native-dialog navigation, active links, role display, skip link, labeled inputs, focus handling and reduced-motion styling. Actual Chrome checks cover mobile overflow, dialog/Escape/focus restoration, role guards and explicit placeholders. Desktop login/overview and mobile overview screenshots were visually inspected.
- Shared button/input/table/header/loading/empty/error primitives; no large component library. Native CSS illustration and lucide icons; no image-generation assets or unrelated reference assets.

Implemented frontend routes:

| Frontend route | Behavior |
|---|---|
| `/login` | Authentication form and session restoration states |
| `/` | Authenticated overview/navigation/workflow guidance, without live metrics |
| `/products` | Labeled planned-view placeholder |
| `/receivings` | Labeled planned-view placeholder |
| `/transactions` | Labeled planned-view placeholder |
| `/inventory` | Labeled planned-view placeholder |
| `/notifications` | Labeled planned-view placeholder |
| `/transactions/review` | Supervisor-only planned-view placeholder; other role gets an access-denied state |
| Unmatched authenticated paths | Page-not-found state |

Business areas have no complete CRUD/receiving/approval/inventory/notification screen. No signup, password reset, profile, realtime chat or WebSocket feature was added.

## Complete file inventory

Created frontend files (39):

1. `frontend/package.json`
2. `frontend/tsconfig.json`
3. `frontend/vite.config.ts`
4. `frontend/vitest.config.ts`
5. `frontend/eslint.config.js`
6. `frontend/.env.example`
7. `frontend/index.html`
8. `frontend/public/favicon.svg`
9. `frontend/src/main.tsx`
10. `frontend/src/styles.css`
11. `frontend/src/app/App.tsx`
12. `frontend/src/app/FeaturePlaceholder.tsx`
13. `frontend/src/app/navigation.ts`
14. `frontend/src/app/query-client.ts`
15. `frontend/src/components/Brand.tsx`
16. `frontend/src/components/layout/AppShell.tsx`
17. `frontend/src/components/ui/Button.tsx`
18. `frontend/src/components/ui/TextField.tsx`
19. `frontend/src/components/ui/PageHeader.tsx`
20. `frontend/src/components/ui/States.tsx`
21. `frontend/src/components/ui/Table.tsx`
22. `frontend/src/lib/api/config.ts`
23. `frontend/src/lib/api/types.ts`
24. `frontend/src/lib/api/client.ts`
25. `frontend/src/features/auth/api.ts`
26. `frontend/src/features/auth/session.ts`
27. `frontend/src/features/auth/context.ts`
28. `frontend/src/features/auth/use-auth.ts`
29. `frontend/src/features/auth/AuthProvider.tsx`
30. `frontend/src/features/auth/LoginPage.tsx`
31. `frontend/src/features/auth/ProtectedRoute.tsx`
32. `frontend/src/features/overview/OverviewPage.tsx`
33. `frontend/.nvmrc`
34. `frontend/.npmrc`
35. `frontend/package-lock.json`
36. `frontend/src/lib/api/client.test.ts`
37. `frontend/src/features/auth/auth.test.tsx`
38. `frontend/playwright.config.ts`
39. `frontend/tests/auth-shell.spec.ts`

Created documentation: `docs/task-3a-report.md`.

Modified:

- `.gitignore` — exclude frontend dependencies/build output, coverage/browser artifacts and TypeScript build metadata.
- `README.md` — tested toolchain, development/build instructions, authentication behavior and browser-test setup.
- `docs/ai-workflow.md` — Task 3A authorization, initial stop, dependency recovery and final verification evidence.

Total: **43 files** (40 created, 3 modified), retaining the original 36 files and adding seven toolchain/lock/test files. Within preserved source, `package.json` gained toolchain metadata and `AppShell.tsx` gained the focus fix below; README/workflow/report now carry final evidence. Backend source, migrations, Compose/Nginx, root environment example and locked specification files are unchanged. No `.env`, credentials, installed dependencies, binaries or generated test/build artifacts are part of the commit. No preserved source file was deleted/reverted.

## Actual verification commands/results

Go build ran from `backend`; npm commands from `frontend`. After installation, `/tmp/wh-task3a-toolchain/bin` contains symlinks to the checksum-verified Node 22.22.2 binary and npm 11.21.0 CLI. npm checks used `PATH=/tmp/wh-task3a-toolchain/bin:$PATH` and `npm_config_cache=/tmp/wh-npm-cache`.

| Exact command | Actual result |
|---|---|
| `env npm_config_cache=/tmp/wh-npm-cache npm exec --yes --package=npm@11.21.0 -- npm install --prefix /tmp/wh-task3a-npm11-probe --package-lock-only --ignore-scripts --no-audit --no-fund` | Original manifest resolved; Node 22.22.1 engine warnings exposed the separate patch-level requirement |
| `env PATH=/tmp/node-v22.22.2-linux-x64/bin:$PATH npm_config_cache=/tmp/wh-npm-cache /tmp/node-v22.22.2-linux-x64/bin/node /tmp/wh-npm-cache/_npx/b0271877f536be0e/node_modules/npm/bin/npm-cli.js install --no-audit --no-fund` | Pass; 240 packages, 24s; supported toolchain, lockfile generated |
| `npm ci --no-audit --no-fund` | Pass; 240 packages, 11s; no peer bypass |
| `npm ls --all --json` | Pass; no invalid/missing dependency/peer problems |
| `npm run typecheck` | Pass; final source/tests checked |
| `npm run lint` | Pass; no warnings/errors |
| `npm run build` | Pass; Vite 8.2.2, 1890 transformed modules, final build 1.63s; JS 282.23 kB (gzip 89.22 kB), CSS 20.58 kB (gzip 5.93 kB) |
| `npm test` | Pass; 2 files, 11 unit/component tests; final rerun 13.05s |
| `env GOTOOLCHAIN=local GOPATH=/tmp/wh-go-path GOMODCACHE=/tmp/wh-go-mod GOCACHE=/tmp/wh-go-build go build -o /tmp/wh-task3a-api .` | Pass; actual unchanged backend compiled for integration |
| `python3 /tmp/wh-task3a-start.py` | First launcher attempt raced the binary build and failed before migration; corrected to await the build and reuse the same labeled containers/credentials; readiness then passed |
| Initial standalone `npm run test:browser` against the exited launcher | Five connection-refused failures; API/Vite did not remain alive after the short launcher exited. No app/browser success was counted from that attempt |
| `python3 /tmp/wh-task3a-browser.py` | Pass; keeps API/Vite/tests under one supervisor; final run five development browser tests (14.8s), then two production-build preview smoke cases (6.2s). All actually ran in installed Chrome, no skips |
| `npm run test:browser` | Invoked by the supervised harness with real runtime credentials outside the repository; all five cases pass |
| `npm run test:browser -- --grep 'anonymous protection\|Supervisor navigation'` | Invoked against built frontend/Vite preview; both selected smoke cases pass |
| `PYTHONPATH=/tmp/wh-contract-validation python3 -m openapi_spec_validator docs/openapi.yml` | Pass; locked OpenAPI unchanged |
| `PYTHONPATH=/tmp/wh-contract-validation python3 /tmp/wh-check-contracts.py` | Pass; 344 cross-document assertions across 21 operations; PRD business text unchanged |

Final dependency review additionally checked lockfile v3 against both manifest dependency maps, public npm HTTPS resolution URLs and SHA-512 integrity fields. `npm ls --all --json` returned exit 0 with no tree problems. SHA-256 comparison against the 36-file recovery snapshot confirmed every original file remains present; only the toolchain metadata, shell focus fix and final documentation changed within those files.

The frontend source fix arose from visual review: focusing the main region on route entry scrolled the page and header away. `AppShell` now scrolls to the top and focuses with `preventScroll`. Added browser assertions for zero scroll offset on login and mobile route changes. Typecheck, lint, build, all 11 unit/component tests and all seven browser executions were rerun afterward and passed. No auth/backend workaround was introduced.

The temporary startup/lifetime mistakes were corrected in `/tmp` harnesses, not hidden or counted as successful coverage. Final required checks pass with no unresolved installation/verification blocker.

## Real backend/browser evidence

- New isolated, labeled containers `wh-task3a-postgres` (PostgreSQL 16), `wh-task3a-redis` (Redis 7) and `wh-task3a-rabbitmq` (RabbitMQ 4) supplied real infrastructure. Generated host ports: 32784/32785/32786. The actual compiled backend applied unchanged migrations, provisioned only isolated test accounts and passed readiness on port 44241. Vite development used 53403; preview used another dynamically allocated localhost port.
- Runtime passwords/signing key were generated and retained only in mode-0600 `/tmp/wh-task3a-runtime.json`; no credentials were hardcoded into the application or committed. Browser output and screenshots remain under `/tmp` and auth traces are disabled. No existing project data/container was deleted or reset. No business seed records were created.
- Chrome checked anonymous protected-route redirection, local form validation/focus, and real backend invalid-credential 401 feedback.
- Operator login verified actual HttpOnly/SameSite Strict cookie paths, no token/cookie exposure through localStorage/sessionStorage/document.cookie, and refresh-token rotation on page reload while preserving session ID. A Supervisor-only page denied Operator access.
- At mobile width 390, checked no horizontal overflow, navigation modal visibility, Escape/focus restoration, navigation closing/focusing main content and labeled placeholders. Desktop and mobile screenshots were inspected after the focus correction.
- Supervisor navigation and every business placeholder were checked. Real logout returned 204, cleared both cookies, removed local auth and protected access remained blocked on subsequent navigation.
- Six concurrent protected reads used a genuinely expired JWT signed with the isolated backend's key and actual user/session IDs. The real server rejected it; the client made exactly one cookie refresh and each read succeeded after its single retry. Actual Operator product-write denial returned 403 without a business write.
- Backend logout independently revoked the current session while the app still held its access token. The next protected request failed and refresh returned 401; the actual app cleared identity/query cache and redirected to login.
- Browser offline transport caused real fetch failure: the workspace was hidden behind a retryable unavailable state. Restoring transport and retrying successfully refreshed the actual backend cookie session. No mocked backend success was used for these browser cases.
- Two selected cases repeated against the production build through Vite preview (anonymous/login/error handling and Supervisor/placeholders/logout). Full API-client module introspection cases run against Vite development modules; no claim that all five ran against preview.
- The supervisor shut down the backend with exit 0 and stopped Vite/preview process groups. Test containers are stopped at handoff with data retained.

## Limitations and completion boundary

Business pages remain placeholders, including notification polling and all receiving/product/inventory UI actions. No frontend Docker/Nginx static hosting, production HTTPS/Secure-cookie deployment or Safari/Firefox verification was performed. Visual/keyboard checks are focused smoke coverage, not a comprehensive accessibility audit. A failed server logout cannot revoke HttpOnly cookies while disconnected; local state is cleared and the UI explicitly reports the unconfirmed server sign-out and offers retry. Only Node 22.22.2/npm 11.21.0 was executed, though engine ranges also declare compatible later Node lines.

Relevant official setup references consulted: [Vite proxy configuration](https://vite.dev/config/server-options), [Tailwind Vite integration](https://tailwindcss.com/docs/installation/using-vite), [React Router declarative routing](https://reactrouter.com/start/declarative/routing), [TanStack Query setup](https://tanstack.com/query/latest/docs/framework/react/quick-start). Actual auth conventions came from the inspected local reference, with locked warehouse adaptations described above.

**Git delivery:** Task 3A is delivered separately as `feat: establish frontend shell and authentication` after review of the complete 43-file manifest and staged diff/check. Resolve the containing commit hash with `git log -1 --format=%H -- docs/task-3a-report.md`; final handoff supplies the actual hash, `git push origin main` result and final Git status. No broad add, force push, amend, reset or destructive cleanup is used. Stop after Task 3A; no Task 3B or complete business pages.
