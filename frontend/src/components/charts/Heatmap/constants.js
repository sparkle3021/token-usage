/** @type {{{ empty: string, level1: string, level2: string, level3: string, level4: string, level5: string }}} */
export const DEFAULT_THEME = {
  empty: '#ebedf0',
  level1: '#9be9a8',
  level2: '#40c463',
  level3: '#30a14e',
  level4: '#216e39',
  level5: '#0e4429',
};

/** 暗色主题：保持「由浅到深」语义（浅=用量少，深=用量多），整体色调偏暗适配暗背景 */
export const DARK_THEME = {
  empty: '#2a2a2a',
  level1: '#6fbf8f',
  level2: '#4aa06a',
  level3: '#2f8a52',
  level4: '#1d6e3d',
  level5: '#0f542c',
};

export const CELL_SIZE = 14;
export const GAP = 3;
export const DEFAULT_WEEKS = 53;
export const WEEKDAYS = ['Sun', 'Mon', 'Tue', 'Wed', 'Thu', 'Fri', 'Sat'];
export const DAY_LABELS = { 1: 'Mon', 3: 'Wed', 5: 'Fri' };
