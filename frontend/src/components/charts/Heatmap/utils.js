import { DEFAULT_THEME } from './constants.js';

/**
 * Parse yyyy-MM-dd to Date (local timezone)
 * @param {string} value
 * @returns {Date}
 */
export function parseLocalDate(value) {
  const [y, m, d] = String(value || '').split('-').map(Number);
  return new Date(y, (m || 1) - 1, d || 1);
}

/**
 * Format Date to yyyy-MM-dd
 * @param {Date} date
 * @returns {string}
 */
export function localDateStr(date) {
  return [date.getFullYear(), String(date.getMonth() + 1).padStart(2, '0'), String(date.getDate()).padStart(2, '0')].join('-');
}

/**
 * Check if a date string is today
 * @param {string} dateStr
 * @returns {boolean}
 */
export function isToday(dateStr) {
  const now = new Date();
  const today = localDateStr(now);
  return dateStr === today;
}

/**
 * Fill missing dates with 0 count within the date range
 * @param {import('./types.js').Contribution[]} data
 * @param {Date} startDate
 * @param {Date} endDate
 * @returns {import('./types.js').Contribution[]}
 */
export function fillMissingDates(data, startDate, endDate) {
  const map = new Map();
  for (const item of data) map.set(item.date, item.count);

  const result = [];
  const cur = new Date(startDate);
  const end = new Date(endDate);
  while (cur <= end) {
    const str = localDateStr(cur);
    result.push({ date: str, count: map.get(str) ?? 0 });
    cur.setDate(cur.getDate() + 1);
  }
  return result;
}

/**
 * Build week columns from filled data.
 * Each week = 7 days (Sun..Sat). First/last column may be partial.
 * @param {import('./types.js').Contribution[]} filledData
 * @returns {import('./types.js').DayData[][]}
 */
export function buildWeeks(filledData) {
  if (filledData.length === 0) return [];

  const first = parseLocalDate(filledData[0].date);
  const last = parseLocalDate(filledData[filledData.length - 1].date);

  const start = new Date(first);
  start.setDate(start.getDate() - start.getDay());

  const end = new Date(last);
  end.setDate(end.getDate() + (6 - end.getDay()));

  const dataMap = new Map();
  for (const item of filledData) dataMap.set(item.date, item.count);

  const weeks = [];
  const cur = new Date(start);
  while (cur <= end) {
    const week = [];
    for (let i = 0; i < 7; i++) {
      const str = localDateStr(cur);
      const inRange = dataMap.has(str);
      week.push({
        date: str,
        count: inRange ? dataMap.get(str) : null,
        isToday: inRange ? isToday(str) : false,
      });
      cur.setDate(cur.getDate() + 1);
    }
    weeks.push(week);
  }

  return weeks;
}

/**
 * Calculate month label positions from weeks array.
 * @param {import('./types.js').DayData[][]} weeks
 * @returns {import('./types.js').MonthLabel[]}
 */
export function buildMonths(weeks) {
  const months = [];
  let prevMonth = -1;

  for (let col = 0; col < weeks.length; col++) {
    const day = weeks[col].find(d => d.count !== null);
    if (!day) continue;
    const month = parseLocalDate(day.date).getMonth();
    if (month !== prevMonth) {
      months.push({ col, label: `${month + 1}月` });
      prevMonth = month;
    }
  }

  return months;
}

/**
 * Get contribution color for a count value (exponential bins).
 * @param {number} count
 * @param {import('./types.js').HeatmapTheme} [theme]
 * @returns {string}
 */
export function getContributionColor(count, theme = DEFAULT_THEME) {
  if (count == null) return 'transparent';
  if (count === 0) return theme.empty;
  if (count <= 10_000) return theme.level1;
  if (count <= 100_000) return theme.level2;
  if (count <= 1_000_000) return theme.level3;
  if (count <= 100_000_000) return theme.level4;
  return theme.level5;
}

/**
 * Format tooltip text for a day.
 * @param {string} dateStr
 * @param {number} count
 * @returns {string}
 */
export function formatTooltip(dateStr, count) {
  const d = parseLocalDate(dateStr);
  const formatted = [d.getFullYear(), String(d.getMonth() + 1).padStart(2, '0'), String(d.getDate()).padStart(2, '0')].join('-');
  if (count === 0) return `${formatted}\n无使用`;
  return `${formatted}\n${count.toLocaleString('zh-CN')} tokens`;
}
