package collector

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const geminiCacheVersion = 1

type GeminiCollector struct{ cache *ParseCache }

func NewGeminiCollector() *GeminiCollector {
	return &GeminiCollector{cache: NewParseCache(geminiCacheVersion)}
}

func (c *GeminiCollector) ID() string    { return "gemini" }
func (c *GeminiCollector) Source() string { return "Gemini CLI" }

func geminiTmpDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".gemini", "tmp")
}

type geminiEvent struct {
	timestamp, date, model, sessionID string
	input, output, cacheRead, reasoning int64
}

func (c *GeminiCollector) Collect(ctx context.Context, pricing TokenCalc) (*CollectResult, error) {
	dailyMap := make(map[string]*dailyAgg)
	sessionMap := make(map[string]*sessionAgg)
	var events []EventRow
	tmpDir := geminiTmpDir()

	entries, _ := os.ReadDir(tmpDir)
	for _, entry := range entries {
		fullPath := filepath.Join(tmpDir, entry.Name())
		if entry.IsDir() {
			chatsDir := filepath.Join(fullPath, "chats")
			chatEntries, _ := os.ReadDir(chatsDir)
			for _, ce := range chatEntries {
				if ce.IsDir() {
					continue
				}
				ext := strings.ToLower(filepath.Ext(ce.Name()))
				if ext != ".json" && ext != ".jsonl" {
					continue
				}
				c.collectFile(filepath.Join(chatsDir, ce.Name()), dailyMap, sessionMap, &events, pricing)
			}
		} else {
			ext := strings.ToLower(filepath.Ext(entry.Name()))
			if ext != ".json" && ext != ".jsonl" {
				continue
			}
			if !strings.HasPrefix(entry.Name(), "session-") {
				continue
			}
			c.collectFile(fullPath, dailyMap, sessionMap, &events, pricing)
		}
	}

	return buildResult("gemini", "Gemini CLI", dailyMap, sessionMap, events), nil
}

func (c *GeminiCollector) collectFile(fp string,
	dailyMap map[string]*dailyAgg, sessionMap map[string]*sessionAgg,
	events *[]EventRow, pricing TokenCalc,
) {
	if cached, ok := c.cache.Get(fp); ok {
		for _, ev := range cached.([]geminiEvent) {
			c.accumulate(ev, dailyMap, sessionMap, events, pricing)
		}
		return
	}

	ext := strings.ToLower(filepath.Ext(fp))
	var results []geminiEvent
	switch ext {
	case ".jsonl":
		results = c.parseJSONL(fp)
	default:
		results = c.parseJSON(fp)
	}
	c.cache.Set(fp, results)
	for _, ev := range results {
		c.accumulate(ev, dailyMap, sessionMap, events, pricing)
	}
}

func (c *GeminiCollector) accumulate(ev geminiEvent,
	dailyMap map[string]*dailyAgg, sessionMap map[string]*sessionAgg,
	events *[]EventRow, pricing TokenCalc,
) {
	date := ev.date
	if date == "" {
		date = time.Now().Format("2006-01-02")
	}
	model := NormalizeModelForGrouping(ev.model)
	t := struct{ Input, Output, CacheRead, CacheWrite, Reasoning int64 }{
		ev.input, ev.output, ev.cacheRead, 0, ev.reasoning,
	}
	cost := pricing.CalculateCost(model, t)

	if keepTimeEvent(ev.timestamp) {
		*events = append(*events, EventRow{
			EventKey: fmt.Sprintf("%s:%s:%d", ev.sessionID, ev.timestamp, ev.input+ev.output),
			EventTime: ev.timestamp, UsageDate: date, Model: model,
			SessionID: ev.sessionID, ProjectPath: ev.sessionID,
			InputTokens: ev.input, OutputTokens: ev.output,
			CacheReadTokens: ev.cacheRead, ReasoningTokens: ev.reasoning, CostUSD: cost,
		})
	}

	dk := date + "::" + model
	if _, ok := dailyMap[dk]; !ok {
		dailyMap[dk] = &dailyAgg{date: date, model: model}
	}
	dailyMap[dk].add(ev.input, ev.output, ev.cacheRead, 0, ev.reasoning, cost)

	sk := ev.sessionID + "::" + model
	if _, ok := sessionMap[sk]; !ok {
		sessionMap[sk] = &sessionAgg{sessionID: ev.sessionID, projectPath: ev.sessionID, model: model}
	}
	sessionMap[sk].add(ev.input, ev.output, ev.cacheRead, 0, ev.reasoning, cost)
}

