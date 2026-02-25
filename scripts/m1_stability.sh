#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

GO_BIN="${GO_BIN:-/Users/empty/.g/go/bin/go}"
ROUNDS="${ROUNDS:-10}"
ROWS="${ROWS:-5000}"
KEY_MOD="${KEY_MOD:-100}"
MR_PORT_BASE="${MR_PORT_BASE:-26000}"
FLOW_TIMEOUT_SEC="${FLOW_TIMEOUT_SEC:-240}"

ENABLE_LEGACY_WORKER_CHAOS="${ENABLE_LEGACY_WORKER_CHAOS:-0}"
CHAOS_ROUNDS="${CHAOS_ROUNDS:-5}"
CHAOS_WORKERS="${CHAOS_WORKERS:-4}"
CHAOS_REDUCERS="${CHAOS_REDUCERS:-1}"
CHAOS_KILL_DELAY_SEC="${CHAOS_KILL_DELAY_SEC:-2}"

MYSQL_CONTAINER="${MYSQL_CONTAINER:-mrkit-m1-mysql}"
REDIS_CONTAINER="${REDIS_CONTAINER:-mrkit-m1-redis}"
MYSQL_PORT="${MYSQL_PORT:-13306}"
REDIS_PORT="${REDIS_PORT:-16379}"
MYSQL_PASSWORD="${MYSQL_PASSWORD:-123456}"

KEEP_CONTAINERS="${KEEP_CONTAINERS:-0}"
KEEP_ARTIFACTS="${KEEP_ARTIFACTS:-1}"

REPORT_ROOT="${REPORT_ROOT:-$ROOT/reports/m1}"
RUN_ID="${RUN_ID:-$(date +%Y%m%d_%H%M%S)}"
RUN_DIR="$REPORT_ROOT/$RUN_ID"
FLOW_DIR="$RUN_DIR/flows"
DETAILS_NDJSON="$RUN_DIR/details.ndjson"
SUMMARY_JSON="$RUN_DIR/summary.json"
SUMMARY_TXT="$RUN_DIR/summary.txt"

mkdir -p "$RUN_DIR" "$FLOW_DIR" .cache/go-build .cache/go-mod .cache/go-tmp
: > "$DETAILS_NDJSON"

export GOCACHE="${GOCACHE:-$ROOT/.cache/go-build}"
export GOMODCACHE="${GOMODCACHE:-$ROOT/.cache/go-mod}"
export GOTMPDIR="${GOTMPDIR:-$ROOT/.cache/go-tmp}"

require_cmd() {
  command -v "$1" >/dev/null 2>&1 || { echo "[m1] missing command: $1" >&2; exit 1; }
}

require_cmd docker
require_cmd jq
require_cmd perl
require_cmd awk
require_cmd python3
command -v "$GO_BIN" >/dev/null 2>&1 || { echo "[m1] go binary not found: $GO_BIN" >&2; exit 1; }

cleanup() {
  if [ "$KEEP_CONTAINERS" = "0" ]; then
    docker rm -f "$MYSQL_CONTAINER" "$REDIS_CONTAINER" >/dev/null 2>&1 || true
  fi
  if [ "$KEEP_ARTIFACTS" = "0" ]; then
    rm -f -- mr-out-*.txt output/imd-*.txt cmd/wc.so >/dev/null 2>&1 || true
  fi
}
trap cleanup EXIT

log_event() {
  local round="$1" scenario="$2" status="$3" duration_ms="$4" message="$5"
  jq -nc \
    --arg ts "$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
    --arg round "$round" \
    --arg scenario "$scenario" \
    --arg status "$status" \
    --arg duration_ms "$duration_ms" \
    --arg message "$message" \
    '{ts:$ts,round:($round|tonumber),scenario:$scenario,status:$status,duration_ms:($duration_ms|tonumber),message:$message}' \
    >> "$DETAILS_NDJSON"
}

ms_now() {
  python3 - <<'PY'
import time
print(int(time.time() * 1000))
PY
}

