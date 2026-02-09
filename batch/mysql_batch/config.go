package mysql_batch

import (
	"fmt"
	"regexp"
)

var identifierRe = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*$`)

// SourceConfig configures source export from MySQL table to text shards.
type SourceConfig struct {
	Table      string `json:"table"`
	PKColumn   string `json:"pkcolumn"`
	KeyColumn  string `json:"keycolumn"`
	ValColumn  string `json:"valcolumn"`
	Where      string `json:"where"`
	Shards     int    `json:"shards"`
	Parallel   int    `json:"parallel"`
	OutputDir  string `json:"outputdir"`
	FilePrefix string `json:"fileprefix"`
}

func (c *SourceConfig) WithDefaults() {
	if c.PKColumn == "" {
		c.PKColumn = "id"
	}
	if c.KeyColumn == "" {
		c.KeyColumn = "biz_key"
	}
	if c.ValColumn == "" {
		c.ValColumn = "metric"
	}
	if c.Where == "" {
		c.Where = "1=1"
	}
	if c.Shards <= 0 {
		c.Shards = 16
	}
	if c.Parallel <= 0 {
		c.Parallel = 4
	}
	if c.OutputDir == "" {
		c.OutputDir = "txt/mysql_source"
	}
	if c.FilePrefix == "" {
		c.FilePrefix = "chunk"
	}
}

// SinkConfig configures reduce output import into MySQL.
type SinkConfig struct {
	TargetTable string `json:"targettable"`
	KeyColumn   string `json:"keycolumn"`
	ValColumn   string `json:"valcolumn"`
	InputGlob   string `json:"inputglob"`
	Replace     bool   `json:"replace"`
	BatchSize   int    `json:"batchsize"`
}

func (c *SinkConfig) WithDefaults() {
	if c.KeyColumn == "" {
		c.KeyColumn = "biz_key"
	}
	if c.ValColumn == "" {
		c.ValColumn = "metric_sum"
	}
	if c.InputGlob == "" {
		c.InputGlob = "mr-out-*.txt"
	}
	if c.BatchSize <= 0 {
		c.BatchSize = 2000
	}
}

func quoteIdentifier(s string) (string, error) {
	if !identifierRe.MatchString(s) {
		return "", fmt.Errorf("invalid identifier: %s", s)
	}
	return "`" + s + "`", nil
}

func asString(v interface{}) string {
	switch t := v.(type) {
	case nil:
		return ""
	case []byte:
		return string(t)
	default:
		return fmt.Sprint(t)
	}
}
