package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"token-dashboard/internal/collector/orchestrator"
	"token-dashboard/internal/config"
	"token-dashboard/internal/database"
	"token-dashboard/internal/model"
	"token-dashboard/internal/pricing"
	"token-dashboard/internal/quota"
	"token-dashboard/internal/service"
)

// App Wails 应用主结构体，持有各服务实例，并将方法绑定到前端 window.go.main.App.*。
// 每个方法仅做参数转发，业务逻辑在各 service 中实现。
type App struct {
	ctx           context.Context
	dashboardSvc  *service.DashboardService
	collectionSvc *service.CollectionService
	importSvc     *service.ImportService
	settingSvc    *service.SettingService
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
	importSvc := service.NewImportService()
	settingSvc := service.NewSettingService(db, collectionSvc)
	quotaSvc := quota.NewService(db)
	pricingSvc := service.NewPricingService(pr, cfg.DataDir)

	return &App{
		dashboardSvc:  dashboardSvc,
		collectionSvc: collectionSvc,
		importSvc:     importSvc,
		settingSvc:    settingSvc,
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

func (a *App) GetSessionsData() []model.SessionAgg {
	return a.dashboardSvc.GetSessionsData()
}

func (a *App) GetSessionModelBreakdown(sessionID string) []model.SessionModelRow {
	return a.dashboardSvc.GetSessionModelBreakdown(sessionID)
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
	return a.collectionSvc.ClearAllData()
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

func (a *App) DetectCCSwitchDB() string {
	return a.importSvc.DetectCCSwitchDB()
}


