// Package collector 的子文件，用量查询供应商接口与注册机制。
package collector

import "token-dashboard/internal/model"

// DisplayType 定义展示形态。
const (
	DisplayQuota   = "quota"
	DisplayBalance = "balance"
)

// QuotaProvider 用量查询供应商接口，每个供应商实现此接口即可接入框架。
type QuotaProvider interface {
	ID() string                 // 供应商标识，如 "opencode"
	PlanName() string           // 套餐/计费类型名，如 "Go"、"API"
	DisplayType() string        // 展示形态: "quota" | "balance"
	SlotsLabels() []string      // quota 类型的用量维度标签，balance 类型返回 nil
	BalanceLabel() string       // balance 类型的余额标签，quota 类型返回 ""
	ConfigFields() []model.ConfigField // 配置表单字段定义
	Fetch(configJSON string) (*model.QuotaData, error) // 拉取实时用量数据
}

var providerRegistry = map[string]QuotaProvider{}

// RegisterQuotaProvider 注册供应商到全局 registry。
func RegisterQuotaProvider(p QuotaProvider) {
	providerRegistry[p.ID()] = p
}

// GetQuotaProvider 根据 ID 获取供应商实现。
func GetQuotaProvider(id string) QuotaProvider {
	return providerRegistry[id]
}

// ListAllProviderSchemas 返回所有已注册供应商的 schema，供前端渲染。
func ListAllProviderSchemas() []model.ProviderSchema {
	var out []model.ProviderSchema
	for _, p := range providerRegistry {
		out = append(out, model.ProviderSchema{
			ID:           p.ID(),
			PlanName:     p.PlanName(),
			DisplayType:  p.DisplayType(),
			SlotsLabels:  p.SlotsLabels(),
			BalanceLabel: p.BalanceLabel(),
			Fields:       p.ConfigFields(),
		})
	}
	return out
}
