# AGENTS.md

## Quick Start

```bash
# Kill old dev server first, then start in background
Get-Process -Name "TokenUsage","wails" -EA 0 | Stop-Process -Force
Start-Process -FilePath "wails" -ArgumentList "dev" -WindowStyle Hidden
# Tail Go logs (in another terminal): Get-Content "$env:USERPROFILE\.token-usage\logs\app.log" -Follow

# Build (must succeed before dev; generates Go bindings)
wails build

# Run linter (frontend only)
cd frontend && npx oxlint@latest --fix

# Go checks
go vet ./...
go build ./...
```

**No tests exist** — do not look for test commands.

**No CI workflows** — no `.github/` directory.

## Architecture

- **Desktop shell**: Wails v2 (Go → WebView2). Entry: `main.go` binds `App` struct methods as `window.go.main.App.*`
- **Go backend**: `app.go` (thin forwarding layer, ~190 lines), `internal/service/` (business logic), `internal/orchestrator/` (collection orchestration), `internal/database/` (SQLite DAO split by table), `internal/config/` (env vars)
- **Frontend**: React 19 JSX (not TSX), Vite 8, Tailwind CSS v4, shadcn/ui (base-nova style, `@/` alias), Recharts
- **Database**: `~/.token-usage/td.db` (SQLite, WAL mode, `modernc.org/sqlite` no CGO). Override via `DATA_DIR` env.
- **Pricing engine**: model resolution chain = exact → prefix → fuzzy → hardcoded override. Seed data bundled via `internal/config/seed/`.

### App Container Height Constraint

App root (`App.jsx:44`) **must** have `flex flex-col min-h-screen` for any child `flex-1 min-h-0` chain to work (e.g. scrollable table panels). Without it, flex-1 in height direction is a no-op because no parent allocates remaining space.

## Collection System

7 collectors, run in parallel goroutine pool (default 4, env `COLLECTOR_PARALLELISM`), writes in per-collector transactions sequentially.

| Collector | Type | Data Path |
|-----------|------|-----------|
| Claude Code | JSONL | `~/.claude/projects/` |
| Codex CLI | JSONL | `~/.codex/sessions/` |
| Gemini CLI | JSONL | `~/.gemini/tmp/` |
| OpenClaw | JSONL | `~/.openclaw/agents/` |
| Hermes | SQLite | `~/.hermes/state.db` |
| OpenCode | SQLite | `~/.local/share/opencode/` |
| CC-Switch | SQLite | `~/.cc-switch/cc-switch.db` (external, configurable) |

### CC-Switch Checkpoint Timing (Fragile)

**Critical**: Checkpoints are NOT saved inside `Collect()`. They are **staged** in struct fields and only persisted via `SavePendingCheckpoints()` **after** `processCollector()` (SQL write transaction) succeeds.

Stale CK detection runs at startup: if CK exists but `source='CC-Switch'` rows are 0 in both `daily_usage` and `hour_usage`, CKs are auto-cleared for full re-sync.

## Data Pipeline

```
JSONL collectors → time_usage → BuildHourUsageFromTimeUsage → hour_usage
CC-Switch proxy  → hour_usage (direct)
hour_usage       → BuildDailyFromHourUsage → daily_usage (SUM + MAX merge)
```

Three-layer data merge for TrendChart/Heatmap: `timeRows → hourRows → dailyRows` fallback.

## Database

Files: `internal/database/` — split by table (same `package database`).

| Table | PK | Notes |
|-------|-----|-------|
| `time_usage` | `(device,source,event_key)` | Per-request events (JSONL collectors) |
| `hour_usage` | `(device,source,usage_date,hour,model)` | MAX semantics for merge |
| `daily_usage` | `(device,source,usage_date,model)` | Cost locked for past dates |
| `session_usage` | `(device,source,session_id)` | Per-session rollups (unused by CC-Switch) |
| `app_config` | `key` | CK storage, CC-Switch DB path, settings |
| `parse_cache` | `(source,file_path)` | `mtime:size` fingerprint cache |
| `collection_runs` | `id` | Sync history (backend only; frontend UI removed) |

## Key Conventions

- **`totalTokens`** = `input + output + cacheRead + cacheWrite` (NO `+ reasoning` — API `output_tokens` already includes thinking tokens)
- **`deltaPct` returns `null`** when `prev === 0` (no badge rendered for no-prior-data cases)
- **Model icon SVGs**: `frontend/src/assets/models/`. Matching via `\b`-anchored prefix/keyword regex.
- **CSS**: Tailwind CSS v4 with `@import "tailwindcss"` (no `tailwind.config.js`)
- **No HTTP API** — Wails IPC bridge. Frontend calls `window.go.main.App.*`.
- **oxlint** only runs two React rules: `rules-of-hooks` and `only-export-components`
- **SourceBadge color** comes from `iconMap.js` `getSourceColor()` → oklch palette. Badge uses `color-mix(in oklch, ${color} 15%, transparent)` for semi-transparent background.
- **Collected data is local only** — no network upload. Database stored in `~/.token-usage/td.db`.
