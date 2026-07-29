// Package collector 的子文件，DeepSeek API 余额查询 Provider 实现。
package collector

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"

	"token-dashboard/internal/model"
)

const deepseekBalanceURL = "https://api.deepseek.com/user/balance"

// deepseekBalanceResp DeepSeek 余额 API 响应结构。
type deepseekBalanceResp struct {
	IsAvailable  bool `json:"is_available"`
	BalanceInfos []struct {
		Currency        string `json:"currency"`
		TotalBalance    string `json:"total_balance"`
		GrantedBalance  string `json:"granted_balance"`
		ToppedUpBalance string `json:"topped_up_balance"`
	} `json:"balance_infos"`
}

// DeepSeekProvider 实现 QuotaProvider 接口。
type DeepSeekProvider struct{}

func (p *DeepSeekProvider) ID() string           { return "deepseek" }
func (p *DeepSeekProvider) PlanName() string     { return "API" }
func (p *DeepSeekProvider) DisplayType() string  { return DisplayBalance }
func (p *DeepSeekProvider) SlotsLabels() []string { return nil }
func (p *DeepSeekProvider) BalanceLabel() string { return "可用余额" }

func (p *DeepSeekProvider) ConfigFields() []model.ConfigField {
	return []model.ConfigField{
		{Key: "apiKey", Label: "API Key", Type: "password", Placeholder: "sk-..."},
	}
}

func (p *DeepSeekProvider) Fetch(configJSON string) (*model.QuotaData, error) {
	var cfg struct {
		APIKey string `json:"apiKey"`
	}
	if err := json.Unmarshal([]byte(configJSON), &cfg); err != nil {
		return nil, fmt.Errorf("解析配置失败: %w", err)
	}
	if cfg.APIKey == "" {
		return nil, fmt.Errorf("API Key 不能为空")
	}

	req, err := http.NewRequest("GET", deepseekBalanceURL, nil)
	if err != nil {
		return nil, fmt.Errorf("构建请求失败: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+cfg.APIKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("网络请求失败: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应失败: %w", err)
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("API 返回 %d: %s", resp.StatusCode, string(body))
	}

	var br deepseekBalanceResp
	if err := json.Unmarshal(body, &br); err != nil {
		return nil, fmt.Errorf("解析余额响应失败: %w", err)
	}

	if !br.IsAvailable || len(br.BalanceInfos) == 0 {
		return nil, fmt.Errorf("余额服务暂不可用")
	}

	// 取第一个币种的 total_balance 作为展示余额
	var balance float64
	if _, err := fmt.Sscanf(br.BalanceInfos[0].TotalBalance, "%f", &balance); err != nil {
		return nil, fmt.Errorf("余额格式异常: %s", br.BalanceInfos[0].TotalBalance)
	}

	log.Printf("[deepseek] balance=%s %s", br.BalanceInfos[0].TotalBalance, br.BalanceInfos[0].Currency)
	return &model.QuotaData{
		Provider: "deepseek",
		Plan:     "API",
		Balance:  &balance,
	}, nil
}

func init() {
	RegisterQuotaProvider(&DeepSeekProvider{})
}
