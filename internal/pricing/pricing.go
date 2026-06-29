package pricing

import (
	"encoding/json"
	"fmt"
	"log"
	"math"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

// Rates holds per-token prices for a model.
type Rates struct {
	Input              float64 `json:"input"`
	InputAbove128k     float64 `json:"inputAbove128k,omitempty"`
	InputAbove200k     float64 `json:"inputAbove200k,omitempty"`
	InputAbove256k     float64 `json:"inputAbove256k,omitempty"`
	InputAbove272k     float64 `json:"inputAbove272k,omitempty"`
	Output             float64 `json:"output"`
	OutputAbove128k    float64 `json:"outputAbove128k,omitempty"`
	OutputAbove200k    float64 `json:"outputAbove200k,omitempty"`
	OutputAbove256k    float64 `json:"outputAbove256k,omitempty"`
	OutputAbove272k    float64 `json:"outputAbove272k,omitempty"`
	CacheRead          float64 `json:"cacheRead"`
	CacheReadAbove200k float64 `json:"cacheReadAbove200k,omitempty"`
	CacheReadAbove272k float64 `json:"cacheReadAbove272k,omitempty"`
	CacheWrite         float64 `json:"cacheWrite"`
	CacheWriteAbove200k float64 `json:"cacheWriteAbove200k,omitempty"`
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
type Engine struct {
	litellm   map[string]*litellmEntry
	openrouter map[string]*litellmEntry

	mu     sync.Mutex
	cache  map[string]*Rates
}

type litellmEntry struct {
	InputCostPerToken                float64 `json:"input_cost_per_token"`
	OutputCostPerToken               float64 `json:"output_cost_per_token"`
	CacheReadInputTokenCost          float64 `json:"cache_read_input_token_cost"`
	CacheCreationInputTokenCost      float64 `json:"cache_creation_input_token_cost"`
	InputCostPerTokenAbove128kTokens  float64 `json:"input_cost_per_token_above_128k_tokens"`
	InputCostPerTokenAbove200kTokens  float64 `json:"input_cost_per_token_above_200k_tokens"`
	InputCostPerTokenAbove256kTokens  float64 `json:"input_cost_per_token_above_256k_tokens"`
	InputCostPerTokenAbove272kTokens  float64 `json:"input_cost_per_token_above_272k_tokens"`
	OutputCostPerTokenAbove128kTokens float64 `json:"output_cost_per_token_above_128k_tokens"`
	OutputCostPerTokenAbove200kTokens float64 `json:"output_cost_per_token_above_200k_tokens"`
	OutputCostPerTokenAbove256kTokens float64 `json:"output_cost_per_token_above_256k_tokens"`
	OutputCostPerTokenAbove272kTokens float64 `json:"output_cost_per_token_above_272k_tokens"`
	CacheReadAbove200kTokens          float64 `json:"cache_read_input_token_cost_above_200k_tokens"`
	CacheReadAbove272kTokens          float64 `json:"cache_read_input_token_cost_above_272k_tokens"`
	CacheCreationAbove200kTokens      float64 `json:"cache_creation_input_token_cost_above_200k_tokens"`
}

// pricingFile wraps the LiteLLM/OpenRouter JSON format.
type pricingFile struct {
	FetchedAt int64                  `json:"fetchedAt"`
	Data      map[string]interface{} `json:"data"`
}

// ---------------------------------------------------------------------------
// Hardcoded overrides (not yet in upstream LiteLLM)
// ---------------------------------------------------------------------------

var cursorOverrides = map[string]Rates{
	"gpt-5.3":             {Input: 1.75e-6, Output: 1.4e-5, CacheRead: 1.75e-7},
	"gpt-5.3-codex":       {Input: 1.75e-6, Output: 1.4e-5, CacheRead: 1.75e-7},
	"gpt-5.3-codex-spark": {Input: 1.75e-6, Output: 1.4e-5, CacheRead: 1.75e-7},
	"composer 1":          {Input: 1.25e-6, Output: 1.0e-5, CacheRead: 1.25e-7},
	"composer-1":          {Input: 1.25e-6, Output: 1.0e-5, CacheRead: 1.25e-7},
	"composer 1.5":        {Input: 3.5e-6, Output: 1.75e-5, CacheRead: 3.5e-7},
	"composer-1.5":        {Input: 3.5e-6, Output: 1.75e-5, CacheRead: 3.5e-7},
	"composer 2":          {Input: 5e-7, Output: 2.5e-6, CacheRead: 2e-7},
	"composer-2":          {Input: 5e-7, Output: 2.5e-6, CacheRead: 2e-7},
	"composer 2 fast":     {Input: 1.5e-6, Output: 7.5e-6, CacheRead: 3.5e-7},
	"composer-2-fast":     {Input: 1.5e-6, Output: 7.5e-6, CacheRead: 3.5e-7},
}

var deepseekOverrides = map[string]Rates{
	"deepseek-chat":     {Input: 1.4e-7, Output: 2.8e-7, CacheRead: 2.8e-9},
	"deepseek-reasoner": {Input: 1.4e-7, Output: 2.8e-7, CacheRead: 2.8e-9},
	"deepseek-v4-flash": {Input: 1.4e-7, Output: 2.8e-7, CacheRead: 2.8e-9},
	"deepseek-v4-pro":   {Input: 4.35e-7, Output: 8.7e-7, CacheRead: 3.625e-9},
}

// excluded prefixes (subscription models with $0 per-token pricing).
var excludedPrefixes = []string{"github_copilot/"}

// ---------------------------------------------------------------------------
// Construction
// ---------------------------------------------------------------------------

func NewEngine(dataDir string) (*Engine, error) {
	e := &Engine{
		cache: make(map[string]*Rates),
	}

	litellmPath := filepath.Join(dataDir, "pricing-litellm.json")
	if err := e.loadLiteLLM(litellmPath); err != nil {
		fmt.Fprintf(os.Stderr, "[pricing] warning: %v\n", err)
		log.Printf("[pricing] NewEngine loadLiteLLM error=%v", err)
	} else {
		log.Printf("[pricing] NewEngine litellm loaded entries=%d", len(e.litellm))
	}

	openrouterPath := filepath.Join(dataDir, "pricing-openrouter.json")
	if err := e.loadOpenRouter(openrouterPath); err != nil {
		fmt.Fprintf(os.Stderr, "[pricing] warning: %v\n", err)
		log.Printf("[pricing] NewEngine loadOpenRouter error=%v", err)
	} else {
		log.Printf("[pricing] NewEngine openrouter loaded entries=%d", len(e.openrouter))
	}

	return e, nil
}

func (e *Engine) loadLiteLLM(path string) error {
	data, err := readPricingFile(path)
	if err != nil {
		return fmt.Errorf("load litellm: %w", err)
	}
	e.litellm = data
	return nil
}

func (e *Engine) loadOpenRouter(path string) error {
	data, err := readPricingFile(path)
	if err != nil {
		return fmt.Errorf("load openrouter: %w", err)
	}
	e.openrouter = data
	return nil
}

func readPricingFile(path string) (map[string]*litellmEntry, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var pf pricingFile
	if err := json.NewDecoder(f).Decode(&pf); err != nil {
		return nil, fmt.Errorf("decode: %w", err)
	}
	if pf.Data == nil {
		// Try flat format (no wrapper)
		f.Seek(0, 0)
		var flat map[string]*litellmEntry
		if err := json.NewDecoder(f).Decode(&flat); err != nil {
			return nil, fmt.Errorf("decode flat: %w", err)
		}
		return flat, nil
	}

	result := make(map[string]*litellmEntry, len(pf.Data))
	for key, val := range pf.Data {
		if isExcluded(key) {
			continue
		}
		entry := toLiteLLMEntry(val)
		if entry != nil {
			result[key] = entry
		}
	}
	return result, nil
}

// ---------------------------------------------------------------------------
// Cost calculation
// ---------------------------------------------------------------------------

// CalculateCost returns the USD cost for a model + token breakdown.
func (e *Engine) CalculateCost(model string, tokens TokenBreakdown, options ...CalcOption) float64 {
	opts := calcOptions{}
	for _, o := range options {
		o(&opts)
	}

	p := e.lookupPricing(model)
	if p == nil {
		return 0
	}

	input := float64(maxInt64(tokens.Input, 0))
	output := float64(maxInt64(tokens.Output, 0))
	cacheRead := float64(maxInt64(tokens.CacheRead, 0))
	cacheWrite := float64(maxInt64(tokens.CacheWrite, 0))
	reasoning := float64(maxInt64(tokens.Reasoning, 0))

	if !opts.tiered {
		return input*validPrice(p.Input) +
			(output+reasoning)*validPrice(p.Output) +
			cacheRead*validPrice(p.CacheRead) +
			cacheWrite*validPrice(p.CacheWrite)
	}

	return tieredCost(input, p.Input, tierDefs{
		{128000, p.InputAbove128k},
		{200000, p.InputAbove200k},
		{256000, p.InputAbove256k},
		{272000, p.InputAbove272k},
	}) +
		tieredCost(output+reasoning, p.Output, tierDefs{
			{128000, p.OutputAbove128k},
			{200000, p.OutputAbove200k},
			{256000, p.OutputAbove256k},
			{272000, p.OutputAbove272k},
		}) +
		tieredCost(cacheRead, p.CacheRead, tierDefs{
			{200000, p.CacheReadAbove200k},
			{272000, p.CacheReadAbove272k},
		}) +
		tieredCost(cacheWrite, p.CacheWrite, tierDefs{
			{200000, p.CacheWriteAbove200k},
		})
}

type CalcOption func(*calcOptions)
type calcOptions struct {
	tiered bool
}

// WithTiered enables tiered pricing based on context window thresholds.
func WithTiered(tiered bool) CalcOption {
	return func(o *calcOptions) { o.tiered = tiered }
}

// ---------------------------------------------------------------------------
// Pricing lookup
// ---------------------------------------------------------------------------

func (e *Engine) lookupPricing(modelID string) *Rates {
	id := strings.TrimSpace(strings.ToLower(modelID))
	if id == "" {
		return nil
	}

	e.mu.Lock()
	if cached, ok := e.cache[id]; ok {
		e.mu.Unlock()
		return cached
	}
	e.mu.Unlock()

	// 1. Check hardcoded overrides first (fast path)
	if r := findOverride(id); r != nil {
		e.mu.Lock()
		e.cache[id] = r
		e.mu.Unlock()
		log.Printf("[pricing] lookup model=%s via=override hasCost=%v", id, r.Input > 0 || r.Output > 0)
		return r
	}

	// 2. Build candidate list for dataset lookup
	candidates := modelCandidates(id)

	// 3. LiteLLM exact/fuzzy
	var hit *Rates
	var via string
	for _, c := range candidates {
		if e.litellm != nil {
			if r := findInDataset(c, e.litellm); r != nil {
				hit = r
				via = "litellm"
				break
			}
		}
	}

	// 4. OpenRouter fallback (only if no cacheRead from LiteLLM)
	if hit == nil || hit.CacheRead == 0 {
		for _, c := range candidates {
			if e.openrouter != nil {
				if r := findInDataset(c, e.openrouter); r != nil {
					if hit == nil || r.CacheRead > 0 {
						hit = r
						via = "openrouter"
					}
					break
				}
			}
		}
	}

	// 5. Fuzzy fallback
	if hit == nil && e.litellm != nil {
		if r := findFuzzy(id, e.litellm); r != nil {
			hit = r
			via = "litellm-fuzzy"
		}
	}
	if hit == nil && e.openrouter != nil {
		if r := findFuzzy(id, e.openrouter); r != nil {
			hit = r
			via = "openrouter-fuzzy"
		}
	}

	e.mu.Lock()
	e.cache[id] = hit
	e.mu.Unlock()

	if hit == nil {
		log.Printf("[pricing] lookup model=%s via=nil (no pricing found)", id)
	} else {
		log.Printf("[pricing] lookup model=%s via=%s inputRate=%g outputRate=%g", id, via, hit.Input, hit.Output)
	}
	return hit
}

func findOverride(id string) *Rates {
	if r, ok := cursorOverrides[id]; ok {
		return &r
	}
	bare := bareModelID(id)
	if r, ok := deepseekOverrides[bare]; ok {
		return &r
	}
	if r, ok := cursorOverrides[bare]; ok {
		return &r
	}
	return nil
}

func findInDataset(id string, data map[string]*litellmEntry) *Rates {
	if data == nil {
		return nil
	}
	if entry, ok := data[id]; ok && entry != nil {
		return entryToRates(entry)
	}
	if entry, ok := data["openai/"+id]; ok && entry != nil {
		return entryToRates(entry)
	}
	for key, entry := range data {
		if strings.EqualFold(key, id) && entry != nil {
			return entryToRates(entry)
		}
	}
	return nil
}

func findFuzzy(id string, data map[string]*litellmEntry) *Rates {
	if data == nil || len(id) < 5 {
		return nil
	}
	normalized := normalizeComparable(id)

	var bestKey string
	var bestEntry *litellmEntry
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
	if bestEntry != nil {
		return entryToRates(bestEntry)
	}
	return nil
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

func entryToRates(entry *litellmEntry) *Rates {
	if entry == nil {
		return nil
	}
	input := entry.InputCostPerToken
	output := entry.OutputCostPerToken
	if input == 0 && output == 0 {
		return nil
	}
	return &Rates{
		Input:               input,
		Output:              output,
		CacheRead:           entry.CacheReadInputTokenCost,
		CacheWrite:          entry.CacheCreationInputTokenCost,
		InputAbove128k:      entry.InputCostPerTokenAbove128kTokens,
		InputAbove200k:      entry.InputCostPerTokenAbove200kTokens,
		InputAbove256k:      entry.InputCostPerTokenAbove256kTokens,
		InputAbove272k:      entry.InputCostPerTokenAbove272kTokens,
		OutputAbove128k:     entry.OutputCostPerTokenAbove128kTokens,
		OutputAbove200k:     entry.OutputCostPerTokenAbove200kTokens,
		OutputAbove256k:     entry.OutputCostPerTokenAbove256kTokens,
		OutputAbove272k:     entry.OutputCostPerTokenAbove272kTokens,
		CacheReadAbove200k:  entry.CacheReadAbove200kTokens,
		CacheReadAbove272k:  entry.CacheReadAbove272kTokens,
		CacheWriteAbove200k: entry.CacheCreationAbove200kTokens,
	}
}

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

func tieredCost(tokens float64, basePrice float64, tiers tierDefs) float64 {
	if tokens <= 0 || basePrice <= 0 {
		return 0
	}
	price := validPrice(basePrice)
	var lower float64
	var cost float64

	for _, t := range tiers {
		tierPrice := validPrice(t.price)
		if t.price == 0 || t.threshold <= lower {
			continue
		}
		if tokens <= t.threshold {
			return cost + math.Max(0, tokens-lower)*price
		}
		cost += (t.threshold - lower) * price
		lower = t.threshold
		price = tierPrice
	}
	return cost + math.Max(0, tokens-lower)*price
}

type tierDef struct {
	threshold float64
	price     float64
}
type tierDefs []tierDef

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
