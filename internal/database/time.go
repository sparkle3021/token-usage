// Package database 的子文件，time_usage 表的 CRUD 操作。
package database

import (
	"database/sql"
	"log"
	"strings"
	"time"

	"token-dashboard/internal/model"
)

func (m *Manager) BulkUpsertTimeUsage(rows []model.TimeUsage) error {
	return m.bulkUpsertTimeUsageExec(m.db, rows)
}

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
			args = append(args, r.Device, r.Source, r.EventKey, r.EventTime, r.UsageDate,
				r.Model, nullIfEmpty(r.ProjectPath), nullIfEmpty(r.SessionID),
				r.InputTokens, r.OutputTokens, r.CacheCreationTokens, r.CacheReadTokens,
				r.ReasoningOutputTokens, r.TotalTokens, r.CostUSD)
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

func (m *Manager) QueryTimeUsage() ([]model.TimeUsage, error) {
	start := time.Now()
	rows, err := m.db.Query(`
		SELECT rowid, device, source, event_time, usage_date, model,
			COALESCE(project_path,''), COALESCE(session_id,''),
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
