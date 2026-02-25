#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

ENV_FILE="${ENV_FILE:-$ROOT/deploy/.env}"
if [ -f "$ENV_FILE" ]; then
  # shellcheck disable=SC1090
  source "$ENV_FILE"
fi

RUN_DIR="${RUN_DIR:-$ROOT/.run/multi-node}"

if [ -d "$RUN_DIR" ]; then
  for pidf in "$RUN_DIR"/*.pid; do
    [ -f "$pidf" ] || continue
    pid="$(cat "$pidf" 2>/dev/null || true)"
    if [ -n "$pid" ] && kill -0 "$pid" >/dev/null 2>&1; then
      kill "$pid" >/dev/null 2>&1 || true
      sleep 0.2
      kill -9 "$pid" >/dev/null 2>&1 || true
      echo "[stop] killed pid=$pid from $(basename "$pidf")"
    fi
    rm -f "$pidf"
  done
fi

pkill -f '/cmd/legacy/master/main.go|/cmd/legacy/worker/main.go' >/dev/null 2>&1 || true
pkill -f 'legacy-master|legacy-worker' >/dev/null 2>&1 || true

echo "[stop] done"
