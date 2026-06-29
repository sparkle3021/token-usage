# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Commands

```bash
# Backend
wails dev       # Live development (Vite HMR + Go backend)
wails build     # Production build

# Frontend (inside frontend/)
npm run dev     # Vite dev server only (browser mode, port 5173)
npm run build   # Vite production build
npm run lint    # Run oxlint
npm run preview # Preview Vite production build

# Lint frontend
npx oxlint@latest --fix  # Auto-fix lint issues
```

## Tech Stack

- **Desktop Shell**: Wails v2 (Go → WebView2)
- **Backend**: Go 1.23+, Wails v2.12+
- **Database**: modernc.org/sqlite (pure Go SQLite, no CGO)
- **Frontend**: React 19, Vite 8, Tailwind CSS v4
- **UI**: shadcn/ui (based on @base-ui/react)
- **Charts**: Recharts (replaced ECharts from reference source project)
- **Linter**: oxlint (replaces ESLint)
- **Font**: Geist Variable

## Architecture

### Go Backend

Entry at `main.go`, Wails binds `App` struct methods as JS-callable APIs.

```
main.go          → Wails app config (title, size, asset embed)
app.go           → App struct: dashboard API + collection API bindings
internal/
  model/         → Data types: DailyUsage, SessionUsage, TimeUsage, CollectionRun, DashboardData
  database/      → SQLite manager: 4 tables, WAL pragmas, upsert/query functions
  pricing/       → Token cost engine: model resolution chain (exact → prefix → fuzzy → hardcoded overrides)
  collector/     → Collector interface + 6 implementations:
    claude_code.go   → Claude Code JSONL log parser (assistant turn dedup)
    codex.go         → Codex CLI JSONL parser (event type dispatch)
    gemini.go        → Gemini CLI JSON/JSONL parser
    others.go        → Hermes, OpenCode (SQLite), OpenClaw (JSONL)
    types.go         → Shared types (CollectResult, EventRow)
    util.go          → Common file discovery helpers
    engine/engine.go → Orchestration: sequential collector execution, per-collector transaction
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
  ui/              → shadcn/ui components (button, card, table, tabs, badge, select, popover, dialog, separator)
```

### Communication Pattern

Frontend calls Go backend via Wails runtime bindings:
- `window.go.main.App.GetDashboardData()` → returns `model.DashboardData` (daily + sessions + runs)
- `window.go.main.App.GetTimeSeriesData()` → returns `model.TimeSeriesData` (time-series)
- `window.go.main.App.StartCollection()` → async, returns `bool`
- `window.go.main.App.CollectStatus()` → returns `model.CollectStatus` (status/message/timestamps)

No HTTP API — Wails handles IPC bridge between WebView2 JS context and Go.

### Data Flow

1. **Collection**: Engine runs 6 collectors sequentially, each in its own transaction
2. **Pricing**: Each token event gets cost calculated via Pricing Engine (model → tier → rate)
3. **Storage**: Results upserted into 4 SQLite tables (daily_usage, session_usage, time_usage, collection_runs)
4. **Presentation**: Frontend calls GetDashboardData → filters by date/source/model → renders charts + tables

### Project Config

- `wails.json` — Wails project config (frontend install/build/dev watcher commands)
- `frontend/package.json` — npm dependencies and scripts
- `frontend/.oxlintrc.json` — oxlint config (React hooks + export rules)
- `frontend/vite.config.js` — `@/` path alias → `src/`, strictPort: false (auto-fallback), Tailwind CSS v4 plugin
