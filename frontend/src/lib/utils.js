import { clsx } from "clsx";
import { twMerge } from "tailwind-merge"

// cn Tailwind CSS 类名合并工具，基于 clsx + tailwind-merge，自动处理冲突类名。
export function cn(...inputs) {
  return twMerge(clsx(inputs));
}

export { compact, compactCN, deltaPct, formatTs, localDateStr, daysAgo, addDays, rangeDates, numFmt, numFmt as fmt } from './formatters.js';
export { getSourceIconUrl, getModelIconUrl, getSourceColor } from './iconMap.js';
export { filterDaily } from './filters.js';
export { aggregateTotals, downloadCSV } from './aggregators.js';
