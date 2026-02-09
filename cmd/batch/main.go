package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/emptyOVO/mrkit-go/batch"
)

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

func getenvBool(name string, d bool) bool {
	v := os.Getenv(name)
	if v == "" {
		return d
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return d
	}
	return b
}

func main() {
	mode := flag.String("mode", "pipeline", "pipeline|prepare|validate|benchmark")
	plugin := flag.String("plugin", filepath.Join("cmd", "agg.so"), "plugin .so path")
	configPath := flag.String("config", "", "Flow config file path (JSON)")
	checkOnly := flag.Bool("check", false, "Validate flow config schema only (requires -config)")
	flag.Parse()

	if *configPath != "" {
		cfg, err := loadFlowConfig(*configPath)
		must(err)
		must(batch.ValidateFlowConfig(cfg))
		if *checkOnly {
			fmt.Println("config check pass")
			return
		}
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Hour)
		defer cancel()
		switch *mode {
		case "pipeline":
			must(batch.RunFlow(ctx, cfg))
			fmt.Println("flow done")
		case "benchmark":
			result, err := batch.RunFlowBenchmark(ctx, cfg)
			must(err)
			fmt.Printf("source=%s transform=%s sink=%s total=%s\n", result.SourceDuration, result.TransformDuration, result.SinkDuration, result.TotalDuration)
		default:
			must(fmt.Errorf("mode %s is not supported with -config (use pipeline|benchmark)", *mode))
		}
		return
	}
	if *checkOnly {
		must(fmt.Errorf("-check requires -config"))
	}

	baseDB := batch.DBConfig{
		Host:     getenvDefault("MYSQL_HOST", "127.0.0.1"),
		Port:     getenvInt("MYSQL_PORT", 3306),
		User:     getenvDefault("MYSQL_USER", "root"),
		Password: os.Getenv("MYSQL_PASSWORD"),
		Database: os.Getenv("MYSQL_DB"),
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
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Hour)
	defer cancel()

	sourceTable := getenvDefault("SOURCE_TABLE", "source_events")
	targetTable := getenvDefault("TARGET_TABLE", "agg_results")

	sourceCfg := batch.SourceConfig{
		Table:     sourceTable,
		PKColumn:  getenvDefault("PK_COL", "id"),
		KeyColumn: getenvDefault("KEY_COL", "biz_key"),
		ValColumn: getenvDefault("VALUE_COL", "metric"),
		Where:     getenvDefault("SOURCE_WHERE", "1=1"),
		Shards:    getenvInt("SOURCE_SHARDS", 16),
		Parallel:  getenvInt("SOURCE_PARALLEL", 4),
		OutputDir: getenvDefault("SOURCE_OUT_DIR", filepath.Join("txt", "mysql_source")),
	}
	sinkCfg := batch.SinkConfig{
		TargetTable: targetTable,
		KeyColumn:   getenvDefault("TARGET_KEY_COL", "biz_key"),
		ValColumn:   getenvDefault("TARGET_VALUE_COL", "metric_sum"),
		InputGlob:   getenvDefault("MR_OUTPUT_GLOB", "mr-out-*.txt"),
		Replace:     getenvBool("SINK_REPLACE", true),
		BatchSize:   getenvInt("SINK_BATCH_SIZE", 2000),
	}

	switch *mode {
	case "pipeline":
		err := batch.RunPipeline(ctx, batch.PipelineConfig{
			DB:         baseDB,
			SourceDB:   sourceDB,
			SinkDB:     targetDB,
			Source:     sourceCfg,
			Sink:       sinkCfg,
			PluginPath: *plugin,
			Reducers:   getenvInt("MR_REDUCERS", 8),
			Workers:    getenvInt("MR_WORKERS", 16),
			InRAM:      getenvBool("MR_IN_RAM", false),
			Port:       getenvInt("MR_PORT", 10000),
		})
		must(err)
		fmt.Println("pipeline done")
	case "prepare":
		dbc, err := batch.OpenForApp(ctx, sourceDB)
		must(err)
		defer dbc.Close()
		err = batch.PrepareSyntheticSource(ctx, dbc, batch.PrepareConfig{
			SourceTable: sourceTable,
			Rows:        int64(getenvInt("ROWS", 10000000)),
			KeyMod:      int64(getenvInt("KEY_MOD", 100000)),
		})
		must(err)
		fmt.Println("prepare done")
	case "validate":
		dbc, err := batch.OpenForApp(ctx, baseDB)
		must(err)
		defer dbc.Close()
		err = batch.ValidateAggregation(ctx, dbc, batch.ValidateConfig{
			SourceTable: sourceTable,
			SourceKey:   getenvDefault("SOURCE_KEY_COL", "biz_key"),
			SourceVal:   getenvDefault("SOURCE_VALUE_COL", "metric"),
			TargetTable: targetTable,
			TargetKey:   getenvDefault("TARGET_KEY_COL", "biz_key"),
			TargetVal:   getenvDefault("TARGET_VALUE_COL", "metric_sum"),
		})
		must(err)
		fmt.Println("validate pass")
	case "benchmark":
		result, err := batch.RunBenchmark(ctx, batch.BenchmarkConfig{
			DB:      baseDB,
			Prepare: getenvBool("PREPARE_DATA", false),
			PrepareC: batch.PrepareConfig{
				SourceTable: sourceTable,
				Rows:        int64(getenvInt("ROWS", 10000000)),
				KeyMod:      int64(getenvInt("KEY_MOD", 100000)),
			},
			Pipeline: batch.PipelineConfig{
				SourceDB:   sourceDB,
				SinkDB:     targetDB,
				Source:     sourceCfg,
				Sink:       sinkCfg,
				PluginPath: *plugin,
				Reducers:   getenvInt("MR_REDUCERS", 8),
				Workers:    getenvInt("MR_WORKERS", 16),
				InRAM:      getenvBool("MR_IN_RAM", false),
				Port:       getenvInt("MR_PORT", 10000),
			},
			Validate: batch.ValidateConfig{
				SourceTable: sourceTable,
				TargetTable: targetTable,
			},
		})
		must(err)
		fmt.Printf("prepare=%s pipeline=%s validate=%s total=%s\n", result.PrepareDuration, result.PipelineDuration, result.ValidateDuration, result.TotalDuration)
	default:
		must(fmt.Errorf("unsupported mode: %s", *mode))
	}
}

func getenvDefault(name, d string) string {
	v := os.Getenv(name)
	if v == "" {
		return d
	}
	return v
}

func must(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func loadFlowConfig(path string) (batch.FlowConfig, error) {
	var cfg batch.FlowConfig
	f, err := os.Open(path)
	if err != nil {
		return cfg, err
	}
	defer f.Close()
	dec := json.NewDecoder(f)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&cfg); err != nil {
		return cfg, err
	}
	return cfg, nil
}
