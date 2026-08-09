// Package database 的子文件，hour_usage 表的 CRUD 及时间聚合操作。
package database

import (
	"database/sql"
	"fmt"
	"log"
	"time"

	"token-dashboard/internal/model"
)

func (m *Manager) BulkUpsertHourUsage(rows []model.HourUsage) error {
	return m.bulkUpsertHourUsageExec(m.db, rows)
}

func (m *Manager) BulkUpsertHourUsageTx(tx *sql.Tx, rows []model.HourUsage) error {
	return m.bulkUpsertHourUsageExec(tx, rows)
}

func (m *Manager) bulkUpsertHourUsageExec(ex preparedExecer, rows []model.HourUsage) error {
	if len(rows) == 0 {
		return nil
	}
	return bulkExecPrepared(ex, rows, `
		INSERT INTO hour_usage (device,source,usage_date,hour,model,
			input_tokens,output_tokens,cache_creation_tokens,cache_read_tokens,
			reasoning_output_tokens,total_tokens,cost_usd,request_count,updated_at)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,datetime('now','localtime'))
		ON CONFLICT(device,source,usage_date,hour,model) DO UPDATE SET
			input_tokens=MAX(excluded.input_tokens,hour_usage.input_tokens),
			output_tokens=MAX(excluded.output_tokens,hour_usage.output_tokens),
			cache_creation_tokens=MAX(excluded.cache_creation_tokens,hour_usage.cache_creation_tokens),
			cache_read_tokens=MAX(excluded.cache_read_tokens,hour_usage.cache_read_tokens),
			reasoning_output_tokens=MAX(excluded.reasoning_output_tokens,hour_usage.reasoning_output_tokens),
			total_tokens=MAX(excluded.total_tokens,hour_usage.total_tokens),
			cost_usd=MAX(excluded.cost_usd,hour_usage.cost_usd),
			request_count=MAX(excluded.request_count,hour_usage.request_count),
			updated_at=datetime('now','localtime')`,
		func(r model.HourUsage) []interface{} {
			return []interface{}{r.Device, r.Source, r.UsageDate, r.Hour, r.Model,
				r.InputTokens, r.OutputTokens, r.CacheCreationTokens, r.CacheReadTokens,
				r.ReasoningOutputTokens, r.TotalTokens, r.CostUSD, r.RequestCount}
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

		hour := extractUTCHour(eventTime)
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
		existing.RequestCount++
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

// extractUTCHour 从时间串提取 UTC 小时；带时区串直接取 UTC，无时区串按本机时区解释后转 UTC。
// 返回 -1 表示解析失败。
func extractUTCHour(ts string) int {
	if ts == "" {
		return -1
	}
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339} {
		t, err := time.Parse(layout, ts)
		if err == nil {
			return t.UTC().Hour()
		}
	}
	for _, layout := range []string{"2006-01-02T15:04:05", "2006-01-02 15:04:05"} {
		t, err := time.ParseInLocation(layout, ts, time.Local)
		if err == nil {
			return t.UTC().Hour()
		}
	}
	return -1
}

// QueryHourUsage 返回 hour_usage 行；days>0 时仅包含最近 days 天（含当天），days<=0 返回全量。
// 窗口基准用 Go 本地时间（与 QueryTimeUsage 的 event_time 过滤同日口径），避免 SQLite date('now') 的 UTC 偏移。
func (m *Manager) QueryHourUsage(days int) ([]model.HourUsage, error) {
	start := time.Now()
	sqlText := `
		SELECT device, source, usage_date, hour, model,
			input_tokens, output_tokens, cache_creation_tokens, cache_read_tokens,
			reasoning_output_tokens, total_tokens, cost_usd, request_count
		FROM hour_usage`
	var args []interface{}
	if days > 0 {
		// 按本地窗口换算 UTC 下限，避免本地当天数据（前一 UTC 日）被滤掉
		sqlText += ` WHERE usage_date >= ?`
		args = append(args, localWindowStartUTC(days-1).Format("2006-01-02"))
	}
	sqlText += ` ORDER BY usage_date DESC, hour ASC`
	rows, err := m.db.Query(sqlText, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []model.HourUsage
	for rows.Next() {
		var r model.HourUsage
		if err := rows.Scan(&r.Device, &r.Source, &r.UsageDate, &r.Hour, &r.Model,
			&r.InputTokens, &r.OutputTokens, &r.CacheCreationTokens, &r.CacheReadTokens,
			&r.ReasoningOutputTokens, &r.TotalTokens, &r.CostUSD, &r.RequestCount,
		); err != nil {
			return nil, err
		}
		results = append(results, r)
	}
	log.Printf("[db] QueryHourUsage rows=%d elapsed=%v", len(results), time.Since(start))
	return results, rows.Err()
}
