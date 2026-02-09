# batch-minimal

A copy-paste-ready minimal integration example for using `github.com/emptyOVO/mrkit-go/batch` in an external project.
This version demonstrates **cross-database** pipeline and **config-driven run**:
- read from source DB/table
- run MapReduce
- write aggregated result to target DB/table

## 1) Files

This example includes:

- `go.mod`
- `main.go`
- `mysqlbatch/flow.mysql.json`
- `mysqlbatch/flow.mysql.count.json`
- `mysqlbatch/flow.mysql.minmax.json`
- `mysqlbatch/flow.mysql.topn.json`
- `mysqlbatch/flow.mysql_to_redis.count.json`
- `redisbatch/flow.redis_to_mysql.count.json`
- `redisbatch/flow.redis_to_redis.count.json`

For local development in this repository, `go.mod` uses:

```go
replace github.com/emptyOVO/mrkit-go => ../..
```

If you copy this example into another repository, replace that line with your own module source (for example a git tag/version).

## 2) Choose transform mode

Recommended: built-in transform (`count` / `minmax` / `topN`) with no plugin build.

```bash
cd /path/to/mrkit-go
go run ./cmd/batch -check -config example/batch-minimal/mysqlbatch/flow.mysql.count.json
```

If you need custom aggregation logic, use plugin mode:

```bash
cd /path/to/mrkit-go
go build -buildmode=plugin -o cmd/agg.so ./mrapps/agg.go
go run ./cmd/batch -check -config example/batch-minimal/mysqlbatch/flow.mysql.json
```

## 3) Prepare flow config

Use `example/batch-minimal/mysqlbatch/flow.mysql.json` as the template.
You mainly need to adjust:

- `source.db` / `sink.db`
- `source.config.table`
- `sink.config.targettable`
- `version` (keep `v1`)
- `transform` (`type: builtin` + `builtin: count|minmax|topN`, or plugin mode)

## 4) (Optional) Prepare demo source data

```bash
cd /path/to/mrkit-go
ROWS=20000 MYSQL_SOURCE_DB=mysql SOURCE_TABLE=source_events \
go run ./cmd/batch -mode prepare
```

Before running pipeline, ensure target database in `mysqlbatch/flow.mysql.json` exists (for example: `mr_target`).

## 5) Run by config file (recommended)

```bash
go run ./cmd/batch -config example/batch-minimal/mysqlbatch/flow.mysql.json

# mysql -> redis
go run ./cmd/batch -config example/batch-minimal/mysqlbatch/flow.mysql_to_redis.count.json

# prepare redis source keys for redis->* examples (db0/event:*)
sed "s/\"db\": 1/\"db\": 0/; s/\"key_prefix\": \"mr:count:\"/\"key_prefix\": \"event:\"/; s/\"value_field\": \"metric_sum\"/\"value_field\": \"metric\"/" \
  example/batch-minimal/mysqlbatch/flow.mysql_to_redis.count.json > /tmp/flow.mysql_to_redis.seed_event.json
go run ./cmd/batch -config /tmp/flow.mysql_to_redis.seed_event.json

# redis -> mysql
go run ./cmd/batch -config example/batch-minimal/redisbatch/flow.redis_to_mysql.count.json

# redis -> redis
go run ./cmd/batch -config example/batch-minimal/redisbatch/flow.redis_to_redis.count.json

# benchmark by config
go run ./cmd/batch -mode benchmark -config example/batch-minimal/mysqlbatch/flow.mysql_to_redis.count.json
go run ./cmd/batch -mode benchmark -config example/batch-minimal/redisbatch/flow.redis_to_redis.count.json
```

What it does:

1. Read source rows from MySQL by primary-key range shards.
2. Run MapReduce in-process using `cmd/agg.so`.
3. Batch upsert aggregated result into target DB/table.

## 6) Validate result

Use direct sink checks after `flow done`.

For MySQL sink flows (`flow.mysql.json`, `flow.mysql.count.json`, `flow.redis_to_mysql.count.json`):

```sql
SELECT COUNT(*) AS rows_written FROM mr_target.agg_results;
SELECT * FROM mr_target.agg_results ORDER BY 1 LIMIT 10;
```

For Redis sink flows (`flow.mysql_to_redis.count.json`, `flow.redis_to_redis.count.json`), check by key pattern:

```bash
go run ./cmd/batch -check -config example/batch-minimal/mysqlbatch/flow.mysql_to_redis.count.json
go run ./cmd/batch -check -config example/batch-minimal/redisbatch/flow.redis_to_redis.count.json
```

Advanced path:
- `go run ./cmd/batch -mode validate` remains available for dedicated default-pipeline checks,
- but it assumes a matching source/target state and is not part of this minimal cross-flow walkthrough.

## Troubleshooting

- `dial tcp localhost:3306`: MySQL not reachable or credentials incorrect.
- `plugin.Open ... different version`: rebuild plugin with the same Go environment as runtime.
- `no reduce output files matched`: pipeline did not produce `mr-out-*.txt` (check plugin path and runtime logs).
