package service

import (
	"testing"
	"time"
)

// TestLocalizeUTCDateHourRoundTrip 验证 UTC 桶平移本机后，反向（本机→UTC）能还原。
// 不依赖具体时区，证明平移与存量迁移互为逆运算。
func TestLocalizeUTCDateHourRoundTrip(t *testing.T) {
	for _, c := range []struct{ date string; hour int }{
		{"2026-08-01", 0}, {"2026-08-01", 12}, {"2026-08-02", 23}, {"2026-12-31", 1},
	} {
		lDate, lHour := localizeUTCDateHour(c.date, c.hour)
		// 反向：把本地桶当本机时刻转 UTC（与 database.localBucketToUTC 同逻辑）
		ts, err := time.ParseInLocation("2006-01-02", lDate, time.Local)
		if err != nil {
			t.Fatalf("parse local date %q: %v", lDate, err)
		}
		ts = ts.Add(time.Duration(lHour) * time.Hour).UTC()
		if gotDate, gotHour := ts.Format("2006-01-02"), ts.Hour(); gotDate != c.date || gotHour != c.hour {
			t.Fatalf("round trip mismatch: utc %s:%d -> local %s:%d -> utc %s:%d",
				c.date, c.hour, lDate, lHour, gotDate, gotHour)
		}
	}
}
