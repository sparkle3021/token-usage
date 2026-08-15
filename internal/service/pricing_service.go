package service

// Package service 的子文件，定价业务逻辑。
// 价格数据唯一存储于 model_pricing 表（LiteLLM 快照 + 用户可改），
// 拉取更新时全量 UPSERT 覆盖；Engine 保持纯内存快照，改价后 ApplyRow 同步。
import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"token-dashboard/internal/assets"
	"token-dashboard/internal/database"
	"token-dashboard/internal/model"
	"token-dashboard/internal/pricing"
)

const (
	pricingLiteLLMURL = "https://raw.githubusercontent.com/BerriAI/litellm/main/model_prices_and_context_window.json"
)

// PricingService 定价业务逻辑：seed 入库、拉取更新、用户改价转发。
type PricingService struct {
	pricing *pricing.Engine
	db      *database.Manager
	dataDir string // 仅用于 seed 时的一次性旧 JSON 迁移
}

// NewPricingService 创建定价服务实例。
func NewPricingService(pr *pricing.Engine, db *database.Manager, dataDir string) *PricingService {
	return &PricingService{pricing: pr, db: db, dataDir: dataDir}
}

// legacyPricingPath 旧版遗留的定价 JSON（config 目录曾承担价格存储，现已入库）。
// 仅作为一次性迁移源：迁移成功即删除，config 目录不再存价格信息。
func (s *PricingService) legacyPricingPath() string {
	return filepath.Join(s.dataDir, "config", "pricing-litellm.json")
}

// EnsureSeeded 保证价格数据已入库并加载进引擎。
// 表为空时 seed：优先迁移旧版遗留 JSON（一次性，成功后删除），否则使用 embed 默认数据。
// 表非空时仅加载进内存，并顺带清理遗留 JSON（价格唯一存储于 model_pricing 表）。
func (s *PricingService) EnsureSeeded() error {
	if s.db == nil || s.pricing == nil {
		return nil
	}
	n, err := s.db.CountModelPricing()
	if err != nil {
		return fmt.Errorf("count model pricing: %w", err)
	}
	if n > 0 {
		s.removeLegacyPricing()
		return s.loadFromDB()
	}

	legacyUsed := false
	var raw []byte
	if legacy, err := os.ReadFile(s.legacyPricingPath()); err == nil && len(legacy) > 0 {
		raw = legacy
		legacyUsed = true
		log.Printf("[service] pricing seed from legacy json bytes=%d", len(raw))
	} else {
		raw = assets.PricingLitellm
		log.Printf("[service] pricing seed from embed bytes=%d", len(raw))
	}

	rows, err := pricing.ParseLiteLLMRaw(raw)
	if err != nil {
		return fmt.Errorf("parse pricing seed: %w", err)
	}
	if err := s.db.UpsertModelPricing(rows); err != nil {
		return fmt.Errorf("upsert pricing seed: %w", err)
	}
	s.pricing.LoadRows(rows)
	if legacyUsed {
		s.removeLegacyPricing()
	}
	log.Printf("[service] pricing seeded rows=%d", len(rows))
	return nil
}

// removeLegacyPricing 删除旧版遗留的定价 JSON（存在才删，失败仅记日志）。
func (s *PricingService) removeLegacyPricing() {
	path := s.legacyPricingPath()
	if err := os.Remove(path); err != nil {
		if !os.IsNotExist(err) {
			log.Printf("[service] remove legacy pricing json failed path=%s err=%v", path, err)
		}
		return
	}
	log.Printf("[service] legacy pricing json removed path=%s", path)
}

func (s *PricingService) loadFromDB() error {
	rows, err := s.db.ListModelPricing()
	if err != nil {
		return fmt.Errorf("list model pricing: %w", err)
	}
	s.pricing.LoadRows(rows)
	return nil
}

// UpdatePricing 从 LiteLLM 拉取最新定价，全量 UPSERT 入库并重载引擎。
// 拉取/解析/入库任一步失败时库与引擎均保持原状（比原文件方案更安全）。
func (s *PricingService) UpdatePricing() model.PricingUpdateResult {
	log.Printf("[service] UpdatePricing started")
	result := model.PricingUpdateResult{}

	litellmData, err := fetchPricingJSON(pricingLiteLLMURL)
	if err != nil {
		log.Printf("[service] UpdatePricing litellm error=%v", err)
		result.Error = fmt.Sprintf("LiteLLM 获取失败: %v", err)
		return result
	}
	rows, err := pricing.ParseLiteLLMRaw(litellmData)
	if err != nil {
		log.Printf("[service] UpdatePricing parse error=%v", err)
		result.Error = fmt.Sprintf("价格数据解析失败: %v", err)
		return result
	}
	if err := s.db.UpsertModelPricing(rows); err != nil {
		log.Printf("[service] UpdatePricing upsert error=%v", err)
		result.Error = fmt.Sprintf("价格数据写入失败: %v", err)
		return result
	}
	s.pricing.LoadRows(rows)

	result.Litellm = len(rows)
	result.Message = fmt.Sprintf("LiteLLM %d 条", len(rows))
	log.Printf("[service] UpdatePricing done %s", result.Message)
	return result
}

// ListModelPricing 返回全部价格行（设置页价格管理列表）。
func (s *PricingService) ListModelPricing() []model.ModelPricing {
	rows, err := s.db.ListModelPricing()
	if err != nil {
		log.Printf("[service] ListModelPricing error=%v", err)
		return []model.ModelPricing{}
	}
	return rows
}

// UpdateModelPricing 用户修改单个模型价格：写库 + 同步引擎内存，立即生效。
func (s *PricingService) UpdateModelPricing(row model.ModelPricing) error {
	if strings.TrimSpace(row.ModelKey) == "" {
		return fmt.Errorf("model key 不能为空")
	}
	if err := s.db.UpdateModelPricing(row); err != nil {
		return err
	}
	s.pricing.ApplyRow(row)
	return nil
}

// DeleteModelPricing 删除单个模型价格（手动添加/残留模型），同步引擎内存。
func (s *PricingService) DeleteModelPricing(modelKey string) error {
	if err := s.db.DeleteModelPricing(modelKey); err != nil {
		return err
	}
	s.pricing.DeleteRow(modelKey)
	return nil
}

// GetPricingMeta 返回价格元信息（最近拉取时间与条目数）。
func (s *PricingService) GetPricingMeta() *model.PricingMeta {
	meta, err := s.db.PricingMeta()
	if err != nil {
		log.Printf("[service] GetPricingMeta error=%v", err)
		return &model.PricingMeta{}
	}
	return meta
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
