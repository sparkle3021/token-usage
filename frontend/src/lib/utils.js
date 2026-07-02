import { clsx } from "clsx";
import { twMerge } from "tailwind-merge"

export function cn(...inputs) {
  return twMerge(clsx(inputs));
}

// ── Source Icons ─────────────────────────────────────────────────

import claudeIcon from '../assets/icons/claude.svg';
import geminiIcon from '../assets/icons/gemini.svg';
import gptIcon from '../assets/icons/gpt.svg';
import hermesIcon from '../assets/icons/hermes.svg';
import openclawIcon from '../assets/icons/openclaw.svg';
import opencodeIcon from '../assets/icons/opencode.svg';

const SOURCE_ICONS = {
  'Claude Code': claudeIcon,
  'Codex CLI': gptIcon,
  'Gemini CLI': geminiIcon,
  'Hermes Agent': hermesIcon,
  'OpenClaw': openclawIcon,
  'OpenCode': opencodeIcon,
};

export function getSourceIconUrl(name) {
  return SOURCE_ICONS[name] || null;
}

// ── Color palette ──────────────────────────────────────────────

const PALETTE = {
  'Claude Code': 'oklch(0.55 0.16 265)',
  'Codex CLI': 'oklch(0.60 0.15 295)',
  'Hermes Agent': 'oklch(0.58 0.14 240)',
  'OpenClaw': 'oklch(0.65 0.11 200)',
  'OpenCode': 'oklch(0.62 0.12 195)',
  'Gemini CLI': 'oklch(0.72 0.14 75)',
  'Cursor': 'oklch(0.68 0.12 220)',
  'Aider': 'oklch(0.65 0.13 155)',
  'Amp': 'oklch(0.62 0.16 20)',
};

const FALLBACK = [
  'oklch(0.55 0.16 265)', 'oklch(0.60 0.15 295)', 'oklch(0.65 0.11 200)',
  'oklch(0.72 0.14 75)',  'oklch(0.65 0.12 150)', 'oklch(0.62 0.16 20)',
  'oklch(0.58 0.14 240)', 'oklch(0.63 0.14 330)', 'oklch(0.68 0.12 220)',
];

export function getSourceColor(name) {
  if (!name) return 'var(--muted)';
  if (PALETTE[name]) return PALETTE[name];
  let h = 0;
  for (let i = 0; i < name.length; i++) h = (h * 31 + name.charCodeAt(i)) >>> 0;
  return FALLBACK[h % FALLBACK.length];
}

// ── Formatting ─────────────────────────────────────────────────

const numFmt = new Intl.NumberFormat('zh-CN');
const usdFmt = new Intl.NumberFormat('en-US', { style: 'currency', currency: 'USD', minimumFractionDigits: 2, maximumFractionDigits: 2 });

export { numFmt as fmt };

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

// ── Date helpers ───────────────────────────────────────────────

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

// ── Filtering ──────────────────────────────────────────────────

export function filterDaily(rows, f) {
  return rows.filter(r =>
    r.usageDate >= f.startDate && r.usageDate <= f.endDate &&
    (f.sources.size === 0 || f.sources.has(r.source)) &&
    (f.devices.size === 0 || f.devices.has(r.device)) &&
    (f.models.size === 0 || f.models.has(r.model)) &&
    (r.totalTokens > 0)
  );
}

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

// ── CSV ────────────────────────────────────────────────────────

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
