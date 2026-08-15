package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"sync/atomic"

	"token-dashboard/internal/collector/orchestrator"
	"token-dashboard/internal/config"
	"token-dashboard/internal/database"
	"token-dashboard/internal/pricing"
	"token-dashboard/internal/quota"
	"token-dashboard/internal/service"
)

// App Wails 应用主结构体，持有各服务实例。
// 绑定方法按 API 域拆分在 app_*.go 中（dashboard/collection/settings/device/transfer/pricing/quota），
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

	// quitting 托盘主动退出标志：置位后 OnBeforeClose 放行（否则 runtime.Quit 也会被拦截，应用退不掉）
	quitting atomic.Bool
	// started 在 startup 回调完成后关闭，托盘据此等待 ctx 就绪
	started chan struct{}
}

// NewApp 初始化配置、数据库、定价引擎和采集引擎，组装各服务并返回 App 实例。
// 若数据库初始化失败，返回降级实例（无 DB 功能可用）。
func NewApp() *App {
	cfg := config.Load()

	db, err := database.New(cfg.DBPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[app] database: %v\n", err)
		log.Printf("[app] NewApp database failed path=%s err=%v", cfg.DBPath, err)
		return &App{ctx: context.Background(), started: make(chan struct{})}
	}
	log.Printf("[app] NewApp database opened path=%s", cfg.DBPath)

	pr := pricing.NewEngine()
	log.Printf("[app] NewApp pricing engine created")

	// 定价服务：保证 model_pricing 表有数据并加载进引擎，之后才可创建采集引擎
	pricingSvc := service.NewPricingService(pr, db, cfg.DataDir)
	if err := pricingSvc.EnsureSeeded(); err != nil {
		log.Printf("[app] pricing seed error=%v", err)
	}

	eng := orchestrator.New(db, pr)
	log.Printf("[app] NewApp engine initialized collectors=%d", len(eng.Collectors()))

	dashboardSvc := service.NewDashboardService(db)
	collectionSvc := service.NewCollectionService(db, eng)
	importSvc := service.NewImportService(db)
	settingSvc := service.NewSettingService(db, collectionSvc)
	deviceSvc := service.NewDeviceService(db)
	exportSvc := service.NewExportService(db)
	quotaSvc := quota.NewService(db)

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
		started:       make(chan struct{}),
	}
}

// startup Wails 启动回调，保存上下文、桥接采集事件、触发 CC-Switch 检测与检查点校验。
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	if a.collectionSvc != nil {
		a.collectionSvc.SetCtx(ctx)
		a.collectionSvc.SetOnCollectionDone(func() { a.dashboardSvc.InvalidateCaches() })
		a.collectionSvc.WireEngineEvents()

		a.settingSvc.EnsureDefaultCCSwitchPath()
		a.collectionSvc.ReconcileStaleCheckpoints()
	}
	// 托盘 onReady 等待此 channel；startup 完成前菜单回调不得访问 ctx
	close(a.started)
}

// shutdown Wails 关闭回调，停止自动同步定时器并关闭数据库连接。
func (a *App) shutdown(ctx context.Context) {
	log.Println("[app] shutdown")
	if a.collectionSvc != nil {
		a.collectionSvc.Shutdown()
	}
}
