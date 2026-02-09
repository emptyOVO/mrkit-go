package mysqlbatch

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	mapreduce "github.com/emptyOVO/mrkit-go"
)

// RunPipeline executes MySQL source -> MapReduce -> MySQL sink in-process.
func RunPipeline(ctx context.Context, cfg PipelineConfig) error {
	cfg.withDefaults()
	if cfg.PluginPath == "" {
		return fmt.Errorf("plugin path is required")
	}
	if cfg.Source.Table == "" {
		return fmt.Errorf("source table is required")
	}
	if cfg.Sink.TargetTable == "" {
		return fmt.Errorf("target table is required")
	}

	sourceDBCfg := cfg.SourceDB
	if sourceDBCfg.Database == "" {
		sourceDBCfg = cfg.DB
	}
	sinkDBCfg := cfg.SinkDB
	if sinkDBCfg.Database == "" {
		sinkDBCfg = cfg.DB
	}

	sourceDB, err := openDB(ctx, sourceDBCfg)
	if err != nil {
		return err
	}
	defer sourceDB.Close()

	sinkDB, err := openDB(ctx, sinkDBCfg)
	if err != nil {
		return err
	}
	defer sinkDB.Close()

	files, err := ExportSourceByPKRange(ctx, sourceDB, cfg.Source)
	if err != nil {
		return err
	}
	if len(files) == 0 {
		return nil
	}

	// cleanup old reduce outputs before a new run.
	if outs, err := filepath.Glob(cfg.Sink.InputGlob); err == nil {
		for _, out := range outs {
			_ = os.Remove(out)
		}
	}

	mapreduce.MasterIP = ":" + strconv.Itoa(cfg.Port)
	mapreduce.StartSingleMachineJob(files, cfg.PluginPath, cfg.Reducers, cfg.Workers, cfg.InRAM)

	return ImportReduceOutputs(ctx, sinkDB, cfg.Sink)
}
