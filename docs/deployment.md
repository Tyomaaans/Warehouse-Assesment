# Warehouse deployment

Target: https://wh.tyomaaans.cloud. Host Nginx is owned by the server operator. This repository never installs, configures, reloads or restarts it. Task 4 full E2E testing is a later, separate task.

## Inspected reference and approved differences

The actual `/home/tyomaans/Projects/clone-mini-erp/.github/workflows/deploy.yml`, `scripts/deploy.sh`, both Dockerfiles and production Compose were inspected. Its current workflow uses no container registry: CI builds/tests a Go binary and Vite bundles, uploads a source artifact, rsyncs over SSH, starts a background VPS build and polls its completion. Runtime images have `<commit>-<run>-<attempt>` tags. A workflow concurrency group plus host `flock` serializes releases. Images are assembled sequentially using a temporary bounded Buildx builder. A private pre-migration PostgreSQL backup and readiness checks precede atomic activation of `current`; previous images/release permit application rollback. Migrations are not reversed.

Warehouse mirrors that mechanism, adapting paths/images, one frontend, PostgreSQL/Redis/RabbitMQ and independent publisher/consumer processes. It uses the already-present five secrets: `DEPLOY_HOST`, `DEPLOY_USER`, `DEPLOY_PORT`, `DEPLOY_KNOWN_HOSTS`, `DEPLOY_SSH_KEY`. The owner confirmed the existing plural known-host name after API metadata verification; no secret was created, renamed or downloaded. Strict known-host checking replaces the reference's unverified `ssh-keyscan`. Node 22.22.2/npm 11.21.0 preserve the verified frontend toolchain. Npm is installed in an isolated tool directory, avoiding a global self-upgrade failure observed in the Node image.

The explicit Task 3D/3E host-Nginx boundary supersedes the older technical-spec deployment example containing project Nginx services. No locked business/API contract changes. Obsolete `deploy/nginx.conf` and its Compose service were removed. The frontend uses a small Node HTTP application serving immutable Vite assets with SPA fallback; no Vite dev/preview server or project Nginx is deployed. Same-origin `/api/v1` URLs preserve cookies. Its API relay supports direct local-container testing, strips supplied forwarding headers, and is not the required production host route.

## Actual pre-deployment audit

Read-only SSH succeeded at `tyomaaans@202.10.40.123:2222` with the existing local Warehouse key and existing trusted known-host entry. An unrelated web-profile stack uses loopback ports 8080/5432/6379/5672/15672 and remains untouched. Ports 18082/18083 and subnet 172.30.83.0/24 were unused. The owner-prepared `/opt/warehouse-assessment` is writable, with empty `releases`/`shared` directories and initially no private `.env`. Existing unrelated volumes/build cache are retained.

The host has about 961 MiB RAM, 468 MiB available, 2 GiB swap and 3.2 GiB free disk at initial inspection. Runtime memory limits total 560 MiB, not a claim of allocated memory or load capacity; monitor actual working sets and swap alongside existing services. Deploy refuses less than 128 MiB available RAM or 2 GiB free disk before assembling images. It does not prune anything to bypass capacity limits.

Host Nginx is active. Readable virtual-host configuration routes `wh.tyomaaans.cloud` to loopback 18082 for all paths. The certificate served through origin TLS includes `*.tyomaaans.cloud` and is valid 2026-08-31 to 2041-08-27. Its file is not readable by the deployment user, and sudo requires a password; no privilege or Nginx change was attempted. Public DNS returns Cloudflare addresses; a direct origin connection with the Warehouse SNI reaches the expected existing Nginx virtual host. Before application deployment both public/origin HTTP responses were 502. Proxied DNS alone does not prove Cloudflare's origin record; the direct origin and public deployment checks provide separate evidence.

## Required operator-owned Nginx routing

The operator must add a dedicated API location to the existing Warehouse HTTPS server. Port choices were checked on the actual VPS. Keep `/` routed to 18082 and add:

```nginx
location /api/v1/ {
    proxy_pass http://127.0.0.1:18083;
    proxy_http_version 1.1;
    proxy_set_header Host $host;
    proxy_set_header X-Real-IP $remote_addr;
    proxy_set_header X-Forwarded-For $remote_addr;
    proxy_set_header X-Forwarded-Proto $scheme;
    proxy_connect_timeout 5s;
    proxy_read_timeout 60s;
}
```

The `proxy_pass` has no URI suffix: `/api/v1/` remains unchanged. The host may use its existing Cloudflare real-IP snippet in this vhost, accepting CF-Connecting-IP only from the Cloudflare ranges it maintains. Do not trust arbitrary supplied X-Forwarded-For; replace it at ingress. Preserve the existing host certificate and HTTP-to-HTTPS redirect. The operator validates/reloads host configuration independently; no repository script performs those operations. Public acceptance remains incomplete while this direct API route is absent, even if the frontend's same-origin relay allows an API request to work.

## Trusted proxies: observe before trusting

`TRUSTED_PROXIES` starts empty. Do not use `0.0.0.0/0`, broad Docker networks or an assumed gateway. API structured request logs include `peer_address` (actual TCP peer) and `client_ip`. To verify the actual public path after the operator's route is active:

