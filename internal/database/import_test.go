package database

import (
	"path/filepath"
	"testing"

	"token-dashboard/internal/model"
)

// TestImportUsage 验证导入写入：成功落库、幂等（重复导入不重复计数/不重复注册设备）、本机设备保留。
func TestImportUsage(t *testing.T) {
	m, err := New(filepath.Join(t.TempDir(), "import.db"))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer m.Close()

	hours := []model.HourUsage{{
		Device: "dev-imported", Source: "Claude Code", UsageDate: "2026-08-01", Hour: 10, Model: "m",
		InputTokens: 100, OutputTokens: 50, TotalTokens: 150, CostUSD: 0.01,
	}}
	daily := []model.DailyUsage{{
		Device: "dev-imported", Source: "Claude Code", UsageDate: "2026-08-01", Model: "m",
		InputTokens: 100, OutputTokens: 50, TotalTokens: 150, CostUSD: 0.01,
	}}
	sessions := []model.SessionUsage{{
		Device: "dev-imported", Source: "Claude Code", SessionID: "sess-1",
		LastActivity: "2026-08-01T10:00:00Z", InputTokens: 100, OutputTokens: 50, TotalTokens: 150, CostUSD: 0.01,
	}}
	// 含本机 device_id：应被 INSERT OR IGNORE 跳过，不计入新增、不覆盖本机记录
	names := map[string]string{"dev-imported": "导入的设备", m.LocalDeviceID(): "本机"}

	h, d, s, dev, err := m.ImportUsage(hours, daily, sessions, names)
	if err != nil {
		t.Fatalf("import: %v", err)
	}
	if h != 1 || d != 1 || s != 1 {
		t.Fatalf("import counts wrong: hours=%d daily=%d sessions=%d", h, d, s)
	}
	if dev != 1 {
		t.Fatalf("new devices = %d, want 1 (local must be skipped)", dev)
	}

	// 三表均落库
	var hTotal int64
	if err := m.db.QueryRow(`SELECT SUM(total_tokens) FROM hour_usage WHERE device='dev-imported'`).Scan(&hTotal); err != nil || hTotal != 150 {
		t.Fatalf("hour total=%d err=%v", hTotal, err)
	}
	var dCount int
	if err := m.db.QueryRow(`SELECT COUNT(*) FROM daily_usage WHERE device='dev-imported'`).Scan(&dCount); err != nil || dCount != 1 {
		t.Fatalf("daily count=%d err=%v", dCount, err)
	}
	var sCount int
	if err := m.db.QueryRow(`SELECT COUNT(*) FROM session_usage WHERE device='dev-imported'`).Scan(&sCount); err != nil || sCount != 1 {
		t.Fatalf("session count=%d err=%v", sCount, err)
	}

	// 本机设备记录保留：is_local=1、display_name 未被导入名覆盖
	localID := m.LocalDeviceID()
	var localName string
	var isLocal int
	if err := m.db.QueryRow(`SELECT display_name, is_local FROM devices WHERE device_id=?`, localID).Scan(&localName, &isLocal); err != nil {
		t.Fatalf("local device: %v", err)
	}
	if isLocal != 1 {
		t.Fatalf("local device is_local overwritten: %d", isLocal)
	}
	if localName == "本机" {
		t.Fatalf("local device display_name overwritten: %q", localName)
	}

	// 重复导入：行数不变（幂等）、设备不再新增、总量不翻倍
	h2, d2, s2, dev2, err := m.ImportUsage(hours, daily, sessions, names)
	if err != nil {
		t.Fatalf("re-import: %v", err)
	}
	if h2 != 1 || d2 != 1 || s2 != 1 || dev2 != 0 {
		t.Fatalf("re-import counts wrong: hours=%d daily=%d sessions=%d devices=%d", h2, d2, s2, dev2)
	}
	var hTotal2 int64
	if err := m.db.QueryRow(`SELECT SUM(total_tokens) FROM hour_usage WHERE device='dev-imported'`).Scan(&hTotal2); err != nil || hTotal2 != 150 {
		t.Fatalf("re-import total=%d err=%v (should stay 150)", hTotal2, err)
	}
}

// TestImportUsageRollback 验证事务性：设备注册失败时，三层用量全部回滚不落库。
func TestImportUsageRollback(t *testing.T) {
	m, err := New(filepath.Join(t.TempDir(), "import-rollback.db"))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer m.Close()

	// 拒绝特定 device_id 的触发器：registerDevicesTx 对该设备必然失败
	if _, err := m.Exec(`CREATE TRIGGER reject_imported_dev AFTER INSERT ON devices
		WHEN NEW.device_id = 'bad-device'
		BEGIN SELECT RAISE(ABORT, 'rejected'); END`); err != nil {
		t.Fatalf("create trigger: %v", err)
	}

	hours := []model.HourUsage{{
		Device: "bad-device", Source: "Claude Code", UsageDate: "2026-08-01", Hour: 10, Model: "m",
		InputTokens: 100, OutputTokens: 50, TotalTokens: 150, CostUSD: 0.01,
	}}
	daily := []model.DailyUsage{{
		Device: "bad-device", Source: "Claude Code", UsageDate: "2026-08-01", Model: "m",
		InputTokens: 100, OutputTokens: 50, TotalTokens: 150, CostUSD: 0.01,
	}}
	sessions := []model.SessionUsage{{
		Device: "bad-device", Source: "Claude Code", SessionID: "sess-1",
		LastActivity: "2026-08-01T10:00:00Z", InputTokens: 100, OutputTokens: 50, TotalTokens: 150, CostUSD: 0.01,
	}}
	names := map[string]string{"bad-device": "坏设备"}

	if _, _, _, _, err := m.ImportUsage(hours, daily, sessions, names); err == nil {
		t.Fatal("import should fail on rejected device, got nil")
	}

	// 失败后三表均无残留（事务回滚）
	for _, table := range []string{"hour_usage", "daily_usage", "session_usage"} {
		var cnt int
		if err := m.db.QueryRow(`SELECT COUNT(*) FROM ` + table + ` WHERE device='bad-device'`).Scan(&cnt); err != nil || cnt != 0 {
			t.Fatalf("%s should be empty after rollback, count=%d err=%v", table, cnt, err)
		}
	}
	// 设备也未被半注册
	var dCnt int
	if err := m.db.QueryRow(`SELECT COUNT(*) FROM devices WHERE device_id='bad-device'`).Scan(&dCnt); err != nil || dCnt != 0 {
		t.Fatalf("devices should not register bad-device, count=%d err=%v", dCnt, err)
	}
}
