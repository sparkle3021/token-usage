package collector

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"token-dashboard/internal/model"

	_ "modernc.org/sqlite"
)

// CCSwitchCollector imports usage data from a CC-Switch SQLite database.
// It supports incremental sync via persisted checkpoints for both
// proxy_request_logs (per-request, current month) and
// usage_daily_rollups (daily aggregated, historical).
type CCSwitchCollector struct {
	store             CheckpointStore
	device            string
	lastResult        CCSwitchStats
	pendingProxyCur   int64
	pendingProxyMax   int64
	pendingRollupDate string
}

// CCSwitchStats holds the result statistics from the last Collect() run.
type CCSwitchStats struct {
	ProxyRows       int
	ProxyKeys       int
	RollupRows      int
	ReconChecked    int
	ReconSupplement int
	ReconSkipped    int
}

// Stats returns the statistics from the last Collect() call.
func (c *CCSwitchCollector) Stats() CCSwitchStats { return c.lastResult }

// SavePendingCheckpoints persists staged checkpoints to the store.
// Must be called after data is successfully committed to the database.
func (c *CCSwitchCollector) SavePendingCheckpoints() {
	if c.pendingProxyCur > 0 {
		c.store.SetCheckpoint(ckCursorProxyLogs, strconv.FormatInt(c.pendingProxyCur, 10))
	} else if c.pendingProxyMax > 0 && c.pendingProxyCur == 0 {
		c.store.SetCheckpoint(ckCursorProxyLogs, strconv.FormatInt(c.pendingProxyMax, 10))
	}
	if c.pendingRollupDate != "" {
		c.store.SetCheckpoint(ckRollupMaxDate, c.pendingRollupDate)
	}
}

// CheckpointStore is the minimal interface the collector needs for checkpoint persistence.
type CheckpointStore interface {
	GetCheckpoint(key string) (string, error)
	SetCheckpoint(key, value string) error
	DeleteCheckpointsByPrefix(prefix string) error
	GetConfig(key string) (string, error)
	UpsertDaily(row *model.DailyUsage) error
	BulkUpsertHourUsage(rows []model.HourUsage) error
	BuildDailyFromHourUsage() error
}

// checkpoint keys for CC-Switch sync state.
const (
	ckCursorProxyLogs = "cc_switch_cursor_proxy_request_logs"
	ckRollupMaxDate   = "cc_switch_rollup_max_date"
)

var _ Collector = (*CCSwitchCollector)(nil)

// NewCCSwitchCollector creates a new CCSwitchCollector.
func NewCCSwitchCollector() *CCSwitchCollector {
	device, _ := os.Hostname()
	if device == "" {
		device = "unknown"
	}
	return &CCSwitchCollector{device: device}
}

// SetStore sets the checkpoint/persistence store (called after construction).
func (c *CCSwitchCollector) SetStore(store CheckpointStore) {
	c.store = store
}

func (c *CCSwitchCollector) ID() string { return "cc-switch" }

func (c *CCSwitchCollector) Source() string { return "CC-Switch" }

func (c *CCSwitchCollector) Collect(ctx context.Context, pricing TokenCalc) (*CollectResult, error) {
	if c.store == nil {
		return nil, fmt.Errorf("CCSwitchCollector: store not set")
	}

	// Read the cc-switch database path from app config
	dbPath, _ := c.store.GetConfig("cc_switch_db_path")
	dbPath = ExpandPath(dbPath)
	if dbPath == "" {
		log.Printf("[collector] CCSwitch cc_switch_db_path not configured, skipping")
		return &CollectResult{Device: c.device, Source: c.Source()}, nil
	}
	if _, err := os.Stat(dbPath); err != nil {
		log.Printf("[collector] CCSwitch db not found at %q, skipping", dbPath)
		return &CollectResult{Device: c.device, Source: c.Source()}, nil
	}

	extDB, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open cc-switch db: %w", err)
	}
	defer extDB.Close()

	start := time.Now()
	ext := &collectResultExt{}

	if err := c.importProxyLogs(extDB, ext); err != nil {
		return nil, fmt.Errorf("proxy logs import: %w", err)
	}

	if err := c.importRollups(extDB, ext); err != nil {
		return nil, fmt.Errorf("rollup import: %w", err)
	}

	log.Printf("[collector] CCSwitch done proxy_rows=%d proxy_keys=%d rollup_rows=%d recon_checked=%d recon_supplement=%d recon_skipped=%d elapsed=%v",
		ext.ProxyRows, ext.ProxyKeys, ext.RollupRows, ext.ReconChecked, ext.ReconSupplement, ext.ReconSkipped, time.Since(start))

	// Stage checkpoints (persisted to store only after SQL write succeeds)
	c.pendingProxyCur = ext.proxyLastCreated
	c.pendingProxyMax = ext.proxyMaxCreated
	c.pendingRollupDate = ext.rollupMaxDate

	c.lastResult = CCSwitchStats{
		ProxyRows:       ext.ProxyRows,
		ProxyKeys:       ext.ProxyKeys,
		RollupRows:      ext.RollupRows,
		ReconChecked:    ext.ReconChecked,
		ReconSupplement: ext.ReconSupplement,
		ReconSkipped:    ext.ReconSkipped,
	}

	return &CollectResult{
		Device:   c.device,
		Source:   c.Source(),
		HourRows: ext.proxyBatch,
		Daily:    ext.rollupRows,
	}, nil
}

