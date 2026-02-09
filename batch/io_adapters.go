package batch

import (
	"context"
	"database/sql"

	"github.com/emptyOVO/mrkit-go/batch/mysql_batch"
	"github.com/emptyOVO/mrkit-go/batch/redis_batch"
)

func ExportSourceByPKRange(ctx context.Context, db *sql.DB, cfg SourceConfig) ([]string, error) {
	return mysql_batch.ExportSourceByPKRange(ctx, db, cfg)
}

func ImportReduceOutputs(ctx context.Context, db *sql.DB, cfg SinkConfig) error {
	return mysql_batch.ImportReduceOutputs(ctx, db, cfg)
}

func ExportSourceFromRedis(ctx context.Context, connCfg RedisConnConfig, cfg RedisSourceConfig) ([]string, error) {
	return redis_batch.ExportSource(ctx, connCfg, cfg)
}

func ImportReduceOutputsToRedis(ctx context.Context, connCfg RedisConnConfig, cfg RedisSinkConfig) error {
	return redis_batch.ImportReduceOutputs(ctx, connCfg, cfg)
}
