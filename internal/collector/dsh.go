package collector

// DSHCollector 从 DeepSeek Harness（~/.dsh）采集会话 token 用量。
//
// 数据源布局（Harness home 可用 DSH_HOME 覆盖，默认 ~/.dsh）：
//
//	<home>/sessions/<project-key>/<session-id>/session.jsonl.zstd
//
// 文件格式：追加式拼接 zstd 帧（每批事件 = 一个独立帧，带校验和），帧内为 JSONL：
//
//	第 1 行 header: {"type":"session","version":0,"id":"session-xxx","cwd":"...","createdAt":...}
//	事件行:       {"type":"assistant/message","seq":N,"time":ms,"data":{"turn":..,"step":..,"usage":{...}}}
//
// 增量策略：ParseCache 的 LastOffset 记录"最后一个完整帧的结束偏移"；末尾不完整
// 帧（torn，写入中或崩溃残留）不推进游标，留待下次追加后继续。文件被截断（DSH
// 崩溃修复）时回退全量重读；EventKey（dsh::<sessionID>::<seq>）幂等，upsert 不重复。

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/klauspost/compress/zstd"

	"token-dashboard/internal/debuglog"
)

const dshCacheVersion = 1

// zstdMagic 是 Zstandard 帧魔数（小端 0xFD2FB528）。
var zstdMagic = []byte{0x28, 0xb5, 0x2f, 0xfd}

// DSHCollector 采集 DeepSeek Harness 会话日志中的 assistant/message 用量事件。
type DSHCollector struct {
	DeviceIdentity
	cache *ParseCache
	zdec  *zstd.Decoder // 共享解码器（DecodeAll 单帧解码）
	store ModelStateStore
}

// ModelStateStore 持久化 DSH 解析状态的存储接口（database.Manager 实现）。
// 只持久化"文件最后解析到的模型归属"，用于增量解析初始化模型，
// 避免为拿模型状态而全量解码历史帧。
type ModelStateStore interface {
	GetConfig(key string) (string, error)
	SetConfig(key, value string) error
}

// modelStateKey 返回文件对应的模型状态配置键。
func modelStateKey(fp string) string {
	return "dsh_last_model::" + fp
}

func NewDSHCollector() *DSHCollector {
	zdec, err := zstd.NewReader(nil)
	if err != nil {
		// 构造期失败视为致命初始化错误；解码器惰性创建兜底
		zdec = nil
	}
	return &DSHCollector{cache: NewParseCache(dshCacheVersion), zdec: zdec}
}

func (c *DSHCollector) ID() string    { return "dsh" }
func (c *DSHCollector) Source() string { return "DeepSeek Harness" }
func (c *DSHCollector) SetPersister(p PersistHandler, source string) { c.cache.SetPersister(p, source) }
func (c *DSHCollector) SetStore(store ModelStateStore) { c.store = store }
func (c *DSHCollector) ClearCache()    { c.cache.Clear() }
func (c *DSHCollector) PersistCache() error { return c.cache.PersistPending() }
func (c *DSHCollector) DiscardCache()  { c.cache.DiscardPending() }

// loadModelState 读取文件上次解析结束时的模型归属；无存储或无记录返回 false。
func (c *DSHCollector) loadModelState(fp string) (string, bool) {
	if c.store == nil {
		return "", false
	}
	v, err := c.store.GetConfig(modelStateKey(fp))
	if err != nil || v == "" {
		return "", false
	}
	return v, true
}

// saveModelState 持久化文件当前解析结束时的模型归属。
func (c *DSHCollector) saveModelState(fp, model string) error {
	if c.store == nil {
		return nil
	}
	if model == "" {
		return nil // 空模型无意义，不写（下次回退全量）
	}
	return c.store.SetConfig(modelStateKey(fp), model)
}

// dshHome 返回 Harness home（DSH_HOME 覆盖，默认 ~/.dsh）。
func dshHome() string {
	if env := os.Getenv("DSH_HOME"); env != "" {
		return env
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".dsh")
}

// dshRoots 返回会话日志根目录列表。
func dshRoots() []string {
	return []string{filepath.Join(dshHome(), "sessions")}
}

// collectZstdJSONLFiles 递归收集目录下所有 *.jsonl.zstd 会话日志。
func collectZstdJSONLFiles(dir string) []string {
	var results []string
	filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil // skip inaccessible
		}
		if !d.IsDir() && strings.HasSuffix(strings.ToLower(d.Name()), ".jsonl.zstd") {
			results = append(results, path)
		}
		return nil
	})
	return results
}

