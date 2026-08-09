package service

// Package service 提供业务逻辑层，负责聚合多表数据、调用定价引擎和采集编排。
// 该层不依赖 Wails 运行时，便于独立测试。
import (
	"log"
	"sync"
	"time"

	"token-dashboard/internal/database"
	"token-dashboard/internal/model"
)

// DashboardService 仪表盘业务逻辑，聚合数据库原始数据并计算定价。
type DashboardService struct {
	db *database.Manager

	// 仪表盘汇总缓存：GetDashboardData 首次计算后缓存，采集完成时失效。
	// daily 在数据源未更新时不变，切时间范围无需重查。
	dailyCache   *model.DashboardData
	dailyCacheMu sync.Mutex
}

// NewDashboardService 创建仪表盘服务实例。
func NewDashboardService(db *database.Manager) *DashboardService {
	return &DashboardService{db: db}
}

// InvalidateCaches 清空仪表盘缓存，采集完成后由 App 调用。
func (s *DashboardService) InvalidateCaches() {
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

	// 日视图读 daily_usage 表（含 hour 重建 + CC-Switch rollup + Hermes 全部来源）。
	// daily_usage.usage_date 为本地日语义（BuildDailyFromHourUsage 按本地日聚合，
	// 直写来源亦按本地日写入），无需二次时区平移。
	// 回归修复：改为从 hour_usage 现场聚合会遗漏 CC-Switch rollup（仅日级、只写 daily_usage、
	// 且与 hour 重建同 key 时取 MAX 增大），导致 token 总量显著减少（v0.2.0 ~113 亿 → 80 亿）。
	daily, derr := s.db.QueryDaily()
	if derr != nil {
		log.Printf("[service] GetDashboardData QueryDaily err=%v", derr)
	}

	log.Printf("[service] GetDashboardData daily=%d elapsed=%v", len(daily), time.Since(start))

	result := &model.DashboardData{Daily: daily}
	s.dailyCache = result
	return result
}

// GetModelRanking 返回按模型聚合的用量排行，供模型排行页渲染。
func (s *DashboardService) GetModelRanking() []model.ModelRanking {
	if s.db == nil {
		log.Printf("[service] GetModelRanking db=nil")
		return []model.ModelRanking{}
	}
	ranking, err := s.db.QueryModelRanking()
	if err != nil {
		log.Printf("[service] GetModelRanking err=%v", err)
		return []model.ModelRanking{}
	}
	return ranking
}

// GetModelSeries 返回单模型的日级 + 小时级时间序列数据，供模型详情页独立拉取。
// hour 由 UTC 桶平移为本地日 + 本地小时（与 GetHourSeries 一致）。
func (s *DashboardService) GetModelSeries(modelName string) *model.ModelSeriesData {
	if s.db == nil || modelName == "" {
		log.Printf("[service] GetModelSeries db=nil or model empty")
		return &model.ModelSeriesData{}
	}
	daily, err := s.db.QueryModelDaily(modelName)
	if err != nil {
		log.Printf("[service] GetModelSeries QueryModelDaily err=%v", err)
		daily = nil
	}
	hour, err := s.db.QueryModelHour(modelName)
	if err != nil {
		log.Printf("[service] GetModelSeries QueryModelHour err=%v", err)
		hour = nil
	}
	for i := range hour {
		hour[i].UsageDate, hour[i].Hour = localizeUTCDateHour(hour[i].UsageDate, hour[i].Hour)
	}
	log.Printf("[service] GetModelSeries model=%s daily=%d hour=%d", modelName, len(daily), len(hour))
	return &model.ModelSeriesData{Daily: daily, Hour: hour}
}

// GetHourSeries 返回指定范围（最近 days 天）的小时聚合序列，供趋势图/钻取按范围渲染。
// hour 由 UTC 桶平移为本地日 + 本地小时（与 GetModelSeries 一致）。
func (s *DashboardService) GetHourSeries(days int) []model.HourUsage {
	start := time.Now()
	if s.db == nil {
		log.Printf("[service] GetHourSeries db=nil")
		return nil
	}
	hourRows, _ := s.db.QueryHourUsage(days)
	for i := range hourRows {
		hourRows[i].UsageDate, hourRows[i].Hour = localizeUTCDateHour(hourRows[i].UsageDate, hourRows[i].Hour)
	}
	log.Printf("[service] GetHourSeries(%d) hourRows=%d elapsed=%v", days, len(hourRows), time.Since(start))
	return hourRows
}

// GetTodayEvents 返回今天的原始事件（time_usage），恒定不随时间范围选择变化，
// 供今天视图 sparkline 与小时兜底。usageDate 统一为本地日语义：新数据为 UTC 日 → 转本机日；旧数据为本地日原样保留。
func (s *DashboardService) GetTodayEvents() []model.TimeUsage {
	start := time.Now()
	if s.db == nil {
		log.Printf("[service] GetTodayEvents db=nil")
		return nil
	}
	timeRows, err := s.db.QueryTimeUsage(1)
	if err != nil {
		log.Printf("[service] GetTodayEvents QueryTimeUsage ERR: %v", err)
	}
	for i := range timeRows {
		if t, perr := time.Parse(time.RFC3339, timeRows[i].EventTime); perr == nil {
			timeRows[i].UsageDate = t.In(time.Local).Format("2006-01-02")
		}
	}
	log.Printf("[service] GetTodayEvents timeRows=%d elapsed=%v", len(timeRows), time.Since(start))
	return timeRows
}

// localizeUTCDateHour 将 UTC (date, hour) 平移为本机时区 (date, hour)。
// 复用 database.UTCBucketToLocal，避免两套平移实现漂移。
func localizeUTCDateHour(utcDate string, utcHour int) (string, int) {
	return database.UTCBucketToLocal(utcDate, utcHour)
}

// localizeDate 将 UTC 日期按"UTC 中午代表点"平移为本机时区日期，
// 用于无小时粒度来源（Hermes Agent）日级的近似本地化。
// 注意：仅适用于尚未本地化的 UTC 日来源；本地日来源勿调用（会二次平移）。
func localizeDate(utcDate string) string {
	t, err := time.ParseInLocation("2006-01-02", utcDate, time.UTC)
	if err != nil {
		return utcDate
	}
	return t.Add(12 * time.Hour).In(time.Local).Format("2006-01-02")
}
