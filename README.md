# Token Dashboard

> 本地优先的 AI Token 消耗看板 —— 离线分析、零上传、多工具聚合。

Token Dashboard 是一款桌面应用，直接读取本机 AI CLI 工具（Claude Code、Codex、Gemini CLI 等）的会话日志，聚合写入本地 SQLite 数据库，并结合 LiteLLM / OpenRouter 定价数据估算模型费用。

数据**全部存储在本地**，不联网、不上传、不依赖任何外部服务。

## ✨ 功能

- **多工具聚合** —— 同时追踪 7 款 AI 编程工具的使用数据
- **多维度统计** —— 按工具、模型、日期维度查看 Token 消耗和费用
- **趋势图表** —— ECharts 折线/柱状图展示每日/每小时用量变化，支持日/周/月/年粒度聚合
- **热力图** —— 按小时×星期的 Token 消耗密度热力图（最近一年窗口），点击日期可钻取小时级明细
- **模型排行** —— 按总用量排名的模型卡片列表，一目了然各模型消耗占比
- **模型详情** —— 单模型请求次数 / Token 三态 / 费用图表，7 档时间维度（今天/昨天/近7/近30/近90/自定义/全部），今天/昨天展示 24 小时分布
- **用量查询** —— 多供应商配额/余额实时查看（DeepSeek、BigModel 等）
- **跨设备合并** —— 设备 UUID 身份识别 + 用量数据导出/导入，可合并多台机器的用量统计
- **明暗主题** —— 亮色/暗色/跟随系统三态切换，图表与弹窗全量适配
- **自动采集** —— 定时自动同步（10 秒/30 秒/1 分钟/5 分钟/不同步，默认 30 秒）
- **定价更新** —— 一键从 LiteLLM / OpenRouter 拉取最新模型定价
- **CC-Switch 兼容** —— 支持从 CC-Switch 直接导入历史数据

## 📥 安装

### 直接下载

最新版本可从 [GitHub Releases](https://github.com/sparkle3021/token-usage/releases) 下载 Windows 可执行文件（`TokenUsage.exe`），解压后直接运行，无需安装环境。

### 从源码构建

**前置条件**：Go ≥ 1.25、Node.js ≥ 18、[Wails CLI](https://wails.io/docs/gettingstarted/installation)

```bash
git clone https://github.com/sparkle3021/token-usage.git
cd token-dashboard
cd frontend && npm install && cd ..
wails build
```

构建产物位于 `build/bin/` 目录。

### 开发模式

```bash
wails dev
```

前端支持 Vite 热更新，后端支持热重载。

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

## 🚀 快速上手

1. 启动应用，首次打开会自动扫描本机已安装的 AI 工具日志
2. 点击顶栏 **同步** 按钮采集数据（支持全量同步）
3. 在**看板**页查看 Token 趋势、热力图与 Top 模型
4. 在**模型排行**页查看模型用量排名，点击卡片进入**模型详情**（7 档时间维度切换）
5. 在**用量查询**页配置供应商（DeepSeek / BigModel）查看实时余额与配额
6. 开启**自动同步**（设置页，10 秒–5 分钟或不同步，默认 30 秒），数据持续更新
7. 多台机器合并用量：任一台导出数据 JSON，在另一台的**设置 → 维护**页导入即可合并

## ♻️ 数据重采（清空重建）

升级后若遇到历史数据日期错位、总量异常（如时区口径变更前的旧数据），可清空数据后全量重采：

1. 关闭应用，删除数据目录中的数据库（默认 `~/.token-usage/td.db`），或仅清空用量表：
   `daily_usage` / `hour_usage` / `time_usage` / `session_usage` / `parse_cache`
2. 重新启动应用，点击 **全量同步** 重新采集所有 AI 工具日志

> ⚠️ 清空后旧数据**不可回滚**，需从本机日志全量重采。数据以本机 AI 工具日志为唯一来源，重采即恢复全部统计。

## ⚙️ 配置

首次启动自动创建数据目录（默认 `~/.token-usage/`，可用环境变量 `DATA_DIR` 覆盖），内含数据库与日志，无需手动配置。

| 环境变量 | 默认值 | 说明 |
|---------|--------|------|
| `DATA_DIR` | `~/.token-usage` | 数据目录（数据库 + 日志 + 定价缓存） |
| `COLLECTOR_PARALLELISM` | `4` | 数据采集并发数 |

## 🛠 技术栈

[Wails v2](https://wails.io) · Go · [modernc.org/sqlite](https://modernc.org/sqlite) · React 19 · Vite 8 · Tailwind CSS v4 · [Ant Design v6](https://ant.design) · [ECharts](https://echarts.apache.org)

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

推送 `v*` 标签自动触发 CI：代码检查 → Windows 构建 → 发布 GitHub Release。

## 📄 License

[MIT](LICENSE)
