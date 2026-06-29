## 1. 项目基础设施

- [x] 1.1 创建 Go 包目录结构：`internal/database/`、`internal/pricing/`、`internal/collector/`、`internal/collector/engine/`、`internal/model/`
- [x] 1.2 添加 Go 依赖：`modernc.org/sqlite`（SQLite 驱动）
- [x] 1.3 复制定价缓存文件：将源项目的 `data/pricing-litellm.json`、`data/pricing-openrouter.json` 复制到本项目
- [x] 1.4 添加前端依赖：`recharts`、`lucide-react`（图标）、`class-variance-authority`（已有）、`tailwind-merge`（已有）

## 2. 数据库层

- [x] 2.1 定义数据模型结构体：`CollectionRun`、`DailyUsage`、`SessionUsage`、`TimeUsage`（在 `internal/model/` 中）
- [x] 2.2 实现数据库管理：SQLite 连接初始化、WAL 模式、busy_timeout、表创建、索引
- [x] 2.3 实现 `upsertDaily`：按 (device, source, usage_date, model) 合并，历史数据锁定定价
- [x] 2.4 实现 `upsertSession`：按 (device, source, session_id) 合并
- [x] 2.5 实现 `upsertTimeUsage` 和 `deleteTimeUsageForSource`：先删后插
- [x] 2.6 实现 `recordRun` 和 `pruneCollectionRuns`：记录采集运行，限制 500 条

## 3. 定价引擎

- [x] 3.1 实现 LiteLLM JSON 加载和解析函数
- [x] 3.2 实现 OpenRouter JSON 补充定价合并
- [x] 3.3 实现模型查找链：精确匹配 → 供应商前缀匹配 → Cursor 覆盖 → DeepSeek 覆盖 → 模糊匹配 → 零费用
- [x] 3.4 实现分档计价：根据 token 阈值（128K、200K、256K、272K）切换单价
- [x] 3.5 实现 `CalculateCost`：传入 model + token 分解 → 返回 USD 费用
- [x] 3.6 实现多级缓存：内存 map 缓存已解析的价格，避免重复模糊匹配
- [ ] 3.7 添加上游对齐测试：从源项目取典型模型 ID 列表，验证 Go 端输出价格与 JS 端一致

## 4. 采集器通用设施

- [x] 4.1 定义 `Collector` 接口和 `CollectResult` 结构体
- [x] 4.2 实现文件指纹缓存（parse-cache）：按 mtime+size 跳过未变 JSONL 文件
- [x] 4.3 实现通用工具函数：`localDateFromTimestamp`、`normalizeModelForGrouping`、`canonicalProvider`、`inferProviderFromModel`
- [x] 4.4 实现 JSONL 文件递归扫描工具函数

## 5. Claude Code 采集器

- [x] 5.1 实现目录扫描：`~/.claude/projects/`、`~/.config/claude/projects/`、macOS local-agent-mode-sessions
- [x] 5.2 实现 JSONL assistant turn 解析：提取 usage 字段，按 message.id+requestId 去重
- [x] 5.3 实现 token 聚合函数：按 (date, model) 和 (workspace, model) 分组
- [x] 5.4 返回标准化的 graphJson + modelsJson + eventsJson 结果

## 6. Codex CLI 采集器

- [x] 6.1 实现目录扫描：`~/.codex/sessions/`、`~/.codex/archived_sessions/`、headless roots
- [x] 6.2 实现 JSONL 三事件类型解析：session_meta、turn_context、event_msg(token_count)
- [x] 6.3 实现 token 增量推导：last_token_usage → total_token_usage delta → 上下文重置检测
- [x] 6.4 实现 token 标准化：cached_input 分离到 cacheRead
- [x] 6.5 返回标准化的 graphJson + modelsJson + eventsJson

## 7. Gemini CLI 采集器

- [x] 7.1 实现文件发现：扫描 `~/.gemini/tmp/` 的 JSON/JSONL 文件
- [x] 7.2 实现 session JSON 解析器：全格式 + 消息级 token 提取
- [x] 7.3 实现 headless stats 解析器：模型分解 + 数据扁平
- [x] 7.4 实现 JSONL 流解析：init/gemini/stats 事件处理 + 消息 ID 去重
- [x] 7.5 实现 cache-inclusive input 分离：根据 total 字段推断输入是否含缓存
- [x] 7.6 返回标准化的 graphJson + modelsJson + eventsJson

## 8. Hermes Agent 采集器