start_services() {
  echo "[m1] start MySQL/Redis containers"
  docker rm -f "$MYSQL_CONTAINER" "$REDIS_CONTAINER" >/dev/null 2>&1 || true
  docker run -d \
    --name "$MYSQL_CONTAINER" \
    -e MYSQL_ROOT_PASSWORD="$MYSQL_PASSWORD" \
    -e MYSQL_DATABASE=mysql \
    -p "${MYSQL_PORT}:3306" \
    mysql:8.0 \
    --default-authentication-plugin=mysql_native_password \
    >/dev/null

  docker run -d \
    --name "$REDIS_CONTAINER" \
    -p "${REDIS_PORT}:6379" \
    redis:7-alpine \
    >/dev/null

  echo "[m1] wait MySQL SQL login"
  for _ in $(seq 1 90); do
    if docker exec "$MYSQL_CONTAINER" mysql -uroot -p"$MYSQL_PASSWORD" -e "SELECT 1" >/dev/null 2>&1; then
      break
    fi
    sleep 1
  done
  docker exec "$MYSQL_CONTAINER" mysql -uroot -p"$MYSQL_PASSWORD" -e "SELECT 1" >/dev/null

  echo "[m1] wait Redis ready"
  for _ in $(seq 1 30); do
    if [ "$(docker exec "$REDIS_CONTAINER" redis-cli ping 2>/dev/null || true)" = "PONG" ]; then
      break
    fi
    sleep 1
  done
  [ "$(docker exec "$REDIS_CONTAINER" redis-cli ping)" = "PONG" ]

  docker exec "$MYSQL_CONTAINER" mysql -uroot -p"$MYSQL_PASSWORD" -e "CREATE DATABASE IF NOT EXISTS mr_target;" >/dev/null
}

prepare_data() {
  echo "[m1] prepare MySQL source data rows=$ROWS key_mod=$KEY_MOD"
  MYSQL_HOST=127.0.0.1 \
  MYSQL_PORT="$MYSQL_PORT" \
  MYSQL_USER=root \
  MYSQL_PASSWORD="$MYSQL_PASSWORD" \
  MYSQL_DB=mysql \
  SOURCE_TABLE=source_events \
  TARGET_TABLE=agg_results \
  ROWS="$ROWS" \
  KEY_MOD="$KEY_MOD" \
  "$GO_BIN" run ./cmd/batch -mode prepare >/dev/null
}

rewrite_flow() {
  local in="$1" out="$2" port="$3"
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
    "$in" > "$out"
}

generate_round_flows() {
  local base="$1"
  rewrite_flow example/batch-minimal/flows/seed/flow.seed.redis_source_event.json "$FLOW_DIR/seed.json" $((base + 0))
  rewrite_flow example/batch-minimal/flows/smoke/flow.mysql.count.json "$FLOW_DIR/m2m.json" $((base + 1))
  rewrite_flow example/batch-minimal/flows/cross-db/flow.mysql_to_redis.count.json "$FLOW_DIR/m2r.json" $((base + 2))
  rewrite_flow example/batch-minimal/flows/cross-db/flow.redis_to_mysql.count.json "$FLOW_DIR/r2m.json" $((base + 3))
  rewrite_flow example/batch-minimal/flows/cross-db/flow.redis_to_redis.count.json "$FLOW_DIR/r2r.json" $((base + 4))
}

run_flow_once() {
  local round="$1" name="$2" cfg="$3"
  local start_ms end_ms duration_ms
  start_ms="$(ms_now)"
  if "$GO_BIN" run ./cmd/batch -check -config "$cfg" >/dev/null 2>&1 && \
     perl -e 'my $t=shift @ARGV; alarm $t; exec @ARGV;' "$FLOW_TIMEOUT_SEC" "$GO_BIN" run ./cmd/batch -config "$cfg" >/dev/null 2>&1; then
    end_ms="$(ms_now)"
    duration_ms=$((end_ms - start_ms))
    log_event "$round" "$name" "pass" "$duration_ms" "ok"
    return 0
  fi
  end_ms="$(ms_now)"
  duration_ms=$((end_ms - start_ms))
  log_event "$round" "$name" "fail" "$duration_ms" "check or run failed"
  return 1
}

