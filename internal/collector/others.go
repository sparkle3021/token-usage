package collector

import (
	"bufio"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"

	"token-dashboard/internal/debuglog"
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

type HermesCollector struct{ DeviceIdentity }

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
	start := time.Now()
	dbPath := hermesDBPath()
	log.Printf("[collector] Hermes dbPath=%s", dbPath)
	if _, err := os.Stat(dbPath); err != nil {
		log.Printf("[collector] Hermes db not found path=%s", dbPath)
		return emptyResult(c.Device(), "hermes", "Hermes Agent"), nil
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return emptyResult(c.Device(), "hermes", "Hermes Agent"), nil
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

	debuglog.Perf("Hermes collect dbPath=%s daily=%d elapsed=%v", dbPath, len(dailyMap), time.Since(start))
	log.Printf("[collector] Hermes done daily=%d", len(dailyMap))

	return buildResult(c.Device(), "hermes", "Hermes Agent", dailyMap, sessionMap, nil), nil
}

// ---------------------------------------------------------------------------
// OpenCode
// ---------------------------------------------------------------------------

type OpenCodeCollector struct{ DeviceIdentity }

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
	start := time.Now()
	dbPath := opencodeDBPath()
	log.Printf("[collector] OpenCode dbPath=%s", dbPath)
	if _, err := os.Stat(dbPath); err != nil {
		log.Printf("[collector] OpenCode db not found path=%s", dbPath)
		return emptyResult(c.Device(), "opencode", "OpenCode"), nil
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		log.Printf("[collector] OpenCode open db error: %v", err)
		return emptyResult(c.Device(), "opencode", "OpenCode"), nil
	}
	defer db.Close()

	dailyMap := make(map[string]*dailyAgg)
	sessionMap := make(map[string]*sessionAgg)
	var events []EventRow

	var scanned, parseFailed, zeroTokens, noModelID int
	var minCreated, maxCreated int64

	rows, err := db.Query(`SELECT m.id, m.session_id, m.time_created, m.data
		FROM message m
		WHERE json_extract(m.data, '$.tokens') IS NOT NULL
		AND json_extract(m.data, '$.modelID') IS NOT NULL
		ORDER BY m.time_created ASC`)
	if err != nil {
		log.Printf("[collector] OpenCode message query error: %v", err)
	} else {
		defer rows.Close()
		for rows.Next() {
			var msgID, sessionID, dataStr string
			var timeCreated int64
			if err := rows.Scan(&msgID, &sessionID, &timeCreated, &dataStr); err != nil {
				continue
			}
			scanned++

			var msg struct {
				ModelID string `json:"modelID"`
				Tokens  *struct {
					Input     int64 `json:"input"`
					Output    int64 `json:"output"`
					Reasoning int64 `json:"reasoning"`
					Cache     struct {
						Read  int64 `json:"read"`
						Write int64 `json:"write"`
					} `json:"cache"`
				} `json:"tokens"`
			}
			if err := json.Unmarshal([]byte(dataStr), &msg); err != nil {
				parseFailed++
				continue
			}
			if msg.Tokens == nil || (msg.Tokens.Input == 0 && msg.Tokens.Output == 0) {
				zeroTokens++
				continue
			}

			if minCreated == 0 || timeCreated < minCreated {
				minCreated = timeCreated
			}
			if timeCreated > maxCreated {
				maxCreated = timeCreated
			}

			modelID := NormalizeModelForGrouping(msg.ModelID)
			if modelID == "" {
				noModelID++
				modelID = "unknown"
			}

			eventTime := time.UnixMilli(timeCreated).Format(time.RFC3339)
			date := time.UnixMilli(timeCreated).Format("2006-01-02")

			t := struct{ Input, Output, CacheRead, CacheWrite, Reasoning int64 }{
				msg.Tokens.Input, msg.Tokens.Output, msg.Tokens.Cache.Read, msg.Tokens.Cache.Write, msg.Tokens.Reasoning,
			}
			cost := pricing.CalculateCost(modelID, t)

			events = append(events, EventRow{
				EventKey:         msgID,
				EventTime:        eventTime,
				UsageDate:        date,
				Model:            modelID,
				SessionID:        sessionID,
				InputTokens:      msg.Tokens.Input,
				OutputTokens:     msg.Tokens.Output,
				CacheReadTokens:  msg.Tokens.Cache.Read,
				CacheWriteTokens: msg.Tokens.Cache.Write,
				ReasoningTokens:  msg.Tokens.Reasoning,
				CostUSD:          cost,
			})

			dk := date + "::" + modelID
			if _, ok := dailyMap[dk]; !ok {
				dailyMap[dk] = &dailyAgg{date: date, model: modelID}
			}
			dailyMap[dk].add(t.Input, t.Output, t.CacheRead, t.CacheWrite, t.Reasoning, cost)
		}
	}

	debuglog.Perf("OpenCode collect scanned=%d events=%d elapsed=%v", scanned, len(events), time.Since(start))
	log.Printf("[collector] OpenCode done scanned=%d events=%d parseFailed=%d zeroTokens=%d noModelID=%d daily=%d dateRange=[%s..%s]",
		scanned, len(events), parseFailed, zeroTokens, noModelID, len(dailyMap),
		time.UnixMilli(minCreated).Format("2006-01-02"),
		time.UnixMilli(maxCreated).Format("2006-01-02"))

	return buildResult(c.Device(), "opencode", "OpenCode", dailyMap, sessionMap, events), nil
}

