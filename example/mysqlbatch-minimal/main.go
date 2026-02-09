package main

import (
	"context"
	"log"

	"github.com/emptyOVO/mrkit-go/mysqlbatch"
)

func main() {
	cfg := mysqlbatch.PipelineConfig{
		DB: mysqlbatch.DBConfig{
			Host:     "127.0.0.1",
			Port:     3306,
			User:     "root",
			Password: "123456",
			Database: "mysql",
		},
		Source: mysqlbatch.SourceConfig{
			Table:    "mr_demo_source",
			Shards:   8,
			Parallel: 4,
		},
		Sink: mysqlbatch.SinkConfig{
			TargetTable: "mr_demo_target",
			Replace:     true,
		},
		PluginPath: "../../cmd/mysql_agg.so",
		Reducers:   4,
		Workers:    8,
		InRAM:      false,
		Port:       10000,
	}

	if err := mysqlbatch.RunPipeline(context.Background(), cfg); err != nil {
		log.Fatal(err)
	}
}
