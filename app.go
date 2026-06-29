package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"token-dashboard/internal/collector/engine"
	"token-dashboard/internal/database"
	"token-dashboard/internal/model"
	"token-dashboard/internal/pricing"
)

type App struct {
	ctx     context.Context
	db      *database.Manager
	pricing *pricing.Engine
	engine  *engine.Engine
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
