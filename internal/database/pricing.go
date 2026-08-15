// Package database 的子文件，model_pricing 表 DAO（模型价格，唯一价格源）。
package database

import (
	"log"
	"time"

	"token-dashboard/internal/debuglog"
	"token-dashboard/internal/model"
)

// UpsertModelPricing 全量 UPSERT 价格行（拉取更新用）。
// 保护规则：新值为 0 时保留库中已有值（防止模型下线/上游字段清空把价格清零，
// 也保留用户手动设置的价格）；新值非 0 才覆盖。
func (m *Manager) UpsertModelPricing(rows []model.ModelPricing) error {
	if len(rows) == 0 {
		return nil
	}
	start := time.Now()
	err := bulkExecPrepared(m.db, rows, `
		INSERT INTO model_pricing (model_key, input_rate, output_rate, cache_read_rate, cache_write_rate, fetched_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, datetime('now','localtime'))
		ON CONFLICT(model_key) DO UPDATE SET
			input_rate = CASE WHEN excluded.input_rate > 0 THEN excluded.input_rate ELSE model_pricing.input_rate END,
			output_rate = CASE WHEN excluded.output_rate > 0 THEN excluded.output_rate ELSE model_pricing.output_rate END,
			cache_read_rate = CASE WHEN excluded.cache_read_rate > 0 THEN excluded.cache_read_rate ELSE model_pricing.cache_read_rate END,
			cache_write_rate = CASE WHEN excluded.cache_write_rate > 0 THEN excluded.cache_write_rate ELSE model_pricing.cache_write_rate END,
			fetched_at = excluded.fetched_at,
			updated_at = datetime('now','localtime')`,
		func(r model.ModelPricing) []interface{} {
			return []interface{}{r.ModelKey, r.InputRate, r.OutputRate, r.CacheReadRate, r.CacheWriteRate, r.FetchedAt}
		})
	if err != nil {
		log.Printf("[db] UpsertModelPricing error=%v", err)
		return err
	}
	debuglog.Perf("db UpsertModelPricing rows=%d elapsed=%v", len(rows), time.Since(start))
	return nil
}

// UpdateModelPricing 单行 UPSERT（用户改价，保留原 fetched_at）。
func (m *Manager) UpdateModelPricing(row model.ModelPricing) error {
	_, err := m.db.Exec(`
		INSERT INTO model_pricing (model_key, input_rate, output_rate, cache_read_rate, cache_write_rate, fetched_at, updated_at)
		VALUES (?, ?, ?, ?, ?, COALESCE(?, (SELECT fetched_at FROM model_pricing WHERE model_key = ?)), datetime('now','localtime'))
		ON CONFLICT(model_key) DO UPDATE SET
			input_rate = excluded.input_rate,
			output_rate = excluded.output_rate,
			cache_read_rate = excluded.cache_read_rate,
			cache_write_rate = excluded.cache_write_rate,
			updated_at = datetime('now','localtime')`,
		row.ModelKey, row.InputRate, row.OutputRate, row.CacheReadRate, row.CacheWriteRate, row.FetchedAt, row.ModelKey)
	if err != nil {
		log.Printf("[db] UpdateModelPricing error=%v", err)
		return err
	}
	return nil
}

// DeleteModelPricing 删除价格行（用户删除手动添加/残留模型）。
func (m *Manager) DeleteModelPricing(modelKey string) error {
	_, err := m.db.Exec(`DELETE FROM model_pricing WHERE model_key = ?`, modelKey)
	if err != nil {
		log.Printf("[db] DeleteModelPricing error=%v", err)
		return err
	}
	return nil
}

// ListModelPricing 返回全部价格行（按模型 key 排序）。
func (m *Manager) ListModelPricing() ([]model.ModelPricing, error) {
	rows, err := m.db.Query(`
		SELECT model_key, input_rate, output_rate, cache_read_rate, cache_write_rate, COALESCE(fetched_at, ''), updated_at
		FROM model_pricing ORDER BY model_key`)
	if err != nil {
		log.Printf("[db] ListModelPricing query error=%v", err)
		return nil, err
	}
	defer rows.Close()

	var out []model.ModelPricing
	for rows.Next() {
		var r model.ModelPricing
		if err := rows.Scan(&r.ModelKey, &r.InputRate, &r.OutputRate, &r.CacheReadRate, &r.CacheWriteRate, &r.FetchedAt, &r.UpdatedAt); err != nil {
			log.Printf("[db] ListModelPricing scan error=%v", err)
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// CountModelPricing 返回价格行数（seed 判空用）。
func (m *Manager) CountModelPricing() (int, error) {
	var n int
	if err := m.db.QueryRow(`SELECT COUNT(*) FROM model_pricing`).Scan(&n); err != nil {
		log.Printf("[db] CountModelPricing error=%v", err)
		return 0, err
	}
	return n, nil
}

// PricingMeta 返回价格元信息：最近拉取时间与条目数。
func (m *Manager) PricingMeta() (*model.PricingMeta, error) {
	var fetchedAt string
	var n int
	if err := m.db.QueryRow(`SELECT COALESCE(MAX(fetched_at), ''), COUNT(*) FROM model_pricing`).Scan(&fetchedAt, &n); err != nil {
		log.Printf("[db] PricingMeta error=%v", err)
		return nil, err
	}
	return &model.PricingMeta{FetchedAt: fetchedAt, Count: n}, nil
}
