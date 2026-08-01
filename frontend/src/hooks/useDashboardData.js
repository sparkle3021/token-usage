import { useState, useEffect, useCallback, useMemo, useRef } from 'react';
import * as api from '../api/client.js';
import { daysAgo } from '../lib/formatters.js';
import { LogPrint } from '../../wailsjs/runtime/runtime.js';

/**
 * 仪表盘数据获取 Hook。
 * 自动在挂载时拉取数据，返回原始数据及计算后的来源/模型列表和热力图数据。
 * 数据分字段存储（daily/time/hour/sessionAgg/runs），未变化字段复用旧引用，
 * 避免切时间范围时全树 useMemo 失效重算。
 * @returns {{ M, loadError, refreshing, fetchData, fetchTimeSeries, allSources, allModels, heatmapData }}
 */
export function useDashboardData() {
  const [daily, setDaily] = useState([]);
  const [time, setTime] = useState([]);
  const [hour, setHour] = useState([]);
  const [sessionAgg, setSessionAgg] = useState([]);
  const [runs, setRuns] = useState([]);
  const [loadError, setLoadError] = useState(null);
  const [refreshing, setRefreshing] = useState(false);
  const [loaded, setLoaded] = useState(false);

  // 请求序号池：快速连切/连刷时丢弃过期响应
  const fetchIdRef = useRef(0);

  const setData = useCallback((data, tsData, sessionAggData) => {
    setDaily(data.daily || []);
    setTime(tsData.time || []);
    setHour(tsData.hour || []);
    setSessionAgg(sessionAggData || []);
    setRuns(data.runs || []);
    setLoaded(true);
    setLoadError(null);
    // [data-log] 数据加载摘要：daily/time/hour 行数 + 今天/昨天总量 + 时间范围
    try {
      const daily = data.daily || [];
      const todayStr = new Date().toISOString().slice(0, 10);
      const yStr = new Date(Date.now() - 86400000).toISOString().slice(0, 10);
      const today = daily.filter(r => r.usageDate === todayStr).reduce((s, r) => s + (r.totalTokens || 0), 0);
      const yesterday = daily.filter(r => r.usageDate === yStr).reduce((s, r) => s + (r.totalTokens || 0), 0);
      const dates = daily.map(r => r.usageDate).filter(Boolean);
      const time = tsData.time || [];
      const hour = tsData.hour || [];
      const timeToday = time.filter(r => (r.usageDate || '').slice(0, 10) === todayStr).length;
      LogPrint(`[data-log] daily=${daily.length} time=${time.length} hour=${hour.length} sessions=${(sessionAggData || []).length} | 今天=${today} 昨天=${yesterday} | daily范围=${dates.length ? dates[dates.length - 1] + '~' + dates[0] : '空'} | time今天=${timeToday}行`);
    } catch (e) { LogPrint(`[data-log] 摘要计算异常: ${e}`); }
  }, []);

  const fetchData = useCallback((silent, days) => {
    const id = ++fetchIdRef.current;
    if (!silent) setRefreshing(true);
    return Promise.all([
      api.getDashboardData(),
      api.getTimeSeriesData(days === undefined ? 90 : days),
      api.getSessionsData(),
    ])
      .then(([data, tsData, sessionAggData]) => {
        if (id !== fetchIdRef.current) return; // 过期响应丢弃
        setData(data, tsData, sessionAggData);
        // 后端 getTimeSeriesData 仅 days==1 返回 time_usage；days!=1 时补拉一次最新 time，
        // 否则今天视图 sparkline 一直用同步前缓存的旧 time 数组
        if (days !== 1) {
          const tid = ++fetchIdRef.current;
          api.getTimeSeriesData(1).then(ts => {
            if (tid !== fetchIdRef.current) return;
            setTime(ts.time || []);
          }).catch(() => {});
        }
      })
      .catch(err => { if (id === fetchIdRef.current) setLoadError(String(err)); })
      .finally(() => { if (!silent) setRefreshing(false); });
  }, [setData]);

  // 切时间范围异步补拉：仅拉时间序列，不重查全量
  const fetchTimeSeries = useCallback((days) => {
    const id = ++fetchIdRef.current;
    api.getTimeSeriesData(days)
      .then(tsData => {
        if (id !== fetchIdRef.current) return;
        setTime(tsData.time || []);
        setHour(tsData.hour || []);
      })
      .catch(() => { /* 静默，保留现有数据 */ });
  }, []);

  useEffect(() => { fetchData(false); }, [fetchData]);

  // M 按字段聚合；daily/sessionAgg/runs 未变时引用稳定，下游 useMemo 不失效
  const M = useMemo(() => {
    if (!loaded) return null;
    return { daily, time, hour, sessionAgg, runs, today: daysAgo(0) };
  }, [loaded, daily, time, hour, sessionAgg, runs]);

  const allSources = useMemo(() => [...new Set(daily.map(r => r.source))].sort(), [daily]);
  const allModels = useMemo(() => [...new Set(daily.map(r => r.model))].filter(Boolean).sort(), [daily]);

  const heatmapData = useMemo(() => {
    const m = new Map();
    for (const r of daily) {
      if (!r.totalTokens) continue;
      m.set(r.usageDate, (m.get(r.usageDate) || 0) + (r.totalTokens || 0));
    }
    return [...m.entries()].map(([date, count]) => ({ date, count }));
  }, [daily]);

  return { M, loadError, refreshing, fetchData, fetchTimeSeries, allSources, allModels, heatmapData };
}
