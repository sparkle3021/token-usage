package collector

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

// ---------------------------------------------------------------------------
// Hermes
// ---------------------------------------------------------------------------

type HermesCollector struct{}

func NewHermesCollector() *HermesCollector {
	return &HermesCollector{}
}

func (c *HermesCollector) ID() string    { return "hermes" }
func (c *HermesCollector) Source() string { return "Hermes Agent" }

func hermesDBPath() string {
	if env := os.Getenv("HERMES_HOME"); env != "" {
		return filepath.Join(env, "state.db")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".hermes", "state.db")
}

func (c *HermesCollector) Collect(ctx context.Context, pricing TokenCalc) (*CollectResult, error) {
	dbPath := hermesDBPath()
	log.Printf("[collector] Hermes dbPath=%s", dbPath)
	if _, err := os.Stat(dbPath); err != nil {
		log.Printf("[collector] Hermes db not found path=%s", dbPath)
		return emptyResult("hermes", "Hermes Agent"), nil
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return emptyResult("hermes", "Hermes Agent"), nil
	}
	defer db.Close()

	dailyMap := make(map[string]*dailyAgg)
	sessionMap := make(map[string]*sessionAgg)

	rows, err := db.Query(`SELECT date, model, input_tokens, output_tokens, cached_tokens FROM daily_usage`)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var date, model string
			var input, output, cached int64
			rows.Scan(&date, &model, &input, &output, &cached)
			model = NormalizeModelForGrouping(model)
			cp := cached
			if cp > input {
				cp = input
			}
			t := struct{ Input, Output, CacheRead, CacheWrite, Reasoning int64 }{
				input - cp, output, cp, 0, 0,
			}
			cost := pricing.CalculateCost(model, t)
			dk := date + "::" + model
			if _, ok := dailyMap[dk]; !ok {
				dailyMap[dk] = &dailyAgg{date: date, model: model}
			}
			dailyMap[dk].add(t.Input, t.Output, t.CacheRead, 0, 0, cost)
		}
	}

	log.Printf("[collector] Hermes done daily=%d", len(dailyMap))

	return buildResult("hermes", "Hermes Agent", dailyMap, sessionMap, nil), nil
}

// ---------------------------------------------------------------------------
// OpenCode
// ---------------------------------------------------------------------------

type OpenCodeCollector struct{}

func NewOpenCodeCollector() *OpenCodeCollector {
	return &OpenCodeCollector{}
}

func (c *OpenCodeCollector) ID() string    { return "opencode" }
func (c *OpenCodeCollector) Source() string { return "OpenCode" }

func opencodeDBPath() string {
	if env := os.Getenv("OPENCODE_DATA_DIR"); env != "" {
		return filepath.Join(env, "opencode.db")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".local", "share", "opencode", "opencode.db")
}

func (c *OpenCodeCollector) Collect(ctx context.Context, pricing TokenCalc) (*CollectResult, error) {
	dbPath := opencodeDBPath()
	log.Printf("[collector] OpenCode dbPath=%s", dbPath)
	if _, err := os.Stat(dbPath); err != nil {
		log.Printf("[collector] OpenCode db not found path=%s", dbPath)
		return emptyResult("opencode", "OpenCode"), nil
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return emptyResult("opencode", "OpenCode"), nil
	}
	defer db.Close()

	dailyMap := make(map[string]*dailyAgg)
	sessionMap := make(map[string]*sessionAgg)

	rows, err := db.Query(`SELECT time_created, model, tokens_input, tokens_output, tokens_reasoning, tokens_cache_read, tokens_cache_write, cost FROM session WHERE model IS NOT NULL AND model != ''`)
	if err != nil {
		log.Printf("[collector] OpenCode session query error: %v", err)
	} else {
		defer rows.Close()
		for rows.Next() {
			var timeCreated int64
			var modelJSON string
			var inp, out, reas, cr, cw int64
			var cost float64
			if err := rows.Scan(&timeCreated, &modelJSON, &inp, &out, &reas, &cr, &cw, &cost); err != nil {
				continue
			}

			modelID := parseModelID(modelJSON)
			if modelID == "" {
				continue
			}
			modelID = NormalizeModelForGrouping(modelID)
			if modelID == "" {
				modelID = "unknown"
			}

			date := time.UnixMilli(timeCreated).Format("2006-01-02")

			t := struct{ Input, Output, CacheRead, CacheWrite, Reasoning int64 }{
				inp, out, cr, cw, reas,
			}
			calcCost := pricing.CalculateCost(modelID, t)
			if calcCost == 0 && cost > 0 {
				calcCost = cost
			}

			dk := date + "::" + modelID
			if _, ok := dailyMap[dk]; !ok {
				dailyMap[dk] = &dailyAgg{date: date, model: modelID}
			}
			dailyMap[dk].add(t.Input, t.Output, t.CacheRead, t.CacheWrite, t.Reasoning, calcCost)
		}
	}

	log.Printf("[collector] OpenCode done daily=%d", len(dailyMap))

	return buildResult("opencode", "OpenCode", dailyMap, sessionMap, nil), nil
}

