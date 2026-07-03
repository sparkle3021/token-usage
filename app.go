package main

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"token-dashboard/internal/collector"
	"token-dashboard/internal/collector/engine"
	"token-dashboard/internal/database"
	"token-dashboard/internal/model"
	"token-dashboard/internal/pricing"

	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

type App struct {
	ctx     context.Context
	db      *database.Manager
	pricing *pricing.Engine
	engine  *engine.Engine
	dataDir string

	autoSyncMu      sync.Mutex
	autoSyncCancel  context.CancelFunc
	autoSyncMinutes int

	refreshMu       sync.Mutex
	refreshCancel   context.CancelFunc
	refreshSeconds  int
}

func NewApp() *App {
	// Resolve data directory: ~/.token-dashboard (or DATA_DIR env override)
	dataDir := resolveDataDir()
	dbPath := filepath.Join(dataDir, "td.db")

	// Initialize database
	db, err := database.New(dbPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[app] database: %v\n", err)
		log.Printf("[app] NewApp database failed path=%s err=%v", dbPath, err)
		return &App{ctx: context.Background()}
	}
	log.Printf("[app] NewApp database opened path=%s", dbPath)

	// Initialize pricing
	pr, err := pricing.NewEngine(dataDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[app] pricing: %v\n", err)
		log.Printf("[app] NewApp pricing error=%v", err)
	}
	log.Printf("[app] NewApp pricing loaded")

	// Initialize collection engine
	eng := engine.New(db, pr)
	log.Printf("[app] NewApp engine initialized collectors=%d", len(eng.Collectors()))

	return &App{
		db:      db,
		pricing: pr,
		engine:  eng,
			dataDir: dataDir,
	}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx

	// Wire engine events to Wails runtime
	if a.engine != nil {
		a.engine.SetEventCallback(func(event string, data interface{}) {
			fmt.Printf("[event] %s: %v\n", event, data)
			wailsRuntime.EventsEmit(a.ctx, event, data)
		})
	}

	// Auto-detect CC-Switch DB path on startup if not configured
	if a.db != nil {
		existing, _ := a.db.GetConfig("cc_switch_db_path")
		if existing == "" {
			if home, err := os.UserHomeDir(); err == nil {
				defaultPath := filepath.Join(home, ".cc-switch", "cc-switch.db")
				if _, err := os.Stat(defaultPath); err == nil {
					a.db.SetConfig("cc_switch_db_path", defaultPath)
					log.Printf("[app] startup auto-detected cc-switch db at %s", defaultPath)
				} else {
					log.Printf("[app] startup cc-switch db not found at %s", defaultPath)
				}
			}
		}

		// Detect stale CC-Switch checkpoints (CK exists but no data was ever written)
		ckProxy, _ := a.db.GetCheckpoint("cc_switch_cursor_proxy_request_logs")
		ckRollup, _ := a.db.GetCheckpoint("cc_switch_rollup_max_date")
		if ckProxy != "" || ckRollup != "" {
			var dailyCnt, hourCnt int
			a.db.DB().QueryRow("SELECT COUNT(*) FROM daily_usage WHERE source='CC-Switch'").Scan(&dailyCnt)
			a.db.DB().QueryRow("SELECT COUNT(*) FROM hour_usage WHERE source='CC-Switch'").Scan(&hourCnt)
			if dailyCnt == 0 && hourCnt == 0 {
				log.Printf("[app] stale CC-Switch checkpoint detected (proxy=%q rollup=%q), data count=0 — resetting for full re-sync", ckProxy, ckRollup)
				a.db.ResetCCSwitchCheckpoints()
			}
		}
	}
}

// ---------------------------------------------------------------------------
// Dashboard API
// ---------------------------------------------------------------------------

