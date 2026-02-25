# Multi-Node Demo (M2 Minimal)

This page provides a lightweight multi-node demo path without heavy orchestration.

## Topology (3 machines)

- Machine A: master
- Machine B: worker-1
- Machine C: worker-2

Example addresses:

- master: `10.0.0.11:11340`
- worker-1: `10.0.0.12`
- worker-2: `10.0.0.13`

## A) Binary-first deployment (recommended)

This path supports: copy binaries + edit env + one command start.

### 1) Prepare binaries and plugin

```bash
# on each machine
GO_BIN=/path/to/go
mkdir -p bin cmd
$GO_BIN build -o bin/legacy-master ./cmd/legacy/master
$GO_BIN build -o bin/legacy-worker ./cmd/legacy/worker
$GO_BIN build -buildmode=plugin -o cmd/wc.so ./mrapps/wc
```

### 2) Copy env template

```bash
cp deploy/env.example deploy/.env
```

Edit key fields in `deploy/.env`:

- `RUN_MODE=bin`
- `MASTER_ADDR=10.0.0.11:11340`
- `MASTER_PORT=11340`
- `ADVERTISE_HOST=<worker-node-ip-or-hostname>` (set per worker node)
- `WORKERS=3`
- `REDUCERS=1`

### 3) Start master (Machine A)

```bash
ENV_FILE=deploy/.env ./deploy/start_master.sh
```

### 4) Start workers (Machine B/C)

```bash
# Machine B
ENV_FILE=deploy/.env WORKER_ID=1 ADVERTISE_HOST=10.0.0.12 ./deploy/start_worker.sh

# Machine C
ENV_FILE=deploy/.env WORKER_ID=2 ADVERTISE_HOST=10.0.0.13 ./deploy/start_worker.sh
```

### 5) Stop all

```bash
ENV_FILE=deploy/.env ./deploy/stop_all.sh
```

## B) Docker Compose local multi-container demo

Use this mode for local presentation where one machine simulates multiple nodes.

```bash
docker compose -f deploy/docker-compose.multi-node.yml up -d
docker compose -f deploy/docker-compose.multi-node.yml logs -f master
docker compose -f deploy/docker-compose.multi-node.yml down -v
```

## C) SSH fanout real multi-machine demo

Use one control machine to batch start/stop workers on remote hosts.

### 1) Configure deploy/.env

Required fields:

- `MASTER_ADDR=10.0.0.11:11340`
- `WORKER_HOSTS=10.0.0.12,10.0.0.13`
- `SSH_USER=ubuntu`
- `SSH_KEY=~/.ssh/id_rsa`
- `REMOTE_ROOT=~/mrkit-go`

### 2) Start workers in batch

```bash
ENV_FILE=deploy/.env ./deploy/ssh_fanout.sh start
```

### 3) Stop workers in batch

```bash
ENV_FILE=deploy/.env ./deploy/ssh_fanout.sh stop
```

## Notes

- For config-driven flow E2E and M1 stability checks, continue using:
  - `scripts/quickstart.sh`
  - `scripts/m1_stability.sh`
- Master-kill failover is not included in current M2.

## Failure and Troubleshooting

- Port conflict (`address already in use`):
  - change `MASTER_PORT` or worker start port range
  - run `lsof -iTCP -sTCP:LISTEN | grep -E "11340|1000[0-9]"` to find occupied ports

- Plugin ELF mismatch (`plugin was built with a different version` / `invalid ELF`):
  - rebuild plugin on the same target environment as runtime binary
  - in container demos, build `.so` inside the container image or set `FORCE_REBUILD_PLUGIN=1`
  - for local demos, `FORCE_REBUILD_PLUGIN=1` on both `start_master.sh` and `start_worker.sh` is the safest default

- Wrong worker advertised address (worker registered but unreachable):
  - set `ADVERTISE_HOST` to an address routable from master (not `127.0.0.1` across machines)
  - verify master can dial `ADVERTISE_HOST:<worker-port>` from network path

- `go run` cache permission issue (`open .../go-build/... operation not permitted`):
  - prefer `RUN_MODE=bin` for demos
  - or provide writable cache env vars when starting scripts:
    - `GOCACHE=/tmp/mrkit-go-cache GOMODCACHE=/tmp/mrkit-go-modcache ENV_FILE=deploy/.env ./deploy/start_master.sh`
    - `GOCACHE=/tmp/mrkit-go-cache GOMODCACHE=/tmp/mrkit-go-modcache ENV_FILE=deploy/.env WORKER_ID=1 ./deploy/start_worker.sh`
