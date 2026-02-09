# mrkit-go

[<img alt="github" src="https://img.shields.io/badge/github-emptyOVO%2Fmrkit--go-blue?style=for-the-badge&logo=appveyor" height="20">](https://github.com/emptyOVO/mrkit-go)
[<img alt="build status" src="https://img.shields.io/github/actions/workflow/status/emptyOVO/mrkit-go/go.yml?style=for-the-badge" height="20">](https://github.com/emptyOVO/mrkit-go/actions/workflows/go.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/emptyOVO/mrkit-go.svg)](https://pkg.go.dev/github.com/emptyOVO/mrkit-go)

`mrkit-go` is a MapReduce toolkit in Go with a config-driven batch runner.

Primary path for new users:
- define `source / transform / sink` in JSON
- run `./cmd/batch` with that config
- support MySQL/Redis source and sink combinations

## Quickstart (Config-Driven)

### 1) Prepare demo data (optional)

```bash
MYSQL_HOST=localhost MYSQL_PORT=3306 MYSQL_USER=root MYSQL_PASSWORD=123456 \
MYSQL_DB=mysql SOURCE_TABLE=source_events TARGET_TABLE=agg_results \
ROWS=5000 KEY_MOD=100 \
go run ./cmd/batch -mode prepare
```

### 2) Validate a config

```bash
go run ./cmd/batch -check -config example/batch-minimal/flows/smoke/flow.mysql.count.json
```

### 3) Run a flow

```bash
go run ./cmd/batch -config example/batch-minimal/flows/smoke/flow.mysql.count.json
```

For cross-DB, seed, benchmark, and plugin scenarios, use the docs below.

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

## Documentation Map

- Config-driven schema, production template, and rerun guidance: [`docs/config-driven-flow.md`](docs/config-driven-flow.md)
- Built-in transforms and plugin mode: [`docs/transforms.md`](docs/transforms.md)
- Go library usage (`batch.RunPipeline`): [`docs/library-usage.md`](docs/library-usage.md)
- Benchmark usage: [`docs/benchmark.md`](docs/benchmark.md)
- Legacy MapReduce entrypoints (`cmd/legacy/main/main.go`, `cmd/legacy/master/main.go`, `cmd/legacy/worker/main.go`): [`docs/legacy-mapreduce.md`](docs/legacy-mapreduce.md)
- Minimal end-to-end examples: [`example/batch-minimal/README.md`](example/batch-minimal/README.md)

## Contributions

Pull requests are always welcome.

<sup>
Created and improved by Yi-fei Gao. All code is
licensed under the <a href="LICENSE">Apache License 2.0</a>.
</sup>
