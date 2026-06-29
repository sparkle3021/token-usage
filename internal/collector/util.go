package collector

import (
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// ---------------------------------------------------------------------------
// Config helpers (path expansion + env overrides)
// ---------------------------------------------------------------------------

// ExpandPath replaces ~ with home dir and resolves $VAR/${VAR} env references.
func ExpandPath(value string) string {
	if value == "" {
		return ""
	}

	home, _ := os.UserHomeDir()

	expanded := value
	if expanded == "~" {
		return home
	} else if strings.HasPrefix(expanded, "~/") {
		expanded = home + expanded[1:]
	}

	expanded = os.Expand(expanded, os.Getenv)
	info, err := os.Stat(expanded)
	if err != nil {
		return expanded
	}
	_ = info
	return expanded
}

// EnvPathList returns a list of expanded paths from an env var (comma-separated).
func EnvPathList(value string, fallback []string) []string {
	paths := strings.Split(strings.TrimSpace(value), ",")
	var result []string
	for _, p := range paths {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		result = append(result, ExpandPath(p))
	}
	if len(result) > 0 {
		return result
	}
	return fallback
}

// ---------------------------------------------------------------------------
// Timestamp helpers
// ---------------------------------------------------------------------------

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

// NormalizeModelForGrouping normalizes a model ID for aggregation.
func NormalizeModelForGrouping(modelID string) string {
	name := strings.TrimSpace(strings.ToLower(modelID))
	if name == "" {
		return "unknown"
	}

	// Strip reasoning tier suffix: "claude-sonnet-4-20250514 (high)" -> "claude-sonnet-4-20250514"
	reasoningTiers := map[string]bool{
		"minimal": true, "low": true, "medium": true, "high": true, "xhigh": true, "auto": true, "none": true,
	}
	if strings.HasSuffix(name, ")") {
		openIdx := strings.LastIndex(name, "(")
		if openIdx > 0 {
			tier := strings.TrimSpace(name[openIdx+1 : len(name)-1])
			if reasoningTiers[tier] {
				name = strings.TrimSpace(name[:openIdx])
			}
		}
	}

	// Strip trailing date suffix like "-20250514"
	if len(name) > 9 {
		suffix := name[len(name)-8:]
		if isDigits8(suffix) && name[len(name)-9] == '-' {
			name = name[:len(name)-9]
		}
	}

	// Claude models: normalize dots to hyphens between digits
	if strings.Contains(name, "claude") {
		name = strings.ReplaceAll(name, ".", "-")
	}

	return name
}

func isDigits8(s string) bool {
	if len(s) != 8 {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// CanonicalProvider normalizes a raw provider string.
func CanonicalProvider(raw string) string {
	if raw == "" {
		return ""
	}
	parts := strings.Split(strings.ReplaceAll(strings.ToLower(raw), "-", "_"), "/")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" || part == "unknown" {
			continue
		}
		switch part {
		case "x_ai", "xai":
			return "xai"
		case "z_ai", "zai":
			return "zai"
		case "moonshot", "moonshotai":
			return "moonshotai"
		case "meta", "meta_llama":
			return "meta_llama"
		case "azure", "azure_ai":
			return "azure_ai"
		case "anthropic", "vertex", "vertex_ai":
			return "anthropic"
		case "together", "together_ai":
			return "together_ai"
		case "fireworks", "fireworks_ai":
			return "fireworks_ai"
		case "google", "gemini":
			return "google"
		case "openai", "openai_codex":
			return "openai"
		case "mistral", "mistralai":
			return "mistralai"
		case "deepseek":
			return "deepseek"
		case "qwen":
			return "qwen"
		}
		if !containsDigit(part) {
			return part
		}
	}
	return ""
}

func containsDigit(s string) bool {
	for _, r := range s {
		if r >= '0' && r <= '9' {
			return true
		}
	}
	return false
}

// InferProviderFromModel tries to guess the provider from a model ID string.
func InferProviderFromModel(model string) string {
	lower := strings.ToLower(model)
	switch {
	case strings.Contains(lower, "claude"), strings.Contains(lower, "anthropic"):
		return "anthropic"
	case strings.Contains(lower, "gpt"), strings.Contains(lower, "openai"):
		return "openai"
	case strings.Contains(lower, "gemini"), strings.Contains(lower, "google"):
		return "google"
	case strings.Contains(lower, "grok"):
		return "xai"
	case strings.Contains(lower, "deepseek"):
		return "deepseek"
	case strings.Contains(lower, "mimo"), strings.Contains(lower, "xiaomi"):
		return "xiaomi"
	case strings.Contains(lower, "mistral"), strings.Contains(lower, "mixtral"):
		return "mistral"
	case strings.Contains(lower, "llama"):
		return "meta_llama"
	case strings.Contains(lower, "qwen"):
		return "qwen"
	case strings.Contains(lower, "glm"):
		return "zai"
	}
	return ""
}

// ---------------------------------------------------------------------------
func hostname() string {
	h, err := os.Hostname()
	if err != nil {
		return "unknown"
	}
	return h
}

// JSONL file scanner
// ---------------------------------------------------------------------------

// CollectJSONLFiles recursively finds all .jsonl files under a directory.
func CollectJSONLFiles(dir string) []string {
	var results []string
	filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil // skip inaccessible
		}
		if !d.IsDir() && strings.HasSuffix(strings.ToLower(d.Name()), ".jsonl") {
			results = append(results, path)
		}
		return nil
	})
	if len(results) > 0 {
		log.Printf("[collector] CollectJSONLFiles dir=%s files=%d", dir, len(results))
	}
	return results
}

// ---------------------------------------------------------------------------
// Non-negative helpers
// ---------------------------------------------------------------------------

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

// ---------------------------------------------------------------------------
// Parse cache (file fingerprint)
// ---------------------------------------------------------------------------

// FileFingerprint returns a fingerprint string (mtime:size) for a file.
func FileFingerprint(path string) string {
	info, err := os.Stat(path)
	if err != nil {
		return ""
	}
	return formatFingerprint(info.ModTime().UnixMilli(), info.Size())
}

func formatFingerprint(mtimeMs int64, size int64) string {
	return strings.Join([]string{
		int64ToStr(mtimeMs),
		int64ToStr(size),
	}, ":")
}

func int64ToStr(n int64) string {
	if n == 0 {
		return "0"
	}
	negative := false
	if n < 0 {
		negative = true
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if negative {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}

// ParseCache is a simple file-fingerprint based parse cache.
type ParseCache struct {
	mu      sync.Mutex
	version int
	store   map[string]*cacheEntry // path -> entry
}

type cacheEntry struct {
	Fingerprint string
	Records     interface{}
}

func NewParseCache(version int) *ParseCache {
	return &ParseCache{
		version: version,
		store:   make(map[string]*cacheEntry),
	}
}

// Get returns cached records if the file is unchanged.
func (c *ParseCache) Get(filePath string) (interface{}, bool) {
	fp := FileFingerprint(filePath)
	if fp == "" {
		return nil, false
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	entry, ok := c.store[filePath]
	if ok && entry.Fingerprint == fp {
		return entry.Records, true
	}
	return nil, false
}

// Set stores parsed records for a file.
func (c *ParseCache) Set(filePath string, records interface{}) {
	fp := FileFingerprint(filePath)
	if fp == "" {
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	c.store[filePath] = &cacheEntry{
		Fingerprint: fp,
		Records:     records,
	}
}
