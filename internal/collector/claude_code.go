package collector

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const (
	claudeClientKey    = "claude"
	claudeSourceLabel  = "Claude Code"
	claudeCacheVersion = 1
)

var claudeEventCutoff = time.Now().AddDate(0, 0, -90).UnixMilli()

type ClaudeCodeCollector struct {
	cache *ParseCache
}

func NewClaudeCodeCollector() *ClaudeCodeCollector {
	return &ClaudeCodeCollector{cache: NewParseCache(claudeCacheVersion)}
}

func (c *ClaudeCodeCollector) ID() string    { return claudeClientKey }
func (c *ClaudeCodeCollector) Source() string { return claudeSourceLabel }

func getClaudeRoots() []string {
	if env := os.Getenv("CLAUDE_CONFIG_DIR"); env != "" {
		return strings.Split(env, ",")
	}
	home, _ := os.UserHomeDir()
	return []string{
		filepath.Join(home, ".claude"),
		filepath.Join(home, ".config", "claude"),
	}
}

func (c *ClaudeCodeCollector) Collect(ctx context.Context, pricing TokenCalc) (*CollectResult, error) {
	dailyMap := make(map[string]*dailyAgg)
	sessionMap := make(map[string]*sessionAgg)
	var events []EventRow

	roots := getClaudeRoots()
	log.Printf("[collector] ClaudeCode roots=%v", roots)
	for _, root := range roots {
		projectsDir := filepath.Join(root, "projects")
		if info, err := os.Stat(projectsDir); err == nil && info.IsDir() {
			log.Printf("[collector] ClaudeCode scanning projects dir=%s", projectsDir)
			c.scanAndParse(projectsDir, dailyMap, sessionMap, &events, pricing)
		}
		transcriptsDir := filepath.Join(root, "transcripts")
		if info, err := os.Stat(transcriptsDir); err == nil && info.IsDir() {
			log.Printf("[collector] ClaudeCode scanning transcripts dir=%s", transcriptsDir)
			c.scanAndParse(transcriptsDir, dailyMap, sessionMap, &events, pricing)
		}
	}

	result := &CollectResult{Device: hostname(), Source: claudeSourceLabel}
	for _, agg := range dailyMap {
		result.Daily = append(result.Daily, DailyRow{
			UsageDate: agg.date, Model: agg.model,
			InputTokens: agg.input, OutputTokens: agg.output,
			CacheReadTokens: agg.cacheRead, CacheWriteTokens: agg.cacheWrite,
			ReasoningTokens: agg.reasoning, CostUSD: agg.cost,
		})
	}
	sort.Slice(result.Daily, func(i, j int) bool {
		return result.Daily[i].UsageDate < result.Daily[j].UsageDate
	})
	for _, agg := range sessionMap {
		result.Session = append(result.Session, SessionRow{
			SessionID: agg.sessionID, LastActivity: time.Now().UTC().Format(time.RFC3339),
			ProjectPath: agg.projectPath, Model: agg.model,
			InputTokens: agg.input, OutputTokens: agg.output,
			CacheReadTokens: agg.cacheRead, CacheWriteTokens: agg.cacheWrite,
			ReasoningTokens: agg.reasoning, CostUSD: agg.cost,
		})
	}
	result.Events = events
	return result, nil
}

