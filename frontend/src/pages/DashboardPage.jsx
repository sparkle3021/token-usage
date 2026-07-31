/**
 * 仪表盘页面：过滤器 + KPI 行 + 趋势图 + TopModels + 热力图 + Drilling。
 * 使用 useFilter 订阅全局过滤器状态，根据过滤条件实时计算聚合指标。
 */

import { useMemo, useState } from 'react';
import { useFilter, rangeDays } from '../store/filterStore.jsx';
import { filterDaily, filterTimeSeries } from '../lib/filters.js';
import { aggregateTotals } from '../lib/aggregators.js';
import { addDays, rangeDates, compactCN, deltaPct } from '../lib/formatters.js';
import * as U from '../lib/utils.js';
import KPI from '../components/common/KPI.jsx';
import FilterBar from '../components/layout/FilterBar.jsx';
import TrendChart from '../components/charts/TrendChart.jsx';
import TopModels from '../components/charts/TopModels.jsx';
import Heatmap from '../components/charts/Heatmap/Heatmap.jsx';
import HeatmapDrillDialog from './dashboard/HeatmapDrillDialog.jsx';
import { Dialog, DialogContent, DialogTitle, DialogDescription } from '../components/ui/dialog.jsx';
import SourceIcon from '../components/SourceIcon.jsx';

