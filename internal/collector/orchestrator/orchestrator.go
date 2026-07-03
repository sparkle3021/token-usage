// Package orchestrator 采集编排引擎，负责并行运行所有 collector、顺序事务写入和检查点管理。
// 核心流程：goroutine pool 并行收集 → 顺序 processCollector 事务写入 → checkpoint 持久化。
package orchestrator

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

// Status 采集引擎运行状态，供前端轮询展示。
type Status struct {
	Status     string  `json:"status"`
	Message    string  `json:"message"`
	StartedAt  *string `json:"startedAt"`
	FinishedAt *string `json:"finishedAt"`
	ExitCode   *int    `json:"exitCode"`
	Stdout     string  `json:"stdout"`
	Stderr     string  `json:"stderr"`
}

// EventCallback 采集事件回调签名，用于桥接到 Wails 运行时。
type EventCallback func(event string, data interface{})

// Engine 采集编排引擎，管理 collector 池、并行执行和事务写入。
type Engine struct {
	collectors []collector.Collector
	db         *database.Manager
	pricing    *pricing.Engine

	ccSwitchCol *collector.CCSwitchCollector

	mu      sync.Mutex
	status  Status
	active  bool
	onEvent EventCallback

	parallelism int
	forceFull   bool // force full collection on next run
}

// New 创建采集编排引擎，初始化所有内置 collector 和 CC-Switch 收集器。
// 从环境变量 COLLECTOR_PARALLELISM 读取并发数，默认 4。
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

	// CCSwitchCollector: needs database store for checkpoint + config access
	ccCol := collector.NewCCSwitchCollector()
	ccCol.SetStore(db)
	e.ccSwitchCol = ccCol
	e.collectors = append(e.collectors, ccCol)

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

