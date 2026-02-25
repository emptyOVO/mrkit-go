#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

GO_BIN="${GO_BIN:-/Users/empty/.g/go/bin/go}"
MASTER_PORT="${MASTER_PORT:-11340}"
WORKER_ID="${WORKER_ID:-1}"
REDUCERS="${REDUCERS:-1}"
IN_RAM="${IN_RAM:-false}"
PLUGIN_PATH="${PLUGIN_PATH:-cmd/wc.so}"
INPUT_GLOB="${INPUT_GLOB:-txt/*.txt}"
RUN_DIR="${RUN_DIR:-$ROOT/.run/multi-node}"
LOG_FILE="${LOG_FILE:-$RUN_DIR/worker-${WORKER_ID}.log}"
PID_FILE="${PID_FILE:-$RUN_DIR/worker-${WORKER_ID}.pid}"

mkdir -p "$RUN_DIR"

if [ ! -f "$PLUGIN_PATH" ]; then
  echo "[worker] build plugin: $PLUGIN_PATH"
  "$GO_BIN" build -buildmode=plugin -o "$PLUGIN_PATH" ./mrapps/wc >/dev/null
fi

echo "[worker] start id=$WORKER_ID master_port=$MASTER_PORT"
"$GO_BIN" run ./cmd/legacy/worker/main/main.go \
  -i "$INPUT_GLOB" \
  -p "$PLUGIN_PATH" \
  -r "$REDUCERS" \
  -w "$WORKER_ID" \
  --port "$MASTER_PORT" \
  -m="$IN_RAM" \
  >"$LOG_FILE" 2>&1 &

echo $! > "$PID_FILE"
echo "[worker] pid=$(cat "$PID_FILE") log=$LOG_FILE"