verify_outputs() {
  local m2m_rows r2m_rows seed_keys m2r_keys r2r_keys
  m2m_rows="$(docker exec "$MYSQL_CONTAINER" mysql -N -uroot -p"$MYSQL_PASSWORD" -e "SELECT COUNT(*) FROM mr_target.agg_count_results;" | tr -d '\r')"
  r2m_rows="$(docker exec "$MYSQL_CONTAINER" mysql -N -uroot -p"$MYSQL_PASSWORD" -e "SELECT COUNT(*) FROM mr_target.agg_from_redis_count;" | tr -d '\r')"
  seed_keys="$(docker exec "$REDIS_CONTAINER" sh -lc "redis-cli -n 0 --scan --pattern 'event:*' | wc -l | tr -d ' '")"
  m2r_keys="$(docker exec "$REDIS_CONTAINER" sh -lc "redis-cli -n 1 --scan --pattern 'mr:count:*' | wc -l | tr -d ' '")"
  r2r_keys="$(docker exec "$REDIS_CONTAINER" sh -lc "redis-cli -n 2 --scan --pattern 'mr:rr:count:*' | wc -l | tr -d ' '")"

  jq -nc \
    --arg ts "$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
    --arg expected "$KEY_MOD" \
    --arg m2m "$m2m_rows" \
    --arg r2m "$r2m_rows" \
    --arg seed "$seed_keys" \
    --arg m2r "$m2r_keys" \
    --arg r2r "$r2r_keys" \
    '{ts:$ts,expected:($expected|tonumber),m2m_rows:($m2m|tonumber),r2m_rows:($r2m|tonumber),seed_keys:($seed|tonumber),m2r_keys:($m2r|tonumber),r2r_keys:($r2r|tonumber)}' \
    > "$RUN_DIR/final_counts.json"

  [ "$m2m_rows" = "$KEY_MOD" ] && [ "$r2m_rows" = "$KEY_MOD" ] && [ "$seed_keys" = "$KEY_MOD" ] && [ "$m2r_keys" = "$KEY_MOD" ] && [ "$r2r_keys" = "$KEY_MOD" ]
}

legacy_worker_chaos() {
  echo "[m1] run optional legacy worker-chaos rounds=$CHAOS_ROUNDS"
  "$GO_BIN" build -buildmode=plugin -o cmd/wc.so ./mrapps/wc >/dev/null
  local r
  for r in $(seq 1 "$CHAOS_ROUNDS"); do
    local base_port mpid victim_idx victim_pid w status start_ms end_ms duration_ms
    base_port=$((MR_PORT_BASE + 1000 + r * 10))
    start_ms="$(ms_now)"
    rm -f -- mr-out-*.txt output/imd-*.txt

    "$GO_BIN" run ./cmd/legacy/master/main.go -i "txt/*.txt" -p "cmd/wc.so" -r "$CHAOS_REDUCERS" -w "$CHAOS_WORKERS" --port "$base_port" -m=false >/tmp/m1_legacy_master_${r}.log 2>&1 &
    mpid=$!
    sleep 1

    declare -a worker_pids=()
    for w in $(seq 1 "$CHAOS_WORKERS"); do
      "$GO_BIN" run ./cmd/legacy/worker/main.go -i "txt/*.txt" -p "cmd/wc.so" -r "$CHAOS_REDUCERS" -w "$w" --port "$base_port" -m=false >/tmp/m1_legacy_worker_${r}_${w}.log 2>&1 &
      worker_pids+=("$!")
    done

    sleep "$CHAOS_KILL_DELAY_SEC"
    victim_idx=$((RANDOM % CHAOS_WORKERS))
    victim_pid="${worker_pids[$victim_idx]}"
    kill -9 "$victim_pid" >/dev/null 2>&1 || true

    status=pass
    wait "$mpid" || status=fail
    for pid in "${worker_pids[@]}"; do
      wait "$pid" >/dev/null 2>&1 || true
    done
    [ -f mr-out-0.txt ] || status=fail

    end_ms="$(ms_now)"
    duration_ms=$((end_ms - start_ms))
    log_event "$r" "legacy_worker_chaos" "$status" "$duration_ms" "killed_worker_pid=$victim_pid"
  done
}

