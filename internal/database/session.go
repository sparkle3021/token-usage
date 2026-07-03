// Package database 的子文件，session_usage 表的 CRUD 操作。
package database

import (
	"database/sql"
	"log"
	"strings"
	"time"

	"token-dashboard/internal/model"
)

func (m *Manager) BulkUpsertSession(rows []model.SessionUsage) error {
	return m.bulkUpsertSessionExec(m.db, rows)
}

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
			args = append(args, r.Device, r.Source, r.SessionID,
				nullIfEmpty(r.LastActivity), nullIfEmpty(r.ProjectPath),
				r.InputTokens, r.OutputTokens, r.CacheCreationTokens, r.CacheReadTokens,
				r.ReasoningOutputTokens, r.TotalTokens, r.CostUSD)
		}
		sqlBuf.WriteString(` ON CONFLICT(device,source,session_id) DO UPDATE SET
			last_activity=excluded.last_activity, project_path=excluded.project_path,
			input_tokens=MAX(excluded.input_tokens,session_usage.input_tokens), output_tokens=MAX(excluded.output_tokens,session_usage.output_tokens),
			cache_creation_tokens=MAX(excluded.cache_creation_tokens,session_usage.cache_creation_tokens), cache_read_tokens=MAX(excluded.cache_read_tokens,session_usage.cache_read_tokens),
			reasoning_output_tokens=MAX(excluded.reasoning_output_tokens,session_usage.reasoning_output_tokens), total_tokens=MAX(excluded.total_tokens,session_usage.total_tokens),
			cost_usd=MAX(excluded.cost_usd,session_usage.cost_usd), updated_at=datetime('now','localtime')`)
		return sqlBuf.String(), args
	})
}

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

func (m *Manager) migrateSessionModel() error {
	m.db.Exec("ALTER TABLE session_usage ADD COLUMN model TEXT NOT NULL DEFAULT ''")
	var pkInfo string
	m.db.QueryRow("SELECT sql FROM sqlite_master WHERE type='table' AND name='session_usage'").Scan(&pkInfo)
	if strings.Contains(pkInfo, "session_id, model") {
		return nil
	}
	log.Printf("[db] session_usage model column already present")
	return nil
}
