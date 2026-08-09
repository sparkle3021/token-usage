package service

// Package service 的子文件，采集调度与自动同步业务逻辑。
import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"token-dashboard/internal/database"
	"token-dashboard/internal/collector/orchestrator"
	"token-dashboard/internal/model"

	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// CollectionService 采集调度与自动同步业务逻辑。
type CollectionService struct {
	db     *database.Manager
	engine *orchestrator.Engine
	ctx    context.Context

	// onCollectionDone 采集完成时回调（App 注入，用于失效依赖采集结果的缓存）
	onCollectionDone func()

	autoSyncMu      sync.Mutex
	autoSyncCancel  context.CancelFunc
	autoSyncSeconds int // 自动同步间隔（秒），≤0 表示禁用
}

// NewCollectionService 创建采集服务实例。
func NewCollectionService(db *database.Manager, eng *orchestrator.Engine) *CollectionService {
	return &CollectionService{db: db, engine: eng}
}

// SetOnCollectionDone 注册采集完成回调，由 App 注入。
func (s *CollectionService) SetOnCollectionDone(fn func()) {
	s.onCollectionDone = fn
}

func (s *CollectionService) SetCtx(ctx context.Context) {
	s.ctx = ctx
}

// WireEngineEvents 将采集引擎的事件桥接到 Wails 运行时，使前端能收到采集进度事件。
func (s *CollectionService) WireEngineEvents() {
	if s.engine == nil {
		return
	}
	s.engine.SetEventCallback(func(event string, data interface{}) {
		log.Printf("[event] %s: %v\n", event, data)
		wailsRuntime.EventsEmit(s.ctx, event, data)
		if event == "collection:done" && s.onCollectionDone != nil {
			s.onCollectionDone()
		}
	})
}

// StartCollection 启动增量采集，使用 checkpoint 跳过已处理文件。返回 false 表示采集已在运行。
func (s *CollectionService) StartCollection() bool {
	if s.engine == nil {
		log.Printf("[service] StartCollection engine=nil")
		return false
	}
	ok := s.engine.StartCollection()
	log.Printf("[service] StartCollection result=%v", ok)
	return ok
}

// StartFullCollection 启动全量采集，忽略所有增量标记，重新解析全部文件。
func (s *CollectionService) StartFullCollection() bool {
	if s.engine == nil {
		log.Printf("[service] StartFullCollection engine=nil")
		return false
	}
	ok := s.engine.StartFullCollection()
	log.Printf("[service] StartFullCollection result=%v", ok)
	return ok
}

func (s *CollectionService) CollectStatus() *model.CollectStatus {
	if s.engine == nil {
		log.Printf("[service] CollectStatus engine=nil")
		return &model.CollectStatus{Status: "idle", Message: "未初始化"}
	}
	st := s.engine.Status()
	log.Printf("[service] CollectStatus status=%s message=%s", st.Status, st.Message)
	return &model.CollectStatus{
		Status:     st.Status,
		Message:    st.Message,
		StartedAt:  st.StartedAt,
		FinishedAt: st.FinishedAt,
		ExitCode:   st.ExitCode,
		Stdout:     st.Stdout,
		Stderr:     st.Stderr,
	}
}

// ClearAllData 清除所有用量数据、采集历史、及同步状态（checkpoint）。
// 委托给 Engine.ClearAllData 统一管理并发锁。
func (s *CollectionService) ClearAllData() error {
	if s.engine == nil {
		return fmt.Errorf("引擎未初始化")
	}
	return s.engine.ClearAllData()
}

func (s *CollectionService) SetAutoSyncInterval(seconds int) {
	s.autoSyncMu.Lock()
	defer s.autoSyncMu.Unlock()

	if s.autoSyncCancel != nil {
		s.autoSyncCancel()
		s.autoSyncCancel = nil
	}
	s.autoSyncSeconds = seconds

	if seconds <= 0 {
		log.Println("[service] Auto-sync disabled")
		return
	}

	ctx, cancel := context.WithCancel(context.Background())
	s.autoSyncCancel = cancel

	go func() {
		ticker := time.NewTicker(time.Duration(seconds) * time.Second)
		defer ticker.Stop()
		log.Printf("[service] Auto-sync started interval=%ds", seconds)
		for {
			select {
			case <-ticker.C:
				log.Println("[service] Auto-sync triggering collection")
				s.StartCollection()
			case <-ctx.Done():
				log.Println("[service] Auto-sync stopped")
				return
			}
		}
	}()
}

func (s *CollectionService) GetAutoSyncInterval() int {
	s.autoSyncMu.Lock()
	defer s.autoSyncMu.Unlock()
	return s.autoSyncSeconds
}

// Shutdown 停止自动同步定时器并关闭数据库连接。
// 数据库连接归属 CollectionService（持有 *database.Manager），app 层不直接持有。
func (s *CollectionService) Shutdown() {
	s.autoSyncMu.Lock()
	defer s.autoSyncMu.Unlock()
	if s.autoSyncCancel != nil {
		s.autoSyncCancel()
		s.autoSyncCancel = nil
	}
	if s.db != nil {
		s.db.Close()
	}
}

// CurrentOp 返回当前正在运行的操作名称（供前端展示），"" 表示空闲。
func (s *CollectionService) CurrentOp() string {
	if s.engine == nil {
		return ""
	}
	return s.engine.CurrentOp()
}

// ReconcileStaleCheckpoints 检测 CC-Switch 陈旧检查点：若检查点存在但库中
// 无任何用量数据（数据被清除过），则重置检查点使下次同步全量重导。
// 原 app.startup 逻辑，下沉至服务层以消除 app 层对数据库的直接访问。
func (s *CollectionService) ReconcileStaleCheckpoints() {
	if s.db == nil {
		return
	}
	ckProxy, _ := s.db.GetCheckpoint("cc_switch_cursor_proxy_request_logs")
	ckRollup, _ := s.db.GetCheckpoint("cc_switch_rollup_max_date")
	if ckProxy == "" && ckRollup == "" {
		return
	}
	var cnt int
	s.db.DB().QueryRow("SELECT (SELECT COUNT(*) FROM daily_usage) + (SELECT COUNT(*) FROM hour_usage)").Scan(&cnt)
	if cnt == 0 {
		log.Printf("[service] stale CC-Switch checkpoint detected (proxy=%q rollup=%q), total_data=0 — resetting for full re-sync", ckProxy, ckRollup)
		s.db.ResetCCSwitchCheckpoints()
	}
}
