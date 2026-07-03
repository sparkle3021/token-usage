/**
 * API 客户端层，封装所有 window.go.main.App.* 调用。
 * 统一错误处理、便于 mock 和后续协议切换。
 */

/** 获取仪表盘汇总数据，对应 Go 后端 GetDashboardData */
export function getDashboardData() {
  return window.go.main.App.GetDashboardData();
}

/** 获取时间序列数据（原始事件 + 小时聚合），对应 Go 后端 GetTimeSeriesData(days) */
export function getTimeSeriesData(days) {
  return window.go.main.App.GetTimeSeriesData(days);
}

/** 触发增量采集，返回 false 表示采集已在运行 */
export function startCollection() {
  return window.go.main.App.StartCollection();
}

/** 触发全量采集，忽略增量检查点 */
export function startFullCollection() {
  return window.go.main.App.StartFullCollection();
}

/** 查询采集状态，前端轮询用 */
export function collectStatus() {
  return window.go.main.App.CollectStatus();
}

/** 清除所有用量数据和采集历史 */
export function clearAllData() {
  return window.go.main.App.ClearAllData();
}

/** 设置自动同步间隔（分钟），≤0 禁用 */
export function setAutoSyncInterval(minutes) {
  return window.go.main.App.SetAutoSyncInterval(minutes);
}

/** 获取当前自动同步间隔 */
export function getAutoSyncInterval() {
  return window.go.main.App.GetAutoSyncInterval();
}

/** 获取应用设置 */
export function getSettings() {
  return window.go.main.App.GetSettings();
}

/** 保存应用设置 */
export function saveSettings(cfg) {
  return window.go.main.App.SaveSettings(cfg);
}

/** 从远程源更新定价数据 */
export function updatePricing() {
  return window.go.main.App.UpdatePricing();
}

/** 检测默认 CC-Switch 数据库路径是否存在 */
export function detectCCSwitchDB() {
  return window.go.main.App.DetectCCSwitchDB();
}

/** 执行 CC-Switch 数据库导入 */
export function importCCSwitchDB() {
  return window.go.main.App.ImportCCSwitchDB();
}


