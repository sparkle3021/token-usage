// Package database 提供 SQLite 数据库管理，包括连接初始化、Schema 迁移和按表拆分的 DAO。
// 所有导出方法接收 *Tilt 或 *sql.Tx，事务由调用方控制。
package database

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
)

type Manager struct {
	db *sql.DB

	stmtDaily   *sql.Stmt
	stmtSession *sql.Stmt
	stmtTime    *sql.Stmt
	stmtRecordRun *sql.Stmt
}

func New(dbPath string) (*Manager, error) {
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("create db directory: %w", err)
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}

	pragmas := []string{
		"PRAGMA busy_timeout = 10000",
		"PRAGMA journal_mode = WAL",
		"PRAGMA foreign_keys = ON",
		"PRAGMA synchronous = NORMAL",
		"PRAGMA cache_size = -65536",
		"PRAGMA temp_store = MEMORY",
		"PRAGMA mmap_size = 268435456",
	}
	for _, p := range pragmas {
		if _, err := db.Exec(p); err != nil {
			db.Close()
			return nil, fmt.Errorf("pragma %q: %w", p, err)
		}
	}

	m := &Manager{db: db}
	if err := m.initSchema(); err != nil {
		db.Close()
		return nil, fmt.Errorf("init schema: %w", err)
	}
	if err := m.initPreparedStmts(); err != nil {
		db.Close()
		return nil, fmt.Errorf("init prepared stmts: %w", err)
	}
	log.Printf("[db] New opened path=%s", dbPath)
	return m, nil
}

func (m *Manager) Close() error {
	if m.stmtDaily != nil {
		m.stmtDaily.Close()
	}
	if m.stmtSession != nil {
		m.stmtSession.Close()
	}
	if m.stmtTime != nil {
		m.stmtTime.Close()
	}
	if m.stmtRecordRun != nil {
		m.stmtRecordRun.Close()
	}
	return m.db.Close()
}

func (m *Manager) DB() *sql.DB {
	return m.db
}

func (m *Manager) Exec(query string, args ...any) (sql.Result, error) {
	start := time.Now()
	result, err := m.db.Exec(query, args...)
	if err != nil {
		log.Printf("[db] Exec error query=%s err=%v", truncateStr(query, 80), err)
	} else {
		n, _ := result.RowsAffected()
		log.Printf("[db] Exec ok rows=%d elapsed=%v", n, time.Since(start))
	}
	return result, err
}