// ---------------------------------------------------------------------------
// JSON parser
// ---------------------------------------------------------------------------

func (c *GeminiCollector) parseJSON(fp string) []geminiEvent {
	data, err := os.ReadFile(fp)
	if err != nil {
		return nil
	}

	// Try full session format
	var raw struct {
		SessionID string          `json:"sessionId"`
		Messages  json.RawMessage `json:"messages"`
	}
	if err := json.Unmarshal(data, &raw); err == nil && raw.SessionID != "" {
		return c.parseSessionMessages(raw.SessionID, raw.Messages)
	}

	// Try headless stats
	var stats struct {
		Stats *json.RawMessage `json:"stats"`
	}
	if err := json.Unmarshal(data, &stats); err == nil && stats.Stats != nil {
		return c.parseHeadlessStats(*stats.Stats, "", time.Now().Format("2006-01-02"), strings.TrimSuffix(filepath.Base(fp), ".json"))
	}

	return nil
}

func (c *GeminiCollector) parseSessionMessages(sessionID string, raw json.RawMessage) []geminiEvent {
	var msgs []struct {
		Type      string `json:"type"`
		Model     string `json:"model"`
		Timestamp string `json:"timestamp"`
		Tokens    *struct {
			Input    json.Number `json:"input"`
			Output   json.Number `json:"output"`
			Cached   json.Number `json:"cached"`
			Thoughts json.Number `json:"thoughts"`
			Tool     json.Number `json:"tool"`
			Total    json.Number `json:"total"`
		} `json:"tokens"`
	}
	if err := json.Unmarshal(raw, &msgs); err != nil {
		return nil
	}

	var results []geminiEvent
	for _, msg := range msgs {
		if msg.Type != "gemini" || msg.Model == "" || msg.Tokens == nil {
			continue
		}
		date := LocalDateFromTimestamp(msg.Timestamp, time.Now().Format("2006-01-02"))
		input := posIntFromJSON(msg.Tokens.Input)
		output := posIntFromJSON(msg.Tokens.Output)
		cached := posIntFromJSON(msg.Tokens.Cached)
		reasoning := posIntFromJSON(msg.Tokens.Thoughts)
		tool := posIntFromJSON(msg.Tokens.Tool)
		total := posIntFromJSON(msg.Tokens.Total)

		netInput, cacheRead := normalizeGeminiCache(input, cached, output, reasoning, tool, total)
		results = append(results, geminiEvent{
			timestamp: msg.Timestamp, date: date, model: msg.Model,
			sessionID: sessionID, input: netInput + tool,
			output: output, cacheRead: cacheRead, reasoning: reasoning,
		})
	}
	return results
}

func (c *GeminiCollector) parseHeadlessStats(raw json.RawMessage, modelHint, date, sessionID string) []geminiEvent {
	var stats struct {
		Models map[string]struct {
			Tokens json.RawMessage `json:"tokens"`
		} `json:"models"`
		InputTokens  json.Number `json:"input_tokens"`
		OutputTokens json.Number `json:"output_tokens"`
		CachedTokens json.Number `json:"cached_tokens"`
	}
	json.Unmarshal(raw, &stats)

	var results []geminiEvent
	if stats.Models != nil {
		for modelName, modelData := range stats.Models {
			var tks struct {
				Input    json.Number `json:"input"`
				Output   json.Number `json:"output"`
				Cached   json.Number `json:"cached"`
				Thoughts json.Number `json:"thoughts"`
			}
			json.Unmarshal(modelData.Tokens, &tks)
			input := posIntFromJSON(tks.Input)
			output := posIntFromJSON(tks.Output)
			cached := posIntFromJSON(tks.Cached)
			reasoning := posIntFromJSON(tks.Thoughts)
			if input == 0 && output == 0 && cached == 0 && reasoning == 0 {
				continue
			}
			netInput, cacheRead := normalizeGeminiCache(input, cached, output, reasoning, 0, 0)
			results = append(results, geminiEvent{
				date: date, model: modelName, sessionID: sessionID,
				input: netInput, output: output, cacheRead: cacheRead, reasoning: reasoning,
			})
		}
		if len(results) > 0 {
			return results
		}
	}

	input := posIntFromJSON(stats.InputTokens)
	output := posIntFromJSON(stats.OutputTokens)
	cached := posIntFromJSON(stats.CachedTokens)
	if input == 0 && output == 0 && cached == 0 {
		return nil
	}
	netInput, cacheRead := normalizeGeminiCache(input, cached, output, 0, 0, 0)
	return []geminiEvent{{
		date: date, model: modelHint, sessionID: sessionID,
		input: netInput, output: output, cacheRead: cacheRead,
	}}
}

