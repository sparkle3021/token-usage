// Package service 的子文件，数据导入业务逻辑：从导出 JSON 合并用量与设备映射。
// 校验：format/version 与导出端（transfer.go）对齐，不匹配即拒绝，不写任何数据。
package service

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"

	"token-dashboard/internal/config"
	"token-dashboard/internal/database"
	"token-dashboard/internal/model"
)

// exportFormat/exportVersion 与导出端一致，导入端据此校验文件。
const (
	exportFormat  = "token-usage-export"
	exportVersion = 1
)

// ImportService 数据导入服务：CC-Switch 路径检测 + 导出 JSON 导入。
type ImportService struct {
	db *database.Manager
}

// NewImportService 创建导入服务实例。
func NewImportService(db *database.Manager) *ImportService {
	return &ImportService{db: db}
}

// ImportFile 读取并导入导出 JSON 文件，返回合并规模（hours/daily/sessions/new_devices）。
// 各失败阶段均记录日志，供导入链路排查。
func (s *ImportService) ImportFile(path string) (*model.ImportResult, error) {
	if s.db == nil {
		return nil, fmt.Errorf("数据库未初始化")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		log.Printf("[service] ImportFile read failed path=%s err=%v", path, err)
		return nil, fmt.Errorf("读取导入文件失败: %w", err)
	}
	payload, err := parseAndValidate(data)
	if err != nil {
		log.Printf("[service] ImportFile validate failed path=%s err=%v", path, err)
		return nil, err
	}

	hours, daily, sessions, devices, err := s.db.ImportUsage(payload.HourUsage, payload.DailyUsage, payload.SessionUsage, payload.DeviceNames)
	if err != nil {
		log.Printf("[service] ImportFile write failed path=%s err=%v", path, err)
		return nil, err
	}
	log.Printf("[service] ImportFile ok path=%s hours=%d daily=%d sessions=%d new_devices=%d", path, hours, daily, sessions, devices)
	return &model.ImportResult{
		Hours:      hours,
		Daily:      daily,
		Sessions:   sessions,
		Devices:    devices,
		ImportedAt: time.Now().Format(time.RFC3339),
	}, nil
}

// parseAndValidate 解析导出 JSON 并校验 format/version。独立方法便于单测（不依赖 db）。
func parseAndValidate(data []byte) (*ExportPayload, error) {
	var payload ExportPayload
	if err := json.Unmarshal(data, &payload); err != nil {
		return nil, fmt.Errorf("解析导入文件失败（非有效 JSON）: %w", err)
	}
	if payload.Format != exportFormat {
		return nil, fmt.Errorf("文件格式不匹配：期望 %q，实际 %q", exportFormat, payload.Format)
	}
	if payload.Version != exportVersion {
		return nil, fmt.Errorf("版本不兼容：期望 %d，实际 %d", exportVersion, payload.Version)
	}
	return &payload, nil
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