func (m *Manager) initPreparedStmts() error {
	var err error

	m.stmtDaily, err = m.db.Prepare(`
		INSERT INTO daily_usage (
			device, source, usage_date, model,
			input_tokens, output_tokens, cache_creation_tokens, cache_read_tokens,
			reasoning_output_tokens, total_tokens, cost_usd, pricing_locked_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?,
			CASE WHEN ? < date('now', 'localtime') THEN datetime('now','localtime') ELSE NULL END,
			datetime('now','localtime')
		)
		ON CONFLICT(device, source, usage_date, model) DO UPDATE SET
			input_tokens = excluded.input_tokens,
			output_tokens = excluded.output_tokens,
			cache_creation_tokens = excluded.cache_creation_tokens,
			cache_read_tokens = excluded.cache_read_tokens,
			reasoning_output_tokens = excluded.reasoning_output_tokens,
			total_tokens = excluded.total_tokens,
			cost_usd = CASE
				WHEN daily_usage.usage_date < date('now', 'localtime') THEN daily_usage.cost_usd
				WHEN excluded.cost_usd = 0 THEN daily_usage.cost_usd
				ELSE excluded.cost_usd
			END,
			pricing_locked_at = CASE
				WHEN daily_usage.usage_date < date('now', 'localtime')
				THEN COALESCE(daily_usage.pricing_locked_at, datetime('now','localtime'))
				ELSE NULL
			END,
			updated_at = datetime('now','localtime')
	`)
	if err != nil {
		return fmt.Errorf("prepare upsertDaily: %w", err)
	}

	m.stmtSession, err = m.db.Prepare(`
		INSERT INTO session_usage (
			device, source, session_id, last_activity, project_path, model,
			input_tokens, output_tokens, cache_creation_tokens, cache_read_tokens,
			reasoning_output_tokens, total_tokens, cost_usd, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, datetime('now','localtime'))
		ON CONFLICT(device, source, session_id) DO UPDATE SET
			last_activity = excluded.last_activity,
			project_path = excluded.project_path,
			model = excluded.model,
			input_tokens = MAX(excluded.input_tokens, session_usage.input_tokens),
			output_tokens = MAX(excluded.output_tokens, session_usage.output_tokens),
			cache_creation_tokens = MAX(excluded.cache_creation_tokens, session_usage.cache_creation_tokens),
			cache_read_tokens = MAX(excluded.cache_read_tokens, session_usage.cache_read_tokens),
			reasoning_output_tokens = MAX(excluded.reasoning_output_tokens, session_usage.reasoning_output_tokens),
			total_tokens = MAX(excluded.total_tokens, session_usage.total_tokens),
			cost_usd = MAX(excluded.cost_usd, session_usage.cost_usd),
			updated_at = datetime('now','localtime')
	`)
	if err != nil {
		return fmt.Errorf("prepare upsertSession: %w", err)
	}

	m.stmtTime, err = m.db.Prepare(`
		INSERT INTO time_usage (
			device, source, event_key, event_time, usage_date, model, project_path, session_id,
			input_tokens, output_tokens, cache_creation_tokens, cache_read_tokens,
			reasoning_output_tokens, total_tokens, cost_usd, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, datetime('now','localtime'))
		ON CONFLICT(device, source, event_key) DO UPDATE SET
			event_time = excluded.event_time,
			usage_date = excluded.usage_date,
			model = excluded.model,
			project_path = excluded.project_path,
			session_id = excluded.session_id,
			input_tokens = excluded.input_tokens,
			output_tokens = excluded.output_tokens,
			cache_creation_tokens = excluded.cache_creation_tokens,
			cache_read_tokens = excluded.cache_read_tokens,
			reasoning_output_tokens = excluded.reasoning_output_tokens,
			total_tokens = excluded.total_tokens,
			cost_usd = excluded.cost_usd,
			updated_at = datetime('now','localtime')
	`)
	if err != nil {
		return fmt.Errorf("prepare upsertTime: %w", err)
	}

	m.stmtRecordRun, err = m.db.Prepare(`
		INSERT INTO collection_runs(device, source, status, message, collected_at, command)
		VALUES (?, ?, ?, ?, datetime('now'), ?)
	`)
	if err != nil {
		return fmt.Errorf("prepare recordRun: %w", err)
	}

	log.Printf("[db] initPreparedStmts ok (4 statements)")
	return nil
}

