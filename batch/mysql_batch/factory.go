package mysql_batch

import (
	"context"
	"database/sql"
)

type SourceAdapter struct {
	cfg SourceConfig
}

func NewSourceAdapter(cfg SourceConfig) SourceAdapter {
	return SourceAdapter{cfg: cfg}
}

func (a SourceAdapter) Export(ctx context.Context, db *sql.DB) ([]string, error) {
	return ExportSourceByPKRange(ctx, db, a.cfg)
}

type SinkAdapter struct {
	cfg SinkConfig
}

func NewSinkAdapter(cfg SinkConfig) SinkAdapter {
	return SinkAdapter{cfg: cfg}
}

func (a SinkAdapter) InputGlob() string {
	cfg := a.cfg
	cfg.WithDefaults()
	return cfg.InputGlob
}

func (a SinkAdapter) Import(ctx context.Context, db *sql.DB) error {
	return ImportReduceOutputs(ctx, db, a.cfg)
}
