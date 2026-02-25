#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

ENV_FILE="${ENV_FILE:-$ROOT/deploy/.env}"
if [ -f "$ENV_FILE" ]; then
  # shellcheck disable=SC1090
  source "$ENV_FILE"
fi

ACTION="${1:-start}" # start | stop

SSH_USER="${SSH_USER:-ubuntu}"
SSH_PORT="${SSH_PORT:-22}"
SSH_KEY="${SSH_KEY:-$HOME/.ssh/id_rsa}"
WORKER_HOSTS="${WORKER_HOSTS:-}"
REMOTE_ROOT="${REMOTE_ROOT:-~/mrkit-go}"
MASTER_ADDR="${MASTER_ADDR:-127.0.0.1:11340}"
MASTER_PORT="${MASTER_PORT:-11340}"
REDUCERS="${REDUCERS:-1}"
IN_RAM="${IN_RAM:-false}"
PLUGIN_PATH="${PLUGIN_PATH:-cmd/wc.so}"
INPUT_GLOB="${INPUT_GLOB:-txt/*.txt}"
RUN_MODE="${RUN_MODE:-bin}"
GO_BIN="${GO_BIN:-go}"

if [ -z "$WORKER_HOSTS" ]; then
  echo "[fanout] WORKER_HOSTS is empty, set it in deploy/.env" >&2
  exit 2
fi

IFS=',' read -r -a HOST_ARR <<< "$WORKER_HOSTS"

ssh_opts=(
  -p "$SSH_PORT"
  -i "$SSH_KEY"
  -o StrictHostKeyChecking=no
  -o UserKnownHostsFile=/dev/null
)

for i in "${!HOST_ARR[@]}"; do
  host="${HOST_ARR[$i]}"
  worker_id="$((i + 1))"
  remote="$SSH_USER@$host"

  if [ "$ACTION" = "start" ]; then
    echo "[fanout] start worker_id=$worker_id host=$host"
    ssh "${ssh_opts[@]}" "$remote" "bash -lc '
      set -euo pipefail
      cd $REMOTE_ROOT
      if [ ! -f $PLUGIN_PATH ]; then
        $GO_BIN build -buildmode=plugin -o $PLUGIN_PATH ./mrapps/wc >/dev/null
      fi
      RUN_MODE=$RUN_MODE MASTER_ADDR=$MASTER_ADDR MASTER_PORT=$MASTER_PORT WORKER_ID=$worker_id \
      ADVERTISE_HOST=$host \
      REDUCERS=$REDUCERS IN_RAM=$IN_RAM PLUGIN_PATH=$PLUGIN_PATH INPUT_GLOB=\"$INPUT_GLOB\" \
      nohup ./deploy/start_worker.sh >/tmp/mrkit-worker-$worker_id.log 2>&1 &
    '"
  elif [ "$ACTION" = "stop" ]; then
    echo "[fanout] stop worker host=$host"
    ssh "${ssh_opts[@]}" "$remote" "bash -lc '
      set -euo pipefail
      cd $REMOTE_ROOT
      ./deploy/stop_all.sh >/tmp/mrkit-stop.log 2>&1 || true
      pkill -f legacy-worker >/dev/null 2>&1 || true
    '"
  else
    echo "[fanout] unknown action: $ACTION (use start|stop)" >&2
    exit 2
  fi
done
