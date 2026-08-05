package service

import "testing"

// TestParseAndValidate 验证导入文件校验：有效通过、format/version 不匹配拒绝、非 JSON 拒绝。
func TestParseAndValidate(t *testing.T) {
	valid := `{
		"format": "token-usage-export", "version": 1, "exportedAt": "2026-08-05T10:00:00+08:00",
		"deviceNames": {"dev-1": "设备一"},
		"hourUsage": [], "dailyUsage": [], "sessionUsage": []
	}`
	p, err := parseAndValidate([]byte(valid))
	if err != nil {
		t.Fatalf("valid payload rejected: %v", err)
	}
	if p.Format != exportFormat || p.DeviceNames["dev-1"] != "设备一" {
		t.Fatalf("payload not parsed correctly: %+v", p)
	}

	cases := []struct{ name, body string }{
		{"format 不匹配", `{"format":"other","version":1}`},
		{"version 不匹配", `{"format":"token-usage-export","version":2}`},
		{"非 JSON", `not-json`},
	}
	for _, c := range cases {
		if _, err := parseAndValidate([]byte(c.body)); err == nil {
			t.Fatalf("%s: expected error, got nil", c.name)
		}
	}
}
