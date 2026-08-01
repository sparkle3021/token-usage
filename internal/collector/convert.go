package collector

import (
	"os"
	"time"
)

// LocalDateFromTimestamp extracts a YYYY-MM-DD local date from a timestamp.
func LocalDateFromTimestamp(value interface{}, fallback string) string {
	if value == nil {
		return fallback
	}

	var ms int64
	switch v := value.(type) {
	case int64:
		ms = v
	case float64:
		ms = int64(v)
	case string:
		if v == "" {
			return fallback
		}
		t, err := parseTime(v)
		if err != nil {
			return fallback
		}
		return t.Local().Format("2006-01-02")
	default:
		return fallback
	}

	if ms > 1e12 {
		// already ms
	} else {
		ms *= 1000
	}

	return time.UnixMilli(ms).Local().Format("2006-01-02")
}

func parseTime(s string) (time.Time, error) {
	formats := []string{
		time.RFC3339,
		time.RFC3339Nano,
		"2006-01-02T15:04:05",
		"2006-01-02 15:04:05",
		"2006-01-02T15:04:05Z",
		"2006-01-02",
	}
	for _, f := range formats {
		if t, err := time.Parse(f, s); err == nil {
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
