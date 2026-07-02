# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Commands

```bash
# Backend
wails dev       # Live development (Vite HMR + Go backend, file watcher)
wails build     # Production build (embeds frontend assets into Go binary)

# Frontend (inside frontend/)
npm run dev     # Vite dev server only (browser mode, port 5173)
npm run build   # Vite production build
npm run lint    # Run oxlint
npm run preview # Preview Vite production build

# Go (project root)
go build ./...          # Build all Go packages
go vet ./...            # Static analysis
go test ./...           # Run all tests
go run .                # Run directly (without Wails build)

# Lint frontend
npx oxlint@latest --fix  # Auto-fix lint issues
```

## Tech Stack

- **Desktop Shell**: Wails v2 (Go → WebView2)
- **Backend**: Go 1.23+, Wails v2.12+
- **Database**: modernc.org/sqlite (pure Go SQLite, no CGO)
- **Frontend**: React 19, Vite 8, Tailwind CSS v4
- **UI**: shadcn/ui (based on @base-ui/react)
- **Charts**: Recharts
- **Linter**: oxlint
- **Font**: Geist Variable

## Architecture

### Go Backend

Entry at `main.go`, Wails binds `App` struct methods as JS-callable APIs.

```
main.go          → Wails app config (title, size, asset embed)
app.go           → App struct: dashboard API + collection + settings API bindings
internal/
  model/         → Data types: DailyUsage, SessionUsage, TimeUsage, HourUsage,
  |                CollectionRun, DashboardData, CCSwitchStats
  database/      → SQLite manager: 5 tables, WAL pragmas, bulk upsert/query functions
  pricing/       → Token cost engine: model resolution chain (exact → prefix → fuzzy → hardcoded overrides)
  collector/     → Collector interface + 7 implementations:
    claude_code.go   → Claude Code JSONL log parser (assistant turn dedup)
    codex.go         → Codex CLI JSONL parser (event type dispatch)
    gemini.go        → Gemini CLI JSON/JSONL parser
    others.go        → Hermes (SQLite), OpenCode (SQLite), OpenClaw (JSONL)
    ccswitch.go      → CC-Switch external SQLite DB importer (proxy logs + rollups)
    types.go         → Shared types (CollectResult, CachePersistence, EventRow, etc.)
    util.go          → File fingerprint caching (ParseCache), JSONL discovery, model normalization
    engine/engine.go → Orchestration: parallel collect (goroutine pool), sequential write (per-collector transaction)
    engine/persist.go → DB persistence adapter for ParseCache fingerprints
```

### React Frontend (`frontend/`)

```
src/App.jsx                        → Root: dashboard layout, filter state, KPI grid, chart grid
src/lib/utils.js                   → Color palette, formatting, date helpers, filtering, CSV export
src/components/
  charts/
    TrendChart.jsx  → Recharts Bar + Line (stacked/overlap modes)
    SourceDonut.jsx → Recharts Pie (innerRadius donut, click-to-focus)
    TopModels.jsx   → HTML bar chart (top-N models by tokens)
    Gauge.jsx       → SVG arc (cache hit rate gauge)
    Heatmap.jsx     → CSS Grid (time-of-day heatmap)
    GrowthPanel.jsx → DoD/WoW/MoM growth stats
  tables/
    TablePanel.jsx  → Sortable/searchable tables (4 tabs: sources, models, sessions, runs)
    DrillDrawer.jsx → Slide-in drawer for detail drill-down
  dialogs/
    SettingsDialog.jsx → CC-Switch DB path, auto-sync interval, pricing overrides
    ImportDialog.jsx   → Manual CCSwitch import trigger
  ui/              → shadcn/ui components (button, card, table, tabs, badge, select, popover, dialog, separator)
```

### Database Schema (5 tables in SQLite)

| Table | PK | Purpose |
|---|---|---|
| `time_usage` | `(device, source, event_key)` | Per-request events from JSONL files |
| `hour_usage` | `(device, source, usage_date, hour, model)` | Hourly aggregates (MAX semantics for merge) |
| `daily_usage` | `(device, source, usage_date, model)` | Daily totals (cost locked for past dates) |
| `session_usage` | `(device, source, session_id)` | Per-session rollups |
| `collection_runs` | `id` (auto-increment) | Sync history with status/message |
| `app_config` | `key` | Key-value config (CCSwitch checkpoints, DB path) |
| `parse_cache` | `(source, file_path)` | File fingerprint cache (`mtime:size`) |

### Communication Pattern

Frontend calls Go backend via Wails runtime bindings:
- `window.go.main.App.GetDashboardData(filter)` → returns `DashboardData`
- `window.go.main.App.GetTimeSeriesData(filter)` → returns `TimeSeriesData`
- `window.go.main.App.StartCollection()` → triggers async collection (all 7 collectors)
- `window.go.main.App.CollectStatus()` → polls collection progress
- `window.go.main.App.ImportCCSwitchDB()` → manual CCSwitch full re-import
- `window.go.main.App.SetAutoSyncInterval(minutes)` → periodic auto-sync

No HTTP API — Wails handles IPC bridge between WebView2 JS context and Go.

### Data Flow

1. **Collection**: Engine runs 7 collectors in parallel goroutine pool (default 4, env `COLLECTOR_PARALLELISM`), then processes results sequentially
2. **Pricing**: Each token event gets cost calculated via Pricing Engine (model → tier → rate)
3. **Storage**: Results upserted into SQLite — JSONL goes `time_usage → hour_usage`, CCSwitch writes `hour_usage` directly
4. **Rebuild**: `BuildDailyFromHourUsage()` runs after all collectors, summarizing `hour_usage → daily_usage`
5. **Presentation**: Frontend calls `GetDashboardData` → filters by date/source/model → renders charts + tables

### Collection Architecture

- **File-based collectors** (Claude Code, Codex, Gemini, OpenClaw): parse JSONL files, use `ParseCache` for `mtime:size` fingerprint caching to skip unchanged files
- **SQLite-based collectors** (Hermes, OpenCode): read from their own SQLite databases directly
- **CC-Switch collector**: reads from external `cc-switch.db`, uses checkpoint-based incremental sync (`cc_switch_cursor_proxy_request_logs`, `cc_switch_rollup_max_date`)
- **Transaction protection**: each collector's writes are wrapped in a per-collector transaction; cache fingerprints are persisted only after successful write commit
- **Auto-sync**: configurable interval ticker in `app.go`, calls `StartCollection()` on each tick

### Key Config

- `wails.json` — Wails project config (frontend install/build/dev watcher commands)
- `frontend/package.json` — npm dependencies and scripts
- `frontend/.oxlintrc.json` — oxlint config (React hooks + export rules)
- `frontend/vite.config.js` — `@/` path alias → `src/`, strictPort: false (auto-fallback), Tailwind CSS v4 plugin
- Env var `COLLECTOR_PARALLELISM` — controls collector goroutine pool size (default 4, max 16)
