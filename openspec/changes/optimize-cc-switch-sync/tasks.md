## 1. Checkpoint 基础设施

- [x] 1.1 在 `database.Manager` 中新增 `GetCheckpoint(key string) (string, error)` 和 `SetCheckpoint(key, value string) error` 方法，封装 `app_config` 读写
- [x] 1.2 新增 `DeleteCheckpointsByPrefix(prefix string) error` 方法，支持按前缀批量删除 checkpoint
- [x] 1.3 定义 checkpoint key 常量：`CCSwitchCursorProxyLogs = "cc_switch_cursor_proxy_request_logs"` 和 `CCSwitchRollupMaxDate = "cc_switch_rollup_max_date"`
- [x] 1.4 新增 `ResetCCSwitchCheckpoints()` 方法，清空所有 CP-Switch 相关 checkpoint

## 2. CCSwitch Collector 抽取

- [x] 2.1 新建 `internal/collector/ccswitch.go`，定义 `CCSwitchCollector` 结构体，实现 `Collector` 接口
- [x] 2.2 实现 `ID()` 返回 `"cc-switch"`，`Source()` 返回 `"CC-Switch"`
- [x] 2.3 将 `app.go:ImportCCSwitchDB()` 中的 `proxy_request_logs` 查询逻辑迁移到 `Collect()` 方法中
- [x] 2.4 将 CCSwitchCollector 注册到 Engine 的 collectors 列表中（`engine.go:New()`）
- [x] 2.5 移除或包装 `app.go:ImportCCSwitchDB()` 以调用新的 collector（向后兼容）

## 3. proxy_request_logs 增量同步

- [x] 3.1 在 `Collect()` 中实现 checkpoint 读取逻辑：首次同步无 checkpoint 时全量，后续同步从 checkpoint 断点续传
- [x] 3.2 修改 SQL 查询，增加 `WHERE created_at > ?` 条件（checkpoint 存在时）
- [x] 3.3 同步完成后，查询 `MAX(created_at)` 并将 checkpoint 持久化到 `app_config`
- [x] 3.4 处理 checkpoint 过期场景：当存储的 cursor 值 > 当前 `MAX(created_at)` 时，回退到全量同步
- [x] 3.5 支持 `forceFull` 模式：Engine 设置后清除 checkpoint 触发全量同步

## 4. usage_daily_rollups 历史导入

- [x] 4.1 在 `Collect()` 中增加对 `usage_daily_rollups` 表的读取逻辑
- [x] 4.2 实现 rollup 数据到 `daily_usage` 的转换和 `UpsertDaily()` 写入
- [x] 4.3 记录 rollup 的 `checkpoint_max_date`，后续仅查新增日期

## 5. 历史数据对账

- [x] 5.1 实现缺失日期检测逻辑：比对 rollup 数据与本地 `daily_usage`，标记缺失 / zero-token 的日期
- [x] 5.2 实现对账补充逻辑：对检测到的缺失日期调用 `UpsertDaily()`
- [x] 5.3 在对账过程中跳过已有有效数据的日期，尊重 `pricing_locked_at`
- [x] 5.4 在 `CollectResult` 中返回对账统计（checked / supplemented / skipped）

## 6. 集成与测试

- [x] 6.1 确保 `BuildDailyFromHourUsage()` 在 CC-Switch collector 运行后正确重建日表
- [ ] 6.2 验证 `forceFull` 模式清除 checkpoint 后能正确触发全量同步
- [ ] 6.3 验证 CSV 导入（`ImportCSV()`）不受影响（仍使用旧逻辑直接写 `daily_usage`）
- [ ] 6.4 验证前端展示无回归（数据源标记、日期范围、token 聚合值正确）