func (a *App) GetDashboardData() *model.DashboardData {
	defer log.Printf("[app] GetDashboardData done")
	start := time.Now()

	if a.db == nil {
		log.Printf("[app] GetDashboardData db=nil")
		return &model.DashboardData{}
	}

	daily, err := a.db.QueryDaily()
	if err != nil {
		log.Printf("[app] GetDashboardData QueryDaily err=%v", err)
	}
	sessions, err := a.db.QuerySessions()
	if err != nil {
		log.Printf("[app] GetDashboardData QuerySessions err=%v", err)
	}
	runs, err := a.db.QueryRuns(500)
	if err != nil {
		log.Printf("[app] GetDashboardData QueryRuns err=%v", err)
	}

	// Enrich daily rows with project path from session data
	projMap := make(map[string]string)
	for _, s := range sessions {
		proj := s.ProjectPath
		if proj == "" {
			parts := strings.Split(s.SessionID, "/")
			proj = parts[len(parts)-1]
		}
		key := s.Device + "::" + s.Source
		if _, ok := projMap[key]; !ok {
			projMap[key] = proj
		}
	}

	for i := range daily {
		if key, ok := projMap[daily[i].Device+"::"+daily[i].Source]; ok {
			daily[i].ProjectPath = key
		}
	}

	// Normalize runs
	for i := range runs {
		runs[i].Message = strings.ReplaceAll(runs[i].Message, "\n", " ")
	}

	log.Printf("[app] GetDashboardData daily=%d sessions=%d runs=%d elapsed=%v",
		len(daily), len(sessions), len(runs), time.Since(start))

	return &model.DashboardData{
		Daily:    daily,
		Sessions: sessions,
		Runs:     runs,
	}
}

func (a *App) GetTimeSeriesData() *model.TimeSeriesData {
	start := time.Now()
	if a.db == nil {
		log.Printf("[app] GetTimeSeriesData db=nil")
		return &model.TimeSeriesData{}
	}
	timeRows, _ := a.db.QueryTimeUsage()
	hourRows, _ := a.db.QueryHourUsage()
	log.Printf("[app] GetTimeSeriesData timeRows=%d hourRows=%d elapsed=%v", len(timeRows), len(hourRows), time.Since(start))
	return &model.TimeSeriesData{Time: timeRows, Hour: hourRows}
}

// ---------------------------------------------------------------------------
// Collection API
// ---------------------------------------------------------------------------

func (a *App) StartCollection() bool {
	if a.engine == nil {
		log.Printf("[app] StartCollection engine=nil")
		return false
	}
	ok := a.engine.StartCollection()
	log.Printf("[app] StartCollection result=%v", ok)
	return ok
}

func (a *App) StartFullCollection() bool {
	if a.engine == nil {
		log.Printf("[app] StartFullCollection engine=nil")
		return false
	}
	ok := a.engine.StartFullCollection()
	log.Printf("[app] StartFullCollection result=%v", ok)
	return ok
}

func (a *App) CollectStatus() *model.CollectStatus {
	if a.engine == nil {
		log.Printf("[app] CollectStatus engine=nil")
		return &model.CollectStatus{Status: "idle", Message: "未初始化"}
	}
	s := a.engine.Status()
	log.Printf("[app] CollectStatus status=%s message=%s", s.Status, s.Message)
	return &model.CollectStatus{
		Status:     s.Status,
		Message:    s.Message,
		StartedAt:  s.StartedAt,
		FinishedAt: s.FinishedAt,
		ExitCode:   s.ExitCode,
		Stdout:     s.Stdout,
		Stderr:     s.Stderr,
	}
}

// ---------------------------------------------------------------------------
// Auto-Sync API
// ---------------------------------------------------------------------------

func (a *App) SetAutoSyncInterval(minutes int) {
	a.autoSyncMu.Lock()
	defer a.autoSyncMu.Unlock()

	// Stop existing ticker
	if a.autoSyncCancel != nil {
		a.autoSyncCancel()
		a.autoSyncCancel = nil
	}
	a.autoSyncMinutes = minutes

	if minutes <= 0 {
		log.Println("[app] Auto-sync disabled")
		return
	}

	ctx, cancel := context.WithCancel(context.Background())
	a.autoSyncCancel = cancel

	go func() {
		ticker := time.NewTicker(time.Duration(minutes) * time.Minute)
		defer ticker.Stop()
		log.Printf("[app] Auto-sync started interval=%dm", minutes)
		for {
			select {
			case <-ticker.C:
				log.Println("[app] Auto-sync triggering collection")
				a.StartCollection()
			case <-ctx.Done():
				log.Println("[app] Auto-sync stopped")
				return
			}
		}
	}()
}

func (a *App) GetAutoSyncInterval() int {
	a.autoSyncMu.Lock()
	defer a.autoSyncMu.Unlock()
	return a.autoSyncMinutes
}

