package service

// Package service 的子文件，应用设置读写业务逻辑。
import (
	"fmt"
	"log"
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
		return model.AppConfig{AutoSyncSeconds: 30}
	}

	result := model.AppConfig{
		AutoSyncSeconds: s.migrateAutoSyncSeconds(cfg),
		CCSwitchDBPath:  cfg["cc_switch_db_path"],
	}

	if result.CCSwitchDBPath == "" {
		path, exists := config.CCSwitchDefaultPath()
		if exists {
			result.CCSwitchDBPath = path
			log.Printf("[service] GetSettings auto-detected cc-switch db at %s", path)
		} else {
			log.Printf("[service] GetSettings cc-switch db not found at %s", path)
		}
	}

	if s.collectionSvc != nil {
		s.collectionSvc.SetAutoSyncInterval(result.AutoSyncSeconds)
	}

	return result
}

// migrateAutoSyncSeconds 读取自动同步间隔（秒）。首次遇到旧「分钟」配置时迁移为秒并清旧 key。
func (s *SettingService) migrateAutoSyncSeconds(cfg map[string]string) int {
	if v, ok := cfg["auto_sync_seconds"]; ok && v != "" {
		return config.AtoiDef(v, 30)
	}
	seconds := 30 // 默认 30s
	if v, ok := cfg["auto_sync_minutes"]; ok && v != "" {
		if m := config.AtoiDef(v, 0); m > 0 {
			seconds = m * 60
		}
	}
	s.db.SetConfig("auto_sync_seconds", strconv.Itoa(seconds))
	s.db.SetConfig("auto_sync_minutes", "")
	return seconds
}

// EnsureDefaultCCSwitchPath 启动时检测 CC-Switch 默认路径并写入配置（若未配置）。
// 原 app.startup 逻辑，下沉至服务层以消除 app 层对数据库的直接访问。
func (s *SettingService) EnsureDefaultCCSwitchPath() {
	if s.db == nil {
		return
	}
	existing, _ := s.db.GetConfig("cc_switch_db_path")
	if existing != "" {
		return
	}
	if path, exists := config.CCSwitchDefaultPath(); exists {
		s.db.SetConfig("cc_switch_db_path", path)
		log.Printf("[service] EnsureDefaultCCSwitchPath auto-detected cc-switch db at %s", path)
	} else {
		log.Printf("[service] EnsureDefaultCCSwitchPath cc-switch db not found at %s", path)
	}
}

// SaveSettings 持久化设置并立即应用自动同步间隔。
func (s *SettingService) SaveSettings(cfg model.AppConfig) error {
	pairs := map[string]string{
		"auto_sync_seconds": strconv.Itoa(cfg.AutoSyncSeconds),
		"cc_switch_db_path": cfg.CCSwitchDBPath,
	}
	for k, v := range pairs {
		if err := s.db.SetConfig(k, v); err != nil {
			return fmt.Errorf("save config %s: %w", k, err)
		}
	}

	if s.collectionSvc != nil {
		s.collectionSvc.SetAutoSyncInterval(cfg.AutoSyncSeconds)
	}

	log.Printf("[service] SaveSettings ok autoSync=%ds", cfg.AutoSyncSeconds)
	return nil
}
