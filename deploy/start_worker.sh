#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

ENV_FILE="${ENV_FILE:-$ROOT/deploy/.env}"
if [ -f "$ENV_FILE" ]; then
  # shellcheck disable=SC1090
  source "$ENV_FILE"
fi

GO_BIN="${GO_BIN:-/Users/empty/.g/go/bin/go}"
MASTER_PORT="${MASTER_PORT:-11340}"
MASTER_ADDR="${MASTER_ADDR:-}"
ADVERTISE_HOST="${ADVERTISE_HOST:-}"
WORKER_ID="${WORKER_ID:-1}"
REDUCERS="${REDUCERS:-1}"
IN_RAM="${IN_RAM:-false}"
RUN_MODE="${RUN_MODE:-go-run}" # go-run | bin
WORKER_BIN="${WORKER_BIN:-$ROOT/bin/legacy-worker}"
PLUGIN_PATH="${PLUGIN_PATH:-cmd/wc.so}"
FORCE_REBUILD_PLUGIN="${FORCE_REBUILD_PLUGIN:-0}"
INPUT_GLOB="${INPUT_GLOB:-txt/*.txt}"
RUN_DIR="${RUN_DIR:-$ROOT/.run/multi-node}"
LOG_FILE="${LOG_FILE:-$RUN_DIR/worker-${WORKER_ID}.log}"
PID_FILE="${PID_FILE:-$RUN_DIR/worker-${WORKER_ID}.pid}"

mkdir -p "$RUN_DIR"

if [ "$FORCE_REBUILD_PLUGIN" = "1" ] || [ ! -f "$PLUGIN_PATH" ]; then
  echo "[worker] build plugin: $PLUGIN_PATH"
  "$GO_BIN" build -buildmode=plugin -o "$PLUGIN_PATH" ./mrapps/wc >/dev/null
fi

MASTER_ARG=()
if [ -n "$MASTER_ADDR" ]; then
  MASTER_ARG=(--master "$MASTER_ADDR")
fi
ADVERTISE_ARG=()
if [ -n "$ADVERTISE_HOST" ]; then
  ADVERTISE_ARG=(--advertise-host "$ADVERTISE_HOST")
fi

echo "[worker] start id=$WORKER_ID master_port=$MASTER_PORT"
if [ "$RUN_MODE" = "bin" ]; then
  if [ ! -x "$WORKER_BIN" ]; then
    echo "[worker] missing executable WORKER_BIN=$WORKER_BIN" >&2
    echo "[worker] build it with: $GO_BIN build -o $WORKER_BIN ./cmd/legacy/worker/main" >&2
    exit 2
  fi
  "$WORKER_BIN" \
    -i "$INPUT_GLOB" \
    -p "$PLUGIN_PATH" \
    -r "$REDUCERS" \
    -w "$WORKER_ID" \
    --port "$MASTER_PORT" \
    "${MASTER_ARG[@]}" \
    "${ADVERTISE_ARG[@]}" \
    -m="$IN_RAM" \
    >"$LOG_FILE" 2>&1 &
else
  "$GO_BIN" run ./cmd/legacy/worker/main.go \
    -i "$INPUT_GLOB" \
    -p "$PLUGIN_PATH" \
    -r "$REDUCERS" \
    -w "$WORKER_ID" \
    --port "$MASTER_PORT" \
    "${MASTER_ARG[@]}" \
    "${ADVERTISE_ARG[@]}" \
    -m="$IN_RAM" \
    >"$LOG_FILE" 2>&1 &
fi

echo $! > "$PID_FILE"
echo "[worker] pid=$(cat "$PID_FILE") log=$LOG_FILE"