build_summary() {
  local total pass fail pass_rate
  total="$(jq -s 'length' "$DETAILS_NDJSON")"
  pass="$(jq -s '[.[] | select(.status=="pass")] | length' "$DETAILS_NDJSON")"
  fail="$(jq -s '[.[] | select(.status=="fail")] | length' "$DETAILS_NDJSON")"
  if [ "$total" -eq 0 ]; then
    pass_rate="0"
  else
    pass_rate="$(awk -v p="$pass" -v t="$total" 'BEGIN{printf "%.2f", (p*100.0)/t}')"
  fi

  jq -n \
    --arg run_id "$RUN_ID" \
    --arg rounds "$ROUNDS" \
    --arg chaos "$ENABLE_LEGACY_WORKER_CHAOS" \
    --arg total "$total" \
    --arg pass "$pass" \
    --arg fail "$fail" \
    --arg pass_rate "$pass_rate" \
    --arg generated_at "$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
    --slurpfile events "$DETAILS_NDJSON" \
    '{
      run_id:$run_id,
      rounds:($rounds|tonumber),
      legacy_worker_chaos_enabled:($chaos=="1"),
      total_cases:($total|tonumber),
      passed:($pass|tonumber),
      failed:($fail|tonumber),
      pass_rate_pct:($pass_rate|tonumber),
      generated_at:$generated_at,
      per_scenario: ($events | sort_by(.scenario) | group_by(.scenario) | map({scenario: .[0].scenario,total:length,passed:([.[]|select(.status=="pass")]|length),failed:([.[]|select(.status=="fail")]|length),avg_ms:(if length==0 then 0 else (map(.duration_ms)|add/length|floor) end)}))
    }' > "$SUMMARY_JSON"

  {
    echo "M1 Stability Report"
    echo "run_id: $RUN_ID"
    echo "rounds: $ROUNDS"
    echo "legacy_worker_chaos: $ENABLE_LEGACY_WORKER_CHAOS"
    echo "total_cases: $total"
    echo "passed: $pass"
    echo "failed: $fail"
    echo "pass_rate_pct: $pass_rate"
    echo
    echo "per_scenario:"
    jq -r '.per_scenario[] | "- \(.scenario): total=\(.total), pass=\(.passed), fail=\(.failed), avg_ms=\(.avg_ms)"' "$SUMMARY_JSON"
    if [ -f "$RUN_DIR/final_counts.json" ]; then
      echo
      echo "final_counts:"
      jq -r '"- expected=\(.expected), m2m=\(.m2m_rows), r2m=\(.r2m_rows), seed=\(.seed_keys), m2r=\(.m2r_keys), r2r=\(.r2r_keys)"' "$RUN_DIR/final_counts.json"
    fi
  } > "$SUMMARY_TXT"
}

main() {
  echo "[m1] run_id=$RUN_ID rounds=$ROUNDS"
  start_services
  prepare_data

  local r base any_fail
  any_fail=0
  for r in $(seq 1 "$ROUNDS"); do
    echo "[m1] round $r/$ROUNDS"
    base=$((MR_PORT_BASE + r * 10))
    generate_round_flows "$base"

    run_flow_once "$r" "seed" "$FLOW_DIR/seed.json" || any_fail=1
    run_flow_once "$r" "m2m" "$FLOW_DIR/m2m.json" || any_fail=1
    run_flow_once "$r" "m2r" "$FLOW_DIR/m2r.json" || any_fail=1
    run_flow_once "$r" "r2m" "$FLOW_DIR/r2m.json" || any_fail=1
    run_flow_once "$r" "r2r" "$FLOW_DIR/r2r.json" || any_fail=1
  done

  if ! verify_outputs; then
    any_fail=1
    echo "[m1] final output verification failed"
  fi

  if [ "$ENABLE_LEGACY_WORKER_CHAOS" = "1" ]; then
    legacy_worker_chaos || any_fail=1
  fi

  build_summary

  echo "[m1] summary: $SUMMARY_TXT"
  echo "[m1] json: $SUMMARY_JSON"
  if [ "$any_fail" -ne 0 ]; then
    echo "[m1] stability check failed" >&2
    exit 1
  fi
  echo "[m1] stability check passed"
}

main
