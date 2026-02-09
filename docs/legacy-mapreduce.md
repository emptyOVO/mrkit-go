# Legacy MapReduce Entrypoints

These are legacy/advanced entrypoints. New users should prefer `./cmd/batch` with JSON config.

## Single Process (Master + Worker Goroutines)

`main.go`:

```go
package main

import (
	mp "github.com/emptyOVO/mrkit-go"
)

func main() {
	mp.StartSingleMachineJob(mp.ParseArg())
}
```

Run:

```bash
go build -race -buildmode=plugin -o cmd/wc.so ./mrapps/wc.go
go run -race ./cmd/main.go -i 'txt/*' -p 'cmd/wc.so' -r 1 -w 4 --port 11320 -m=false
```

## Multi Process (Master + Workers)

`master.go`:

```go
package main

import (
	mp "github.com/emptyOVO/mrkit-go"
)

func main() {
	mp.StartMaster(mp.ParseArg())
}
```

`worker.go`:

```go
package main

import (
	mp "github.com/emptyOVO/mrkit-go"
)

func main() {
	mp.StartWorker(mp.ParseArg())
}
```

Run:

```bash
go build -race -buildmode=plugin -o cmd/wc.so ./mrapps/wc.go
go run -race ./cmd/master.go -i 'txt/*' -p 'cmd/wc.so' -r 1 -w 2 --port 11340 -m=false &
sleep 1
go run -race ./cmd/worker.go -i 'txt/*' -p 'cmd/wc.so' -r 1 -w 1 --port 11340 -m=false &
go run -race ./cmd/worker.go -i 'txt/*' -p 'cmd/wc.so' -r 1 -w 2 --port 11340 -m=false &
```

Notes:

- Keep plugin build mode and runtime mode consistent (both with `-race`, or both without).
- `master -w N` means you should start `N` workers for this demo.

## CLI Help

```text
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
