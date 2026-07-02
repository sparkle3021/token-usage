package database

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"

	"token-dashboard/internal/model"
)

type Manager struct {
	db *sql.DB

	// Prepared statements cache
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

	// Performance and concurrency pragmas
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

	// Lock pricing for past dates that haven't been locked yet
	if _, err := m.db.Exec(`
		UPDATE daily_usage
		SET pricing_locked_at = datetime('now','localtime')
		WHERE pricing_locked_at IS NULL
		  AND usage_date < date('now', 'localtime')
	`); err != nil {
		return fmt.Errorf("lock past pricing: %w", err)
	}

		// Create hour_usage table (separate from main schema for backward compat)
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
	// Migration: add last_file_mtime to collection_runs for incremental collection
	m.db.Exec("ALTER TABLE collection_runs ADD COLUMN last_file_mtime INTEGER")

	// Migration: remove deprecated cc-switch config keys
	m.db.Exec("DELETE FROM app_config WHERE key IN ('cc_switch_enabled', 'cc_switch_auto_sync')")

	// Migration: add model to session_usage
	if err := m.migrateSessionModel(); err != nil {
		log.Printf("[db] migrateSessionModel: %v", err)
	}

	m.pruneCollectionRuns(500)
	return nil
}

// ---------------------------------------------------------------------------
// Bulk upsert (batch INSERT ... ON CONFLICT)
// ---------------------------------------------------------------------------

const bulkBatchSize = 500

// execer is satisfied by both *sql.DB and *sql.Tx.
type execer interface {
	Exec(query string, args ...any) (sql.Result, error)
}

// querier is satisfied by both *sql.DB and *sql.Tx.
type querier interface {
	Query(query string, args ...any) (*sql.Rows, error)
}

// batchExecer combines Exec and Query for operations that need both.
type batchExecer interface {
	execer
	querier
}

func (m *Manager) BulkUpsertDaily(rows []model.DailyUsage) error {
	if len(rows) == 0 {
		return nil
	}
	return bulkExec(m.db, rows, bulkBatchSize, func(batch []model.DailyUsage) (string, []interface{}) {
		var sqlBuf strings.Builder
		sqlBuf.WriteString(`INSERT INTO daily_usage (device,source,usage_date,model,
			input_tokens,output_tokens,cache_creation_tokens,cache_read_tokens,
			reasoning_output_tokens,total_tokens,cost_usd,pricing_locked_at,updated_at) VALUES `)
		var args []interface{}
		for i, r := range batch {
			if i > 0 {
				sqlBuf.WriteString(", ")
			}
			sqlBuf.WriteString("(?,?,?,?,?,?,?,?,?,?,?,")
			sqlBuf.WriteString("CASE WHEN ?<date('now','localtime') THEN datetime('now','localtime') ELSE NULL END,")
			sqlBuf.WriteString("datetime('now','localtime'))")
			args = append(args, r.Device,r.Source,r.UsageDate,r.Model,
				r.InputTokens,r.OutputTokens,r.CacheCreationTokens,r.CacheReadTokens,
				r.ReasoningOutputTokens,r.TotalTokens,r.CostUSD,r.UsageDate)
		}
		sqlBuf.WriteString(` ON CONFLICT(device,source,usage_date,model) DO UPDATE SET
			input_tokens=excluded.input_tokens, output_tokens=excluded.output_tokens,
			cache_creation_tokens=excluded.cache_creation_tokens, cache_read_tokens=excluded.cache_read_tokens,
			reasoning_output_tokens=excluded.reasoning_output_tokens, total_tokens=excluded.total_tokens,
			cost_usd=CASE WHEN daily_usage.usage_date<date('now','localtime') THEN daily_usage.cost_usd ELSE excluded.cost_usd END,
			pricing_locked_at=CASE WHEN daily_usage.usage_date<date('now','localtime') THEN COALESCE(daily_usage.pricing_locked_at,datetime('now','localtime')) ELSE NULL END,
			updated_at=datetime('now','localtime')`)
		return sqlBuf.String(), args
	})
}

func (m *Manager) BulkUpsertSession(rows []model.SessionUsage) error {
	return m.bulkUpsertSessionExec(m.db, rows)
}

