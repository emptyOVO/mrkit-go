package mysql_batch

import (
	"bufio"
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// ImportReduceOutputs imports mr-out-* files into target table with batch upsert.
func ImportReduceOutputs(ctx context.Context, db *sql.DB, cfg SinkConfig) error {
	cfg.WithDefaults()
	if cfg.TargetTable == "" {
		return fmt.Errorf("target table is required")
	}

	table, err := quoteIdentifier(cfg.TargetTable)
	if err != nil {
		return err
	}
	keyCol, err := quoteIdentifier(cfg.KeyColumn)
	if err != nil {
		return err
	}
	valCol, err := quoteIdentifier(cfg.ValColumn)
	if err != nil {
		return err
	}

	files, err := filepath.Glob(cfg.InputGlob)
	if err != nil {
		return err
	}
	if len(files) == 0 {
		return fmt.Errorf("no reduce output files matched: %s", cfg.InputGlob)
	}

	stageName := cfg.TargetTable + "_staging_tmp"
	stageTable, err := quoteIdentifier(stageName)
	if err != nil {
		return err
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, fmt.Sprintf(`
CREATE TABLE IF NOT EXISTS %s (
  %s VARCHAR(255) NOT NULL,
  %s BIGINT NOT NULL,
  PRIMARY KEY (%s)
)`, table, keyCol, valCol, keyCol)); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, fmt.Sprintf(`DROP TABLE IF EXISTS %s`, stageTable)); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, fmt.Sprintf(`
CREATE TABLE %s (
  %s VARCHAR(255) NOT NULL,
  %s BIGINT NOT NULL,
  KEY idx_key (%s)
)`, stageTable, keyCol, valCol, keyCol)); err != nil {
		return err
	}

	if err := loadReduceFilesIntoStage(ctx, tx, files, stageTable, keyCol, valCol, cfg.BatchSize); err != nil {
		return err
	}

	if cfg.Replace {
		if _, err := tx.ExecContext(ctx, fmt.Sprintf(`TRUNCATE TABLE %s`, table)); err != nil {
			return err
		}
	}

	upsertSQL := fmt.Sprintf(`
INSERT INTO %s (%s, %s)
SELECT %s, SUM(%s) AS total
FROM %s
GROUP BY %s
ON DUPLICATE KEY UPDATE %s=VALUES(%s)
`, table, keyCol, valCol, keyCol, valCol, stageTable, keyCol, valCol, valCol)
	if _, err := tx.ExecContext(ctx, upsertSQL); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, fmt.Sprintf(`DROP TABLE %s`, stageTable)); err != nil {
		return err
	}

	return tx.Commit()
}

func loadReduceFilesIntoStage(ctx context.Context, tx *sql.Tx, files []string, stageTable string, keyCol string, valCol string, batchSize int) error {
	batch := make([][2]interface{}, 0, batchSize)
	flush := func() error {
		if len(batch) == 0 {
			return nil
		}
		args := make([]interface{}, 0, len(batch)*2)
		valueSQL := make([]string, 0, len(batch))
		for _, row := range batch {
			valueSQL = append(valueSQL, "(?, ?)")
			args = append(args, row[0], row[1])
		}
		sqlStr := fmt.Sprintf("INSERT INTO %s (%s, %s) VALUES %s", stageTable, keyCol, valCol, strings.Join(valueSQL, ","))
		if _, err := tx.ExecContext(ctx, sqlStr, args...); err != nil {
			return err
		}
		batch = batch[:0]
		return nil
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
			v, err := strconv.ParseInt(fields[1], 10, 64)
			if err != nil {
				continue
			}
			batch = append(batch, [2]interface{}{fields[0], v})
			if len(batch) >= batchSize {
				if err := flush(); err != nil {
					f.Close()
					return err
				}
			}
		}
		if err := scanner.Err(); err != nil {
			f.Close()
			return err
		}
		f.Close()
	}

	return flush()
}
