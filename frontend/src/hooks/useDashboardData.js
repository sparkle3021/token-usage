import { useState, useEffect, useCallback, useMemo, useRef } from 'react';
import * as api from '@/api/client.js';
import { daysAgo } from '@/lib/formatters.js';
import { deviceNamesFromList } from '@/lib/devices.js';

/**
 * 仪表盘数据获取 Hook。
 * 自动在挂载时拉取数据，返回原始数据及计算后的来源/模型列表和热力图数据。
 * 数据分字段存储（daily/time/hour/sessionAgg），未变化字段复用旧引用，
 * 避免切时间范围时全树 useMemo 失效重算。
 * @returns {{ M, loadError, refreshing, fetchData, fetchTimeSeries, allSources, allModels, heatmapData }}
 */
export function useDashboardData() {
  const [daily, setDaily] = useState([]);
  const [time, setTime] = useState([]);
  const [hour, setHour] = useState([]);
  const [sessionAgg, setSessionAgg] = useState([]);
  const [deviceNames, setDeviceNames] = useState({});
  const [loadError, setLoadError] = useState(null);
  const [refreshing, setRefreshing] = useState(false);
  const [loaded, setLoaded] = useState(false);

  // 请求序号池：快速连切/连刷时丢弃过期响应
  const fetchIdRef = useRef(0);

  // 设备映射：后端附带映射优先，启动时补拉 GetDevices 兜底（sessions 等无附带的数据源）
  useEffect(() => {
    api.getDevices().then(list => {
      setDeviceNames(prev => ({ ...deviceNamesFromList(list), ...prev }));
    }).catch(() => {});
  }, []);

  const setData = useCallback((data, tsData, sessionAggData) => {
    setDaily(data.daily || []);
    setTime(tsData.time || []);
    setHour(tsData.hour || []);
    setSessionAgg(sessionAggData || []);
    setDeviceNames(prev => ({ ...prev, ...(data.deviceNames || {}), ...(tsData.deviceNames || {}) }));
    setLoaded(true);
    setLoadError(null);
  }, []);

  const fetchData = useCallback((silent, days) => {
    if (!silent) setRefreshing(true);
    // 本次调用抢占的序号段：主请求 id；days!=1 时补拉"今天"额外抢占 todayFetchId。
    // 校验统一以本次抢占的最终序号 latestId 为基准——任何后续用户操作/新刷新都会推高
    // fetchIdRef.current 使本次调用整体作废；补拉序号也必须在发起前抢占（不随主请求延迟而变号）
    const id = ++fetchIdRef.current;
    const todayFetchId = days !== 1 ? ++fetchIdRef.current : null;
    const latestId = todayFetchId ?? id;
    return Promise.all([
      api.getDashboardData(),
      api.getTimeSeriesData(days === undefined ? 90 : days),
      api.getSessionsData(),
    ])
      .then(([data, tsData, sessionAggData]) => {
        if (latestId !== fetchIdRef.current) return; // 过期响应丢弃
        setData(data, tsData, sessionAggData);
        // 后端 getTimeSeriesData 仅 days==1 返回 time_usage；days!=1 时补拉一次最新 time，
        // 否则今天视图 sparkline 一直用同步前缓存的旧 time 数组
        if (todayFetchId !== null && todayFetchId === fetchIdRef.current) {
          api.getTimeSeriesData(1).then(ts => {
            if (todayFetchId !== fetchIdRef.current) return;
            setTime(ts.time || []);
          }).catch(() => {});
        }
      })
      .catch(err => { if (latestId === fetchIdRef.current) setLoadError(String(err)); })
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
    return { daily, time, hour, sessionAgg, deviceNames, today: daysAgo(0) };
  }, [loaded, daily, time, hour, sessionAgg, deviceNames]);

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