// BulkUpsertSessionTx is like BulkUpsertSession but uses the given transaction.
func (m *Manager) BulkUpsertSessionTx(tx *sql.Tx, rows []model.SessionUsage) error {
	return m.bulkUpsertSessionExec(tx, rows)
}

func (m *Manager) bulkUpsertSessionExec(ex execer, rows []model.SessionUsage) error {
	if len(rows) == 0 {
		return nil
	}
	return bulkExec(ex, rows, bulkBatchSize, func(batch []model.SessionUsage) (string, []interface{}) {
		var sqlBuf strings.Builder
		sqlBuf.WriteString(`INSERT INTO session_usage (device,source,session_id,last_activity,project_path,
			input_tokens,output_tokens,cache_creation_tokens,cache_read_tokens,
			reasoning_output_tokens,total_tokens,cost_usd,updated_at) VALUES `)
		var args []interface{}
		for i, r := range batch {
			if i > 0 {
				sqlBuf.WriteString(", ")
			}
			sqlBuf.WriteString("(?,?,?,?,?,?,?,?,?,?,?,?,datetime('now','localtime'))")
			args = append(args, r.Device,r.Source,r.SessionID,
				nullIfEmpty(r.LastActivity),nullIfEmpty(r.ProjectPath),
				r.InputTokens,r.OutputTokens,r.CacheCreationTokens,r.CacheReadTokens,
				r.ReasoningOutputTokens,r.TotalTokens,r.CostUSD)
		}
		sqlBuf.WriteString(` ON CONFLICT(device,source,session_id) DO UPDATE SET
			last_activity=excluded.last_activity, project_path=excluded.project_path,
			input_tokens=excluded.input_tokens, output_tokens=excluded.output_tokens,
			cache_creation_tokens=excluded.cache_creation_tokens, cache_read_tokens=excluded.cache_read_tokens,
			reasoning_output_tokens=excluded.reasoning_output_tokens, total_tokens=excluded.total_tokens,
			cost_usd=excluded.cost_usd, updated_at=datetime('now','localtime')`)
		return sqlBuf.String(), args
	})
}

func (m *Manager) BulkUpsertTimeUsage(rows []model.TimeUsage) error {
	return m.bulkUpsertTimeUsageExec(m.db, rows)
}

// BulkUpsertTimeUsageTx is like BulkUpsertTimeUsage but uses the given transaction.
func (m *Manager) BulkUpsertTimeUsageTx(tx *sql.Tx, rows []model.TimeUsage) error {
	return m.bulkUpsertTimeUsageExec(tx, rows)
}

func (m *Manager) bulkUpsertTimeUsageExec(ex execer, rows []model.TimeUsage) error {
	if len(rows) == 0 {
		return nil
	}
	return bulkExec(ex, rows, bulkBatchSize, func(batch []model.TimeUsage) (string, []interface{}) {
		var sqlBuf strings.Builder
		sqlBuf.WriteString(`INSERT INTO time_usage (device,source,event_key,event_time,usage_date,
			model,project_path,session_id,input_tokens,output_tokens,cache_creation_tokens,
			cache_read_tokens,reasoning_output_tokens,total_tokens,cost_usd,updated_at) VALUES `)
		var args []interface{}
		for i, r := range batch {
			if i > 0 {
				sqlBuf.WriteString(", ")
			}
			sqlBuf.WriteString("(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,datetime('now','localtime'))")
			args = append(args, r.Device,r.Source,r.EventKey,r.EventTime,r.UsageDate,
				r.Model, nullIfEmpty(r.ProjectPath),nullIfEmpty(r.SessionID),
				r.InputTokens,r.OutputTokens,r.CacheCreationTokens,r.CacheReadTokens,
				r.ReasoningOutputTokens,r.TotalTokens,r.CostUSD)
		}
		sqlBuf.WriteString(` ON CONFLICT(device,source,event_key) DO UPDATE SET
			event_time=excluded.event_time, usage_date=excluded.usage_date,
			model=excluded.model, project_path=excluded.project_path, session_id=excluded.session_id,
			input_tokens=excluded.input_tokens, output_tokens=excluded.output_tokens,
			cache_creation_tokens=excluded.cache_creation_tokens, cache_read_tokens=excluded.cache_read_tokens,
			reasoning_output_tokens=excluded.reasoning_output_tokens, total_tokens=excluded.total_tokens,
			cost_usd=excluded.cost_usd, updated_at=datetime('now','localtime')`)
		return sqlBuf.String(), args
	})
}