func (c *ClaudeCodeCollector) scanAndParse(dir string,
	dailyMap map[string]*dailyAgg, sessionMap map[string]*sessionAgg,
	events *[]EventRow, pricing TokenCalc,
) {
	files := CollectJSONLFiles(dir)
	recordCount := 0
	for _, filePath := range files {
		records := c.parseFile(filePath)
		recordCount += len(records)
		for _, rec := range records {
			date := LocalDateFromTimestamp(rec.timestamp, "unknown")
			if date == "unknown" {
				continue
			}
			model := NormalizeModelForGrouping(rec.model)
			t := struct{ Input, Output, CacheRead, CacheWrite, Reasoning int64 }{
				rec.input, rec.output, rec.cacheRead, rec.cacheWrite, rec.reasoning,
			}
			cost := pricing.CalculateCost(model, t)

			workspaceKey := workspaceKeyFromPath(dir, filePath)
			workspaceLabel := decodeWorkspaceLabel(workspaceKey)

			if keepTimeEvent(rec.timestamp) {
				*events = append(*events, EventRow{
					EventKey:   fmt.Sprintf("%s:%s:%s:%d", filePath, rec.timestamp, model, rec.input+rec.output),
					EventTime: rec.timestamp, UsageDate: date, Model: model,
					SessionID: filePath, ProjectPath: workspaceLabel,
					InputTokens: rec.input, OutputTokens: rec.output,
					CacheReadTokens: rec.cacheRead, CacheWriteTokens: rec.cacheWrite,
					ReasoningTokens: rec.reasoning, CostUSD: cost,
				})
			}

			dk := date + "::" + model
			if _, ok := dailyMap[dk]; !ok {
				dailyMap[dk] = &dailyAgg{date: date, model: model}
			}
			dailyMap[dk].add(rec.input, rec.output, rec.cacheRead, rec.cacheWrite, rec.reasoning, cost)

			sk := workspaceKey + "::" + model
			if _, ok := sessionMap[sk]; !ok {
				sessionMap[sk] = &sessionAgg{
					sessionID: filePath, projectPath: workspaceLabel, model: model,
				}
			}
			sessionMap[sk].add(rec.input, rec.output, rec.cacheRead, rec.cacheWrite, rec.reasoning, cost)
		}
	}
	log.Printf("[collector] ClaudeCode scanAndParse dir=%s files=%d records=%d", dir, len(files), recordCount)
}

func (c *ClaudeCodeCollector) parseFile(filePath string) []claudeRecord {
	if cached, ok := c.cache.Get(filePath); ok {
		return cached.([]claudeRecord)
	}

	f, err := os.Open(filePath)
	if err != nil {
		return nil
	}
	defer f.Close()

	var records []claudeRecord
	dedupIndex := make(map[string]int)
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 1024*1024), 10*1024*1024)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var obj claudeJSONLine
		if err := json.Unmarshal([]byte(line), &obj); err != nil {
			continue
		}
		if obj.Type != "assistant" || obj.Message == nil || obj.Message.Usage == nil {
			continue
		}

		usage := obj.Message.Usage
		rec := claudeRecord{
			timestamp: obj.Timestamp,
			model:     obj.Message.Model,
		}
		if obj.Model != "" && rec.model == "" {
			rec.model = obj.Model
		}
		if rec.model == "" {
			rec.model = "unknown"
		}
			if strings.Contains(rec.model, "synthetic") {
				continue
			}

		rec.input = posIntFromJSON(usage.InputTokens)
		rec.output = posIntFromJSON(usage.OutputTokens)
		cr := posIntFromJSON(usage.CacheReadInputTokens)
		cc := posIntFromJSON(usage.CacheCreationInputTokens)
		if cr > cc {
			rec.cacheRead = cr
		} else {
			rec.cacheRead = cc
		}
		rec.reasoning = maxInt64(posIntFromJSON(usage.ReasoningTokens), posIntFromJSON(usage.ThinkingTokens))
		if obj.CostUSD > 0 {
			rec.cost = obj.CostUSD
		}

		dedupKey := ""
		if obj.Message.ID != "" {
			if obj.RequestID != "" {
				dedupKey = obj.Message.ID + ":" + obj.RequestID
			} else {
				dedupKey = "message:" + obj.Message.ID
			}
		}
		if dedupKey != "" {
			if idx, ok := dedupIndex[dedupKey]; ok {
				existing := &records[idx]
				if rec.input > existing.input {
					existing.input = rec.input
				}
				if rec.output > existing.output {
					existing.output = rec.output
				}
				if rec.cacheRead > existing.cacheRead {
					existing.cacheRead = rec.cacheRead
				}
				if rec.reasoning > existing.reasoning {
					existing.reasoning = rec.reasoning
				}
				if rec.cost > existing.cost {
					existing.cost = rec.cost
				}
				continue
			}
			dedupIndex[dedupKey] = len(records)
		}
		records = append(records, rec)
	}

	c.cache.Set(filePath, records)
	return records
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func workspaceKeyFromPath(root, filePath string) string {
	rel, err := filepath.Rel(root, filePath)
	if err != nil {
		return filePath
	}
	parts := strings.Split(rel, string(filepath.Separator))
	if len(parts) > 0 && parts[0] != "" {
		return parts[0]
	}
	return filePath
}

