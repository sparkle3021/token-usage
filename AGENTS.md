# AGENTS.md

## Quick Start

```bash
# Kill old dev server first, then start in background
Get-Process -Name "TokenUsage","wails" -EA 0 | Stop-Process -Force
Start-Process -FilePath "wails" -ArgumentList "dev" -WindowStyle Hidden
# Tail Go logs (in another terminal): Get-Content "$env:USERPROFILE.token-usagelogsapp.log" -Follow

# Build (must succeed before dev; generates Go bindings)
wails build

# Run linter (frontend only)
cd frontend && npx oxlint@latest --fix

# Go checks
go vet ./...
go build ./...
```

**No tests exist** — do not look for test commands.

**CI**: `.github/workflows/ci.yml` — runs on `v*` tags: go vet + build (ubuntu) and frontend lint/build.

> **dev 重启陷阱**: wails dev 的文件 watcher 重启应用时，若旧实例仍在运行，单实例锁会拦住新实例并导致 dev 退出——改 Go 代码前先从托盘退出应用（或杀掉 TokenUsage 进程）。

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
go test ./...           # Run all tests (none exist)
go run .                # Run directly (without Wails build)

# Lint frontend
npx oxlint@latest --fix  # Auto-fix lint issues
```

## Tech Stack

- **Desktop shell**: Wails v2.12+ (Go → WebView2), Go 1.25
- **Database**: modernc.org/sqlite (pure Go SQLite, no CGO), WAL mode. DB at `~/.token-usage/td.db`, override via `DATA_DIR`
- **Frontend**: React 19 JSX (not TSX), Vite 8, Tailwind CSS v4 (`@import "tailwindcss"`, no tailwind.config.js)
- **UI**: antd v6（ConfigProvider 驱动明暗主题，`App.useApp()` 提供 message）
- **Charts**: Recharts
- **Linter**: oxlint（仅 rules-of-hooks + only-export-components 两个 React 规则）
- **Font**: Geist Variable

## Architecture

### Repository Layout

Root `package main` = 应用外壳（入口 + Wails 绑定 + 托盘/平台适配），业务全部在 internal。

```
根目录 (package main)
  main.go            入口：wails.Run 配置、单实例检查、启动编排
  app.go             App 结构 + 生命周期（NewApp/startup/shutdown）+ quitting/started 标志
  app_*.go           7 个 API 域绑定文件（dashboard/collection/settings/device/transfer/pricing/quota）
  tray.go            托盘（平台无关）：菜单 + showWindow/quitFromTray/onBeforeClose
  platform_*.go      平台适配：platform_windows.go（命名 Mutex/Event + ICO）/
                     platform_darwin.go（flock + unix socket + PNG 模板图标）
  logging.go         日志：setupLogging/logTee/rotateLog（5MB×3 轮转，双写 stderr 短路保护）
internal/            业务内核（Go internal 规则：外部不可 import）
  model/             纯数据类型（无任何 internal 依赖）
  database/          SQLite manager: 7 表 schema、WAL pragmas、prepared 复用、按表拆分 DAO
  pricing/           Token cost engine: model resolution chain (exact → prefix → fuzzy → overrides)
  collector/         Collector 接口 + 8 实现 + orchestrator/（goroutine 池并行编排 + 顺序事务写入）
  quota/             用量查询独立域: provider registry + 3 供应商实现 + DAO + 业务编排
  service/           业务逻辑层: Dashboard/Collection/Setting/Import/Pricing
  config/            配置加载（DATA_DIR/COLLECTOR_PARALLELISM）+ CC-Switch 路径 helper
  assets/            embed 默认定价数据 pricing-litellm.json（首次运行离线 seed 兜底）
  debuglog/          [perf] 探针开关（DEBUG_PERF=1 时输出）
frontend/            React 前端（wails build 时 embed 进二进制）
build/               Wails 构建产物（exe、图标）
```

### React Frontend (`frontend/`)

```
src/App.jsx                        → Root: 页面编排 + 主题 + FilterProvider + api 收敛
src/api/client.js                  → Wails 调用唯一出口（所有 window.go 必须经此）
src/hooks/                         → 数据逻辑: useDashboardData/useCollection/useSettings
src/store/filterStore.jsx          → 全局过滤器状态（Context + useReducer）
src/lib/                           → 纯函数: formatters/iconMap/filters/aggregators/message
src/components/
  common/       → 跨页面通用: SourceBadge/SourceIcon/MultiSelect/KPI/QuotaCard
  dialogs/      → 弹窗: SettingsDialog/QuotaDialog/HeatmapDrillDialog/SourceDrillDialog
  charts/       → 图表: TrendChart/SourceDonut/TopModels/Gauge/GrowthPanel + Heatmap/ 子目录
  layout/       → 布局: Header/FilterBar
