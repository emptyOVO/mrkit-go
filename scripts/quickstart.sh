#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

GO_BIN="${GO_BIN:-go}"

MYSQL_CONTAINER="${MYSQL_CONTAINER:-mrkit-quickstart-mysql}"
REDIS_CONTAINER="${REDIS_CONTAINER:-mrkit-quickstart-redis}"
MYSQL_PORT="${MYSQL_PORT:-13306}"
REDIS_PORT="${REDIS_PORT:-16379}"
MYSQL_PASSWORD="${MYSQL_PASSWORD:-123456}"
ROWS="${ROWS:-5000}"
KEY_MOD="${KEY_MOD:-100}"
MR_PORT_BASE="${MR_PORT_BASE:-24010}"

require_cmd() {
  command -v "$1" >/dev/null 2>&1 || {
    echo "[quickstart] missing command: $1" >&2
    exit 1
  }
}

require_cmd docker
require_cmd jq
if ! command -v "$GO_BIN" >/dev/null 2>&1; then
  echo "[quickstart] go command not found: $GO_BIN" >&2
  echo "[quickstart] set GO_BIN explicitly, e.g. GO_BIN=/path/to/go ./scripts/quickstart.sh" >&2
  exit 1
fi

mkdir -p .cache/go-build .cache/go-mod .cache/go-tmp
export GOCACHE="${GOCACHE:-$ROOT/.cache/go-build}"
export GOMODCACHE="${GOMODCACHE:-$ROOT/.cache/go-mod}"
export GOTMPDIR="${GOTMPDIR:-$ROOT/.cache/go-tmp}"

echo "[quickstart] clean stale process/cache"
pkill -f '/cmd/batch -config' >/dev/null 2>&1 || true
pkill -f '/cmd/batch -check' >/dev/null 2>&1 || true
rm -rf .cache/batch-builtins .cache/mysqlbatch-builtins
rm -f -- mr-out-*.txt output/imd-*.txt

echo "[quickstart] start mysql container"
docker rm -f "$MYSQL_CONTAINER" >/dev/null 2>&1 || true
docker run -d \
  --name "$MYSQL_CONTAINER" \
  -e MYSQL_ROOT_PASSWORD="$MYSQL_PASSWORD" \
  -e MYSQL_DATABASE=mysql \
  -p "${MYSQL_PORT}:3306" \
  mysql:8.0 \
  --default-authentication-plugin=mysql_native_password \
  >/dev/null

echo "[quickstart] start redis container"
docker rm -f "$REDIS_CONTAINER" >/dev/null 2>&1 || true
docker run -d \
  --name "$REDIS_CONTAINER" \
  -p "${REDIS_PORT}:6379" \
  redis:7-alpine \
  >/dev/null

echo "[quickstart] wait mysql ready"
for _ in $(seq 1 60); do
  if docker exec "$MYSQL_CONTAINER" mysqladmin ping -uroot -p"$MYSQL_PASSWORD" --silent >/dev/null 2>&1; then
    break
  fi
  sleep 1
done
docker exec "$MYSQL_CONTAINER" mysqladmin ping -uroot -p"$MYSQL_PASSWORD" --silent >/dev/null

echo "[quickstart] wait redis ready"
for _ in $(seq 1 30); do
  if [ "$(docker exec "$REDIS_CONTAINER" redis-cli ping 2>/dev/null || true)" = "PONG" ]; then
    break
  fi
  sleep 1
done
[ "$(docker exec "$REDIS_CONTAINER" redis-cli ping)" = "PONG" ]

echo "[quickstart] create mysql target db"
docker exec "$MYSQL_CONTAINER" \
  mysql -uroot -p"$MYSQL_PASSWORD" \
  -e "CREATE DATABASE IF NOT EXISTS mr_target;" >/dev/null

echo "[quickstart] prepare synthetic source table"
MYSQL_HOST=127.0.0.1 \
MYSQL_PORT="$MYSQL_PORT" \
MYSQL_USER=root \
MYSQL_PASSWORD="$MYSQL_PASSWORD" \
MYSQL_DB=mysql \
SOURCE_TABLE=source_events \
TARGET_TABLE=agg_results \
ROWS="$ROWS" \
KEY_MOD="$KEY_MOD" \
"$GO_BIN" run ./cmd/batch -mode prepare

