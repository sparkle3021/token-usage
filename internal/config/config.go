// Package config 集中管理应用配置，包括数据目录解析、环境变量读取和默认值。
// 所有 os.Getenv 调用均收敛于此，避免散落在各包中。
package config

import (
	"log"
	"os"
	"path/filepath"
	"strconv"

	"token-dashboard/internal/config/seed"
)

// Config 应用配置，从环境变量和默认值加载。
type Config struct {
	DataDir              string // 数据目录，默认 ~/.token-dashboard，可用 DATA_DIR 覆盖
	DBPath               string // SQLite 数据库路径
	CollectorParallelism int    // 采集并发数，默认 4，环境变量 COLLECTOR_PARALLELISM
}

// Load 加载配置，解析环境变量并初始化数据目录。
// 首次运行时自动写入定价兜底文件，确保分发后无需外部文件即可使用。
func Load() *Config {
	dataDir := resolveDataDir()
	ensurePricingDefaults(dataDir)
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

	os.MkdirAll(target, 0755)
	return target
}

// ensurePricingDefaults 将嵌入的默认定价数据写入配置目录，仅当文件不存在时写入。
func ensurePricingDefaults(dataDir string) {
	priceDir := filepath.Join(dataDir, "config")
	if err := os.MkdirAll(priceDir, 0755); err != nil {
		log.Printf("[config] create price dir error: %v", err)
		return
	}

	defaults := map[string][]byte{
		"pricing-litellm.json":   seed.PricingLitellm,
		"pricing-openrouter.json": seed.PricingOpenRouter,
	}

	for name, data := range defaults {
		dst := filepath.Join(priceDir, name)
		if _, err := os.Stat(dst); err == nil {
			continue // 已存在，不覆盖
		}
		if err := os.WriteFile(dst, data, 0644); err != nil {
			log.Printf("[config] write pricing default %s error: %v", name, err)
		} else {
			log.Printf("[config] wrote pricing default %s (%d bytes)", name, len(data))
		}
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


