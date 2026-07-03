// Package database 的子文件，app_config 表的读写及检查点管理。
package database

import (
	"database/sql"
)

const (
	CCSwitchCursorProxyLogs = "cc_switch_cursor_proxy_request_logs"
	CCSwitchRollupMaxDate   = "cc_switch_rollup_max_date"
)

func (m *Manager) GetConfig(key string) (string, error) {
	var value string
	err := m.db.QueryRow(`SELECT value FROM app_config WHERE key = ?`, key).Scan(&value)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return value, err
}

func (m *Manager) SetConfig(key, value string) error {
	_, err := m.db.Exec(`
		INSERT INTO app_config(key, value, updated_at) VALUES (?, ?, datetime('now','localtime'))
		ON CONFLICT(key) DO UPDATE SET value = excluded.value, updated_at = datetime('now','localtime')
	`, key, value)
	return err
}

func (m *Manager) GetAllConfigs() (map[string]string, error) {
	rows, err := m.db.Query(`SELECT key, value FROM app_config`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make(map[string]string)
	for rows.Next() {
		var k, v string
		if err := rows.Scan(&k, &v); err != nil {
			return nil, err
		}
		result[k] = v
	}
	return result, rows.Err()
}

func (m *Manager) GetCheckpoint(key string) (string, error) {
	return m.GetConfig(key)
}

func (m *Manager) SetCheckpoint(key, value string) error {
	return m.SetConfig(key, value)
}

func (m *Manager) DeleteCheckpointsByPrefix(prefix string) error {
	_, err := m.db.Exec(`DELETE FROM app_config WHERE key LIKE ?`, prefix+"%")
	return err
}

func (m *Manager) ResetCCSwitchCheckpoints() error {
	if err := m.DeleteCheckpointsByPrefix("cc_switch_cursor_"); err != nil {
		return err
	}
	return m.DeleteCheckpointsByPrefix("cc_switch_rollup_")
}

func (m *Manager) GetMinUsageDate(source string) (string, error) {
	var v sql.NullString
	err := m.db.QueryRow(`SELECT MIN(usage_date) FROM daily_usage WHERE source = ?`, source).Scan(&v)
	if err != nil || !v.Valid {
		return "", err
	}
	return v.String, nil
}
