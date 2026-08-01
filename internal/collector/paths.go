package collector

import (
	"os"
	"path/filepath"
	"strings"
	"time"

	"token-dashboard/internal/debuglog"
)

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

// CollectJSONLFiles recursively finds all .jsonl files under a directory.
func CollectJSONLFiles(dir string) []string {
	start := time.Now()
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
		debuglog.Perf("CollectJSONLFiles dir=%s files=%d elapsed=%v", dir, len(results), time.Since(start))
	}
	return results
}
