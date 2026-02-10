# Performance Analysis (mrkit-go vs Hadoop Streaming)

This page summarizes host-vs-host measurements collected on the same machine, for `COUNT` workload (`100,000` rows, `10,000` keys, `3` rounds, median used).

## Scope

- Compared tools:
  - `mrkit` (legacy runner path)
  - `mrkit_bin` (optimized binary-path run)
  - `hadoop_streaming_host` (local Hadoop Streaming)
- Data source files (local machine):
  - `/tmp/hadoop-host-bench/final_matrix_after_fix_20260210_191545.csv`
  - `/tmp/hadoop-host-bench/final_matrix_bin_after_fix_20260210_192818.csv`
- Metrics:
  - `wall_ms`: end-to-end elapsed time
  - `input_rps`: processed rows / second

## Median Matrix

### A) Runner path (`mrkit`) vs Hadoop Streaming

| Reducers | mrkit wall_ms | mrkit rps | hadoop wall_ms | hadoop rps |
|---|---:|---:|---:|---:|
| 1 | 8523 | 11,732.96 | 2093 | 47,778.31 |
| 4 | 2637 | 37,921.88 | 1911 | 52,328.62 |
| 8 | 2598 | 38,491.15 | 1889 | 52,938.06 |

Observation:
- In legacy runner mode, Hadoop Streaming is faster in this benchmark setup.

### B) Optimized binary path (`mrkit_bin`) vs Hadoop Streaming

| Reducers | mrkit_bin wall_ms | mrkit_bin rps | hadoop wall_ms | hadoop rps |
|---|---:|---:|---:|---:|
| 1 | 550 | 181,818.18 | 1909 | 52,383.45 |
| 4 | 562 | 177,935.94 | 1892 | 52,854.12 |
| 8 | 1576 | 63,451.78 | 1863 | 53,676.87 |

Observation:
- With optimized binary-path execution, `mrkit_bin` outperforms Hadoop Streaming in reducers `1/4`, and remains competitive at `8`.
- Reducers `8` is sensitive to local scheduling and runtime overhead on this host.

## Why Results Can Differ

- Runner path includes extra framework overhead (startup, RPC scheduling, plugin/runtime orchestration).
- Hadoop Streaming has mature local sort/shuffle pipeline even in local mode.
- Reducer count increase does not always improve throughput; local CPU, disk, and context switching become bottlenecks.

## Repro Command Reference

Use your local scripts (already used in this repo workflow):

```bash
# runner-path matrix
ROWS=100000 REDUCERS=1 ROUNDS=3 /tmp/run_host_hadoop_vs_mrkit_count.sh
ROWS=100000 REDUCERS=4 ROUNDS=3 /tmp/run_host_hadoop_vs_mrkit_count.sh
ROWS=100000 REDUCERS=8 ROUNDS=3 /tmp/run_host_hadoop_vs_mrkit_count.sh

# binary-path matrix
ROWS=100000 REDUCERS=1 ROUNDS=3 /tmp/run_host_hadoop_vs_mrkit_count_bin.sh
ROWS=100000 REDUCERS=4 ROUNDS=3 /tmp/run_host_hadoop_vs_mrkit_count_bin.sh
ROWS=100000 REDUCERS=8 ROUNDS=3 /tmp/run_host_hadoop_vs_mrkit_count_bin.sh
```

If you want stronger confidence, repeat with larger scales and add CPU/memory sampling.
