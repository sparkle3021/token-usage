package model

type CollectionRun struct {
	ID          int64  `json:"id"`
	Device      string `json:"device"`
	Source      string `json:"source"`
	Status      string `json:"status"`
	Message     string `json:"message"`
	CollectedAt string `json:"collectedAt"`
	Command     string `json:"command,omitempty"`
}

type DailyUsage struct {
	Device                   string  `json:"device"`
	Source                   string  `json:"source"`
	UsageDate                string  `json:"usageDate"`
	Model                    string  `json:"model"`
	InputTokens              int64   `json:"inputTokens"`
	OutputTokens             int64   `json:"outputTokens"`
	CacheCreationTokens      int64   `json:"cacheCreationTokens"`
	CacheReadTokens          int64   `json:"cacheReadTokens"`
	ReasoningOutputTokens    int64   `json:"reasoningOutputTokens"`
	TotalTokens              int64   `json:"totalTokens"`
	CostUSD                  float64 `json:"costUSD"`
	PricingLockedAt          *string `json:"pricingLockedAt,omitempty"`
	ProjectPath              string  `json:"projectPath,omitempty"`
	UpdatedAt                string  `json:"-"`
}

type SessionUsage struct {
	Device                string  `json:"device"`
	Source                string  `json:"source"`
	SessionID             string  `json:"sessionId"`
	LastActivity          string  `json:"lastActivity"`
	ProjectPath           string  `json:"projectPath"`
	Model                 string  `json:"model"`
	InputTokens           int64   `json:"inputTokens"`
	OutputTokens          int64   `json:"outputTokens"`
	CacheCreationTokens   int64   `json:"cacheCreationTokens"`
	CacheReadTokens       int64   `json:"cacheReadTokens"`
	ReasoningOutputTokens int64   `json:"reasoningOutputTokens"`
	TotalTokens           int64   `json:"totalTokens"`
	CostUSD               float64 `json:"costUSD"`
	UpdatedAt             string  `json:"-"`
}

// SessionModelRow 单个会话内按模型聚合的一行，供会话详情弹窗展示。
type SessionModelRow struct {
	Model                 string  `json:"model"`
	InputTokens           int64   `json:"inputTokens"`
	OutputTokens          int64   `json:"outputTokens"`
	CacheCreationTokens   int64   `json:"cacheCreationTokens"`
	CacheReadTokens       int64   `json:"cacheReadTokens"`
	ReasoningOutputTokens int64   `json:"reasoningOutputTokens"`
	TotalTokens           int64   `json:"totalTokens"`
	CostUSD               float64 `json:"costUSD"`
}

// SessionAgg 会话聚合：由 time_usage 按 session_id 聚合而来，作为会话 Tab 数据源。
type SessionAgg struct {
	Device                string   `json:"device"`
	Source                string   `json:"source"`
	SessionID             string   `json:"sessionId"`
	ProjectPath           string   `json:"projectPath"`
	Models                []string `json:"models"`
	EventCount            int64    `json:"eventCount"`
	FirstTs               string   `json:"firstTs"`
	LastTs                string   `json:"lastTs"`
	InputTokens           int64    `json:"inputTokens"`
	OutputTokens          int64    `json:"outputTokens"`
	CacheCreationTokens   int64    `json:"cacheCreationTokens"`
	CacheReadTokens       int64    `json:"cacheReadTokens"`
	ReasoningOutputTokens int64    `json:"reasoningOutputTokens"`
	TotalTokens           int64    `json:"totalTokens"`
	CostUSD               float64  `json:"costUSD"`
}

type TimeUsage struct {
	Device                string  `json:"device"`
	Source                string  `json:"source"`
	EventKey              string  `json:"-"`
	EventTime             string  `json:"eventTime"`
	UsageDate             string  `json:"usageDate"`
	Model                 string  `json:"model"`
	ProjectPath           string  `json:"projectPath"`
	SessionID             string  `json:"sessionId"`
	InputTokens           int64   `json:"inputTokens"`
	OutputTokens          int64   `json:"outputTokens"`
	CacheCreationTokens   int64   `json:"cacheCreationTokens"`
	CacheReadTokens       int64   `json:"cacheReadTokens"`
	ReasoningOutputTokens int64   `json:"reasoningOutputTokens"`
	TotalTokens           int64   `json:"totalTokens"`
	CostUSD               float64 `json:"costUSD"`
	UpdatedAt             string  `json:"-"`
}

type DashboardData struct {
	Daily    []DailyUsage    `json:"daily"`
	Sessions []SessionUsage `json:"sessions"`
	Runs     []CollectionRun `json:"runs"`
	// DeviceNames 设备身份 → 展示名映射，前端据此渲染可读设备名。
	DeviceNames map[string]string `json:"deviceNames"`
}

