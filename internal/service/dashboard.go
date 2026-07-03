package service

// Package service 提供业务逻辑层，负责聚合多表数据、调用定价引擎和采集编排。
// 该层不依赖 Wails 运行时，便于独立测试。
import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"token-dashboard/internal/database"
	"token-dashboard/internal/model"
	"token-dashboard/internal/pricing"
)

const (
	pricingLiteLLMURL   = "https://raw.githubusercontent.com/BerriAI/litellm/main/model_prices_and_context_window.json"
	pricingOpenRouterURL = "https://openrouter.ai/api/v1/usage/rates"
)

// DashboardService 仪表盘业务逻辑，聚合数据库原始数据并计算定价。
type DashboardService struct {
	db      *database.Manager
	pricing *pricing.Engine
	dataDir string // 数据目录，用于读写定价配置文件
}

// NewDashboardService 创建仪表盘服务实例。
func NewDashboardService(db *database.Manager, pr *pricing.Engine, dataDir string) *DashboardService {
	return &DashboardService{db: db, pricing: pr, dataDir: dataDir}
}

// GetDashboardData 获取仪表盘汇总数据，包含日用量、会话和采集运行记录。
// 自动关联 project_path 到日用量记录，并规范化运行日志。
func (s *DashboardService) GetDashboardData() *model.DashboardData {
	defer log.Printf("[service] GetDashboardData done")
	start := time.Now()

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

	return &model.DashboardData{
		Daily:    daily,
		Sessions: sessions,
		Runs:     runs,
	}
}

// GetTimeSeriesData 获取时间序列数据，包含原始事件和小时聚合两层的用量。
// 前端按 timeRows → hourRows → dailyRows 三级回退渲染趋势图。
func (s *DashboardService) GetTimeSeriesData() *model.TimeSeriesData {
	start := time.Now()
	if s.db == nil {
		log.Printf("[service] GetTimeSeriesData db=nil")
		return &model.TimeSeriesData{}
	}
	timeRows, _ := s.db.QueryTimeUsage()
	hourRows, _ := s.db.QueryHourUsage()
	log.Printf("[service] GetTimeSeriesData timeRows=%d hourRows=%d elapsed=%v", len(timeRows), len(hourRows), time.Since(start))
	return &model.TimeSeriesData{Time: timeRows, Hour: hourRows}
}

// UpdatePricing 从 LiteLLM 和 OpenRouter 拉取最新定价数据并重载定价引擎。
func (s *DashboardService) UpdatePricing() model.PricingUpdateResult {
	log.Printf("[service] UpdatePricing started")
	priceDir := filepath.Join(s.dataDir, "config")
	os.MkdirAll(priceDir, 0755)

	result := model.PricingUpdateResult{}

	litellmData, err := fetchPricingJSON(pricingLiteLLMURL)
	if err != nil {
		log.Printf("[service] UpdatePricing litellm error=%v", err)
		result.Error = fmt.Sprintf("LiteLLM 获取失败: %v", err)
		return result
	}
	if err := os.WriteFile(filepath.Join(priceDir, "pricing-litellm.json"), wrapPricingJSON(litellmData), 0644); err != nil {
		log.Printf("[service] UpdatePricing litellm write error=%v", err)
		result.Error = fmt.Sprintf("LiteLLM 写入失败: %v", err)
		return result
	}

	var litellmRaw map[string]interface{}
	json.Unmarshal(litellmData, &litellmRaw)
	result.Litellm = len(litellmRaw)

	openrouterData, err := fetchPricingJSON(pricingOpenRouterURL)
	if err == nil {
		if err := os.WriteFile(filepath.Join(priceDir, "pricing-openrouter.json"), openrouterData, 0644); err == nil {
			var orRaw map[string]interface{}
			json.Unmarshal(openrouterData, &orRaw)
			result.OpenRouter = len(orRaw)
		}
	} else {
		log.Printf("[service] UpdatePricing openrouter skipped: %v", err)
	}

	if s.pricing != nil {
		if err := s.pricing.Reload(s.dataDir); err != nil {
			log.Printf("[service] UpdatePricing reload error=%v", err)
			result.Error = fmt.Sprintf("价格引擎重载失败: %v", err)
			return result
		}
	}

	result.Message = fmt.Sprintf("LiteLLM %d 条, OpenRouter %d 条", result.Litellm, result.OpenRouter)
	log.Printf("[service] UpdatePricing done %s", result.Message)
	return result
}

func fetchPricingJSON(url string) ([]byte, error) {
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("fetch %s: %w", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetch %s: HTTP %d", url, resp.StatusCode)
	}
	return io.ReadAll(resp.Body)
}

func wrapPricingJSON(raw []byte) []byte {
	wrapped := map[string]interface{}{
		"fetchedAt": time.Now().UnixMilli(),
		"data":      json.RawMessage(raw),
	}
	b, _ := json.Marshal(wrapped)
	return b
}
