// Package config 集中管理应用配置，包括数据目录解析、环境变量读取和默认值。
// 所有 os.Getenv 调用均收敛于此，避免散落在各包中。
package config

import (
	"log"
	"os"
	"path/filepath"
	"strconv"
)

// Config 应用配置，从环境变量和默认值加载。
type Config struct {
	DataDir              string // 数据目录，默认 ~/.token-usage，可用 DATA_DIR 覆盖
	DBPath               string // SQLite 数据库路径
	CollectorParallelism int    // 采集并发数，默认 4，环境变量 COLLECTOR_PARALLELISM
}

// Load 加载配置，解析环境变量并初始化数据目录。
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
	target := filepath.Join(home, ".token-usage")

	os.MkdirAll(target, 0755)
	return target
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

// CCSwitchDefaultPath 返回 CC-Switch 数据库的默认路径（~/.cc-switch/cc-switch.db），
// 并检测该路径是否存在。三处使用方（app startup、设置服务、导入服务）共用此 helper。
func CCSwitchDefaultPath() (path string, exists bool) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", false
	}
	path = filepath.Join(home, ".cc-switch", "cc-switch.db")
	_, err = os.Stat(path)
	return path, err == nil
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


