package service

// Package service 提供业务逻辑层，负责聚合多表数据、调用定价引擎和采集编排。
// 该层不依赖 Wails 运行时，便于独立测试。
import (
	"log"
	"strings"
	"sync"
	"time"

	"token-dashboard/internal/database"
	"token-dashboard/internal/model"
)

// DashboardService 仪表盘业务逻辑，聚合数据库原始数据并计算定价。
type DashboardService struct {
	db *database.Manager

	// 会话聚合缓存：GetSessionsData 首次计算后缓存，采集完成时失效。
	// time_usage 全表聚合较慢（~2.5s），缓存避免每次拉取都重算。
	sessionsCache   []model.SessionAgg
	sessionsCacheMu sync.Mutex

	// 仪表盘汇总缓存：GetDashboardData 首次计算后缓存，采集完成时失效。
	// daily/runs 在数据源未更新时不变，切时间范围无需重查。
	dailyCache   *model.DashboardData
	dailyCacheMu sync.Mutex
}

// NewDashboardService 创建仪表盘服务实例。
func NewDashboardService(db *database.Manager) *DashboardService {
	return &DashboardService{db: db}
}

// InvalidateCaches 清空仪表盘与会话聚合缓存，采集完成后由 App 调用。
func (s *DashboardService) InvalidateCaches() {
	s.sessionsCacheMu.Lock()
	s.sessionsCache = nil
	s.sessionsCacheMu.Unlock()
	s.dailyCacheMu.Lock()
	s.dailyCache = nil
	s.dailyCacheMu.Unlock()
}

// GetDashboardData 获取仪表盘汇总数据，包含日用量、会话和采集运行记录。
// 自动关联 project_path 到日用量记录，并规范化运行日志。
// 结果缓存于内存（dailyCache），采集完成后由 InvalidateCaches 失效。
func (s *DashboardService) GetDashboardData() *model.DashboardData {
	defer log.Printf("[service] GetDashboardData done")
	start := time.Now()

	s.dailyCacheMu.Lock()
	defer s.dailyCacheMu.Unlock()
	if s.dailyCache != nil {
		return s.dailyCache
	}

	if s.db == nil {
		log.Printf("[service] GetDashboardData db=nil")
		return &model.DashboardData{}
	}

	daily, err := s.db.QueryDaily()
	if err != nil {
		log.Printf("[service] GetDashboardData QueryDaily err=%v", err)
	}
	sessions, err := s.db.QuerySessions()
	if err != nil {
		log.Printf("[service] GetDashboardData QuerySessions err=%v", err)
	}
	runs, err := s.db.QueryRuns(500)
	if err != nil {
		log.Printf("[service] GetDashboardData QueryRuns err=%v", err)
	}

	projMap := make(map[string]string)
	for _, s := range sessions {
		proj := s.ProjectPath
		if proj == "" {
			parts := strings.Split(s.SessionID, "/")
			proj = parts[len(parts)-1]
		}
		key := s.Device + "::" + s.Source
		if _, ok := projMap[key]; !ok {
			projMap[key] = proj
		}
	}

	for i := range daily {
		if key, ok := projMap[daily[i].Device+"::"+daily[i].Source]; ok {
			daily[i].ProjectPath = key
		}
	}

	for i := range runs {
		runs[i].Message = strings.ReplaceAll(runs[i].Message, "\n", " ")
	}

	log.Printf("[service] GetDashboardData daily=%d sessions=%d runs=%d elapsed=%v",
		len(daily), len(sessions), len(runs), time.Since(start))

	result := &model.DashboardData{
		Daily:    daily,
		Sessions: sessions,
		Runs:     runs,
	}
	s.dailyCache = result
	return result
}

// GetSessionsData 按 session_id 聚合 time_usage 返回会话明细，作为会话 Tab 数据源。
// 结果缓存于内存，首次计算后复用，采集完成后由 InvalidateCaches 失效。
func (s *DashboardService) GetSessionsData() []model.SessionAgg {
	if s.db == nil {
		return nil
	}
	s.sessionsCacheMu.Lock()
	defer s.sessionsCacheMu.Unlock()
	if s.sessionsCache != nil {
		return s.sessionsCache
	}
	sessions, err := s.db.QuerySessionsFromTimeUsage()
	if err != nil {
		log.Printf("[service] GetSessionsData QuerySessionsFromTimeUsage err=%v", err)
		return nil
	}
	s.sessionsCache = sessions
	return sessions
}

// GetSessionModelBreakdown 按 session_id 返回该会话的模型拆分明细。
func (s *DashboardService) GetSessionModelBreakdown(sessionID string) []model.SessionModelRow {
	if s.db == nil || sessionID == "" {
		return nil
	}
	rows, err := s.db.QuerySessionModelBreakdown(sessionID)
	if err != nil {
		log.Printf("[service] GetSessionModelBreakdown err=%v", err)
		return nil
	}
	return rows
}

// GetTimeSeriesData 获取时间序列数据，包含原始事件和小时聚合两层的用量。
// 前端按 timeRows → hourRows → dailyRows 三级回退渲染趋势图。
func (s *DashboardService) GetTimeSeriesData(days int) *model.TimeSeriesData {
	start := time.Now()
	if s.db == nil {
		log.Printf("[service] GetTimeSeriesData db=nil")
		return &model.TimeSeriesData{}
	}
	var timeRows []model.TimeUsage
	// time_usage 仅在"今天"小时视图需要（days=1），其他时间范围前端只用 daily 数据
	if days == 1 {
		var err error
		timeRows, err = s.db.QueryTimeUsage(days)
		if err != nil {
			log.Printf("[service] GetTimeSeriesData(%d) QueryTimeUsage ERR: %v", days, err)
		}
	}
	hourRows, _ := s.db.QueryHourUsage(days)
	log.Printf("[service] GetTimeSeriesData(%d) timeRows=%d hourRows=%d elapsed=%v", days, len(timeRows), len(hourRows), time.Since(start))
	return &model.TimeSeriesData{Time: timeRows, Hour: hourRows}
}
