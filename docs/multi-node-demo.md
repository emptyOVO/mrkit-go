# Multi-Node Demo (Minimal)

This is a lightweight 2~3 node demo path using existing legacy master/worker entrypoints.

## Prerequisites

- same repo content on each machine
- Go toolchain available (`GO_BIN` can be set)
- network connectivity from workers to master port

## 1) Build plugin once (on each machine or shared path)

```bash
GO_BIN=/path/to/go
$GO_BIN build -buildmode=plugin -o cmd/wc.so ./mrapps/wc
```

## 2) Start master (machine A)

```bash
MASTER_PORT=11340 WORKERS=3 REDUCERS=1 GO_BIN=/path/to/go ./deploy/start_master.sh
```

## 3) Start workers (machine B/C/...)

```bash
MASTER_PORT=11340 WORKER_ID=1 REDUCERS=1 GO_BIN=/path/to/go ./deploy/start_worker.sh
MASTER_PORT=11340 WORKER_ID=2 REDUCERS=1 GO_BIN=/path/to/go ./deploy/start_worker.sh
MASTER_PORT=11340 WORKER_ID=3 REDUCERS=1 GO_BIN=/path/to/go ./deploy/start_worker.sh
```

## 4) Inspect logs/output

```bash
ls -la .run/multi-node
ls -la mr-out-*.txt
```

## 5) Stop all

```bash
./deploy/stop_all.sh
```

## Notes

- This path is for quick demo and manual verification.
- It intentionally avoids heavy orchestration dependencies.
- For config-driven batch flows and M1 stability, use `scripts/quickstart.sh` and `scripts/m1_stability.sh`.
