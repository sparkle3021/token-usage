import { createContext, useContext, useReducer, useMemo, useEffect, useRef } from 'react';
import { daysAgo, addDays } from '@/lib/formatters.js';

/**
 * 全局过滤器状态管理，基于 Context + useReducer。
 * 所有过滤器状态提升至此，避免仪表盘和数据页各自维护。
 */

const FilterContext = createContext(null);

const ranges = [
  { id: 'today', label: '今天' },
  { id: 'yesterday', label: '昨天' },
  { id: '7d', label: '近七天' },
  { id: '30d', label: '近30天' },
  { id: 'month', label: '本月' },
  { id: 'lastMonth', label: '上月' },
  { id: 'all', label: '全部' },
];

function fmtDate(d) {
  return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`;
}

/** 各范围对应的时间序列查询窗口天数；all 返回 undefined（前端按 daily 全量） */
function getRangeDays(rangeId) {
  switch (rangeId) {
    case 'today': return 1;
    // 昨天：窗口含昨天+今天（后端按「最近 N 天」查，1 只会拉到今天，导致弹窗点昨天无 hour 数据）
    case 'yesterday': return 2;
    case '7d': return 7;
    case '30d': return 30;
    case 'month': return new Date().getDate();
    // 上月：上月天数 + 本月至今，窗口起点恰好落在上月 1 号，覆盖上月整月（hour 数据不缺边界）
    case 'lastMonth': return new Date(new Date().getFullYear(), new Date().getMonth(), 0).getDate() + new Date().getDate();
    default: return undefined;
  }
}

/** 各范围对应的起止日期（YYYY-MM-DD）；all 返回 null（由 SET_RANGE 按 daily 全量计算） */
function getRangeSpan(rangeId) {
  const today = new Date();
  const todayStr = daysAgo(0);
  switch (rangeId) {
    case 'today': return { start: todayStr, end: todayStr };
    case 'yesterday': { const y = daysAgo(1); return { start: y, end: y }; }
    case '7d': return { start: daysAgo(6), end: todayStr };
    case '30d': return { start: daysAgo(29), end: todayStr };
    case 'month': return { start: fmtDate(new Date(today.getFullYear(), today.getMonth(), 1)), end: todayStr };
    case 'lastMonth': return {
      start: fmtDate(new Date(today.getFullYear(), today.getMonth() - 1, 1)),
      end: fmtDate(new Date(today.getFullYear(), today.getMonth(), 0)),
    };
    default: return null;
  }
}

/** 各范围的对比周期（用于 KPI 环比）；all 返回 null（由调用方按等长平移） */
function getCompareSpan(rangeId, startDate) {
  switch (rangeId) {
    case 'today': { const y = daysAgo(1); return { start: y, end: y }; }
    case 'yesterday': { const y = daysAgo(2); return { start: y, end: y }; }
    case '7d': return { start: addDays(startDate, -7), end: addDays(startDate, -1) };
    case '30d': return { start: addDays(startDate, -30), end: addDays(startDate, -1) };
    case 'month': {
      // startDate 为本月 1 号；上月 = 往前推 2 个月（m-2 为上上月月末，m-1 为上上月 0 号 = 上月末）
      const [y, m] = String(startDate).split('-').map(Number);
      return { start: fmtDate(new Date(y, m - 2, 1)), end: fmtDate(new Date(y, m - 1, 0)) };
    }
    case 'lastMonth': {
      const [y, m] = String(startDate).split('-').map(Number);
      return { start: fmtDate(new Date(y, m - 2, 1)), end: fmtDate(new Date(y, m - 1, 0)) };
    }
    default: return null;
  }
}

/** 过滤器 reducer，支持 SET_RANGE / TOGGLE_SOURCE / SET_MODELS / TOGGLE_COMPARE / RESET */
function filterReducer(state, action) {
  switch (action.type) {
    case 'SET_RANGE': {
      const { rangeId, daily = [] } = action;
      if (rangeId === 'all') {
        const sorted = daily.map(x => x.usageDate).filter(Boolean).sort();
        return {
          ...state, rangeId,
          startDate: sorted[0] || daysAgo(0),
          endDate: sorted[sorted.length - 1] || daysAgo(0),
        };
      }
      const span = getRangeSpan(rangeId) || { start: daysAgo(0), end: daysAgo(0) };
      return { ...state, rangeId, startDate: span.start, endDate: span.end };
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

  // 跨天回滚：本地日期变化时重新锚定相对时间范围（今天/昨天/近N天/本月/上月）。
  // 锚定原本只在挂载/手动切换时计算（createInitialState / SET_RANGE → getRangeSpan）；
  // 应用持续开启跨天后，自动同步只刷新数据、从不重算 startDate/endDate，
  // 「今天」会一直停留在昨天的日期上，需手动切换时间维度才恢复。
  // 用轮询而非定时到午夜：睡眠唤醒、时钟校正后也能自愈；'all' 由数据驱动，不在此处理。
  const anchorRef = useRef(daysAgo(0));
  useEffect(() => {
    const timer = setInterval(() => {
      const today = daysAgo(0);
      if (today === anchorRef.current) return;
      anchorRef.current = today;
      if (f.rangeId !== 'all') dispatch({ type: 'SET_RANGE', rangeId: f.rangeId });
    }, 30 * 1000);
    return () => clearInterval(timer);
  }, [f.rangeId, dispatch]);

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

export { ranges, getRangeDays, getCompareSpan };