// ---------------------------------------------------------------------------
// Proxy request logs (incremental, per-hour granularity)
// ---------------------------------------------------------------------------

type hourAccKey struct{ date, source, model string; hour int }

type collectResultExt struct {
	ProxyRows        int
	ProxyKeys        int
	RollupRows       int
	ReconChecked     int
	ReconSupplement  int
	ReconSkipped     int
	proxyBatch       []model.HourUsage
	rollupRows       []DailyRow
	proxyLastCreated int64
	proxyMaxCreated  int64
	rollupMaxDate    string
}

func (c *CCSwitchCollector) importProxyLogs(extDB *sql.DB, ext *collectResultExt) error {
	cursorStr, _ := c.store.GetCheckpoint(ckCursorProxyLogs)
	var cursorVal int64
	if cursorStr != "" {
		cursorVal, _ = strconv.ParseInt(cursorStr, 10, 64)
	}

	var maxCreated int64
	_ = extDB.QueryRow(`SELECT COALESCE(MAX(created_at), 0) FROM proxy_request_logs`).Scan(&maxCreated)

	if cursorVal > maxCreated {
		log.Printf("[collector] CCSwitch proxy checkpoint stale (cursor=%d > max=%d), resetting to full sync", cursorVal, maxCreated)
		cursorVal = 0
	}

	query := `SELECT created_at,
		date(created_at, 'unixepoch', 'localtime') as date,
		CAST(strftime('%H', datetime(created_at, 'unixepoch', 'localtime')) AS INTEGER) as hour,
		app_type, model, input_tokens, output_tokens,
		cache_read_tokens, cache_creation_tokens, total_cost_usd
		FROM proxy_request_logs
		WHERE data_source = 'proxy'
		  AND status_code >= 200 AND status_code < 300
		  AND app_type NOT IN ('claude', 'codex', 'opencode')`
	args := []interface{}{}
	if cursorVal > 0 {
		query += ` AND created_at > ?`
		args = append(args, cursorVal)
	}
	query += ` ORDER BY created_at`

	rows, err := extDB.Query(query, args...)
	if err != nil {
		return fmt.Errorf("query proxy_request_logs: %w", err)
	}
	defer rows.Close()

	acc := make(map[hourAccKey]*model.HourUsage)
	var lastCreated int64

	for rows.Next() {
		var createdAt int64
		var dateStr string
		var hour int
		var appType, modelName string
		var inputTokens, outputTokens, cacheRead, cacheCreation int64
		var costStr string
		if err := rows.Scan(&createdAt, &dateStr, &hour, &appType, &modelName,
			&inputTokens, &outputTokens, &cacheRead, &cacheCreation, &costStr); err != nil {
			continue
		}
		if createdAt > lastCreated {
			lastCreated = createdAt
		}
		usageDate := normalizeCCDate(dateStr)
		if usageDate == "" {
			continue
		}
		modelName = NormalizeModelForGrouping(modelName)
		if modelName == "" {
			modelName = "unknown"
		}
		costUSD, _ := strconv.ParseFloat(costStr, 64)
		source := ccSourceFromAppType(appType)
		key := hourAccKey{date: usageDate, hour: hour, source: source, model: modelName}
		existing, ok := acc[key]
		if !ok {
			acc[key] = &model.HourUsage{
				Device: c.device, Source: source, UsageDate: usageDate, Hour: hour, Model: modelName,
			}
			existing = acc[key]
		}
		existing.InputTokens += inputTokens
		existing.OutputTokens += outputTokens
		existing.CacheReadTokens += cacheRead
		existing.CacheCreationTokens += cacheCreation
		existing.CostUSD += costUSD
		ext.ProxyRows++
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("rows iteration: %w", err)
	}

	var batch []model.HourUsage
	for _, row := range acc {
		row.TotalTokens = row.InputTokens + row.OutputTokens + row.CacheCreationTokens + row.CacheReadTokens
		if row.TotalTokens == 0 && row.CostUSD == 0 {
			continue
		}
		batch = append(batch, *row)
	}
	ext.ProxyKeys = len(batch)
	ext.proxyBatch = batch
	ext.proxyLastCreated = lastCreated
	ext.proxyMaxCreated = maxCreated

	return nil
}

