package mysqlbatch

import (
	"context"
	"os"
	"path/filepath"
	"strconv"

	mapreduce "github.com/emptyOVO/mrkit-go"
)

// FlowConfig describes a SeaTunnel-like source/transform/sink pipeline.
// Current implementation supports:
// - source.type: "mysql"
// - transform.type: "mapreduce"
// - sink.type: "mysql"
type FlowConfig struct {
	Version   string              `json:"version"`
	Source    FlowSourceConfig    `json:"source"`
	Transform FlowTransformConfig `json:"transform"`
	Sink      FlowSinkConfig      `json:"sink"`
}

type FlowSourceConfig struct {
	Type        string            `json:"type"`
	DB          DBConfig          `json:"db"`
	Redis       redisConnConfig   `json:"redis"`
	Config      SourceConfig      `json:"config"`
	RedisConfig redisSourceConfig `json:"redis_config"`
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
	Redis       redisConnConfig `json:"redis"`
	Config      SinkConfig      `json:"config"`
	RedisConfig redisSinkConfig `json:"redis_config"`
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
	c.Source.Config.withDefaults()
	c.Sink.Config.withDefaults()
	c.Source.RedisConfig.withDefaults()
	c.Sink.RedisConfig.withDefaults()
}

// RunFlow executes source -> transform -> sink defined by FlowConfig.
func RunFlow(ctx context.Context, cfg FlowConfig) error {
	cfg.withDefaults()
	if err := ValidateFlowConfig(cfg); err != nil {
		return err
	}

	// Apply transform params as environment variables during this flow run.
	restore := make(map[string]*string, len(cfg.Transform.Params))
	for k, v := range cfg.Transform.Params {
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

	pluginPath, err := resolveTransformPlugin(cfg.Transform)
	if err != nil {
		return err
	}

	var files []string
	switch cfg.Source.Type {
	case "mysql":
		sourceDB, err := openDB(ctx, cfg.Source.DB)
		if err != nil {
			return err
		}
		defer sourceDB.Close()
		files, err = ExportSourceByPKRange(ctx, sourceDB, cfg.Source.Config)
		if err != nil {
			return err
		}
	case "redis":
		files, err = ExportSourceFromRedis(ctx, cfg.Source.Redis, cfg.Source.RedisConfig)
		if err != nil {
			return err
		}
	}
	if len(files) == 0 {
		return nil
	}

	// cleanup old reduce outputs before a new run.
	inputGlob := cfg.Sink.Config.InputGlob
	if cfg.Sink.Type == "redis" {
		inputGlob = cfg.Sink.RedisConfig.InputGlob
	}
	if outs, err := filepath.Glob(inputGlob); err == nil {
		for _, out := range outs {
			_ = os.Remove(out)
		}
	}

	mapreduce.MasterIP = ":" + strconv.Itoa(cfg.Transform.Port)
	mapreduce.StartSingleMachineJob(files, pluginPath, cfg.Transform.Reducers, cfg.Transform.Workers, cfg.Transform.InRAM)

	switch cfg.Sink.Type {
	case "mysql":
		sinkDB, err := openDB(ctx, cfg.Sink.DB)
		if err != nil {
			return err
		}
		defer sinkDB.Close()
		return ImportReduceOutputs(ctx, sinkDB, cfg.Sink.Config)
	case "redis":
		return ImportReduceOutputsToRedis(ctx, cfg.Sink.Redis, cfg.Sink.RedisConfig)
	default:
		return nil
	}
}
