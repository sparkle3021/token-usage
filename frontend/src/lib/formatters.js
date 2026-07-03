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
