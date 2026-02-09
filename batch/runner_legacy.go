package batch

import (
	"context"
	"fmt"
	"strconv"

	mapreduce "github.com/emptyOVO/mrkit-go"
)

// LegacyRunner uses the current in-process legacy mapreduce runtime.
type LegacyRunner struct{}

func (LegacyRunner) Run(_ context.Context, cfg MapReduceRunConfig) error {
	if len(cfg.Files) == 0 {
		return nil
	}
	if cfg.PluginPath == "" {
		return fmt.Errorf("plugin path is required")
	}
	if cfg.Reducers <= 0 {
		return fmt.Errorf("reducers must be > 0")
	}
	if cfg.Workers <= 0 {
		return fmt.Errorf("workers must be > 0")
	}
	if cfg.Port <= 0 {
		cfg.Port = 10000
	}

	mapreduce.MasterIP = ":" + strconv.Itoa(cfg.Port)
	mapreduce.StartSingleMachineJob(cfg.Files, cfg.PluginPath, cfg.Reducers, cfg.Workers, cfg.InRAM)
	return nil
}