// dshSessionCWDs 从 session_projcache.json 提取 sessionID → 项目路径映射，
// 供增量解析（增量段不含 header）补充 projectPath。
func dshSessionCWDs() map[string]string {
	result := make(map[string]string)
	data, err := os.ReadFile(filepath.Join(dshHome(), "storages", "session_projcache.json"))
	if err != nil {
		return result
	}
	var cache struct {
		Tables struct {
			Sessions map[string]struct {
				Identity struct {
					CWD string `json:"cwd"`
				} `json:"identity"`
			} `json:"sessions"`
		} `json:"tables"`
	}
	if err := json.Unmarshal(data, &cache); err != nil {
		return result
	}
	for id, s := range cache.Tables.Sessions {
		if s.Identity.CWD != "" {
			result[id] = s.Identity.CWD
		}
	}
	return result
}

func (c *DSHCollector) Collect(ctx context.Context, pricing TokenCalc) (*CollectResult, error) {
	roots := dshRoots()
	log.Printf("[collector] DSH roots=%v", roots)

	discoverStart := time.Now()
	var allFiles []string
	for _, root := range roots {
		allFiles = append(allFiles, collectZstdJSONLFiles(root)...)
	}
	debuglog.Perf("DSH discover files=%d elapsed=%v", len(allFiles), time.Since(discoverStart))

	loadStart := time.Now()
	c.cache.LoadFromDB(c.Source(), allFiles)
	debuglog.Perf("DSH LoadFromDB files=%d elapsed=%v", len(allFiles), time.Since(loadStart))

	if c.cache.AllCached(allFiles) {
		log.Printf("[collector] DSH all files cached, skipping")
		return &CollectResult{Device: c.Device(), Source: c.Source(), Cached: true}, nil
	}

	cwdStart := time.Now()
	cwdBySession := dshSessionCWDs()
	debuglog.Perf("DSH projcache cwd=%d elapsed=%v", len(cwdBySession), time.Since(cwdStart))

	dailyMap := make(map[string]*dailyAgg)
	sessionMap := make(map[string]*sessionAgg)
	var events []EventRow
	totalRecords := 0

	parseStart := time.Now()
	for _, fp := range allFiles {
		if c.cache.FileUnchanged(fp) {
			continue
		}
		recs := c.parseSessionFile(fp, cwdBySession)
		totalRecords += len(recs)
		for _, rec := range recs {
			date := UTCDateFromTimestamp(rec.timestamp, "unknown")
			model := NormalizeModelForGrouping(rec.model)

			t := struct{ Input, Output, CacheRead, CacheWrite, Reasoning int64 }{
				rec.input, rec.output, rec.cacheRead, rec.cacheWrite, rec.reasoning,
			}
			cost := pricing.CalculateCost(model, t)

			events = append(events, EventRow{
				EventKey:   fmt.Sprintf("dsh::%s::%d", rec.sessionID, rec.seq),
				EventTime:  rec.timestamp, UsageDate: date, Model: model,
				SessionID:  rec.sessionID, ProjectPath: rec.project,
				InputTokens: rec.input, OutputTokens: rec.output,
				CacheReadTokens: rec.cacheRead, CacheWriteTokens: rec.cacheWrite,
				ReasoningTokens: rec.reasoning, CostUSD: cost,
			})

			dk := date + "::" + model
			if _, ok := dailyMap[dk]; !ok {
				dailyMap[dk] = &dailyAgg{date: date, model: model}
			}
			dailyMap[dk].add(rec.input, rec.output, rec.cacheRead, rec.cacheWrite, rec.reasoning, cost)

			// 会话粒度：DSH 一个会话一个文件，session_usage PK 是 session_id
			sk := rec.sessionID + "::" + model
			agg, ok := sessionMap[sk]
			if !ok {
				agg = &sessionAgg{sessionID: rec.sessionID, projectPath: rec.project, model: model}
				sessionMap[sk] = agg
			}
			agg.add(rec.input, rec.output, rec.cacheRead, rec.cacheWrite, rec.reasoning, cost)
			// RFC3339 UTC 同格式可字典序比较；事件按 seq 顺序遍历，最后覆盖 = 最新
			if rec.timestamp > agg.lastActivity {
				agg.lastActivity = rec.timestamp
			}
		}
	}

	debuglog.Perf("DSH parse files=%d records=%d elapsed=%v", len(allFiles), totalRecords, time.Since(parseStart))
	log.Printf("[collector] DSH done files=%d records=%d daily=%d sessions=%d events=%d",
		len(allFiles), totalRecords, len(dailyMap), len(sessionMap), len(events))

	return buildResult(c.Device(), "dsh", c.Source(), dailyMap, sessionMap, events), nil
}

