package collector

import "token-dashboard/internal/model"

// CachePersistence is an optional interface collectors can implement to
// control when file fingerprints are persisted. The Engine calls
// PersistCache after successfully writing data, and DiscardCache on failure.
type CachePersistence interface {
	PersistCache() error
	DiscardCache()
}

type CollectResult struct {
	Device string
	Source string

	Daily   []DailyRow
	Session []SessionRow
	Events  []EventRow
	HourRows []model.HourUsage

	// Cached is true when all data was served from cache and nothing changed.
	// When true, Engine should skip SQL writes and preserve existing DB data.
	Cached bool
}

type DailyRow struct {
	Source    string // 可选，为空时使用 collector.Source()
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
