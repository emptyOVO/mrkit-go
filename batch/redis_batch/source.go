package redis_batch

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
)

type SourceConfig struct {
	KeyPattern string `json:"key_pattern"`
	KeyField   string `json:"key_field"`
	ValField   string `json:"val_field"`
	ScanCount  int    `json:"scan_count"`
	OutputDir  string `json:"outputdir"`
	FilePrefix string `json:"fileprefix"`
}

func (c *SourceConfig) WithDefaults() {
	if c.KeyPattern == "" {
		c.KeyPattern = "event:*"
	}
	if c.KeyField == "" {
		c.KeyField = "biz_key"
	}
	if c.ValField == "" {
		c.ValField = "metric"
	}
	if c.ScanCount <= 0 {
		c.ScanCount = 500
	}
	if c.OutputDir == "" {
		c.OutputDir = "txt/redis_source"
	}
	if c.FilePrefix == "" {
		c.FilePrefix = "chunk"
	}
}

func ExportSource(ctx context.Context, connCfg ConnConfig, cfg SourceConfig) ([]string, error) {
	cfg.WithDefaults()
	c, err := openRedis(ctx, connCfg)
	if err != nil {
		return nil, err
	}
	defer c.close()

	if err := os.MkdirAll(cfg.OutputDir, 0o755); err != nil {
		return nil, err
	}
	pattern := filepath.Join(cfg.OutputDir, cfg.FilePrefix+"-*.txt")
	oldFiles, err := filepath.Glob(pattern)
	if err != nil {
		return nil, err
	}
	for _, f := range oldFiles {
		_ = os.Remove(f)
	}

	outFile := filepath.Join(cfg.OutputDir, fmt.Sprintf("%s-%05d.txt", cfg.FilePrefix, 0))
	f, err := os.Create(outFile)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	w := bufio.NewWriterSize(f, 1<<20)
	defer w.Flush()

	cursor := "0"
	lineID := int64(1)
	for {
		v, err := c.do("SCAN", cursor, "MATCH", cfg.KeyPattern, "COUNT", strconv.Itoa(cfg.ScanCount))
		if err != nil {
			return nil, err
		}
		arr, ok := v.([]interface{})
		if !ok || len(arr) != 2 {
			return nil, fmt.Errorf("unexpected SCAN response")
		}
		cursor = toString(arr[0])
		keysRaw, ok := arr[1].([]interface{})
		if !ok {
			return nil, fmt.Errorf("unexpected SCAN keys response")
		}
		for _, kv := range keysRaw {
			keyName := toString(kv)
			if keyName == "" {
				continue
			}
			hv, err := c.do("HMGET", keyName, cfg.KeyField, cfg.ValField)
			if err != nil {
				continue
			}
			harr, ok := hv.([]interface{})
			if !ok || len(harr) < 2 {
				continue
			}
			bizKey := toString(harr[0])
			if bizKey == "" {
				bizKey = keyName
			}
			metric := toString(harr[1])
			if metric == "" {
				continue
			}
			if _, err := fmt.Fprintf(w, "%d\t%s\t%s\n", lineID, bizKey, metric); err != nil {
				return nil, err
			}
			lineID++
		}
		if cursor == "0" {
			break
		}
	}

	if lineID == 1 {
		return []string{}, nil
	}
	return []string{outFile}, nil
}