// ---------------------------------------------------------------------------
// JSONL parser
// ---------------------------------------------------------------------------

func (c *GeminiCollector) parseJSONL(fp string) []geminiEvent {
	f, err := os.Open(fp)
	if err != nil {
		return nil
	}
	defer f.Close()

	var results []geminiEvent
	var currentModel, sessionID string
	sessionID = strings.TrimSuffix(filepath.Base(fp), ".jsonl")
	msgIndex := make(map[string]int)

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 1<<20), 10<<20)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var obj struct {
			Type      string          `json:"type"`
			Model     string          `json:"model"`
			SID       string          `json:"session_id"`
			Timestamp string          `json:"timestamp"`
			ID        string          `json:"id"`
			Tokens    json.RawMessage `json:"tokens"`
			Stats     json.RawMessage `json:"stats"`
			Result    *struct {
				Stats json.RawMessage `json:"stats"`
			} `json:"result"`
		}
		if err := json.Unmarshal([]byte(line), &obj); err != nil {
			continue
		}

		if obj.SID != "" {
			sessionID = obj.SID
		}
		if obj.Model != "" {
			currentModel = obj.Model
		}
		if obj.Type == "init" {
			continue
		}

		if obj.Type == "gemini" && obj.Tokens != nil {
			type geminiTokens struct {
				Input    json.Number `json:"input"`
				Output   json.Number `json:"output"`
				Cached   json.Number `json:"cached"`
				Thoughts json.Number `json:"thoughts"`
				Tool     json.Number `json:"tool"`
				Total    json.Number `json:"total"`
			}
			var tks geminiTokens
			json.Unmarshal(obj.Tokens, &tks)

			input := posIntFromJSON(tks.Input)
			output := posIntFromJSON(tks.Output)
			cached := posIntFromJSON(tks.Cached)
			reasoning := posIntFromJSON(tks.Thoughts)
			tool := posIntFromJSON(tks.Tool)
			total := posIntFromJSON(tks.Total)
			netInput, cacheRead := normalizeGeminiCache(input, cached, output, reasoning, tool, total)
			date := LocalDateFromTimestamp(obj.Timestamp, time.Now().Format("2006-01-02"))

			ev := geminiEvent{
				timestamp: obj.Timestamp, date: date, model: currentModel,
				sessionID: sessionID, input: netInput + tool,
				output: output, cacheRead: cacheRead, reasoning: reasoning,
			}

			if obj.ID != "" {
				if idx, ok := msgIndex[obj.ID]; ok {
					results[idx] = ev
				} else {
					msgIndex[obj.ID] = len(results)
					results = append(results, ev)
				}
			} else {
				results = append(results, ev)
			}
			continue
		}

		stats := obj.Stats
		if stats == nil && obj.Result != nil {
			stats = obj.Result.Stats
		}
		if stats != nil {
			date := LocalDateFromTimestamp(obj.Timestamp, time.Now().Format("2006-01-02"))
			parsed := c.parseHeadlessStats(stats, currentModel, date, sessionID)
			results = append(results, parsed...)
		}
	}

	return results
}

func normalizeGeminiCache(input, cached, output, reasoning, tool, total int64) (netInput, cacheRead int64) {
	if input < 0 {
		input = 0
	}
	if cached < 0 {
		cached = 0
	}
	if total == 0 {
		cp := cached
		if cp > input {
			cp = input
		}
		return input - cp, cached
	}
	inclusiveTotal := input + output + reasoning + tool
	exclusiveTotal := inclusiveTotal + cached
	if cached > 0 && total == inclusiveTotal && total != exclusiveTotal {
		cp := cached
		if cp > input {
			cp = input
		}
		return input - cp, cached
	}
	return input, cached
}
