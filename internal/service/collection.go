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

	autoSyncMu      sync.Mutex
	autoSyncCancel  context.CancelFunc
	autoSyncMinutes int // 自动同步间隔（分钟），≤0 表示禁用
}

// NewCollectionService 创建采集服务实例。
func NewCollectionService(db *database.Manager, eng *orchestrator.Engine) *CollectionService {
	return &CollectionService{db: db, engine: eng}
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

// ClearAllData 清除所有用量数据和采集历史，保留 app_config 设置。
// 同时清除采集器的文件解析缓存，确保下次同步会重新解析。
func (s *CollectionService) ClearAllData() error {
	if s.db == nil {
		return fmt.Errorf("数据库未初始化")
	}
	if err := s.db.ClearAllUsageData(); err != nil {
		log.Printf("[service] ClearAllData error: %v", err)
		return fmt.Errorf("清除失败: %v", err)
	}
	if s.engine != nil {
		s.engine.ClearCollectorCaches()
	}
	log.Printf("[service] ClearAllData ok")
	return nil
}

func (s *CollectionService) SetAutoSyncInterval(minutes int) {
	s.autoSyncMu.Lock()
	defer s.autoSyncMu.Unlock()

	if s.autoSyncCancel != nil {
		s.autoSyncCancel()
		s.autoSyncCancel = nil
	}
	s.autoSyncMinutes = minutes

	if minutes <= 0 {
		log.Println("[service] Auto-sync disabled")
		return
	}

	ctx, cancel := context.WithCancel(context.Background())
	s.autoSyncCancel = cancel

	go func() {
		ticker := time.NewTicker(time.Duration(minutes) * time.Minute)
		defer ticker.Stop()
		log.Printf("[service] Auto-sync started interval=%dm", minutes)
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
	return s.autoSyncMinutes
}

func (s *CollectionService) Shutdown() {
	s.autoSyncMu.Lock()
	defer s.autoSyncMu.Unlock()
	if s.autoSyncCancel != nil {
		s.autoSyncCancel()
		s.autoSyncCancel = nil
	}
}
