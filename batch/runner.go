package batch

import (
	"context"
	"sync"
)

// MapReduceRunConfig describes a runtime invocation for a map-reduce job.
type MapReduceRunConfig struct {
	Files      []string
	PluginPath string
	Reducers   int
	Workers    int
	InRAM      bool
	Port       int
}

// Runner abstracts runtime startup strategy for map-reduce execution.
type Runner interface {
	Run(ctx context.Context, cfg MapReduceRunConfig) error
}

var (
	defaultRunner   Runner = LegacyRunner{}
	defaultRunnerMu sync.RWMutex
)

// SetDefaultRunner overrides the process-wide runtime strategy.
func SetDefaultRunner(r Runner) {
	if r == nil {
		return
	}
	defaultRunnerMu.Lock()
	defer defaultRunnerMu.Unlock()
	defaultRunner = r
}

// DefaultRunner returns the current process-wide runtime strategy.
func DefaultRunner() Runner {
	defaultRunnerMu.RLock()
	defer defaultRunnerMu.RUnlock()
	return defaultRunner
}

// RunMapReduce executes map-reduce through the configured runner.
func RunMapReduce(ctx context.Context, cfg MapReduceRunConfig) error {
	return DefaultRunner().Run(ctx, cfg)
}
