import { useMemo } from 'react';
import { fillMissingDates, buildWeeks, buildMonths } from './utils.js';
import { DEFAULT_WEEKS } from './constants.js';

/**
 * @param {import('./types.js').Contribution[]} data
 * @param {Date} [startDate]
 * @param {Date} [endDate]
 * @returns {{ weeks: import('./types.js').DayData[][], months: import('./types.js').MonthLabel[], filledCount: number }}
 */
export function useHeatmap(data = [], startDate, endDate) {
  return useMemo(() => {
    const now = new Date();
    const end = endDate || new Date(now);
    const start = startDate || (() => {
      const d = new Date(now);
      d.setDate(d.getDate() - DEFAULT_WEEKS * 7 + 1);
      return d;
    })();

    const filled = fillMissingDates(data, start, end);
    const weeks = buildWeeks(filled);
    const months = buildMonths(weeks);

    return {
      weeks,
      months,
      filledCount: filled.length,
    };
  }, [data, startDate?.getTime(), endDate?.getTime()]);
}