- [x] 8.1 实现 SQLite 数据库路径解析（`~/.hermes/state.db` / `$HERMES_HOME`）
- [x] 8.2 查询 Hermes DB 提取 daily + session + time 数据

## 9. OpenCode 采集器

- [x] 9.1 实现 SQLite 数据库路径解析（`~/.local/share/opencode/`）
- [x] 9.2 查询 OpenCode DB 提取 daily + session 数据

## 10. OpenClaw 采集器

- [x] 10.1 实现多 agent 根目录扫描（`~/.openclaw/agents/` 等）
- [x] 10.2 实现 JSONL 会话解析 + token 提取
- [x] 10.3 返回标准化的 graphJson + modelsJson + eventsJson

## 11. 采集编排引擎

- [x] 11.1 实现 `engine.Engine`：注册 6 个采集器，按顺序执行
- [x] 11.2 实现数据标准化：将各采集器输出转换为数据库 upsert 参数
- [x] 11.3 实现 per-collector 事务写入 + 错误隔离（一个采集器失败不影响其他）
- [x] 11.4 实现采集状态跟踪：idle / running / ok / error + 进度信息
- [x] 11.5 实现管理函数：`StartCollection`（异步）、`CollectionStatus`（查询）
- [x] 11.6 实现配置读取：扫描路径可被环境变量覆盖（`CLAUDE_CONFIG_DIR`、`CODEX_HOME` 等）
- [x] 11.7 实现定时采集调度器

## 12. Wails API 绑定

- [x] 12.1 在 `App` 结构体中集成 `database.Manager`、`engine.Engine`、`pricing.Engine`
- [x] 12.2 实现 `GetDashboardData()`：查询 daily/session/run 并返回
- [x] 12.3 实现 `GetTimeSeriesData()`：查询 time_usage 并返回
- [x] 12.4 实现 `StartCollection()`：触发异步采集 → 通过 Wails EventsEmit 推送进度
- [x] 12.5 实现 `CollectStatus()`：返回当前采集状态

## 13. 前端核心框架

- [x] 13.1 重写 `App.jsx`：数据加载、useState 状态管理、过滤衍生数据计算
- [x] 13.2 实现 Topbar 组件：品牌标识、采集按钮、刷新按钮、同步状态指示器
- [x] 13.3 实现 KPI 卡片行：6 个指标 + SVG sparkline + 上周期 delta

## 14. 前端筛选与过滤

- [x] 14.1 实现时间范围选择器：预设按钮（今天/7天/14天/30天/全部）+ 日历控件
- [x] 14.2 实现来源筛选 pill 组件：品牌图标 + 颜色标记
- [x] 14.3 实现设备/模型多选下拉组件
- [x] 14.4 实现上周期对比开关
- [x] 14.5 实现 CSV 导出功能

## 15. 前端图表（recharts + 自绘 SVG）

- [x] 15.1 实现趋势图（recharts）：堆叠柱状/折线/分组柱状三种模式 + 7日均线 + 上周期对比
- [x] 15.2 实现饼图（recharts）：来源占比 + 图例聚焦交互
- [x] 15.3 实现 Top Models 水平柱状条（HTML）
- [x] 15.4 实现缓存命中率仪表盘（自绘 SVG arc）
- [x] 15.5 实现热力图（自绘 CSS Grid）：日×小时分布
- [x] 15.6 实现环比面板：DoD/WoW/MoM 增长统计

## 16. 前端数据表格与 Drill-down

- [x] 16.1 实现可排序、可搜索的 DataTable 通用组件
- [x] 16.2 实现 4 标签页数据面板：Sources / Models / Sessions / Runs
- [x] 16.3 实现 DrillDrawer 侧边抽屉：KPI + Trend Spark + Token 分布
- [x] 16.4 实现 FilterContext 数据联动：聚焦某个来源时所有图表联动

## 17. 集成与收尾

- [x] 17.1 更新 `main.go` starter 回调：初始化 database、pricing、engine
- [x] 17.2 端到端测试：用源项目已有 SQLite 数据验证采集 → 入库 → 显示一致性
- [x] 17.3 清理：移除不再需要的空壳文件（`internal/util/util.go` 等）
- [x] 17.4 更新 README 和 CLI 使用说明

## 18. 源项目数据迁移

- [x] 18.1 将源项目的 `data/pricing-litellm.json` 复制到本项目
- [x] 18.2 将源项目的 `data/pricing-openrouter.json` 复制到本项目
- [x] 18.3 验证目标项目能正确加载定价数据并计算费用
