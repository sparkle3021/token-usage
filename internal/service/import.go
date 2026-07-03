package service

// Package service 的子文件，CC-Switch 导入和 CSV 导入业务逻辑。
import (
	"encoding/csv"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"token-dashboard/internal/collector"
	"token-dashboard/internal/collector/orchestrator"
	"token-dashboard/internal/config"
	"token-dashboard/internal/database"
	"token-dashboard/internal/model"
)

// ImportService CC-Switch 导入和 CSV 导入业务逻辑。
type ImportService struct {
	db     *database.Manager
	engine *orchestrator.Engine
}

// NewImportService 创建导入服务实例。
func NewImportService(db *database.Manager, eng *orchestrator.Engine) *ImportService {
	return &ImportService{db: db, engine: eng}
}

func (s *ImportService) DetectCCSwitchDB() string {
	if home, err := os.UserHomeDir(); err == nil {
		defaultPath := filepath.Join(home, ".cc-switch", "cc-switch.db")
		if _, err := os.Stat(defaultPath); err == nil {
			log.Printf("[service] DetectCCSwitchDB found at %s", defaultPath)
			return defaultPath
		}
		log.Printf("[service] DetectCCSwitchDB not found at %s", defaultPath)
	}
	return ""
}

func (s *ImportService) ImportCCSwitchDB() model.CCSwitchImportResult {
	if s.db == nil {
		return model.CCSwitchImportResult{Error: "数据库未初始化"}
	}
	if s.engine == nil {
		return model.CCSwitchImportResult{Error: "引擎未初始化"}
	}

	stats, err := s.engine.SyncCCSwitch()
	if err != nil {
		return model.CCSwitchImportResult{Error: fmt.Sprintf("导入失败: %v", err)}
	}
	log.Printf("[service] ImportCCSwitchDB sync done proxy_rows=%d proxy_keys=%d rollup_rows=%d recon=%d",
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

func (s *ImportService) ImportCSV(filePath string) model.CSVImportResult {
	if s.db == nil {
		return model.CSVImportResult{Error: "数据库未初始化"}
	}
	if filePath == "" {
		return model.CSVImportResult{Error: "未选择文件"}
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

		usageDate := config.NormalizeCSVDate(date)
		if usageDate == "" {
			log.Printf("[service] ImportCSV skipping row %d: unparseable date %q", rowIdx+2, date)
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

	var imported int
	for _, row := range acc {
		row.TotalTokens = row.InputTokens + row.OutputTokens + row.CacheCreationTokens + row.CacheReadTokens
		if err := s.db.UpsertDaily(row); err != nil {
			log.Printf("[service] ImportCSV upsert error source=%s date=%s model=%s err=%v",
				row.Source, row.UsageDate, row.Model, err)
			continue
		}
		imported++
	}

	log.Printf("[service] ImportCSV done total=%d skipped=%d imported=%d (accumulated from %d CSV rows)",
		len(acc), skippedRows, imported, totalRows)
	return model.CSVImportResult{
		Total:    totalRows,
		Imported: imported,
		Skipped:  skippedRows,
	}
}

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