// ClearAllData deletes all usage data and collection history.
func (a *App) ClearAllData() error {
	if a.db == nil {
		return fmt.Errorf("数据库未初始化")
	}
	if err := a.db.ClearAllUsageData(); err != nil {
		log.Printf("[app] ClearAllData error: %v", err)
		return fmt.Errorf("清除失败: %v", err)
	}
	// Clear in-memory caches so collectors re-parse files on next sync
	if a.engine != nil {
		a.engine.ClearCollectorCaches()
	}
	log.Printf("[app] ClearAllData ok")
	return nil
}

// ---------------------------------------------------------------------------
// Settings API
// ---------------------------------------------------------------------------

func (a *App) GetSettings() model.AppConfig {
	s, err := a.db.GetAllConfigs()
	if err != nil {
		return model.AppConfig{AutoSyncMinutes: 5}
	}

	cfg := model.AppConfig{
		AutoSyncMinutes: atoiDef(s["auto_sync_minutes"], 5),
		CCSwitchDBPath:  s["cc_switch_db_path"],
	}
	// Auto-detect default CC-Switch DB path if not configured
	if cfg.CCSwitchDBPath == "" {
		if home, err := os.UserHomeDir(); err == nil {
			defaultPath := filepath.Join(home, ".cc-switch", "cc-switch.db")
			if _, err := os.Stat(defaultPath); err == nil {
				cfg.CCSwitchDBPath = defaultPath
				log.Printf("[app] GetSettings auto-detected cc-switch db at %s", defaultPath)
			} else {
				log.Printf("[app] GetSettings cc-switch db not found at %s", defaultPath)
			}
		}
	}

	// Sync runtime auto-sync with config
	a.autoSyncMu.Lock()
	if cfg.AutoSyncMinutes != a.autoSyncMinutes {
		a.autoSyncMu.Unlock()
		a.SetAutoSyncInterval(cfg.AutoSyncMinutes)
	} else {
		a.autoSyncMu.Unlock()
	}

	return cfg
}

func (a *App) SaveSettings(cfg model.AppConfig) error {
	pairs := map[string]string{
		"auto_sync_minutes": strconv.Itoa(cfg.AutoSyncMinutes),
		"cc_switch_db_path": cfg.CCSwitchDBPath,
	}
	for k, v := range pairs {
		if err := a.db.SetConfig(k, v); err != nil {
			return fmt.Errorf("save config %s: %w", k, err)
		}
	}

	// Apply auto-sync immediately
	a.SetAutoSyncInterval(cfg.AutoSyncMinutes)

	log.Printf("[app] SaveSettings ok autoSync=%d", cfg.AutoSyncMinutes)
	return nil
}

// DetectCCSwitchDB checks the default cc-switch database path and returns it if found.
func (a *App) DetectCCSwitchDB() string {
	if home, err := os.UserHomeDir(); err == nil {
		defaultPath := filepath.Join(home, ".cc-switch", "cc-switch.db")
		if _, err := os.Stat(defaultPath); err == nil {
			log.Printf("[app] DetectCCSwitchDB found at %s", defaultPath)
			return defaultPath
		}
		log.Printf("[app] DetectCCSwitchDB not found at %s", defaultPath)
	}
	return ""
}

// ImportCCSwitchDB runs CC-Switch import synchronously and returns the result.
func (a *App) ImportCCSwitchDB() model.CCSwitchImportResult {
	if a.db == nil {
		return model.CCSwitchImportResult{Error: "数据库未初始化"}
	}
	if a.engine == nil {
		return model.CCSwitchImportResult{Error: "引擎未初始化"}
	}

	stats, err := a.engine.SyncCCSwitch()
	if err != nil {
		return model.CCSwitchImportResult{Error: fmt.Sprintf("导入失败: %v", err)}
	}
	log.Printf("[app] ImportCCSwitchDB sync done proxy_rows=%d proxy_keys=%d rollup_rows=%d recon=%d",
		stats.ProxyRows, stats.ProxyKeys, stats.RollupRows, stats.ReconSupplement)

	imported := stats.ProxyKeys + stats.ReconSupplement
	total := stats.ProxyRows + stats.RollupRows
	return model.CCSwitchImportResult{
		Total:    total,
		Imported: imported,
		Message:  fmt.Sprintf("共 %d 条记录，导入 %d 条（proxy: %d 行→%d 聚合, rollup: %d 行, 补充: %d 条）",
			total, imported, stats.ProxyRows, stats.ProxyKeys, stats.RollupRows, stats.ReconSupplement),
	}
}

