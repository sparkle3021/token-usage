package database

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"

	"token-dashboard/internal/model"
)

type Manager struct {
	db *sql.DB
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
	log.Printf("[db] New opened path=%s", dbPath)
	return m, nil
}

func (m *Manager) Close() error {
	return m.db.Close()
}

func (m *Manager) DB() *sql.DB {
	return m.db
}

func (m *Manager) initSchema() error {
	schema := `
	CREATE TABLE IF NOT EXISTS collection_runs (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		device TEXT NOT NULL,
		source TEXT NOT NULL,
		status TEXT NOT NULL,
		message TEXT,
		collected_at TEXT NOT NULL DEFAULT (datetime('now')),
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
		updated_at TEXT NOT NULL DEFAULT (datetime('now')),
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
		updated_at TEXT NOT NULL DEFAULT (datetime('now')),
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
		updated_at TEXT NOT NULL DEFAULT (datetime('now')),
		PRIMARY KEY (device, source, event_key)
	);

	CREATE INDEX IF NOT EXISTS idx_daily_usage_date ON daily_usage(usage_date);
	CREATE INDEX IF NOT EXISTS idx_daily_usage_source ON daily_usage(source);
	CREATE INDEX IF NOT EXISTS idx_session_usage_total ON session_usage(total_tokens DESC);
	CREATE INDEX IF NOT EXISTS idx_time_usage_time ON time_usage(event_time);
	CREATE INDEX IF NOT EXISTS idx_time_usage_date_source ON time_usage(usage_date, source);
	`
	if _, err := m.db.Exec(schema); err != nil {
		return fmt.Errorf("exec schema: %w", err)
	}

	// Lock pricing for past dates that haven't been locked yet
	if _, err := m.db.Exec(`
		UPDATE daily_usage
		SET pricing_locked_at = datetime('now')
		WHERE pricing_locked_at IS NULL
		  AND usage_date < date('now', 'localtime')
	`); err != nil {
		return fmt.Errorf("lock past pricing: %w", err)
	}

	m.pruneCollectionRuns(500)
	return nil
}

// ---------------------------------------------------------------------------
// Daily usage
// ---------------------------------------------------------------------------

func (m *Manager) UpsertDaily(row *model.DailyUsage) error {
	start := time.Now()
	_, err := m.db.Exec(`
		INSERT INTO daily_usage (
			device, source, usage_date, model,
			input_tokens, output_tokens, cache_creation_tokens, cache_read_tokens,
			reasoning_output_tokens, total_tokens, cost_usd, pricing_locked_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?,
			CASE WHEN ? < date('now', 'localtime') THEN datetime('now') ELSE NULL END,
			datetime('now')
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
				THEN COALESCE(daily_usage.pricing_locked_at, datetime('now'))
				ELSE NULL
			END,
			updated_at = datetime('now')
	`, row.Device, row.Source, row.UsageDate, row.Model,
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
	_, err := m.db.Exec(`
		INSERT INTO session_usage (
			device, source, session_id, last_activity, project_path,
			input_tokens, output_tokens, cache_creation_tokens, cache_read_tokens,
			reasoning_output_tokens, total_tokens, cost_usd, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, datetime('now'))
		ON CONFLICT(device, source, session_id) DO UPDATE SET
			last_activity = excluded.last_activity,
			project_path = excluded.project_path,
			input_tokens = excluded.input_tokens,
			output_tokens = excluded.output_tokens,
			cache_creation_tokens = excluded.cache_creation_tokens,
			cache_read_tokens = excluded.cache_read_tokens,
			reasoning_output_tokens = excluded.reasoning_output_tokens,
			total_tokens = excluded.total_tokens,
			cost_usd = excluded.cost_usd,
			updated_at = datetime('now')
	`, row.Device, row.Source, row.SessionID, row.LastActivity, row.ProjectPath,
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
	_, err := m.db.Exec(`
		INSERT INTO time_usage (
			device, source, event_key, event_time, usage_date, model, project_path, session_id,
			input_tokens, output_tokens, cache_creation_tokens, cache_read_tokens,
			reasoning_output_tokens, total_tokens, cost_usd, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, datetime('now'))
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
			updated_at = datetime('now')
	`, row.Device, row.Source, row.EventKey, row.EventTime, row.UsageDate,
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
	_, err := m.db.Exec(`
		INSERT INTO collection_runs(device, source, status, message, collected_at, command)
		VALUES (?, ?, ?, ?, datetime('now'), ?)
	`, device, source, status, nullIfEmpty(message), nullIfEmpty(command))
	if err != nil {
		log.Printf("[db] RecordRun error source=%s status=%s err=%v", source, status, err)
	} else {
		log.Printf("[db] RecordRun ok source=%s status=%s message=%s", source, status, truncateID(message))
	}
	return err
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

func (m *Manager) QuerySessions() ([]model.SessionUsage, error) {
	start := time.Now()
	rows, err := m.db.Query(`
		SELECT device, source, session_id, COALESCE(last_activity,''), COALESCE(project_path,''),
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
		if err := rows.Scan(&r.Device, &r.Source, &r.SessionID, &r.LastActivity, &r.ProjectPath,
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
