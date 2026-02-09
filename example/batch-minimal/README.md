# batch-minimal

A copy-paste-ready minimal integration example for `github.com/emptyOVO/mrkit-go/batch`.

This example is now organized by **use case** under `flows/`:
- `smoke/`
- `cross-db/`
- `seed/`
- `benchmark/`

## 1) Files

- `go.mod`
- `go.sum`
- `main.go`
- `flows/**.json`

For local development in this repository, `go.mod` uses:

```go
replace github.com/emptyOVO/mrkit-go => ../..
```

## 2) Flow Index

| Use Case | Config Path | Source -> Sink | Notes |
|---|---|---|---|
| Smoke | `example/batch-minimal/flows/smoke/flow.mysql.json` | MySQL -> MySQL | plugin transform (`mapreduce`) |
| Smoke | `example/batch-minimal/flows/smoke/flow.mysql.count.json` | MySQL -> MySQL | builtin `count` |
| Smoke | `example/batch-minimal/flows/smoke/flow.mysql.minmax.json` | MySQL -> MySQL | builtin `minmax` |
| Smoke | `example/batch-minimal/flows/smoke/flow.mysql.topn.json` | MySQL -> MySQL | builtin `topN` |
| Cross-DB | `example/batch-minimal/flows/cross-db/flow.mysql_to_redis.count.json` | MySQL -> Redis | builtin `count` |
| Seed | `example/batch-minimal/flows/seed/flow.seed.redis_source_event.json` | MySQL -> Redis(db0/event:*) | prepare Redis source for `redis -> *` |
| Cross-DB | `example/batch-minimal/flows/cross-db/flow.redis_to_mysql.count.json` | Redis -> MySQL | reads `db0/event:*` |
| Cross-DB | `example/batch-minimal/flows/cross-db/flow.redis_to_redis.count.json` | Redis -> Redis | reads `db0/event:*`, writes db2 |
| Benchmark | `example/batch-minimal/flows/benchmark/flow.benchmark.mysql_to_redis.count.json` | MySQL -> Redis | benchmark-oriented path |
| Benchmark | `example/batch-minimal/flows/benchmark/flow.benchmark.redis_to_redis.count.json` | Redis -> Redis | benchmark-oriented path |

## 3) Quick Run

Recommended builtin check:

```bash
go run ./cmd/batch -check -config example/batch-minimal/flows/smoke/flow.mysql.count.json
```

Optional plugin mode:

```bash
go build -buildmode=plugin -o cmd/agg.so ./mrapps/agg.go
go run ./cmd/batch -check -config example/batch-minimal/flows/smoke/flow.mysql.json
```

Optional synthetic data prepare:

```bash
ROWS=20000 MYSQL_SOURCE_DB=mysql SOURCE_TABLE=source_events \
go run ./cmd/batch -mode prepare
```

Run core paths:

```bash
go run ./cmd/batch -config example/batch-minimal/flows/smoke/flow.mysql.json

go run ./cmd/batch -config example/batch-minimal/flows/cross-db/flow.mysql_to_redis.count.json
go run ./cmd/batch -config example/batch-minimal/flows/seed/flow.seed.redis_source_event.json
go run ./cmd/batch -config example/batch-minimal/flows/cross-db/flow.redis_to_mysql.count.json
go run ./cmd/batch -config example/batch-minimal/flows/cross-db/flow.redis_to_redis.count.json

go run ./cmd/batch -mode benchmark -config example/batch-minimal/flows/benchmark/flow.benchmark.mysql_to_redis.count.json
go run ./cmd/batch -mode benchmark -config example/batch-minimal/flows/benchmark/flow.benchmark.redis_to_redis.count.json
```

## 4) Validate Result

For MySQL sink flows:

```sql
SELECT COUNT(*) AS rows_written FROM mr_target.agg_results;
SELECT * FROM mr_target.agg_results ORDER BY 1 LIMIT 10;
```

For Redis sink flows, verify config is valid:

```bash
go run ./cmd/batch -check -config example/batch-minimal/flows/cross-db/flow.mysql_to_redis.count.json
go run ./cmd/batch -check -config example/batch-minimal/flows/cross-db/flow.redis_to_redis.count.json
```

## Troubleshooting

- `dial tcp localhost:3306`: MySQL not reachable or credentials incorrect.
- `plugin.Open ... different version`: rebuild plugin with the same Go environment as runtime.
- `no reduce output files matched`: pipeline did not produce `mr-out-*.txt` (check plugin path/runtime logs).

## Repro Checklist

For a full end-to-end reproducible checklist (local and Docker), see:

- `docs/repro-checklist.md`
