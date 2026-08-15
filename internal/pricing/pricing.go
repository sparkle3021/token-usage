package pricing

import (
	"encoding/json"
	"fmt"
	"log"
	"math"
	"strings"
	"sync"
	"time"

	"token-dashboard/internal/model"
)

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

// Rates holds per-token prices for a model.
type Rates struct {
	Input      float64 `json:"input"`
	Output     float64 `json:"output"`
	CacheRead  float64 `json:"cacheRead"`
	CacheWrite float64 `json:"cacheWrite"`
}

// TokenBreakdown is the input to cost calculation.
type TokenBreakdown struct {
	Input     int64
	Output    int64
	CacheRead int64
	CacheWrite int64
	Reasoning int64
}

// Engine loads and queries model pricing data.
// 数据为 model_pricing 表的纯内存快照（LoadRows 整体加载 / ApplyRow 单行更新），
// 不直接依赖 database 包，保持 domain 层解耦。
type Engine struct {
	data  map[string]*Rates // 原始模型 key → 费率

	mu     sync.Mutex
	cache  map[string]*Rates
}

// litellmEntry LiteLLM JSON 单条原始字段（仅保留实际参与计算的 4 项基础费率）。
type litellmEntry struct {
	InputCostPerToken           float64 `json:"input_cost_per_token"`
	OutputCostPerToken          float64 `json:"output_cost_per_token"`
	CacheReadInputTokenCost     float64 `json:"cache_read_input_token_cost"`
	CacheCreationInputTokenCost float64 `json:"cache_creation_input_token_cost"`
}

// pricingFile wraps the LiteLLM/OpenRouter JSON format.
type pricingFile struct {
	FetchedAt int64                  `json:"fetchedAt"`
	Data      map[string]interface{} `json:"data"`
}

// excluded prefixes (subscription models with $0 per-token pricing).
var excludedPrefixes = []string{"github_copilot/"}

// ---------------------------------------------------------------------------
// Construction
// ---------------------------------------------------------------------------

// NewEngine 创建空的价格引擎，数据由 LoadRows 从 model_pricing 表加载。
func NewEngine() *Engine {
	return &Engine{
		data:  make(map[string]*Rates),
		cache: make(map[string]*Rates),
	}
}

// LoadRows 整体替换内存价格数据（启动加载与拉取更新后调用），并清空解析缓存。
func (e *Engine) LoadRows(rows []model.ModelPricing) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.data = make(map[string]*Rates, len(rows))
	for _, r := range rows {
		e.data[r.ModelKey] = &Rates{Input: r.InputRate, Output: r.OutputRate, CacheRead: r.CacheReadRate, CacheWrite: r.CacheWriteRate}
	}
	e.cache = make(map[string]*Rates)
	log.Printf("[pricing] LoadRows entries=%d", len(e.data))
}

// ApplyRow 单行更新内存价格（用户改价后调用），并清空缓存保证查找一致性。
func (e *Engine) ApplyRow(row model.ModelPricing) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.data == nil {
		e.data = make(map[string]*Rates)
	}
	e.data[row.ModelKey] = &Rates{Input: row.InputRate, Output: row.OutputRate, CacheRead: row.CacheReadRate, CacheWrite: row.CacheWriteRate}
	e.cache = make(map[string]*Rates)
}

// DeleteRow 删除内存价格行（用户删除后调用）。
func (e *Engine) DeleteRow(modelKey string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	delete(e.data, modelKey)
	e.cache = make(map[string]*Rates)
}

// ParseLiteLLMRaw 解析 LiteLLM 原始 JSON（支持 {fetchedAt,data} 包装或平铺格式），
// 输出 model_pricing 表行。fetchedAt 优先取包装内时间戳，否则当前时间。
func ParseLiteLLMRaw(raw []byte) ([]model.ModelPricing, error) {
	var pf pricingFile
	if err := json.Unmarshal(raw, &pf); err != nil {
		return nil, fmt.Errorf("decode pricing: %w", err)
	}

	var data map[string]interface{}
	if pf.Data != nil {
		data = pf.Data
	} else {
		// 平铺格式（无包装）
		var flat map[string]interface{}
		if err := json.Unmarshal(raw, &flat); err != nil {
			return nil, fmt.Errorf("decode flat pricing: %w", err)
		}
		data = flat
	}

	fetchedAt := time.Now().UTC().Format(time.RFC3339)
	if pf.FetchedAt > 0 {
		fetchedAt = time.UnixMilli(pf.FetchedAt).UTC().Format(time.RFC3339)
	}

	rows := make([]model.ModelPricing, 0, len(data))
	for key, val := range data {
		if isExcluded(key) {
			continue
		}
		entry := toLiteLLMEntry(val)
		if entry == nil {
			continue
		}
		rows = append(rows, model.ModelPricing{
			ModelKey:       key,
			InputRate:      entry.InputCostPerToken,
			OutputRate:     entry.OutputCostPerToken,
			CacheReadRate:  entry.CacheReadInputTokenCost,
			CacheWriteRate: entry.CacheCreationInputTokenCost,
			FetchedAt:      fetchedAt,
		})
	}
	return rows, nil
}



