# Master Failover Design Draft (M3.5)

Status: design only, not implemented.

## Scope and Goal

- Current runtime handles worker retry/reassignment, but not master process failover.
- Goal for M3.5: allow master restart or standby takeover without losing in-flight job progress.

## Boundaries

In scope:

- master state persistence
- startup recovery
- active/standby role handover

Out of scope (this phase):

- cross-region replication
- exactly-once sink semantics across external DB failures
- arbitrary long-running speculative execution

## Proposed Architecture

1. Persistent State Store

- Persist scheduler states (`pending/running/success/failed`, attempts, last error)
- Persist stage boundaries and IMD metadata index
- Persist worker lease snapshots with heartbeat timestamp

2. Recovery Flow

- On master start, detect unfinished job session
- Rebuild in-memory scheduler from persisted snapshot
- Mark stale running tasks as pending if worker lease expired
- Continue map/reduce from recovered stage cursor

3. Active/Standby Handover

- One active master, one standby
- Lease/lock record in persistent store elects active owner
- Standby only serves read-only health/metrics until lock ownership changes
- On active loss, standby acquires lock and runs recovery flow

## Data Model (minimal)

- `job_session`: job_id, stage, status, updated_at
- `task_state`: task_id, stage, state, worker_id, attempt, updated_at
- `worker_lease`: worker_id, addr, state, lease_expire
- `imd_index`: reduce_partition, imd_location, checksum

## Failure Cases to Cover

- master crash during map stage assignment
- master crash between reduce retries
- standby takeover when some workers are still alive
- stale lock owner due to split-brain risk

## Consistency Contract

- At-least-once task execution during failover window
- Idempotent sink writes strongly recommended
- No silent data loss: every unfinished task must be either resumed or explicitly failed

## Rollout Plan

1. implement persistent snapshot writer/loader (single active master only)
2. add restart recovery tests
3. add active/standby lock and takeover
4. add failover chaos tests and SLO-based gate
