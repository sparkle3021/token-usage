package service

// Package service 的子文件，CC-Switch 数据库路径检测业务逻辑。
import (
	"log"

	"token-dashboard/internal/config"
)

// ImportService CC-Switch 数据库路径检测服务。
type ImportService struct{}

// NewImportService 创建导入服务实例。
func NewImportService() *ImportService {
	return &ImportService{}
}

func (s *ImportService) DetectCCSwitchDB() string {
	path, exists := config.CCSwitchDefaultPath()
	if exists {
		log.Printf("[service] DetectCCSwitchDB found at %s", path)
		return path
	}
	log.Printf("[service] DetectCCSwitchDB not found at %s", path)
	return ""
}


