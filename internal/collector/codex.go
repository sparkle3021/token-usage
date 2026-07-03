package collector

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
)

const codexCacheVersion = 1

type CodexCollector struct{ cache *ParseCache }

func NewCodexCollector() *CodexCollector {
	return &CodexCollector{cache: NewParseCache(codexCacheVersion)}
}

func (c *CodexCollector) ID() string    { return "codex" }
func (c *CodexCollector) Source() string { return "Codex CLI" }
func (c *CodexCollector) SetPersister(p PersistHandler, source string) { c.cache.SetPersister(p, source) }
func (c *CodexCollector) ClearCache() { c.cache.Clear() }
func (c *CodexCollector) PersistCache() error { return c.cache.PersistPending() }
func (c *CodexCollector) DiscardCache() { c.cache.DiscardPending() }

func codexRoots() []string {
	if env := os.Getenv("CODEX_HOME"); env != "" {
		return strings.Split(env, ",")
	}
	home, _ := os.UserHomeDir()
	base := filepath.Join(home, ".codex")
	return []string{filepath.Join(base, "sessions"), filepath.Join(base, "archived_sessions")}
}

func (c *CodexCollector) Collect(ctx context.Context, pricing TokenCalc) (*CollectResult, error) {
	roots := codexRoots()
	log.Printf("[collector] Codex roots=%v", roots)

	// Collect all file paths for cache check
	var allFiles []string
	for _, root := range roots {
		allFiles = append(allFiles, CollectJSONLFiles(root)...)
	}

	// Pre-load cache from DB and check if unchanged
	c.cache.LoadFromDB(c.Source(), allFiles)
	if c.cache.AllCached(allFiles) {
		log.Printf("[collector] Codex all files cached, skipping")
		return &CollectResult{Device: hostname(), Source: "Codex CLI", Cached: true}, nil
	}

	dailyMap := make(map[string]*dailyAgg)
	sessionMap := make(map[string]*sessionAgg)
	var events []EventRow
	totalFiles := 0
	totalRecords := 0
	for _, root := range roots {
		files := CollectJSONLFiles(root)
		totalFiles += len(files)
		totalFiles += len(files)
		for _, fp := range files {
			records := c.parseSessionFile(fp)
			totalRecords += len(records)
			sessionID := strings.TrimSuffix(filepath.Base(fp), ".jsonl")
			for _, rec := range records {
				date := LocalDateFromTimestamp(rec.timestamp, "unknown")
				model := NormalizeModelForGrouping(rec.model)

				t := struct{ Input, Output, CacheRead, CacheWrite, Reasoning int64 }{
					rec.input, rec.output, rec.cacheRead, 0, rec.reasoning,
				}
				cost := pricing.CalculateCost(model, t)
				workspaceKey := rec.workspace
				if workspaceKey == "" {
					workspaceKey = sessionID
				}

				events = append(events, EventRow{
				EventKey:   fmt.Sprintf("%s::%s::%d", fp, rec.timestamp, rec.input+rec.output),
				EventTime: rec.timestamp, UsageDate: date, Model: model,
				SessionID: sessionID, ProjectPath: workspaceKey,
				InputTokens: rec.input, OutputTokens: rec.output,
				CacheReadTokens: rec.cacheRead, ReasoningTokens: rec.reasoning, CostUSD: cost,
			})

				dk := date + "::" + model
				if _, ok := dailyMap[dk]; !ok {
					dailyMap[dk] = &dailyAgg{date: date, model: model}
				}
				dailyMap[dk].add(rec.input, rec.output, rec.cacheRead, 0, rec.reasoning, cost)

				sk := workspaceKey + "::" + model
				if _, ok := sessionMap[sk]; !ok {
					sessionMap[sk] = &sessionAgg{sessionID: sessionID, projectPath: workspaceKey, model: model}
				}
				sessionMap[sk].add(rec.input, rec.output, rec.cacheRead, 0, rec.reasoning, cost)
			}
		}
	}

	log.Printf("[collector] Codex done files=%d records=%d daily=%d sessions=%d events=%d",
		totalFiles, totalRecords, len(dailyMap), len(sessionMap), len(events))

	return buildResult("codex", "Codex CLI", dailyMap, sessionMap, events), nil
}

