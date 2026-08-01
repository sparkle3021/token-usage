package service

// Package service 的子文件，CC-Switch 数据库路径检测业务逻辑。
import (
	"log"
	"os"
	"path/filepath"
)

// ImportService CC-Switch 数据库路径检测服务。
type ImportService struct{}

// NewImportService 创建导入服务实例。
func NewImportService() *ImportService {
	return &ImportService{}
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


