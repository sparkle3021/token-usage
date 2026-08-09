// Package database 的子文件，daily_usage 表的 CRUD 操作。
package database

import (
	"database/sql"
	"fmt"
	"log"
	"time"

	"token-dashboard/internal/model"
)

func (m *Manager) BulkUpsertDaily(rows []model.DailyUsage) error {
	return m.bulkUpsertDailyExec(m.db, rows)
}

func (m *Manager) BulkUpsertDailyTx(tx *sql.Tx, rows []model.DailyUsage) error {
	return m.bulkUpsertDailyExec(tx, rows)
}

func (m *Manager) bulkUpsertDailyExec(ex preparedExecer, rows []model.DailyUsage) error {
	if len(rows) == 0 {
		return nil
	}
	return bulkExecPrepared(ex, rows, `
		INSERT INTO daily_usage (device,source,usage_date,model,
			input_tokens,output_tokens,cache_creation_tokens,cache_read_tokens,
			reasoning_output_tokens,total_tokens,cost_usd,request_count,pricing_locked_at,updated_at)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?,
			CASE WHEN ?<date('now','localtime') THEN datetime('now','localtime') ELSE NULL END,
			datetime('now','localtime'))
		ON CONFLICT(device,source,usage_date,model) DO UPDATE SET
			input_tokens=excluded.input_tokens, output_tokens=excluded.output_tokens,
			cache_creation_tokens=excluded.cache_creation_tokens, cache_read_tokens=excluded.cache_read_tokens,
			reasoning_output_tokens=excluded.reasoning_output_tokens, total_tokens=excluded.total_tokens,
			cost_usd=CASE WHEN daily_usage.usage_date<date('now','localtime') THEN daily_usage.cost_usd WHEN excluded.cost_usd=0 THEN daily_usage.cost_usd ELSE excluded.cost_usd END,
			request_count=excluded.request_count,
			pricing_locked_at=CASE WHEN daily_usage.usage_date<date('now','localtime') THEN COALESCE(daily_usage.pricing_locked_at,datetime('now','localtime')) ELSE NULL END,
			updated_at=datetime('now','localtime')`,
		func(r model.DailyUsage) []interface{} {
			return []interface{}{r.Device, r.Source, r.UsageDate, r.Model,
				r.InputTokens, r.OutputTokens, r.CacheCreationTokens, r.CacheReadTokens,
				r.ReasoningOutputTokens, r.TotalTokens, r.CostUSD, r.RequestCount, r.UsageDate}
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
			reasoning_output_tokens, total_tokens, cost_usd, request_count
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
			&r.ReasoningOutputTokens, &r.TotalTokens, &r.CostUSD, &r.RequestCount,
		); err != nil {
			return nil, err
		}
		results = append(results, r)
	}
	log.Printf("[db] QueryDaily rows=%d elapsed=%v", len(results), time.Since(start))
	return results, rows.Err()
}

// QueryModelRanking 返回按模型聚合的用量排行（总用量/费用/请求次数），按总用量降序。
// 空表返回空切片。
func (m *Manager) QueryModelRanking() ([]model.ModelRanking, error) {
	start := time.Now()
	rows, err := m.db.Query(`
		SELECT model,
			SUM(total_tokens) AS total_tokens,
			SUM(cost_usd) AS cost_usd,
			SUM(request_count) AS request_count
		FROM daily_usage
		GROUP BY model
		ORDER BY SUM(total_tokens) DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []model.ModelRanking
	for rows.Next() {
		var r model.ModelRanking
		if err := rows.Scan(&r.Model, &r.TotalTokens, &r.CostUSD, &r.RequestCount); err != nil {
			return nil, err
		}
		results = append(results, r)
	}
	log.Printf("[db] QueryModelRanking rows=%d elapsed=%v", len(results), time.Since(start))
	return results, rows.Err()
}

// BuildDailyFromHourUsage 从 hour_usage 重建 daily_usage（权威聚合层投影）。
// 每个 UTC 小时桶按本机时区平移为本地 (date, hour) 后按本地日期聚合，
// 使 daily_usage.usage_date 为本地日语义，与展示层及直写来源（Hermes/CC-Switch rollup）对齐。
// token 类字段以本地日聚合值直接覆盖（消除旧 MAX 冻结），cost 与定价锁定沿用 upsert 逻辑。
func (m *Manager) BuildDailyFromHourUsage() error {
	rows, err := m.QueryHourUsage(0)
	if err != nil {
		return fmt.Errorf("query hour usage: %w", err)
	}

	type dailyKey struct{ device, source, date, model string }
	acc := make(map[dailyKey]*model.DailyUsage)
	for _, r := range rows {
		localDate, _ := UTCBucketToLocal(r.UsageDate, r.Hour)
		k := dailyKey{r.Device, r.Source, localDate, r.Model}
		e, ok := acc[k]
		if !ok {
			e = &model.DailyUsage{Device: r.Device, Source: r.Source, UsageDate: localDate, Model: r.Model}
			acc[k] = e
		}
		e.InputTokens += r.InputTokens
		e.OutputTokens += r.OutputTokens
		e.CacheCreationTokens += r.CacheCreationTokens
		e.CacheReadTokens += r.CacheReadTokens
		e.ReasoningOutputTokens += r.ReasoningOutputTokens
		e.TotalTokens += r.TotalTokens
		e.CostUSD += r.CostUSD
		e.RequestCount += r.RequestCount
	}

	if len(acc) == 0 {
		return nil
	}
	batch := make([]model.DailyUsage, 0, len(acc))
	for _, e := range acc {
		batch = append(batch, *e)
	}

	tx, err := m.db.Begin()
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()
	if err := m.bulkUpsertDailyExec(tx, batch); err != nil {
		return fmt.Errorf("upsert daily from hour: %w", err)
	}
	return tx.Commit()
}