1. Send a public `/api/v1/health` request and record its `X-Request-ID` response header.
2. Find that ID in the API logs and inspect `peer_address`.
3. Compare it with the Warehouse network/host path and ensure it is the host-Nginx connection, not the frontend relay/container or an unrelated proxy.
4. Put only that single IP in private `TRUSTED_PROXIES` and recreate API. Repeat with a known client and a spoofed forwarding header, verifying ingress replacement and the resolved address.

A direct VPS loopback API probe independently measures the host TCP source, but does not substitute for the public Nginx-path check. No host configuration is automatically changed to make that test pass. The local production gate also measures its actual host-to-Gin peer with trust disabled.

## Private configuration and initial accounts

Keep a mode-0600 shell-compatible `/opt/warehouse-assessment/.env`, outside releases/source/images, following `.env.production.example`. Use independently generated URL-safe hexadecimal values for `POSTGRES_PASSWORD`, `REDIS_PASSWORD`, `RABBITMQ_PASSWORD` and `JWT_SECRET_KEY` (at least 32 random bytes for JWT). Database/Redis/broker have no public or loopback published ports. Neither CI nor frontend receives these production secrets. Existing deployment configuration is never overwritten.

Set `BOOTSTRAP_USERS=true` only for explicit initial provisioning with `OPERATOR_EMAIL`, `OPERATOR_PASSWORD`, `SUPERVISOR_EMAIL`, `SUPERVISOR_PASSWORD`. Deployment passes these into the one-shot `provision-users` command, not the running API/frontend environment. Existing same-role accounts/passwords are preserved. Disable bootstrap after success. There is no public signup/reset workflow. Store generated account credentials privately on the VPS and retrieve through your existing SSH account; do not paste them into logs/issues.

## Local production gate

Use the documented Node/npm and Go toolchains, Docker/Compose, installed Chrome and Python OpenAPI validation packages (`openapi-spec-validator==0.9.0`, which also supplies the schema validator). Install them in a virtual environment or isolated temporary tool directory; CI installs the same pinned validator. Install/test/build frontend first, then run:

```sh
python3 scripts/check-deployment.py
python3 scripts/verify-production.py
```

The latter builds API/frontend images with the production Dockerfiles, creates a uniquely named isolated seven-service stack with three private volumes, applies migrations twice, runs real Go infrastructure/race tests, provisions disposable roles and verifies production assets/SPA/API/auth/receiving/inventory/outbox/notifications. It exercises a broker outage, worker restart and application recreation with persisted data, then runs focused real Chrome frontend smoke cases. Private credentials/captures/logs stay in a mode-0700 temporary directory. Only its own containers are stopped at the end; volumes are never removed. `VERIFY_SKIP_BUILD=1` may reuse exactly the images just built for the same source; it does not skip tests. These are Task 3D/3E acceptance checks, not execution of Task 4.

## CI/CD, release and recovery

`.github/workflows/deploy.yml` runs on main pushes/manual dispatch with contents-read permission and non-cancelling production concurrency. It executes frontend ci/typecheck/lint/tests/build, Go format/build/vet, OpenAPI/deployment validation and the isolated production gate. Only then does it upload a versioned source/binary/assets artifact and enter the production job. SSH uses the existing five secrets and strict host verification; runtime config stays on the VPS. No registry credentials or additional deployment secret is required.

On the VPS `scripts/deploy.sh /opt/warehouse-assessment <commit>-<run>-<attempt>` locks only Warehouse, validates resources/config/image names, pulls infrastructure images and assembles API/frontend runtime layers from CI's compiled inputs. It stops Warehouse writers, starts infrastructure, writes a private PostgreSQL dump, runs checksum/advisory-lock migrations, optionally provisions roles, and recreates application/worker containers. `current` changes atomically only after all seven services are healthy and loopback frontend/SPA/API checks pass. The owner explicitly left public acceptance pending. The workflow records a warning/summary after loopback deployment; it does not claim public acceptance. Successful Docker startup does not establish host routing, measured public proxy trust or public success. Documentation-only evidence commits do not redeploy application containers.

Failures before replacement retain the running release. Later failures restore previous application images when available; first-release failure stops only Warehouse applications and retains infrastructure/data/backups. No automatic volume deletion, global prune, old-release removal, backup deletion or Nginx management. Retention and off-host backup/restore drills require operator action. A code rollback is safe only while schema changes remain backward compatible; restore a verified database backup separately if needed. Notification transport remains at least once, with existing event/recipient deduplication.

Inspect status/logs without printing configuration:

```sh
cd /opt/warehouse-assessment/current
docker compose -p warehouse --env-file /opt/warehouse-assessment/.env --env-file images.env -f compose.production.yml ps
docker compose -p warehouse --env-file /opt/warehouse-assessment/.env --env-file images.env -f compose.production.yml logs --tail 100 api outbox-publisher notification-consumer
```

The authoritative actual local/CI/VPS/public results and any pending operator work are recorded in `docs/task-3d-3e-report.md` and the final delivery handoff. Task 4 coverage is planned separately in `docs/task-4-test-plan.md`.
