/**
 * API 客户端层，封装所有 window.go.main.App.* 调用。
 * 统一错误处理、便于 mock 和后续协议切换。
 */

/**
 * 统一调用入口：安全探测 Wails 运行时（window.go?.main?.App）。
 * 无运行时（浏览器 dev 模式）返回 rejected Promise，避免访问 window.go.main 同步 throw 导致页面白屏。
 */
function invoke(name, ...args) {
  const app = window.go?.main?.App;
  if (!app) {
    return Promise.reject(new Error('Wails 运行时不可用，请在桌面应用中打开'));
  }
  return app[name](...args);
}

/** 获取仪表盘汇总数据，对应 Go 后端 GetDashboardData */
export function getDashboardData() {
  return invoke('GetDashboardData');
}

/** 获取指定范围小时聚合序列，对应 Go 后端 GetHourSeries(days) */
export function getHourSeries(days) {
  return invoke('GetHourSeries', days);
}

/** 获取今天原始事件（恒定，不随范围变化），对应 Go 后端 GetTodayEvents */
export function getTodayEvents() {
  return invoke('GetTodayEvents');
}

/** 获取模型维度聚合排行（总用量/费用/请求次数），对应 Go 后端 GetModelRanking */
export function getModelRanking() {
  return invoke('GetModelRanking');
}

/** 获取单模型时间序列数据（日级 + 小时级），对应 Go 后端 GetModelSeries(model) */
export function getModelSeries(model) {
  return invoke('GetModelSeries', model);
}

/** 触发增量采集，返回 false 表示采集已在运行 */
export function startCollection() {
  return invoke('StartCollection');
}

/** 触发全量采集，忽略增量检查点 */
export function startFullCollection() {
  return invoke('StartFullCollection');
}

/** 查询采集状态，前端轮询用 */
export function collectStatus() {
  return invoke('CollectStatus');
}

/** 清除所有用量数据和采集历史 */
export function clearAllData() {
  return invoke('ClearAllData');
}

/** 设置自动同步间隔（分钟），≤0 禁用 */
export function setAutoSyncInterval(minutes) {
  return invoke('SetAutoSyncInterval', minutes);
}

/** 获取当前自动同步间隔 */
export function getAutoSyncInterval() {
  return invoke('GetAutoSyncInterval');
}

/** 查询当前正在运行的操作名称（"collection"/"cc-import"/"clear-data"/""） */
export function currentOp() {
  return invoke('CurrentOp');
}

/** 获取应用版本号（设置页展示） */
export function getAppVersion() {
  return invoke('GetAppVersion');
}

/** 获取应用设置 */
export function getSettings() {
  return invoke('GetSettings');
}

/** 保存应用设置 */
export function saveSettings(cfg) {
  return invoke('SaveSettings', cfg);
}

/** 从远程源更新定价数据（LiteLLM → model_pricing 表，UPSERT 覆盖） */
export function updatePricing() {
  return invoke('UpdatePricing');
}

/** 获取全部模型价格（设置-价格管理） */
export function listModelPricing() {
  return invoke('ListModelPricing');
}

/** 修改单个模型价格（用户调整，写库 + 同步引擎，立即生效） */
export function updateModelPricing(row) {
  return invoke('UpdateModelPricing', row);
}

/** 删除单个模型价格（手动添加/残留模型） */
export function deleteModelPricing(modelKey) {
  return invoke('DeleteModelPricing', modelKey);
}

/** 获取价格元信息（最近拉取时间 + 条目数） */
export function getPricingMeta() {
  return invoke('GetPricingMeta');
}

/** 检测默认 CC-Switch 数据库路径是否存在 */
export function detectCCSwitchDB() {
  return invoke('DetectCCSwitchDB');
}

// ── 设备 API ──

/** 获取设备注册表（device_id → hostname/display_name/is_local） */
export function getDevices() { return invoke('GetDevices'); }

/** 重命名设备展示名 */
export function renameDevice(deviceId, displayName) { return invoke('RenameDevice', deviceId, displayName); }

/** 导出用量数据到 JSON 文件，返回保存路径（取消返回空串） */
export function exportData() { return invoke('ExportData'); }

/** 从导出 JSON 文件导入用量数据，返回合并规模（取消返回 null） */
export function importData() { return invoke('ImportData'); }

// ── 用量查询 API ──

/** 获取所有用量查询配置 */
export function listQuotaConfigs() { return invoke('ListQuotaConfigs'); }

/** 获取所有供应商 schema */
export function getProviderSchemas() { return invoke('GetProviderSchemas'); }

/** 创建用量查询配置 */
export function createQuotaConfig(cfg) { return invoke('CreateQuotaConfig', cfg); }

/** 修改用量查询配置 */
export function updateQuotaConfig(cfg) { return invoke('UpdateQuotaConfig', cfg); }

/** 删除用量查询配置 */
export function deleteQuotaConfig(id) { return invoke('DeleteQuotaConfig', id); }

/** 拉取单个用量数据 */
export function fetchQuota(id) { return invoke('FetchQuota', id); }

/** 并发拉取所有用量数据 */
export function fetchAllQuota() { return invoke('FetchAllQuota'); }
