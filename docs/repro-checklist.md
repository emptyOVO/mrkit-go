# Reproducible Test Checklist

This page provides copy-paste commands to reproduce the main verification paths.

## Prerequisites

- Go toolchain installed (`go version`)
- Docker installed and daemon running (`docker version`)
- MySQL available at `localhost:3306` (`root` / `123456`)
- Redis available at `localhost:6379`

## A) Local E2E (Config-Driven)

### 1) Prepare MySQL demo data

```bash
MYSQL_HOST=localhost MYSQL_PORT=3306 MYSQL_USER=root MYSQL_PASSWORD=123456 \
MYSQL_DB=mysql SOURCE_TABLE=source_events TARGET_TABLE=agg_results \
ROWS=5000 KEY_MOD=100 \
go run ./cmd/batch -mode prepare
```

### 2) Seed Redis source keys (`db0/event:*`) for `redis -> *`

```bash
go run ./cmd/batch -check -config example/batch-minimal/flows/seed/flow.seed.redis_source_event.json
go run ./cmd/batch -config example/batch-minimal/flows/seed/flow.seed.redis_source_event.json
```

### 3) Four cross-DB paths (check + run)

```bash
# mysql -> mysql
go run ./cmd/batch -check -config example/batch-minimal/flows/smoke/flow.mysql.count.json
go run ./cmd/batch -config example/batch-minimal/flows/smoke/flow.mysql.count.json

# mysql -> redis
go run ./cmd/batch -check -config example/batch-minimal/flows/cross-db/flow.mysql_to_redis.count.json
go run ./cmd/batch -config example/batch-minimal/flows/cross-db/flow.mysql_to_redis.count.json

# redis -> mysql
go run ./cmd/batch -check -config example/batch-minimal/flows/cross-db/flow.redis_to_mysql.count.json
go run ./cmd/batch -config example/batch-minimal/flows/cross-db/flow.redis_to_mysql.count.json

# redis -> redis
go run ./cmd/batch -check -config example/batch-minimal/flows/cross-db/flow.redis_to_redis.count.json
go run ./cmd/batch -config example/batch-minimal/flows/cross-db/flow.redis_to_redis.count.json
```

## B) Docker Reproduction

### 1) Build image

```bash
docker build -t mrkit-go-batch:local .
```

### 2) Legacy checks in container

```bash
# legacy single-machine
docker run --rm --entrypoint /bin/bash \
  -v "$(pwd)":/app -w /app mrkit-go-batch:local \
  -lc 'set -euo pipefail; "$GO" build -buildmode=plugin -o cmd/wc.so ./mrapps/wc.go; rm -f -- mr-out-*.txt output/imd-*.txt; "$GO" run ./cmd/legacy/main/main.go -i "txt/*" -p "cmd/wc.so" -r 1 -w 4 --port 21100 -m=false; test -f mr-out-0.txt'

# legacy master+worker
docker run --rm --entrypoint /bin/bash \
  -v "$(pwd)":/app -w /app mrkit-go-batch:local \
  -lc 'set -euo pipefail; "$GO" build -buildmode=plugin -o cmd/wc.so ./mrapps/wc.go; rm -f -- mr-out-*.txt output/imd-*.txt /tmp/m.log /tmp/w1.log /tmp/w2.log; "$GO" run ./cmd/legacy/master/main.go -i "txt/*" -p "cmd/wc.so" -r 1 -w 2 --port 21110 -m=false >/tmp/m.log 2>&1 & MP=$!; sleep 1; "$GO" run ./cmd/legacy/worker/main.go -i "txt/*" -p "cmd/wc.so" -r 1 -w 1 --port 21110 -m=false >/tmp/w1.log 2>&1 & W1=$!; "$GO" run ./cmd/legacy/worker/main.go -i "txt/*" -p "cmd/wc.so" -r 1 -w 2 --port 21110 -m=false >/tmp/w2.log 2>&1 & W2=$!; wait $MP; wait $W1; wait $W2; test -f mr-out-0.txt'
```

### 3) Docker E2E (seed + 4 cross-DB paths)

```bash
mkdir -p /tmp/mrkit-docker-flows

jq 'if .source.type=="mysql" then .source.db.host="host.docker.internal" else . end | if .source.type=="redis" then .source.redis.host="host.docker.internal" else . end | if .sink.type=="mysql" then .sink.db.host="host.docker.internal" else . end | if .sink.type=="redis" then .sink.redis.host="host.docker.internal" else . end | .transform.port=22110' example/batch-minimal/flows/seed/flow.seed.redis_source_event.json > /tmp/mrkit-docker-flows/seed.json
jq 'if .source.type=="mysql" then .source.db.host="host.docker.internal" else . end | if .source.type=="redis" then .source.redis.host="host.docker.internal" else . end | if .sink.type=="mysql" then .sink.db.host="host.docker.internal" else . end | if .sink.type=="redis" then .sink.redis.host="host.docker.internal" else . end | .transform.port=22111' example/batch-minimal/flows/smoke/flow.mysql.count.json > /tmp/mrkit-docker-flows/m2m.json
jq 'if .source.type=="mysql" then .source.db.host="host.docker.internal" else . end | if .source.type=="redis" then .source.redis.host="host.docker.internal" else . end | if .sink.type=="mysql" then .sink.db.host="host.docker.internal" else . end | if .sink.type=="redis" then .sink.redis.host="host.docker.internal" else . end | .transform.port=22112' example/batch-minimal/flows/cross-db/flow.mysql_to_redis.count.json > /tmp/mrkit-docker-flows/m2r.json
jq 'if .source.type=="mysql" then .source.db.host="host.docker.internal" else . end | if .source.type=="redis" then .source.redis.host="host.docker.internal" else . end | if .sink.type=="mysql" then .sink.db.host="host.docker.internal" else . end | if .sink.type=="redis" then .sink.redis.host="host.docker.internal" else . end | .transform.port=22113' example/batch-minimal/flows/cross-db/flow.redis_to_mysql.count.json > /tmp/mrkit-docker-flows/r2m.json
jq 'if .source.type=="mysql" then .source.db.host="host.docker.internal" else . end | if .source.type=="redis" then .source.redis.host="host.docker.internal" else . end | if .sink.type=="mysql" then .sink.db.host="host.docker.internal" else . end | if .sink.type=="redis" then .sink.redis.host="host.docker.internal" else . end | .transform.port=22114' example/batch-minimal/flows/cross-db/flow.redis_to_redis.count.json > /tmp/mrkit-docker-flows/r2r.json

for name in seed m2m m2r r2m r2r; do
  docker run --rm -v /tmp/mrkit-docker-flows:/flows mrkit-go-batch:local -check -config "/flows/${name}.json"
  docker run --rm -v /tmp/mrkit-docker-flows:/flows mrkit-go-batch:local -config "/flows/${name}.json"
done
```

## Cleanup

```bash
rm -f -- mr-out-*.txt output/imd-*.txt txt/mysql_source/chunk-*.txt txt/redis_source/chunk-*.txt cmd/*.so
```
