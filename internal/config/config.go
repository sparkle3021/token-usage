// Package config 集中管理应用配置，包括数据目录解析、环境变量读取和默认值。
// 所有 os.Getenv 调用均收敛于此，避免散落在各包中。
package config

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// Config 应用配置，从环境变量和默认值加载。
type Config struct {
	DataDir              string // 数据目录，默认 ~/.token-dashboard，可用 DATA_DIR 覆盖
	DBPath               string // SQLite 数据库路径
	CollectorParallelism int    // 采集并发数，默认 4，环境变量 COLLECTOR_PARALLELISM
}

// Load 加载配置，解析环境变量并初始化数据目录。
// 首次运行时自动从 data/ 目录迁移旧数据。
func Load() *Config {
	dataDir := resolveDataDir()
	return &Config{
		DataDir:              dataDir,
		DBPath:               filepath.Join(dataDir, "td.db"),
		CollectorParallelism: getEnvInt("COLLECTOR_PARALLELISM", 4),
	}
}

func resolveDataDir() string {
	if env := os.Getenv("DATA_DIR"); env != "" {
		abs, err := filepath.Abs(env)
		if err == nil {
			return abs
		}
		return env
	}

	home, err := os.UserHomeDir()
	if err != nil {
		log.Printf("[config] resolveDataDir cannot get home dir, falling back to ./data: %v", err)
		abs, _ := filepath.Abs("data")
		return abs
	}
	target := filepath.Join(home, ".token-dashboard")

	if err := migrateFromOldData(target); err != nil {
		log.Printf("[config] resolveDataDir migration warning: %v", err)
	}

	migratePricingToSubdir(target)

	os.MkdirAll(target, 0755)
	return target
}

func migrateFromOldData(target string) error {
	if _, err := os.Stat(filepath.Join(target, "td.db")); err == nil {
		return nil
	}

	oldDir := "data"
	if _, err := os.Stat(filepath.Join(oldDir, "usage.sqlite")); err != nil {
		return nil
	}

	log.Printf("[config] migrating data/ → %s", target)
	if err := os.MkdirAll(target, 0755); err != nil {
		return fmt.Errorf("create target dir: %w", err)
	}

	priceDir := filepath.Join(target, "config")
	os.MkdirAll(priceDir, 0755)

	entries, err := os.ReadDir(oldDir)
	if err != nil {
		return fmt.Errorf("read old dir: %w", err)
	}

	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		src := filepath.Join(oldDir, e.Name())

		dstDir := target
		if strings.HasPrefix(e.Name(), "pricing-") {
			dstDir = priceDir
		}

		dst := filepath.Join(dstDir, e.Name())
		data, err := os.ReadFile(src)
		if err != nil {
			log.Printf("[config] migrate skip %s: %v", e.Name(), err)
			continue
		}
		if err := os.WriteFile(dst, data, 0644); err != nil {
			return fmt.Errorf("write %s: %w", e.Name(), err)
		}
		log.Printf("[config] migrate copied %s", e.Name())
	}

	log.Printf("[config] migration complete: data/ → %s", target)
	return nil
}

func migratePricingToSubdir(target string) {
	priceDir := filepath.Join(target, "config")
	os.MkdirAll(priceDir, 0755)

	for _, name := range []string{"pricing-litellm.json", "pricing-openrouter.json"} {
		src := filepath.Join(target, name)
		if _, err := os.Stat(src); err != nil {
			continue
		}
		dst := filepath.Join(priceDir, name)
		if _, err := os.Stat(dst); err == nil {
			os.Remove(src)
			log.Printf("[config] migratePricingToSubdir cleaned up %s", name)
			continue
		}
		data, err := os.ReadFile(src)
		if err != nil {
			log.Printf("[config] migratePricingToSubdir skip %s: %v", name, err)
			continue
		}
		if err := os.WriteFile(dst, data, 0644); err != nil {
			log.Printf("[config] migratePricingToSubdir write %s: %v", name, err)
			continue
		}
		os.Remove(src)
		log.Printf("[config] migratePricingToSubdir moved %s → price/", name)
	}
}

func getEnvInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		n, err := strconv.Atoi(v)
		if err == nil {
			return n
		}
	}
	return def
}

func AtoiDef(s string, def int) int {
	if s == "" {
		return def
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return def
	}
	return n
}

func PadNum(s string) string {
	s = strings.TrimSpace(s)
	if len(s) == 1 {
		return "0" + s
	}
	return s
}

func NormalizeCSVDate(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	parts := strings.Split(s, "/")
	if len(parts) == 3 && len(parts[0]) == 4 {
		return fmt.Sprintf("%s-%s-%s", parts[0], PadNum(parts[1]), PadNum(parts[2]))
	}
	for _, layout := range []string{"2006-01-02", "2006/1/2", time.RFC3339[:10]} {
		if t, err := time.Parse(layout, s); err == nil {
			return t.Format("2006-01-02")
		}
	}
	return ""
}
