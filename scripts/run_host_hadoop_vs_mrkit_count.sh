#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
GO_BIN="${GO_BIN:-go}"
ROWS="${ROWS:-100000}"
KEY_MOD="${KEY_MOD:-10000}"
REDUCERS="${REDUCERS:-4}"
ROUNDS="${ROUNDS:-3}"
MR_PORT_BASE="${MR_PORT_BASE:-19400}"

OUTROOT="${OUTROOT:-$ROOT/benchmark/hadoop-host}"
RUN_ID="${RUN_ID:-results_$(date +%Y%m%d_%H%M%S)}"
OUTDIR="$OUTROOT/$RUN_ID"
PLUGIN_DIR="$OUTDIR/plugins"
INPUT="$OUTDIR/input.tsv"
CSV="$OUTDIR/count_compare.csv"

mkdir -p "$OUTDIR" "$PLUGIN_DIR"

ms_now() {
  python3 - <<'PY'
import time
print(int(time.time() * 1000))
PY
}

require_cmd() {
  command -v "$1" >/dev/null 2>&1 || {
    echo "missing command: $1" >&2
    exit 1
  }
}

if ! command -v "$GO_BIN" >/dev/null 2>&1; then
  echo "go command not found: $GO_BIN" >&2
  echo "set GO_BIN explicitly, e.g. GO_BIN=/path/to/go $0" >&2
  exit 1
fi
require_cmd hadoop
require_cmd awk

HADOOP_STREAMING_JAR="${HADOOP_STREAMING_JAR:-$(hadoop classpath 2>/dev/null | tr ':' '\n' | grep -E 'hadoop-streaming-.*\.jar' | head -n1 || true)}"
if [[ -z "$HADOOP_STREAMING_JAR" || ! -f "$HADOOP_STREAMING_JAR" ]]; then
  HADOOP_HOME_DIR="${HADOOP_HOME:-$(cd "$(dirname "$(command -v hadoop 2>/dev/null || echo /tmp)")/.." && pwd)}"
  CANDIDATE="$(find "$HADOOP_HOME_DIR" -type f -name 'hadoop-streaming-*.jar' 2>/dev/null | head -n1 || true)"
  if [[ -n "$CANDIDATE" ]]; then
    HADOOP_STREAMING_JAR="$CANDIDATE"
  fi
fi
if [[ -z "$HADOOP_STREAMING_JAR" || ! -f "$HADOOP_STREAMING_JAR" ]]; then
  echo "cannot find hadoop streaming jar, set HADOOP_STREAMING_JAR explicitly" >&2
  exit 1
fi

echo "generating input: rows=$ROWS key_mod=$KEY_MOD -> $INPUT"
awk -v rows="$ROWS" -v mod="$KEY_MOD" 'BEGIN{for(i=1;i<=rows;i++) printf "%d\tk%06d\t1\n", i, i%mod}' >"$INPUT"

echo "building mrkit count plugin"
GOCACHE="$ROOT/.cache/go-build" \
GOMODCACHE="$ROOT/.cache/go-mod" \
GOTMPDIR="$ROOT/.cache/go-tmp" \
"$GO_BIN" build -buildmode=plugin -o "$PLUGIN_DIR/count.so" "$ROOT/mrapps/count.go"

MAPPER="$OUTDIR/mapper_count.sh"
REDUCER="$OUTDIR/reducer_count.sh"
cat >"$MAPPER" <<'EOF'
#!/usr/bin/env bash
awk -F '\t' 'NF>=2 {print $2 "\t1"}'
EOF
cat >"$REDUCER" <<'EOF'
#!/usr/bin/env bash
awk -F '\t' '
{
  if (NR==1) {k=$1; s=$2+0; next}
  if ($1==k) {s+=$2+0; next}
  print k "\t" s
  k=$1; s=$2+0
}
END{
  if (NR>0) print k "\t" s
}'
EOF
chmod +x "$MAPPER" "$REDUCER"

echo "reducers,round,tool,rows,wall_ms,input_rps,output_rows,status" >"$CSV"

for round in $(seq 1 "$ROUNDS"); do
  rm -f "$ROOT"/mr-out-*.txt "$ROOT"/output/imd-*.txt
  mr_log="$OUTDIR/mrkit_round${round}.log"
  start_ms="$(ms_now)"
  if GOCACHE="$ROOT/.cache/go-build" \
    GOMODCACHE="$ROOT/.cache/go-mod" \
    GOTMPDIR="$ROOT/.cache/go-tmp" \
    "$GO_BIN" run "$ROOT/cmd/legacy/main/main.go" \
      -i "$INPUT" \
      -p "$PLUGIN_DIR/count.so" \
      -r "$REDUCERS" \
      -w "$REDUCERS" \
      --port "$((MR_PORT_BASE + round))" \
      -m=false >"$mr_log" 2>&1; then
    end_ms="$(ms_now)"
    wall_ms="$((end_ms - start_ms))"
    out_rows="$(cat "$ROOT"/mr-out-*.txt 2>/dev/null | wc -l | tr -d ' ')"
    rps="$(awk -v n="$ROWS" -v ms="$wall_ms" 'BEGIN{ if (ms<=0) print 0; else printf "%.2f", (n*1000.0/ms) }')"
    echo "$REDUCERS,$round,mrkit,$ROWS,$wall_ms,$rps,$out_rows,PASS" >>"$CSV"
  else
    echo "$REDUCERS,$round,mrkit,$ROWS,0,0,0,FAIL" >>"$CSV"
  fi

  hadoop_log="$OUTDIR/hadoop_round${round}.log"
  rm -rf "$OUTDIR/out-hadoop"
  start_ms="$(ms_now)"
  if hadoop jar "$HADOOP_STREAMING_JAR" \
    -D mapreduce.framework.name=local \
    -D mapreduce.job.reduces="$REDUCERS" \
    -input "$INPUT" \
    -output "$OUTDIR/out-hadoop" \
    -mapper "bash \"$MAPPER\"" \
    -reducer "bash \"$REDUCER\"" >"$hadoop_log" 2>&1; then
    end_ms="$(ms_now)"
    wall_ms="$((end_ms - start_ms))"
    out_rows="$(cat "$OUTDIR"/out-hadoop/part-* 2>/dev/null | wc -l | tr -d ' ')"
    rps="$(awk -v n="$ROWS" -v ms="$wall_ms" 'BEGIN{ if (ms<=0) print 0; else printf "%.2f", (n*1000.0/ms) }')"
    echo "$REDUCERS,$round,hadoop_streaming_host,$ROWS,$wall_ms,$rps,$out_rows,PASS" >>"$CSV"
  else
    echo "$REDUCERS,$round,hadoop_streaming_host,$ROWS,0,0,0,FAIL" >>"$CSV"
  fi
done

echo "RESULT_CSV=$CSV"
