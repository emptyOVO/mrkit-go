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

	db, err := openDB(ctx, cfg.DB)
	if err != nil {
		return err
	}
	defer db.Close()

	files, err := ExportSourceByPKRange(ctx, db, cfg.Source)
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

	return ImportReduceOutputs(ctx, db, cfg.Sink)
}
