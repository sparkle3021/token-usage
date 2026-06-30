package engine

import (
	"context"
	"fmt"
	"log"
	"os"
	"strconv"
	"sync"
	"time"

	"token-dashboard/internal/collector"
	"token-dashboard/internal/database"
	"token-dashboard/internal/model"
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

	parallelism int
	forceFull   bool // force full collection on next run
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

	// Read parallelism from env, default to 4
	if p := os.Getenv("COLLECTOR_PARALLELISM"); p != "" {
		n, err := strconv.Atoi(p)
		if err == nil && n > 0 && n <= 16 {
			e.parallelism = n
		} else {
			e.parallelism = 4
		}
	} else {
		e.parallelism = 4
	}
	log.Printf("[engine] New parallelism=%d collectors=%d", e.parallelism, len(e.collectors))

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

// StartFullCollection forces a full collection, ignoring all incremental markers.
func (e *Engine) StartFullCollection() bool {
	e.mu.Lock()
	if e.active {
		e.mu.Unlock()
		return false
	}
	e.forceFull = true
	e.mu.Unlock()
	return e.StartCollection()
}

func (e *Engine) runCollection() {
	startedAt := time.Now().UTC().Format(time.RFC3339)

	type collectorResult struct {
		col    collector.Collector
		result *collector.CollectResult
		err    error
	}

	results := make([]*collectorResult, len(e.collectors))

	// Check forceFull: clear caches to bypass AllCached checks
	e.mu.Lock()
	ff := e.forceFull
	e.forceFull = false
	e.mu.Unlock()
	if ff {
		for _, col := range e.collectors {
			if s, ok := col.(interface{ ClearCache() }); ok {
				s.ClearCache()
			}
		}
		log.Printf("[engine] forceFull=true, cache cleared for all collectors")
	}

	// goroutine pool: parallel collect, sequential write
	sem := make(chan struct{}, e.parallelism)
	var wg sync.WaitGroup

	for i, col := range e.collectors {
		wg.Add(1)
		sem <- struct{}{}
		go func(idx int, c collector.Collector) {
			defer wg.Done()
			defer func() { <-sem }()
			defer func() {
				if r := recover(); r != nil {
					results[idx] = &collectorResult{
						col: c,
						err: fmt.Errorf("panic: %v", r),
					}
					log.Printf("[engine] collector panic source=%s err=%v", c.Source(), r)
				}
			}()

			e.emit("collector:start", map[string]interface{}{
				"source": c.Source(), "id": c.ID(),
			})

			result, err := c.Collect(context.Background(), &pricingEngine{e.pricing})
			results[idx] = &collectorResult{col: c, result: result, err: err}
		}(i, col)
	}
	wg.Wait()

	// Sequential write in original order
	var hadError bool
	var stderr string
	for _, r := range results {
		if r == nil {
			continue
		}
		if r.err != nil {
			hadError = true
			errMsg := fmt.Sprintf("[%s] %v", r.col.Source(), r.err)
			stderr += errMsg + "\n"
			log.Printf("[collector] %s", errMsg)
			e.db.RecordRun(hostname(), r.col.Source(), "error", r.err.Error(), "go-collector:"+r.col.ID())
			e.emit("collector:done", map[string]interface{}{
				"source": r.col.Source(), "status": "error", "error": r.err.Error(),
			})
			continue
		}

		ok := e.processCollector(r.result, r.col)
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
		Stderr: truncateStr(stderr, 12000),
	}
	e.active = false
	e.mu.Unlock()
	e.emit("collection:done", map[string]string{"status": status, "message": msg})

	elapsed := time.Now().UTC().Sub(mustParseRFC3339(startedAt))
	log.Printf("[engine] runCollection complete status=%s elapsed=%v", status, elapsed)
}

func (e *Engine) processCollector(result *collector.CollectResult, col collector.Collector) bool {
	// Skip SQL writes when nothing changed
	if result.Cached {
		e.db.RecordRun(result.Device, col.Source(), "ok", "cached (no changes)", "go-collector:"+col.ID())
		e.emit("collector:done", map[string]interface{}{
			"source": col.Source(), "status": "cached",
		})
		log.Printf("[engine] processCollector source=%s cached (skipped)", col.Source())
		return true
	}

	// Bulk upsert with per-call atomicity (UPSERT semantics ensure correctness)
	if err := e.db.BulkUpsertDaily(dailyToModel(result.Device, col.Source(), result.Daily)); err != nil {
		e.db.RecordRun(result.Device, col.Source(), "error",
			fmt.Sprintf("[%s] bulk daily: %v", col.Source(), err), "go-collector:"+col.ID())
		return false
	}

	if err := e.db.BulkUpsertSession(sessionToModel(result.Device, col.Source(), result.Session)); err != nil {
		e.db.RecordRun(result.Device, col.Source(), "error",
			fmt.Sprintf("[%s] bulk session: %v", col.Source(), err), "go-collector:"+col.ID())
		return false
	}

	if len(result.Events) > 0 {
		if err := e.db.BulkUpsertTimeUsage(eventsToModel(result.Device, col.Source(), result.Events)); err != nil {
			e.db.RecordRun(result.Device, col.Source(), "error",
				fmt.Sprintf("[%s] bulk time: %v", col.Source(), err), "go-collector:"+col.ID())
			return false
		}
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

// dailyToModel converts collector DailyRow to model.DailyUsage slice for bulk upsert.
func dailyToModel(device, source string, rows []collector.DailyRow) []model.DailyUsage {
	out := make([]model.DailyUsage, len(rows))
	for i, r := range rows {
		total := r.InputTokens + r.OutputTokens + r.CacheReadTokens + r.CacheWriteTokens + r.ReasoningTokens
		out[i] = model.DailyUsage{
			Device: device, Source: source, UsageDate: r.UsageDate, Model: r.Model,
			InputTokens: r.InputTokens, OutputTokens: r.OutputTokens,
			CacheCreationTokens: r.CacheWriteTokens, CacheReadTokens: r.CacheReadTokens,
			ReasoningOutputTokens: r.ReasoningTokens, TotalTokens: total, CostUSD: r.CostUSD,
		}
	}
	return out
}

// sessionToModel converts collector SessionRow to model.SessionUsage slice.
func sessionToModel(device, source string, rows []collector.SessionRow) []model.SessionUsage {
	out := make([]model.SessionUsage, len(rows))
	for i, r := range rows {
		total := r.InputTokens + r.OutputTokens + r.CacheReadTokens + r.CacheWriteTokens + r.ReasoningTokens
		out[i] = model.SessionUsage{
			Device: device, Source: source, SessionID: r.SessionID,
			LastActivity: r.LastActivity, ProjectPath: r.ProjectPath,
			InputTokens: r.InputTokens, OutputTokens: r.OutputTokens,
			CacheCreationTokens: r.CacheWriteTokens, CacheReadTokens: r.CacheReadTokens,
			ReasoningOutputTokens: r.ReasoningTokens, TotalTokens: total, CostUSD: r.CostUSD,
		}
	}
	return out
}

// eventsToModel converts collector EventRow to model.TimeUsage slice.
func eventsToModel(device, source string, rows []collector.EventRow) []model.TimeUsage {
	out := make([]model.TimeUsage, len(rows))
	for i, r := range rows {
		total := r.InputTokens + r.OutputTokens + r.CacheReadTokens + r.CacheWriteTokens + r.ReasoningTokens
		out[i] = model.TimeUsage{
			Device: device, Source: source, EventKey: r.EventKey,
			EventTime: r.EventTime, UsageDate: r.UsageDate, Model: r.Model,
			ProjectPath: r.ProjectPath, SessionID: r.SessionID,
			InputTokens: r.InputTokens, OutputTokens: r.OutputTokens,
			CacheCreationTokens: r.CacheWriteTokens, CacheReadTokens: r.CacheReadTokens,
			ReasoningOutputTokens: r.ReasoningTokens, TotalTokens: total, CostUSD: r.CostUSD,
		}
	}
	return out
}

func hostname() string {
	h, err := os.Hostname()
	if err != nil {
		return "unknown"
	}
	return h
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------


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
