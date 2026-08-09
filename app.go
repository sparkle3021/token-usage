package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"token-dashboard/internal/collector/orchestrator"
	"token-dashboard/internal/config"
	"token-dashboard/internal/database"
	"token-dashboard/internal/model"
	"token-dashboard/internal/pricing"
	"token-dashboard/internal/quota"
	"token-dashboard/internal/service"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// App Wails 应用主结构体，持有各服务实例，并将方法绑定到前端 window.go.main.App.*。
// 每个方法仅做参数转发，业务逻辑在各 service 中实现。
type App struct {
	ctx           context.Context
	dashboardSvc  *service.DashboardService
	collectionSvc *service.CollectionService
	importSvc     *service.ImportService
	settingSvc    *service.SettingService
	deviceSvc     *service.DeviceService
	exportSvc     *service.ExportService
	quotaSvc      *quota.Service
	pricingSvc    *service.PricingService
	dataDir       string
}

// NewApp 初始化配置、数据库、定价引擎和采集引擎，组装各服务并返回 App 实例。
// 若数据库初始化失败，返回降级实例（无 DB 功能可用）。
func NewApp() *App {
	cfg := config.Load()

	db, err := database.New(cfg.DBPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[app] database: %v\n", err)
		log.Printf("[app] NewApp database failed path=%s err=%v", cfg.DBPath, err)
		return &App{ctx: context.Background()}
	}
	log.Printf("[app] NewApp database opened path=%s", cfg.DBPath)

	pr, err := pricing.NewEngine(cfg.DataDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[app] pricing: %v\n", err)
		log.Printf("[app] NewApp pricing error=%v", err)
	}
	log.Printf("[app] NewApp pricing loaded")

	eng := orchestrator.New(db, pr)
	log.Printf("[app] NewApp engine initialized collectors=%d", len(eng.Collectors()))

	dashboardSvc := service.NewDashboardService(db)
	collectionSvc := service.NewCollectionService(db, eng)
	importSvc := service.NewImportService(db)
	settingSvc := service.NewSettingService(db, collectionSvc)
	deviceSvc := service.NewDeviceService(db)
	exportSvc := service.NewExportService(db)
	quotaSvc := quota.NewService(db)
	pricingSvc := service.NewPricingService(pr, cfg.DataDir)

	return &App{
		dashboardSvc:  dashboardSvc,
		collectionSvc: collectionSvc,
		importSvc:     importSvc,
		settingSvc:    settingSvc,
		deviceSvc:     deviceSvc,
		exportSvc:     exportSvc,
		quotaSvc:      quotaSvc,
		pricingSvc:    pricingSvc,
		dataDir:       cfg.DataDir,
	}
}

// startup Wails 启动回调，保存上下文、桥接采集事件、触发 CC-Switch 检测与检查点校验。
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	a.collectionSvc.SetCtx(ctx)
	a.collectionSvc.SetOnCollectionDone(func() { a.dashboardSvc.InvalidateCaches() })
	a.collectionSvc.WireEngineEvents()

	a.settingSvc.EnsureDefaultCCSwitchPath()
	a.collectionSvc.ReconcileStaleCheckpoints()
}

// shutdown Wails 关闭回调，停止自动同步定时器并关闭数据库连接。
func (a *App) shutdown(ctx context.Context) {
	log.Println("[app] shutdown")
	if a.collectionSvc != nil {
		a.collectionSvc.Shutdown()
	}
}

// ---------------------------------------------------------------------------
// Dashboard API
// ---------------------------------------------------------------------------

func (a *App) GetDashboardData() *model.DashboardData {
	return a.dashboardSvc.GetDashboardData()
}

func (a *App) GetTimeSeriesData(days int) *model.TimeSeriesData {
	return a.dashboardSvc.GetTimeSeriesData(days)
}

func (a *App) GetModelRanking() []model.ModelRanking {
	return a.dashboardSvc.GetModelRanking()
}

func (a *App) GetModelSeries(modelName string) *model.ModelSeriesData {
	return a.dashboardSvc.GetModelSeries(modelName)
}

// ---------------------------------------------------------------------------
// Collection API
// ---------------------------------------------------------------------------

func (a *App) StartCollection() bool {
	return a.collectionSvc.StartCollection()
}

func (a *App) StartFullCollection() bool {
	return a.collectionSvc.StartFullCollection()
}

func (a *App) CollectStatus() *model.CollectStatus {
	return a.collectionSvc.CollectStatus()
}

// CurrentOp 返回当前正在运行的操作名称，供前端展示。返回 "" 表示空闲。
func (a *App) CurrentOp() string {
	return a.collectionSvc.CurrentOp()
}

func (a *App) ClearAllData() error {
	if err := a.collectionSvc.ClearAllData(); err != nil {
		return err
	}
	// 清空数据库后失效仪表盘/会话缓存，否则前端拿到旧缓存数据，表现"清空无效"。
	if a.dashboardSvc != nil {
		a.dashboardSvc.InvalidateCaches()
	}
	return nil
}

// ---------------------------------------------------------------------------
// Auto-Sync API
// ---------------------------------------------------------------------------

func (a *App) SetAutoSyncInterval(minutes int) {
	a.collectionSvc.SetAutoSyncInterval(minutes)
}

func (a *App) GetAutoSyncInterval() int {
	return a.collectionSvc.GetAutoSyncInterval()
}

// ---------------------------------------------------------------------------
// Settings API
// ---------------------------------------------------------------------------

func (a *App) GetSettings() model.AppConfig {
	return a.settingSvc.GetSettings()
}

func (a *App) SaveSettings(cfg model.AppConfig) error {
	return a.settingSvc.SaveSettings(cfg)
}

// ---------------------------------------------------------------------------
// Device API
// ---------------------------------------------------------------------------

func (a *App) GetDevices() []model.DeviceInfo {
	return a.deviceSvc.ListDevices()
}

func (a *App) RenameDevice(deviceID, displayName string) error {
	return a.deviceSvc.RenameDevice(deviceID, displayName)
}

// ---------------------------------------------------------------------------
// Transfer API（导出）
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

// ---------------------------------------------------------------------------
// Pricing API
// ---------------------------------------------------------------------------

// UpdatePricing 从远程源拉取最新定价数据并重载定价引擎。
func (a *App) UpdatePricing() model.PricingUpdateResult {
	return a.pricingSvc.UpdatePricing()
}

// ---------------------------------------------------------------------------
// Quota API
// ---------------------------------------------------------------------------

func (a *App) ListQuotaConfigs() []model.QuotaConfig {
	return a.quotaSvc.List()
}

func (a *App) GetProviderSchemas() []model.ProviderSchema {
	return a.quotaSvc.GetProviderSchemas()
}

func (a *App) CreateQuotaConfig(cfg model.QuotaConfig) model.QuotaConfig {
	created, err := a.quotaSvc.Create(cfg)
	if err != nil {
		return model.QuotaConfig{ConfigJSON: err.Error()}
	}
	return created
}

func (a *App) UpdateQuotaConfig(cfg model.QuotaConfig) error {
	return a.quotaSvc.Update(cfg)
}

func (a *App) DeleteQuotaConfig(id int64) error {
	return a.quotaSvc.Delete(id)
}

func (a *App) FetchQuota(id int64) *model.QuotaData {
	return a.quotaSvc.Fetch(id)
}

func (a *App) FetchAllQuota() []model.QuotaData {
	return a.quotaSvc.FetchAll()
}

// ---------------------------------------------------------------------------
// Import API
// ---------------------------------------------------------------------------

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


