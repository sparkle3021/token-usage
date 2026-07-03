// Package database 的子文件，包含公共 DAO 工具和辅助函数。
package database

import (
	"database/sql"
	"fmt"
	"log"
	"time"
)

const bulkBatchSize = 500

type execer interface {
	Exec(query string, args ...any) (sql.Result, error)
}

type querier interface {
	Query(query string, args ...any) (*sql.Rows, error)
}

type batchExecer interface {
	execer
	querier
}

func bulkExec[T any](ex execer, rows []T, batchSize int, build func([]T) (string, []interface{})) error {
	for i := 0; i < len(rows); i += batchSize {
		end := i + batchSize
		if end > len(rows) {
			end = len(rows)
		}
		sql, args := build(rows[i:end])
		if _, err := ex.Exec(sql, args...); err != nil {
			return fmt.Errorf("bulk batch %d: %w", i/batchSize, err)
		}
	}
	return nil
}

func (m *Manager) ClearAllUsageData() error {
	start := time.Now()
	tx, err := m.db.Begin()
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	tables := []string{"daily_usage", "hour_usage", "time_usage", "session_usage", "collection_runs", "parse_cache"}
	var total int64
	for _, table := range tables {
		result, err := tx.Exec(`DELETE FROM ` + table)
		if err != nil {
			return fmt.Errorf("delete %s: %w", table, err)
		}
		n, _ := result.RowsAffected()
		total += n
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit: %w", err)
	}

	log.Printf("[db] ClearAllUsageData ok deleted_total=%d elapsed=%v", total, time.Since(start))
	return nil
}

func (m *Manager) migrateTotalTokens() {
	for _, table := range []string{"daily_usage", "time_usage", "session_usage", "hour_usage"} {
		result, err := m.db.Exec(fmt.Sprintf(
			`UPDATE %s SET total_tokens = input_tokens + output_tokens + cache_creation_tokens + cache_read_tokens
			 WHERE reasoning_output_tokens > 0 AND total_tokens != input_tokens + output_tokens + cache_creation_tokens + cache_read_tokens`,
			table,
		))
		if err != nil {
			log.Printf("[db] migrateTotalTokens %s error: %v", table, err)
		} else if n, _ := result.RowsAffected(); n > 0 {
			log.Printf("[db] migrateTotalTokens %s fixed %d rows", table, n)
		}
	}
}

func nullIfEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func truncateID(s string) string {
	return truncateStr(s, 60)
}

func truncateStr(s string, max int) string {
	if len(s) > max {
		return s[:max] + "..."
	}
	return s
}
