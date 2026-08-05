// Package database 的子文件，数据导入（跨设备合并）写入逻辑。
package database

import (
	"database/sql"
	"fmt"
	"log"
	"time"

	"token-dashboard/internal/model"
)

// ImportUsage 单事务导入：注册设备映射 + 三层用量 upsert。
// 幂等：各层均为 upsert MAX 合并，重复导入不重复计数；任一步失败整体回滚。
// 返回各层写入行数（=传入行数）与新增设备数（INSERT OR IGNORE 实际命中数）。
// 日志：[db] ImportUsage ok/failed 含各层行数与耗时，供导入链路排查。
func (m *Manager) ImportUsage(hours []model.HourUsage, daily []model.DailyUsage, sessions []model.SessionUsage, deviceNames map[string]string) (importedHours, importedDaily, importedSessions, newDevices int, err error) {
	start := time.Now()
	tx, err := m.db.Begin()
	if err != nil {
		return m.importFail("begin tx", err, start)
	}
	defer tx.Rollback()

	newDevices, err = m.registerDevicesTx(tx, deviceNames)
	if err != nil {
		return m.importFail("register devices", err, start)
	}
	if err := m.BulkUpsertHourUsageTx(tx, hours); err != nil {
		return m.importFail("hour_usage", err, start)
	}
	if err := m.BulkUpsertDailyTx(tx, daily); err != nil {
		return m.importFail("daily_usage", err, start)
	}
	if err := m.BulkUpsertSessionTx(tx, sessions); err != nil {
		return m.importFail("session_usage", err, start)
	}

	if err := tx.Commit(); err != nil {
		return m.importFail("commit", err, start)
	}
	log.Printf("[db] ImportUsage ok hours=%d daily=%d sessions=%d new_devices=%d elapsed=%v",
		len(hours), len(daily), len(sessions), newDevices, time.Since(start))
	return len(hours), len(daily), len(sessions), newDevices, nil
}

// importFail 记录导入失败日志（含阶段与耗时）并返回 0 值 + 包装错误。
func (m *Manager) importFail(stage string, err error, start time.Time) (int, int, int, int, error) {
	log.Printf("[db] ImportUsage %s failed: %v elapsed=%v", stage, err, time.Since(start))
	return 0, 0, 0, 0, fmt.Errorf("import %s: %w", stage, err)
}

// registerDevicesTx 将导入文件的设备映射注册进 devices 表。
// device_id 已存在（含本机自身）时保留既有记录；is_local 恒为 0（非本机设备）。
// hostname 无事实来源故留空，展示名用导入的 display_name。
// 返回实际新增设备数。
func (m *Manager) registerDevicesTx(tx *sql.Tx, names map[string]string) (int, error) {
	newDevices := 0
	for id, name := range names {
		if id == "" {
			continue
		}
		res, err := tx.Exec(`
			INSERT INTO devices(device_id, hostname, display_name, is_local)
			VALUES (?, '', ?, 0)
			ON CONFLICT(device_id) DO NOTHING`, id, name)
		if err != nil {
			return 0, fmt.Errorf("register device %s: %w", TruncateStr(id, 24), err)
		}
		if n, err := res.RowsAffected(); err == nil {
			newDevices += int(n)
		}
	}
	if newDevices > 0 {
		log.Printf("[db] registerDevicesTx imported new devices=%d", newDevices)
	}
	return newDevices, nil
}
