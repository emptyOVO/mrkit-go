package batch

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/emptyOVO/mrkit-go/batch/mysql_batch"
	"github.com/emptyOVO/mrkit-go/batch/redis_batch"
)

var transformEnvMu sync.Mutex

// FlowConfig describes a SeaTunnel-like source/transform/sink pipeline.
type FlowConfig struct {
	Version   string              `json:"version"`
	Source    FlowSourceConfig    `json:"source"`
	Transform FlowTransformConfig `json:"transform"`
	Sink      FlowSinkConfig      `json:"sink"`
}

type FlowSourceConfig struct {
	Type        string            `json:"type"`
	DB          DBConfig          `json:"db"`
	Redis       RedisConnConfig   `json:"redis"`
	Config      SourceConfig      `json:"config"`
	RedisConfig RedisSourceConfig `json:"redis_config"`
}

type FlowTransformConfig struct {
	Type       string            `json:"type"`
	Builtin    string            `json:"builtin"`
	PluginPath string            `json:"plugin_path"`
	Reducers   int               `json:"reducers"`
	Workers    int               `json:"workers"`
	InRAM      bool              `json:"in_ram"`
	Port       int               `json:"port"`
	Params     map[string]string `json:"params"`
}

type FlowSinkConfig struct {
	Type        string          `json:"type"`
	DB          DBConfig        `json:"db"`
	Redis       RedisConnConfig `json:"redis"`
	Config      SinkConfig      `json:"config"`
	RedisConfig RedisSinkConfig `json:"redis_config"`
}

func (c *FlowConfig) withDefaults() {
	if c.Source.Type == "" {
		c.Source.Type = "mysql"
	}
	if c.Transform.Type == "" {
		c.Transform.Type = "builtin"
	}
	if c.Sink.Type == "" {
		c.Sink.Type = "mysql"
	}
	if c.Transform.Reducers <= 0 {
		c.Transform.Reducers = 8
	}
	if c.Transform.Workers <= 0 {
		c.Transform.Workers = 16
	}
	if c.Transform.Port == 0 {
		c.Transform.Port = 10000
	}
	c.Source.Config.WithDefaults()
	c.Sink.Config.WithDefaults()
	c.Source.RedisConfig.WithDefaults()
	c.Sink.RedisConfig.WithDefaults()
}

// FlowBenchmarkResult captures source/transform/sink stage durations.
type FlowBenchmarkResult struct {
	SourceDuration    time.Duration
	TransformDuration time.Duration
	SinkDuration      time.Duration
	TotalDuration     time.Duration
}

// RunFlow executes source -> transform -> sink defined by FlowConfig.
func RunFlow(ctx context.Context, cfg FlowConfig) error {
	_, err := runFlowInternal(ctx, cfg, false)
	return err
}

// RunFlowBenchmark executes a config-driven flow and reports stage durations.
func RunFlowBenchmark(ctx context.Context, cfg FlowConfig) (FlowBenchmarkResult, error) {
	return runFlowInternal(ctx, cfg, true)
}

func runFlowInternal(ctx context.Context, cfg FlowConfig, collectDur bool) (FlowBenchmarkResult, error) {
	var bench FlowBenchmarkResult
	started := time.Now()

	cfg.withDefaults()
	if err := ValidateFlowConfig(cfg); err != nil {
		return bench, err
	}

	if err := withTransformParams(cfg.Transform.Params, func() error {
		pluginPath, err := resolveTransformPlugin(cfg.Transform)
		if err != nil {
			return err
		}

		sSource := time.Now()
		var files []string
		switch cfg.Source.Type {
		case "mysql":
			sourceDB, err := openDB(ctx, cfg.Source.DB)
			if err != nil {
				return err
			}
			files, err = mysql_batch.NewSourceAdapter(cfg.Source.Config).Export(ctx, sourceDB)
			sourceDB.Close()
			if err != nil {
				return err
			}
		case "redis":
			files, err = redis_batch.NewSourceAdapter(cfg.Source.Redis, cfg.Source.RedisConfig).Export(ctx)
			if err != nil {
				return err
			}
		default:
			return nil
		}
		if collectDur {
			bench.SourceDuration = time.Since(sSource)
		}
		if len(files) == 0 {
			return nil
		}

		var inputGlob string
		switch cfg.Sink.Type {
		case "mysql":
			inputGlob = mysql_batch.NewSinkAdapter(cfg.Sink.Config).InputGlob()
		case "redis":
			inputGlob = redis_batch.NewSinkAdapter(cfg.Sink.Redis, cfg.Sink.RedisConfig).InputGlob()
		default:
			return nil
		}
		cleanupReduceOutputs(inputGlob)

		sTransform := time.Now()
		if err := runMapReduce(ctx, files, pluginPath, cfg.Transform); err != nil {
			return err
		}
		if collectDur {
			bench.TransformDuration = time.Since(sTransform)
		}

		sSink := time.Now()
		switch cfg.Sink.Type {
		case "mysql":
			sinkDB, err := openDB(ctx, cfg.Sink.DB)
			if err != nil {
				return err
			}
			err = mysql_batch.NewSinkAdapter(cfg.Sink.Config).Import(ctx, sinkDB)
			sinkDB.Close()
			if err != nil {
				return err
			}
		case "redis":
			if err := redis_batch.NewSinkAdapter(cfg.Sink.Redis, cfg.Sink.RedisConfig).Import(ctx); err != nil {
				return err
			}
		default:
			return nil
		}
		if collectDur {
			bench.SinkDuration = time.Since(sSink)
		}
		return nil
	}); err != nil {
		return bench, err
	}

	if collectDur {
		bench.TotalDuration = time.Since(started)
	}
	return bench, nil
}

func withTransformParams(params map[string]string, run func() error) error {
	if len(params) == 0 {
		return run()
	}
	transformEnvMu.Lock()
	defer transformEnvMu.Unlock()

	restore := make(map[string]*string, len(params))
	for k, v := range params {
		if old, ok := os.LookupEnv(k); ok {
			oldCopy := old
			restore[k] = &oldCopy
		} else {
			restore[k] = nil
		}
		if err := os.Setenv(k, v); err != nil {
			return err
		}
	}
	defer func() {
		for k, old := range restore {
			if old == nil {
				_ = os.Unsetenv(k)
			} else {
				_ = os.Setenv(k, *old)
			}
		}
	}()
	return run()
}

func cleanupReduceOutputs(inputGlob string) {
	if outs, err := filepath.Glob(inputGlob); err == nil {
		for _, out := range outs {
			_ = os.Remove(out)
		}
	}
}

func runMapReduce(ctx context.Context, files []string, pluginPath string, tf FlowTransformConfig) error {
	return RunMapReduce(ctx, MapReduceRunConfig{
		Files:      files,
		PluginPath: pluginPath,
		Reducers:   tf.Reducers,
		Workers:    tf.Workers,
		InRAM:      tf.InRAM,
		Port:       tf.Port,
	})
}
