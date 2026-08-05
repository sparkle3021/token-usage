package database

import (
	"path/filepath"
	"testing"

	"github.com/google/uuid"
)

// TestMigrateDeviceIdentity 验证存量 hostname → UUID 迁移：值替换、本机 device_id 设置、幂等重跑。
func TestMigrateDeviceIdentity(t *testing.T) {
	m, err := New(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer m.Close()

	// 模拟升级前存量：hostname 身份的数据行（四表 + collection_runs 各插一条）
	_, err = m.Exec(`INSERT INTO time_usage
		(device, source, event_key, event_time, usage_date, model,
		 input_tokens, output_tokens, cache_creation_tokens, cache_read_tokens,
		 reasoning_output_tokens, total_tokens, cost_usd)
		VALUES ('OldHost', 'Claude Code', 'k1', '2026-08-01T10:00:00Z', '2026-08-01',
			'model-x', 10, 20, 0, 0, 0, 30, 0.001)`)
	if err != nil {
		t.Fatalf("insert time_usage: %v", err)
	}
	_, err = m.Exec(`INSERT INTO hour_usage
		(device, source, usage_date, hour, model,
		 input_tokens, output_tokens, cache_creation_tokens, cache_read_tokens,
		 reasoning_output_tokens, total_tokens, cost_usd)
		VALUES ('OldHost', 'Claude Code', '2026-08-01', 10, 'model-x',
			10, 20, 0, 0, 0, 30, 0.001)`)
	if err != nil {
		t.Fatalf("insert hour_usage: %v", err)
	}
	_, err = m.Exec(`INSERT INTO daily_usage
		(device, source, usage_date, model, input_tokens, output_tokens,
		 cache_creation_tokens, cache_read_tokens, reasoning_output_tokens, total_tokens, cost_usd)
		VALUES ('OldHost', 'Claude Code', '2026-08-01', 'model-x',
			10, 20, 0, 0, 0, 30, 0.001)`)
	if err != nil {
		t.Fatalf("insert daily_usage: %v", err)
	}
	_, err = m.Exec(`INSERT INTO session_usage
		(device, source, session_id, model, input_tokens, output_tokens,
		 cache_creation_tokens, cache_read_tokens, reasoning_output_tokens, total_tokens, cost_usd)
		VALUES ('OldHost', 'Claude Code', 'sess-1', 'model-x',
			10, 20, 0, 0, 0, 30, 0.001)`)
	if err != nil {
		t.Fatalf("insert session_usage: %v", err)
	}
	_, err = m.Exec(`INSERT INTO collection_runs(device, source, status) VALUES ('OldHost', 'Claude Code', 'ok')`)
	if err != nil {
		t.Fatalf("insert collection_runs: %v", err)
	}

	// 执行迁移
	if err := m.migrateDeviceIdentity(); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	// 断言：四表 + collection_runs 的 device 全部替换为合法 UUID
	for _, table := range deviceUsageTables {
		var device string
		if err := m.db.QueryRow(`SELECT device FROM `+table+` WHERE source = 'Claude Code' LIMIT 1`).Scan(&device); err != nil {
			t.Fatalf("query %s: %v", table, err)
		}
		if _, err := uuid.Parse(device); err != nil {
			t.Fatalf("%s device not migrated to UUID, got %q", table, device)
		}
	}

	// 断言：本机 device_id 已设置且为 UUID
	localID, err := m.GetConfig(localDeviceIDKey)
	if err != nil || localID == "" {
		t.Fatalf("device_id not set, got %q err=%v", localID, err)
	}
	if _, err := uuid.Parse(localID); err != nil {
		t.Fatalf("local device_id not a UUID: %q", localID)
	}

	// 幂等：再次迁移，device 保持同一 UUID（不重复生成身份）
	migrated := make(map[string]string)
	for _, table := range deviceUsageTables {
		var device string
		if err := m.db.QueryRow(`SELECT device FROM `+table+` WHERE source = 'Claude Code' LIMIT 1`).Scan(&device); err != nil {
			t.Fatalf("re-query %s: %v", table, err)
		}
		migrated[table] = device
	}
	if err := m.migrateDeviceIdentity(); err != nil {
		t.Fatalf("re-migrate: %v", err)
	}
	for _, table := range deviceUsageTables {
		var device string
		if err := m.db.QueryRow(`SELECT device FROM `+table+` WHERE source = 'Claude Code' LIMIT 1`).Scan(&device); err != nil {
			t.Fatalf("re-query2 %s: %v", table, err)
		}
		if device != migrated[table] {
			t.Fatalf("idempotency broken: %s changed %s -> %s", table, migrated[table], device)
		}
	}

	// 断言：devices 表已注册 OldHost，且本机记录 is_local=1
	var hostDevice string
	if err := m.db.QueryRow(`SELECT device_id FROM devices WHERE hostname = 'OldHost'`).Scan(&hostDevice); err != nil {
		t.Fatalf("OldHost not registered: %v", err)
	}
	if hostDevice != migrated["time_usage"] {
		t.Fatalf("OldHost mapping mismatch: %s != %s", hostDevice, migrated["time_usage"])
	}
	var localCount int
	if err := m.db.QueryRow(`SELECT COUNT(*) FROM devices WHERE device_id = ? AND is_local = 1`, localID).Scan(&localCount); err != nil {
		t.Fatalf("local count: %v", err)
	}
	if localCount != 1 {
		t.Fatalf("local device is_local flag not set, count=%d", localCount)
	}
}

// TestMigrateDeviceIdentityDedup 验证 hostname 与 UUID 双值并存时的迁移收敛：
// 同一事件的双身份重复行去重保留 UUID，非重复 hostname 行迁移为 UUID，无 hostname 残留。
func TestMigrateDeviceIdentityDedup(t *testing.T) {
	m, err := New(filepath.Join(t.TempDir(), "dedup.db"))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer m.Close()

	localID, err := m.GetConfig(localDeviceIDKey)
	if err != nil || localID == "" {
		t.Fatalf("local device_id missing: %q err=%v", localID, err)
	}

	// 本机 hostname：devices.hostname 已映射到 localID，迁移时 hostname 行的目标就是 localID
	host := machineHostname()

	// 老版本以 hostname 写入的重复事件（k-dup，与新版 UUID 行同 event_key）
	if _, err := m.Exec(`INSERT INTO time_usage
		(device, source, event_key, event_time, usage_date, model, total_tokens)
		VALUES (?, 'S', 'k-dup', '2026-08-01T10:00:00Z', '2026-08-01', 'm', 100)`, host); err != nil {
		t.Fatalf("insert host dup: %v", err)
	}
	if _, err := m.Exec(`INSERT INTO time_usage
		(device, source, event_key, event_time, usage_date, model, total_tokens)
		VALUES (?, 'S', 'k-dup', '2026-08-01T10:00:00Z', '2026-08-01', 'm', 100)`, localID); err != nil {
		t.Fatalf("insert uuid dup: %v", err)
	}
	// 老版本采集的非重复新数据（k-new，UUID 中不存在）
	if _, err := m.Exec(`INSERT INTO time_usage
		(device, source, event_key, event_time, usage_date, model, total_tokens)
		VALUES (?, 'S', 'k-new', '2026-08-01T11:00:00Z', '2026-08-01', 'm', 50)`, host); err != nil {
		t.Fatalf("insert host new: %v", err)
	}

	if err := m.migrateDeviceIdentity(); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	// 无 hostname 残留
	var hostCount int
	if err := m.db.QueryRow(`SELECT COUNT(*) FROM time_usage WHERE device=?`, host).Scan(&hostCount); err != nil || hostCount != 0 {
		t.Fatalf("hostname rows remain: %d err=%v", hostCount, err)
	}

	// k-dup：去重只剩 1 行且保留 UUID 身份
	var dupCount int
	if err := m.db.QueryRow(`SELECT COUNT(*) FROM time_usage WHERE source='S' AND event_key='k-dup'`).Scan(&dupCount); err != nil || dupCount != 1 {
		t.Fatalf("dup should dedup to 1 row, got %d err=%v", dupCount, err)
	}
	var dupDev string
	if err := m.db.QueryRow(`SELECT device FROM time_usage WHERE source='S' AND event_key='k-dup'`).Scan(&dupDev); err != nil || dupDev != localID {
		t.Fatalf("dup row should keep local uuid, got %q err=%v", dupDev, err)
	}

	// k-new：非重复 hostname 迁移为 UUID（数据保留）
	var newDev string
	if err := m.db.QueryRow(`SELECT device FROM time_usage WHERE source='S' AND event_key='k-new'`).Scan(&newDev); err != nil {
		t.Fatalf("k-new missing: %v", err)
	}
	if _, perr := uuid.Parse(newDev); perr != nil {
		t.Fatalf("k-new not migrated to uuid: %q", newDev)
	}

	// 幂等：再次迁移不报错、不残留
	if err := m.migrateDeviceIdentity(); err != nil {
		t.Fatalf("re-migrate: %v", err)
	}
	if err := m.db.QueryRow(`SELECT COUNT(*) FROM time_usage WHERE device=?`, host).Scan(&hostCount); err != nil || hostCount != 0 {
		t.Fatalf("re-migrate hostname remain: %d err=%v", hostCount, err)
	}
}
