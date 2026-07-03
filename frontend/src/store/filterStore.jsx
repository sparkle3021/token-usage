import { createContext, useContext, useReducer, useMemo } from 'react';
import { daysAgo } from '../lib/formatters.js';

/**
 * 全局过滤器状态管理，基于 Context + useReducer。
 * 所有过滤器状态提升至此，避免仪表盘和数据页各自维护。
 */

const FilterContext = createContext(null);

const ranges = [
  { id: 'today', label: '今天' }, { id: '7d', label: '7 天' }, { id: '14d', label: '14 天' },
  { id: '30d', label: '30 天' }, { id: '90d', label: '90 天' }, { id: 'all', label: '全部' },
];

const rangeDays = { today: 1, '7d': 7, '14d': 14, '30d': 30, '90d': 90 };

/** 过滤器 reducer，支持 SET_RANGE / TOGGLE_SOURCE / SET_MODELS / TOGGLE_COMPARE / RESET */
function filterReducer(state, action) {
  switch (action.type) {
    case 'SET_RANGE': {
      const { rangeId, daily = [] } = action;
      let startDate, endDate = daysAgo(0);
      if (rangeId === 'all') {
        const sorted = daily.map(x => x.usageDate).filter(Boolean).sort();
        startDate = sorted[0] || daysAgo(0);
        endDate = sorted[sorted.length - 1] || daysAgo(0);
      } else {
        const days = rangeDays[rangeId] || 30;
        startDate = daysAgo(days - 1);
      }
      return { ...state, rangeId, startDate, endDate };
    }
    case 'TOGGLE_SOURCE': {
      const n = new Set(state.sources);
      if (n.has(action.source)) { n.delete(action.source); } else { n.add(action.source); }
      return { ...state, sources: n };
    }
    case 'SET_MODELS':
      return { ...state, models: action.models };
    case 'TOGGLE_COMPARE':
      return { ...state, compare: !state.compare };
    case 'RESET':
      return createInitialState();
    default:
      return state;
  }
}

function createInitialState() {
  return {
    rangeId: 'today',
    startDate: daysAgo(0),
    endDate: daysAgo(0),
    sources: new Set(),
    devices: new Set(),
    models: new Set(),
    compare: true,
  };
}

export function FilterProvider({ children }) {
  const [f, dispatch] = useReducer(filterReducer, undefined, createInitialState);
  const value = useMemo(() => ({ f, dispatch, ranges }), [f, dispatch]);
  return (
    <FilterContext.Provider value={value}>
      {children}
    </FilterContext.Provider>
  );
}

export function useFilter() {
  const ctx = useContext(FilterContext);
  if (!ctx) throw new Error('useFilter must be used within FilterProvider');
  return ctx;
}

export { ranges, rangeDays };