type TimeSeriesData struct {
	Time []TimeUsage `json:"time"`
	Hour []HourUsage `json:"hour"`
	// DeviceNames 设备身份 → 展示名映射。
	DeviceNames map[string]string `json:"deviceNames"`
}

type CollectStatus struct {
	Status     string  `json:"status"`
	Message    string  `json:"message"`
	StartedAt  *string `json:"startedAt"`
	FinishedAt *string `json:"finishedAt"`
	ExitCode   *int    `json:"exitCode"`
	Stdout     string  `json:"stdout"`
	Stderr     string  `json:"stderr"`
}

// AppConfig holds persistent application settings.
type AppConfig struct {
	AutoSyncMinutes int    `json:"autoSyncMinutes"`
	CCSwitchDBPath  string `json:"ccSwitchDBPath"`
}

// CCSwitchImportResult is returned by import operations.
type CCSwitchImportResult struct {
	Total    int    `json:"total"`
	Imported int    `json:"imported"`
	Error    string `json:"error,omitempty"`
	Message  string `json:"message,omitempty"`
}

// PricingUpdateResult is returned by UpdatePricing.
type PricingUpdateResult struct {
	Litellm int    `json:"litellm"`
	Message string `json:"message"`
	Error   string `json:"error,omitempty"`
}

// ── 用量查询（Quota）相关结构体 ──

// QuotaConfig 持久化的用量查询配置。
type QuotaConfig struct {
	ID          int64  `json:"id"`
	Provider    string `json:"provider"`
	Plan        string `json:"plan"`
	DisplayName string `json:"displayName"`
	Seq         int    `json:"seq"`
	ConfigJSON  string `json:"configJson,omitempty"`
	IsValid     bool   `json:"isValid"`
	CreatedAt   string `json:"createdAt"`
	UpdatedAt   string `json:"updatedAt"`
}

// QuotaData 单次拉取的用量数据。
type QuotaData struct {
	ConfigID       int64           `json:"configId"`
	Provider       string          `json:"provider"`
	Plan           string          `json:"plan"`
	Name           string          `json:"name"`
	Slots          []QuotaSlot     `json:"slots,omitempty"`
	Balance        *float64        `json:"balance,omitempty"`
	BalanceDetails []BalanceDetail `json:"balanceDetails,omitempty"`
	Error          string          `json:"error,omitempty"`
	FetchedAt      string          `json:"fetchedAt"`
}

// BalanceDetail 余额明细（多币种/多类型拆分，如 DeepSeek 的充值/赠送余额）。
type BalanceDetail struct {
	Currency string  `json:"currency"`
	Total    float64 `json:"total"`
	Granted  float64 `json:"granted,omitempty"`
	ToppedUp float64 `json:"toppedUp,omitempty"`
}

// QuotaSlot 单个用量槽（quota 类型）。
type QuotaSlot struct {
	Label        string `json:"label"`
	UsagePercent int    `json:"usagePercent"`
	ResetInSec   int    `json:"resetInSec"`
}

// ProviderSchema 供应商注册信息，前端据此渲染配置表单。
type ProviderSchema struct {
	ID         string        `json:"id"`
	PlanName   string        `json:"planName"`
	DisplayType string       `json:"displayType"` // "quota" | "balance"
	SlotsLabels []string     `json:"slotsLabels,omitempty"`
	BalanceLabel string      `json:"balanceLabel,omitempty"`
	Fields      []ConfigField `json:"fields"`
}

// ConfigField 配置表单字段定义。
type ConfigField struct {
	Key         string `json:"key"`
	Label       string `json:"label"`
	Type        string `json:"type"`        // "text" | "password"
	Placeholder string `json:"placeholder"`
}

// HourUsage holds hourly aggregated token usage from both JSONL and CC-Switch.
type HourUsage struct {
	Device                string  `json:"device"`
	Source                string  `json:"source"`
	UsageDate             string  `json:"usageDate"`
	Hour                  int     `json:"hour"`
	Model                 string  `json:"model"`
	InputTokens           int64   `json:"inputTokens"`
	OutputTokens          int64   `json:"outputTokens"`
	CacheCreationTokens   int64   `json:"cacheCreationTokens"`
	CacheReadTokens       int64   `json:"cacheReadTokens"`
	ReasoningOutputTokens int64   `json:"reasoningOutputTokens"`
	TotalTokens           int64   `json:"totalTokens"`
	CostUSD               float64 `json:"costUSD"`
	UpdatedAt             string  `json:"-"`
}

// DeviceInfo 设备注册表记录：身份（UUID）与事实（hostname）与展示名（display_name）解耦。
type DeviceInfo struct {
	DeviceID    string `json:"deviceId"`
	Hostname    string `json:"hostname"`
	DisplayName string `json:"displayName"`
	IsLocal     bool   `json:"isLocal"`
}