func parseModelID(jsonStr string) string {
	var obj struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal([]byte(jsonStr), &obj); err != nil {
		return ""
	}
	return obj.ID
}

// ---------------------------------------------------------------------------
// OpenClaw
// ---------------------------------------------------------------------------

type OpenClawCollector struct {
	cache *ParseCache
}

func NewOpenClawCollector() *OpenClawCollector {
	return &OpenClawCollector{cache: NewParseCache(1)}
}

func (c *OpenClawCollector) ID() string    { return "openclaw" }
func (c *OpenClawCollector) Source() string { return "OpenClaw" }
func (c *OpenClawCollector) SetPersister(p PersistHandler, source string) { c.cache.SetPersister(p, source) }
func (c *OpenClawCollector) ClearCache() { c.cache.Clear() }
func (c *OpenClawCollector) PersistCache() error { return c.cache.PersistPending() }
func (c *OpenClawCollector) DiscardCache() { c.cache.DiscardPending() }

func openclawRoots() []string {
	home, _ := os.UserHomeDir()
	return []string{
		filepath.Join(home, ".openclaw", "agents"),
		filepath.Join(home, ".clawdbot", "agents"),
		filepath.Join(home, ".moltbot", "agents"),
		filepath.Join(home, ".moldbot", "agents"),
	}
}

func (c *OpenClawCollector) Collect(ctx context.Context, pricing TokenCalc) (*CollectResult, error) {
	roots := openclawRoots()
	log.Printf("[collector] OpenClaw roots=%v", roots)

	var allFiles []string
	for _, root := range roots {
		allFiles = append(allFiles, CollectJSONLFiles(root)...)
	}

	// Pre-load cache from DB and check if unchanged
	c.cache.LoadFromDB(c.Source(), allFiles)
	if c.cache.AllCached(allFiles) {
		log.Printf("[collector] OpenClaw all files cached, skipping")
		return &CollectResult{Device: hostname(), Source: "OpenClaw", Cached: true}, nil
	}

	dailyMap := make(map[string]*dailyAgg)
	sessionMap := make(map[string]*sessionAgg)
	var events []EventRow
	totalFiles := 0
	totalRecords := 0
	for _, root := range roots {
		files := CollectJSONLFiles(root)
		totalFiles += len(files)
		for _, fp := range files {
			records := c.parseFile(fp)
			totalRecords += len(records)
			for _, rec := range records {
				date := LocalDateFromTimestamp(rec.timestamp, time.Now().Format("2006-01-02"))
				model := NormalizeModelForGrouping(rec.model)
				t := struct{ Input, Output, CacheRead, CacheWrite, Reasoning int64 }{
					rec.input, rec.output, rec.cacheRead, 0, rec.reasoning,
				}
				cost := pricing.CalculateCost(model, t)

				if keepTimeEvent(rec.timestamp) {
					events = append(events, EventRow{
						EventKey:   fmt.Sprintf("%s:%s:%d", fp, rec.timestamp, rec.input+rec.output),
						EventTime:  rec.timestamp, UsageDate: date, Model: model,
						InputTokens: rec.input, OutputTokens: rec.output,
						CacheReadTokens: rec.cacheRead, ReasoningTokens: rec.reasoning, CostUSD: cost,
					})
				}

				dk := date + "::" + model
				if _, ok := dailyMap[dk]; !ok {
					dailyMap[dk] = &dailyAgg{date: date, model: model}
				}
				dailyMap[dk].add(rec.input, rec.output, rec.cacheRead, 0, rec.reasoning, cost)
			}
		}
	}

	log.Printf("[collector] OpenClaw done files=%d records=%d daily=%d sessions=%d events=%d",
		totalFiles, totalRecords, len(dailyMap), len(sessionMap), len(events))

	return buildResult("openclaw", "OpenClaw", dailyMap, sessionMap, events), nil
}

