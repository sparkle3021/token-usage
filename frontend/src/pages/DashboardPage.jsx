/**
 * 仪表盘页面：过滤器 + KPI 行 + 趋势图 + TopModels + 热力图 + Drilling。
 * 使用 useFilter 订阅全局过滤器状态，根据过滤条件实时计算聚合指标。
 */

import { useMemo, useState } from 'react';
import { useFilter } from '../store/filterStore.jsx';
import { filterDaily } from '../lib/filters.js';
import { aggregateTotals } from '../lib/aggregators.js';
import { addDays, rangeDates, compactCN, deltaPct } from '../lib/formatters.js';
import KPI from '../components/common/KPI.jsx';
import FilterBar from '../components/layout/FilterBar.jsx';
import TrendChart from '../components/charts/TrendChart.jsx';
import TopModels from '../components/charts/TopModels.jsx';
import Heatmap from '../components/charts/Heatmap/Heatmap.jsx';
import HeatmapDrillDialog from '../components/charts/Heatmap/HeatmapDrillDialog.jsx';
import DrillDrawer from '../components/tables/DrillDrawer.jsx';

export default function DashboardPage({ M, allSources, allModels, heatmapData, onRefresh }) {
  const { f, dispatch } = useFilter();
  const [trendMode, setTrendMode] = useState('stacked');
  const [drill, setDrill] = useState(null);
  const [heatmapDate, setHeatmapDate] = useState(null);

  const filtered = useMemo(() => filterDaily(M?.daily || [], f), [f, M]);

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

    for (const r of M.time || []) {
      if (r.usageDate !== todayStr) continue;
      const d = new Date(r.eventTime);
      if (isNaN(d.getTime())) continue;
      const h = d.getHours();
      hd[h].total += r.totalTokens || 0; hd[h].input += r.inputTokens || 0;
      hd[h].output += r.outputTokens || 0; hd[h].cacheRd += r.cacheReadTokens || 0;
      hd[h].reason += r.reasoningOutputTokens || 0; hd[h].cost += r.costUSD || 0;
    }

    const covered = new Set((M.time || []).filter(r => r.usageDate === todayStr).map(r => r.source));
    for (const r of M.hour || []) {
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
  }, [isHourly, dates, M, filtered]);

  const sparkValues = useMemo(
    () => hourlySpark ? hourlySpark.total : dates.map(d => dailyMap.get(d) || 0),
    [hourlySpark, dates, dailyMap],
  );

  const sparkBy = useMemo(() => (key) => {
    if (hourlySpark) {
      const m = { totalTokens: 'total', inputTokens: 'input', outputTokens: 'output', cacheReadTokens: 'cacheRead', reasoningOutputTokens: 'reasoning', costUSD: 'cost' };
      return hourlySpark[m[key]] || hourlySpark.total;
    }
    const m = new Map();
    for (const r of filtered) m.set(r.usageDate, (m.get(r.usageDate) || 0) + (r[key] || 0));
    return dates.map(d => m.get(d) || 0);
  }, [hourlySpark, filtered, dates]);

  const presentSources = useMemo(() => Array.from(f.sources.size ? f.sources : new Set(allSources)), [f.sources, allSources]);

  const setRange = (rangeId) => {
    dispatch({ type: 'SET_RANGE', rangeId, daily: M?.daily || [] });
    onRefresh();
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
        onRefresh={onRefresh}
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
          <TrendChart rows={filtered} dates={dates} sources={presentSources} mode={trendMode} onModeChange={setTrendMode} totals={totals} timeRows={M?.time} hourRows={M?.hour} isHourly={f.rangeId === 'today'} />
        </div>
        <div className="lg:w-80 2xl:w-96 shrink-0 max-lg:min-h-0 lg:relative">
          <div className="flex flex-col min-h-0 max-lg:h-auto lg:absolute lg:inset-0">
            <TopModels rows={filtered} onDrillModel={r => setDrill({ kind: 'model', row: r })} />
          </div>
        </div>
      </div>

      <Heatmap data={heatmapData} onSelect={setHeatmapDate} />
      {heatmapDate && <HeatmapDrillDialog date={heatmapDate} daily={M?.daily} timeRows={M?.time} hourRows={M?.hour} onClose={() => setHeatmapDate(null)} />}

      <DrillDrawer drill={drill} daily={M?.daily} timeRows={M?.time} onClose={() => setDrill(null)} />
    </>
  );
}