// bulkExec splits rows into batches and builds + executes SQL for each batch.
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

// ---------------------------------------------------------------------------
// Daily usage
// ---------------------------------------------------------------------------

// BulkUpsertHourUsage inserts or updates hourly usage records with MAX semantics.
// When a record for the same key exists, each field is set to the larger of the
// existing and incoming value. This allows JSONL and CC-Switch data to coexist:
// whichever source observed more tokens for a given hour wins per field.
func (m *Manager) BulkUpsertHourUsage(rows []model.HourUsage) error {
	return m.bulkUpsertHourUsageExec(m.db, rows)
}

// BulkUpsertHourUsageTx is like BulkUpsertHourUsage but uses the given transaction.
func (m *Manager) BulkUpsertHourUsageTx(tx *sql.Tx, rows []model.HourUsage) error {
	return m.bulkUpsertHourUsageExec(tx, rows)
}

func (m *Manager) bulkUpsertHourUsageExec(ex execer, rows []model.HourUsage) error {
	if len(rows) == 0 {
		return nil
	}
	return bulkExec(ex, rows, bulkBatchSize, func(batch []model.HourUsage) (string, []interface{}) {
		var sqlBuf strings.Builder
		sqlBuf.WriteString(`INSERT INTO hour_usage (device,source,usage_date,hour,model,
			input_tokens,output_tokens,cache_creation_tokens,cache_read_tokens,
			reasoning_output_tokens,total_tokens,cost_usd,updated_at) VALUES `)
		var args []interface{}
		for i, r := range batch {
			if i > 0 {
				sqlBuf.WriteString(", ")
			}
			sqlBuf.WriteString("(?,?,?,?,?,?,?,?,?,?,?,?,datetime('now','localtime'))")
			args = append(args, r.Device, r.Source, r.UsageDate, r.Hour, r.Model,
				r.InputTokens, r.OutputTokens, r.CacheCreationTokens, r.CacheReadTokens,
				r.ReasoningOutputTokens, r.TotalTokens, r.CostUSD)
		}
		sqlBuf.WriteString(` ON CONFLICT(device,source,usage_date,hour,model) DO UPDATE SET
			input_tokens=MAX(excluded.input_tokens,hour_usage.input_tokens),
			output_tokens=MAX(excluded.output_tokens,hour_usage.output_tokens),
			cache_creation_tokens=MAX(excluded.cache_creation_tokens,hour_usage.cache_creation_tokens),
			cache_read_tokens=MAX(excluded.cache_read_tokens,hour_usage.cache_read_tokens),
			reasoning_output_tokens=MAX(excluded.reasoning_output_tokens,hour_usage.reasoning_output_tokens),
			total_tokens=MAX(excluded.total_tokens,hour_usage.total_tokens),
			cost_usd=MAX(excluded.cost_usd,hour_usage.cost_usd),
			updated_at=datetime('now','localtime')`)
		return sqlBuf.String(), args
	})
}

// BuildHourUsageFromTimeUsage rebuilds hour_usage from time_usage for a given source and date.
// Parses event_time in Go (ISO 8601 -> local time) to extract hour correctly,
// then aggregates into hour_usage with MAX semantics.
func (m *Manager) BuildHourUsageFromTimeUsage(device, source, date string) error {
	return m.buildHourUsageFromTimeUsageExec(m.db, device, source, date)
}

// BuildHourUsageFromTimeUsageTx is like BuildHourUsageFromTimeUsage but uses the given transaction.
func (m *Manager) BuildHourUsageFromTimeUsageTx(tx *sql.Tx, device, source, date string) error {
	return m.buildHourUsageFromTimeUsageExec(tx, device, source, date)
}

