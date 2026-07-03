import { useState, useEffect, useCallback, useMemo } from 'react';
import * as api from '../api/client.js';
import { daysAgo } from '../lib/formatters.js';

/**
 * 仪表盘数据获取 Hook。
 * 自动在挂载时拉取数据，返回原始数据及计算后的来源/模型列表和热力图数据。
 * @returns {{ M, loadError, refreshing, fetchData, setData, allSources, allModels, heatmapData }}
 */
export function useDashboardData() {
  const [raw, setRaw] = useState(null);
  const [loadError, setLoadError] = useState(null);
  const [refreshing, setRefreshing] = useState(false);

  const setData = useCallback((data, tsData) => {
    setRaw({ ...data, daily: data.daily || [], today: daysAgo(0), time: tsData.time || [], hour: tsData.hour || [] });
    setLoadError(null);
  }, []);

  const fetchData = useCallback((silent) => {
    if (!silent) setRefreshing(true);
    return Promise.all([
      api.getDashboardData(),
      api.getTimeSeriesData()
    ])
      .then(([data, tsData]) => setData(data, tsData))
      .catch(err => setLoadError(String(err)))
      .finally(() => { if (!silent) setRefreshing(false); });
  }, [setData]);

  useEffect(() => { fetchData(false); }, [fetchData]);

  const M = raw;

  const allSources = useMemo(() => M ? [...new Set(M.daily.map(r => r.source))] : [], [M]);
  const allModels = useMemo(() => M ? [...new Set(M.daily.map(r => r.model))].filter(Boolean).sort() : [], [M]);

  const heatmapData = useMemo(() => {
    if (!M) return [];
    const m = new Map();
    for (const r of M.daily) {
      if (!r.totalTokens) continue;
      m.set(r.usageDate, (m.get(r.usageDate) || 0) + (r.totalTokens || 0));
    }
    return [...m.entries()].map(([date, count]) => ({ date, count }));
  }, [M]);

  return { M, loadError, refreshing, fetchData, setData, allSources, allModels, heatmapData };
}
