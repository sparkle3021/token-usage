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
	InputTokens           int64   `json:"inputTokens"`
	OutputTokens          int64   `json:"outputTokens"`
	CacheCreationTokens   int64   `json:"cacheCreationTokens"`
	CacheReadTokens       int64   `json:"cacheReadTokens"`
	ReasoningOutputTokens int64   `json:"reasoningOutputTokens"`
	TotalTokens           int64   `json:"totalTokens"`
	CostUSD               float64 `json:"costUSD"`
	UpdatedAt             string  `json:"-"`
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
}

type TimeSeriesData struct {
	Time []TimeUsage `json:"time"`
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

// CSVImportResult is returned by ImportCSV.
type CSVImportResult struct {
	Total    int    `json:"total"`
	Imported int    `json:"imported"`
	Skipped  int    `json:"skipped"`
	Error    string `json:"error,omitempty"`
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