func (m *Manager) buildHourUsageFromTimeUsageExec(ex batchExecer, device, source, date string) error {
	rows, err := ex.Query(`SELECT device, source, usage_date, model,
		input_tokens, output_tokens, cache_creation_tokens, cache_read_tokens,
		reasoning_output_tokens, total_tokens, cost_usd, event_time
		FROM time_usage
		WHERE device = ? AND source = ? AND usage_date = ?`, device, source, date)
	if err != nil {
		return fmt.Errorf("query time_usage: %w", err)
	}
	defer rows.Close()

	type hourKey struct{ device, source, date, model string; hour int }
	acc := make(map[hourKey]*model.HourUsage)

	for rows.Next() {
		var dev, src, d, mdl string
		var inp, out, cc, cr, reas, total int64
		var cost float64
		var eventTime string
		if err := rows.Scan(&dev, &src, &d, &mdl,
			&inp, &out, &cc, &cr, &reas, &total, &cost, &eventTime); err != nil {
			continue
		}

		// Parse ISO 8601 UTC timestamp, convert to local time, extract hour
		hour := extractLocalHour(eventTime)
		if hour < 0 {
			continue
		}

		key := hourKey{device: dev, source: src, date: d, hour: hour, model: mdl}
		existing, ok := acc[key]
		if !ok {
			acc[key] = &model.HourUsage{
				Device: dev, Source: src, UsageDate: d, Hour: hour, Model: mdl,
			}
			existing = acc[key]
		}
		existing.InputTokens += inp
		existing.OutputTokens += out
		existing.CacheCreationTokens += cc
		existing.CacheReadTokens += cr
		existing.ReasoningOutputTokens += reas
		existing.TotalTokens += total
		existing.CostUSD += cost
	}

	if rows.Err() != nil {
		return fmt.Errorf("rows iteration: %w", rows.Err())
	}

	// Bulk upsert into hour_usage with MAX semantics
	var batch []model.HourUsage
	for _, row := range acc {
		batch = append(batch, *row)
	}
	if len(batch) > 0 {
		if err := m.bulkUpsertHourUsageExec(ex, batch); err != nil {
			return fmt.Errorf("upsert hour_usage: %w", err)
		}
	}

	log.Printf("[db] BuildHourUsageFromTimeUsage ok device=%s source=%s date=%s rows=%d hour_keys=%d", device, source, date, len(batch), len(batch))
	return nil
}

// extractLocalHour parses an ISO 8601 UTC timestamp string and returns the local hour (0-23).
// Returns -1 on parse failure.
func extractLocalHour(ts string) int {
	if ts == "" {
		return -1
	}
	// Try parsing ISO 8601 with timezone
	layouts := []string{
		time.RFC3339Nano,           // "2026-06-29T16:22:00.130Z"
		time.RFC3339,               // "2026-06-29T16:22:00Z"
		"2006-01-02T15:04:05",      // "2026-06-29T16:22:00" (no TZ)
		"2006-01-02 15:04:05",      // "2026-06-29 16:22:00"
	}
	var t time.Time
	var err error
	for _, layout := range layouts {
		t, err = time.Parse(layout, ts)
		if err == nil {
			break
		}
	}
	if err != nil {
		return -1
	}
	// Convert to local timezone and extract hour
	return t.Local().Hour()
}

