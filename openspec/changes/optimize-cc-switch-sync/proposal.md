## Why

当前 CC-Switch 数据导入是全量同步模式，每次同步都会重复拉取已处理数据，存在以下问题：
1. 浪费带宽与处理时间：CC-Switch 中的数据是增量不变的（同一记录的 token 值不会变化），全量重扫无效
2. 数据源覆盖不全：仅依赖单一数据源（proxy_request_logs），该表仅存当月数据，无法获取更早的历史用量
3. 历史数据丢失：若 Claude Code 历史保留期未配置导致本地日志被清除，应用将永久丢失对应时段的用量数据

## What Changes

- 引入 **Checkpoint 机制**：每次同步记录最后一条已处理数据的唯一标识（cursor/token），后续同步仅拉取增量
- 双源导入策略：
  - `proxy_request_logs` 作为**当月**数据源，通过 checkpoint 实现增量拉取
  - `usage_daily_rollups` 作为**历史**数据源，按日汇总补充/更新过往数据（本月前）
- 历史数据对账（Reconciliation）：当本地数据库缺失某日数据时，通过 `usage_daily_rollups` 自动补全
- 同步性能显著提升：首次全量后，后续同步仅处理增量部分（预计减少 80-90% 处理量）

## Capabilities

### New Capabilities
- `proxy-incremental-sync`: 从 CC-Switch `proxy_request_logs` 表增量拉取当月用量数据，基于 checkpoint cursor 实现断点续传
- `rollup-history-sync`: 从 CC-Switch `usage_daily_rollups` 表按日拉取历史汇总数据，补充/更新以"日"为粒度的历史用量
- `incremental-checkpoint`: Checkpoint 持久化管理，记录每次同步的游标位置，支持多数据源独立游标
- `history-reconciliation`: 历史数据对账机制，检测本地缺失的日期范围，自动从 rollup 表补全

### Modified Capabilities
- （无现有 spec 需要修改）

## Impact

- **Backend** (`internal/`):
  - `collector/` — 修改 CC-Switch collector（`ccswitch.go` 或类似），拆分增量/历史导入逻辑
  - `database/` — 新增 checkpoint 存储表或字段，SQLite schema 变更
  - `model/` — 可能新增 Checkpoint 相关数据类型
  - `collector/engine/` — 引擎层需支持 checkpoint 读取/写入生命周期

- **Frontend**: 无直接影响（数据展示逻辑不变，仅数据源更完整）

- **Database**: SQLite 新增 `sync_checkpoints` 表（源标识、游标值、更新时间）

- **CC-Switch API**: 需确认 `proxy_request_logs` 和 `usage_daily_rollups` 的查询接口是否支持 cursor/pagination 参数
