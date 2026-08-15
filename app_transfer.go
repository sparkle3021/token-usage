package main

import (
	"fmt"
	"log"
	"os"
	"time"

	"token-dashboard/internal/model"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// ---------------------------------------------------------------------------
// Transfer API（导出 / 导入）
// ---------------------------------------------------------------------------

// ExportData 导出用量数据（hour_usage + session_usage + 设备映射）到用户选择的 JSON 文件。
// 返回保存路径；用户取消时返回空字符串。失败阶段记录日志；成功日志含文件大小与总耗时。
func (a *App) ExportData() (string, error) {
	start := time.Now()
	if a.exportSvc == nil {
		return "", fmt.Errorf("导出服务未初始化")
	}
	payload, err := a.exportSvc.BuildExport()
	if err != nil {
		log.Printf("[app] ExportData build failed: %v", err)
		return "", err
	}
	payload.ExportedAt = time.Now().Format(time.RFC3339)
	data, err := payload.Marshal()
	if err != nil {
		log.Printf("[app] ExportData marshal failed: %v", err)
		return "", fmt.Errorf("序列化导出数据失败: %w", err)
	}

	host, _ := os.Hostname()
	deviceID := "unknown"
	if a.exportSvc != nil {
		deviceID = a.exportSvc.LocalDeviceID()
	}
	ts := time.Now().Format("20060102-150405")
	path, err := runtime.SaveFileDialog(a.ctx, runtime.SaveDialogOptions{
		DefaultFilename: fmt.Sprintf("token-usage-export-%s-%s-%s.json", host, deviceID, ts),
		Title:           "导出用量数据",
		Filters:         []runtime.FileFilter{{DisplayName: "JSON 文件", Pattern: "*.json"}},
	})
	if err != nil {
		log.Printf("[app] ExportData save dialog failed: %v", err)
		return "", err
	}
	if path == "" {
		log.Printf("[app] ExportData canceled by user")
		return "", nil // 用户取消
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		log.Printf("[app] ExportData write failed path=%s err=%v", path, err)
		return "", fmt.Errorf("写入导出文件失败: %w", err)
	}
	log.Printf("[app] ExportData ok path=%s size=%d elapsed=%v", path, len(data), time.Since(start))
	return path, nil
}

// ImportData 从用户选择的导出 JSON 文件导入用量数据并失效缓存。
// 返回合并规模；用户取消文件选择时返回 (nil, nil)。
func (a *App) ImportData() (*model.ImportResult, error) {
	if a.importSvc == nil {
		return nil, fmt.Errorf("导入服务未初始化")
	}
	path, err := runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{
		Title:   "导入用量数据",
		Filters: []runtime.FileFilter{{DisplayName: "JSON 文件", Pattern: "*.json"}},
	})
	if err != nil {
		log.Printf("[app] ImportData open dialog failed: %v", err)
		return nil, err
	}
	if path == "" {
		log.Printf("[app] ImportData canceled by user")
		return nil, nil // 用户取消
	}
	result, err := a.importSvc.ImportFile(path)
	if err != nil {
		log.Printf("[app] ImportData failed path=%s err=%v", path, err)
		return nil, err
	}
	// 导入合并后失效仪表盘/会话缓存，否则前端拿到旧缓存数据，表现"导入无效"。
	if a.dashboardSvc != nil {
		a.dashboardSvc.InvalidateCaches()
	}
	log.Printf("[app] ImportData ok path=%s hours=%d daily=%d sessions=%d devices=%d", path, result.Hours, result.Daily, result.Sessions, result.Devices)
	return result, nil
}

func (a *App) DetectCCSwitchDB() string {
	return a.importSvc.DetectCCSwitchDB()
}