type openclawRecord struct {
	timestamp, model string
	input, output, cacheRead, reasoning int64
}

func (c *OpenClawCollector) parseFile(fp string) []openclawRecord {
	if cached, ok := c.cache.Get(fp); ok {
		return cached.([]openclawRecord)
	}

	data, err := os.ReadFile(fp)
	if err != nil {
		return nil
	}

	var records []openclawRecord
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var obj struct {
			Type      string `json:"type"`
			Model     string `json:"model"`
			Timestamp string `json:"timestamp"`
			Usage     *struct {
				InputTokens            json.Number `json:"input_tokens"`
				OutputTokens           json.Number `json:"output_tokens"`
				CacheReadInputTokens   json.Number `json:"cache_read_input_tokens"`
				ReasoningTokens        json.Number `json:"reasoning_tokens"`
			} `json:"usage"`
		}
		if err := json.Unmarshal([]byte(line), &obj); err != nil {
			continue
		}
		if obj.Type != "assistant" || obj.Usage == nil {
			continue
		}
		records = append(records, openclawRecord{
			timestamp: obj.Timestamp, model: obj.Model,
			input:     posIntFromJSON(obj.Usage.InputTokens),
			output:    posIntFromJSON(obj.Usage.OutputTokens),
			cacheRead: posIntFromJSON(obj.Usage.CacheReadInputTokens),
			reasoning: posIntFromJSON(obj.Usage.ReasoningTokens),
		})
	}

	c.cache.Set(fp, records)
	return records
}

// ---------------------------------------------------------------------------
// Common helpers
// ---------------------------------------------------------------------------

func emptyResult(id, source string) *CollectResult {
	return &CollectResult{Device: hostname(), Source: source}
}

func buildResult(id, source string, dailyMap map[string]*dailyAgg, sessionMap map[string]*sessionAgg, events []EventRow) *CollectResult {
	r := &CollectResult{Device: hostname(), Source: source}

	for _, agg := range dailyMap {
		r.Daily = append(r.Daily, DailyRow{
			UsageDate: agg.date, Model: agg.model,
			InputTokens: agg.input, OutputTokens: agg.output,
			CacheReadTokens: agg.cacheRead, CacheWriteTokens: agg.cacheWrite,
			ReasoningTokens: agg.reasoning, CostUSD: agg.cost,
		})
	}
	sort.Slice(r.Daily, func(i, j int) bool {
		return r.Daily[i].UsageDate < r.Daily[j].UsageDate
	})

	for _, agg := range sessionMap {
		r.Session = append(r.Session, SessionRow{
			SessionID: agg.sessionID, ProjectPath: agg.projectPath, Model: agg.model,
			InputTokens: agg.input, OutputTokens: agg.output,
			CacheReadTokens: agg.cacheRead, CacheWriteTokens: agg.cacheWrite,
			ReasoningTokens: agg.reasoning, CostUSD: agg.cost,
		})
	}

	if events != nil {
		r.Events = events
	}

	return r
}