// parseSessionFile 增量解析一个会话日志。
//
// 两条路径：
//  1. 增量（游标存在 + 持久化模型状态可用）：只读取并解码游标之后的字节，
//     模型归属从持久化状态初始化（上次解析结束时写入）。新帧内出现
//     request/context 会更新模型并持久化。性能 O(新增帧)，与会话历史无关。
//  2. 全量（首次 / 无模型状态 / 文件截断 / 增量边界异常）：读取并解码全部
//     帧推进模型状态，只产出游标之后的帧事件，结束时持久化模型状态。
//     事件入库幂等（EventKey=dsh::sessionID::seq），重复解析不产生重复数据。
//
// torn 尾帧不推进游标，留待下次追加后继续。
func (c *DSHCollector) parseSessionFile(fp string, cwdBySession map[string]string) []dshEvent {
	_, offset, state := c.cache.GetWithOffset(fp)
	if state == StateCached {
		return nil // 外层 FileUnchanged 已拦截未变文件；此处防御
	}

	// 增量路径：有持久化模型状态时只读新增字节，避免全量 IO + 解码
	if state == StateIncremental && offset > 0 {
		if model, ok := c.loadModelState(fp); ok {
			return c.parseIncremental(fp, offset, cwdBySession, model)
		}
		log.Printf("[collector] DSH no model state for %s, fallback full decode", filepath.Base(fp))
	}

	// 全量路径
	data, err := os.ReadFile(fp)
	if err != nil {
		return nil
	}
	return c.parseFull(fp, data, offset, cwdBySession)
}

// parseFull 全量路径：解码全部帧推进模型状态，只产出游标后的事件。
func (c *DSHCollector) parseFull(fp string, data []byte, offset int64, cwdBySession map[string]string) []dshEvent {
	t0 := time.Now()
	frames, _ := scanZstdFrames(data, 0)
	if len(frames) == 0 {
		return nil
	}

	// sessionID 从目录名取（布局固定 <project>/<session-id>/session.jsonl.zstd）
	sessionID := filepath.Base(filepath.Dir(fp))
	parser := &dshLogParser{sessionID: sessionID, cwd: cwdBySession[sessionID]}
	lastEnd := int64(frames[len(frames)-1][1])

	var newEvents []dshEvent
	for _, f := range frames {
		inNew := int64(f[1]) > offset
		out, err := c.zdec.DecodeAll(data[f[0]:f[1]], nil)
		if err != nil {
			log.Printf("[collector] DSH frame decode error file=%s off=%d err=%v", fp, f[0], err)
			continue
		}
		if inNew {
			newEvents = append(newEvents, parser.parseFrame(string(out), true)...)
		} else {
			parser.parseFrame(string(out), false)
		}
	}
	debuglog.Perf("DSH full file=%s events=%d frames=%d elapsed=%v", filepath.Base(sessionID), len(newEvents), len(frames), time.Since(t0))

	// 游标推进与模型状态持久化必须一致：状态写成功才推进游标，
	// 否则下次增量会重解本批帧（幂等）并重试保存。
	if len(newEvents) > 0 || lastEnd > offset {
		if err := c.saveModelState(fp, parser.currentModel); err != nil {
			log.Printf("[collector] DSH save model state error file=%s err=%v", filepath.Base(sessionID), err)
			return newEvents
		}
		c.cache.SetWithOffset(fp, newEvents, lastEnd)
	}
	return newEvents
}

