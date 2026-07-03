# Token Dashboard

> 本地优先的 AI Token 消耗看板 —— 离线分析、零上传、多工具聚合。

Token Dashboard 是一款桌面应用，直接读取本机 AI CLI 工具（Claude Code、Codex、Gemini CLI 等）的会话日志，聚合写入本地 SQLite 数据库，并结合 LiteLLM / OpenRouter 定价数据估算模型费用。

数据**全部存储在本地**，不联网、不上传、不依赖任何外部服务。

## ✨ 功能

- **多工具聚合** —— 同时追踪 7 款 AI 编程工具的使用数据
- **多维度统计** —— 按工具、模型、日期维度查看 Token 消耗和费用
- **趋势图表** —— 折线图展示每日/每小时用量变化趋势
- **热力图** —— 按小时×星期的 Token 消耗密度热力图
- **会话详情** —— 查看每次 AI 对话的 Token 用量明细
- **自动采集** —— 支持设置定时自动同步（1–60 分钟间隔）
- **定价更新** —— 一键从 LiteLLM / OpenRouter 拉取最新模型定价
- **CC-Switch 兼容** —— 支持从 CC-Switch 直接导入历史数据

## 📊 支持的 AI 工具

| 工具 | 数据格式 | 数据路径 |
|------|---------|---------|
| Claude Code | JSONL | `~/.claude/projects/` |
| Codex CLI | JSONL | `~/.codex/sessions/` |
| Gemini CLI | JSONL | `~/.gemini/tmp/` |
| OpenCode | SQLite | `~/.local/share/opencode/` |
| OpenClaw | JSONL | `~/.openclaw/agents/` |
| Hermes Agent | SQLite | `~/.hermes/state.db` |
| CC-Switch | SQLite | `~/.cc-switch/cc-switch.db`（外部导入） |

## 🛠 技术栈

| 层 | 技术 |
|----|------|
| 桌面框架 | [Wails v2](https://wails.io) (Go + WebView2) |
| 后端 | Go 1.25 + [modernc.org/sqlite](https://modernc.org/sqlite)（纯 Go，无 CGO） |
| 前端 | React 19 + Vite 8 + Tailwind CSS v4 |
| UI 组件 | [shadcn/ui](https://ui.shadcn.com) (base-nova 风格) |
| 图表 | [Recharts](https://recharts.org) |
| 定价数据 | LiteLLM + OpenRouter 模型定价 |

## 📦 安装

### 从源码构建

**前置条件**：Go ≥ 1.25、Node.js ≥ 18、[Wails CLI](https://wails.io/docs/gettingstarted/installation)

```bash
# 克隆仓库
git clone https://github.com/sparkle3021/token-usage.git
cd token-dashboard

# 安装前端依赖并构建
cd frontend && npm install && cd ..

# 构建桌面应用（自动生成 Go bindings + 打包前端）
wails build
```

构建产物位于 `build/bin/` 目录。

### 直接运行（开发模式）

```bash
wails dev
```

启动后前端支持 Vite HMR 热更新，Go 后端支持热重载。

## ⚙️ 配置

通过环境变量配置：

| 环境变量 | 默认值 | 说明 |
|---------|--------|------|
| `DATA_DIR` | `~/.token-dashboard` | 数据目录（数据库 + 日志 + 定价缓存） |
| `COLLECTOR_PARALLELISM` | `4` | 采集并发数 |

首次启动时，应用会自动在数据目录下创建 SQLite 数据库和默认定价文件，无需额外配置。

### 数据目录结构

```
~/.token-dashboard/
├── td.db                   # SQLite 数据库（WAL 模式）
├── logs/
│   └── app.log             # 应用日志
└── config/
    ├── pricing-litellm.json     # LiteLLM 定价缓存
    └── pricing-openrouter.json  # OpenRouter 定价缓存
```

## 🏗 架构

```
┌─────────────────────────────────────────────────┐
│                    Frontend                      │
│         React 19 + shadcn/ui + Recharts         │
│            window.go.main.App.* (IPC)           │
└──────────────────────┬──────────────────────────┘
                       │ Wails IPC Bridge
┌──────────────────────▼──────────────────────────┐
│                   app.go                        │
│              (方法绑定 & 转发)                    │
├─────────────────────────────────────────────────┤
│              internal/service/                   │
│   DashboardService  CollectionService  ...       │
├─────────────────────────────────────────────────┤
│     internal/orchestrator/   internal/database/  │
│     (并发采集调度)             (SQLite DAO)       │
└─────────────────────────────────────────────────┘
```

### 数据流

```
JSONL 采集器 → time_usage → BuildHourUsageFromTimeUsage → hour_usage
CC-Switch   → hour_usage（直接写入）
hour_usage  → BuildDailyFromHourUsage → daily_usage（SUM + MAX 合并）
```

三条数据管线在趋势图/热力图渲染时按 `timeRows → hourRows → dailyRows` 优先级逐层回退。

## 🔒 隐私

- **数据完全本地** —— 所有 Token 消耗数据存储在本地 SQLite 文件，不经过任何网络传输
- **定价数据** —— 仅通过「更新定价」按钮手动拉取，不自动联网
- **无遥测** —— 不收集任何使用数据、不发送任何统计信息

## 📝 开发

```bash
# 前端代码检查
cd frontend && npx oxlint@latest --fix

# Go 代码检查
go vet ./...
go build ./...

# 构建
wails build
```

项目没有测试用例和 CI 流水线，当前处于早期开发阶段。

## 📄 License

[MIT](LICENSE)
