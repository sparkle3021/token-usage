/**
 * @typedef {{ date: string, count: number }} Contribution
 */

/**
 * @typedef {'empty'|'level1'|'level2'|'level3'|'level4'|'level5'} ColorLevel
 */

/**
 * @typedef {{ empty: string, level1: string, level2: string, level3: string, level4: string, level5: string }} HeatmapTheme
 */

/**
 * @typedef {{ date: string, count: number, isToday: boolean }} DayData
 */

/**
 * @typedef {DayData[]} WeekData - 7 days (Sun..Sat)
 */

/**
 * @typedef {{ col: number, label: string }} MonthLabel
 */

/**
 * @typedef {Object} HeatmapProps
 * @property {Contribution[]} data
 * @property {Date} [startDate]
 * @property {Date} [endDate]
 * @property {number} [cellSize]
 * @property {number} [gap]
 * @property {(date: string) => void} [onSelect]
 * @property {string} [className]
 * @property {HeatmapTheme} [theme]
 */

export const TYPES = {};
