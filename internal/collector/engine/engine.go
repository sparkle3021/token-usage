package engine

import (
	"context"
	"fmt"
	"log"
	"os"
	"sync"
	"time"

	"token-dashboard/internal/collector"
	"token-dashboard/internal/database"
	"token-dashboard/internal/pricing"
)

type Status struct {
	Status     string  `json:"status"`
	Message    string  `json:"message"`
	StartedAt  *string `json:"startedAt"`
	FinishedAt *string `json:"finishedAt"`
	ExitCode   *int    `json:"exitCode"`
	Stdout     string  `json:"stdout"`
	Stderr     string  `json:"stderr"`
}

type EventCallback func(event string, data interface{})

type Engine struct {
	collectors []collector.Collector
	db         *database.Manager
	pricing    *pricing.Engine

	mu      sync.Mutex
	status  Status
	active  bool
	onEvent EventCallback
}

func New(db *database.Manager, pr *pricing.Engine) *Engine {
	e := &Engine{
		db:     db,
		pricing: pr,
		status: Status{Status: "idle", Message: "尚未启动采集"},
	}
	e.collectors = []collector.Collector{
		collector.NewClaudeCodeCollector(),
		collector.NewCodexCollector(),
		collector.NewGeminiCollector(),
		collector.NewHermesCollector(),
		collector.NewOpenCodeCollector(),
		collector.NewOpenClawCollector(),
	}
	_ = (*pricingEngine)(nil) // compile check
	return e
}

// pricingEngine adapts *pricing.Engine to collector.TokenCalc.
type pricingEngine struct{ inner *pricing.Engine }

func (a *pricingEngine) CalculateCost(model string, tokens struct {
	Input, Output, CacheRead, CacheWrite, Reasoning int64
}) float64 {
	return a.inner.CalculateCost(model, pricing.TokenBreakdown{
		Input: tokens.Input, Output: tokens.Output,
		CacheRead: tokens.CacheRead, CacheWrite: tokens.CacheWrite,
		Reasoning: tokens.Reasoning,
	})
}

func (e *Engine) SetEventCallback(cb EventCallback) {
	e.onEvent = cb
}

func (e *Engine) Collectors() []collector.Collector {
	return e.collectors
}

func (e *Engine) emit(event string, data interface{}) {
	if e.onEvent != nil {
		e.onEvent(event, data)
	}
}

func (e *Engine) Status() Status {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.status
}

func (e *Engine) StartCollection() bool {
	e.mu.Lock()
	if e.active {
		e.mu.Unlock()
		log.Printf("[engine] StartCollection skipped (already active)")
		return false
	}
	e.active = true
	now := time.Now().UTC().Format(time.RFC3339)
	e.status = Status{
		Status:    "running",
		Message:   "正在采集本机用量",
		StartedAt: &now,
	}
	e.mu.Unlock()
	log.Printf("[engine] StartCollection beginning collectors=%d", len(e.collectors))
	go e.runCollection()
	return true
}

func (e *Engine) runCollection() {
	startedAt := time.Now().UTC().Format(time.RFC3339)
	var hadError bool
	var stdout, stderr string

	for _, col := range e.collectors {
		e.emit("collector:start", map[string]interface{}{
			"source": col.Source(), "id": col.ID(),
		})

		result, err := col.Collect(context.Background(), &pricingEngine{e.pricing})
		if err != nil {
			hadError = true
			errMsg := fmt.Sprintf("[%s] %v", col.Source(), err)
			stderr += errMsg + "\n"
			log.Printf("[collector] %s", errMsg)
			e.db.RecordRun(hostname(), col.Source(), "error", err.Error(), "go-collector:"+col.ID())
			e.emit("collector:done", map[string]interface{}{
				"source": col.Source(), "status": "error", "error": err.Error(),
			})
			continue
		}

		ok := e.processCollector(result, col)
		if !ok {
			hadError = true
		}
	}

	e.mu.Lock()
	finishedAt := time.Now().UTC().Format(time.RFC3339)
	status := "ok"
	msg := "采集完成"
	exitCode := 0
	if hadError {
		status = "error"
		msg = "采集失败"
		exitCode = 1
	}
	e.status = Status{
		Status: status, Message: msg,
		StartedAt: &startedAt, FinishedAt: &finishedAt,
		ExitCode: &exitCode,
		Stdout: truncateStr(stdout, 12000),
		Stderr: truncateStr(stderr, 12000),
	}
	e.active = false
	e.mu.Unlock()
	e.emit("collection:done", map[string]string{"status": status, "message": msg})

	elapsed := time.Now().UTC().Sub(mustParseRFC3339(startedAt))
	log.Printf("[engine] runCollection complete status=%s elapsed=%v", status, elapsed)
}

