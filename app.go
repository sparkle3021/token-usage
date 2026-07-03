package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"token-dashboard/internal/collector/orchestrator"
	"token-dashboard/internal/config"
	"token-dashboard/internal/database"
	"token-dashboard/internal/model"
	"token-dashboard/internal/pricing"
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

	db      *database.Manager      // 保留用于 startup 中的 CC-Switch 检测
	engine  *orchestrator.Engine   // 保留用于 startup 中的事件桥接
	dataDir string
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

	dashboardSvc := service.NewDashboardService(db, pr, cfg.DataDir)
	collectionSvc := service.NewCollectionService(db, eng)
	importSvc := service.NewImportService(db, eng)
	settingSvc := service.NewSettingService(db, collectionSvc)

	return &App{
		dashboardSvc:  dashboardSvc,
		collectionSvc: collectionSvc,
		importSvc:     importSvc,
		settingSvc:    settingSvc,
		db:            db,
		engine:        eng,
		dataDir:       cfg.DataDir,
	}
}

// startup Wails 启动回调，保存上下文、桥接采集事件、检测 CC-Switch 状态。
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	a.collectionSvc.SetCtx(ctx)
	a.collectionSvc.WireEngineEvents()

	// Auto-detect CC-Switch DB path on startup if not configured
	if a.db != nil {
		existing, _ := a.db.GetConfig("cc_switch_db_path")
		if existing == "" {
			if home, err := os.UserHomeDir(); err == nil {
				defaultPath := filepath.Join(home, ".cc-switch", "cc-switch.db")
				if _, err := os.Stat(defaultPath); err == nil {
					a.db.SetConfig("cc_switch_db_path", defaultPath)
					log.Printf("[app] startup auto-detected cc-switch db at %s", defaultPath)
				} else {
					log.Printf("[app] startup cc-switch db not found at %s", defaultPath)
				}
			}
		}

		ckProxy, _ := a.db.GetCheckpoint("cc_switch_cursor_proxy_request_logs")
		ckRollup, _ := a.db.GetCheckpoint("cc_switch_rollup_max_date")
		if ckProxy != "" || ckRollup != "" {
			var cnt int
			a.db.DB().QueryRow("SELECT (SELECT COUNT(*) FROM daily_usage) + (SELECT COUNT(*) FROM hour_usage)").Scan(&cnt)
			if cnt == 0 {
				log.Printf("[app] stale CC-Switch checkpoint detected (proxy=%q rollup=%q), total_data=0 — resetting for full re-sync", ckProxy, ckRollup)
				a.db.ResetCCSwitchCheckpoints()
			}
		}
	}
}

// shutdown Wails 关闭回调，停止自动同步定时器并关闭数据库连接。
func (a *App) shutdown(ctx context.Context) {
	log.Println("[app] shutdown")
	if a.collectionSvc != nil {
		a.collectionSvc.Shutdown()
	}
	if a.db != nil {
		a.db.Close()
	}
}

// ---------------------------------------------------------------------------
// Dashboard API
// ---------------------------------------------------------------------------

func (a *App) GetDashboardData() *model.DashboardData {
	return a.dashboardSvc.GetDashboardData()
}

func (a *App) GetTimeSeriesData() *model.TimeSeriesData {
	return a.dashboardSvc.GetTimeSeriesData()
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
	return a.dashboardSvc.UpdatePricing()
}

// ---------------------------------------------------------------------------
// Import API
// ---------------------------------------------------------------------------

func (a *App) DetectCCSwitchDB() string {
	return a.importSvc.DetectCCSwitchDB()
}

func (a *App) ImportCCSwitchDB() model.CCSwitchImportResult {
	return a.importSvc.ImportCCSwitchDB()
}


