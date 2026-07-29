// Package database 的子文件，quota_configs 表的 CRUD。
package database

import (
	"database/sql"
	"fmt"

	"token-dashboard/internal/model"
)

// ListQuotaConfigs 返回所有用量查询配置，按 id 排序。
func (m *Manager) ListQuotaConfigs() ([]model.QuotaConfig, error) {
	rows, err := m.db.Query(`
		SELECT id, provider, plan, display_name, seq, is_valid, config_json, created_at, updated_at
		FROM quota_configs ORDER BY id ASC
	`)
	if err != nil {
		return nil, fmt.Errorf("list quota configs: %w", err)
	}
	defer rows.Close()

	var out []model.QuotaConfig
	for rows.Next() {
		var c model.QuotaConfig
		if err := rows.Scan(&c.ID, &c.Provider, &c.Plan, &c.DisplayName, &c.Seq, &c.IsValid, &c.ConfigJSON, &c.CreatedAt, &c.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan quota config: %w", err)
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// CreateQuotaConfig 新建配置，自动计算 seq = MAX(seq)+1。
func (m *Manager) CreateQuotaConfig(c *model.QuotaConfig) error {
	var maxSeq sql.NullInt64
	err := m.db.QueryRow(`
		SELECT MAX(seq) FROM quota_configs WHERE provider = ? AND plan = ?
	`, c.Provider, c.Plan).Scan(&maxSeq)
	if err != nil {
		return fmt.Errorf("query max seq: %w", err)
	}
	seq := 1
	if maxSeq.Valid {
		seq = int(maxSeq.Int64) + 1
	}
	c.Seq = seq

	res, err := m.db.Exec(`
		INSERT INTO quota_configs (provider, plan, display_name, seq, config_json, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, datetime('now','localtime'), datetime('now','localtime'))
	`, c.Provider, c.Plan, c.DisplayName, c.Seq, c.ConfigJSON)
	if err != nil {
		return fmt.Errorf("insert quota config: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return fmt.Errorf("last insert id: %w", err)
	}
	c.ID = id
	return nil
}

// UpdateQuotaConfig 修改配置的 display_name 和 config_json。
func (m *Manager) UpdateQuotaConfig(c *model.QuotaConfig) error {
	_, err := m.db.Exec(`
		UPDATE quota_configs SET display_name = ?, config_json = ?, updated_at = datetime('now','localtime')
		WHERE id = ?
	`, c.DisplayName, c.ConfigJSON, c.ID)
	if err != nil {
		return fmt.Errorf("update quota config: %w", err)
	}
	return nil
}

// DeleteQuotaConfig 删除指定配置。
func (m *Manager) DeleteQuotaConfig(id int64) error {
	_, err := m.db.Exec(`DELETE FROM quota_configs WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete quota config: %w", err)
	}
	return nil
}

// SetQuotaConfigValid 更新配置的 is_valid 状态。
func (m *Manager) SetQuotaConfigValid(id int64, valid bool) error {
	val := 0
	if valid { val = 1 }
	_, err := m.db.Exec(`UPDATE quota_configs SET is_valid = ?, updated_at = datetime('now','localtime') WHERE id = ?`, val, id)
	return err
}

// GetQuotaConfigByID 根据 ID 获取单条配置。
func (m *Manager) GetQuotaConfigByID(id int64) (*model.QuotaConfig, error) {
	var c model.QuotaConfig
	err := m.db.QueryRow(`
		SELECT id, provider, plan, display_name, seq, is_valid, config_json, created_at, updated_at
		FROM quota_configs WHERE id = ?
	`, id).Scan(&c.ID, &c.Provider, &c.Plan, &c.DisplayName, &c.Seq, &c.IsValid, &c.ConfigJSON, &c.CreatedAt, &c.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get quota config: %w", err)
	}
	return &c, nil
}
