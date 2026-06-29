package collector

import "context"

// Collector is the interface all data source collectors implement.
type Collector interface {
	// ID returns a unique short identifier (e.g. "claude-code", "codex").
	ID() string

	// Source returns the human-readable label (e.g. "Claude Code").
	Source() string

	// Collect scans the local machine for this tool's usage data and returns
	// normalized daily, session, and event rows.
	Collect(ctx context.Context, pricing TokenCalc) (*CollectResult, error)
}

// TokenCalc is a minimal pricing interface for collectors.
type TokenCalc interface {
	CalculateCost(model string, tokens struct {
		Input, Output, CacheRead, CacheWrite, Reasoning int64
	}) float64
}
