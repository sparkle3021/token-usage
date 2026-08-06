package database

import (
	"path/filepath"
	"testing"
	"time"
)

// TestLocalBucketToUTCRoundTrip 验证本地桶 → UTC 桶换算后，按查询层反向平移能还原。
// 不依赖具体时区：任意本机时区下往返一致即证明换算正确。
func TestLocalBucketToUTCRoundTrip(t *testing.T) {
	cases := []struct{ date string; hour int }{
		{"2026-08-01", 0}, {"2026-08-01", 12}, {"2026-08-01", 23},
		{"2026-08-02", 1}, {"2026-12-31", 23},
	}
	for _, c := range cases {
		uDate, uHour := localBucketToUTC(c.date, c.hour)
		// 反向：UTC 桶 → 本机时区
		ts, err := time.ParseInLocation("2006-01-02", uDate, time.UTC)
		if err != nil {
			t.Fatalf("parse utc date %q: %v", uDate, err)
		}
		ts = ts.Add(time.Duration(uHour) * time.Hour).In(time.Local)
		if gotDate, gotHour := ts.Format("2006-01-02"), ts.Hour(); gotDate != c.date || gotHour != c.hour {
			t.Fatalf("round trip mismatch: %s:%d -> utc %s:%d -> local %s:%d",
				c.date, c.hour, uDate, uHour, gotDate, gotHour)
		}
	}
}

// TestUTCBucketToLocalRoundTrip 验证 UTC 桶平移本机后，反向（本机→UTC）能还原。
func TestUTCBucketToLocalRoundTrip(t *testing.T) {
	for _, c := range []struct{ date string; hour int }{
		{"2026-08-01", 0}, {"2026-08-01", 12}, {"2026-08-01", 23},
		{"2026-08-02", 1}, {"2026-12-31", 23},
	} {
		lDate, lHour := UTCBucketToLocal(c.date, c.hour)
		uDate, uHour := localBucketToUTC(lDate, lHour)
		if uDate != c.date || uHour != c.hour {
			t.Fatalf("round trip mismatch: utc %s:%d -> local %s:%d -> utc %s:%d",
				c.date, c.hour, lDate, lHour, uDate, uHour)
		}
	}
}

// TestMigrateHourTimezone 验证存量本地桶 hour 迁移为 UTC 桶：平移一致 + 幂等（标记后跳过）。
func TestMigrateHourTimezone(t *testing.T) {
	m, err := New(filepath.Join(t.TempDir(), "tz.db"))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer m.Close()

	// 模拟存量本地桶数据
	now := time.Now()
	localDate := now.Format("2006-01-02")
	localHour := now.Hour()
	if _, err := m.Exec(`INSERT INTO hour_usage
		(device, source, usage_date, hour, model,
		 input_tokens, output_tokens, cache_creation_tokens, cache_read_tokens,
		 reasoning_output_tokens, total_tokens, cost_usd)
		VALUES ('dev1', 'Claude Code', ?, ?, 'model-x', 10, 20, 0, 0, 0, 30, 0.001)`,
		localDate, localHour); err != nil {
		t.Fatalf("insert hour: %v", err)
	}

	// New() 对空库已标记迁移完成；此处模拟"有存量数据的库首次迁移"（清除标记）
	if _, err := m.Exec(`DELETE FROM app_config WHERE key = ?`, tzMigratedKey); err != nil {
		t.Fatalf("clear tz marker: %v", err)
	}

	if err := m.migrateHourTimezone(); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	// 断言：迁移后桶按查询层反向平移回到原本地桶（展示不变）
	rows, err := m.QueryHourUsage(0)
	if err != nil {
		t.Fatalf("query hour: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected 1 hour row after migrate, got %d", len(rows))
	}
	r := rows[0]
	ts, _ := time.ParseInLocation("2006-01-02", r.UsageDate, time.UTC)
	ts = ts.Add(time.Duration(r.Hour) * time.Hour).In(time.Local)
	if ts.Format("2006-01-02") != localDate || ts.Hour() != localHour {
		t.Fatalf("migrated bucket inconsistent: utc=%s:%d -> local=%s:%d, want %s:%d",
			r.UsageDate, r.Hour, ts.Format("2006-01-02"), ts.Hour(), localDate, localHour)
	}

	// 幂等：迁移后再插入新数据（UTC 桶），重跑迁移应跳过不重写
	if _, err := m.Exec(`INSERT INTO hour_usage
		(device, source, usage_date, hour, model, input_tokens, output_tokens, total_tokens)
		VALUES ('dev1', 'Codex CLI', '2026-08-01', 0, 'model-y', 5, 5, 10)`); err != nil {
		t.Fatalf("insert new hour: %v", err)
	}
	if err := m.migrateHourTimezone(); err != nil {
		t.Fatalf("re-migrate: %v", err)
	}
	var cnt int
	if err := m.db.QueryRow(`SELECT COUNT(*) FROM hour_usage WHERE source='Codex CLI' AND usage_date='2026-08-01' AND hour=0`).Scan(&cnt); err != nil || cnt != 1 {
		t.Fatalf("re-migrate should skip, codex row count=%d err=%v", cnt, err)
	}
}
