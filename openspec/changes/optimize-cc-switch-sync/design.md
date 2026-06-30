## Context

当前 CC-Switch 数据导入实现在 `app.go:ImportCCSwitchDB()` 中，工作方式：
1. 直接打开 CC-Switch SQLite 数据库（路径存于 `app_config.cc_switch_db_path`）
2. 全量扫描 `proxy_request_logs` 表，按 `(date, hour, source, model)` 聚合
3. 写入本地 `hour_usage` 表，然后调用 `BuildDailyFromHourUsage()` 重建日表

**存在的问题：**
- 全量扫描 → 每次导入都读取整个 `proxy_request_logs` 表（数据量随使用时间线性增长）
- 仅依赖 `proxy_request_logs` → 该表仅存当月数据，无法获取历史用量
- 无增量机制 → 每次导入都在重复处理已导入的旧数据
- CSV 导入（`ImportCSV`）仅支持手动操作，未与自动同步集成

**已有基础设施：**
- `app_config` 表可用于持久化 checkpoint 值
- `hour_usage` 已有 MAX 语义 upsert，支持多源共存
- `collection_runs` 表已有 `last_file_mtime` 字段（用于 JSONL collector 增量模式）

## Goals / Non-Goals

**Goals：**
- 为 `proxy_request_logs` 实现增量同步（checkpoint cursor），首次全量后仅拉取增量
- 新增 `usage_daily_rollups` 数据源导入，按日聚合补充历史数据（本月前的数据）
- 历史数据对账机制：检测本地缺失的日期，自动从 rollup 表补全
- 向后兼容：现有全量导入功能保留为"强制全量同步"入口
- 同步性能：后续增量同步减少 80-90% 处理量

**Non-Goals：**
- 不修改前端展示逻辑（数据更完整但展示不变）
- 不修改 CC-Switch 数据库结构（只读访问）
- 不处理 `proxy_request_logs` 的删除/更新场景（数据仅追加）
- 不做分布式或多设备 checkpoint 同步

## Decisions

### Decision 1: Checkpoint 存储位置 — 使用 `app_config` 表

**选择：** 复用 `app_config` 表（key-value），以 `cc_switch_cursor_<table>` 为 key 存储 checkpoint 值。

**理由：**
- 已有成熟读写接口 `GetConfig`/`SetConfig`
- 无需新增表或 schema 变更
- checkpoint 数据量极小（每条 < 200 字节），key-value 模型足够

**替代方案考虑：**
- 新建 `sync_checkpoints` 表 → 可支持更丰富的 metadata（时间戳、状态），但当前场景不需要
- `collection_runs` 扩展字段 → 语义不匹配，run log 与 checkpoint 生命周期不同

### Decision 2: Checkpoint 标识选择 — 使用 `created_at` 最大时间戳

**选择：** 以 `MAX(created_at)` 作为 `proxy_request_logs` 的 cursor，查询时通过 `WHERE created_at > ?` 过滤。

**理由：**
- `created_at` 是单调递增的 UNIX 时间戳，天然适合做 cursor
- 无需依赖自增 ID（SQLite rowid 可能因 VACUUM 变化）
- 查询简单高效，可走索引

**替代方案考虑：**
- 使用 rowid → 简单但 VACUUM 后可能变化
- 使用 request_id 等 UUID → 无法做范围查询，需改分页模式

### Decision 3: `proxy_request_logs` 增量策略 — 精确时间戳匹配

**策略：**
1. 首次同步：正常全量扫描，完成后记录 `MAX(created_at)` 到 checkpoint
2. 后续同步：从 checkpoint 恢复，查询 `WHERE created_at > checkpoint_value`
3. 每次同步结束后更新 checkpoint 为新的 `MAX(created_at)`

**注意事项：**
- 同一秒内可能有多条记录 → 理论上存在边界漏数据风险。但由于数据的 token 值不变（增量不变），缺失的数据会在下次同步中补回（cursor 不变，重复扫描边界秒）。
- 更精确的做法是记录 last rowid + 最后一条的全部值，但此处无需 100% 精确（token 不变，漏了下次自己补回）。

### Decision 4: `usage_daily_rollups` 导入 — 按天全量拉取 + 对账

**策略：**
1. 读取 `usage_daily_rollups` 表的所有行（通常行数有限，按天+model 聚合）
2. 与本地 `daily_usage` 表按 `(source, usage_date, model)` 比对
3. 对本地缺失的日期或 token 为零的日期，从 rollup 补充写入 `daily_usage`
4. 记录已处理的最大日期范围到 checkpoint

**理由：**
- `usage_daily_rollups` 是日级数据，行数固定（365 天 × N 模型），全量读取开销可接受
- 不按小时细分，写入 `daily_usage` 即可（不写入 `hour_usage`，因为缺乏小时级精度）

### Decision 5: 代码结构 — 新建 `ccswitch.go` collector

**选择：** 将 CC-Switch 导入逻辑从 `app.go` 抽取到 `internal/collector/ccswitch.go`，实现 `Collector` 接口。

**理由：**
- 与现有 collector 架构一致，可利用 Engine 的并行/顺序执行框架
- 解耦业务逻辑与 Wails app 层，便于测试
- 可复用 `PersistHandler` 缓存机制

**实现方式：**
- 新 `CCSwitchCollector` 实现 `Collector` 接口
- `Collect()` 方法内执行增量/历史/对账三步逻辑
- 替换 `app.go:ImportCCSwitchDB()` 中的直接 SQL 操作

### Decision 6: 对账（Reconciliation）策略 — 惰性对账

**策略：**
- 不主动扫描本地缺失的日期范围
- 在 rollup 导入时读取全部 rollup 行，对每个日期检查本地 `daily_usage` 是否存在
- 如果不存在或 token 为 0，则从 rollup 填充
- 执行时机：每次增量同步之后（用于补充当月）+ 单独的 trigger（用户手动触发）

**理由：**
- 大多数历史数据是稳定的（不回填），被动对账即可覆盖
- 避免每次都全量扫描本地 db 的额外开销

## Risks / Trade-offs

| 风险 | 影响 | 缓解措施 |
|------|------|----------|
| `proxy_request_logs` 表数据量大时初始查询慢 | 首次同步时间长 | 首次全量不可避免，后续增量秒级完成；可加进度反馈 |
| `created_at` 索引缺失 | 查询性能差 | CC-Switch 数据库通常有默认索引；可在首次同步前检测 |
| checkpoints 不一致（同步中途崩溃） | 重新处理部分数据 | 幂等写入（hour_usage MAX 语义），重复处理无害 |
| `usage_daily_rollups` 表结构变化 | 导入失败 | 添加结构验证 + 明确的错误日志 |
| cc-switch 数据库文件被替换/重建 | checkpoint 指向不存在的数据 | 检测到 checkpoint 值不在范围内时触发全量回退 |