// ── Pricing URLs ──────────────────────────────────────────────────────────

const (
	pricingLiteLLMURL   = "https://raw.githubusercontent.com/BerriAI/litellm/main/model_prices_and_context_window.json"
	pricingOpenRouterURL = "https://openrouter.ai/api/v1/usage/rates"
)

// UpdatePricing fetches the latest pricing data and reloads the engine.
func (a *App) UpdatePricing() model.PricingUpdateResult {
	log.Printf("[app] UpdatePricing started")
	priceDir := filepath.Join(a.dataDir, "config")
	os.MkdirAll(priceDir, 0755)

	result := model.PricingUpdateResult{}

	// Fetch LiteLLM
	litellmData, err := fetchPricingJSON(pricingLiteLLMURL)
	if err != nil {
		log.Printf("[app] UpdatePricing litellm error=%v", err)
		result.Error = fmt.Sprintf("LiteLLM 获取失败: %v", err)
		return result
	}
	if err := os.WriteFile(filepath.Join(priceDir, "pricing-litellm.json"), wrapPricingJSON(litellmData), 0644); err != nil {
		log.Printf("[app] UpdatePricing litellm write error=%v", err)
		result.Error = fmt.Sprintf("LiteLLM 写入失败: %v", err)
		return result
	}

	// Count entries in the fetched data
	var litellmRaw map[string]interface{}
	json.Unmarshal(litellmData, &litellmRaw)
	result.Litellm = len(litellmRaw)

	// Fetch OpenRouter (best-effort)
	openrouterData, err := fetchPricingJSON(pricingOpenRouterURL)
	if err == nil {
		if err := os.WriteFile(filepath.Join(priceDir, "pricing-openrouter.json"), openrouterData, 0644); err == nil {
			var orRaw map[string]interface{}
			json.Unmarshal(openrouterData, &orRaw)
			result.OpenRouter = len(orRaw)
		}
	} else {
		log.Printf("[app] UpdatePricing openrouter skipped: %v", err)
	}

	// Reload pricing engine
	if a.pricing != nil {
		if err := a.pricing.Reload(a.dataDir); err != nil {
			log.Printf("[app] UpdatePricing reload error=%v", err)
			result.Error = fmt.Sprintf("价格引擎重载失败: %v", err)
			return result
		}
	}

	result.Message = fmt.Sprintf("LiteLLM %d 条, OpenRouter %d 条", result.Litellm, result.OpenRouter)
	log.Printf("[app] UpdatePricing done %s", result.Message)
	return result
}

// fetchPricingJSON fetches a URL and returns the raw JSON body.
func fetchPricingJSON(url string) ([]byte, error) {
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("fetch %s: %w", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetch %s: HTTP %d", url, resp.StatusCode)
	}
	return io.ReadAll(resp.Body)
}

// wrapPricingJSON wraps raw JSON data in the {fetchedAt, data} format.
func wrapPricingJSON(raw []byte) []byte {
	wrapped := map[string]interface{}{
		"fetchedAt": time.Now().UnixMilli(),
		"data":      json.RawMessage(raw),
	}
	b, _ := json.Marshal(wrapped)
	return b
}

