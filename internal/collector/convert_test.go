package collector

import "testing"

// TestToUTCRFC3339 验证时间规范化：带时区/偏移/Unix 毫秒/无时区（按本机时区解释）。
func TestToUTCRFC3339(t *testing.T) {
	// RFC3339 带 Z（绝对时刻）
	if s, ok := ToUTCRFC3339("2026-08-01T10:00:00Z"); !ok || s != "2026-08-01T10:00:00Z" {
		t.Fatalf("Z string: got %q ok=%v", s, ok)
	}
	// 带时区偏移 → 归一为 UTC
	if s, ok := ToUTCRFC3339("2026-08-01T10:00:00+08:00"); !ok || s != "2026-08-01T02:00:00Z" {
		t.Fatalf("offset string: got %q ok=%v", s, ok)
	}
	// Unix 毫秒
	if s, ok := ToUTCRFC3339(int64(1700000000000)); !ok || s != "2023-11-14T22:13:20Z" {
		t.Fatalf("unix ms: got %q ok=%v", s, ok)
	}
	// Unix 秒（<1e12 视为秒）
	if s, ok := ToUTCRFC3339(int64(1700000000)); !ok || s != "2023-11-14T22:13:20Z" {
		t.Fatalf("unix s: got %q ok=%v", s, ok)
	}
	// 无时区字符串：按本机时区解释，结果应为 RFC3339 且可解析（不依赖具体时区）
	if s, ok := ToUTCRFC3339("2026-08-01T10:00:00"); !ok || len(s) < 20 {
		t.Fatalf("naive string: got %q ok=%v", s, ok)
	}
	// 非法输入
	if _, ok := ToUTCRFC3339("not-a-time"); ok {
		t.Fatal("invalid string should fail")
	}
	if _, ok := ToUTCRFC3339(nil); ok {
		t.Fatal("nil should fail")
	}
}

// TestUTCDateFromTimestamp 验证 UTC 日期提取。
func TestUTCDateFromTimestamp(t *testing.T) {
	if got := UTCDateFromTimestamp("2026-08-01T10:00:00Z", "x"); got != "2026-08-01" {
		t.Fatalf("Z date: got %q", got)
	}
	if got := UTCDateFromTimestamp(int64(1700000000000), "x"); got != "2023-11-14" {
		t.Fatalf("ms date: got %q", got)
	}
	if got := UTCDateFromTimestamp("garbage", "fallback"); got != "fallback" {
		t.Fatalf("fallback: got %q", got)
	}
}