// BuildDailyFromHourUsage rebuilds daily_usage from hour_usage.
// Should be called after all sources (JSONL collectors + CC-Switch import) have
// written their data to hour_usage for the target dates.
func (m *Manager) BuildDailyFromHourUsage() error {
	tx, err := m.db.Begin()
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	_, err = tx.Exec(`
		INSERT INTO daily_usage (device, source, usage_date, model,
			input_tokens, output_tokens, cache_creation_tokens, cache_read_tokens,
			reasoning_output_tokens, total_tokens, cost_usd, pricing_locked_at, updated_at)
		SELECT device, source, usage_date, model,
			SUM(input_tokens), SUM(output_tokens),
			SUM(cache_creation_tokens), SUM(cache_read_tokens),
			SUM(reasoning_output_tokens), SUM(total_tokens), SUM(cost_usd),
			NULL, datetime('now','localtime')
		FROM hour_usage
		GROUP BY device, source, usage_date, model
		ON CONFLICT(device, source, usage_date, model) DO UPDATE SET
			input_tokens=excluded.input_tokens,
			output_tokens=excluded.output_tokens,
			cache_creation_tokens=excluded.cache_creation_tokens,
			cache_read_tokens=excluded.cache_read_tokens,
			reasoning_output_tokens=excluded.reasoning_output_tokens,
			total_tokens=excluded.total_tokens,
			cost_usd=CASE
				WHEN daily_usage.usage_date < date('now','localtime') THEN daily_usage.cost_usd
				ELSE excluded.cost_usd
			END,
			pricing_locked_at=CASE
				WHEN daily_usage.usage_date < date('now','localtime')
				THEN COALESCE(daily_usage.pricing_locked_at, datetime('now','localtime'))
				ELSE NULL
			END,
			updated_at=datetime('now','localtime')
	`)
	if err != nil {
		return fmt.Errorf("build daily from hour_usage: %w", err)
	}

	return tx.Commit()
}



func (m *Manager) UpsertDaily(row *model.DailyUsage) error {
	start := time.Now()
	_, err := m.stmtDaily.Exec(
		row.Device, row.Source, row.UsageDate, row.Model,
		row.InputTokens, row.OutputTokens, row.CacheCreationTokens, row.CacheReadTokens,
		row.ReasoningOutputTokens, row.TotalTokens, row.CostUSD,
		row.UsageDate,
	)
	if err != nil {
		log.Printf("[db] UpsertDaily error source=%s date=%s model=%s err=%v", row.Source, row.UsageDate, row.Model, err)
	} else {
		log.Printf("[db] UpsertDaily ok source=%s date=%s model=%s total=%d elapsed=%v", row.Source, row.UsageDate, row.Model, row.TotalTokens, time.Since(start))
	}
	return err
}

// ---------------------------------------------------------------------------
// Session usage
// ---------------------------------------------------------------------------

func (m *Manager) UpsertSession(row *model.SessionUsage) error {
	start := time.Now()
	_, err := m.stmtSession.Exec(
		row.Device, row.Source, row.SessionID, row.LastActivity, row.ProjectPath,
		row.InputTokens, row.OutputTokens, row.CacheCreationTokens, row.CacheReadTokens,
		row.ReasoningOutputTokens, row.TotalTokens, row.CostUSD,
	)
	if err != nil {
		log.Printf("[db] UpsertSession error source=%s session=%s err=%v", row.Source, row.SessionID, err)
	} else {
		log.Printf("[db] UpsertSession ok source=%s session=%s total=%d elapsed=%v", row.Source, truncateID(row.SessionID), row.TotalTokens, time.Since(start))
	}
	return err
}

// ---------------------------------------------------------------------------
// Time usage
// ---------------------------------------------------------------------------

func (m *Manager) DeleteTimeUsageForSource(device, source string) error {
	start := time.Now()
	result, err := m.db.Exec(`DELETE FROM time_usage WHERE device = ? AND source = ?`, device, source)
	if err != nil {
		log.Printf("[db] DeleteTimeUsageForSource error source=%s err=%v", source, err)
	} else {
		n, _ := result.RowsAffected()
		log.Printf("[db] DeleteTimeUsageForSource ok source=%s deleted=%d elapsed=%v", source, n, time.Since(start))
	}
	return err
}

func (m *Manager) UpsertTimeUsage(row *model.TimeUsage) error {
	start := time.Now()
	_, err := m.stmtTime.Exec(
		row.Device, row.Source, row.EventKey, row.EventTime, row.UsageDate,
		row.Model, nullIfEmpty(row.ProjectPath), nullIfEmpty(row.SessionID),
		row.InputTokens, row.OutputTokens, row.CacheCreationTokens, row.CacheReadTokens,
		row.ReasoningOutputTokens, row.TotalTokens, row.CostUSD,
	)
	if err != nil {
		log.Printf("[db] UpsertTimeUsage error source=%s key=%s err=%v", row.Source, truncateID(row.EventKey), err)
	} else {
		log.Printf("[db] UpsertTimeUsage ok source=%s key=%s total=%d elapsed=%v", row.Source, truncateID(row.EventKey), row.TotalTokens, time.Since(start))
	}
	return err
}

