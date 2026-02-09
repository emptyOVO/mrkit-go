package redis_batch

import "context"

type SourceAdapter struct {
	connCfg ConnConfig
	cfg     SourceConfig
}

func NewSourceAdapter(connCfg ConnConfig, cfg SourceConfig) SourceAdapter {
	return SourceAdapter{connCfg: connCfg, cfg: cfg}
}

func (a SourceAdapter) Export(ctx context.Context) ([]string, error) {
	return ExportSource(ctx, a.connCfg, a.cfg)
}

type SinkAdapter struct {
	connCfg ConnConfig
	cfg     SinkConfig
}

func NewSinkAdapter(connCfg ConnConfig, cfg SinkConfig) SinkAdapter {
	return SinkAdapter{connCfg: connCfg, cfg: cfg}
}

func (a SinkAdapter) InputGlob() string {
	cfg := a.cfg
	cfg.WithDefaults()
	return cfg.InputGlob
}

func (a SinkAdapter) Import(ctx context.Context) error {
	return ImportReduceOutputs(ctx, a.connCfg, a.cfg)
}
