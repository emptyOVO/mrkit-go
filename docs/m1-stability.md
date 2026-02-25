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

## Notes

- master-kill failover is not part of this runbook yet.
- worker chaos currently uses `cmd/legacy/master` + `cmd/legacy/worker` as an optional subtest, because config-driven flow runs in-process legacy runtime.
