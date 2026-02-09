package mysqlbatch

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type redisSourceConfig struct {
	KeyPattern string `json:"key_pattern"`
	KeyField   string `json:"key_field"`
	ValField   string `json:"val_field"`
	ScanCount  int    `json:"scan_count"`
	OutputDir  string `json:"outputdir"`
	FilePrefix string `json:"fileprefix"`
}

func (c *redisSourceConfig) withDefaults() {
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

type redisSinkConfig struct {
	KeyPrefix  string `json:"key_prefix"`
	ValueField string `json:"value_field"`
	InputGlob  string `json:"inputglob"`
	Replace    bool   `json:"replace"`
}

func (c *redisSinkConfig) withDefaults() {
	if c.KeyPrefix == "" {
		c.KeyPrefix = "mr:result:"
	}
	if c.ValueField == "" {
		c.ValueField = "metric_sum"
	}
	if c.InputGlob == "" {
		c.InputGlob = "mr-out-*.txt"
	}
}

func ExportSourceFromRedis(ctx context.Context, connCfg redisConnConfig, cfg redisSourceConfig) ([]string, error) {
	cfg.withDefaults()
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
		cursor = redisToString(arr[0])
		keysRaw, ok := arr[1].([]interface{})
		if !ok {
			return nil, fmt.Errorf("unexpected SCAN keys response")
		}
		for _, kv := range keysRaw {
			keyName := redisToString(kv)
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
			bizKey := redisToString(harr[0])
			if bizKey == "" {
				bizKey = keyName
			}
			metric := redisToString(harr[1])
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

func ImportReduceOutputsToRedis(ctx context.Context, connCfg redisConnConfig, cfg redisSinkConfig) error {
	cfg.withDefaults()
	_ = ctx
	c, err := openRedis(context.Background(), connCfg)
	if err != nil {
		return err
	}
	defer c.close()

	files, err := filepath.Glob(cfg.InputGlob)
	if err != nil {
		return err
	}
	if len(files) == 0 {
		return fmt.Errorf("no reduce output files matched: %s", cfg.InputGlob)
	}

	if cfg.Replace {
		cursor := "0"
		for {
			v, err := c.do("SCAN", cursor, "MATCH", cfg.KeyPrefix+"*", "COUNT", "1000")
			if err != nil {
				return err
			}
			arr, ok := v.([]interface{})
			if !ok || len(arr) != 2 {
				return fmt.Errorf("unexpected SCAN response")
			}
			cursor = redisToString(arr[0])
			keysRaw, ok := arr[1].([]interface{})
			if !ok {
				return fmt.Errorf("unexpected SCAN keys response")
			}
			for _, kv := range keysRaw {
				k := redisToString(kv)
				if k != "" {
					_, _ = c.do("DEL", k)
				}
			}
			if cursor == "0" {
				break
			}
		}
	}

	for _, file := range files {
		f, err := os.Open(file)
		if err != nil {
			return err
		}
		scanner := bufio.NewScanner(f)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line == "" {
				continue
			}
			fields := strings.Fields(line)
			if len(fields) < 2 {
				continue
			}
			key := cfg.KeyPrefix + fields[0]
			val := fields[1]
			if _, err := c.do("HSET", key, cfg.ValueField, val); err != nil {
				f.Close()
				return err
			}
		}
		if err := scanner.Err(); err != nil {
			f.Close()
			return err
		}
		f.Close()
	}
	return nil
}
