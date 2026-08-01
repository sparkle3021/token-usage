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
- **UI**: antd v6（ConfigProvider 驱动明暗主题，`App.useApp()` 提供 message）
- **Charts**: Recharts
- **Linter**: oxlint
- **Font**: Geist Variable

## Architecture

### Go Backend

Entry at `main.go`, Wails binds `App` struct methods as JS-callable APIs.

```
main.go          → Wails app config (title, size, asset embed) + setupLogging (5MB×3 轮转)
app.go           → App struct: 纯绑定转发（无 db/engine 裸引用，业务全部委托 service）
internal/
  model/         → 纯数据类型（无任何 internal 依赖）: DailyUsage, SessionUsage, TimeUsage,
  |                HourUsage, CollectionRun, DashboardData, QuotaData, AppConfig
  database/      → SQLite manager: 7 表 schema、WAL pragmas、prepared 复用、按表拆分 DAO
  pricing/       → Token cost engine: model resolution chain (exact → prefix → fuzzy → overrides)
  collector/     → Collector interface + 7 implementations:
    claude_code.go   → Claude Code JSONL log parser (assistant turn dedup)
    codex.go         → Codex CLI JSONL parser (event type dispatch)
    gemini.go        → Gemini CLI JSON/JSONL parser
    others.go        → Hermes (SQLite), OpenCode (SQLite), OpenClaw (JSONL)
    ccswitch.go      → CC-Switch external SQLite DB importer (proxy logs + rollups)
    types.go         → Shared types (CollectResult, CachePersistence, EventRow, etc.)
    paths.go         → 路径展开与 JSONL 发现（原 util.go 拆分）
    modelnorm.go     → 模型/供应商归一化（原 util.go 拆分）
    parse_cache.go   → 文件指纹缓存 ParseCache（原 util.go 拆分）
    convert.go       → 时间戳/数值转换 + Hostname（原 util.go 拆分）
    orchestrator/    → 并行采集编排（goroutine pool）+ 顺序事务写入 + checkpoint 管理
  quota/           → 用量查询独立域: provider 接口+registry + 3 供应商实现 + DAO + 业务编排
  service/         → 业务逻辑层: Dashboard/Collection/Setting/Import/Pricing
  config/          → 配置加载（DATA_DIR/COLLECTOR_PARALLELISM）+ CC-Switch 路径 helper
  assets/          → embed 默认定价数据（pricing-litellm/openrouter.json）
  debuglog/        → [perf] 探针开关（DEBUG_PERF=1 时输出）
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

### Database Schema (8 tables in SQLite)

| Table | PK | Purpose |
|---|---|---|
| `time_usage` | `(device, source, event_key)` | Per-request events from JSONL files |
| `hour_usage` | `(device, source, usage_date, hour, model)` | Hourly aggregates — **权威聚合层**（time_usage 汇总 + CC-Switch 直接写入） |
| `daily_usage` | `(device, source, usage_date, model)` | Daily totals, rebuilt from hour_usage after each collection |
| `session_usage` | `(device, source, session_id)` | Per-session rollups |
| `collection_runs` | `id` (auto-increment) | Sync history with status/message |
| `app_config` | `key` | Key-value config (CCSwitch checkpoints, DB path, auto-sync interval) |
| `parse_cache` | `(source, file_path)` | File fingerprint cache (`mtime:size` + last_parsed_offset) |
| `quota_configs` | `id` | 用量查询配置（provider/plan/display_name/seq/config_json） |

### Communication Pattern

Frontend calls Go backend via Wails runtime bindings (唯一出口 `frontend/src/api/client.js`):
- `window.go.main.App.GetDashboardData()` → returns `DashboardData`（daily + sessions + runs 全量）
- `window.go.main.App.GetTimeSeriesData(days)` → returns `TimeSeriesData`（days==1 时含 time_usage，否则仅 hour）
- `window.go.main.App.GetSessionsData()` → sessions tab 数据源（time_usage 按 session 聚合）
- `window.go.main.App.StartCollection()` / `StartFullCollection()` → 增量/全量采集
- `window.go.main.App.CollectStatus()` → 轮询采集进度
- `window.go.main.App.CurrentOp()` → 当前操作名（"" 空闲）
- `window.go.main.App.GetSettings()` / `SaveSettings(cfg)` → 设置读写
- `window.go.main.App.UpdatePricing()` → 从 LiteLLM 拉取定价并重载
- `window.go.main.App.ListQuotaConfigs()` 等 7 个 → 用量查询配置 CRUD + 余额拉取

No HTTP API — Wails handles IPC bridge between WebView2 JS context and Go.

### Data Flow

1. **Collection**: Engine runs 7 collectors in parallel goroutine pool (default 4, env `COLLECTOR_PARALLELISM`), then processes results sequentially
2. **Pricing**: Each token event gets cost calculated via Pricing Engine (model → tier → rate)
3. **Storage**: Results upserted into SQLite — JSONL goes `time_usage → hour_usage`, CCSwitch writes `hour_usage` directly
4. **Rebuild**: `BuildDailyFromHourUsage()` runs after all collectors, summarizing `hour_usage → daily_usage`
5. **Presentation**: Frontend calls `GetDashboardData` → filters by date/source/model → renders charts + tables
6. **权威聚合**: `hour_usage` 是权威层（time_usage 汇总 + CC-Switch 直接写入），`daily_usage` 由它重建；图表渲染 hour 优先、time 兜底（`TrendChart byKey` / `DashboardPage hourlySpark` 均此逻辑）

### Collection Architecture

- **File-based collectors** (Claude Code, Codex, Gemini, OpenClaw): parse JSONL files, use `ParseCache` for `mtime:size` fingerprint caching to skip unchanged files
- **SQLite-based collectors** (Hermes, OpenCode): read from their own SQLite databases directly
- **CC-Switch collector**: reads from external `cc-switch.db`, uses checkpoint-based incremental sync (`cc_switch_cursor_proxy_request_logs`, `cc_switch_rollup_max_date`)
- **Transaction protection**: each collector's writes are wrapped in a per-collector transaction; cache fingerprints are persisted only after successful write commit
- **Auto-sync**: configurable interval ticker in `CollectionService`, calls `StartCollection()` on each tick

### Frontend Conventions (关键行为约定)

- **明暗主题**: `html.dark` class 单点驱动（App.jsx），antd ConfigProvider algorithm + Tailwind `dark:` variant + 热力图 MutationObserver 均跟随
- **热力图**: 全量数据有意设计（不随过滤器变化）；窗口 = 最近一年（52 周，`DEFAULT_WEEKS`）；月份标签跳过末尾单列（GitHub 风格「最右为上月」）
- **弹窗**: antd Modal 统一；`centered` + 响应式宽度 + body flex 布局（标题固定、内容区 `.flex-dialog-body` 滚动）；body 有滚动条时 ScrollLocker 注入宽度补偿——页面滚动承载在内容容器（`overflow-y-auto scrollbar-none`）
- **message**: 必须经 `AntdApp.useApp()` 上下文获取（`lib/message.js` 单例注入），静态 `message.xxx` 不继承主题
- **请求竞态**: `fetchIdRef` 序号池丢弃过期响应（useDashboardData）

### Key Config

- `wails.json` — Wails project config (frontend install/build/dev watcher commands)
- `frontend/package.json` — npm dependencies and scripts
- `frontend/.oxlintrc.json` — oxlint config (React hooks + export rules)
- `frontend/vite.config.js` — `@/` path alias → `src/`, strictPort: false (auto-fallback), Tailwind CSS v4 plugin
- Env var `COLLECTOR_PARALLELISM` — controls collector goroutine pool size (default 4, max 16)
- Env var `DEBUG_PERF=1` — enables `[perf]` probe logs (off by default)
- Env var `DATA_DIR` — data directory override (default `~/.token-usage`)
