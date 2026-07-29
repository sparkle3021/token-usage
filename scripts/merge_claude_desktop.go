// Merge "claude-desktop" source data into "Claude Code" across all tables.
// After merging, deletes stale "claude-desktop" rows and parse cache entries.
//
// Usage:   go run scripts/merge_claude_desktop.go [db_path]
// Default: ~/.token-usage/td.db

package main

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

func main() {
	dbPath := ""
	if home, err := os.UserHomeDir(); err == nil {
		dbPath = filepath.Join(home, ".token-usage", "td.db")
	}
	if len(os.Args) > 1 {
		dbPath = os.Args[1]
	}
	if dbPath == "" {
		log.Fatal("请指定数据库路径: go run scripts/merge_claude_desktop.go <db_path>")
	}
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		log.Fatalf("数据库不存在: %s", dbPath)
	}

	db, err := sql.Open("sqlite", dbPath+"?_txlock=immediate")
	if err != nil {
		log.Fatalf("无法打开数据库: %v", err)
	}
	defer db.Close()

	count := countDesktopRows(db)
	if count == 0 {
		fmt.Println("✅ 没有找到 'claude-desktop' 数据，无需合并。")
		return
	}
	fmt.Printf("找到 %d 条 'claude-desktop' 记录，开始合并...\n\n", count)

	tx, err := db.Begin()
	if err != nil {
		log.Fatalf("无法开启事务: %v", err)
	}
	rollback := true
	defer func() {
		if rollback {
			tx.Rollback()
			fmt.Println("\n⚠️ 事务已回滚，数据库未修改。")
		}
	}()

	// 1. daily_usage — PK (device, source, usage_date, model)
	//    Merge "claude-desktop" into "Claude Code" by summing matching rows
	fmt.Println("[1/6] 合并 daily_usage ...")
	mergeDaily(tx)

	// 2. hour_usage — PK (device, source, usage_date, hour, model)
	fmt.Println("[2/6] 合并 hour_usage ...")
	mergeHourly(tx)

	// 3. time_usage — PK (device, source, event_key), event_key is globally unique
	//    No merge risk, simple rename
	fmt.Println("[3/6] 重命名 time_usage ...")
	execOrWarn(tx, "UPDATE time_usage SET source = 'Claude Code' WHERE source = 'claude-desktop'")

	// 4. session_usage — PK (device, source, session_id)
	//    Only rename; if duplicate PK exist, skip those (keep existing Claude Code rows)
	fmt.Println("[4/6] 重命名 session_usage ...")
	renameSessions(tx)

	// 5. collection_runs — no PK conflict
	fmt.Println("[5/6] 重命名 collection_runs ...")
	execOrWarn(tx, "UPDATE collection_runs SET source = 'Claude Code' WHERE source = 'claude-desktop'")

	// 6. parse_cache — PK (source, file_path). Stale entries can be deleted.
	fmt.Println("[6/6] 清理 parse_cache ...")
	execOrWarn(tx, "DELETE FROM parse_cache WHERE source = 'claude-desktop'")

	if err := tx.Commit(); err != nil {
		log.Fatalf("事务提交失败: %v", err)
	}
	rollback = false

	remaining := countDesktopRows(db)
	if remaining > 0 {
		fmt.Printf("\n⚠️ 仍有 %d 条残留记录（可能是 session 重复键跳过），可再次运行本脚本清理。\n", remaining)
	} else {
		fmt.Println("\n✅ 合并完成，所有 'claude-desktop' 数据已迁移！")
	}
}

// ── helpers ──

func countDesktopRows(db *sql.DB) int {
	var total int
	db.QueryRow(`
		SELECT
			(SELECT COUNT(*) FROM daily_usage WHERE source='claude-desktop') +
			(SELECT COUNT(*) FROM hour_usage WHERE source='claude-desktop') +
			(SELECT COUNT(*) FROM time_usage WHERE source='claude-desktop') +
			(SELECT COUNT(*) FROM session_usage WHERE source='claude-desktop') +
			(SELECT COUNT(*) FROM collection_runs WHERE source='claude-desktop') +
			(SELECT COUNT(*) FROM parse_cache WHERE source='claude-desktop')
	`).Scan(&total)
	return total
}