func (m *Manager) initSchema() error {
	schema := `
	CREATE TABLE IF NOT EXISTS collection_runs (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		device TEXT NOT NULL,
		source TEXT NOT NULL,
		status TEXT NOT NULL,
		message TEXT,
		collected_at TEXT NOT NULL DEFAULT (datetime('now','localtime')),
		command TEXT
	);

	CREATE TABLE IF NOT EXISTS daily_usage (
		device TEXT NOT NULL,
		source TEXT NOT NULL,
		usage_date TEXT NOT NULL,
		model TEXT NOT NULL DEFAULT '',
		input_tokens INTEGER NOT NULL DEFAULT 0,
		output_tokens INTEGER NOT NULL DEFAULT 0,
		cache_creation_tokens INTEGER NOT NULL DEFAULT 0,
		cache_read_tokens INTEGER NOT NULL DEFAULT 0,
		reasoning_output_tokens INTEGER NOT NULL DEFAULT 0,
		total_tokens INTEGER NOT NULL DEFAULT 0,
		cost_usd REAL NOT NULL DEFAULT 0,
		pricing_locked_at TEXT,
		updated_at TEXT NOT NULL DEFAULT (datetime('now','localtime')),
		PRIMARY KEY (device, source, usage_date, model)
	);

	CREATE TABLE IF NOT EXISTS session_usage (
		device TEXT NOT NULL,
		source TEXT NOT NULL,
		session_id TEXT NOT NULL,
		last_activity TEXT,
		project_path TEXT,
		input_tokens INTEGER NOT NULL DEFAULT 0,
		output_tokens INTEGER NOT NULL DEFAULT 0,
		cache_creation_tokens INTEGER NOT NULL DEFAULT 0,
		cache_read_tokens INTEGER NOT NULL DEFAULT 0,
		reasoning_output_tokens INTEGER NOT NULL DEFAULT 0,
		total_tokens INTEGER NOT NULL DEFAULT 0,
		cost_usd REAL NOT NULL DEFAULT 0,
		updated_at TEXT NOT NULL DEFAULT (datetime('now','localtime')),
		PRIMARY KEY (device, source, session_id)
	);

	CREATE TABLE IF NOT EXISTS time_usage (
		device TEXT NOT NULL,
		source TEXT NOT NULL,
		event_key TEXT NOT NULL,
		event_time TEXT NOT NULL,
		usage_date TEXT NOT NULL,
		model TEXT NOT NULL DEFAULT '',
		project_path TEXT,
		session_id TEXT,
		input_tokens INTEGER NOT NULL DEFAULT 0,
		output_tokens INTEGER NOT NULL DEFAULT 0,
		cache_creation_tokens INTEGER NOT NULL DEFAULT 0,
		cache_read_tokens INTEGER NOT NULL DEFAULT 0,
		reasoning_output_tokens INTEGER NOT NULL DEFAULT 0,
		total_tokens INTEGER NOT NULL DEFAULT 0,
		cost_usd REAL NOT NULL DEFAULT 0,
		updated_at TEXT NOT NULL DEFAULT (datetime('now','localtime')),
		PRIMARY KEY (device, source, event_key)
	);

	CREATE INDEX IF NOT EXISTS idx_daily_usage_date ON daily_usage(usage_date);
	CREATE INDEX IF NOT EXISTS idx_daily_usage_source ON daily_usage(source);
	CREATE INDEX IF NOT EXISTS idx_session_usage_total ON session_usage(total_tokens DESC);
	CREATE INDEX IF NOT EXISTS idx_time_usage_time ON time_usage(event_time);
	CREATE INDEX IF NOT EXISTS idx_time_usage_date_source ON time_usage(usage_date, source);

	CREATE TABLE IF NOT EXISTS app_config (
		key TEXT PRIMARY KEY,
		value TEXT NOT NULL,
		updated_at TEXT NOT NULL DEFAULT (datetime('now','localtime'))
	);

	CREATE TABLE IF NOT EXISTS parse_cache (
		source TEXT NOT NULL,
		file_path TEXT NOT NULL,
		fingerprint TEXT NOT NULL,
		records BLOB,
		updated_at TEXT NOT NULL DEFAULT (datetime('now','localtime')),
		PRIMARY KEY (source, file_path)
	);
	CREATE INDEX IF NOT EXISTS idx_parse_cache_source ON parse_cache(source);
	CREATE INDEX IF NOT EXISTS idx_parse_cache_updated ON parse_cache(updated_at);

	`
	if _, err := m.db.Exec(schema); err != nil {
		return fmt.Errorf("exec schema: %w", err)
	}

	if _, err := m.db.Exec(`
		UPDATE daily_usage
		SET pricing_locked_at = datetime('now','localtime')
		WHERE pricing_locked_at IS NULL
		  AND usage_date < date('now', 'localtime')
	`); err != nil {
		return fmt.Errorf("lock past pricing: %w", err)
	}

	if _, err := m.db.Exec(`CREATE TABLE IF NOT EXISTS hour_usage (
		device TEXT NOT NULL,
		source TEXT NOT NULL,
		usage_date TEXT NOT NULL,
		hour INTEGER NOT NULL,
		model TEXT NOT NULL DEFAULT '',
		input_tokens INTEGER NOT NULL DEFAULT 0,
		output_tokens INTEGER NOT NULL DEFAULT 0,
		cache_creation_tokens INTEGER NOT NULL DEFAULT 0,
		cache_read_tokens INTEGER NOT NULL DEFAULT 0,
		reasoning_output_tokens INTEGER NOT NULL DEFAULT 0,
		total_tokens INTEGER NOT NULL DEFAULT 0,
		cost_usd REAL NOT NULL DEFAULT 0,
		updated_at TEXT NOT NULL DEFAULT (datetime('now','localtime')),
		PRIMARY KEY (device, source, usage_date, hour, model)
	);
	CREATE INDEX IF NOT EXISTS idx_hour_usage_date_source ON hour_usage(usage_date, source);
	`); err != nil {
		return fmt.Errorf("create hour_usage table: %w", err)
	}

	m.db.Exec("ALTER TABLE collection_runs ADD COLUMN last_file_mtime INTEGER")
	m.db.Exec("ALTER TABLE parse_cache ADD COLUMN last_parsed_offset INTEGER NOT NULL DEFAULT 0")
	m.db.Exec("DELETE FROM app_config WHERE key IN ('cc_switch_enabled', 'cc_switch_auto_sync')")

	if err := m.migrateSessionModel(); err != nil {
		log.Printf("[db] migrateSessionModel: %v", err)
	}

	m.migrateTotalTokens()
	m.pruneCollectionRuns(500)
	return nil
}
