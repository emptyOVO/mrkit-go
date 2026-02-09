# mysqlbatch-minimal

A copy-paste-ready minimal integration example for using `github.com/emptyOVO/mrkit-go/mysqlbatch` in an external project.

## 1) Files

This example only needs:

- `go.mod`
- `main.go`

For local development in this repository, `go.mod` uses:

```go
replace github.com/emptyOVO/mrkit-go => ../..
```

If you copy this example into another repository, replace that line with your own module source (for example a git tag/version).

## 2) Prepare plugin

`mysqlbatch` pipeline needs a MapReduce plugin (`.so`). Build one first:

```bash
cd /Users/empty/Library/Mobile Documents/com~apple~CloudDocs/毕设/mapreduce
go build -buildmode=plugin -o cmd/mysql_agg.so ./mrapps/mysql_agg.go
```

## 3) Set MySQL env vars

```bash
export MYSQL_HOST=127.0.0.1
export MYSQL_PORT=3306
export MYSQL_USER=root
export MYSQL_PASSWORD=123456
export MYSQL_DB=mysql

# Source/target tables used by main.go
export SOURCE_TABLE=mr_demo_source
export TARGET_TABLE=mr_demo_target
```

## 4) (Optional) Prepare demo source data

Use the built-in runner once to create demo data:

```bash
cd /Users/empty/Library/Mobile Documents/com~apple~CloudDocs/毕设/mapreduce
ROWS=20000 go run ./cmd/mysqlbatch -mode prepare
```

## 5) Run this minimal external example

```bash
cd /Users/empty/Library/Mobile Documents/com~apple~CloudDocs/毕设/mapreduce/example/mysqlbatch-minimal
go mod tidy
go run .
```

What it does:

1. Read source rows from MySQL by primary-key range shards.
2. Run MapReduce in-process using `cmd/mysql_agg.so`.
3. Batch upsert aggregated result into target table.

## 6) Validate result

```bash
cd /Users/empty/Library/Mobile Documents/com~apple~CloudDocs/毕设/mapreduce
go run ./cmd/mysqlbatch -mode validate
```

If everything is correct, output is:

```text
validate pass
```

## Troubleshooting

- `dial tcp 127.0.0.1:3306`: MySQL not reachable or credentials incorrect.
- `plugin.Open ... different version`: rebuild plugin with the same Go environment as runtime.
- `no reduce output files matched`: pipeline did not produce `mr-out-*.txt` (check plugin path and runtime logs).
