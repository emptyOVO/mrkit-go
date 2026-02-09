#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
GO_BIN="${GO_BIN:-/Users/empty/.g/go/bin/go}"

cd "$ROOT"

mkdir -p .cache/go-build .cache/go-mod .cache/go-tmp
export GOCACHE="${GOCACHE:-$ROOT/.cache/go-build}"
export GOMODCACHE="${GOMODCACHE:-$ROOT/.cache/go-mod}"
export GOTMPDIR="${GOTMPDIR:-$ROOT/.cache/go-tmp}"

run_flow() {
  local name="$1"
  local cfg="$2"

  echo "[E2E] $name"
  local try
  for try in 1 2 3; do
    local port cfg_run
    port=$((20000 + RANDOM % 20000))
    cfg_run="/tmp/mrkit-e2e-${port}.json"
    jq --argjson p "$port" '.transform.port = $p' "$cfg" > "$cfg_run"

    pkill -f '/Users/empty/.g/go/bin/go run ./cmd/batch' >/dev/null 2>&1 || true
    rm -f -- mr-out-*.txt output/imd-*.txt
    if "$GO_BIN" run ./cmd/batch -check -config "$cfg_run" && \
       perl -e 'my $t=shift @ARGV; alarm $t; exec @ARGV;' 180 "$GO_BIN" run ./cmd/batch -config "$cfg_run"; then
      return 0
    fi
    if [ "$try" -lt 3 ]; then
      echo "[E2E] $name failed on attempt ${try}, retrying..."
      sleep 1
    fi
  done
  echo "[E2E] $name failed after 3 attempts"
  return 1
}

run_flow "seed(mysql->redis db0 event:*)" "example/batch-minimal/flows/seed/flow.seed.redis_source_event.json"
run_flow "m2m(mysql->mysql count)" "example/batch-minimal/flows/smoke/flow.mysql.count.json"
run_flow "m2r(mysql->redis count)" "example/batch-minimal/flows/cross-db/flow.mysql_to_redis.count.json"
run_flow "r2m(redis->mysql count)" "example/batch-minimal/flows/cross-db/flow.redis_to_mysql.count.json"
run_flow "r2r(redis->redis count)" "example/batch-minimal/flows/cross-db/flow.redis_to_redis.count.json"

echo "[E2E] done"