// ClearCollectorCaches clears in-memory file fingerprint caches so the next
// Collect() call re-parses all source files instead of returning Cached=true.
func (e *Engine) ClearCollectorCaches() {
	for _, col := range e.collectors {
		if s, ok := col.(interface{ ClearCache() }); ok {
			s.ClearCache()
		}
	}
	log.Printf("[engine] ClearCollectorCaches ok")
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
	now := time.Now().Format(time.RFC3339)
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

// SyncCCSwitch runs the CC-Switch collector synchronously and returns stats.
// Unlike StartCollection/StartFullCollection, this blocks until complete.
func (e *Engine) SyncCCSwitch() (collector.CCSwitchStats, error) {
	if e.ccSwitchCol == nil {
		return collector.CCSwitchStats{}, fmt.Errorf("cc-switch collector not initialized")
	}
	// Reset checkpoints to force full sync, then run synchronously
	e.db.ResetCCSwitchCheckpoints()
	e.ccSwitchCol.SetStore(e.db)
	result, err := e.ccSwitchCol.Collect(context.Background(), &pricingEngine{e.pricing})
	if err != nil {
		return e.ccSwitchCol.Stats(), err
	}
	// Write data through processCollector which uses a transaction
	if !e.processCollector(result, e.ccSwitchCol) {
		return e.ccSwitchCol.Stats(), fmt.Errorf("sync cc-switch: process failed")
	}
	// Persist checkpoints only after data is safely committed
	e.ccSwitchCol.SavePendingCheckpoints()
	// Rebuild daily from the hour data
	if err := e.db.BuildDailyFromHourUsage(); err != nil {
		return e.ccSwitchCol.Stats(), fmt.Errorf("rebuild daily: %w", err)
	}
	return e.ccSwitchCol.Stats(), nil
}

func (e *Engine) runCollection() {
	startedAt := time.Now().Format(time.RFC3339)

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

	// Wire up cache persister for collectors that support it
	persister := newCachePersister(e.db)
	for _, col := range e.collectors {
		if s, ok := col.(interface {
			SetPersister(collector.PersistHandler, string)
		}); ok {
			s.SetPersister(persister, col.Source())
		}
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
		} else if cs, ok := r.col.(interface{ SavePendingCheckpoints() }); ok {
			cs.SavePendingCheckpoints()
		}
	}

	e.mu.Lock()
	finishedAt := time.Now().Format(time.RFC3339)
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

	elapsed := time.Now().Sub(mustParseRFC3339(startedAt))
			// Rebuild daily_usage from hour_usage after all collectors finish
		if err := e.db.BuildDailyFromHourUsage(); err != nil {
			log.Printf("[engine] BuildDailyFromHourUsage error: %v", err)
		}
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

	// Filter out zero-token records before writing
	filteredDaily := make([]collector.DailyRow, 0, len(result.Daily))
	for _, r := range result.Daily {
		if r.InputTokens != 0 || r.OutputTokens != 0 || r.CacheReadTokens != 0 || r.CacheWriteTokens != 0 || r.ReasoningTokens != 0 {
			filteredDaily = append(filteredDaily, r)
		}
	}
	filteredEvents := make([]collector.EventRow, 0, len(result.Events))
	for _, r := range result.Events {
		if r.InputTokens != 0 || r.OutputTokens != 0 || r.CacheReadTokens != 0 || r.CacheWriteTokens != 0 || r.ReasoningTokens != 0 {
			filteredEvents = append(filteredEvents, r)
		}
	}

	// Write all data in a single transaction for atomicity.
	// If any step fails, the entire batch is rolled back and no cache
	// fingerprints are persisted, ensuring a subsequent run will retry.
	tx, txErr := e.db.DB().Begin()
	if txErr != nil {
		e.discardCollectorCache(col)
		log.Printf("[engine] begin tx error source=%s err=%v", col.Source(), txErr)
		return false
	}
	rollback := true
	defer func() {
		if rollback {
			tx.Rollback()
		}
	}()

	// Bulk upsert session (session data not in hour_usage, write directly)
	if err := e.db.BulkUpsertSessionTx(tx, sessionToModel(result.Device, col.Source(), result.Session)); err != nil {
		e.discardCollectorCache(col)
		e.db.RecordRun(result.Device, col.Source(), "error",
			fmt.Sprintf("[%s] bulk session: %v", col.Source(), err), "go-collector:"+col.ID())
		return false
	}

	// Event-producing collectors: write time_usage → hour_usage.
	// daily_usage is then rebuilt from hour_usage in BuildDailyFromHourUsage.
	if len(filteredEvents) > 0 {
		if err := e.db.BulkUpsertTimeUsageTx(tx, eventsToModel(result.Device, col.Source(), filteredEvents)); err != nil {
			e.discardCollectorCache(col)
			e.db.RecordRun(result.Device, col.Source(), "error",
				fmt.Sprintf("[%s] bulk time: %v", col.Source(), err), "go-collector:"+col.ID())
			return false
		}
		// Build hour_usage from time_usage for affected dates
		dateSeen := make(map[string]bool)
		for _, r := range result.Daily {
			if !dateSeen[r.UsageDate] {
				dateSeen[r.UsageDate] = true
				if err := e.db.BuildHourUsageFromTimeUsageTx(tx, result.Device, col.Source(), r.UsageDate); err != nil {
					log.Printf("[engine] BuildHourUsageFromTimeUsage error source=%s date=%s err=%v",
						col.Source(), r.UsageDate, err)
				}
			}
		}
	} else if len(filteredDaily) > 0 {
		// Non-event collectors (e.g. Hermes, OpenCode): write daily directly
		if err := e.db.BulkUpsertDailyTx(tx, dailyToModel(result.Device, col.Source(), filteredDaily)); err != nil {
			e.discardCollectorCache(col)
			e.db.RecordRun(result.Device, col.Source(), "error",
				fmt.Sprintf("[%s] bulk daily: %v", col.Source(), err), "go-collector:"+col.ID())
			return false
		}
	}

	// CC-Switch hour-level data: write directly to hour_usage within the transaction
	if len(result.HourRows) > 0 {
		if err := e.db.BulkUpsertHourUsageTx(tx, result.HourRows); err != nil {
			e.discardCollectorCache(col)
			e.db.RecordRun(result.Device, col.Source(), "error",
				fmt.Sprintf("[%s] bulk hour: %v", col.Source(), err), "go-collector:"+col.ID())
			return false
		}
	}

	// Commit the transaction — if this fails, the rollback happens automatically
	if err := tx.Commit(); err != nil {
		e.discardCollectorCache(col)
		e.db.RecordRun(result.Device, col.Source(), "error",
			fmt.Sprintf("[%s] commit tx: %v", col.Source(), err), "go-collector:"+col.ID())
		return false
	}
	rollback = false

	// Persist cache fingerprints after data is safely committed
	if err := e.persistCollectorCache(col); err != nil {
		log.Printf("[engine] PersistCache error source=%s err=%v", col.Source(), err)
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

// persistCollectorCache persists cache fingerprints after successful write.
func (e *Engine) persistCollectorCache(col collector.Collector) error {
	if cp, ok := col.(collector.CachePersistence); ok {
		return cp.PersistCache()
	}
	return nil
}

// discardCollectorCache discards pending fingerprints after a failed write.
func (e *Engine) discardCollectorCache(col collector.Collector) {
	if cp, ok := col.(collector.CachePersistence); ok {
		cp.DiscardCache()
	}
}

// sessionToModel converts collector SessionRow to model.SessionUsage slice.
func sessionToModel(device, source string, rows []collector.SessionRow) []model.SessionUsage {
	out := make([]model.SessionUsage, len(rows))
	for i, r := range rows {
		total := r.InputTokens + r.OutputTokens + r.CacheReadTokens + r.CacheWriteTokens
		out[i] = model.SessionUsage{
			Device: device, Source: source, SessionID: r.SessionID,
			LastActivity: r.LastActivity, ProjectPath: r.ProjectPath, Model: r.Model,
			InputTokens: r.InputTokens, OutputTokens: r.OutputTokens,
			CacheCreationTokens: r.CacheWriteTokens, CacheReadTokens: r.CacheReadTokens,
			ReasoningOutputTokens: r.ReasoningTokens, TotalTokens: total, CostUSD: r.CostUSD,
		}
	}
	return out
}

// dailyToModel converts collector DailyRow to model.DailyUsage slice.
// Uses row.Source when set (e.g. CC-Switch per-app_type attribution),
// otherwise falls back to the collector's default source.
func dailyToModel(device, defaultSource string, rows []collector.DailyRow) []model.DailyUsage {
	out := make([]model.DailyUsage, len(rows))
	for i, r := range rows {
		total := r.InputTokens + r.OutputTokens + r.CacheReadTokens + r.CacheWriteTokens
		src := r.Source
		if src == "" {
			src = defaultSource
		}
		out[i] = model.DailyUsage{
			Device: device, Source: src, UsageDate: r.UsageDate, Model: r.Model,
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
		total := r.InputTokens + r.OutputTokens + r.CacheReadTokens + r.CacheWriteTokens
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
		return time.Now()
	}
	return t
}
