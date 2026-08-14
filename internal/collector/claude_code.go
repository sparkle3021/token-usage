package collector

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log"

	"token-dashboard/internal/debuglog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

const (
	claudeClientKey    = "claude"
	claudeSourceLabel  = "Claude Code"
	claudeCacheVersion = 1
)

type ClaudeCodeCollector struct {
	DeviceIdentity
	cache *ParseCache
}

func NewClaudeCodeCollector() *ClaudeCodeCollector {
	return &ClaudeCodeCollector{cache: NewParseCache(claudeCacheVersion)}
}

func (c *ClaudeCodeCollector) ID() string    { return claudeClientKey }
func (c *ClaudeCodeCollector) Source() string { return claudeSourceLabel }
func (c *ClaudeCodeCollector) SetPersister(p PersistHandler, source string) { c.cache.SetPersister(p, source) }
func (c *ClaudeCodeCollector) ClearCache() { c.cache.Clear() }
func (c *ClaudeCodeCollector) PersistCache() error { return c.cache.PersistPending() }
func (c *ClaudeCodeCollector) DiscardCache() { c.cache.DiscardPending() }

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
	roots := getClaudeRoots()
	log.Printf("[collector] ClaudeCode roots=%v", roots)

	// Collect all file paths for cache check（每目录只 walk 一次，结果按目录分组
	// 供 scanAndParse 复用，避免重复遍历）
	type dirFiles struct {
		dir   string
		files []string
	}
	var groups []dirFiles
	var allFiles []string
	for _, root := range roots {
		for _, sub := range []string{"projects", "transcripts"} {
			d := filepath.Join(root, sub)
			if info, err := os.Stat(d); err == nil && info.IsDir() {
				fs := CollectJSONLFiles(d)
				groups = append(groups, dirFiles{dir: d, files: fs})
				allFiles = append(allFiles, fs...)
			}
		}
	}

	// 目录级 mtime 预过滤：按 jsonl 的**直接父目录**（workspace 会话目录）分组。
	// 该目录 mtime 只在其中 jsonl 增删改时更新，因此"目录未变 ⇒ 组内文件全
	// 未变"成立（Windows 目录 mtime 不反映更深层变化，不能按 projects 根预过滤）。
	// 目录指纹直接以目录真实路径为 ParseCache 键（目录路径与文件路径不冲突，
	// FileUnchanged/SetWithOffset 内部对键路径 stat 目录得到 mtime:size）；
	// 文件级指纹仍是最终判定。
	parentMap := make(map[string][]string) // jsonl 父目录 -> 文件列表
	parentRoot := make(map[string]string)  // jsonl 父目录 -> projects/transcripts 根
	for _, g := range groups {
		for _, f := range g.files {
			d := filepath.Dir(f)
			parentMap[d] = append(parentMap[d], f)
			parentRoot[d] = g.dir
		}
	}
	dirPaths := make([]string, 0, len(parentMap))
	for d := range parentMap {
		dirPaths = append(dirPaths, d)
	}

	// Pre-load cache from DB（文件 + 目录指纹批量加载）
	loadStart := time.Now()
	allPaths := make([]string, 0, len(allFiles)+len(dirPaths))
	allPaths = append(allPaths, allFiles...)
	allPaths = append(allPaths, dirPaths...)
	c.cache.LoadFromDB(c.Source(), allPaths)
	debuglog.Perf("ClaudeCode LoadFromDB files=%d dirs=%d elapsed=%v", len(allFiles), len(dirPaths), time.Since(loadStart))

	// 目录预过滤：只对"父目录 mtime 变化"的组做文件级检查。
	// Windows 目录 stat 比文件 stat 慢（约 5-10 倍），并行执行摊薄成本。
	checkStart := time.Now()
	changedByRoot := make(map[string][]string) // root -> 需要解析的文件
	var changedDirs []string                    // 目录 mtime 变化的父目录（需推进指纹）
	skippedByDir := 0
	var mu sync.Mutex
	var wg sync.WaitGroup
	sem := make(chan struct{}, 8)
	for d, files := range parentMap {
		wg.Add(1)
		sem <- struct{}{}
		go func(d string, files []string) {
			defer wg.Done()
			defer func() { <-sem }()
			if c.cache.FileUnchanged(d) {
				mu.Lock()
				skippedByDir += len(files)
				mu.Unlock()
				return // 父目录未变，整组跳过（零文件 stat）
			}
			mu.Lock()
			changedDirs = append(changedDirs, d)
			mu.Unlock()
			root := parentRoot[d]
			for _, f := range files {
				if !c.cache.FileUnchanged(f) {
					mu.Lock()
					changedByRoot[root] = append(changedByRoot[root], f)
					mu.Unlock()
				}
			}
		}(d, files)
	}
	wg.Wait()

	// 推进"变化目录"的指纹（未变目录指纹无需重写），使下次预过滤命中；
	// Cached 路径在此自行落盘（引擎对 Cached 结果不会调用 PersistCache），
	// 非 Cached 路径由引擎写库成功后统一落盘
	for _, d := range changedDirs {
		c.cache.SetWithOffset(d, nil, 0)
	}
	debuglog.Perf("ClaudeCode dir-prefilter parents=%d skippedFiles=%d changedFiles=%d elapsed=%v",
		len(parentMap), skippedByDir, len(changedByRoot), time.Since(checkStart))

	if len(changedByRoot) == 0 {
		if err := c.cache.PersistPending(); err != nil {
			log.Printf("[collector] ClaudeCode persist dir fingerprints error: %v", err)
		}
		debuglog.Perf("ClaudeCode AllCached hit files=%d elapsed=%v", len(allFiles), time.Since(checkStart))
		log.Printf("[collector] ClaudeCode all files cached, skipping")
		return &CollectResult{Device: c.Device(), Source: claudeSourceLabel, Cached: true}, nil
	}

	dailyMap := make(map[string]*dailyAgg)
	sessionMap := make(map[string]*sessionAgg)
	var events []EventRow

	for _, g := range groups {
		if files := changedByRoot[g.dir]; len(files) > 0 {
			log.Printf("[collector] ClaudeCode scanning dir=%s files=%d", g.dir, len(files))
			c.scanAndParse(g.dir, files, dailyMap, sessionMap, &events, pricing)
		}
	}

	result := &CollectResult{Device: c.Device(), Source: claudeSourceLabel}
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
			SessionID: agg.sessionID, LastActivity: time.Now().Format(time.RFC3339),
			ProjectPath: agg.projectPath, Model: agg.model,
			InputTokens: agg.input, OutputTokens: agg.output,
			CacheReadTokens: agg.cacheRead, CacheWriteTokens: agg.cacheWrite,
			ReasoningTokens: agg.reasoning, CostUSD: agg.cost,
		})
	}
	result.Events = events
	return result, nil
}