FLOW_DIR="$(mktemp -d /tmp/mrkit-quickstart-flows.XXXXXX)"
echo "[quickstart] generate runtime flows in $FLOW_DIR"

rewrite_flow() {
  local in="$1"
  local out="$2"
  local port="$3"
  jq \
    --arg host "127.0.0.1" \
    --argjson mysql_port "$MYSQL_PORT" \
    --argjson redis_port "$REDIS_PORT" \
    --argjson mr_port "$port" \
    '
      if .source.type=="mysql" then
        .source.db.host=$host | .source.db.port=$mysql_port
      elif .source.type=="redis" then
        .source.redis.host=$host | .source.redis.port=$redis_port
      else . end
      | if .sink.type=="mysql" then
          .sink.db.host=$host | .sink.db.port=$mysql_port
        elif .sink.type=="redis" then
          .sink.redis.host=$host | .sink.redis.port=$redis_port
        else . end
      | .transform.port=$mr_port
    ' \
    "$in" >"$out"
}

rewrite_flow example/batch-minimal/flows/seed/flow.seed.redis_source_event.json "$FLOW_DIR/seed.json" $((MR_PORT_BASE + 0))
rewrite_flow example/batch-minimal/flows/smoke/flow.mysql.count.json "$FLOW_DIR/m2m.json" $((MR_PORT_BASE + 1))
rewrite_flow example/batch-minimal/flows/cross-db/flow.mysql_to_redis.count.json "$FLOW_DIR/m2r.json" $((MR_PORT_BASE + 2))
rewrite_flow example/batch-minimal/flows/cross-db/flow.redis_to_mysql.count.json "$FLOW_DIR/r2m.json" $((MR_PORT_BASE + 3))
rewrite_flow example/batch-minimal/flows/cross-db/flow.redis_to_redis.count.json "$FLOW_DIR/r2r.json" $((MR_PORT_BASE + 4))

run_flow() {
  local name="$1"
  local cfg="$2"
  echo "[quickstart] $name check"
  "$GO_BIN" run ./cmd/batch -check -config "$cfg"
  echo "[quickstart] $name run"
  "$GO_BIN" run ./cmd/batch -config "$cfg"
}

run_flow seed "$FLOW_DIR/seed.json"
run_flow m2m "$FLOW_DIR/m2m.json"
run_flow m2r "$FLOW_DIR/m2r.json"
run_flow r2m "$FLOW_DIR/r2m.json"
run_flow r2r "$FLOW_DIR/r2r.json"

mysql_count() {
  local table="$1"
  docker exec "$MYSQL_CONTAINER" \
    mysql -N -uroot -p"$MYSQL_PASSWORD" \
    -e "SELECT COUNT(*) FROM mr_target.${table};" | tr -d '\r'
}

redis_count() {
  local db="$1"
  local pattern="$2"
  docker exec "$REDIS_CONTAINER" sh -lc "redis-cli -n ${db} --scan --pattern '${pattern}' | wc -l | tr -d ' '"
}

echo "[quickstart] verify results"
m2m_rows="$(mysql_count agg_count_results)"
r2m_rows="$(mysql_count agg_from_redis_count)"
seed_keys="$(redis_count 0 'event:*')"
m2r_keys="$(redis_count 1 'mr:count:*')"
r2r_keys="$(redis_count 2 'mr:rr:count:*')"

echo "----- quickstart result -----"
echo "mysql mr_target.agg_count_results rows: $m2m_rows"
echo "mysql mr_target.agg_from_redis_count rows: $r2m_rows"
echo "redis db0 event:* keys: $seed_keys"
echo "redis db1 mr:count:* keys: $m2r_keys"
echo "redis db2 mr:rr:count:* keys: $r2r_keys"
echo "expected distinct keys (KEY_MOD): $KEY_MOD"

if [ "$m2m_rows" != "$KEY_MOD" ] || [ "$r2m_rows" != "$KEY_MOD" ] || \
   [ "$seed_keys" != "$KEY_MOD" ] || [ "$m2r_keys" != "$KEY_MOD" ] || [ "$r2r_keys" != "$KEY_MOD" ]; then
  echo "[quickstart] verification failed" >&2
  exit 1
fi

echo "[quickstart] all checks passed"
echo "[quickstart] to stop services: docker rm -f $MYSQL_CONTAINER $REDIS_CONTAINER"
