package batch

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

// PrepareSyntheticSource creates a synthetic source table for benchmark.
func PrepareSyntheticSource(ctx context.Context, db *sql.DB, cfg PrepareConfig) error {
	cfg.withDefaults()
	table, err := quoteIdentifier(cfg.SourceTable)
	if err != nil {
		return err
	}

	if _, err := db.ExecContext(ctx, fmt.Sprintf(`DROP TABLE IF EXISTS %s`, table)); err != nil {
		return err
	}
	if _, err := db.ExecContext(ctx, fmt.Sprintf(`
CREATE TABLE %s (
  id BIGINT NOT NULL,
  biz_key VARCHAR(64) NOT NULL,
  metric INT NOT NULL,
  PRIMARY KEY (id),
  KEY idx_biz_key (biz_key)
) ENGINE=InnoDB
`, table)); err != nil {
		return err
	}

	const batchSize int64 = 5000
	for start := int64(0); start < cfg.Rows; start += batchSize {
		end := start + batchSize
		if end > cfg.Rows {
			end = cfg.Rows
		}
		rowN := end - start

		placeholders := make([]string, 0, rowN)
		args := make([]interface{}, 0, rowN*3)
		for i := start; i < end; i++ {
			id := i + 1
			key := fmt.Sprintf("key_%05d", i%cfg.KeyMod)
			metric := 1 + (i % 5)
			placeholders = append(placeholders, "(?, ?, ?)")
			args = append(args, id, key, metric)
		}

		insertSQL := fmt.Sprintf(
			"INSERT INTO %s (id, biz_key, metric) VALUES %s",
			table,
			strings.Join(placeholders, ","),
		)
		if _, err := db.ExecContext(ctx, insertSQL, args...); err != nil {
			return err
		}
	}

	_, err = db.ExecContext(ctx, fmt.Sprintf(`ANALYZE TABLE %s`, table))
	return err
}

// ValidateAggregation checks source(group by) equals target table data.
func ValidateAggregation(ctx context.Context, db *sql.DB, cfg ValidateConfig) error {
	cfg.withDefaults()
	if cfg.SourceTable == "" || cfg.TargetTable == "" {
		return fmt.Errorf("source table and target table are required")
	}

	srcTable, err := quoteIdentifier(cfg.SourceTable)
	if err != nil {
		return err
	}
	srcKey, err := quoteIdentifier(cfg.SourceKey)
	if err != nil {
		return err
	}
	srcVal, err := quoteIdentifier(cfg.SourceVal)
	if err != nil {
		return err
	}
	tgtTable, err := quoteIdentifier(cfg.TargetTable)
	if err != nil {
		return err
	}
	tgtKey, err := quoteIdentifier(cfg.TargetKey)
	if err != nil {
		return err
	}
	tgtVal, err := quoteIdentifier(cfg.TargetVal)
	if err != nil {
		return err
	}

	expectedSQL := fmt.Sprintf(`
SELECT %s, SUM(%s) AS total
FROM %s
GROUP BY %s
ORDER BY %s`, srcKey, srcVal, srcTable, srcKey, srcKey)
	actualSQL := fmt.Sprintf(`
SELECT %s, %s
FROM %s
ORDER BY %s`, tgtKey, tgtVal, tgtTable, tgtKey)

	expectedRows, err := db.QueryContext(ctx, expectedSQL)
	if err != nil {
		return err
	}
	defer expectedRows.Close()

	actualRows, err := db.QueryContext(ctx, actualSQL)
	if err != nil {
		return err
	}
	defer actualRows.Close()

	idx := 0
	for {
		eNext := expectedRows.Next()
		aNext := actualRows.Next()
		if !eNext || !aNext {
			if eNext != aNext {
				return fmt.Errorf("row count mismatch in validation")
			}
			break
		}
		idx++
		var ek string
		var ev int64
		if err := expectedRows.Scan(&ek, &ev); err != nil {
			return err
		}
		var ak string
		var av int64
		if err := actualRows.Scan(&ak, &av); err != nil {
			return err
		}
		if ek != ak || ev != av {
			return fmt.Errorf("validation mismatch at row %d: expected (%s,%d), actual (%s,%d)", idx, ek, ev, ak, av)
		}
	}
	if err := expectedRows.Err(); err != nil {
		return err
	}
	if err := actualRows.Err(); err != nil {
		return err
	}
	return nil
}
