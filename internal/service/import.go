package service

// Package service 的子文件，CC-Switch 导入业务逻辑。
import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"token-dashboard/internal/collector/orchestrator"
	"token-dashboard/internal/database"
	"token-dashboard/internal/model"
)

// ImportService CC-Switch 导入和 CSV 导入业务逻辑。
type ImportService struct {
	db     *database.Manager
	engine *orchestrator.Engine
}

// NewImportService 创建导入服务实例。
func NewImportService(db *database.Manager, eng *orchestrator.Engine) *ImportService {
	return &ImportService{db: db, engine: eng}
}

func (s *ImportService) DetectCCSwitchDB() string {
	if home, err := os.UserHomeDir(); err == nil {
		defaultPath := filepath.Join(home, ".cc-switch", "cc-switch.db")
		if _, err := os.Stat(defaultPath); err == nil {
			log.Printf("[service] DetectCCSwitchDB found at %s", defaultPath)
			return defaultPath
		}
		log.Printf("[service] DetectCCSwitchDB not found at %s", defaultPath)
	}
	return ""
}

func (s *ImportService) ImportCCSwitchDB() model.CCSwitchImportResult {
	if s.db == nil {
		return model.CCSwitchImportResult{Error: "数据库未初始化"}
	}
	if s.engine == nil {
		return model.CCSwitchImportResult{Error: "引擎未初始化"}
	}

	stats, err := s.engine.SyncCCSwitch()
	if err != nil {
		return model.CCSwitchImportResult{Error: fmt.Sprintf("导入失败: %v", err)}
	}
	log.Printf("[service] ImportCCSwitchDB sync done proxy_rows=%d proxy_keys=%d rollup_rows=%d recon=%d",
		stats.ProxyRows, stats.ProxyKeys, stats.RollupRows, stats.ReconSupplement)

	imported := stats.ProxyKeys + stats.ReconSupplement
	total := stats.ProxyRows + stats.RollupRows
	return model.CCSwitchImportResult{
		Total:    total,
		Imported: imported,
		Message:  fmt.Sprintf("共 %d 条记录，导入 %d 条（proxy: %d 行→%d 聚合, rollup: %d 行, 补充: %d 条）",
			total, imported, stats.ProxyRows, stats.ProxyKeys, stats.RollupRows, stats.ReconSupplement),
	}
}


