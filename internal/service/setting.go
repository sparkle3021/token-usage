package service

// Package service 的子文件，应用设置读写业务逻辑。
import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"

	"token-dashboard/internal/config"
	"token-dashboard/internal/database"
	"token-dashboard/internal/model"
)

// SettingService 应用设置读写业务逻辑，负责设置持久化和自动同步联动。
type SettingService struct {
	db *database.Manager
	collectionSvc *CollectionService // 设置变更后同步更新自动同步间隔
}

// NewSettingService 创建设置服务实例。
func NewSettingService(db *database.Manager, collectionSvc *CollectionService) *SettingService {
	return &SettingService{db: db, collectionSvc: collectionSvc}
}

// GetSettings 读取持久化设置，自动检测 CC-Switch 数据库路径。
// 同步更新运行时的自动同步间隔到 CollectionService。
func (s *SettingService) GetSettings() model.AppConfig {
	cfg, err := s.db.GetAllConfigs()
	if err != nil {
		return model.AppConfig{AutoSyncMinutes: 5}
	}

	result := model.AppConfig{
		AutoSyncMinutes: config.AtoiDef(cfg["auto_sync_minutes"], 5),
		CCSwitchDBPath:  cfg["cc_switch_db_path"],
	}

	if result.CCSwitchDBPath == "" {
		if home, err := os.UserHomeDir(); err == nil {
			defaultPath := filepath.Join(home, ".cc-switch", "cc-switch.db")
			if _, err := os.Stat(defaultPath); err == nil {
				result.CCSwitchDBPath = defaultPath
				log.Printf("[service] GetSettings auto-detected cc-switch db at %s", defaultPath)
			} else {
				log.Printf("[service] GetSettings cc-switch db not found at %s", defaultPath)
			}
		}
	}

	if s.collectionSvc != nil {
		s.collectionSvc.SetAutoSyncInterval(result.AutoSyncMinutes)
	}

	return result
}

// SaveSettings 持久化设置并立即应用自动同步间隔。
func (s *SettingService) SaveSettings(cfg model.AppConfig) error {
	pairs := map[string]string{
		"auto_sync_minutes": strconv.Itoa(cfg.AutoSyncMinutes),
		"cc_switch_db_path": cfg.CCSwitchDBPath,
	}
	for k, v := range pairs {
		if err := s.db.SetConfig(k, v); err != nil {
			return fmt.Errorf("save config %s: %w", k, err)
		}
	}

	if s.collectionSvc != nil {
		s.collectionSvc.SetAutoSyncInterval(cfg.AutoSyncMinutes)
	}

	log.Printf("[service] SaveSettings ok autoSync=%d", cfg.AutoSyncMinutes)
	return nil
}
