// Package database 的子文件，daily_usage 表的 CRUD 操作。
package database

import (
	"database/sql"
	"fmt"
	"log"
	"strings"
	"time"

	"token-dashboard/internal/model"
)

func (m *Manager) BulkUpsertDaily(rows []model.DailyUsage) error {
	return m.bulkUpsertDailyExec(m.db, rows)
}

func (m *Manager) BulkUpsertDailyTx(tx *sql.Tx, rows []model.DailyUsage) error {
	return m.bulkUpsertDailyExec(tx, rows)
}

func (m *Manager) bulkUpsertDailyExec(ex execer, rows []model.DailyUsage) error {
	if len(rows) == 0 {
		return nil
	}
	return bulkExec(ex, rows, bulkBatchSize, func(batch []model.DailyUsage) (string, []interface{}) {
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
			args = append(args, r.Device, r.Source, r.UsageDate, r.Model,
				r.InputTokens, r.OutputTokens, r.CacheCreationTokens, r.CacheReadTokens,
				r.ReasoningOutputTokens, r.TotalTokens, r.CostUSD, r.UsageDate)
		}
		sqlBuf.WriteString(` ON CONFLICT(device,source,usage_date,model) DO UPDATE SET
			input_tokens=excluded.input_tokens, output_tokens=excluded.output_tokens,
			cache_creation_tokens=excluded.cache_creation_tokens, cache_read_tokens=excluded.cache_read_tokens,
			reasoning_output_tokens=excluded.reasoning_output_tokens, total_tokens=excluded.total_tokens,
			cost_usd=CASE WHEN daily_usage.usage_date<date('now','localtime') THEN daily_usage.cost_usd WHEN excluded.cost_usd=0 THEN daily_usage.cost_usd ELSE excluded.cost_usd END,
			pricing_locked_at=CASE WHEN daily_usage.usage_date<date('now','localtime') THEN COALESCE(daily_usage.pricing_locked_at,datetime('now','localtime')) ELSE NULL END,
			updated_at=datetime('now','localtime')`)
		return sqlBuf.String(), args
	})
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
			input_tokens=MAX(excluded.input_tokens, daily_usage.input_tokens),
			output_tokens=MAX(excluded.output_tokens, daily_usage.output_tokens),
			cache_creation_tokens=MAX(excluded.cache_creation_tokens, daily_usage.cache_creation_tokens),
			cache_read_tokens=MAX(excluded.cache_read_tokens, daily_usage.cache_read_tokens),
			reasoning_output_tokens=MAX(excluded.reasoning_output_tokens, daily_usage.reasoning_output_tokens),
			total_tokens=MAX(excluded.total_tokens, daily_usage.total_tokens),
			cost_usd=CASE
				WHEN daily_usage.usage_date < date('now','localtime') THEN daily_usage.cost_usd
				WHEN excluded.cost_usd = 0 THEN daily_usage.cost_usd
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