func mergeDaily(tx *sql.Tx) {
	// Copy "claude-desktop" daily rows into a temp table, then merge into "Claude Code" rows
	// by summing input/output/cache/reasoning tokens and cost.
	tx.Exec(`
		CREATE TEMP TABLE IF NOT EXISTS _merge_daily AS
		SELECT device, usage_date, model,
			SUM(input_tokens)     AS input_tokens,
			SUM(output_tokens)    AS output_tokens,
			SUM(cache_read_tokens)     AS cache_read_tokens,
			SUM(cache_creation_tokens) AS cache_creation_tokens,
			SUM(reasoning_output_tokens) AS reasoning_output_tokens,
			SUM(total_tokens)     AS total_tokens,
			SUM(cost_usd)         AS cost_usd
		FROM daily_usage
		WHERE source = 'claude-desktop'
		GROUP BY device, usage_date, model
	`)
	// Upsert: for each merged row, add values to the matching "Claude Code" row
	tx.Exec(`
		INSERT INTO daily_usage (device, source, usage_date, model,
			input_tokens, output_tokens, cache_read_tokens, cache_creation_tokens,
			reasoning_output_tokens, total_tokens, cost_usd)
		SELECT m.device, 'Claude Code', m.usage_date, m.model,
			m.input_tokens, m.output_tokens, m.cache_read_tokens, m.cache_creation_tokens,
			m.reasoning_output_tokens, m.total_tokens, m.cost_usd
		FROM _merge_daily m
		ON CONFLICT(device, source, usage_date, model) DO UPDATE SET
			input_tokens     = input_tokens     + excluded.input_tokens,
			output_tokens    = output_tokens    + excluded.output_tokens,
			cache_read_tokens     = cache_read_tokens     + excluded.cache_read_tokens,
			cache_creation_tokens = cache_creation_tokens + excluded.cache_creation_tokens,
			reasoning_output_tokens = reasoning_output_tokens + excluded.reasoning_output_tokens,
			total_tokens     = total_tokens     + excluded.total_tokens,
			cost_usd         = cost_usd         + excluded.cost_usd
	`)
	execOrWarn(tx, "DELETE FROM daily_usage WHERE source = 'claude-desktop'")
	tx.Exec("DROP TABLE IF EXISTS _merge_daily")
	var cnt int
	tx.QueryRow("SELECT COUNT(*) FROM daily_usage WHERE source = 'Claude Code'").Scan(&cnt)
	fmt.Printf("   daily_usage: Claude Code 合并后共 %d 行\n", cnt)
}

func mergeHourly(tx *sql.Tx) {
	tx.Exec(`
		CREATE TEMP TABLE IF NOT EXISTS _merge_hour AS
		SELECT device, usage_date, hour, model,
			SUM(input_tokens)     AS input_tokens,
			SUM(output_tokens)    AS output_tokens,
			SUM(cache_read_tokens)     AS cache_read_tokens,
			SUM(cache_creation_tokens) AS cache_creation_tokens,
			SUM(reasoning_output_tokens) AS reasoning_output_tokens,
			SUM(total_tokens)     AS total_tokens,
			SUM(cost_usd)         AS cost_usd
		FROM hour_usage
		WHERE source = 'claude-desktop'
		GROUP BY device, usage_date, hour, model
	`)
	tx.Exec(`
		INSERT INTO hour_usage (device, source, usage_date, hour, model,
			input_tokens, output_tokens, cache_read_tokens, cache_creation_tokens,
			reasoning_output_tokens, total_tokens, cost_usd)
		SELECT m.device, 'Claude Code', m.usage_date, m.hour, m.model,
			m.input_tokens, m.output_tokens, m.cache_read_tokens, m.cache_creation_tokens,
			m.reasoning_output_tokens, m.total_tokens, m.cost_usd
		FROM _merge_hour m
		ON CONFLICT(device, source, usage_date, hour, model) DO UPDATE SET
			input_tokens     = input_tokens     + excluded.input_tokens,
			output_tokens    = output_tokens    + excluded.output_tokens,
			cache_read_tokens     = cache_read_tokens     + excluded.cache_read_tokens,
			cache_creation_tokens = cache_creation_tokens + excluded.cache_creation_tokens,
			reasoning_output_tokens = reasoning_output_tokens + excluded.reasoning_output_tokens,
			total_tokens     = total_tokens     + excluded.total_tokens,
			cost_usd         = cost_usd         + excluded.cost_usd
	`)
	execOrWarn(tx, "DELETE FROM hour_usage WHERE source = 'claude-desktop'")
	tx.Exec("DROP TABLE IF EXISTS _merge_hour")
	var cnt int
	tx.QueryRow("SELECT COUNT(*) FROM hour_usage WHERE source = 'Claude Code'").Scan(&cnt)
	fmt.Printf("   hour_usage:  Claude Code 合并后共 %d 行\n", cnt)
}

func renameSessions(tx *sql.Tx) {
	// For sessions where Claude Code already has a row with the same PK, delete
	// the claude-desktop duplicate (keeping the Claude Code version).
	tx.Exec(`
		DELETE FROM session_usage
		WHERE source = 'claude-desktop'
		  AND (device, 'Claude Code', session_id) IN (
			SELECT device, 'Claude Code', session_id FROM session_usage WHERE source = 'Claude Code'
		)
	`)
	execOrWarn(tx, "UPDATE session_usage SET source = 'Claude Code' WHERE source = 'claude-desktop'")
}

func execOrWarn(tx *sql.Tx, q string) {
	result, err := tx.Exec(q)
	if err != nil {
		fmt.Printf("   ⚠️ %v\n", err)
		return
	}
	n, _ := result.RowsAffected()
	if n > 0 {
		fmt.Printf("   影响 %d 行\n", n)
	}
}