// parseIncremental 增量路径：只读取并解码游标后的字节，模型从持久化状态初始化。
func (c *DSHCollector) parseIncremental(fp string, offset int64, cwdBySession map[string]string, model string) []dshEvent {
	t0 := time.Now()
	fi, err := os.Stat(fp)
	if err != nil || fi.Size() <= offset {
		return nil // 无新增字节
	}
	f, err := os.Open(fp)
	if err != nil {
		return nil
	}
	defer f.Close()
	if _, err := f.Seek(offset, io.SeekStart); err != nil {
		return nil
	}
	chunk, err := io.ReadAll(f)
	if err != nil {
		return nil
	}

	// 从游标处（应为帧边界）扫描新帧；chunk 开头不是合法帧头说明文件
	// 被截断重写（offset 失效），回退全量保证正确性
	frames, torn := scanZstdFrames(chunk, 0)
	if len(frames) == 0 {
		if torn == 0 {
			log.Printf("[collector] DSH incremental boundary mismatch file=%s, fallback full", filepath.Base(fp))
			data, err := os.ReadFile(fp)
			if err != nil {
				return nil
			}
			return c.parseFull(fp, data, offset, cwdBySession)
		}
		return nil // 只有 torn 尾帧，等下次
	}

	sessionID := filepath.Base(filepath.Dir(fp))
	parser := &dshLogParser{sessionID: sessionID, cwd: cwdBySession[sessionID], currentModel: model}
	var newEvents []dshEvent
	lastEnd := offset + int64(frames[len(frames)-1][1])
	for _, fr := range frames {
		out, err := c.zdec.DecodeAll(chunk[fr[0]:fr[1]], nil)
		if err != nil {
			log.Printf("[collector] DSH frame decode error file=%s off=%d err=%v", fp, offset+int64(fr[0]), err)
			continue
		}
		newEvents = append(newEvents, parser.parseFrame(string(out), true)...)
	}
	debuglog.Perf("DSH incr file=%s events=%d newBytes=%d elapsed=%v", filepath.Base(sessionID), len(newEvents), len(chunk), time.Since(t0))

	if len(newEvents) > 0 || lastEnd > offset {
		if err := c.saveModelState(fp, parser.currentModel); err != nil {
			log.Printf("[collector] DSH save model state error file=%s err=%v", filepath.Base(sessionID), err)
			return newEvents
		}
		c.cache.SetWithOffset(fp, newEvents, lastEnd)
	}
	return newEvents
}

// scanZstdFrames 扫描 data[start:] 中的完整 zstd 帧边界。
// 逐块解析 frame header / block header（与 DSH 的 scanZstdFrames 同构），
// 而非搜索魔数——压缩数据内部可能出现魔数字节序列。
// 返回完整帧的 [start,end) 区间；结构解析越界即视为 torn 尾帧并停止。
func scanZstdFrames(data []byte, start int) ([][2]int, int) {
	var frames [][2]int
	tornStart := -1
	offset := start

	for offset < len(data) {
		frameStart := offset
		if len(data)-offset < 4 || !bytes.Equal(data[offset:offset+4], zstdMagic) {
			tornStart = offset
			return frames, tornStart
		}
		offset += 4
		if offset == len(data) {
			tornStart = frameStart
			return frames, tornStart
		}

		descriptor := data[offset]
		offset++
		if descriptor&0x18 != 0 { // reserved bits
			tornStart = frameStart
			return frames, tornStart
		}
		singleSegment := descriptor&0x20 != 0
		checksum := descriptor&0x04 != 0
		dictFlag := descriptor & 0x03
		dictBytes := int(dictFlag)
		if dictFlag == 3 {
			dictBytes = 4
		}
		contentSizeFlag := descriptor >> 6
		var contentSizeBytes int
		if contentSizeFlag == 0 {
			if singleSegment {
				contentSizeBytes = 1
			} else {
				contentSizeBytes = 0
			}
		} else {
			contentSizeBytes = 1 << contentSizeFlag
		}
		remaining := dictBytes + contentSizeBytes
		if !singleSegment {
			remaining++ // window descriptor
		}
		if len(data)-offset < remaining {
			tornStart = frameStart
			return frames, tornStart
		}
		offset += remaining

		for {
			if len(data)-offset < 3 {
				tornStart = frameStart
				return frames, tornStart
			}
			blockHeader := uint32(data[offset]) | uint32(data[offset+1])<<8 | uint32(data[offset+2])<<16
			offset += 3
			last := blockHeader&1 != 0
			blockType := (blockHeader >> 1) & 3
			blockSize := int(blockHeader >> 3)
			if blockType == 3 { // reserved block type
				tornStart = frameStart
				return frames, tornStart
			}
			payload := blockSize
			if blockType == 1 { // RLE block: 1 byte payload
				payload = 1
			}
			if len(data)-offset < payload {
				tornStart = frameStart
				return frames, tornStart
			}
			offset += payload
			if last {
				break
			}
		}

		if checksum {
			if len(data)-offset < 4 {
				tornStart = frameStart
				return frames, tornStart
			}
			offset += 4
		}
		frames = append(frames, [2]int{frameStart, offset})
	}
	return frames, tornStart
}