func (c *ClaudeCodeCollector) scanAndParse(dir string, files []string,
	dailyMap map[string]*dailyAgg, sessionMap map[string]*sessionAgg,
	events *[]EventRow, pricing TokenCalc,
) {
	recordCount := 0
	parseStart := time.Now()
	skippedFiles := 0
	for _, filePath := range files {
		if c.cache.FileUnchanged(filePath) {
			skippedFiles++
			continue
		}
		records := c.parseFile(filePath)
		recordCount += len(records)
		for _, rec := range records {
			date := UTCDateFromTimestamp(rec.timestamp, "unknown")
			if date == "unknown" {
				continue
			}
			eventTime := rec.timestamp
			if u, ok := ToUTCRFC3339(rec.timestamp); ok {
				eventTime = u
			}
			model := NormalizeModelForGrouping(rec.model)
			t := struct{ Input, Output, CacheRead, CacheWrite, Reasoning int64 }{
				rec.input, rec.output, rec.cacheRead, rec.cacheWrite, rec.reasoning,
			}
			cost := pricing.CalculateCost(model, t)

			workspaceKey := workspaceKeyFromPath(dir, filePath)
			workspaceLabel := decodeWorkspaceLabel(workspaceKey)

			*events = append(*events, EventRow{
				EventKey:   fmt.Sprintf("%s:%s:%s:%d", filePath, rec.timestamp, model, rec.input+rec.output),
				EventTime: eventTime, UsageDate: date, Model: model,
				SessionID: strings.TrimSuffix(filepath.Base(filePath), ".jsonl"), ProjectPath: workspaceLabel,
				InputTokens: rec.input, OutputTokens: rec.output,
				CacheReadTokens: rec.cacheRead, CacheWriteTokens: rec.cacheWrite,
				ReasoningTokens: rec.reasoning, CostUSD: cost,
			})

			dk := date + "::" + model
			if _, ok := dailyMap[dk]; !ok {
				dailyMap[dk] = &dailyAgg{date: date, model: model}
			}
			dailyMap[dk].add(rec.input, rec.output, rec.cacheRead, rec.cacheWrite, rec.reasoning, cost)

			sk := workspaceKey + "::" + model
			if _, ok := sessionMap[sk]; !ok {
				sessionMap[sk] = &sessionAgg{
					sessionID: strings.TrimSuffix(filepath.Base(filePath), ".jsonl"), projectPath: workspaceLabel, model: model,
				}
			}
			sessionMap[sk].add(rec.input, rec.output, rec.cacheRead, rec.cacheWrite, rec.reasoning, cost)
		}
	}
		debuglog.Perf("ClaudeCode scanAndParse dir=%s files=%d skipped=%d records=%d elapsed=%v", dir, len(files), skippedFiles, recordCount, time.Since(parseStart))
}

func (c *ClaudeCodeCollector) parseFile(filePath string) []claudeRecord {
	records, offset, state := c.cache.GetWithOffset(filePath)
	if state == StateCached {
		return records.([]claudeRecord)
	}

	f, err := os.Open(filePath)
	if err != nil {
		return nil
	}
	defer f.Close()

	fi, _ := f.Stat()
	fileSize := fi.Size()

	if state == StateIncremental && offset > 0 {
		f.Seek(offset, 0)
	}

	var parsed []claudeRecord
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
		rec.cacheRead = posIntFromJSON(usage.CacheReadInputTokens)
		rec.cacheWrite = posIntFromJSON(usage.CacheCreationInputTokens)
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
				existing := &parsed[idx]
				if rec.input > existing.input {
					existing.input = rec.input
				}
				if rec.output > existing.output {
					existing.output = rec.output
				}
				if rec.cacheRead > existing.cacheRead {
					existing.cacheRead = rec.cacheRead
				}
				if rec.cacheWrite > existing.cacheWrite {
					existing.cacheWrite = rec.cacheWrite
				}
				if rec.reasoning > existing.reasoning {
					existing.reasoning = rec.reasoning
				}
				if rec.cost > existing.cost {
					existing.cost = rec.cost
				}
				continue
			}
			dedupIndex[dedupKey] = len(parsed)
		}
		parsed = append(parsed, rec)
	}

	c.cache.SetWithOffset(filePath, parsed, fileSize)
	return parsed
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
	lastActivity                                               string
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
