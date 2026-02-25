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
REDUCERS="${REDUCERS:-1}"
WORKERS="${WORKERS:-2}"
IN_RAM="${IN_RAM:-false}"
RUN_MODE="${RUN_MODE:-go-run}" # go-run | bin
MASTER_BIN="${MASTER_BIN:-$ROOT/bin/legacy-master}"
PLUGIN_PATH="${PLUGIN_PATH:-cmd/wc.so}"
FORCE_REBUILD_PLUGIN="${FORCE_REBUILD_PLUGIN:-0}"
INPUT_GLOB="${INPUT_GLOB:-txt/*.txt}"
RUN_DIR="${RUN_DIR:-$ROOT/.run/multi-node}"
LOG_FILE="${LOG_FILE:-$RUN_DIR/master.log}"
PID_FILE="${PID_FILE:-$RUN_DIR/master.pid}"

mkdir -p "$RUN_DIR"

if [ "$FORCE_REBUILD_PLUGIN" = "1" ] || [ ! -f "$PLUGIN_PATH" ]; then
  echo "[master] build plugin: $PLUGIN_PATH"
  "$GO_BIN" build -buildmode=plugin -o "$PLUGIN_PATH" ./mrapps/wc >/dev/null
fi

echo "[master] start port=$MASTER_PORT workers=$WORKERS reducers=$REDUCERS"
if [ "$RUN_MODE" = "bin" ]; then
  if [ ! -x "$MASTER_BIN" ]; then
    echo "[master] missing executable MASTER_BIN=$MASTER_BIN" >&2
    echo "[master] build it with: $GO_BIN build -o $MASTER_BIN ./cmd/legacy/master/main" >&2
    exit 2
  fi
  "$MASTER_BIN" \
    -i "$INPUT_GLOB" \
    -p "$PLUGIN_PATH" \
    -r "$REDUCERS" \
    -w "$WORKERS" \
    --port "$MASTER_PORT" \
    -m="$IN_RAM" \
    >"$LOG_FILE" 2>&1 &
else
  "$GO_BIN" run ./cmd/legacy/master/main.go \
    -i "$INPUT_GLOB" \
    -p "$PLUGIN_PATH" \
    -r "$REDUCERS" \
    -w "$WORKERS" \
    --port "$MASTER_PORT" \
    -m="$IN_RAM" \
    >"$LOG_FILE" 2>&1 &
fi

echo $! > "$PID_FILE"
echo "[master] pid=$(cat "$PID_FILE") log=$LOG_FILE"
