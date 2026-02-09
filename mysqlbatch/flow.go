package mysqlbatch

import (
	"context"
	"fmt"
	"os"
)

// FlowConfig describes a SeaTunnel-like source/transform/sink pipeline.
// Current implementation supports:
// - source.type: "mysql"
// - transform.type: "mapreduce"
// - sink.type: "mysql"
type FlowConfig struct {
	Source    FlowSourceConfig    `json:"source"`
	Transform FlowTransformConfig `json:"transform"`
	Sink      FlowSinkConfig      `json:"sink"`
}

type FlowSourceConfig struct {
	Type   string       `json:"type"`
	DB     DBConfig     `json:"db"`
	Config SourceConfig `json:"config"`
}

type FlowTransformConfig struct {
	Type       string            `json:"type"`
	PluginPath string            `json:"plugin_path"`
	Reducers   int               `json:"reducers"`
	Workers    int               `json:"workers"`
	InRAM      bool              `json:"in_ram"`
	Port       int               `json:"port"`
	Params     map[string]string `json:"params"`
}

type FlowSinkConfig struct {
	Type   string     `json:"type"`
	DB     DBConfig   `json:"db"`
	Config SinkConfig `json:"config"`
}

func (c *FlowConfig) withDefaults() {
	if c.Source.Type == "" {
		c.Source.Type = "mysql"
	}
	if c.Transform.Type == "" {
		c.Transform.Type = "mapreduce"
	}
	if c.Sink.Type == "" {
		c.Sink.Type = "mysql"
	}
}

// RunFlow executes source -> transform -> sink defined by FlowConfig.
func RunFlow(ctx context.Context, cfg FlowConfig) error {
	cfg.withDefaults()

	if cfg.Source.Type != "mysql" {
		return fmt.Errorf("unsupported source.type: %s", cfg.Source.Type)
	}
	if cfg.Transform.Type != "mapreduce" {
		return fmt.Errorf("unsupported transform.type: %s", cfg.Transform.Type)
	}
	if cfg.Sink.Type != "mysql" {
		return fmt.Errorf("unsupported sink.type: %s", cfg.Sink.Type)
	}
	if cfg.Transform.PluginPath == "" {
		return fmt.Errorf("transform.plugin_path is required")
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

	return RunPipeline(ctx, PipelineConfig{
		SourceDB:   cfg.Source.DB,
		SinkDB:     cfg.Sink.DB,
		Source:     cfg.Source.Config,
		Sink:       cfg.Sink.Config,
		PluginPath: cfg.Transform.PluginPath,
		Reducers:   cfg.Transform.Reducers,
		Workers:    cfg.Transform.Workers,
		InRAM:      cfg.Transform.InRAM,
		Port:       cfg.Transform.Port,
	})
}
