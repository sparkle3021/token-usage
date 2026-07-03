// Package database 的子文件，hour_usage 表的 CRUD 及时间聚合操作。
package database

import (
	"database/sql"
	"fmt"
	"log"
	"strings"
	"time"

	"token-dashboard/internal/model"
)

func (m *Manager) BulkUpsertHourUsage(rows []model.HourUsage) error {
	return m.bulkUpsertHourUsageExec(m.db, rows)
}

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

func (m *Manager) BuildHourUsageFromTimeUsage(device, source, date string) error {
	return m.buildHourUsageFromTimeUsageExec(m.db, device, source, date)
}

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

func extractLocalHour(ts string) int {
	if ts == "" {
		return -1
	}
	layouts := []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02T15:04:05",
		"2006-01-02 15:04:05",
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
	return t.Local().Hour()
}

func (m *Manager) QueryHourUsage() ([]model.HourUsage, error) {
	start := time.Now()
	rows, err := m.db.Query(`
		SELECT device, source, usage_date, hour, model,
			input_tokens, output_tokens, cache_creation_tokens, cache_read_tokens,
			reasoning_output_tokens, total_tokens, cost_usd
		FROM hour_usage
		ORDER BY usage_date DESC, hour ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []model.HourUsage
	for rows.Next() {
		var r model.HourUsage
		if err := rows.Scan(&r.Device, &r.Source, &r.UsageDate, &r.Hour, &r.Model,
			&r.InputTokens, &r.OutputTokens, &r.CacheCreationTokens, &r.CacheReadTokens,
			&r.ReasoningOutputTokens, &r.TotalTokens, &r.CostUSD,
		); err != nil {
			return nil, err
		}
		results = append(results, r)
	}
	log.Printf("[db] QueryHourUsage rows=%d elapsed=%v", len(results), time.Since(start))
	return results, rows.Err()
}
