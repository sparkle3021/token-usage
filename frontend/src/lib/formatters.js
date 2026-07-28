/**
 * 格式化工具函数集。
 * 包括数字简写（K/M/B、万/亿）、百分比变化、时间戳格式化、日期计算。
 */

export const numFmt = new Intl.NumberFormat('zh-CN');

export function compact(v) {
  if (v == null) return '—';
  const a = Math.abs(v);
  if (a >= 1e9) return (v / 1e9).toFixed(1).replace(/\.0$/, '') + 'B';
  if (a >= 1e6) return (v / 1e6).toFixed(1).replace(/\.0$/, '') + 'M';
  if (a >= 1e3) return (v / 1e3).toFixed(1).replace(/\.0$/, '') + 'K';
  return numFmt.format(v);
}

export function compactCN(v) {
  if (v == null) return '—';
  const a = Math.abs(v);
  if (a >= 1e8) return (v / 1e8).toFixed(2).replace(/\.?0+$/, '') + ' 亿';
  if (a >= 1e4) return (v / 1e4).toFixed(1).replace(/\.0$/, '') + ' 万';
  return numFmt.format(v);
}

export function deltaPct(curr, prev) {
  if (prev == null || prev === 0) return null;
  return ((curr - prev) / prev) * 100;
}

export function formatTs(v) {
  if (!v) return '—';
  const normalized = String(v).includes('T') ? v : String(v).replace(' ', 'T');
  const hasZone = /(?:Z|[+-]\d{2}:?\d{2})$/.test(normalized);
  const d = new Date(hasZone ? normalized : normalized + 'Z');
  if (isNaN(d.getTime())) return String(v).slice(0, 16);
  return new Intl.DateTimeFormat('zh-CN', {
    year: 'numeric', month: '2-digit', day: '2-digit',
    hour: '2-digit', minute: '2-digit', hour12: false
  }).format(d);
}

export function localDateStr(date) {
  return [date.getFullYear(), String(date.getMonth() + 1).padStart(2, '0'), String(date.getDate()).padStart(2, '0')].join('-');
}

export function daysAgo(n) {
  const d = new Date(); d.setHours(0, 0, 0, 0); d.setDate(d.getDate() - n);
  return localDateStr(d);
}

export function addDays(dateStr, days) {
  const d = parseLocalDate(dateStr); d.setDate(d.getDate() + days);
  return localDateStr(d);
}

export function rangeDates(startStr, endStr) {
  const out = [];
  const s = parseLocalDate(startStr), e = parseLocalDate(endStr);
  for (let d = new Date(s); d <= e; d.setDate(d.getDate() + 1)) out.push(localDateStr(d));
  return out;
}

function parseLocalDate(value) {
  const [y, m, d] = String(value || '').split('-').map(Number);
  return new Date(y, (m || 1) - 1, d || 1);
}

// ── 聚合工具函数 ──────────────────────────────

function getISOWeekNumber(date) {
  const d = new Date(Date.UTC(date.getFullYear(), date.getMonth(), date.getDate()));
  const dayNum = (d.getUTCDay() + 6) % 7;
  d.setUTCDate(d.getUTCDate() - dayNum + 3);
  const firstThursday = d.getTime();
  d.setUTCMonth(0, 1);
  if (d.getUTCDay() !== 4) {
    d.setUTCMonth(0, 1 + ((4 - d.getUTCDay()) + 7) % 7);
  }
  return Math.ceil((firstThursday - d.getTime()) / 604800000);
}

function bucketLabel(dateStr, granularity) {
  const d = parseLocalDate(dateStr);
  if (granularity === 'weekly') {
    const wn = getISOWeekNumber(d);
    return `${d.getFullYear()}-W${String(wn).padStart(2, '0')}`;
  }
  if (granularity === 'monthly') return dateStr.slice(0, 7);
  if (granularity === 'yearly') return dateStr.slice(0, 4);
  return dateStr;
}

/**
 * 将日级数据按指定粒度聚合（周/月/年）。
 * @param {Array} rows 过滤后的 daily 数据 [{ usageDate, source, totalTokens, ... }]
 * @param {string[]} dates 日期范围
 * @param {'weekly'|'monthly'|'yearly'} granularity
 * @returns {Map<string, Map<string, number>>} label -> source -> totalTokens
 */
export function aggregateRows(rows, dates, granularity) {
  const buckets = new Map();
  for (const date of dates) {
    const label = bucketLabel(date, granularity);
    if (!buckets.has(label)) buckets.set(label, new Map());
  }
  for (const r of rows) {
    const label = bucketLabel(r.usageDate, granularity);
    const sourceMap = buckets.get(label);
    if (!sourceMap) continue;
    sourceMap.set(r.source, (sourceMap.get(r.source) || 0) + (r.totalTokens || 0));
  }
  return buckets;
}

/**
 * 从聚合后的 buckets 计算活跃度和最长连续空白。
 * @param {Map<string, Map<string, number>>} buckets
 * @returns {{ active: number, longestGap: number }}
 */
export function computeActivityStats(buckets) {
  const labels = [...buckets.keys()].sort();
  let active = 0, longestGap = 0, currentGap = 0;
  for (const label of labels) {
    const sourceMap = buckets.get(label);
    if (sourceMap && sourceMap.size > 0) {
      active++;
      currentGap = 0;
    } else {
      currentGap++;
      longestGap = Math.max(longestGap, currentGap);
    }
  }
  return { active, longestGap };
}

/**
 * 将 Map<date, value> 按粒度聚合为值数组。
 * @param {Map<string, number>} dailyMap
 * @param {string[]} dates
 * @param {'weekly'|'monthly'|'yearly'} granularity
 * @returns {number[]}
 */
export function aggregateMapToArray(dailyMap, dates, granularity) {
  const buckets = new Map();
  for (const date of dates) {
    buckets.set(bucketLabel(date, granularity), 0);
  }
  for (const [date, val] of dailyMap) {
    const label = bucketLabel(date, granularity);
    if (buckets.has(label)) buckets.set(label, buckets.get(label) + val);
  }
  return [...buckets.values()];
}

/**
 * 从 rows 中按粒度聚合指定字段。
 * @param {Array} rows
 * @param {string[]} dates
 * @param {'weekly'|'monthly'|'yearly'} granularity
 * @param {string} field 字段名如 'inputTokens'、'outputTokens'
 * @returns {number[]}
 */
export function aggregateField(rows, dates, granularity, field) {
  const buckets = new Map();
  for (const date of dates) {
    buckets.set(bucketLabel(date, granularity), 0);
  }
  for (const r of rows) {
    const label = bucketLabel(r.usageDate, granularity);
    if (buckets.has(label)) buckets.set(label, buckets.get(label) + (r[field] || 0));
  }
  return [...buckets.values()];
}
