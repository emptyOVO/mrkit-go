# mrkit-go

[<img alt="github" src="https://img.shields.io/badge/github-emptyOVO%2Fmrkit--go-blue?style=for-the-badge&logo=appveyor" height="20">](https://github.com/emptyOVO/mrkit-go)
[<img alt="build status" src="https://img.shields.io/github/actions/workflow/status/emptyOVO/mrkit-go/go.yml?style=for-the-badge" height="20">](https://github.com/emptyOVO/mrkit-go/actions/workflows/go.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/emptyOVO/mrkit-go.svg)](https://pkg.go.dev/github.com/emptyOVO/mrkit-go)

`mrkit-go` is an easy-to-use MapReduce toolkit in Go.
It is based on an existing Go MapReduce implementation and repackaged as a more plug-and-play library/runner for local use.

![mapReduce](https://github.com/kiarash8112/MapReduce/assets/133909368/03c6b149-213c-4906-91b7-a05c4f083c9e)



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

- package: `mysqlbatch`
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

	"github.com/emptyOVO/mrkit-go/mysqlbatch"
)

func main() {
	_ = mysqlbatch.RunPipeline(context.Background(), mysqlbatch.PipelineConfig{
		DB: mysqlbatch.DBConfig{
			Host:     "127.0.0.1",
			Port:     3306,
			User:     "root",
			Password: "123456",
			Database: "mysql",
		},
		SourceDB: mysqlbatch.DBConfig{
			Host:     "127.0.0.1",
			Port:     3306,
			User:     "root",
			Password: "123456",
			Database: "mysql",
		},
		SinkDB: mysqlbatch.DBConfig{
			Host:     "127.0.0.1",
			Port:     3306,
			User:     "root",
			Password: "123456",
			Database: "mr_target",
		},
		Source: mysqlbatch.SourceConfig{
			Table: "source_events",
		},
		Sink: mysqlbatch.SinkConfig{
			TargetTable: "agg_results",
			Replace:     true,
		},
		PluginPath: "cmd/mysql_agg.so",
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
go run ./cmd/mysqlbatch -mode prepare

# 2) run end-to-end pipeline (source mysql -> target mr_target)
MYSQL_HOST=localhost MYSQL_PORT=3306 MYSQL_USER=root MYSQL_PASSWORD=123456 MYSQL_DB=mysql \
MYSQL_SOURCE_DB=mysql MYSQL_TARGET_DB=mr_target \
SOURCE_TABLE=source_events TARGET_TABLE=agg_results \
go run ./cmd/mysqlbatch -mode pipeline -plugin cmd/mysql_agg.so

# 3) validate correctness
MYSQL_HOST=127.0.0.1 MYSQL_PORT=3306 MYSQL_USER=root MYSQL_PASSWORD=123456 MYSQL_DB=mysql \
SOURCE_TABLE=source_events TARGET_TABLE=agg_results \
go run ./cmd/mysqlbatch -mode validate
```

Minimal external integration example:
- `example/mysqlbatch-minimal/go.mod`
- `example/mysqlbatch-minimal/main.go`

### Benchmark

```bash
MYSQL_HOST=127.0.0.1 MYSQL_PORT=3306 MYSQL_USER=root MYSQL_PASSWORD=123456 MYSQL_DB=mysql \
SOURCE_TABLE=source_events TARGET_TABLE=agg_results PREPARE_DATA=true ROWS=10000000 \
go run ./cmd/mysqlbatch -mode benchmark -plugin cmd/mysql_agg.so
```

## Contributions
Pull requests are always welcome!


<sup>
Created and improved by Yi-fei Gao. All code is
licensed under the <a href="LICENSE">Apache License 2.0</a>.
</sup>
