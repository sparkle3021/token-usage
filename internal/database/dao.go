// Package database 的子文件，包含公共 DAO 工具和辅助函数。
package database

import (
	"database/sql"
	"fmt"
	"log"

	"token-dashboard/internal/debuglog"
	"time"
)

type querier interface {
	Query(query string, args ...any) (*sql.Rows, error)
}

// preparedExecer 支持 Prepare + Exec，*sql.DB 与 *sql.Tx 均满足。
type preparedExecer interface {
	Prepare(query string) (*sql.Stmt, error)
	Exec(query string, args ...any) (sql.Result, error)
}

type batchExecer interface {
	preparedExecer
	querier
}

// bulkExecPrepared 复用单行 prepared statement 逐行写入。
// 相比旧的巨型多行 SQL（每次 Exec 重新 prepare 数千占位符），
// 此实现只 prepare 一次单行语句，大幅降低大批量写入开销。
// 实测 multi-row 批处理在 modernc.org/sqlite 上反而不如逐行快，故保留逐行。
func bulkExecPrepared[T any](ex preparedExecer, rows []T, sqlTemplate string, rowArgs func(T) []interface{}) error {
	if len(rows) == 0 {
		return nil
	}
	start := time.Now()
	stmt, err := ex.Prepare(sqlTemplate)
	if err != nil {
		return fmt.Errorf("prepare: %w", err)
	}
	defer stmt.Close()
	for i := range rows {
		if _, err := stmt.Exec(rowArgs(rows[i])...); err != nil {
			return fmt.Errorf("row %d: %w", i, err)
		}
	}
	debuglog.Perf("db bulkExecPrepared rows=%d elapsed=%v", len(rows), time.Since(start))
	return nil
}

func (m *Manager) ClearAllUsageData() error {
	start := time.Now()
	tx, err := m.db.Begin()
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	tables := []string{"daily_usage", "hour_usage", "time_usage", "session_usage", "parse_cache"}
	var total int64
	for _, table := range tables {
		result, err := tx.Exec(`DELETE FROM ` + table)
		if err != nil {
			return fmt.Errorf("delete %s: %w", table, err)
		}
		n, _ := result.RowsAffected()
		total += n
	}

	// Clear sync-state checkpoints (keep user configs: cc_switch_db_path, auto_sync_seconds, etc.)
	_, err = tx.Exec(`DELETE FROM app_config WHERE key LIKE 'cc_switch_cursor_%' OR key LIKE 'cc_switch_rollup_%'`)
	if err != nil {
		return fmt.Errorf("delete checkpoints: %w", err)
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
	return TruncateStr(s, 60)
}

// TruncateStr 截断字符串到 max 长度并追加省略号，用于日志输出的长字段截断。
// 供 database 及 orchestrator 等包共用（避免重复实现）。
func TruncateStr(s string, max int) string {
	if len(s) > max {
		return s[:max] + "..."
	}
	return s
}
