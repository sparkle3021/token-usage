import { useMemo } from 'react';
import { fillMissingDates, buildWeeks, buildMonths } from '@/components/charts/Heatmap/utils.js';
import { DEFAULT_WEEKS } from '@/components/charts/Heatmap/constants.js';

/**
 * @param {{{ date: string, count: number }}[]} data
 * @param {Date} [startDate]
 * @param {Date} [endDate]
 * @returns {{ weeks: {{ date: string, count: number, isToday: boolean }}[][], months: {{ col: number, label: string }}[], filledCount: number }}
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
