package batch

import (
	"context"
	"database/sql"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/emptyOVO/mrkit-go/batch/mysql_batch"
	"github.com/emptyOVO/mrkit-go/batch/redis_batch"
	_ "github.com/go-sql-driver/mysql"
)

var identifierRe = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*$`)

// DBConfig defines MySQL connection parameters.
type DBConfig struct {
	Host     string
	Port     int
	User     string
	Password string
	Database string
	Params   map[string]string
}

func (c DBConfig) dsn() string {
	host := c.Host
	if host == "" {
		host = "127.0.0.1"
	}
	port := c.Port
	if port == 0 {
		port = 3306
	}
	params := map[string]string{
		"parseTime": "true",
		"charset":   "utf8mb4",
	}
	for k, v := range c.Params {
		params[k] = v
	}
	keys := make([]string, 0, len(params))
	for k := range params {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, fmt.Sprintf("%s=%s", k, params[k]))
	}
	return fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?%s",
		c.User,
		c.Password,
		host,
		port,
		c.Database,
		strings.Join(parts, "&"),
	)
}

func openDB(ctx context.Context, cfg DBConfig) (*sql.DB, error) {
	if cfg.User == "" {
		return nil, fmt.Errorf("db user is required")
	}
	if cfg.Database == "" {
		return nil, fmt.Errorf("db database is required")
	}
	db, err := sql.Open("mysql", cfg.dsn())
	if err != nil {
		return nil, err
	}
	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, err
	}
	return db, nil
}

// OpenForApp opens a MySQL connection for advanced/custom flows.
func OpenForApp(ctx context.Context, cfg DBConfig) (*sql.DB, error) {
	return openDB(ctx, cfg)
}

func quoteIdentifier(s string) (string, error) {
	if !identifierRe.MatchString(s) {
		return "", fmt.Errorf("invalid identifier: %s", s)
	}
	return "`" + s + "`", nil
}

// Unified source/sink config aliases exposed by batch package.
type SourceConfig = mysql_batch.SourceConfig
type SinkConfig = mysql_batch.SinkConfig
type RedisConnConfig = redis_batch.ConnConfig
type RedisSourceConfig = redis_batch.SourceConfig
type RedisSinkConfig = redis_batch.SinkConfig

// PipelineConfig describes end-to-end MySQL -> MapReduce -> MySQL job.
type PipelineConfig struct {
	DB         DBConfig // backward compatibility fallback when SourceDB/SinkDB are not set
	SourceDB   DBConfig
	SinkDB     DBConfig
	Source     SourceConfig
	Sink       SinkConfig
	PluginPath string
	Reducers   int
	Workers    int
	InRAM      bool
	Port       int
}

func (c *PipelineConfig) withDefaults() {
	if c.Reducers <= 0 {
		c.Reducers = 8
	}
	if c.Workers <= 0 {
		c.Workers = 16
	}
	if c.Port == 0 {
		c.Port = 10000
	}
	c.Source.WithDefaults()
	c.Sink.WithDefaults()
}

// PrepareConfig configures synthetic source table generation for benchmarking.
type PrepareConfig struct {
	SourceTable string
	Rows        int64
	KeyMod      int64
}

func (c *PrepareConfig) withDefaults() {
	if c.SourceTable == "" {
		c.SourceTable = "source_events"
	}
	if c.Rows <= 0 {
		c.Rows = 10000000
	}
	if c.KeyMod <= 0 {
		c.KeyMod = 100000
	}
}

// ValidateConfig compares source aggregation with target table.
type ValidateConfig struct {
	SourceTable string
	SourceKey   string
	SourceVal   string
	TargetTable string
	TargetKey   string
	TargetVal   string
}

func (c *ValidateConfig) withDefaults() {
	if c.SourceKey == "" {
		c.SourceKey = "biz_key"
	}
	if c.SourceVal == "" {
		c.SourceVal = "metric"
	}
	if c.TargetKey == "" {
		c.TargetKey = "biz_key"
	}
	if c.TargetVal == "" {
		c.TargetVal = "metric_sum"
	}
}