// ---------------------------------------------------------------------------
// Collection runs
// ---------------------------------------------------------------------------

func (m *Manager) RecordRun(device, source, status, message, command string) error {
	_, err := m.stmtRecordRun.Exec(device, source, status, nullIfEmpty(message), nullIfEmpty(command))
	if err != nil {
		log.Printf("[db] RecordRun error source=%s status=%s err=%v", source, status, err)
	} else {
		log.Printf("[db] RecordRun ok source=%s status=%s message=%s", source, status, truncateID(message))
	}
	return err
}

// RecordRunWithMtime records a collection run with the associated file mtime.
func (m *Manager) RecordRunWithMtime(device, source, status, message, command string, lastFileMtime int64) error {
	_, err := m.db.Exec(`
		INSERT INTO collection_runs(device, source, status, message, collected_at, command, last_file_mtime)
		VALUES (?, ?, ?, ?, datetime('now'), ?, ?)
	`, device, source, status, nullIfEmpty(message), nullIfEmpty(command), lastFileMtime)
	if err != nil {
		log.Printf("[db] RecordRunWithMtime error source=%s mtime=%d err=%v", source, lastFileMtime, err)
	} else {
		log.Printf("[db] RecordRunWithMtime ok source=%s mtime=%d message=%s", source, lastFileMtime, truncateID(message))
	}
	return err
}

// GetLastMtime returns the maximum last_file_mtime recorded for a (device, source).
// Returns 0 if no previous runs exist.
func (m *Manager) GetLastMtime(device, source string) (int64, error) {
	var mtime int64
	err := m.db.QueryRow(`
		SELECT COALESCE(MAX(last_file_mtime), 0) FROM collection_runs
		WHERE device = ? AND source = ? AND last_file_mtime IS NOT NULL
	`, device, source).Scan(&mtime)
	if err != nil {
		return 0, err
	}
	return mtime, nil
}

func (m *Manager) pruneCollectionRuns(keep int) {
	if keep <= 0 {
		keep = 500
	}
	result, err := m.db.Exec(`
		DELETE FROM collection_runs
		WHERE id NOT IN (
			SELECT id FROM collection_runs ORDER BY id DESC LIMIT ?
		)
	`, keep)
	if err != nil {
		log.Printf("[db] pruneCollectionRuns error keep=%d err=%v", keep, err)
	} else if n, _ := result.RowsAffected(); n > 0 {
		log.Printf("[db] pruneCollectionRuns pruned=%d keep=%d", n, keep)
	}
}

// ---------------------------------------------------------------------------
// Parse cache (persistent file-fingerprint cache)
// ---------------------------------------------------------------------------

func (m *Manager) UpsertParseCache(source, filePath, fingerprint string, records []byte) error {
	_, err := m.db.Exec(`
		INSERT INTO parse_cache(source, file_path, fingerprint, records, updated_at)
		VALUES (?, ?, ?, ?, datetime('now','localtime'))
		ON CONFLICT(source, file_path) DO UPDATE SET
			fingerprint = excluded.fingerprint,
			records = excluded.records,
			updated_at = datetime('now','localtime')
	`, source, filePath, fingerprint, records)
	return err
}

// UpsertParseCacheFingerprint is a light version that only stores the fingerprint.
func (m *Manager) UpsertParseCacheFingerprint(source, filePath, fingerprint string) error {
	_, err := m.db.Exec(`
		INSERT INTO parse_cache(source, file_path, fingerprint, records, updated_at)
		VALUES (?, ?, ?, NULL, datetime('now','localtime'))
		ON CONFLICT(source, file_path) DO UPDATE SET
			fingerprint = excluded.fingerprint,
			updated_at = datetime('now','localtime')
	`, source, filePath, fingerprint)
	return err
}

