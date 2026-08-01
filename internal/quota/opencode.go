// Package quota 的子文件，OpenCode Go 套餐用量查询 Provider 实现。
package quota

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"token-dashboard/internal/model"
)

// 已知有效的 OpenCode 服务端函数实例 ID。
const openCodeServerID = "c7389bd0e731f80f49593e5ee53835475f4e28594dd6bd83eb229bab753498cd"

// opencodeConfig OpenCode 配置 JSON 结构。
type opencodeConfig struct {
	AuthCookie  string `json:"authCookie"`
	WorkspaceID string `json:"workspaceId"`
}

// OpenCodeProvider 实现 QuotaProvider 接口。
type OpenCodeProvider struct{}

func (p *OpenCodeProvider) ID() string             { return "opencode" }
func (p *OpenCodeProvider) PlanName() string       { return "Go" }
func (p *OpenCodeProvider) DisplayType() string    { return DisplayQuota }
func (p *OpenCodeProvider) BalanceLabel() string   { return "" }

func (p *OpenCodeProvider) SlotsLabels() []string {
	return []string{"5小时用量", "每周用量", "每月用量"}
}

func (p *OpenCodeProvider) ConfigFields() []model.ConfigField {
	return []model.ConfigField{
		{Key: "authCookie", Label: "Auth Cookie", Type: "text", Placeholder: "Fe26.2**..."},
		{Key: "workspaceId", Label: "Workspace ID", Type: "text", Placeholder: "wrk_xxxxxxxxxxxx"},
	}
}

func (p *OpenCodeProvider) Fetch(configJSON string) (*model.QuotaData, error) {
	var cfg opencodeConfig
	if err := json.Unmarshal([]byte(configJSON), &cfg); err != nil {
		return nil, fmt.Errorf("解析配置失败: %w", err)
	}
	if cfg.AuthCookie == "" || cfg.WorkspaceID == "" {
		return nil, fmt.Errorf("Auth Cookie 和 Workspace ID 不能为空")
	}

	argsJSON := fmt.Sprintf(
		`{"t":{"t":9,"i":0,"l":1,"a":[{"t":1,"s":"%s"}],"o":0},"f":31,"m":[]}`,
		cfg.WorkspaceID,
	)
	argsEncoded := url.QueryEscape(argsJSON)
	reqURL := fmt.Sprintf("https://opencode.ai/_server?id=%s&args=%s", openCodeServerID, argsEncoded)

	req, err := http.NewRequest("GET", reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("构建请求失败: %w", err)
	}
	req.Header.Set("x-server-id", openCodeServerID)
	req.Header.Set("x-server-instance", "server-fn:1")
	req.Header.Set("accept", "*/*")
	req.AddCookie(&http.Cookie{Name: "auth", Value: cfg.AuthCookie, Path: "/", Domain: "opencode.ai"})

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("网络请求失败: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应失败: %w", err)
	}
	bodyStr := string(body)

	if strings.Contains(bodyStr, "/auth/authorize") {
		return nil, fmt.Errorf("认证已过期，请更新 OpenCode Auth Cookie")
	}

	log.Printf("[opencode] HTTP %d, body length=%d", resp.StatusCode, len(bodyStr))
	return parseOpenCodeResponse(bodyStr)
}

// parseOpenCodeResponse 从 JS 响应中提取配额数据。
func parseOpenCodeResponse(body string) (*model.QuotaData, error) {
	keyMap := []struct {
		jsKey string
		label string
	}{
		{"rollingUsage", "5小时用量"},
		{"weeklyUsage", "每周用量"},
		{"monthlyUsage", "每月用量"},
	}

	slots := make([]model.QuotaSlot, 0, len(keyMap))
	for _, km := range keyMap {
		slot := extractSlot(body, km.jsKey)
		slot.Label = km.label
		slots = append(slots, slot)
	}

	return &model.QuotaData{
		Provider: "opencode",
		Plan:     "Go",
		Slots:    slots,
	}, nil
}

// extractSlot 从 JS 响应中提取单个 usage slot。
func extractSlot(body, key string) model.QuotaSlot {
	slot := model.QuotaSlot{}
	pat := regexp.MustCompile(key + `:\$R\[\d+\]=\{([^}]*)\}`)
	m := pat.FindStringSubmatch(body)
	if len(m) < 2 {
		return slot
	}
	inner := m[1]
	if s := regexp.MustCompile(`resetInSec:(\d+)`).FindStringSubmatch(inner); len(s) > 1 {
		n, _ := strconv.Atoi(s[1])
		slot.ResetInSec = n
	}
	if s := regexp.MustCompile(`usagePercent:(\d+)`).FindStringSubmatch(inner); len(s) > 1 {
		n, _ := strconv.Atoi(s[1])
		slot.UsagePercent = n
	}
	return slot
}

func init() {
	RegisterQuotaProvider(&OpenCodeProvider{})
}
