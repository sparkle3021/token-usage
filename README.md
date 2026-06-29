# Token Dashboard

个人 AI Token 消耗看板，基于 Wails v2 构建。直接读取本机会话日志，聚合写入本地 SQLite，估算各模型费用。

## 技术栈

- **桌面框架**: Wails v2 (Go + WebView2)
- **后端**: Go 1.25, modernc.org/sqlite
- **前端**: React 19, Vite 8, Tailwind CSS v4, shadcn/ui
- **图表**: Recharts

## 开发

```bash
# 启动开发模式（Vite HMR + Go 后端热重载）
wails dev

# 单独启动前端开发服务器
cd frontend && npm run dev

# 构建生产包
wails build
```

## 采集用法

启动应用后，点击右上角「采集」按钮读取本机以下 AI 工具的使用数据：

| 工具 | 数据位置 |
|------|---------|
| Claude Code | `~/.claude/projects/` |
| Codex CLI | `~/.codex/sessions/` |
| Gemini CLI | `~/.gemini/tmp/` |
| Hermes Agent | `~/.hermes/state.db` |
| OpenCode | `~/.local/share/opencode/` |
| OpenClaw | `~/.openclaw/agents/` |

采集的数据写入 `data/usage.sqlite`，不联网、不上传。

## 数据目录

| 路径 | 说明 |
|------|------|
| `data/usage.sqlite` | SQLite 数据库 |
| `data/pricing-litellm.json` | LiteLLM 定价缓存 |
| `data/pricing-openrouter.json` | OpenRouter 定价缓存 |

设 `DATA_DIR` 环境变量可自定义数据目录。
