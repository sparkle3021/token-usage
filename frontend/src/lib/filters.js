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
