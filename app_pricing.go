package main

import "token-dashboard/internal/model"

// ---------------------------------------------------------------------------
// Pricing API
// ---------------------------------------------------------------------------

// UpdatePricing 从 LiteLLM 拉取最新定价，全量 UPSERT 入库并重载引擎。
func (a *App) UpdatePricing() model.PricingUpdateResult {
	return a.pricingSvc.UpdatePricing()
}

// ListModelPricing 返回全部模型价格（设置页价格管理列表）。
func (a *App) ListModelPricing() []model.ModelPricing {
	return a.pricingSvc.ListModelPricing()
}

// UpdateModelPricing 用户修改单个模型价格（写库 + 同步引擎，立即生效）。
func (a *App) UpdateModelPricing(row model.ModelPricing) error {
	return a.pricingSvc.UpdateModelPricing(row)
}

// DeleteModelPricing 删除单个模型价格。
func (a *App) DeleteModelPricing(modelKey string) error {
	return a.pricingSvc.DeleteModelPricing(modelKey)
}

// GetPricingMeta 返回价格元信息（最近拉取时间与条目数）。
func (a *App) GetPricingMeta() *model.PricingMeta {
	return a.pricingSvc.GetPricingMeta()
}