// ---------------------------------------------------------------------------
// Cost calculation
// ---------------------------------------------------------------------------

// CalculateCost returns the USD cost for a model + token breakdown.
// 使用 model_pricing 表快照中的基础费率（input/output/cacheRead/cacheWrite）计算。
func (e *Engine) CalculateCost(model string, tokens TokenBreakdown) float64 {
	p := e.lookupPricing(model)
	if p == nil {
		return 0
	}

	input := float64(maxInt64(tokens.Input, 0))
	output := float64(maxInt64(tokens.Output, 0))
	cacheRead := float64(maxInt64(tokens.CacheRead, 0))
	cacheWrite := float64(maxInt64(tokens.CacheWrite, 0))
	reasoning := float64(maxInt64(tokens.Reasoning, 0))

	return input*validPrice(p.Input) +
		(output+reasoning)*validPrice(p.Output) +
		cacheRead*validPrice(p.CacheRead) +
		cacheWrite*validPrice(p.CacheWrite)
}

// ---------------------------------------------------------------------------
// Pricing lookup
// ---------------------------------------------------------------------------

// LookupRates returns the rate card for a model, caching results internally.
// Safe for concurrent use. Returns nil if no pricing data is found.
func (e *Engine) LookupRates(modelID string) *Rates {
	return e.lookupPricing(modelID)
}

func (e *Engine) lookupPricing(modelID string) *Rates {
	id := strings.TrimSpace(strings.ToLower(modelID))
	if id == "" {
		return nil
	}

	// Fast cache-check path with minimal lock hold time
	e.mu.Lock()
	cached, ok := e.cache[id]
	e.mu.Unlock()
	if ok {
		return cached
	}

	// Datasets are read-only after init — no lock needed for lookup
	hit := e.lookupUncached(id)

	e.mu.Lock()
	e.cache[id] = hit
	e.mu.Unlock()

	if hit == nil {
		log.Printf("[pricing] lookup model=%s via=nil (no pricing found)", id)
	} else {
		log.Printf("[pricing] lookup model=%s inputRate=%g outputRate=%g", id, hit.Input, hit.Output)
	}
	return hit
}

func (e *Engine) lookupUncached(id string) *Rates {
	var hit *Rates
	var candidates []string

	if len(e.data) > 0 {
		candidates = modelCandidates(id)
	}

	// 精确/前缀候选链
	for _, c := range candidates {
		if r := findInDataset(c, e.data); r != nil {
			hit = r
			break
		}
	}

	// Fuzzy fallback
	if hit == nil && len(e.data) > 0 {
		if r := findFuzzy(id, e.data); r != nil {
			hit = r
		}
	}

	return hit
}

func findInDataset(id string, data map[string]*Rates) *Rates {
	if data == nil {
		return nil
	}
	if r, ok := data[id]; ok {
		return r
	}
	if r, ok := data["openai/"+id]; ok {
		return r
	}
	for key, r := range data {
		if strings.EqualFold(key, id) && r != nil {
			return r
		}
	}
	return nil
}

func findFuzzy(id string, data map[string]*Rates) *Rates {
	if data == nil || len(id) < 5 {
		return nil
	}
	normalized := normalizeComparable(id)

	var bestKey string
	var bestEntry *Rates
	for key, entry := range data {
		if entry == nil || isExcluded(key) {
			continue
		}
		keyBare := bareModelID(strings.ToLower(key))
		keyNorm := normalizeComparable(keyBare)
		if keyNorm == normalized || strings.HasPrefix(keyNorm, normalized+"-") || strings.HasPrefix(normalized, keyNorm+"-") {
			if bestKey == "" || fuzzyScore(key, id) > fuzzyScore(bestKey, id) {
				bestKey = key
				bestEntry = entry
			}
		}
	}
	return bestEntry
}

// ---------------------------------------------------------------------------
// Candidates
// ---------------------------------------------------------------------------

