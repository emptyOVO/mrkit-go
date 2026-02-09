package main

import (
	"context"
	"log"
	"os"
	"strconv"

	"github.com/emptyOVO/mrkit-go/batch"
)

func getenvDefault(name, d string) string {
	v := os.Getenv(name)
	if v == "" {
		return d
	}
	return v
}

func getenvInt(name string, d int) int {
	v := os.Getenv(name)
	if v == "" {
		return d
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return d
	}
	return n
}

func main() {
	baseDB := batch.DBConfig{
		Host:     getenvDefault("MYSQL_HOST", "localhost"),
		Port:     getenvInt("MYSQL_PORT", 3306),
		User:     getenvDefault("MYSQL_USER", "root"),
		Password: getenvDefault("MYSQL_PASSWORD", "123456"),
		Database: getenvDefault("MYSQL_DB", "mysql"),
	}
	sourceDB := batch.DBConfig{
		Host:     getenvDefault("MYSQL_SOURCE_HOST", baseDB.Host),
		Port:     getenvInt("MYSQL_SOURCE_PORT", baseDB.Port),
		User:     getenvDefault("MYSQL_SOURCE_USER", baseDB.User),
		Password: getenvDefault("MYSQL_SOURCE_PASSWORD", baseDB.Password),
		Database: getenvDefault("MYSQL_SOURCE_DB", baseDB.Database),
	}
	targetDB := batch.DBConfig{
		Host:     getenvDefault("MYSQL_TARGET_HOST", baseDB.Host),
		Port:     getenvInt("MYSQL_TARGET_PORT", baseDB.Port),
		User:     getenvDefault("MYSQL_TARGET_USER", baseDB.User),
		Password: getenvDefault("MYSQL_TARGET_PASSWORD", baseDB.Password),
		Database: getenvDefault("MYSQL_TARGET_DB", baseDB.Database),
	}

	cfg := batch.PipelineConfig{
		DB:       baseDB,
		SourceDB: sourceDB,
		SinkDB:   targetDB,
		Source: batch.SourceConfig{
			Table:    getenvDefault("SOURCE_TABLE", "source_events"),
			Shards:   getenvInt("SOURCE_SHARDS", 8),
			Parallel: getenvInt("SOURCE_PARALLEL", 4),
		},
		Sink: batch.SinkConfig{
			TargetTable: getenvDefault("TARGET_TABLE", "agg_results_xdb"),
			Replace:     true,
		},
		PluginPath: "../../cmd/mysql_agg.so",
		Reducers:   4,
		Workers:    8,
		InRAM:      false,
		Port:       10000,
	}

	if err := batch.RunPipeline(context.Background(), cfg); err != nil {
		log.Fatal(err)
	}
}
