package collector

import (
	"os"
	"strconv"
	"time"
)

// ToUTCRFC3339 将采集来源的时间值规范化为 RFC3339 UTC 绝对时刻。
// 数字按 Unix 毫秒（<1e12 视为秒）、字符串按多种格式解析：
// 带时区/UTC 指示的字符串为绝对时刻直接解析；无时区字符串按采集机本地时区解释。
func ToUTCRFC3339(value interface{}) (string, bool) {
	if value == nil {
		return "", false
	}
	switch v := value.(type) {
	case int64:
		ms := v
		if ms <= 0 {
			return "", false
		}
		if ms < 1e12 {
			ms *= 1000
		}
		return time.UnixMilli(ms).UTC().Format(time.RFC3339), true
	case float64:
		ms := int64(v)
		if ms <= 0 {
			return "", false
		}
		if ms < 1e12 {
			ms *= 1000
		}
		return time.UnixMilli(ms).UTC().Format(time.RFC3339), true
	case string:
		if v == "" {
			return "", false
		}
		if t, err := parseTime(v); err == nil {
			return t.UTC().Format(time.RFC3339), true
		}
		if ms, err := strconv.ParseInt(v, 10, 64); err == nil && ms > 0 {
			if ms < 1e12 {
				ms *= 1000
			}
			return time.UnixMilli(ms).UTC().Format(time.RFC3339), true
		}
		return "", false
	default:
		return "", false
	}
}

// UTCDateFromTimestamp 提取 UTC 日期（YYYY-MM-DD）；解析失败返回 fallback。
func UTCDateFromTimestamp(value interface{}, fallback string) string {
	if rfc, ok := ToUTCRFC3339(value); ok {
		if t, err := time.Parse(time.RFC3339, rfc); err == nil {
			return t.UTC().Format("2006-01-02")
		}
	}
	return fallback
}

// UTCHour 从 RFC3339 UTC 串提取 UTC 小时；失败返回 -1。
func UTCHour(rfc3339 string) int {
	if rfc3339 == "" {
		return -1
	}
	t, err := time.Parse(time.RFC3339, rfc3339)
	if err != nil {
		return -1
	}
	return t.UTC().Hour()
}

func parseTime(s string) (time.Time, error) {
	// 带时区指示的格式优先（绝对时刻，保留原始时区偏移）
	for _, f := range []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02T15:04:05Z07:00",
	} {
		if t, err := time.Parse(f, s); err == nil {
			return t, nil
		}
	}
	// 无时区格式按采集机本地时区解释（原始数据为本地时间语义）
	for _, f := range []string{
		"2006-01-02T15:04:05",
		"2006-01-02 15:04:05",
		"2006-01-02",
	} {
		if t, err := time.ParseInLocation(f, s, time.Local); err == nil {
			return t, nil
		}
	}
	return time.Time{}, &time.ParseError{Layout: "?", Value: s}
}

// Hostname 返回本机主机名，供采集运行记录使用（orchestrator 等包共用）。
func Hostname() string {
	h, err := os.Hostname()
	if err != nil {
		return "unknown"
	}
	return h
}

// PosInt converts a numeric value to a non-negative int64.
func PosInt(v interface{}) int64 {
	switch n := v.(type) {
	case int64:
		if n > 0 {
			return n
		}
	case float64:
		if n > 0 {
			return int64(n)
		}
	case int:
		if n > 0 {
			return int64(n)
		}
	}
	return 0
}
