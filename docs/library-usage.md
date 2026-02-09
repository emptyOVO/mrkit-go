# Go Library Usage

You can call the library directly (without JSON flow files):

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

For minimal integration setup, see:

- `example/batch-minimal/go.mod`
- `example/batch-minimal/main.go`
