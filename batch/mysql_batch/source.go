package mysql_batch

import (
	"bufio"
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// ExportSourceByPKRange exports source rows into shard text files.
// Each output line format is: id\tbiz_key\tmetric
func ExportSourceByPKRange(ctx context.Context, db *sql.DB, cfg SourceConfig) ([]string, error) {
	cfg.WithDefaults()
	if cfg.Table == "" {
		return nil, fmt.Errorf("source table is required")
	}

	table, err := quoteIdentifier(cfg.Table)
	if err != nil {
		return nil, err
	}
	pk, err := quoteIdentifier(cfg.PKColumn)
	if err != nil {
		return nil, err
	}
	key, err := quoteIdentifier(cfg.KeyColumn)
	if err != nil {
		return nil, err
	}
	val, err := quoteIdentifier(cfg.ValColumn)
	if err != nil {
		return nil, err
	}

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

	boundsSQL := fmt.Sprintf("SELECT COALESCE(MIN(%s),0), COALESCE(MAX(%s),0), COUNT(*) FROM %s WHERE %s", pk, pk, table, cfg.Where)
	var minID, maxID, rowCount int64
	if err := db.QueryRowContext(ctx, boundsSQL).Scan(&minID, &maxID, &rowCount); err != nil {
		return nil, err
	}
	if rowCount == 0 {
		return []string{}, nil
	}

	span := maxID - minID + 1
	step := (span + int64(cfg.Shards) - 1) / int64(cfg.Shards)
	if step < 1 {
		step = 1
	}

	type shardTask struct {
		start int64
		end   int64
		file  string
	}
	tasks := make([]shardTask, 0, cfg.Shards)
	for i := 0; i < cfg.Shards; i++ {
		start := minID + int64(i)*step
		if start > maxID {
			break
		}
		end := start + step
		file := filepath.Join(cfg.OutputDir, fmt.Sprintf("%s-%05d.txt", cfg.FilePrefix, i))
		tasks = append(tasks, shardTask{start: start, end: end, file: file})
	}

	workerN := cfg.Parallel
	if workerN > len(tasks) {
		workerN = len(tasks)
	}
	if workerN < 1 {
		workerN = 1
	}

	jobs := make(chan shardTask)
	errCh := make(chan error, 1)
	var wg sync.WaitGroup

	querySQL := fmt.Sprintf("SELECT %s, %s, %s FROM %s WHERE %s >= ? AND %s < ? AND %s ORDER BY %s", pk, key, val, table, pk, pk, cfg.Where, pk)
	for i := 0; i < workerN; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for task := range jobs {
				if err := exportOneShard(ctx, db, querySQL, task.start, task.end, task.file); err != nil {
					select {
					case errCh <- err:
					default:
					}
					return
				}
			}
		}()
	}

	for _, task := range tasks {
		select {
		case err := <-errCh:
			close(jobs)
			wg.Wait()
			return nil, err
		default:
		}
		jobs <- task
	}
	close(jobs)
	wg.Wait()

	select {
	case err := <-errCh:
		return nil, err
	default:
	}

	out := make([]string, 0, len(tasks))
	for _, task := range tasks {
		out = append(out, task.file)
	}
	return out, nil
}

func exportOneShard(ctx context.Context, db *sql.DB, querySQL string, start int64, end int64, outFile string) error {
	rows, err := db.QueryContext(ctx, querySQL, start, end)
	if err != nil {
		return err
	}
	defer rows.Close()

	f, err := os.Create(outFile)
	if err != nil {
		return err
	}
	defer f.Close()

	w := bufio.NewWriterSize(f, 1<<20)
	defer w.Flush()

	for rows.Next() {
		var c1, c2, c3 interface{}
		if err := rows.Scan(&c1, &c2, &c3); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(w, "%s\t%s\t%s\n", asString(c1), asString(c2), asString(c3)); err != nil {
			return err
		}
	}
	return rows.Err()
}
