package collector

import "token-dashboard/internal/model"

// DeviceIdentity 采集器设备身份：默认本机 hostname，由 orchestrator 注入本机
// device_id（UUID）后统一使用稳定身份。迁移失败等注入缺失场景回退 hostname 保持可用。
type DeviceIdentity struct {
	device string
}

// SetDevice 注入设备身份（UUID），由 orchestrator 在构造后调用。
func (d *DeviceIdentity) SetDevice(id string) { d.device = id }

// Device 返回当前设备身份；未注入时回退本机 hostname。
func (d *DeviceIdentity) Device() string {
	if d.device == "" {
		return Hostname()
	}
	return d.device
}

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
	Source     string
	UsageDate  string
	Model      string
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
