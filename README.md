# mrkit-go

[<img alt="github" src="https://img.shields.io/badge/github-emptyOVO%2Fmrkit--go-blue?style=for-the-badge&logo=appveyor" height="20">](https://github.com/emptyOVO/mrkit-go)
[<img alt="build status" src="https://img.shields.io/github/actions/workflow/status/emptyOVO/mrkit-go/go.yml?style=for-the-badge" height="20">](https://github.com/emptyOVO/mrkit-go/actions/workflows/go.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/emptyOVO/mrkit-go.svg)](https://pkg.go.dev/github.com/emptyOVO/mrkit-go)

`mrkit-go` is an easy-to-use MapReduce toolkit in Go.
It is based on an existing Go MapReduce implementation and repackaged as a more plug-and-play library/runner for local use.

![mapReduce](https://github.com/kiarash8112/MapReduce/assets/133909368/03c6b149-213c-4906-91b7-a05c4f083c9e)


## Quickstart (Run in 10 Minutes)

This section is for first-time users who just want to run an end-to-end MySQL batch job:

`MySQL source table -> MapReduce -> MySQL sink table`

### 0) Prerequisites

- Go 1.21+ installed
- MySQL running and reachable (example below uses `localhost:3306`)
- MySQL account with read/write permission on source/sink databases

### 1) Prepare demo source data

```bash
cd /path/to/mrkit-go
MYSQL_HOST=localhost MYSQL_PORT=3306 MYSQL_USER=root MYSQL_PASSWORD=123456 \
MYSQL_DB=mysql SOURCE_TABLE=source_events TARGET_TABLE=agg_results \
ROWS=5000 KEY_MOD=100 \
go run ./cmd/batch -mode prepare
```

### 2) Validate config schema (v1)

```bash
go run ./cmd/batch -check -config example/batch-minimal/mysqlbatch/flow.mysql.count.json
```

Expected output:

```text
config check pass
```

### 3) Run config-driven flow

Run one of these:

```bash
go run ./cmd/batch -config example/batch-minimal/mysqlbatch/flow.mysql.count.json
go run ./cmd/batch -config example/batch-minimal/mysqlbatch/flow.mysql.minmax.json
go run ./cmd/batch -config example/batch-minimal/mysqlbatch/flow.mysql.topn.json
```

Cross-DB examples:

```bash
# mysql -> redis
go run ./cmd/batch -config example/batch-minimal/mysqlbatch/flow.mysql_to_redis.count.json

# redis -> mysql
go run ./cmd/batch -config example/batch-minimal/redisbatch/flow.redis_to_mysql.count.json

# redis -> redis
go run ./cmd/batch -config example/batch-minimal/redisbatch/flow.redis_to_redis.count.json
```

Flow benchmark by config (works for mysql/redis source/sink):

```bash
go run ./cmd/batch -mode benchmark -config example/batch-minimal/mysqlbatch/flow.mysql_to_redis.count.json
go run ./cmd/batch -mode benchmark -config example/batch-minimal/redisbatch/flow.redis_to_redis.count.json
```

### 4) Verify success

Expected log includes:

```text
flow done
```

Result tables (default sink DB: `mr_target`):

- `agg_count_results`
- `agg_minmax_results`
- `agg_topn_results`

### Notes

- `count` is additive and can run with multiple reducers.
- `minmax` and `topN` are non-additive. Use `reducers=1` in config to avoid cross-reducer merge distortion.
- For built-in transforms (`count`/`minmax`/`topN`), no prebuilt `.so` is required.


## Features
- Multiple worker goroutines in one process on a single machine.
- Multiple worker processes on a single machine.
- Fault tolerance.
- Easy to parallel your code with Map and Reduce functions.


## Library Usage - Your own map and reduce function
Here's a simply example for word count program.
wc.go
```golang
package main
import (
	"strconv"
	"strings"
	"unicode"

	"github.com/emptyOVO/mrkit-go/worker"
)
func Map(filename string, contents string, ctx worker.MrContext) {
	// function to detect word separators.
	ff := func(r rune) bool { return !unicode.IsLetter(r) }

	// split contents into an array of words.
	words := strings.FieldsFunc(contents, ff)

	for _, w := range words {
		ctx.EmitIntermediate(w, "1")
	}
}
func Reduce(key string, values []string, ctx worker.MrContext) {
	// return the number of occurrences of this word.
	ctx.Emit(key, strconv.Itoa(len(values)))
}
```

### Usage - 1 program with master, worker goroutine

main.go
```golang
package main

import (
	mp "github.com/emptyOVO/mrkit-go"
)

func main() {
	mp.StartSingleMachineJob(mp.ParseArg())
}
```

Run with :
```
# Compile plugin
go build -race -buildmode=plugin -o wc.so wc.go

# Word count
go run -race main.go -i 'input/files' -p 'wc.so' -r 1 -w 8
```

Output file name is `mr-out-0.txt`

More example can be found in the [`mrapps/`](mrapps/) folder, and we will add more example in the future.

## Usage - Master program, and worker program (Isolate master and workers)

master.go
```golang
package main

import (
	mp "github.com/emptyOVO/mrkit-go"
)

func main() {
	mp.StartMaster(mp.ParseArg())
}
```

worker.go
```golang
package main

import (
	mp "github.com/emptyOVO/mrkit-go"
)

func main() {
	mp.StartWorker(mp.ParseArg())
}
```

Run with :
```
# Compile plugin
go build -race -buildmode=plugin -o wc.so wc.go

# Word count
go run -race cmd/master.go -i 'txt/*' -p 'cmd/wc.so' -r 1 -w 8 &
sleep 1
go run -race cmd/worker.go -i 'txt/*' -p 'cmd/wc.so' -r 1 -w 1 &
go run -race cmd/worker.go -i 'txt/*' -p 'cmd/wc.so' -r 1 -w 2 &
go run -race cmd/worker.go -i 'txt/*' -p 'cmd/wc.so' -r 1 -w 3 &
go run -race cmd/worker.go -i 'txt/*' -p 'cmd/wc.so' -r 1 -w 4 &
go run -race cmd/worker.go -i 'txt/*' -p 'cmd/wc.so' -r 1 -w 5 &
go run -race cmd/worker.go -i 'txt/*' -p 'cmd/wc.so' -r 1 -w 6 &
go run -race cmd/worker.go -i 'txt/*' -p 'cmd/wc.so' -r 1 -w 7 &
go run -race cmd/worker.go -i 'txt/*' -p 'cmd/wc.so' -r 1 -w 8 
```



## Help
```
MapReduce is an easy-to-use MapReduce Go parallel-computing toolkit inspired by 2021 6.824 lab1.
It supports multiple workers threads on a single machine and multiple processes on a single machine right now.

Usage:
  mapreduce [flags]

Flags:
  -h, --help            help for mapreduce
  -m, --inRAM           Whether write the intermediate file in RAM (default true)
  -i, --input strings   Input files
  -p, --plugin string   Plugin .so file
      --port int        Port number (default 10000)
  -r, --reduce int      Number of Reducers (default 1)
  -w, --worker int      Number of Workers(for master node)
                        ID of worker(for worker node) (default 4)
```

## MySQL Batch Processing (Go Library)

The framework now provides a **Go library** for MySQL source/sink and benchmark workflows:

- package: `batch`
- source read: primary-key range sharding (`id` range split)
- sink write: batch stage insert + grouped upsert
- end-to-end: MySQL -> MapReduce -> MySQL in one Go call
- supports cross-database pipeline: source DB and target DB can be configured separately

Reducer partitioning is fixed to `hash(key) % nReduce`.

### Library usage

```go
package main

import (
	"context"

	"github.com/emptyOVO/mrkit-go/batch"
)

func main() {
	_ = batch.RunPipeline(context.Background(), batch.PipelineConfig{
		DB: batch.DBConfig{
			Host:     "127.0.0.1",
			Port:     3306,
			User:     "root",
			Password: "123456",
			Database: "mysql",
		},
		SourceDB: batch.DBConfig{
			Host:     "127.0.0.1",
			Port:     3306,
			User:     "root",
			Password: "123456",
			Database: "mysql",
		},
		SinkDB: batch.DBConfig{
			Host:     "127.0.0.1",
			Port:     3306,
			User:     "root",
			Password: "123456",
			Database: "mr_target",
		},
		Source: batch.SourceConfig{
			Table: "source_events",
		},
		Sink: batch.SinkConfig{
			TargetTable: "agg_results",
			Replace:     true,
		},
		PluginPath: "cmd/agg.so",
		Reducers:   8,
		Workers:    16,
		InRAM:      false,
		Port:       10000,
	})
}
```

### Built-in Go runner

This repo also includes a runner command based on the same library:

```bash
# 1) prepare synthetic source table (default 10m rows)
MYSQL_HOST=127.0.0.1 MYSQL_PORT=3306 MYSQL_USER=root MYSQL_PASSWORD=123456 MYSQL_DB=mysql \
SOURCE_TABLE=source_events TARGET_TABLE=agg_results ROWS=10000000 \
go run ./cmd/batch -mode prepare

# 2) run end-to-end pipeline (source mysql -> target mr_target)
MYSQL_HOST=localhost MYSQL_PORT=3306 MYSQL_USER=root MYSQL_PASSWORD=123456 MYSQL_DB=mysql \
MYSQL_SOURCE_DB=mysql MYSQL_TARGET_DB=mr_target \
SOURCE_TABLE=source_events TARGET_TABLE=agg_results \
go run ./cmd/batch -mode pipeline -plugin cmd/agg.so

# 3) validate correctness
MYSQL_HOST=127.0.0.1 MYSQL_PORT=3306 MYSQL_USER=root MYSQL_PASSWORD=123456 MYSQL_DB=mysql \
SOURCE_TABLE=source_events TARGET_TABLE=agg_results \
go run ./cmd/batch -mode validate
```

### Config-driven flow (source/transform/sink)

For a plug-and-play experience (SeaTunnel-like), you can run by a single JSON config:

```bash
go run ./cmd/batch -check -config example/batch-minimal/mysqlbatch/flow.mysql.json
go run ./cmd/batch -config example/batch-minimal/mysqlbatch/flow.mysql.json
```

Config sections:
- `source`: MySQL source connection + extract config
- `transform`: built-in transform (`count` / `minmax` / `topN`) or plugin mode
- `sink`: MySQL or Redis sink config

Production template (source/sink split + concurrency):

```json
{
  "version": "v1",
  "source": {
    "type": "mysql",
    "db": {
      "host": "10.20.1.11",
      "port": 3306,
      "user": "etl_reader",
      "password": "REPLACE_ME",
      "database": "biz_source",
      "params": {
        "readTimeout": "60s",
        "writeTimeout": "60s",
        "timeout": "10s",
        "loc": "Local",
        "multiStatements": "false"
      }
    },
    "config": {
      "table": "order_events",
      "pkcolumn": "id",
      "keycolumn": "biz_key",
      "valcolumn": "metric",
      "where": "event_time >= '2026-02-01 00:00:00' AND event_time < '2026-02-02 00:00:00'",
      "shards": 64,
      "parallel": 16,
      "outputdir": "txt/mysql_source",
      "fileprefix": "chunk"
    }
  },
  "transform": {
    "type": "builtin",
    "builtin": "count",
    "reducers": 16,
    "workers": 32,
    "in_ram": false,
    "port": 18000
  },
  "sink": {
    "type": "mysql",
    "db": {
      "host": "10.20.2.15",
      "port": 3306,
      "user": "etl_writer",
      "password": "REPLACE_ME",
      "database": "biz_dw",
      "params": {
        "readTimeout": "60s",
        "writeTimeout": "60s",
        "timeout": "10s",
        "loc": "Local",
        "multiStatements": "false"
      }
    },
    "config": {
      "targettable": "order_metric_daily",
      "keycolumn": "biz_key",
      "valcolumn": "metric_sum",
      "inputglob": "mr-out-*.txt",
      "replace": true,
      "batchsize": 5000
    }
  }
}
```

Run:

```bash
go run ./cmd/batch -config /absolute/path/flow.prod.json
```

Plugin mode is still available (advanced use case):

```json
{
  "version": "v1",
  "transform": {
    "type": "mapreduce",
    "plugin_path": "cmd/agg.so",
    "reducers": 8,
    "workers": 16,
    "in_ram": false,
    "port": 10000
  }
}
```

Failure rerun suggestions:
- Keep `where` windowed (time range or batch id) so each run has deterministic input.
- If `sink.config.replace=true`, rerunning the same batch is idempotent (overwrite behavior).
- If you need history accumulation, use `replace=false` and ensure batch filters avoid duplicates.
- Before rerun, clean local artifacts: `mr-out-*.txt`, `txt/mysql_source/chunk-*.txt`.
- Start with moderate parallelism: `shards=4x~8x reducers`, `workers=2x reducers`, then tune by DB load.

Minimal external integration example:
- `example/batch-minimal/go.mod`
- `example/batch-minimal/main.go`

### Benchmark

```bash
MYSQL_HOST=127.0.0.1 MYSQL_PORT=3306 MYSQL_USER=root MYSQL_PASSWORD=123456 MYSQL_DB=mysql \
SOURCE_TABLE=source_events TARGET_TABLE=agg_results PREPARE_DATA=true ROWS=10000000 \
go run ./cmd/batch -mode benchmark -plugin cmd/agg.so
```

## Contributions
Pull requests are always welcome!


<sup>
Created and improved by Yi-fei Gao. All code is
licensed under the <a href="LICENSE">Apache License 2.0</a>.
</sup>
