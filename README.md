# mrkit-go

[<img alt="github" src="https://img.shields.io/badge/github-emptyOVO%2Fmrkit--go-blue?style=for-the-badge&logo=appveyor" height="20">](https://github.com/emptyOVO/mrkit-go)
[<img alt="build status" src="https://img.shields.io/github/actions/workflow/status/emptyOVO/mrkit-go/go.yml?branch=dev&style=for-the-badge" height="20">](https://github.com/emptyOVO/mrkit-go/actions/workflows/go.yml?query=branch%3Adev)
[![Go Reference](https://pkg.go.dev/badge/github.com/emptyOVO/mrkit-go.svg)](https://pkg.go.dev/github.com/emptyOVO/mrkit-go)

`mrkit-go` is a lightweight MapReduce framework implemented in Go, ready to use out of the box, with config-driven batch flows for common big-data databases.

Compared with Hadoop-style MapReduce stacks, `mrkit-go` emphasizes simpler deployment, lower operational overhead on Go service hosts, and faster iteration for small-to-medium batch pipelines. The project is still actively evolving.

Primary path for new users:
- define `source / transform / sink` in JSON
- run `./cmd/batch` with that config
- support MySQL/Redis source and sink combinations

## Quickstart (One Command, Recommended)

This path deploys MySQL + Redis automatically (Docker), creates required DBs, prepares synthetic data, runs all core flows (`seed/m2m/m2r/r2m/r2r`), and validates results.

Environment Check (one line):

```bash
command -v go docker jq bash awk python3 >/dev/null && echo "env ok" || echo "missing deps"
```

```bash
chmod +x scripts/quickstart.sh
./scripts/quickstart.sh
```

Expected final output includes:
- `all checks passed`
- MySQL tables row counts and Redis key counts equal to `KEY_MOD` (default `100`)

Default quickstart service ports:
- MySQL: `127.0.0.1:13306`
- Redis: `127.0.0.1:16379`

Useful overrides:

```bash
KEY_MOD=200 ROWS=10000 ./scripts/quickstart.sh
MYSQL_PORT=23306 REDIS_PORT=26379 ./scripts/quickstart.sh
GO_BIN=/path/to/go ./scripts/quickstart.sh
```

Stop quickstart services:

```bash
docker rm -f mrkit-quickstart-mysql mrkit-quickstart-redis
```

## Quickstart (Manual, Config-Driven)

If you already have MySQL/Redis and want to run commands manually, follow:
- [`docs/repro-checklist.md`](docs/repro-checklist.md)
- [`docs/config-driven-flow.md`](docs/config-driven-flow.md)
## Quickstart (Docker)

Build local image:

```bash
docker build -t mrkit-go-batch:local .
```

Validate config in container:

```bash
docker run --rm \
  -v "$(pwd)/example/batch-minimal/flows:/app/flows:ro" \
  mrkit-go-batch:local \
  -check -config /app/flows/smoke/flow.mysql.count.json
```

Run flow in container (use `host.docker.internal` for local DB access):

```bash
docker run --rm \
  -v "$(pwd)/example/batch-minimal/flows:/app/flows:ro" \
  -e MYSQL_HOST=host.docker.internal \
  -e MYSQL_PORT=3306 \
  -e MYSQL_USER=root \
  -e MYSQL_PASSWORD=123456 \
  -e MYSQL_DB=mysql \
  mrkit-go-batch:local \
  -config /app/flows/smoke/flow.mysql.count.json
```

Performance note: use `go run` for development checks, and prebuilt binaries (`go build` then run) for production/performance benchmarking.

## Documentation Map

- Config-driven schema, production template, and rerun guidance: [`docs/config-driven-flow.md`](docs/config-driven-flow.md)
- Full reproducible test checklist (local + Docker + seed + 4 cross-DB paths): [`docs/repro-checklist.md`](docs/repro-checklist.md)
- Built-in transforms and plugin mode: [`docs/transforms.md`](docs/transforms.md)
- Go library usage (`batch.RunPipeline`): [`docs/library-usage.md`](docs/library-usage.md)
- Benchmark usage: [`docs/benchmark.md`](docs/benchmark.md)
- Performance analysis (mrkit-go vs local Hadoop Streaming): [`docs/performance-analysis.md`](docs/performance-analysis.md)
- Legacy MapReduce entrypoints (`cmd/legacy/main/main.go`, `cmd/legacy/master/main.go`, `cmd/legacy/worker/main.go`): [`docs/legacy-mapreduce.md`](docs/legacy-mapreduce.md)
- Minimal end-to-end examples: [`example/batch-minimal/README.md`](example/batch-minimal/README.md)

## Contributions

Pull requests are always welcome.

<sup>
Created and improved by Yi-fei Gao. All code is
licensed under the <a href="LICENSE">Apache License 2.0</a>.
</sup>
