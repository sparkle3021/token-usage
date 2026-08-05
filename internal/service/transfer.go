package service

// Package service 的子文件，跨设备数据导出的业务组装。
// 导出范围：hour_usage（权威聚合层）+ daily_usage（含 CC-Switch rollup 日级历史）
// + session_usage（会话汇总）+ 设备映射。
// 不导出：
//   - time_usage：明细体积大、event_key 含本地路径且为 json:"-" 隐藏字段，导入无意义
//   - parse_cache / collection_runs / app_config / quota_configs（含密钥）：本地状态
import (
	"encoding/json"
	"fmt"
	"log"
	"time"

	"token-dashboard/internal/database"
	"token-dashboard/internal/model"
)

// ExportPayload 导出文件的顶层结构，format/version 供导入端校验。
type ExportPayload struct {
	Format       string                `json:"format"`
	Version      int                   `json:"version"`
	ExportedAt   string                `json:"exportedAt"`
	DeviceNames  map[string]string     `json:"deviceNames"`
	HourUsage    []model.HourUsage     `json:"hourUsage"`
	DailyUsage   []model.DailyUsage    `json:"dailyUsage"`
	SessionUsage []model.SessionUsage  `json:"sessionUsage"`
}

// ExportService 数据导出业务逻辑。
type ExportService struct {
	db *database.Manager
}

// NewExportService 构建导出服务实例。
func NewExportService(db *database.Manager) *ExportService {
	return &ExportService{db: db}
}

// LocalDeviceID 返回本机设备身份（UUID），用于导出文件命名标识来源设备。
func (s *ExportService) LocalDeviceID() string {
	if s.db == nil {
		return "unknown"
	}
	return s.db.LocalDeviceID()
}

// BuildExport 组装导出数据（hour_usage + daily_usage + session_usage 全量 + 设备映射）。
// 失败阶段记录日志；成功日志含各层行数与总耗时，供导出链路性能排查。
func (s *ExportService) BuildExport() (*ExportPayload, error) {
	start := time.Now()
	if s.db == nil {
		return nil, fmt.Errorf("数据库未初始化")
	}

	hours, err := s.db.QueryHourUsage(0)
	if err != nil {
		return s.exportFail("hour_usage", err, start)
	}
	daily, err := s.db.QueryDaily()
	if err != nil {
		return s.exportFail("daily_usage", err, start)
	}
	sessions, err := s.db.QuerySessions()
	if err != nil {
		return s.exportFail("session_usage", err, start)
	}
	names, err := s.db.DeviceNames()
	if err != nil {
		return s.exportFail("设备映射", err, start)
	}

	log.Printf("[service] BuildExport ok hours=%d daily=%d sessions=%d elapsed=%v", len(hours), len(daily), len(sessions), time.Since(start))
	return &ExportPayload{
		Format:       "token-usage-export",
		Version:      1,
		DeviceNames:  names,
		HourUsage:    hours,
		DailyUsage:   daily,
		SessionUsage: sessions,
	}, nil
}

// exportFail 记录导出构建失败日志（含阶段与耗时）并返回包装错误。
func (s *ExportService) exportFail(stage string, err error, start time.Time) (*ExportPayload, error) {
	log.Printf("[service] BuildExport %s failed: %v elapsed=%v", stage, err, time.Since(start))
	return nil, fmt.Errorf("查询 %s: %w", stage, err)
}

// Marshal 序列化为格式化 JSON。
func (p *ExportPayload) Marshal() ([]byte, error) {
	return json.MarshalIndent(p, "", "  ")
}
