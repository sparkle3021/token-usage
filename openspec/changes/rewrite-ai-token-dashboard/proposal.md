## Why

将 ai-token-dashboard（Node.js + 纯 JS + SQLite + ECharts）的全部功能重写到当前 Wails v2（Go + React + shadcn/ui）技术栈，消除运行时对 Node.js 的依赖，利用 Wails 原生桌面能力提供更稳定的独立应用体验。

## What Changes

- 用 Go 重写 SQLite 数据库层（schema、upsert、查询接口）
- 用 Go 重写 LiteLLM/OpenRouter 定价引擎（含模型匹配、分档计价）
- 用 Go 重写采集编排系统（调度、并行执行、状态管理）
- 用 Go 重写 6 个 AI 工具数据采集器（Claude Code、Codex CLI、Gemini CLI、Hermes Agent、OpenClaw、OpenCode）
- 用 Wails Go 绑定暴露 API 给前端（数据查询、采集触发、状态轮询）
- 用 React + shadcn/ui + recharts 重写前端看板（KPI 卡片、趋势图、饼图、热力图、仪表盘、表格、drill-down drawer）
- 移除原项目的 HTTP 服务器、`node:sqlite`、ECharts 依赖
- 移除原项目的定价缓存更新脚本（改为 Go 嵌入式或按需下载）

## Capabilities

### New Capabilities

- `database`: SQLite 数据库 schema 定义、数据访问层（CRUD/upsert）、连接管理
- `pricing`: 基于 LiteLLM + OpenRouter 的模型定价匹配与 token 费用估算引擎
- `data-collection`: 采集编排入口，调度 6 个采集器，统一数据标准化与入库
- `claude-code-collector`: Claude Code JSONL 会话日志解析采集器
- `codex-collector`: Codex CLI JSONL 会话日志解析采集器
- `gemini-collector`: Gemini CLI 会话文件（JSON/JSONL）解析采集器
- `hermes-collector`: Hermes Agent SQLite 数据库读取采集器
- `opencode-collector`: OpenCode SQLite 数据库读取采集器
- `openclaw-collector`: OpenClaw JSONL 会话日志解析采集器
- `api-bindings`: Wails Go 结构体方法暴露给前端的 API 层（数据查询、采集控制）
- `dashboard`: React 前端看板（筛选栏、KPI 卡片、趋势图、饼图、热力图、仪表盘、数据表格、drill-down drawer、CSV 导出）

### Modified Capabilities

- 无（项目目前仅有空壳结构，无已有 capability）

## Impact

- **新增 Go 包**: `internal/database/`、`internal/pricing/`、`internal/collector/`（含 6 个子采集器）、`internal/collector/engine/`（编排）
- **修改前端**: 重写 `frontend/src/App.jsx`，新增 `components/`、添加 recharts 依赖，将 ECharts 图表替换为 recharts
- **构建配置**: `wails.json` 无需修改，`package.json` 添加 recharts 依赖
- **数据文件**: 需要从源项目复制 `data/pricing-litellm.json` 和 `data/pricing-openrouter.json` 到本项目
- **移除**: 不再需要 `npm run collect` / `npm run serve` / HTTP 服务器模式