src/pages/                         → 页面壳: DashboardPage/TablePage/QuotaPage（不承载弹窗）
```

### Architecture Conventions (分层规约)

- **Go 四层依赖方向**: `app`（绑定）→ `service`（业务）→ `domain`（database/collector/pricing/quota）→ `model`（纯数据）。依赖只指向下方，禁止 app 直连 db/engine 或执行 SQL
- **前端六层**: `pages` → `components` → `hooks` / `api`（唯一 window.go 出口）/ `lib` / `store`。组件禁止直接访问 `window.go`，必须经 `api/client.js`
- **日志**: 标准库 `log`（`[模块]` 前缀）唯一体系，输出到 `app.log`（5MB×3 轮转）；性能探针用 `debuglog.Perf`（`DEBUG_PERF=1` 生效）
- **文件组织**: 单文件不承载多类职责（>400 行且多职责须拆分）；工具函数不跨包重复实现；quota 相关代码必须位于 `internal/quota`
- **import 路径**: 前端统一 `@/` alias（vite 配置），禁止相对路径跨目录引用

## Collection System

8 collectors, run in parallel goroutine pool (default 4, env `COLLECTOR_PARALLELISM`), writes in per-collector transactions sequentially.

| Collector | Type | Data Path |
|-----------|------|-----------|
| Claude Code | JSONL | `~/.claude/projects/` (env `CLAUDE_CONFIG_DIR`) |
| Codex CLI | JSONL | `~/.codex/sessions/` (env `CODEX_HOME`) |
| Gemini CLI | JSONL | `~/.gemini/tmp/` |
| OpenClaw | JSONL | `~/.openclaw/agents/` 等 |
| DeepSeek Harness | JSONL | `~/.dsh/sessions/` (env `DSH_HOME`) |
| Hermes | SQLite | `~/.hermes/state.db` (env `HERMES_HOME`) |
| OpenCode | SQLite | `~/.local/share/opencode/` (env `OPENCODE_DATA_DIR`) |
| CC-Switch | SQLite | `~/.cc-switch/cc-switch.db` (external, **configurable in settings UI**) |

路径均为跨平台 `~/.xxx` 约定（macOS 一致）；CC-Switch 是唯一可在设置页配置的采集源。

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
| `hour_usage` | `(device,source,usage_date,hour,model)` | **权威聚合层** — MAX semantics for merge |
| `daily_usage` | `(device,source,usage_date,model)` | Rebuilt from hour_usage; cost locked for past dates |
| `session_usage` | `(device,source,session_id)` | Per-session rollups (unused by CC-Switch) |
| `app_config` | `key` | CK storage, CC-Switch DB path, auto-sync interval |
| `parse_cache` | `(source,file_path)` | `mtime:size` fingerprint cache |
| `quota_configs` | `id` | 用量查询配置（provider/plan/display_name/seq/config_json） |

## Communication Pattern

Frontend calls Go backend via Wails runtime bindings (唯一出口 `frontend/src/api/client.js`):
- `App.GetDashboardData()` → daily + sessions + runs 全量
- `App.GetHourSeries(days)` / `GetTodayEvents()` / `GetModelRanking()` / `GetModelSeries(model)` → 图表数据
- `App.StartCollection()` / `StartFullCollection()` / `CollectStatus()` / `CurrentOp()` → 采集控制
- `App.ClearAllData()` → 清库（含缓存失效）
- `App.SetAutoSyncInterval(minutes)` / `GetAutoSyncInterval()` → 自动同步
- `App.GetSettings()` / `SaveSettings(cfg)` → 设置读写
- `App.UpdatePricing()` / `ListModelPricing()` / `UpdateModelPricing(row)` / `DeleteModelPricing(key)` / `GetPricingMeta()` → 价格管理
- `App.ExportData()` / `ImportData()` / `DetectCCSwitchDB()` → 导入导出
- `App.GetDevices()` / `RenameDevice(id, name)` → 设备管理
- `App.ListQuotaConfigs()` 等 7 个 → 用量查询配置 CRUD + 余额拉取

No HTTP API — Wails handles IPC bridge between WebView2 JS context and Go.

## Data Flow

1. **Collection**: Engine runs 8 collectors in parallel goroutine pool (default 4, env `COLLECTOR_PARALLELISM`), then processes results sequentially
2. **Pricing**: Each token event gets cost calculated via Pricing Engine (model → tier → rate); price source = `model_pricing` 表（首次运行 embed seed 离线兜底，设置页可拉 LiteLLM 更新）
3. **Storage**: Results upserted into SQLite — JSONL goes `time_usage → hour_usage`, CCSwitch writes `hour_usage` directly
4. **Rebuild**: `BuildDailyFromHourUsage()` runs after all collectors, summarizing `hour_usage → daily_usage`
5. **Presentation**: Frontend calls `GetDashboardData` → filters by date/source/model → renders charts + tables
6. **权威聚合**: `hour_usage` 是权威层（time_usage 汇总 + CC-Switch 直接写入），`daily_usage` 由它重建；图表渲染 hour 优先、time 兜底（`TrendChart byKey` / `DashboardPage hourlySpark` 均此逻辑）
   - **daily 口径**: `daily_usage.usage_date` 一律为**本地日**（`BuildDailyFromHourUsage` 将 hour 的 UTC 桶按本机时区平移后聚合；Hermes/CC-Switch rollup 直写亦按本地日），服务层不再二次平移（`localizeDate` 仅限未本地化的 UTC 日来源）

## Collection Architecture

- **File-based collectors** (Claude Code, Codex, Gemini, OpenClaw, DSH): parse JSONL files, use `ParseCache` for `mtime:size` fingerprint caching to skip unchanged files
- **SQLite-based collectors** (Hermes, OpenCode): read from their own SQLite databases directly
- **CC-Switch collector**: reads from external `cc-switch.db`, uses checkpoint-based incremental sync (`cc_switch_cursor_proxy_request_logs`, `cc_switch_rollup_max_date`)
- **Transaction protection**: each collector's writes are wrapped in a per-collector transaction; cache fingerprints are persisted only after successful write commit
- **Auto-sync**: configurable interval ticker in `CollectionService` (default 30s, configurable in settings), calls `StartCollection()` on each tick — first collection happens ~30s after first launch

## Frontend Conventions (关键行为约定)

- **明暗主题**: `html.dark` class 单点驱动（App.jsx），antd ConfigProvider algorithm + Tailwind `dark:` variant + 热力图 MutationObserver 均跟随
- **热力图**: 全量数据有意设计（不随过滤器变化）；窗口 = 最近一年（52 周，`DEFAULT_WEEKS`）；月份标签跳过末尾单列（GitHub 风格「最右为上月」）
- **弹窗**: antd Modal 统一；`centered` + 响应式宽度 + body flex 布局（标题固定、内容区 `.flex-dialog-body` 滚动）；body 有滚动条时 ScrollLocker 注入宽度补偿——页面滚动承载在内容容器（`overflow-y-auto scrollbar-none`）
- **message**: 必须经 `AntdApp.useApp()` 上下文获取（`lib/message.js` 单例注入），静态 `message.xxx` 不继承主题
- **请求竞态**: `fetchIdRef` 序号池丢弃过期响应（useDashboardData）

## Key Conventions

- **`totalTokens`** = `input + output + cacheRead + cacheWrite` (NO `+ reasoning` — API `output_tokens` already includes thinking tokens)
- **`deltaPct` returns `null`** when `prev === 0` (no badge rendered for no-prior-data cases)
- **Model icon SVGs**: `frontend/src/assets/models/`. Matching via `\b`-anchored prefix/keyword regex.
- **CSS**: Tailwind CSS v4 with `@import "tailwindcss"` (no `tailwind.config.js`)
- **oxlint** only runs two React rules: `rules-of-hooks` and `only-export-components`
- **SourceBadge color** comes from `iconMap.js` `getSourceColor()` → oklch palette. Badge uses `color-mix(in oklch, ${color} 15%, transparent)` for semi-transparent background.
- **Collected data is local only** — no network upload. Database stored in `~/.token-usage/td.db`.

## Platform Notes（托盘 / 单实例）

- **托盘**: 点窗口 × → 隐藏到托盘常驻（`onBeforeClose` 拦截 + `WindowHide`）；托盘菜单：显示主窗口 / 立即采集 / 退出
- **退出链（关键）**: Wails 的 `runtime.Quit` 也会先调用 `OnBeforeClose`——托盘"退出"必须先置 `quitting` 标志（atomic.Bool）放行，否则返回 true 会把退出也拦成隐藏，应用永远退不掉
- **单实例**: Windows 命名 Mutex + 命名 Event 激活已有窗口；macOS `flock` 文件锁 + unix socket 激活（`platform_*.go` 各自实现，main.go 调用签名一致）
- **托盘图标**: Windows 用 `build/windows/icon.ico`（ICO），macOS 用 `build/appicon.png`（模板图标）——embed 在 `platform_*.go`，`setupTrayIcon()` 封装平台差异
- **macOS 验证边界**: systray 的 mac 实现含 cgo，Windows 上无法交叉编译 darwin 分支，须在 mac 环境验证

## Key Config

- `wails.json` — Wails project config (frontend install/build/dev watcher commands, productVersion 0.4.0)
- `frontend/package.json` — npm dependencies and scripts
- `frontend/.oxlintrc.json` — oxlint config (React hooks + export rules)
- `frontend/vite.config.js` — `@/` path alias → `src/`, strictPort: false (auto-fallback), Tailwind CSS v4 plugin
- Env var `COLLECTOR_PARALLELISM` — controls collector goroutine pool size (default 4, max 16)
- Env var `DEBUG_PERF=1` — enables `[perf]` probe logs (off by default)
- Env var `DATA_DIR` — data directory override (default `~/.token-usage`)
