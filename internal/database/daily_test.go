package database

import (
	"path/filepath"
	"testing"

	"token-dashboard/internal/model"
)

// TestBuildDailyFromHourUsageLocalDay 验证重建按本地日聚合：跨 UTC 日界的桶并入同一本地日，
// 各本地日总量与 UTCBucketToLocal 平移后的期望一致。
func TestBuildDailyFromHourUsageLocalDay(t *testing.T) {
	m, err := New(filepath.Join(t.TempDir(), "d.db"))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer m.Close()

	rows := []model.HourUsage{
		{Device: "d1", Source: "Claude Code", UsageDate: "2026-08-04", Hour: 16, Model: "m1", TotalTokens: 100, InputTokens: 100},
		{Device: "d1", Source: "Claude Code", UsageDate: "2026-08-05", Hour: 0, Model: "m1", TotalTokens: 50, InputTokens: 50},
		{Device: "d1", Source: "Claude Code", UsageDate: "2026-08-05", Hour: 8, Model: "m1", TotalTokens: 30, InputTokens: 30},
	}
	if err := m.BulkUpsertHourUsage(rows); err != nil {
		t.Fatalf("insert hour: %v", err)
	}

	if err := m.BuildDailyFromHourUsage(); err != nil {
		t.Fatalf("rebuild: %v", err)
	}

	got, err := m.QueryDaily()
	if err != nil {
		t.Fatalf("query daily: %v", err)
	}
	totals := map[string]int64{}
	for _, r := range got {
		totals[r.UsageDate] += r.TotalTokens
	}

	// 期望：按 UTCBucketToLocal 平移后的本地日期分组
	want := map[string]int64{}
	for _, r := range rows {
		ld, _ := UTCBucketToLocal(r.UsageDate, r.Hour)
		want[ld] += r.TotalTokens
	}
	if len(totals) != len(want) {
		t.Fatalf("local-day count mismatch: got %d want %d (totals=%v want=%v)", len(totals), len(want), totals, want)
	}
	for d, v := range want {
		if totals[d] != v {
			t.Fatalf("local day %s total=%d want %d (totals=%v)", d, totals[d], v, totals)
		}
	}
}

// TestBuildDailyFromHourUsageConsistency 验证重建后 daily 各 token 字段总和与 hour 全量一致。
func TestBuildDailyFromHourUsageConsistency(t *testing.T) {
	m, err := New(filepath.Join(t.TempDir(), "d.db"))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer m.Close()

	rows := []model.HourUsage{
		{Device: "d1", Source: "Claude Code", UsageDate: "2026-08-03", Hour: 23, Model: "m1",
			InputTokens: 10, OutputTokens: 20, CacheCreationTokens: 5, CacheReadTokens: 8,
			ReasoningOutputTokens: 3, TotalTokens: 46, CostUSD: 0.5},
		{Device: "d1", Source: "Claude Code", UsageDate: "2026-08-04", Hour: 1, Model: "m2",
			InputTokens: 100, OutputTokens: 200, CacheCreationTokens: 50, CacheReadTokens: 80,
			ReasoningOutputTokens: 30, TotalTokens: 460, CostUSD: 5.0},
		{Device: "d2", Source: "OpenCode", UsageDate: "2026-08-04", Hour: 8, Model: "m1",
			InputTokens: 7, OutputTokens: 3, CacheCreationTokens: 0, CacheReadTokens: 2,
			ReasoningOutputTokens: 1, TotalTokens: 13, CostUSD: 0.2},
	}
	if err := m.BulkUpsertHourUsage(rows); err != nil {
		t.Fatalf("insert hour: %v", err)
	}

	if err := m.BuildDailyFromHourUsage(); err != nil {
		t.Fatalf("rebuild: %v", err)
	}

	daily, err := m.QueryDaily()
	if err != nil {
		t.Fatalf("query daily: %v", err)
	}
	type sum struct{ in, out, cc, cr, re, tot int64 }
	dsum, hsum := sum{}, sum{}
	for _, r := range daily {
		dsum.in += r.InputTokens
		dsum.out += r.OutputTokens
		dsum.cc += r.CacheCreationTokens
		dsum.cr += r.CacheReadTokens
		dsum.re += r.ReasoningOutputTokens
		dsum.tot += r.TotalTokens
	}
	for _, r := range rows {
		hsum.in += r.InputTokens
		hsum.out += r.OutputTokens
		hsum.cc += r.CacheCreationTokens
		hsum.cr += r.CacheReadTokens
		hsum.re += r.ReasoningOutputTokens
		hsum.tot += r.TotalTokens
	}
	if dsum != hsum {
		t.Fatalf("daily sum %+v != hour sum %+v", dsum, hsum)
	}
}

// TestBuildDailyFromHourUsageOverwritesStaleValue 验证重建以聚合值覆盖旧 daily 行（token 字段），
// 而非 MAX 保留历史冻结值。
func TestBuildDailyFromHourUsageOverwritesStaleValue(t *testing.T) {
	m, err := New(filepath.Join(t.TempDir(), "d.db"))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer m.Close()

	lDate, _ := UTCBucketToLocal("2026-08-04", 12)

	// 预插一条"MAX 冻结"的旧 daily 大值（与实际本地日期对齐）
	if err := m.BulkUpsertDaily([]model.DailyUsage{
		{Device: "d1", Source: "Claude Code", UsageDate: lDate, Model: "m1", TotalTokens: 999999},
	}); err != nil {
		t.Fatalf("insert stale daily: %v", err)
	}
	// hour 层只有 100
	if err := m.BulkUpsertHourUsage([]model.HourUsage{
		{Device: "d1", Source: "Claude Code", UsageDate: "2026-08-04", Hour: 12, Model: "m1", TotalTokens: 100},
	}); err != nil {
		t.Fatalf("insert hour: %v", err)
	}

	if err := m.BuildDailyFromHourUsage(); err != nil {
		t.Fatalf("rebuild: %v", err)
	}

	got, err := m.QueryDaily()
	if err != nil {
		t.Fatalf("query daily: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 daily row, got %d (%+v)", len(got), got)
	}
	if got[0].TotalTokens != 100 {
		t.Fatalf("stale value not overwritten: got total=%d want 100", got[0].TotalTokens)
	}
}
