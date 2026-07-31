/**
 * 按过滤条件筛选日用量数据。
 * @param {Array} rows 原始 daily 数据
 * @param {object} f 过滤条件 { startDate, endDate, sources, devices, models }
 * @returns {Array} 过滤后的行
 */
export function filterDaily(rows, f) {
  return rows.filter(r =>
    r.usageDate >= f.startDate && r.usageDate <= f.endDate &&
    (f.sources.size === 0 || f.sources.has(r.source)) &&
    (f.devices.size === 0 || f.devices.has(r.device)) &&
    (f.models.size === 0 || f.models.has(r.model)) &&
    (r.totalTokens > 0)
  );
}

/**
 * 按来源/模型过滤时间序列数据（time_usage / hour_usage 行）。
 * 与 filterDaily 同规则：空集合视为不限制。日期范围不在此过滤（today 由调用方保证）。
 */
export function filterTimeSeries(rows, f) {
  return rows.filter(r =>
    (f.sources.size === 0 || f.sources.has(r.source)) &&
    (f.models.size === 0 || f.models.has(r.model))
  );
}