// ---------------------------------------------------------------------------
// OpenClaw
// ---------------------------------------------------------------------------

type OpenClawCollector struct {
	DeviceIdentity
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
	loadStart := time.Now()
	c.cache.LoadFromDB(c.Source(), allFiles)
	debuglog.Perf("OpenClaw LoadFromDB files=%d elapsed=%v", len(allFiles), time.Since(loadStart))
	checkStart := time.Now()
	if c.cache.AllCached(allFiles) {
		debuglog.Perf("OpenClaw AllCached hit files=%d elapsed=%v", len(allFiles), time.Since(checkStart))
		log.Printf("[collector] OpenClaw all files cached, skipping")
		return &CollectResult{Device: c.Device(), Source: "OpenClaw", Cached: true}, nil
	}
	debuglog.Perf("OpenClaw AllCached miss files=%d elapsed=%v", len(allFiles), time.Since(checkStart))

	dailyMap := make(map[string]*dailyAgg)
	sessionMap := make(map[string]*sessionAgg)
	var events []EventRow
	totalFiles := 0
	totalRecords := 0
	skippedFiles := 0
	parseStart := time.Now()
	for _, root := range roots {
		files := CollectJSONLFiles(root)
		totalFiles += len(files)
		for _, fp := range files {
			if c.cache.FileUnchanged(fp) {
				skippedFiles++
				continue
			}
			records := c.parseFile(fp)
			totalRecords += len(records)
			for _, rec := range records {
				date := UTCDateFromTimestamp(rec.timestamp, time.Now().Format("2006-01-02"))
				eventTime := rec.timestamp
				if u, ok := ToUTCRFC3339(rec.timestamp); ok {
					eventTime = u
				}
				model := NormalizeModelForGrouping(rec.model)
				t := struct{ Input, Output, CacheRead, CacheWrite, Reasoning int64 }{
					rec.input, rec.output, rec.cacheRead, 0, rec.reasoning,
				}
				cost := pricing.CalculateCost(model, t)

			events = append(events, EventRow{
				EventKey:   fmt.Sprintf("%s:%s:%d", fp, rec.timestamp, rec.input+rec.output),
				EventTime:  eventTime, UsageDate: date, Model: model,
				InputTokens: rec.input, OutputTokens: rec.output,
				CacheReadTokens: rec.cacheRead, ReasoningTokens: rec.reasoning, CostUSD: cost,
			})

				dk := date + "::" + model
				if _, ok := dailyMap[dk]; !ok {
					dailyMap[dk] = &dailyAgg{date: date, model: model}
				}
				dailyMap[dk].add(rec.input, rec.output, rec.cacheRead, 0, rec.reasoning, cost)
			}
		}
	}

	debuglog.Perf("OpenClaw parse files=%d skipped=%d records=%d elapsed=%v", totalFiles, skippedFiles, totalRecords, time.Since(parseStart))
	log.Printf("[collector] OpenClaw done files=%d records=%d daily=%d sessions=%d events=%d",
		totalFiles, totalRecords, len(dailyMap), len(sessionMap), len(events))

	return buildResult(c.Device(), "openclaw", "OpenClaw", dailyMap, sessionMap, events), nil
}

type openclawRecord struct {
	timestamp, model string
	input, output, cacheRead, reasoning int64
}

func (c *OpenClawCollector) parseFile(fp string) []openclawRecord {
	records, offset, state := c.cache.GetWithOffset(fp)
	if state == StateCached {
		return records.([]openclawRecord)
	}

	f, err := os.Open(fp)
	if err != nil {
		return nil
	}
	defer f.Close()

	fi, _ := f.Stat()
	fileSize := fi.Size()

	if state == StateIncremental && offset > 0 {
		f.Seek(offset, 0)
	}

	var parsed []openclawRecord
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 1<<20), 10<<20)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
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
		parsed = append(parsed, openclawRecord{
			timestamp: obj.Timestamp, model: obj.Model,
			input:     posIntFromJSON(obj.Usage.InputTokens),
			output:    posIntFromJSON(obj.Usage.OutputTokens),
			cacheRead: posIntFromJSON(obj.Usage.CacheReadInputTokens),
			reasoning: posIntFromJSON(obj.Usage.ReasoningTokens),
		})
	}

	c.cache.SetWithOffset(fp, parsed, fileSize)
	return parsed
}

// ---------------------------------------------------------------------------
// Common helpers
// ---------------------------------------------------------------------------

func emptyResult(device, id, source string) *CollectResult {
	return &CollectResult{Device: device, Source: source}
}

func buildResult(device, id, source string, dailyMap map[string]*dailyAgg, sessionMap map[string]*sessionAgg, events []EventRow) *CollectResult {
	r := &CollectResult{Device: device, Source: source}

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