func (a *App) ImportCSV() model.CSVImportResult {
	if a.db == nil {
		return model.CSVImportResult{Error: "数据库未初始化"}
	}

	filePath, err := wailsRuntime.OpenFileDialog(a.ctx, wailsRuntime.OpenDialogOptions{
		Title: "选择 cc-switch 历史数据 CSV",
		Filters: []wailsRuntime.FileFilter{
			{DisplayName: "CSV Files (*.csv)", Pattern: "*.csv"},
		},
	})
	if err != nil {
		return model.CSVImportResult{Error: fmt.Sprintf("打开文件对话框失败: %v", err)}
	}
	if filePath == "" {
		return model.CSVImportResult{Error: "已取消"}
	}

	f, err := os.Open(filePath)
	if err != nil {
		return model.CSVImportResult{Error: fmt.Sprintf("打开文件失败: %v", err)}
	}
	defer f.Close()

	r := csv.NewReader(f)
	records, err := r.ReadAll()
	if err != nil {
		return model.CSVImportResult{Error: fmt.Sprintf("读取 CSV 失败: %v", err)}
	}
	if len(records) < 2 {
		return model.CSVImportResult{Error: "CSV 文件无数据行"}
	}

	// Validate header
	header := records[0]
	expectedHeaders := []string{"date", "app_type", "provider_id", "model", "request_model",
		"pricing_model", "request_count", "success_count", "input_tokens", "output_tokens",
		"cache_read_tokens", "cache_creation_tokens", "total_cost_usd", "avg_latency_ms"}
	if len(header) < len(expectedHeaders) {
		return model.CSVImportResult{Error: fmt.Sprintf("CSV 列数不足 (got %d, want %d)", len(header), len(expectedHeaders))}
	}
	for i, want := range expectedHeaders {
		if strings.TrimSpace(header[i]) != want {
			return model.CSVImportResult{Error: fmt.Sprintf("CSV 表头不符: 第 %d 列 got %q, want %q", i+1, header[i], want)}
		}
	}

	device, _ := os.Hostname()
	if device == "" {
		device = "unknown"
	}

	// Accumulate rows by (source, date, model) to avoid PK conflicts
	type accKey struct{ source, date, model string }
	acc := make(map[accKey]*model.DailyUsage)
	var totalRows, skippedRows int

	for rowIdx, row := range records[1:] {
		if len(row) < len(expectedHeaders) {
			skippedRows++
			continue
		}

		date := strings.TrimSpace(row[0])
		appType := strings.TrimSpace(row[1])
		modelName := strings.TrimSpace(row[3])
		inputTokens, _ := strconv.ParseInt(strings.TrimSpace(row[8]), 10, 64)
		outputTokens, _ := strconv.ParseInt(strings.TrimSpace(row[9]), 10, 64)
		cacheReadTokens, _ := strconv.ParseInt(strings.TrimSpace(row[10]), 10, 64)
		cacheCreationTokens, _ := strconv.ParseInt(strings.TrimSpace(row[11]), 10, 64)
		costUSD, _ := strconv.ParseFloat(strings.TrimSpace(row[12]), 64)

		usageDate := normalizeCSVDate(date)
		if usageDate == "" {
			log.Printf("[app] ImportCSV skipping row %d: unparseable date %q", rowIdx+2, date)
			skippedRows++
			continue
		}

		modelName = collector.NormalizeModelForGrouping(modelName)
		if modelName == "" {
			modelName = "unknown"
		}

		source := sourceFromAppType(appType)

		key := accKey{source: source, date: usageDate, model: modelName}
		existing, ok := acc[key]
		if !ok {
			acc[key] = &model.DailyUsage{
				Device:    device,
				Source:    source,
				UsageDate: usageDate,
				Model:     modelName,
			}
			existing = acc[key]
		}
		existing.InputTokens += inputTokens
		existing.OutputTokens += outputTokens
		existing.CacheReadTokens += cacheReadTokens
		existing.CacheCreationTokens += cacheCreationTokens
		existing.CostUSD += costUSD
		totalRows++
	}

	// Upsert accumulated rows
	var imported int
	for _, row := range acc {
		row.TotalTokens = row.InputTokens + row.OutputTokens + row.CacheCreationTokens + row.CacheReadTokens
		if err := a.db.UpsertDaily(row); err != nil {
			log.Printf("[app] ImportCSV upsert error source=%s date=%s model=%s err=%v",
				row.Source, row.UsageDate, row.Model, err)
			continue
		}
		imported++
	}

	log.Printf("[app] ImportCSV done total=%d skipped=%d imported=%d (accumulated from %d CSV rows)",
		len(acc), skippedRows, imported, totalRows)
	return model.CSVImportResult{
		Total:    totalRows,
		Imported: imported,
		Skipped:  skippedRows,
	}
}

// sourceFromAppType maps cc-switch app_type to the dashboard source name.
func sourceFromAppType(appType string) string {
	switch strings.ToLower(strings.TrimSpace(appType)) {
	case "claude":
		return "Claude Code"
	case "codex":
		return "Codex CLI"
	case "opencode":
		return "OpenCode"
	default:
		return appType
	}
}

