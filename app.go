package main

import (
	"context"
	"encoding/csv"
	"fmt"
	"log"
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

	autoSyncMu      sync.Mutex
	autoSyncCancel  context.CancelFunc
	autoSyncMinutes int
}

func NewApp() *App {
	// Resolve data directory
	dataDir := "data"
	if env := os.Getenv("DATA_DIR"); env != "" {
		dataDir = env
	}
	absData, err := filepath.Abs(dataDir)
	if err == nil {
		dataDir = absData
	}
	os.MkdirAll(dataDir, 0755)

	dbPath := filepath.Join(dataDir, "usage.sqlite")

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
	}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx

	// Wire engine events to Wails runtime
	if a.engine != nil {
		a.engine.SetEventCallback(func(event string, data interface{}) {
			// In a full implementation, this would use runtime.EventsEmit
			// For now, just log
			fmt.Printf("[event] %s: %v\n", event, data)
		})
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
	log.Printf("[app] GetTimeSeriesData rows=%d elapsed=%v", len(timeRows), time.Since(start))
	return &model.TimeSeriesData{Time: timeRows}
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

// ---------------------------------------------------------------------------
// CSV Import API
// ---------------------------------------------------------------------------

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
