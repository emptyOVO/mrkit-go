package redis_batch

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type SinkConfig struct {
	KeyPrefix  string `json:"key_prefix"`
	ValueField string `json:"value_field"`
	InputGlob  string `json:"inputglob"`
	Replace    bool   `json:"replace"`
}

func (c *SinkConfig) WithDefaults() {
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

func ImportReduceOutputs(ctx context.Context, connCfg ConnConfig, cfg SinkConfig) error {
	cfg.WithDefaults()
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
			cursor = toString(arr[0])
			keysRaw, ok := arr[1].([]interface{})
			if !ok {
				return fmt.Errorf("unexpected SCAN keys response")
			}
			for _, kv := range keysRaw {
				k := toString(kv)
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