func (e *Engine) processCollector(result *collector.CollectResult, col collector.Collector) bool {
	tx, err := e.db.DB().Begin()
	if err != nil {
		e.db.RecordRun(result.Device, col.Source(), "error", fmt.Sprintf("tx: %v", err), "go-collector:"+col.ID())
		return false
	}
	defer tx.Rollback() // harmless if already committed

	for _, row := range result.Daily {
		daily := rowToDaily(result.Device, col.Source(), row)
		if _, err := tx.Exec(upsertDailySQL, dailyArgs(daily)...); err != nil {
			e.db.RecordRun(result.Device, col.Source(), "error",
				fmt.Sprintf("[%s] upsert daily: %v", col.Source(), err), "go-collector:"+col.ID())
			return false
		}
	}

	for _, row := range result.Session {
		sess := rowToSession(result.Device, col.Source(), row)
		if _, err := tx.Exec(upsertSessionSQL, sessionArgs(sess)...); err != nil {
			e.db.RecordRun(result.Device, col.Source(), "error",
				fmt.Sprintf("[%s] upsert session: %v", col.Source(), err), "go-collector:"+col.ID())
			return false
		}
	}

	if len(result.Events) > 0 {
		if _, err := tx.Exec("DELETE FROM time_usage WHERE device = ? AND source = ?", result.Device, col.Source()); err != nil {
			e.db.RecordRun(result.Device, col.Source(), "error",
				fmt.Sprintf("[%s] delete time: %v", col.Source(), err), "go-collector:"+col.ID())
			return false
		}
		for _, row := range result.Events {
			ev := rowToTimeUsage(result.Device, col.Source(), row)
			if _, err := tx.Exec(upsertTimeSQL, timeArgs(ev)...); err != nil {
				e.db.RecordRun(result.Device, col.Source(), "error",
					fmt.Sprintf("[%s] upsert time: %v", col.Source(), err), "go-collector:"+col.ID())
				return false
			}
		}
	}

	if err := tx.Commit(); err != nil {
		e.db.RecordRun(result.Device, col.Source(), "error",
			fmt.Sprintf("[%s] commit: %v", col.Source(), err), "go-collector:"+col.ID())
		return false
	}

	summary := fmt.Sprintf("daily=%d, time=%d, workspace_model=%d", len(result.Daily), len(result.Events), len(result.Session))
	e.db.RecordRun(result.Device, col.Source(), "ok", summary, "go-collector:"+col.ID())
	e.emit("collector:done", map[string]interface{}{
		"source": col.Source(), "status": "ok", "summary": summary,
	})
	log.Printf("[engine] processCollector source=%s ok daily=%d sessions=%d events=%d",
		col.Source(), len(result.Daily), len(result.Session), len(result.Events))
	return true
}

func hostname() string {
	h, err := os.Hostname()
	if err != nil {
		return "unknown"
	}
	return h
}

// ---------------------------------------------------------------------------
// SQL statements
// ---------------------------------------------------------------------------

var upsertDailySQL = `INSERT INTO daily_usage (device, source, usage_date, model,
input_tokens, output_tokens, cache_creation_tokens, cache_read_tokens,
reasoning_output_tokens, total_tokens, cost_usd, pricing_locked_at, updated_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?,
CASE WHEN ? < date('now', 'localtime') THEN datetime('now') ELSE NULL END,
datetime('now')
) ON CONFLICT(device, source, usage_date, model) DO UPDATE SET
input_tokens=excluded.input_tokens, output_tokens=excluded.output_tokens,
cache_creation_tokens=excluded.cache_creation_tokens, cache_read_tokens=excluded.cache_read_tokens,
reasoning_output_tokens=excluded.reasoning_output_tokens, total_tokens=excluded.total_tokens,
cost_usd=CASE WHEN daily_usage.usage_date < date('now','localtime') THEN daily_usage.cost_usd ELSE excluded.cost_usd END,
pricing_locked_at=CASE WHEN daily_usage.usage_date < date('now','localtime') THEN COALESCE(daily_usage.pricing_locked_at, datetime('now')) ELSE NULL END,
updated_at=datetime('now')`

var upsertSessionSQL = `INSERT INTO session_usage (device, source, session_id, last_activity, project_path,
input_tokens, output_tokens, cache_creation_tokens, cache_read_tokens,
reasoning_output_tokens, total_tokens, cost_usd, updated_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, datetime('now'))
ON CONFLICT(device, source, session_id) DO UPDATE SET
last_activity=excluded.last_activity, project_path=excluded.project_path,
input_tokens=excluded.input_tokens, output_tokens=excluded.output_tokens,
cache_creation_tokens=excluded.cache_creation_tokens, cache_read_tokens=excluded.cache_read_tokens,
reasoning_output_tokens=excluded.reasoning_output_tokens, total_tokens=excluded.total_tokens,
cost_usd=excluded.cost_usd, updated_at=datetime('now')`

