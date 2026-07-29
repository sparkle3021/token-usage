// Package collector 的子文件，BigModel（智谱 AI）用量查询 Provider 实现。
package collector

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"token-dashboard/internal/model"
)

const bigModelQuotaURL = "https://www.bigmodel.cn/api/monitor/usage/quota/limit"

// bigModelResp BigModel 配额 API 响应结构。
type bigModelResp struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Data struct {
		Level  string `json:"level"`
		Limits []struct {
			Type          string  `json:"type"`
			Unit          int     `json:"unit"`
			Number        int     `json:"number"`
			Usage         int     `json:"usage"`
			CurrentValue  int     `json:"currentValue"`
			Remaining     int     `json:"remaining"`
			Percentage    int     `json:"percentage"`
			NextResetTime *int64  `json:"nextResetTime"`
		} `json:"limits"`
	} `json:"data"`
}

// unitLabel 将 BigModel unit 编码映射为展示标签。
func unitLabel(unit int) string {
	switch unit {
	case 6:
		return "5小时用量"
	case 3:
		return "每周用量"
	case 5:
		return "每月用量"
	default:
		return fmt.Sprintf("用量(%d)", unit)
	}
}

// BigModelProvider 实现 QuotaProvider 接口。
type BigModelProvider struct{}

func (p *BigModelProvider) ID() string            { return "bigmodel" }
func (p *BigModelProvider) PlanName() string      { return "Plan" }
func (p *BigModelProvider) DisplayType() string   { return DisplayQuota }
func (p *BigModelProvider) BalanceLabel() string  { return "" }

func (p *BigModelProvider) SlotsLabels() []string {
	return []string{"5小时用量", "每周用量", "每月用量"}
}

func (p *BigModelProvider) ConfigFields() []model.ConfigField {
	return []model.ConfigField{
		{Key: "token", Label: "Authorization Token", Type: "text", Placeholder: "eyJhbGciOiJIUzUxMiJ9..."},
	}
}

func (p *BigModelProvider) Fetch(configJSON string) (*model.QuotaData, error) {
	var cfg struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal([]byte(configJSON), &cfg); err != nil {
		return nil, fmt.Errorf("解析配置失败: %w", err)
	}
	if cfg.Token == "" {
		return nil, fmt.Errorf("Authorization Token 不能为空")
	}

	req, err := http.NewRequest("GET", bigModelQuotaURL, nil)
	if err != nil {
		return nil, fmt.Errorf("构建请求失败: %w", err)
	}
	req.Header.Set("Authorization", cfg.Token)

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

	var br bigModelResp
	if err := json.Unmarshal(body, &br); err != nil {
		return nil, fmt.Errorf("解析响应失败: %w", err)
	}

	if br.Code != 200 {
		return nil, fmt.Errorf("BigModel 返回错误: %s", br.Msg)
	}

	now := time.Now().UnixMilli()
	slots := make([]model.QuotaSlot, 0, len(br.Data.Limits))
	for _, l := range br.Data.Limits {
		label := unitLabel(l.Unit)
		slot := model.QuotaSlot{
			Label:        label,
			UsagePercent: l.Percentage,
		}
		if l.NextResetTime != nil && *l.NextResetTime > 0 {
			slot.ResetInSec = int((*l.NextResetTime - now) / 1000)
			if slot.ResetInSec < 0 {
				slot.ResetInSec = 0
			}
		}
		slots = append(slots, slot)
	}

	log.Printf("[bigmodel] level=%s limits=%d", br.Data.Level, len(slots))
	return &model.QuotaData{
		Provider: "bigmodel",
		Plan:     "Plan",
		Slots:    slots,
	}, nil
}

func init() {
	RegisterQuotaProvider(&BigModelProvider{})
}
