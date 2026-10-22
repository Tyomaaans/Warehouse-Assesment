#!/usr/bin/env bash
set -Eeuo pipefail
umask 077
root=${1:?Usage: deploy.sh /opt/warehouse-assessment release-id}
release_id=${2:?Release ID is required}
[[ "$root" =~ ^/[a-zA-Z0-9_./-]+$ && -d "$root" && "$root" != / ]]
root=$(realpath -e -- "$root")
[[ "$release_id" =~ ^[a-f0-9]{40}-[0-9]+-[0-9]+$ ]]
release="$root/releases/$release_id"
test -f "$root/.env"
test -f "$release/images.env"
test -f "$release/compose.production.yml"
chmod 600 "$root/.env"
exec 9> "$root/.deploy.lock"
flock -n 9 || { echo 'Another Warehouse deployment is running.'; exit 1; }
export COMPOSE_PARALLEL_LIMIT=1
export BUILDX_CONFIG
BUILDX_CONFIG=$(mktemp -d "$root/.buildx.XXXXXX")
builder="warehouse-$release_id"
builder_created=0
previous=$(readlink -f "$root/current" 2>/dev/null || true)
changed=0
compose() {
  local directory=$1
  shift
  docker compose --project-name warehouse --env-file "$root/.env" --env-file "$directory/images.env" \
    --project-directory "$directory" -f "$directory/compose.production.yml" "$@"
}
cleanup() {
  if (( builder_created )); then docker buildx rm "$builder" > /dev/null 2>&1 || true; fi
  # Only remove this invocation's temporary, empty Buildx client metadata.
  find "$BUILDX_CONFIG" -type f -delete
  find "$BUILDX_CONFIG" -depth -type d -empty -delete
}
rollback() {
  local status=$?
  trap - ERR
  set +e
  if (( changed )); then
    if [[ "$previous" == "$root/releases/"* && -f "$previous/images.env" ]]; then
      echo 'Restoring the previous Warehouse application release; data is retained.'
      compose "$previous" up -d --no-build --pull never --force-recreate --wait --wait-timeout 180 api outbox-publisher notification-consumer frontend || echo 'Operator recovery required.'
    else
      echo 'First deployment failed; stopping Warehouse applications, retaining infrastructure/data.'
      compose "$release" stop api outbox-publisher notification-consumer frontend
    fi
  fi
  echo 'Migrations are not automatically reversed. Backups and releases are retained.'
  exit "$status"
}
trap cleanup EXIT
trap rollback ERR
trap 'false' HUP INT TERM
[[ $(docker info --format '{{.Architecture}}') == x86_64 ]]
available_kib=$(awk '/MemAvailable:/ {print $2}' /proc/meminfo)
(( available_kib >= 131072 )) || { echo 'Less than 128 MiB RAM available.'; exit 1; }
available_bytes=$(df -B1 --output=avail "$root" | tail -1 | tr -d ' ')
(( available_bytes >= 2147483648 )) || { echo 'At least 2 GiB free disk required.'; exit 1; }
compose "$release" config --quiet
# Refuse accidental unversioned or cross-project application image names.
grep -Fxq "API_IMAGE=warehouse-api:$release_id" "$release/images.env"
grep -Fxq "FRONTEND_IMAGE=warehouse-frontend:$release_id" "$release/images.env"
for service in postgres redis rabbitmq; do compose "$release" pull "$service"; done
docker buildx create --name "$builder" --driver docker-container --driver-opt memory=384m,memory-swap=1g,cpu-period=100000,cpu-quota=100000
builder_created=1
for service in api frontend; do compose "$release" build --builder "$builder" "$service"; done
docker buildx rm "$builder"
builder_created=0
changed=1
compose "$release" stop api outbox-publisher notification-consumer
compose "$release" up -d --no-build --pull never --wait --wait-timeout 180 postgres redis rabbitmq
mkdir -p "$root/backups"
backup="$root/backups/pre-$release_id.dump"
compose "$release" exec -T postgres pg_dump -U warehouse -d warehouse -Fc > "$backup.tmp"
test -s "$backup.tmp"
mv -- "$backup.tmp" "$backup"
compose "$release" run --rm --no-deps migrate
# Provisioning is explicit and only enabled by private operator-controlled bootstrap settings.
if grep -q '^BOOTSTRAP_USERS=true$' "$root/.env"; then
  set -a
  # The private file is operator-owned shell-compatible KEY=value configuration.
  source "$root/.env"
  set +a
  compose "$release" run --rm --no-deps -e OPERATOR_EMAIL -e OPERATOR_PASSWORD -e SUPERVISOR_EMAIL -e SUPERVISOR_PASSWORD api provision-users
fi
compose "$release" up -d --no-build --pull never --force-recreate --wait --wait-timeout 180 api outbox-publisher notification-consumer frontend
api_endpoint=$(compose "$release" port api 8080)
web_endpoint=$(compose "$release" port frontend 8080)
[[ "$api_endpoint" == 127.0.0.1:* && "$web_endpoint" == 127.0.0.1:* ]]
for endpoint in "http://$api_endpoint/api/v1/ready" "http://$web_endpoint/health" "http://$web_endpoint/receivings"; do
  curl --fail --silent --show-error --max-time 15 "$endpoint" > /dev/null
done
ln -s -- "$release" "$root/.current-$release_id"
mv -Tf -- "$root/.current-$release_id" "$root/current"
changed=0
echo "Warehouse release $release_id is healthy on loopback. Host Nginx/public acceptance is a separate gate."
# No global prune, release deletion, volume deletion, host Nginx operation or automatic schema rollback.
