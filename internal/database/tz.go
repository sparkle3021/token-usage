// Package database 的子文件，存量小时桶时区迁移（本地桶 → UTC 桶，单次执行）。
package database

import (
	"fmt"
	"log"
	"time"

	"token-dashboard/internal/model"
)

// app_config key：小时桶 UTC 迁移标记。
const tzMigratedKey = "tz_migrated"

// migrateHourTimezone 将存量 hour_usage 的"本机本地时区桶"迁移为 UTC 桶。
// 跨设备统一时间轴的地基：存储统一 UTC，查询层按本机时区平移展示。
// 幂等：app_config 标记 tz_migrated 存在即跳过。整体在单事务内，失败回滚不丢数据。
func (m *Manager) migrateHourTimezone() error {
	if existing, _ := m.GetConfig(tzMigratedKey); existing != "" {
		return nil
	}

	rows, err := m.QueryHourUsage(0)
	if err != nil {
		return fmt.Errorf("migrate tz: query hour: %w", err)
	}
	if len(rows) == 0 {
		if err := m.SetConfig(tzMigratedKey, "1"); err != nil {
			return fmt.Errorf("migrate tz: mark empty: %w", err)
		}
		return nil
	}

	// 本地桶 → UTC 桶；冲突（多个本地桶映射到同一 UTC 桶）按 key 聚合。
	type bucketKey struct{ device, source, date, model string; hour int }
	acc := make(map[bucketKey]*model.HourUsage)
	for _, r := range rows {
		utcDate, utcHour := localBucketToUTC(r.UsageDate, r.Hour)
		k := bucketKey{r.Device, r.Source, utcDate, r.Model, utcHour}
		e, ok := acc[k]
		if !ok {
			e = &model.HourUsage{Device: r.Device, Source: r.Source, UsageDate: utcDate, Hour: utcHour, Model: r.Model}
			acc[k] = e
		}
		e.InputTokens += r.InputTokens
		e.OutputTokens += r.OutputTokens
		e.CacheCreationTokens += r.CacheCreationTokens
		e.CacheReadTokens += r.CacheReadTokens
		e.ReasoningOutputTokens += r.ReasoningOutputTokens
		e.TotalTokens += r.TotalTokens
		e.CostUSD += r.CostUSD
	}

	batch := make([]model.HourUsage, 0, len(acc))
	for _, e := range acc {
		batch = append(batch, *e)
	}

	tx, err := m.db.Begin()
	if err != nil {
		return fmt.Errorf("migrate tz: begin tx: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`DELETE FROM hour_usage`); err != nil {
		return fmt.Errorf("migrate tz: clear hour: %w", err)
	}
	if err := m.bulkUpsertHourUsageExec(tx, batch); err != nil {
		return fmt.Errorf("migrate tz: upsert hour: %w", err)
	}
	if _, err := tx.Exec(`
		INSERT INTO app_config(key, value, updated_at) VALUES (?, '1', datetime('now','localtime'))
		ON CONFLICT(key) DO UPDATE SET value = excluded.value, updated_at = datetime('now','localtime')
	`, tzMigratedKey); err != nil {
		return fmt.Errorf("migrate tz: mark: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("migrate tz: commit: %w", err)
	}
	log.Printf("[db] migrateHourTimezone ok buckets=%d", len(batch))
	return nil
}

// localBucketToUTC 将"本机本地时区桶" (date, hour) 换算为 UTC 桶 (date, hour)。
// 假设采集时区 = 当前本机时区；迁移后查询层按本机时区平移回本地，单机展示不变。
func localBucketToUTC(date string, hour int) (string, int) {
	t, err := time.ParseInLocation("2006-01-02", date, time.Local)
	if err != nil {
		return date, hour
	}
	t = t.Add(time.Duration(hour) * time.Hour).UTC()
	return t.Format("2006-01-02"), t.Hour()
}

// localWindowStartUTC 返回"本地今天 00:00 往前 daysAgo 天"对应窗口起点的 UTC 时刻。
// 用于 days 过滤的 UTC 口径：本地当天数据可能落在前一 UTC 日，须按本地窗口换算下限，
// 避免用本地日期字符串当 UTC 下限导致本地凌晨数据被滤掉。
func localWindowStartUTC(daysAgo int) time.Time {
	now := time.Now()
	localStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.Local).AddDate(0, 0, -daysAgo)
	return localStart.UTC()
}