export default function DashboardPage({ M, allSources, allModels, heatmapData, onRefresh }) {
  const { f, dispatch } = useFilter();
  const [trendMode, setTrendMode] = useState('stacked');
  const [topModelDrill, setTopModelDrill] = useState(null);
  const [heatmapDate, setHeatmapDate] = useState(null);
  const [granularity, setGranularity] = useState('daily');

  const filtered = useMemo(() => filterDaily(M?.daily || [], f), [f, M]);

  const filteredTime = useMemo(() => filterTimeSeries(M?.time || [], f), [f, M]);
  const filteredHour = useMemo(() => filterTimeSeries(M?.hour || [], f), [f, M]);

  const totals = useMemo(() => aggregateTotals(filtered), [filtered]);

  const dates = useMemo(() => rangeDates(f.startDate, f.endDate), [f]);

  const compareData = useMemo(() => {
    if (!f.compare) return { totals: null };
    const days = dates.length;
    const endStr = addDays(f.startDate, -1);
    const startStr = addDays(endStr, -(days - 1));
    return { totals: aggregateTotals(filterDaily(M?.daily || [], { ...f, startDate: startStr, endDate: endStr })) };
  }, [f, dates, M]);

  const dailyMap = useMemo(() => {
    const m = new Map();
    for (const r of filtered) m.set(r.usageDate, (m.get(r.usageDate) || 0) + r.totalTokens);
    return m;
  }, [filtered]);

  const isHourly = f.rangeId === 'today';

  const hourlySpark = useMemo(() => {
    if (!isHourly || !M) return null;
    const todayStr = dates[0];
    const hd = Array.from({ length: 24 }, () => ({ total: 0, input: 0, output: 0, cacheRd: 0, reason: 0, cost: 0 }));

    for (const r of filteredTime) {
      if (r.usageDate !== todayStr) continue;
      const d = new Date(r.eventTime);
      if (isNaN(d.getTime())) continue;
      const h = d.getHours();
      hd[h].total += r.totalTokens || 0; hd[h].input += r.inputTokens || 0;
      hd[h].output += r.outputTokens || 0; hd[h].cacheRd += r.cacheReadTokens || 0;
      hd[h].reason += r.reasoningOutputTokens || 0; hd[h].cost += r.costUSD || 0;
    }

    const covered = new Set(filteredTime.filter(r => r.usageDate === todayStr).map(r => r.source));
    for (const r of filteredHour) {
      if (r.usageDate !== todayStr || covered.has(r.source)) continue;
      const h = r.hour;
      hd[h].total += r.totalTokens || 0; hd[h].input += r.inputTokens || 0;
      hd[h].output += r.outputTokens || 0; hd[h].cacheRd += r.cacheReadTokens || 0;
      hd[h].reason += r.reasoningOutputTokens || 0; hd[h].cost += r.costUSD || 0;
      covered.add(r.source);
    }

    const curHour = new Date().getHours();
    for (const r of filtered) {
      if (r.usageDate !== todayStr || covered.has(r.source)) continue;
      hd[curHour].total += r.totalTokens || 0; hd[curHour].input += r.inputTokens || 0;
      hd[curHour].output += r.outputTokens || 0; hd[curHour].cacheRd += r.cacheReadTokens || 0;
      hd[curHour].reason += r.reasoningOutputTokens || 0; hd[curHour].cost += r.costUSD || 0;
      covered.add(r.source);
    }

    return {
      total: hd.map(h => h.total), input: hd.map(h => h.input),
      output: hd.map(h => h.output), cacheRead: hd.map(h => h.cacheRd),
      reasoning: hd.map(h => h.reason), cost: hd.map(h => h.cost),
    };
  }, [isHourly, dates, M, filtered, filteredTime, filteredHour]);

  const sparkValues = useMemo(
    () => hourlySpark ? hourlySpark.total
      : granularity !== 'daily' ? U.aggregateMapToArray(dailyMap, dates, granularity)
      : dates.map(d => dailyMap.get(d) || 0),
    [hourlySpark, granularity, dailyMap, dates],
  );

  const sparkBy = useMemo(() => (key) => {
    if (hourlySpark) {
      const m = { totalTokens: 'total', inputTokens: 'input', outputTokens: 'output', cacheReadTokens: 'cacheRead', reasoningOutputTokens: 'reasoning', costUSD: 'cost' };
      return hourlySpark[m[key]] || hourlySpark.total;
    }
    if (granularity !== 'daily') return U.aggregateField(filtered, dates, granularity, key);
    const m = new Map();
    for (const r of filtered) m.set(r.usageDate, (m.get(r.usageDate) || 0) + (r[key] || 0));
    return dates.map(d => m.get(d) || 0);
  }, [hourlySpark, granularity, filtered, dates]);

  const presentSources = useMemo(() => Array.from(f.sources.size ? f.sources : new Set(allSources)), [f.sources, allSources]);

  const setRange = (rangeId) => {
    dispatch({ type: 'SET_RANGE', rangeId, daily: M?.daily || [] });
    onRefresh(false, rangeDays[rangeId]);
    // Compute expected range length for granularity recommendation
    let days = rangeDays[rangeId];
    if (!days && rangeId === 'all' && M?.daily?.length) {
      const sorted = M.daily.map(x => x.usageDate).filter(Boolean).sort();
      if (sorted.length > 0) days = Math.max(1, Math.round((Date.now() - new Date(sorted[0]).getTime()) / 86400000));
    }
    if (days > 730) setGranularity('yearly');
    else if (days > 365) setGranularity('monthly');
    else if (days > 90) setGranularity('weekly');
    else setGranularity('daily');
  };

  return (
    <>
      <FilterBar
        f={f}
        allSources={allSources}
        allModels={allModels}
        onSetRange={setRange}
        onToggleSource={(s) => dispatch({ type: 'TOGGLE_SOURCE', source: s })}
        onSetModels={(models) => dispatch({ type: 'SET_MODELS', models })}
        onToggleCompare={() => dispatch({ type: 'TOGGLE_COMPARE' })}
      />

      <div className="grid grid-cols-2 sm:grid-cols-3 lg:grid-cols-6 gap-3">
        <KPI label="总 Token" value={compactCN(totals.totalTokens)} delta={deltaPct(totals.totalTokens, compareData.totals?.totalTokens)} spark={sparkValues} color="oklch(0.55 0.16 265)" />
        <KPI label="Input" value={compactCN(totals.inputTokens)} delta={deltaPct(totals.inputTokens, compareData.totals?.inputTokens)} spark={sparkBy('inputTokens')} color="oklch(0.62 0.13 240)" />
        <KPI label="Output" value={compactCN(totals.outputTokens)} delta={deltaPct(totals.outputTokens, compareData.totals?.outputTokens)} spark={sparkBy('outputTokens')} color="oklch(0.60 0.15 295)" />
        <KPI label="Cache" value={compactCN(totals.cacheReadTokens)} sub={`${totals.cacheHitRate.toFixed(2)}% 命中`} delta={deltaPct(totals.cacheReadTokens, compareData.totals?.cacheReadTokens)} subDelta={deltaPct(totals.cacheHitRate, compareData.totals?.cacheHitRate)} spark={sparkBy('cacheReadTokens')} color="oklch(0.65 0.11 200)" />
        <KPI label="Reasoning" value={compactCN(totals.reasoningTokens)} delta={deltaPct(totals.reasoningTokens, compareData.totals?.reasoningTokens)} spark={sparkBy('reasoningOutputTokens')} color="oklch(0.65 0.12 150)" />
        <KPI label="费用" value={`$${(totals.costUSD || 0).toFixed(2)}`} delta={deltaPct(totals.costUSD, compareData.totals?.costUSD)} spark={sparkBy('costUSD')} color="oklch(0.72 0.14 75)" />
      </div>

      <div className="flex flex-col lg:flex-row gap-4">
        <div className="flex-1 min-w-0">
          <TrendChart rows={filtered} dates={dates} sources={presentSources} mode={trendMode} onModeChange={setTrendMode} totals={totals} timeRows={filteredTime} hourRows={filteredHour} isHourly={f.rangeId === 'today'} granularity={granularity} onGranularityChange={setGranularity} />
        </div>
        <div className="lg:w-80 2xl:w-96 shrink-0 max-lg:min-h-0 lg:relative">
          <div className="flex flex-col min-h-0 max-lg:h-auto lg:absolute lg:inset-0">
            <TopModels rows={filtered} onDrillModel={r => setTopModelDrill(r)} allDaily={M?.daily} />
          </div>
        </div>
      </div>

      <Heatmap data={heatmapData} onSelect={setHeatmapDate} />
      {heatmapDate && <HeatmapDrillDialog date={heatmapDate} daily={M?.daily} timeRows={M?.time} hourRows={M?.hour} onClose={() => setHeatmapDate(null)} />}

      {topModelDrill && (
        <Dialog open onOpenChange={o => { if (!o) setTopModelDrill(null); }}>
          <DialogContent className="sm:max-w-lg max-h-[85vh] overflow-y-auto" showCloseButton>
            <DialogTitle className="sr-only">{topModelDrill.model} 详情</DialogTitle>
            <DialogDescription className="sr-only">模型用量详情</DialogDescription>

            <div className="mb-4">
              <div className="text-xs text-muted-foreground mb-0.5">模型详情</div>
              <h3 className="text-sm font-semibold">{topModelDrill.model}</h3>
            </div>

            <div className="flex items-center gap-4 text-xs text-muted-foreground mb-4 flex-wrap">
              <span>总 Token <strong className="text-foreground">{U.compactCN(topModelDrill.total)}</strong></span>
              <span>费用 <strong className="text-foreground">${(topModelDrill.cost || 0).toFixed(2)}</strong></span>
              <span>活跃 <strong className="text-foreground">{topModelDrill.dayCount}</strong> 天</span>
            </div>

            {topModelDrill.sources?.length > 0 && (
              <div className="space-y-1.5">
                {topModelDrill.sources.map(s => {
                  const pct = (s.total / topModelDrill.total * 100);
                  return (
                    <div key={s.source} className="grid grid-cols-[1fr_auto] items-center gap-3 px-1.5 py-1.5">
                      <div className="min-w-0">
                        <div className="flex items-center gap-1.5">
                          <SourceIcon name={s.source} className="w-3.5 h-3.5 shrink-0" />
                          <span className="text-xs font-medium truncate">{s.source}</span>
                        </div>
                        <div className="h-1.5 rounded-full bg-muted mt-1.5 overflow-hidden">
                          <div className="h-full" style={{ width: `${pct}%`, background: U.getSourceColor(s.source) }} />
                        </div>
                      </div>
                      <div className="text-right shrink-0">
                        <div className="text-xs font-semibold tabular-nums">{U.compactCN(s.total)}</div>
                        <div className="text-[10px] text-muted-foreground tabular-nums">{pct.toFixed(1)}%</div>
                      </div>
                    </div>
                  );
                })}
              </div>
            )}
          </DialogContent>
        </Dialog>
      )}
    </>
  );
}
