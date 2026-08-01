package service

// Package service 的子文件，定价更新业务逻辑。
// 从远程源（LiteLLM）拉取最新定价数据、落盘并重载定价引擎。
import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"token-dashboard/internal/model"
	"token-dashboard/internal/pricing"
)

const (
	pricingLiteLLMURL = "https://raw.githubusercontent.com/BerriAI/litellm/main/model_prices_and_context_window.json"
)

// PricingService 定价更新业务逻辑。
type PricingService struct {
	pricing *pricing.Engine
	dataDir string
}

// NewPricingService 创建定价服务实例。
func NewPricingService(pr *pricing.Engine, dataDir string) *PricingService {
	return &PricingService{pricing: pr, dataDir: dataDir}
}

// UpdatePricing 从 LiteLLM 拉取最新定价数据并重载定价引擎。
func (s *PricingService) UpdatePricing() model.PricingUpdateResult {
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

	if s.pricing != nil {
		if err := s.pricing.Reload(s.dataDir); err != nil {
			log.Printf("[service] UpdatePricing reload error=%v", err)
			result.Error = fmt.Sprintf("价格引擎重载失败: %v", err)
			return result
		}
	}

	result.Message = fmt.Sprintf("LiteLLM %d 条", result.Litellm)
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