var upsertTimeSQL = `INSERT INTO time_usage (device, source, event_key, event_time, usage_date, model, project_path, session_id,
input_tokens, output_tokens, cache_creation_tokens, cache_read_tokens,
reasoning_output_tokens, total_tokens, cost_usd, updated_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, datetime('now'))
ON CONFLICT(device, source, event_key) DO UPDATE SET
event_time=excluded.event_time, usage_date=excluded.usage_date,
model=excluded.model, project_path=excluded.project_path, session_id=excluded.session_id,
input_tokens=excluded.input_tokens, output_tokens=excluded.output_tokens,
cache_creation_tokens=excluded.cache_creation_tokens, cache_read_tokens=excluded.cache_read_tokens,
reasoning_output_tokens=excluded.reasoning_output_tokens, total_tokens=excluded.total_tokens,
cost_usd=excluded.cost_usd, updated_at=datetime('now')`

// ---------------------------------------------------------------------------
// Row converters
// ---------------------------------------------------------------------------

type dailyUpsert struct {
	Device, Source, UsageDate, Model string
	Input, Output, CacheCreation, CacheRead, Reasoning, Total int64
	CostUSD float64
}

func rowToDaily(device, source string, r collector.DailyRow) dailyUpsert {
	total := r.InputTokens + r.OutputTokens + r.CacheReadTokens + r.CacheWriteTokens + r.ReasoningTokens
	return dailyUpsert{
		Device: device, Source: source, UsageDate: r.UsageDate, Model: r.Model,
		Input: r.InputTokens, Output: r.OutputTokens,
		CacheCreation: r.CacheWriteTokens, CacheRead: r.CacheReadTokens,
		Reasoning: r.ReasoningTokens, Total: total, CostUSD: r.CostUSD,
	}
}

func dailyArgs(d dailyUpsert) []interface{} {
	return []interface{}{d.Device, d.Source, d.UsageDate, d.Model,
		d.Input, d.Output, d.CacheCreation, d.CacheRead,
		d.Reasoning, d.Total, d.CostUSD, d.UsageDate,
	}
}

type sessionUpsert struct {
	Device, Source, SessionID, LastActivity, ProjectPath string
	Input, Output, CacheCreation, CacheRead, Reasoning, Total int64
	CostUSD float64
}

func rowToSession(device, source string, r collector.SessionRow) sessionUpsert {
	total := r.InputTokens + r.OutputTokens + r.CacheReadTokens + r.CacheWriteTokens + r.ReasoningTokens
	return sessionUpsert{
		Device: device, Source: source, SessionID: r.SessionID,
		LastActivity: r.LastActivity, ProjectPath: r.ProjectPath,
		Input: r.InputTokens, Output: r.OutputTokens,
		CacheCreation: r.CacheWriteTokens, CacheRead: r.CacheReadTokens,
		Reasoning: r.ReasoningTokens, Total: total, CostUSD: r.CostUSD,
	}
}

func sessionArgs(s sessionUpsert) []interface{} {
	return []interface{}{s.Device, s.Source, s.SessionID, strPtr(s.LastActivity), strPtr(s.ProjectPath),
		s.Input, s.Output, s.CacheCreation, s.CacheRead,
		s.Reasoning, s.Total, s.CostUSD,
	}
}

type timeUpsert struct {
	Device, Source, EventKey, EventTime, UsageDate, Model, ProjectPath, SessionID string
	Input, Output, CacheCreation, CacheRead, Reasoning, Total int64
	CostUSD float64
}

func rowToTimeUsage(device, source string, r collector.EventRow) timeUpsert {
	total := r.InputTokens + r.OutputTokens + r.CacheReadTokens + r.CacheWriteTokens + r.ReasoningTokens
	return timeUpsert{
		Device: device, Source: source,
		EventKey: r.EventKey, EventTime: r.EventTime, UsageDate: r.UsageDate,
		Model: r.Model, ProjectPath: r.ProjectPath, SessionID: r.SessionID,
		Input: r.InputTokens, Output: r.OutputTokens,
		CacheCreation: r.CacheWriteTokens, CacheRead: r.CacheReadTokens,
		Reasoning: r.ReasoningTokens, Total: total, CostUSD: r.CostUSD,
	}
}

func timeArgs(t timeUpsert) []interface{} {
	return []interface{}{t.Device, t.Source, t.EventKey, t.EventTime, t.UsageDate,
		t.Model, strPtr(t.ProjectPath), strPtr(t.SessionID),
		t.Input, t.Output, t.CacheCreation, t.CacheRead,
		t.Reasoning, t.Total, t.CostUSD,
	}
}

func strPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func truncateStr(s string, max int) string {
	if len(s) > max {
		return s[len(s)-max:]
	}
	return s
}

func mustParseRFC3339(s string) time.Time {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return time.Now().UTC()
	}
	return t
}
