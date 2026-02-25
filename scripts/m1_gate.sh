#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

RUN_PATH="${1:-}"
MIN_PASS_RATE="${MIN_PASS_RATE:-95}"

if [ -z "$RUN_PATH" ]; then
  echo "usage: $0 <run_id|report_dir>" >&2
  exit 2
fi

if [ -d "$RUN_PATH" ]; then
  RUN_DIR="$RUN_PATH"
else
  RUN_DIR="$ROOT/reports/m1/$RUN_PATH"
fi

SUMMARY_JSON="$RUN_DIR/summary.json"
COUNTS_JSON="$RUN_DIR/final_counts.json"

[ -f "$SUMMARY_JSON" ] || { echo "summary not found: $SUMMARY_JSON" >&2; exit 2; }
[ -f "$COUNTS_JSON" ] || { echo "final counts not found: $COUNTS_JSON" >&2; exit 2; }

pass_rate="$(jq -r '.pass_rate_pct' "$SUMMARY_JSON")"
failed_cases="$(jq -r '.failed' "$SUMMARY_JSON")"
expected="$(jq -r '.expected' "$COUNTS_JSON")"
m2m="$(jq -r '.m2m_rows' "$COUNTS_JSON")"
r2m="$(jq -r '.r2m_rows' "$COUNTS_JSON")"
seed="$(jq -r '.seed_keys' "$COUNTS_JSON")"
m2r="$(jq -r '.m2r_keys' "$COUNTS_JSON")"
r2r="$(jq -r '.r2r_keys' "$COUNTS_JSON")"

python3 - <<PY
import sys
pass_rate=float("$pass_rate")
min_rate=float("$MIN_PASS_RATE")
if pass_rate < min_rate:
    print(f"[gate] FAIL: pass_rate_pct={pass_rate:.2f} < {min_rate:.2f}")
    sys.exit(1)
print(f"[gate] PASS: pass_rate_pct={pass_rate:.2f} >= {min_rate:.2f}")
PY

if [ "$failed_cases" != "0" ]; then
  echo "[gate] WARN: summary failed cases=$failed_cases"
fi

if [ "$m2m" != "$expected" ] || [ "$r2m" != "$expected" ] || [ "$seed" != "$expected" ] || [ "$m2r" != "$expected" ] || [ "$r2r" != "$expected" ]; then
  echo "[gate] FAIL: final counts mismatch expected=$expected m2m=$m2m r2m=$r2m seed=$seed m2r=$m2r r2r=$r2r" >&2
  exit 1
fi

echo "[gate] PASS: final counts all equal expected=$expected"
echo "[gate] run_dir=$RUN_DIR"
