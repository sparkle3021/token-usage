package collector

// CollectResult holds the normalized output from a collector run.
type CollectResult struct {
	Device string
	Source string

	Daily   []DailyRow
	Session []SessionRow
	Events  []EventRow

	// Cached is true when all data was served from cache and nothing changed.
	// When true, Engine should skip SQL writes and preserve existing DB data.
	Cached bool
}

type DailyRow struct {
	UsageDate string
	Model     string
	InputTokens int64
	OutputTokens int64
	CacheReadTokens int64
	CacheWriteTokens int64
	ReasoningTokens int64
	CostUSD    float64
}

type SessionRow struct {
	SessionID    string
	LastActivity string
	ProjectPath  string
	Model        string
	InputTokens  int64
	OutputTokens int64
	CacheReadTokens int64
	CacheWriteTokens int64
	ReasoningTokens int64
	CostUSD       float64
}

type EventRow struct {
	EventKey   string
	EventTime  string
	UsageDate  string
	Model      string
	ProjectPath string
	SessionID  string
	InputTokens  int64
	OutputTokens int64
	CacheReadTokens int64
	CacheWriteTokens int64
	ReasoningTokens int64
	CostUSD     float64
}
