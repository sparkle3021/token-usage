// Package database 的子文件，设备身份注册表（devices 表）DAO 与存量身份迁移。
package database

import (
	"fmt"
	"log"
	"os"

	"github.com/google/uuid"
	"token-dashboard/internal/model"
)

// app_config key：本机设备身份（UUID）。
const localDeviceIDKey = "device_id"

// deviceUsageTables 四张用量表，device 列需迁移的集合。
var deviceUsageTables = []string{"time_usage", "hour_usage", "daily_usage", "session_usage"}

// machineHostname 返回本机主机名，供设备注册事实记录。
func machineHostname() string {
	h, err := os.Hostname()
	if err != nil {
		return "unknown"
	}
	return h
}

// migrateDeviceIdentity 迁移存量身份：将四张用量表中每个
// 非 UUID 的旧 device 值（hostname）映射为独立 UUID，UPDATE 替换并注册进 devices 表。
// 幂等锚点：devices.hostname 已有记录则复用其 device_id；app_config.device_id 缺失时
// 由本机 hostname 映射补齐。整体包在单事务内，失败回滚。
func (m *Manager) migrateDeviceIdentity() error {
	host := machineHostname()
	tx, err := m.db.Begin()
	if err != nil {
		return fmt.Errorf("migrate device: begin tx: %w", err)
	}
	defer tx.Rollback()

	// 1. 收集所有 distinct device 值（四表 + 既有本机身份）
	distinct := map[string]bool{}
	for _, table := range deviceUsageTables {
		rows, err := tx.Query(`SELECT DISTINCT device FROM ` + table)
		if err != nil {
			return fmt.Errorf("migrate device: query %s: %w", table, err)
		}
		for rows.Next() {
			var d string
			if err := rows.Scan(&d); err != nil {
				rows.Close()
				return fmt.Errorf("migrate device: scan %s: %w", table, err)
			}
			if d != "" {
				distinct[d] = true
			}
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return fmt.Errorf("migrate device: rows %s: %w", table, err)
		}
		rows.Close()
	}

	var cur string
	if err := tx.QueryRow(`SELECT value FROM app_config WHERE key = ?`, localDeviceIDKey).Scan(&cur); err != nil {
		cur = "" // app_config 中尚无 device_id
	}
	if cur != "" {
		distinct[cur] = true
	}

	// 2. 建立旧值 → UUID 映射；已注册的 hostname 复用原 device_id
	mapping := map[string]string{}
	for old := range distinct {
		if old == "" {
			continue
		}
		// 已注册的 hostname 复用（幂等锚点）
		var existing string
		if err := tx.QueryRow(`SELECT device_id FROM devices WHERE hostname = ? LIMIT 1`, old).Scan(&existing); err == nil {
			mapping[old] = existing
			continue
		}
		// 本身已是合法 UUID：注册进 devices 表（若缺失）即可，无需替换
		if _, err := uuid.Parse(old); err == nil {
			var c int
			if err := tx.QueryRow(`SELECT COUNT(*) FROM devices WHERE device_id = ?`, old).Scan(&c); err == nil && c == 0 {
				if _, err := tx.Exec(`INSERT INTO devices(device_id, hostname, display_name, is_local) VALUES (?, ?, ?, 0)`, old, old, old); err != nil {
					return fmt.Errorf("migrate device: register uuid %s: %w", old, err)
				}
			}
			mapping[old] = old
			continue
		}
		// 新身份：生成 UUID 并注册
		id := uuid.NewString()
		if _, err := tx.Exec(`INSERT INTO devices(device_id, hostname, display_name, is_local) VALUES (?, ?, ?, 0)`, id, old, old); err != nil {
			return fmt.Errorf("migrate device: register %s: %w", old, err)
		}
		mapping[old] = id
	}

	// 3. UPDATE 替换四表
	//    四表 device 是 PK 成分：UPDATE 可能撞 UNIQUE（同 PK 组合已存在目标 UUID 行，
	//    即同一事件的重复写入，如老版本采集器以 hostname 写入后新版又以 UUID 写入）。
	//    用 UPDATE OR IGNORE 迁移非重复行，再 DELETE 残留 hostname（重复事件，去重）。
	for old, id := range mapping {
		if old == id {
			continue
		}
		for _, table := range deviceUsageTables {
			if _, err := tx.Exec(`UPDATE OR IGNORE `+table+` SET device = ? WHERE device = ?`, id, old); err != nil {
				return fmt.Errorf("migrate device: update %s: %w", table, err)
			}
			if _, err := tx.Exec(`DELETE FROM `+table+` WHERE device = ?`, old); err != nil {
				return fmt.Errorf("migrate device: dedup %s: %w", table, err)
			}
		}
	}

	// 4. 确保本机 device_id：app_config 既有值优先，否则由本机 hostname 映射补齐，缺失则新建
	localID := cur
	if localID == "" {
		if id, ok := mapping[host]; ok {
			localID = id
		} else {
			localID = uuid.NewString()
			if _, err := tx.Exec(`INSERT INTO devices(device_id, hostname, display_name, is_local) VALUES (?, ?, ?, 1)`, localID, host, host); err != nil {
				return fmt.Errorf("migrate device: register local: %w", err)
			}
		}
	} else {
		var c int
		if err := tx.QueryRow(`SELECT COUNT(*) FROM devices WHERE device_id = ?`, localID).Scan(&c); err == nil && c == 0 {
			if _, err := tx.Exec(`INSERT INTO devices(device_id, hostname, display_name, is_local) VALUES (?, ?, ?, 0)`, localID, host, host); err != nil {
				return fmt.Errorf("migrate device: register local row: %w", err)
			}
		}
	}
	if _, err := tx.Exec(`UPDATE devices SET is_local = 1 WHERE device_id = ?`, localID); err != nil {
		return fmt.Errorf("migrate device: mark local: %w", err)
	}
	if _, err := tx.Exec(`
		INSERT INTO app_config(key, value, updated_at) VALUES (?, ?, datetime('now','localtime'))
		ON CONFLICT(key) DO UPDATE SET value = excluded.value, updated_at = datetime('now','localtime')
	`, localDeviceIDKey, localID); err != nil {
		return fmt.Errorf("migrate device: persist device_id: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("migrate device: commit: %w", err)
	}
	m.localDeviceID = localID
	log.Printf("[db] migrateDeviceIdentity ok host=%s distinct=%d local_id=%s", host, len(distinct), localID)
	return nil
}

// LocalDeviceID 返回本机设备身份（UUID）。迁移后始终非空。
func (m *Manager) LocalDeviceID() string {
	return m.localDeviceID
}

// ListDevices 返回 devices 表全量设备信息。
func (m *Manager) ListDevices() ([]model.DeviceInfo, error) {
	rows, err := m.db.Query(`
		SELECT device_id, hostname, display_name, is_local
		FROM devices ORDER BY is_local DESC, display_name ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []model.DeviceInfo
	for rows.Next() {
		var d model.DeviceInfo
		if err := rows.Scan(&d.DeviceID, &d.Hostname, &d.DisplayName, &d.IsLocal); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

// DeviceNames 返回 device_id → display_name（缺省 hostname）映射，供查询层附带。
func (m *Manager) DeviceNames() (map[string]string, error) {
	rows, err := m.db.Query(`SELECT device_id, hostname, display_name FROM devices`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make(map[string]string)
	for rows.Next() {
		var id, host, disp string
		if err := rows.Scan(&id, &host, &disp); err != nil {
			return nil, err
		}
		name := disp
		if name == "" {
			name = host
		}
		out[id] = name
	}
	return out, rows.Err()
}

// RenameDevice 修改设备展示名，仅作用于 devices.display_name，不触碰任何用量表。
func (m *Manager) RenameDevice(deviceID, displayName string) error {
	if _, err := m.db.Exec(`
		UPDATE devices SET display_name = ?, updated_at = datetime('now','localtime')
		WHERE device_id = ?`, displayName, deviceID); err != nil {
		return fmt.Errorf("rename device: %w", err)
	}
	return nil
}
