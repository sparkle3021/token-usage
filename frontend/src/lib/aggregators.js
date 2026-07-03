/**
 * 聚合工具函数集。
 */

/**
 * 对过滤后的数据行计算汇总指标（总计 Token、费用、缓存命中率等）。
 * @param {Array} rows 过滤后的 daily 数据
 * @returns {{ totalTokens, inputTokens, outputTokens, cacheReadTokens, cacheCreationTokens, cacheTokens, reasoningTokens, costUSD, cacheHitRate }}
 */
export function aggregateTotals(rows) {
  let total = 0, inp = 0, out = 0, cacheRd = 0, cacheCr = 0, reason = 0, cost = 0;
  for (const r of rows) {
    total += r.totalTokens; inp += r.inputTokens; out += r.outputTokens;
    cacheRd += r.cacheReadTokens; cacheCr += r.cacheCreationTokens;
    reason += r.reasoningOutputTokens; cost += r.costUSD;
  }
  return {
    totalTokens: total, inputTokens: inp, outputTokens: out,
    cacheReadTokens: cacheRd, cacheCreationTokens: cacheCr,
    cacheTokens: cacheRd + cacheCr, reasoningTokens: reason, costUSD: cost,
    cacheHitRate: total ? (cacheRd / total) * 100 : 0
  };
}

/**
 * 将表格数据下载为 CSV 文件。
 * @param {string} filename 文件名
 * @param {Array} rows 数据行
 * @param {Array} columns 列定义 [{ title, field | value }]
 */
export function downloadCSV(filename, rows, columns) {
  const header = columns.map(c => c.title).join(',');
  const body = rows.map(r => columns.map(c => {
    const v = typeof c.value === 'function' ? c.value(r) : r[c.field];
    const s = v == null ? '' : String(v);
    return /[",\n]/.test(s) ? `"${s.replace(/"/g, '""')}"` : s;
  }).join(',')).join('\n');
  const blob = new Blob([header + '\n' + body], { type: 'text/csv;charset=utf-8' });
  const url = URL.createObjectURL(blob);
  const a = document.createElement('a'); a.href = url; a.download = filename; a.click();
  setTimeout(() => URL.revokeObjectURL(url), 1000);
}
