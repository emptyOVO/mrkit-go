#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

GO_BIN="${GO_BIN:-go}"
ROWS="${ROWS:-100000}"
KEY_MOD="${KEY_MOD:-10000}"
ROUNDS="${ROUNDS:-3}"
REDUCERS_SET="${REDUCERS_SET:-1 4 8}"
MODE="${MODE:-both}" # runner|bin|both
AUTO_INSTALL_HADOOP="${AUTO_INSTALL_HADOOP:-1}" # 1: try install when hadoop missing
OUTROOT="${OUTROOT:-$ROOT/benchmark/hadoop-host}"
RUN_STAMP="${RUN_STAMP:-oneshot_$(date +%Y%m%d_%H%M%S)}"
SUMMARY_DIR="$OUTROOT/$RUN_STAMP"
SUMMARY_CSV="$SUMMARY_DIR/summary.csv"

require_cmd() {
  command -v "$1" >/dev/null 2>&1 || {
    echo "[compare] missing command: $1" >&2
    exit 1
  }
}

install_hadoop() {
  local os
  os="$(uname -s)"
  echo "[compare] hadoop not found, attempting auto install (AUTO_INSTALL_HADOOP=1)"

  if command -v brew >/dev/null 2>&1; then
    echo "[compare] install via Homebrew"
    brew list hadoop >/dev/null 2>&1 || brew install hadoop
    return
  fi

  if [[ "$os" == "Linux" ]] && command -v apt-get >/dev/null 2>&1; then
    if ! command -v sudo >/dev/null 2>&1; then
      echo "[compare] apt-get found but sudo missing; cannot auto install on this host" >&2
      return 1
    fi
    echo "[compare] try install via apt-get"
    sudo apt-get update
    if apt-cache show hadoop >/dev/null 2>&1; then
      sudo apt-get install -y hadoop
      return
    fi
    echo "[compare] package 'hadoop' not found in apt repos on this host" >&2
    return 1
  fi

  echo "[compare] auto install not supported on this environment" >&2
  echo "[compare] install hadoop manually, then re-run script" >&2
  return 1
}

if ! command -v "$GO_BIN" >/dev/null 2>&1; then
  echo "[compare] go command not found: $GO_BIN" >&2
  echo "[compare] set GO_BIN explicitly, e.g. GO_BIN=/path/to/go ./scripts/benchmark_hadoop_compare.sh" >&2
  exit 1
fi
require_cmd awk

if ! command -v hadoop >/dev/null 2>&1; then
  if [[ "$AUTO_INSTALL_HADOOP" == "1" ]]; then
    install_hadoop
  else
    echo "[compare] hadoop command missing and AUTO_INSTALL_HADOOP=0" >&2
    exit 1
  fi
fi
require_cmd hadoop

case "$MODE" in
  runner|bin|both) ;;
  *)
    echo "[compare] invalid MODE=$MODE (use runner|bin|both)" >&2
    exit 1
    ;;
esac

mkdir -p "$SUMMARY_DIR"
echo "mode,reducers,round,tool,rows,wall_ms,input_rps,output_rows,status,csv_path" >"$SUMMARY_CSV"

append_csv_rows() {
  local mode="$1"
  local csv="$2"
  awk -F, -v mode="$mode" -v csv="$csv" 'NR>1{print mode "," $0 "," csv}' "$csv" >>"$SUMMARY_CSV"
}

echo "[compare] start hadoop-vs-mrkit matrix"
echo "[compare] MODE=$MODE ROWS=$ROWS KEY_MOD=$KEY_MOD ROUNDS=$ROUNDS REDUCERS_SET=$REDUCERS_SET"

for reducers in $REDUCERS_SET; do
  if [[ "$MODE" == "runner" || "$MODE" == "both" ]]; then
    run_id="${RUN_STAMP}_runner_r${reducers}"
    echo "[compare] runner reducers=$reducers"
    GO_BIN="$GO_BIN" ROWS="$ROWS" KEY_MOD="$KEY_MOD" REDUCERS="$reducers" ROUNDS="$ROUNDS" OUTROOT="$OUTROOT" RUN_ID="$run_id" \
      ./scripts/run_host_hadoop_vs_mrkit_count.sh
    csv="$OUTROOT/$run_id/count_compare.csv"
    append_csv_rows "runner" "$csv"
  fi

  if [[ "$MODE" == "bin" || "$MODE" == "both" ]]; then
    run_id="${RUN_STAMP}_bin_r${reducers}"
    echo "[compare] bin reducers=$reducers"
    GO_BIN="$GO_BIN" ROWS="$ROWS" KEY_MOD="$KEY_MOD" REDUCERS="$reducers" ROUNDS="$ROUNDS" OUTROOT="$OUTROOT" RUN_ID="$run_id" \
      ./scripts/run_host_hadoop_vs_mrkit_count_bin.sh
    csv="$OUTROOT/$run_id/count_compare.csv"
    append_csv_rows "bin" "$csv"
  fi
done

echo "[compare] summary: $SUMMARY_CSV"
echo "[compare] quick view:"
awk -F, 'NR==1 || $9!="PASS" || $4 ~ /mrkit|hadoop/ {print}' "$SUMMARY_CSV"