func decodeWorkspaceLabel(dirName string) string {
	if strings.Contains(dirName, "%") {
		if decoded, err := urlDecode(dirName); err == nil {
			if strings.HasPrefix(decoded, "/") || (len(decoded) > 2 && decoded[1] == ':' && decoded[2] == '\\') {
				return decoded
			}
		}
	}
	return dirName
}

func urlDecode(s string) (string, error) {
	var result strings.Builder
	result.Grow(len(s))
	i := 0
	for i < len(s) {
		if s[i] == '%' && i+2 < len(s) {
			hi := unhex(s[i+1])
			lo := unhex(s[i+2])
			if hi >= 0 && lo >= 0 {
				result.WriteByte(byte(hi<<4 | lo))
				i += 3
				continue
			}
		}
		result.WriteByte(s[i])
		i++
	}
	return result.String(), nil
}

func unhex(c byte) int {
	switch {
	case '0' <= c && c <= '9':
		return int(c - '0')
	case 'a' <= c && c <= 'f':
		return int(c - 'a' + 10)
	case 'A' <= c && c <= 'F':
		return int(c - 'A' + 10)
	}
	return -1
}

func keepTimeEvent(timestamp string) bool {
	t, err := time.Parse(time.RFC3339, timestamp)
	if err != nil {
		return false
	}
	return t.UnixMilli() >= claudeEventCutoff
}

// ---------------------------------------------------------------------------
// Aggregation
// ---------------------------------------------------------------------------

type dailyAgg struct {
	date, model                                                string
	input, output, cacheRead, cacheWrite, reasoning           int64
	cost                                                       float64
}

func (a *dailyAgg) add(in, out, cr, cw, re int64, c float64) {
	a.input += in; a.output += out
	a.cacheRead += cr; a.cacheWrite += cw; a.reasoning += re
	a.cost += c
}

type sessionAgg struct {
	sessionID, projectPath, model                              string
	input, output, cacheRead, cacheWrite, reasoning           int64
	cost                                                       float64
}

func (a *sessionAgg) add(in, out, cr, cw, re int64, c float64) {
	a.input += in; a.output += out
	a.cacheRead += cr; a.cacheWrite += cw; a.reasoning += re
	a.cost += c
}

// ---------------------------------------------------------------------------
// JSON types
// ---------------------------------------------------------------------------

type claudeJSONLine struct {
	Type      string         `json:"type"`
	Model     string         `json:"model"`
	Timestamp string         `json:"timestamp"`
	CostUSD   float64        `json:"costUSD"`
	RequestID string         `json:"requestId"`
	Message   *claudeMessage `json:"message"`
}

type claudeMessage struct {
	ID    string       `json:"id"`
	Model string       `json:"model"`
	Usage *claudeUsage `json:"usage"`
}

type claudeUsage struct {
	InputTokens              json.Number `json:"input_tokens"`
	OutputTokens             json.Number `json:"output_tokens"`
	CacheReadInputTokens     json.Number `json:"cache_read_input_tokens"`
	CacheCreationInputTokens json.Number `json:"cache_creation_input_tokens"`
	ReasoningTokens          json.Number `json:"reasoning_tokens"`
	ThinkingTokens           json.Number `json:"thinking_tokens"`
}

type claudeRecord struct {
	timestamp                                                            string
	model                                                                string
	input, output, cacheRead, cacheWrite, reasoning                     int64
	cost                                                                 float64
}

func posIntFromJSON(n json.Number) int64 {
	if n == "" {
		return 0
	}
	v, err := n.Int64()
	if err != nil {
		if f, err := n.Float64(); err == nil && f > 0 {
			return int64(f)
		}
		return 0
	}
	if v < 0 {
		return 0
	}
	return v
}

func maxInt64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}
