## Context

当前项目是 Wails v2 初始模板，Go 后端仅有空壳（`internal/service/`、`internal/model/`、`internal/util/` 均为空包），前端是 React 19 + shadcn/ui 最小骨架。需要将 ai-token-dashboard 的全部功能移植过来。

源项目采用 Node.js 22 + 纯 JS HTTP 服务器 + ECharts + 手写 CSS，与目标栈差异显著。

## Goals / Non-Goals

**Goals:**
- 完整的 SQLite 数据库 schema（4 张表）与数据访问层
- 与源项目行为一致的 LiteLLM + OpenRouter 定价引擎（精度对齐，含分档计价）
- 6 个 AI 工具采集器，使用与原项目等效的解析逻辑
- 采集编排系统（串行执行各采集器、事务性入库、状态上报）
- 通过 Wails Bind 暴露的 Go API（数据查询、采集触发、状态轮询）
- React 前端看板保留全部功能（筛选栏、KPI、趋势图、饼图、表格、drill-down drawer、CSV 导出、热力图、仪表盘、环比面板）
- 源项目的定时采集功能
- 源项目的多设备支持（device 标签，session_usage 含 project_path）

**Non-Goals:**
- 不实现多设备汇聚（ingest hub / Docker 部署）
- 不实现订阅额度查询（subscription quota，涉及厂商 OAuth）
- 不实现复盘视图（review page，原项目有但调用频率低）
- 不保留原项目的 HTTP API 模式（全部改为 Wails IPC 调用）
- 不保留定价缓存在线更新脚本（改为随仓库提供静态 JSON）

## Decisions

### 1. 图表库：recharts 替代 ECharts
- **原因**: ECharts 体积大（~1MB），在 Wails WebView2 中有渲染兼容风险；recharts 基于 React 组件化设计，与 shadcn/ui 生态一致，Tree-shaking 后体积可控
- **代价**: 热力图、仪表盘需自绘 SVG（recharts 不直接支持），参考原项目的 SVG 实现思路

### 2. Go SQLite 驱动：modernc.org/sqlite（纯 Go）
- **原因**: Wails 跨平台构建时 CGO 会引入大量兼容性问题；modernc.org/sqlite 是纯 Go 实现，与 `go:build` tag 无关
- **备选**: mattn/go-sqlite3（需 CGO，构建复杂），已排除

### 3. 采集器接口设计：Go interface + 注册模式
- **原因**: 6 个采集器共享相同的生命周期（scan → parse → normalize → upsert），interface 确保一致性；注册模式让 engine 包无需显式 import 每个采集器
- **定义**:
  ```go
  type Collector interface {
      ID() string
      Collect(ctx context.Context, pricing *pricing.Engine) (*CollectResult, error)
  }
  ```

### 4. 定价引擎架构：静态数据 + 运行时缓存
- **原因**: LiteLLM JSON（~2000 模型）在每次采集时重复解析代价高；Go 端在进程启动时加载到内存 map 中，运行时 Lookup 用 sync.Map 做二级缓存
- **与原项目差异**: 不支持 `PRICING_REFRESH=1` 运行时更新（桌面应用场景不需要）

### 5. 前端状态管理：React useState + useMemo（不引入状态库）
- **原因**: 数据流是单向的（API fetch → filter → render），无复杂交互状态；源项目也只用 useState/useMemo，多加 Zustand 等没有收益

### 6. 采集状态传递：Go callback → Wails EventsEmit
- **原因**: 采集是异步后台操作，前端需要实时状态更新。Wails 的 `EventsEmit` 机制允许 Go 后端推送事件到前端，无需轮询
- **实现**: 采集引擎启动后通过 `context.Context` 传入 Wails runtime，各阶段通过 `runtime.EventsEmit` 推送进度

### 7. 解析缓存：Go 版本的文件指纹缓存
- **原因**: 源项目的 parse-cache 按文件的 mtime+size 决定是否重解析，该模式简单且有效。Go 端用 `os.Stat` 获取相同信息，缓存在内存 map 中，采集结束时写入 JSON 文件持久化

## Risks / Trade-offs

| 风险 | 缓解措施 |
|------|---------|
| 定价引擎模型匹配精度与原项目不一致 | 为 pricing 编写独立的 Go 测试，从源项目 JS 取典型模型 ID 和预期价格做对照 |
| 采集器 JSONL 解析逻辑与源项目行为偏差 | 每个采集器在 Go 中实现后，找一份真实数据做端到端验证 |
| recharts 无法完全复现 ECharts 的交互效果（tooltip/zoom） | 趋势图保留 dataZoom 等效功能使用 recharts Brush/ReferenceArea；热力图和仪表盘自绘 SVG |
| shadcn/ui 默认浅色主题与源项目的暖色主题差异大 | 通过 CSS 变量覆盖适配，而非大改组件 |
