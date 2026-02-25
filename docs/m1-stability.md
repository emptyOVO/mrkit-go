# M1 Stability Runbook

This runbook executes repeated config-driven E2E flows and writes both machine-readable and human-readable reports.

## What It Covers

- repeated runs of 5 scenarios per round:
  - `seed` (mysql->redis source seeding)
  - `m2m` (mysql->mysql)
  - `m2r` (mysql->redis)
  - `r2m` (redis->mysql)
  - `r2r` (redis->redis)
- per-case status + duration in NDJSON
- final result validation against `KEY_MOD`
- optional legacy worker chaos (randomly kill one worker process)

## Command

```bash
chmod +x scripts/m1_stability.sh
ROUNDS=30 KEY_MOD=100 ROWS=5000 ./scripts/m1_stability.sh
```

Useful flags:

```bash
# optional legacy worker-chaos subtest
ENABLE_LEGACY_WORKER_CHAOS=1 CHAOS_ROUNDS=10 ./scripts/m1_stability.sh

# keep mysql/redis containers for inspection
KEEP_CONTAINERS=1 ./scripts/m1_stability.sh

# custom output root
REPORT_ROOT=reports/m1 ./scripts/m1_stability.sh
```

## Outputs

Per run, reports are generated under:

- `reports/m1/<run_id>/details.ndjson`
- `reports/m1/<run_id>/summary.json`
- `reports/m1/<run_id>/summary.txt`
- `reports/m1/<run_id>/final_counts.json`

## Suggested M1 Gate

- pass rate >= 95% across all cases
- no repeated failure on a fixed scenario
- final counts match `KEY_MOD` for all sink targets

Manual gate command:

```bash
# use run_id printed by scripts/m1_stability.sh
MIN_PASS_RATE=95 ./scripts/m1_gate.sh <run_id>
```

Release preflight integration:

- tag release flow (`v*`) now runs the same gate automatically in CI before Docker release image publish.
- current policy is non-required branch check, but every release run includes explicit gate result logs.

## Report Visualization (CSV + SVG)

```bash
# generate CSV and lightweight SVG charts from one report
./scripts/m1_visualize.py <run_id>
```

Outputs:

- `reports/m1/<run_id>/viz/scenario_summary.csv`
- `reports/m1/<run_id>/viz/details.csv`
- `reports/m1/<run_id>/viz/scenario_pass_rate.svg`
- `reports/m1/<run_id>/viz/scenario_avg_ms.svg`
- `reports/m1/<run_id>/viz/visualization.md`

## Notes

- master-kill failover is not part of this runbook yet.
- worker chaos currently uses `cmd/legacy/master` + `cmd/legacy/worker` as an optional subtest, because config-driven flow runs in-process legacy runtime.