func (m *Manager) GetParseCache(source, filePath string) (fingerprint string, records []byte, ok bool) {
	err := m.db.QueryRow(`
		SELECT fingerprint, records FROM parse_cache
		WHERE source = ? AND file_path = ?
	`, source, filePath).Scan(&fingerprint, &records)
	if err != nil {
		return "", nil, false
	}
	return fingerprint, records, true
}

func (m *Manager) DeleteParseCacheBySource(source string) error {
	_, err := m.db.Exec(`DELETE FROM parse_cache WHERE source = ?`, source)
	return err
}

func (m *Manager) PruneParseCache(maxAgeDays, maxRows int) {
	if maxAgeDays <= 0 {
		maxAgeDays = 30
	}
	if maxRows <= 0 {
		maxRows = 1000
	}
	m.db.Exec(`DELETE FROM parse_cache WHERE updated_at < datetime('now', ?)`, fmt.Sprintf("-%d days", maxAgeDays))
	m.db.Exec(`DELETE FROM parse_cache WHERE rowid NOT IN (SELECT rowid FROM parse_cache ORDER BY updated_at DESC LIMIT ?)`, maxRows)
}

// ---------------------------------------------------------------------------
// App config (key-value store)
// ---------------------------------------------------------------------------

func (m *Manager) GetConfig(key string) (string, error) {
	var value string
	err := m.db.QueryRow(`SELECT value FROM app_config WHERE key = ?`, key).Scan(&value)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return value, err
}

// ---------------------------------------------------------------------------
// Checkpoint (incremental sync cursor via app_config)
// ---------------------------------------------------------------------------

const (
	// CCSwitchCursorProxyLogs is the checkpoint key for proxy_request_logs cursor (UNIX timestamp).
	CCSwitchCursorProxyLogs = "cc_switch_cursor_proxy_request_logs"
	// CCSwitchRollupMaxDate is the checkpoint key for usage_daily_rollups max synced date (ISO date).
	CCSwitchRollupMaxDate = "cc_switch_rollup_max_date"
)

// GetCheckpoint reads a checkpoint value by key. Returns empty string if not set.
func (m *Manager) GetCheckpoint(key string) (string, error) {
	return m.GetConfig(key)
}

// SetCheckpoint writes a checkpoint value by key.
func (m *Manager) SetCheckpoint(key, value string) error {
	return m.SetConfig(key, value)
}

// DeleteCheckpointsByPrefix deletes all app_config rows whose key starts with the given prefix.
func (m *Manager) DeleteCheckpointsByPrefix(prefix string) error {
	_, err := m.db.Exec(`DELETE FROM app_config WHERE key LIKE ?`, prefix+"%")
	return err
}

// ResetCCSwitchCheckpoints clears all CC-Switch related checkpoints, forcing a full re-sync.
func (m *Manager) ResetCCSwitchCheckpoints() error {
	return m.DeleteCheckpointsByPrefix("cc_switch_cursor_")
}

func (m *Manager) SetConfig(key, value string) error {
	_, err := m.db.Exec(`
		INSERT INTO app_config(key, value, updated_at) VALUES (?, ?, datetime('now','localtime'))
		ON CONFLICT(key) DO UPDATE SET value = excluded.value, updated_at = datetime('now','localtime')
	`, key, value)
	return err
}

