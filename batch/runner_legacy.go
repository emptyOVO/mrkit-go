package batch

import (
	"context"
	"fmt"
	"strconv"
	"sync"

	mapreduce "github.com/emptyOVO/mrkit-go"
)

// LegacyRunner uses the current in-process legacy mapreduce runtime.
type LegacyRunner struct{}

var legacyRuntimeMu sync.Mutex

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

	legacyRuntimeMu.Lock()
	defer legacyRuntimeMu.Unlock()
	return mapreduce.StartSingleMachineJobWithAddr(
		cfg.Files,
		cfg.PluginPath,
		cfg.Reducers,
		cfg.Workers,
		cfg.InRAM,
		":"+strconv.Itoa(cfg.Port),
	)
}
