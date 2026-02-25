#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

GO_BIN="${GO_BIN:-/Users/empty/.g/go/bin/go}"
MASTER_PORT="${MASTER_PORT:-11340}"
REDUCERS="${REDUCERS:-1}"
WORKERS="${WORKERS:-2}"
IN_RAM="${IN_RAM:-false}"
PLUGIN_PATH="${PLUGIN_PATH:-cmd/wc.so}"
INPUT_GLOB="${INPUT_GLOB:-txt/*.txt}"
RUN_DIR="${RUN_DIR:-$ROOT/.run/multi-node}"
LOG_FILE="${LOG_FILE:-$RUN_DIR/master.log}"
PID_FILE="${PID_FILE:-$RUN_DIR/master.pid}"

mkdir -p "$RUN_DIR"

if [ ! -f "$PLUGIN_PATH" ]; then
  echo "[master] build plugin: $PLUGIN_PATH"
  "$GO_BIN" build -buildmode=plugin -o "$PLUGIN_PATH" ./mrapps/wc >/dev/null
fi

echo "[master] start port=$MASTER_PORT workers=$WORKERS reducers=$REDUCERS"
"$GO_BIN" run ./cmd/legacy/master/main/main.go \
  -i "$INPUT_GLOB" \
  -p "$PLUGIN_PATH" \
  -r "$REDUCERS" \
  -w "$WORKERS" \
  --port "$MASTER_PORT" \
  -m="$IN_RAM" \
  >"$LOG_FILE" 2>&1 &

echo $! > "$PID_FILE"
echo "[master] pid=$(cat "$PID_FILE") log=$LOG_FILE"