// ---------------------------------------------------------------------------
// JSON 解析
// ---------------------------------------------------------------------------

type dshEvent struct {
	seq       int64
	timestamp string // RFC3339 UTC
	model     string
	project   string
	sessionID string
	input, output, cacheRead, cacheWrite, reasoning int64
}

type dshUsage struct {
	InputTokens     int64  `json:"inputTokens"`
	OutputTokens    int64  `json:"outputTokens"`
	CacheReadTokens *int64 `json:"cacheReadTokens"`
	CacheWriteTokens *int64 `json:"cacheWriteTokens"`
	ReasoningTokens *int64 `json:"reasoningTokens"`
}

func opt(v *int64) int64 {
	if v == nil {
		return 0
	}
	return *v
}

type dshLine struct {
	Type string          `json:"type"`
	Seq  int64           `json:"seq"`
	Time int64           `json:"time"`
	Data json.RawMessage `json:"data"`
	// 会话 header 的元数据在 JSON 顶层（事件行才用 data 包装）
	ID  string `json:"id"`
	CWD string `json:"cwd"`
}

// dshLogParser 维护跨帧的解析状态（header 元数据、当前模型）。
type dshLogParser struct {
	sessionID    string
	cwd          string
	currentModel string
}

// parseFrame 解析一帧的明文 JSONL，推进跨帧状态（header 元数据、当前模型）。
// emit 为 true 时返回本帧产出的用量事件（assistant/message），否则仅推进状态。
//
// 注意：帧通常只有几 KB~几十 KB（每帧 = DSH 一次追加批次），直接用 Split 逐行，
// 不要用 bufio.Scanner——每帧新建 Scanner 会重复分配 1MB 缓冲，大文件（上万帧）
// 造成 GB 级分配。
func (p *dshLogParser) parseFrame(plain string, emit bool) []dshEvent {
	var out []dshEvent
	for _, line := range strings.Split(plain, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var obj dshLine
		if err := json.Unmarshal([]byte(line), &obj); err != nil {
			continue
		}
		if e := p.handle(obj); e != nil && emit {
			out = append(out, *e)
		}
	}
	return out
}

// handle 处理一行事件，推进解析状态；assistant/message 携带有效 usage 时
// 返回产出的用量事件，其余情况返回 nil。
func (p *dshLogParser) handle(obj dshLine) *dshEvent {
	switch obj.Type {
	case "session": // 会话 header（仅首帧出现）；id/cwd 在 JSON 顶层
		if obj.ID != "" {
			p.sessionID = obj.ID
		}
		if obj.CWD != "" {
			p.cwd = obj.CWD
		}

	case "request/context":
		var rc struct {
			Model string `json:"model"`
		}
		if json.Unmarshal(obj.Data, &rc) == nil && rc.Model != "" {
			p.currentModel = rc.Model
		}

	case "request/header":
		var rh struct {
			Header struct {
				Config struct {
					Model string `json:"model"`
				} `json:"config"`
			} `json:"header"`
		}
		if json.Unmarshal(obj.Data, &rh) == nil && rh.Header.Config.Model != "" {
			p.currentModel = rh.Header.Config.Model
		}

	case "assistant/message":
		var msg struct {
			Turn  int       `json:"turn"`
			Step  int       `json:"step"`
			Usage *dshUsage `json:"usage"`
		}
		if json.Unmarshal(obj.Data, &msg) != nil {
			return nil
		}
		if msg.Usage == nil || (msg.Usage.InputTokens == 0 && msg.Usage.OutputTokens == 0) {
			return nil
		}
		u := msg.Usage
		return &dshEvent{
			seq:       obj.Seq,
			timestamp: time.UnixMilli(obj.Time).UTC().Format(time.RFC3339),
			model:     p.currentModel,
			sessionID: p.sessionID,
			project:   p.cwd,
			input:     u.InputTokens,
			output:    u.OutputTokens,
			cacheRead: opt(u.CacheReadTokens),
			cacheWrite: opt(u.CacheWriteTokens),
			reasoning: opt(u.ReasoningTokens),
		}
	}
	return nil
}