func modelCandidates(id string) []string {
	candidates := make([]string, 0, 16)
	base := bareModelID(id)

	add := func(s string) {
		if s != "" {
			candidates = append(candidates, s)
		}
	}

	add(id)
	add(base)
	add(normalizeVersionSep(id))
	add(normalizeVersionSep(base))

	// Strip prefixes and suffixes
	for _, s := range stripSuffixes(id) {
		add(s)
	}
	for _, s := range stripSuffixes(base) {
		add(s)
	}
	for _, s := range stripPrefixes(id) {
		add(s)
	}
	for _, s := range stripPrefixes(base) {
		add(s)
	}

	// Provider-prefixed variants
	provider := inferProvider(id)
	datasetPrefix := providerToDatasetPrefix(provider)
	for _, c := range candidates {
		if datasetPrefix != "" {
			add(datasetPrefix + "/" + c)
		}
	}

	return unique(candidates)
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func toLiteLLMEntry(v interface{}) *litellmEntry {
	b, err := json.Marshal(v)
	if err != nil {
		return nil
	}
	var entry litellmEntry
	if err := json.Unmarshal(b, &entry); err != nil {
		return nil
	}
	if entry.InputCostPerToken == 0 && entry.OutputCostPerToken == 0 {
		return nil
	}
	return &entry
}

func isExcluded(key string) bool {
	lower := strings.ToLower(key)
	for _, p := range excludedPrefixes {
		if strings.HasPrefix(lower, p) {
			return true
		}
	}
	return false
}

func bareModelID(id string) string {
	parts := strings.Split(strings.ToLower(strings.TrimSpace(id)), "/")
	return parts[len(parts)-1]
}

func normalizeComparable(id string) string {
	normalized := normalizeVersionSep(strings.ToLower(id))
	if normalized != "" {
		return normalized
	}
	return strings.ToLower(id)
}

func normalizeVersionSep(id string) string {
	chars := []rune(id)
	changed := false
	result := make([]rune, len(chars))
	copy(result, chars)

	for i := 1; i < len(chars)-1; i++ {
		if result[i] != '-' {
			continue
		}
		if !isDigit(result[i-1]) || !isDigit(result[i+1]) {
			continue
		}
		multiBefore := i >= 2 && isDigit(result[i-2])
		multiAfter := i+2 < len(chars) && isDigit(result[i+2])
		if multiBefore || multiAfter {
			continue
		}
		result[i] = '.'
		changed = true
	}
	if !changed {
		return ""
	}
	return string(result)
}

func stripSuffixes(id string) []string {
	parts := strings.Split(id, "-")
	if len(parts) <= 1 {
		return nil
	}
	maxStrip := len(parts) - 1
	if maxStrip > 4 {
		maxStrip = 4
	}
	var results []string
	for strip := 1; strip <= maxStrip; strip++ {
		candidate := strings.Join(parts[:len(parts)-strip], "-")
		if len(candidate) >= 2 {
			results = append(results, candidate)
		}
	}
	return results
}

func stripPrefixes(id string) []string {
	parts := strings.Split(id, "-")
	if len(parts) <= 1 {
		return nil
	}
	maxSkip := len(parts) - 1
	if maxSkip > 2 {
		maxSkip = 2
	}
	var results []string
	for skip := 1; skip <= maxSkip; skip++ {
		candidate := strings.Join(parts[skip:], "-")
		if len(candidate) >= 2 {
			results = append(results, candidate)
			results = append(results, stripSuffixes(candidate)...)
		}
	}
	return results
}

func fuzzyScore(key, id string) int {
	lower := strings.ToLower(key)
	provider := strings.Split(id, "-")[0]
	score := len(key)
	if provider != "" && strings.HasPrefix(lower, provider+"/") {
		score += 10_000
	}
	if strings.HasPrefix(lower, "openrouter/") {
		score -= 5_000
	}
	if strings.HasPrefix(lower, "vertex_ai/") || strings.HasPrefix(lower, "bedrock/") {
		score -= 2_000
	}
	if strings.Contains(lower, "/") {
		score += 100
	}
	return score
}

func inferProvider(model string) string {
	lower := strings.ToLower(model)
	switch {
	case strings.Contains(lower, "claude"), strings.Contains(lower, "anthropic"):
		return "anthropic"
	case strings.Contains(lower, "gpt"), strings.Contains(lower, "openai"):
		return "openai"
	case strings.Contains(lower, "gemini"), strings.Contains(lower, "google"):
		return "google"
	case strings.Contains(lower, "grok"):
		return "xai"
	case strings.Contains(lower, "deepseek"):
		return "deepseek"
	case strings.Contains(lower, "mistral"), strings.Contains(lower, "mixtral"):
		return "mistral"
	case strings.Contains(lower, "llama"):
		return "meta_llama"
	case strings.Contains(lower, "qwen"):
		return "qwen"
	default:
		return ""
	}
}

func providerToDatasetPrefix(provider string) string {
	prefixes := map[string]string{
		"azure_ai":     "azure_ai",
		"fireworks_ai": "fireworks_ai",
		"meta_llama":   "meta-llama",
		"mistralai":    "mistralai",
		"moonshotai":  "moonshotai",
		"openai":      "openai",
		"anthropic":   "anthropic",
		"google":      "google",
		"deepseek":    "deepseek",
		"qwen":        "qwen",
		"xai":         "x-ai",
		"zai":         "zai",
	}
	if p, ok := prefixes[provider]; ok {
		return p
	}
	return provider
}

func validPrice(v float64) float64 {
	if math.IsNaN(v) || math.IsInf(v, 0) || v < 0 {
		return 0
	}
	return v
}

func isDigit(r rune) bool { return r >= '0' && r <= '9' }

func unique(items []string) []string {
	seen := make(map[string]struct{}, len(items))
	result := make([]string, 0, len(items))
	for _, item := range items {
		if _, ok := seen[item]; !ok {
			seen[item] = struct{}{}
			result = append(result, item)
		}
	}
	return result
}

func maxInt64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}