// GetAllConfigs returns all config rows as a map.
func (m *Manager) GetAllConfigs() (map[string]string, error) {
	rows, err := m.db.Query(`SELECT key, value FROM app_config`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make(map[string]string)
	for rows.Next() {
		var k, v string
		if err := rows.Scan(&k, &v); err != nil {
			return nil, err
		}
		result[k] = v
	}
	return result, rows.Err()
}

// ---------------------------------------------------------------------------
// Queries
// ---------------------------------------------------------------------------

func (m *Manager) QueryDaily() ([]model.DailyUsage, error) {
	start := time.Now()
	rows, err := m.db.Query(`
		SELECT device, source, usage_date, model,
			input_tokens, output_tokens, cache_creation_tokens, cache_read_tokens,
			reasoning_output_tokens, total_tokens, cost_usd
		FROM daily_usage
		ORDER BY usage_date DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []model.DailyUsage
	for rows.Next() {
		var r model.DailyUsage
		if err := rows.Scan(&r.Device, &r.Source, &r.UsageDate, &r.Model,
			&r.InputTokens, &r.OutputTokens, &r.CacheCreationTokens, &r.CacheReadTokens,
			&r.ReasoningOutputTokens, &r.TotalTokens, &r.CostUSD,
	); err != nil {
			return nil, err
		}
		results = append(results, r)
	}
	log.Printf("[db] QueryDaily rows=%d elapsed=%v", len(results), time.Since(start))
	return results, rows.Err()
}


func (m *Manager) migrateSessionModel() error {
	// Step 1: add model column (no-op if already exists)
	m.db.Exec("ALTER TABLE session_usage ADD COLUMN model TEXT NOT NULL DEFAULT ''")
	// Step 2: check if PK already includes model
	var pkInfo string
	m.db.QueryRow("SELECT sql FROM sqlite_master WHERE type='table' AND name='session_usage'").Scan(&pkInfo)
	if strings.Contains(pkInfo, "session_id, model") {
		return nil
	}
	log.Printf("[db] session_usage model column already present")
	return nil
}

func (m *Manager) QuerySessions() ([]model.SessionUsage, error) {
	start := time.Now()
	rows, err := m.db.Query(`
		SELECT device, source, session_id, COALESCE(last_activity,''), COALESCE(project_path,''),
			COALESCE(model,''),
			input_tokens, output_tokens, cache_creation_tokens, cache_read_tokens,
			reasoning_output_tokens, total_tokens, cost_usd
		FROM session_usage
		ORDER BY total_tokens DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []model.SessionUsage
	for rows.Next() {
		var r model.SessionUsage
		if err := rows.Scan(&r.Device, &r.Source, &r.SessionID, &r.LastActivity, &r.ProjectPath, &r.Model,
			&r.InputTokens, &r.OutputTokens, &r.CacheCreationTokens, &r.CacheReadTokens,
			&r.ReasoningOutputTokens, &r.TotalTokens, &r.CostUSD,
	); err != nil {
			return nil, err
		}
		results = append(results, r)
	}
	log.Printf("[db] QuerySessions rows=%d elapsed=%v", len(results), time.Since(start))
	return results, rows.Err()
}

func (m *Manager) QueryRuns(limit int) ([]model.CollectionRun, error) {
	start := time.Now()
	rows, err := m.db.Query(`
		SELECT id, device, source, status, message, collected_at
		FROM collection_runs
		ORDER BY id DESC
		LIMIT ?
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []model.CollectionRun
	for rows.Next() {
		var r model.CollectionRun
		var collectedAt string
		if err := rows.Scan(&r.ID, &r.Device, &r.Source, &r.Status, &r.Message, &collectedAt); err != nil {
			return nil, err
		}
		r.CollectedAt = collectedAt
		results = append(results, r)
	}
	log.Printf("[db] QueryRuns rows=%d elapsed=%v", len(results), time.Since(start))
	return results, rows.Err()
}

func (m *Manager) QueryTimeUsage() ([]model.TimeUsage, error) {
	start := time.Now()
	rows, err := m.db.Query(`
		SELECT rowid, device, source, event_time, usage_date, model, project_path, session_id,
			input_tokens, output_tokens, cache_creation_tokens, cache_read_tokens,
			reasoning_output_tokens, total_tokens, cost_usd
		FROM time_usage
		ORDER BY event_time DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []model.TimeUsage
	for rows.Next() {
		var r model.TimeUsage
		var id int64
		if err := rows.Scan(&id, &r.Device, &r.Source, &r.EventTime, &r.UsageDate, &r.Model,
			&r.ProjectPath, &r.SessionID,
			&r.InputTokens, &r.OutputTokens, &r.CacheCreationTokens, &r.CacheReadTokens,
			&r.ReasoningOutputTokens, &r.TotalTokens, &r.CostUSD,
	); err != nil {
			return nil, err
		}
		results = append(results, r)
	}
	log.Printf("[db] QueryTimeUsage rows=%d elapsed=%v", len(results), time.Since(start))
	return results, rows.Err()
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

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

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
