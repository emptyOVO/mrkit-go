package mysqlbatch

import (
	"context"
	"time"
)

// BenchmarkConfig configures benchmark workflow.
type BenchmarkConfig struct {
	DB       DBConfig
	Prepare  bool
	PrepareC PrepareConfig
	Pipeline PipelineConfig
	Validate ValidateConfig
}

// BenchmarkResult captures stage durations.
type BenchmarkResult struct {
	PrepareDuration  time.Duration
	PipelineDuration time.Duration
	ValidateDuration time.Duration
	TotalDuration    time.Duration
}

func RunBenchmark(ctx context.Context, cfg BenchmarkConfig) (BenchmarkResult, error) {
	var result BenchmarkResult
	startAll := time.Now()

	db, err := openDB(ctx, cfg.DB)
	if err != nil {
		return result, err
	}
	defer db.Close()

	if cfg.Prepare {
		s := time.Now()
		if err := PrepareSyntheticSource(ctx, db, cfg.PrepareC); err != nil {
			return result, err
		}
		result.PrepareDuration = time.Since(s)
	}

	s := time.Now()
	cfg.Pipeline.DB = cfg.DB
	if err := RunPipeline(ctx, cfg.Pipeline); err != nil {
		return result, err
	}
	result.PipelineDuration = time.Since(s)

	s = time.Now()
	if err := ValidateAggregation(ctx, db, cfg.Validate); err != nil {
		return result, err
	}
	result.ValidateDuration = time.Since(s)

	result.TotalDuration = time.Since(startAll)
	return result, nil
}
