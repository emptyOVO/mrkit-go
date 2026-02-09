# mysqlbatch-minimal

A copy-paste-ready minimal integration example for using `github.com/emptyOVO/mrkit-go/mysqlbatch` in an external project.
This version demonstrates **cross-database** pipeline and **config-driven run**:
- read from source DB/table
- run MapReduce
- write aggregated result to target DB/table

## 1) Files

This example includes:

- `go.mod`
- `main.go`
- `flow.mysql.json`

For local development in this repository, `go.mod` uses:

```go
replace github.com/emptyOVO/mrkit-go => ../..
```

If you copy this example into another repository, replace that line with your own module source (for example a git tag/version).

## 2) Prepare plugin

`mysqlbatch` pipeline needs a MapReduce plugin (`.so`). Build one first:

```bash
cd /Users/empty/Library/Mobile Documents/com~apple~CloudDocs/毕设/mrkit-go
go build -buildmode=plugin -o cmd/mysql_agg.so ./mrapps/mysql_agg.go
```

## 3) Prepare flow config

Use `example/mysqlbatch-minimal/flow.mysql.json` as the template.
You mainly need to adjust:

- `source.db` / `sink.db`
- `source.config.table`
- `sink.config.targettable`
- `transform.plugin_path`

## 4) (Optional) Prepare demo source data

```bash
cd /Users/empty/Library/Mobile Documents/com~apple~CloudDocs/毕设/mrkit-go
ROWS=20000 MYSQL_SOURCE_DB=mysql SOURCE_TABLE=source_events \
go run ./cmd/mysqlbatch -mode prepare
```

Before running pipeline, ensure target database in `flow.mysql.json` exists (for example: `mr_target`).

## 5) Run by config file (recommended)

```bash
cd /Users/empty/Library/Mobile Documents/com~apple~CloudDocs/毕设/mrkit-go
go run ./cmd/mysqlbatch -config example/mysqlbatch-minimal/flow.mysql.json
```

What it does:

1. Read source rows from MySQL by primary-key range shards.
2. Run MapReduce in-process using `cmd/mysql_agg.so`.
3. Batch upsert aggregated result into target DB/table.

## 6) Validate result

```bash
cd /Users/empty/Library/Mobile Documents/com~apple~CloudDocs/毕设/mrkit-go
MYSQL_DB=mysql SOURCE_TABLE=source_events \
TARGET_TABLE=agg_results \
go run ./cmd/mysqlbatch -mode validate
```

If everything is correct, output is:

```text
validate pass
```

## Troubleshooting

- `dial tcp localhost:3306`: MySQL not reachable or credentials incorrect.
- `plugin.Open ... different version`: rebuild plugin with the same Go environment as runtime.
- `no reduce output files matched`: pipeline did not produce `mr-out-*.txt` (check plugin path and runtime logs).