type codexEvent struct {
	timestamp                                          string
	model, workspace                                   string
	input, output, cacheRead, reasoning                int64
}

type codexUsageSummary struct {
	Input     int64 `json:"input_tokens"`
	Output    int64 `json:"output_tokens"`
	Cached    int64 `json:"cached_input_tokens"`
	Reasoning int64 `json:"reasoning_output_tokens"`
}

func (s *codexUsageSummary) equal(o codexUsageSummary) bool {
	return s.Input == o.Input && s.Output == o.Output && s.Cached == o.Cached && s.Reasoning == o.Reasoning
}

func (s *codexUsageSummary) delta(o codexUsageSummary) *codexUsageSummary {
	if s.Input < o.Input || s.Output < o.Output || s.Cached < o.Cached || s.Reasoning < o.Reasoning {
		return nil
	}
	return &codexUsageSummary{
		Input: s.Input - o.Input, Output: s.Output - o.Output,
		Cached: s.Cached - o.Cached, Reasoning: s.Reasoning - o.Reasoning,
	}
}

func (s *codexUsageSummary) isZero() bool {
	return s.Input == 0 && s.Output == 0 && s.Cached == 0 && s.Reasoning == 0
}

func (c *CodexCollector) parseSessionFile(fp string) []codexEvent {
	records, offset, state := c.cache.GetWithOffset(fp)
	if state == StateCached {
		return records.([]codexEvent)
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

	var events []codexEvent
	var currentModel string
	var previousTotal *codexUsageSummary

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 1<<20), 10<<20)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var obj struct {
			Type      string          `json:"type"`
			Payload   json.RawMessage `json:"payload"`
			Timestamp string          `json:"timestamp"`
		}
		if err := json.Unmarshal([]byte(line), &obj); err != nil {
			continue
		}

		switch obj.Type {
		case "session_meta":
			var meta struct {
				CWD string `json:"cwd"`
			}
			json.Unmarshal(obj.Payload, &meta)
			_ = meta.CWD // workspace available via event context

		case "turn_context":
			var tc struct {
				Model     string  `json:"model"`
				ModelInfo *struct {
					Slug string `json:"slug"`
				} `json:"model_info"`
			}
			json.Unmarshal(obj.Payload, &tc)
			if tc.Model != "" {
				currentModel = tc.Model
			} else if tc.ModelInfo != nil && tc.ModelInfo.Slug != "" {
				currentModel = tc.ModelInfo.Slug
			}

		case "event_msg":
			var msg struct {
				Type string          `json:"type"`
				Info json.RawMessage `json:"info"`
			}
			json.Unmarshal(obj.Payload, &msg)
			if msg.Type != "token_count" {
				continue
			}

			var info struct {
				LastUsage  *codexUsageSummary `json:"last_token_usage"`
				TotalUsage *codexUsageSummary `json:"total_token_usage"`
			}
			json.Unmarshal(msg.Info, &info)

			modelID := NormalizeModelForGrouping(currentModel)
			ts := obj.Timestamp

			if info.TotalUsage != nil && previousTotal != nil && info.TotalUsage.equal(*previousTotal) {
				continue
			}

			var inc *codexUsageSummary
			if info.LastUsage != nil {
				inc = info.LastUsage
			} else if info.TotalUsage != nil && previousTotal != nil {
				d := info.TotalUsage.delta(*previousTotal)
				if d == nil {
					previousTotal = info.TotalUsage
					continue
				}
				inc = d
			} else if info.TotalUsage != nil {
				inc = info.TotalUsage
			}

			if info.TotalUsage != nil {
				previousTotal = info.TotalUsage
			}

			if inc == nil || inc.isZero() {
				continue
			}

			// Normalize: cached portion separated from input
			cf := inc.Cached
			if cf > inc.Input {
				cf = inc.Input
			}
			events = append(events, codexEvent{
				timestamp: ts, model: modelID,
				input: inc.Input - cf, output: inc.Output,
				cacheRead: cf, reasoning: inc.Reasoning,
			})
		}
	}

	c.cache.SetWithOffset(fp, events, fileSize)
	return events
}