// normalizeCSVDate converts cc-switch date format (e.g. "2026/1/16") to "2006-01-02".
func normalizeCSVDate(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	// Try YYYY/M/D format first
	parts := strings.Split(s, "/")
	if len(parts) == 3 && len(parts[0]) == 4 {
		return fmt.Sprintf("%s-%s-%s", parts[0], padNum(parts[1]), padNum(parts[2]))
	}
	// Fallback to standard parsers
	for _, layout := range []string{"2006-01-02", "2006/1/2", time.RFC3339[:10]} {
		if t, err := time.Parse(layout, s); err == nil {
			return t.Format("2006-01-02")
		}
	}
	return ""
}

func padNum(s string) string {
	s = strings.TrimSpace(s)
	if len(s) == 1 {
		return "0" + s
	}
	return s
}

func atoiDef(s string, def int) int {
	if s == "" {
		return def
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return def
	}
	return n
}

// resolveDataDir returns the application data directory.
// Defaults to ~/.token-dashboard, with DATA_DIR env var override.
// On first run, migrates existing data/ directory content automatically.
func resolveDataDir() string {
	if env := os.Getenv("DATA_DIR"); env != "" {
		abs, err := filepath.Abs(env)
		if err == nil {
			return abs
		}
		return env
	}

	home, err := os.UserHomeDir()
	if err != nil {
		log.Printf("[app] resolveDataDir cannot get home dir, falling back to ./data: %v", err)
		abs, _ := filepath.Abs("data")
		return abs
	}
	target := filepath.Join(home, ".token-dashboard")

	// Migrate from old data/ directory on first run
	if err := migrateFromOldData(target); err != nil {
		log.Printf("[app] resolveDataDir migration warning: %v", err)
	}

	// Ensure pricing files are in price/ subdirectory
	migratePricingToSubdir(target)

	os.MkdirAll(target, 0755)
	return target
}

// migrateFromOldData migrates data/ content to target dir if target is empty.
func migrateFromOldData(target string) error {
	// Skip if target already has a database
	if _, err := os.Stat(filepath.Join(target, "td.db")); err == nil {
		return nil
	}

	oldDir := "data"
	if _, err := os.Stat(filepath.Join(oldDir, "usage.sqlite")); err != nil {
		return nil // no old data to migrate
	}

	log.Printf("[app] migrating data/ → %s", target)
	if err := os.MkdirAll(target, 0755); err != nil {
		return fmt.Errorf("create target dir: %w", err)
	}

	priceDir := filepath.Join(target, "config")
	os.MkdirAll(priceDir, 0755)

	entries, err := os.ReadDir(oldDir)
	if err != nil {
		return fmt.Errorf("read old dir: %w", err)
	}

	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		src := filepath.Join(oldDir, e.Name())

		// Route pricing files to price/ subdirectory
		dstDir := target
		if strings.HasPrefix(e.Name(), "pricing-") {
			dstDir = priceDir
		}

		dst := filepath.Join(dstDir, e.Name())
		data, err := os.ReadFile(src)
		if err != nil {
			log.Printf("[app] migrate skip %s: %v", e.Name(), err)
			continue
		}
		if err := os.WriteFile(dst, data, 0644); err != nil {
			return fmt.Errorf("write %s: %w", e.Name(), err)
		}
		log.Printf("[app] migrate copied %s", e.Name())
	}

	log.Printf("[app] migration complete: data/ → %s", target)
	return nil
}

// migratePricingToSubdir moves pricing files from target root into price/ subdirectory.
// This handles the case where files exist from a previous migration before the price/ dir was introduced.
func migratePricingToSubdir(target string) {
	priceDir := filepath.Join(target, "config")
	os.MkdirAll(priceDir, 0755)

	for _, name := range []string{"pricing-litellm.json", "pricing-openrouter.json"} {
		src := filepath.Join(target, name)
		if _, err := os.Stat(src); err != nil {
			continue // not in root, already in price/ or doesn't exist
		}
		dst := filepath.Join(priceDir, name)
		// Only move if not already in price/
		if _, err := os.Stat(dst); err == nil {
			os.Remove(src) // duplicate in root, clean up
			log.Printf("[app] migratePricingToSubdir cleaned up %s", name)
			continue
		}
		data, err := os.ReadFile(src)
		if err != nil {
			log.Printf("[app] migratePricingToSubdir skip %s: %v", name, err)
			continue
		}
		if err := os.WriteFile(dst, data, 0644); err != nil {
			log.Printf("[app] migratePricingToSubdir write %s: %v", name, err)
			continue
		}
		os.Remove(src)
		log.Printf("[app] migratePricingToSubdir moved %s → price/", name)
	}
}