// ---------------------------------------------------------------------------
// Usage daily rollups (historical, day-level granularity)
// ---------------------------------------------------------------------------

type rollupAccKey struct {
	source string
	date   string
	model  string
}

func (c *CCSwitchCollector) importRollups(extDB *sql.DB, ext *collectResultExt) error {
	rollupDate, _ := c.store.GetCheckpoint(ckRollupMaxDate)

	query := `SELECT date, app_type, model, input_tokens, output_tokens,
		cache_read_tokens, cache_creation_tokens, total_cost_usd
		FROM usage_daily_rollups`
	args := []interface{}{}
	if rollupDate != "" {
		query += ` AND date > ?`
		args = append(args, rollupDate)
	}
	query += ` ORDER BY date`

	rows, err := extDB.Query(query, args...)
	if err != nil {
		if isNoSuchTableErr(err) {
			log.Printf("[collector] CCSwitch usage_daily_rollups table not found, skipping")
			return nil
		}
		return fmt.Errorf("query usage_daily_rollups: %w", err)
	}
	defer rows.Close()

	rollupAcc := make(map[rollupAccKey]*model.DailyUsage)
	var maxDate string

	for rows.Next() {
		var dateStr, appType, modelName string
		var inputTokens, outputTokens, cacheRead, cacheCreation int64
		var costStr string
		if err := rows.Scan(&dateStr, &appType, &modelName,
			&inputTokens, &outputTokens, &cacheRead, &cacheCreation, &costStr); err != nil {
			continue
		}
		usageDate := normalizeCCDate(dateStr)
		if usageDate == "" {
			continue
		}
		modelName = NormalizeModelForGrouping(modelName)
		if modelName == "" {
			modelName = "unknown"
		}
		costUSD, _ := strconv.ParseFloat(costStr, 64)
		source := ccSourceFromAppType(appType)

		if usageDate > maxDate {
			maxDate = usageDate
		}

		key := rollupAccKey{source: source, date: usageDate, model: modelName}
		existing, ok := rollupAcc[key]
		if !ok {
			rollupAcc[key] = &model.DailyUsage{
				Device: c.device, Source: source, UsageDate: usageDate, Model: modelName,
			}
			existing = rollupAcc[key]
		}
		existing.InputTokens += inputTokens
		existing.OutputTokens += outputTokens
		existing.CacheReadTokens += cacheRead
		existing.CacheCreationTokens += cacheCreation
		existing.CostUSD += costUSD
		ext.RollupRows++
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("rows iteration: %w", err)
	}

		var dailyRows []DailyRow
		for _, row := range rollupAcc {
			row.TotalTokens = row.InputTokens + row.OutputTokens + row.CacheCreationTokens + row.CacheReadTokens
			if row.TotalTokens == 0 && row.CostUSD == 0 {
				continue
			}
			ext.ReconChecked++
			dailyRows = append(dailyRows, DailyRow{
				Source:          row.Source,
				UsageDate:       row.UsageDate,
				Model:           row.Model,
				InputTokens:     row.InputTokens,
				OutputTokens:    row.OutputTokens,
				CacheReadTokens: row.CacheReadTokens,
				CacheWriteTokens: row.CacheCreationTokens,
				ReasoningTokens: row.ReasoningOutputTokens,
				CostUSD:         row.CostUSD,
			})
		ext.ReconSupplement++
	}
	ext.ReconSkipped = ext.ReconChecked - ext.ReconSupplement
	ext.rollupRows = dailyRows

	if maxDate > rollupDate {
		ext.rollupMaxDate = maxDate
	}

	return nil
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func ccSourceFromAppType(appType string) string {
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

func normalizeCCDate(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	parts := strings.Split(s, "/")
	if len(parts) == 3 && len(parts[0]) == 4 {
		return fmt.Sprintf("%s-%s-%s", parts[0], padNum(parts[1]), padNum(parts[2]))
	}
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

func isNoSuchTableErr(err error) bool {
	return err != nil && (strings.Contains(err.Error(), "no such table") ||
		strings.Contains(err.Error(), "does not exist"))
}
