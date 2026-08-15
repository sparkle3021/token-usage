/**
 * 仪表盘页面：过滤器 + KPI 行 + 趋势图 + TopModels + 热力图 + Drilling。
 * 使用 useFilter 订阅全局过滤器状态，根据过滤条件实时计算聚合指标。
 */

import { aggregateField, aggregateMapToArray, addDays, compactCN, deltaPct, rangeDates } from '@/lib/formatters.js';
import { getSourceColor } from '@/lib/iconMap.js';
import { useMemo, useState } from 'react';
import { useFilter, getRangeDays, getCompareSpan } from '@/store/filterStore.jsx';
import { filterDaily, filterTimeSeries } from '@/lib/filters.js';
import { aggregateTotals } from '@/lib/aggregators.js';
import KPI from '@/components/common/KPI.jsx';
import FilterBar from '@/components/layout/FilterBar.jsx';
import TrendChart from '@/components/charts/TrendChart.jsx';
import TopModels from '@/components/charts/TopModels.jsx';
import Heatmap from '@/components/charts/Heatmap/Heatmap.jsx';
import HeatmapDrillDialog from '@/components/dialogs/HeatmapDrillDialog.jsx';
import { Modal } from 'antd';
import SourceIcon from '@/components/common/SourceIcon.jsx';

export default function DashboardPage({ M, allSources, allModels, heatmapData, onRangeSwitch }) {
  const { f, dispatch } = useFilter();
  const [trendMode, setTrendMode] = useState('stacked');
  const [topModelDrill, setTopModelDrill] = useState(null);
  const [heatmapDate, setHeatmapDate] = useState(null);
  const [granularity, setGranularity] = useState('daily');

  const filtered = useMemo(() => filterDaily(M?.daily || [], f), [f, M]);

  const filteredTime = useMemo(() => filterTimeSeries(M?.time || [], f), [f, M]);
  const filteredHour = useMemo(() => filterTimeSeries(M?.hour || [], f), [f, M]);

  const dates = useMemo(() => rangeDates(f.startDate, f.endDate), [f.startDate, f.endDate]);

  const isToday = f.rangeId === 'today';
  // 小时视图：今天 + 昨天（昨天也看 24h 分布）
  const isHourly = isToday || f.rangeId === 'yesterday';

  const hourlySpark = useMemo(() => {
    if (!isHourly || !M) return null;
    const todayStr = dates[0];
    const hd = Array.from({ length: 24 }, () => ({ total: 0, input: 0, output: 0, cacheRd: 0, reason: 0, cost: 0 }));

    // hour_usage 优先（权威聚合：time_usage 汇总 + CC-Switch 直接写入），
    // time_usage 兜底补缺——避免 time_usage 有缺口时 sparkline 少数据
    const covered = new Set();
    for (const r of filteredHour) {
      if (r.usageDate !== todayStr) continue;
      const key = `${r.source}::${r.hour}`;
      const h = r.hour;
      hd[h].total += r.totalTokens || 0; hd[h].input += r.inputTokens || 0;
      hd[h].output += r.outputTokens || 0; hd[h].cacheRd += r.cacheReadTokens || 0;
      hd[h].reason += r.reasoningOutputTokens || 0; hd[h].cost += r.costUSD || 0;
      covered.add(key);
    }

    for (const r of filteredTime) {
      const d = new Date(r.eventTime);
      if (isNaN(d.getTime())) continue;
      // event_time 为 RFC3339 UTC（新数据）或本地串（存量单机），
      // 统一按展示机本地时区解析出本地日/时，与后端平移后的 hour 桶对齐
      const localDate = `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`;
      if (localDate !== todayStr) continue;
      const h = d.getHours();
      if (covered.has(`${r.source}::${h}`)) continue;
      hd[h].total += r.totalTokens || 0; hd[h].input += r.inputTokens || 0;
      hd[h].output += r.outputTokens || 0; hd[h].cacheRd += r.cacheReadTokens || 0;
      hd[h].reason += r.reasoningOutputTokens || 0; hd[h].cost += r.costUSD || 0;
      covered.add(`${r.source}::${h}`);
    }

    // 纯日级来源兜底：今天归到当前小时（进行中）；昨天已结束归到 23 点（有小时数据的来源不兜底）
    const fallbackHour = isToday ? new Date().getHours() : 23;
    const hourlySources = new Set();
    for (const r of filteredHour) if (r.usageDate === todayStr) hourlySources.add(r.source);
    for (const r of filteredTime) if (r.usageDate === todayStr) hourlySources.add(r.source);
    for (const r of filtered) {
      if (r.usageDate !== todayStr || hourlySources.has(r.source)) continue;
      hd[fallbackHour].total += r.totalTokens || 0; hd[fallbackHour].input += r.inputTokens || 0;
      hd[fallbackHour].output += r.outputTokens || 0; hd[fallbackHour].cacheRd += r.cacheReadTokens || 0;
      hd[fallbackHour].reason += r.reasoningOutputTokens || 0; hd[fallbackHour].cost += r.costUSD || 0;
    }

    return {
      total: hd.map(h => h.total), input: hd.map(h => h.input),
      output: hd.map(h => h.output), cacheRead: hd.map(h => h.cacheRd),
      reasoning: hd.map(h => h.reason), cost: hd.map(h => h.cost),
    };
  }, [isToday, isHourly, dates, M, filtered, filteredTime, filteredHour]);

  // KPI 总量：今天用小时级数据精确聚合（hour 按本地时区平移后含本地今天全天，
  // 避免 daily 表按 UTC 日拆分导致本地今天凌晨数据漏算）；昨天/其他范围用 daily 聚合
  // （昨天的 daily 含无小时粒度的来源，用 hourlySpark 会少算，保持 daily 为准）。
  const totals = useMemo(() => {
    if (isToday && hourlySpark) {
      const sum = arr => arr.reduce((a, b) => a + b, 0);
      const total = sum(hourlySpark.total);
      const cacheRd = sum(hourlySpark.cacheRead);
      return {
        totalTokens: total,
        inputTokens: sum(hourlySpark.input),
        outputTokens: sum(hourlySpark.output),
        cacheReadTokens: cacheRd,
        cacheCreationTokens: 0,
        cacheTokens: cacheRd,
        reasoningTokens: sum(hourlySpark.reasoning),
        costUSD: sum(hourlySpark.cost),
        cacheHitRate: total ? (cacheRd / total) * 100 : 0,
      };
    }
    return aggregateTotals(filtered);
  }, [isToday, hourlySpark, filtered]);

  const compareData = useMemo(() => {
    if (!f.compare) return { totals: null };
    // 对比周期按范围语义：昨天→前天、近7天→前7天、本月→上月、上月→上上月；全部按等长平移
    let span;
    if (f.rangeId === 'all') {
      const days = dates.length;
      const endStr = addDays(f.startDate, -1);
      span = { start: addDays(endStr, -(days - 1)), end: endStr };
    } else {
      span = getCompareSpan(f.rangeId, f.startDate);
    }
    if (!span) return { totals: null };
    return { totals: aggregateTotals(filterDaily(M?.daily || [], { ...f, startDate: span.start, endDate: span.end })) };
  }, [f, dates, M]);

  const dailyMap = useMemo(() => {
    const m = new Map();
    for (const r of filtered) m.set(r.usageDate, (m.get(r.usageDate) || 0) + r.totalTokens);
    return m;
  }, [filtered]);

  const sparkValues = useMemo(
    () => hourlySpark ? hourlySpark.total
      : granularity !== 'daily' ? aggregateMapToArray(dailyMap, dates, granularity)
      : dates.map(d => dailyMap.get(d) || 0),
    [hourlySpark, granularity, dailyMap, dates],
  );

  const sparkBy = useMemo(() => (key) => {
    if (hourlySpark) {
      const m = { totalTokens: 'total', inputTokens: 'input', outputTokens: 'output', cacheReadTokens: 'cacheRead', reasoningOutputTokens: 'reasoning', costUSD: 'cost' };
      return hourlySpark[m[key]] || hourlySpark.total;
    }
    if (granularity !== 'daily') return aggregateField(filtered, dates, granularity, key);
    const m = new Map();
    for (const r of filtered) m.set(r.usageDate, (m.get(r.usageDate) || 0) + (r[key] || 0));
    return dates.map(d => m.get(d) || 0);
  }, [hourlySpark, granularity, filtered, dates]);

  const presentSources = useMemo(() => Array.from(f.sources.size ? f.sources : new Set(allSources)), [f.sources, allSources]);

  const setRange = (rangeId) => {
    dispatch({ type: 'SET_RANGE', rangeId, daily: M?.daily || [] });
    // 切时间：本地立即渲染（dispatch 已触发），后台异步补拉时间序列
    let days = getRangeDays(rangeId);
    onRangeSwitch?.(days);
    // Compute expected range length for granularity recommendation
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
    <div className="flex flex-col gap-4 h-full min-h-0">
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
          <TrendChart rows={filtered} dates={dates} sources={presentSources} mode={trendMode} onModeChange={setTrendMode} totals={totals} timeRows={filteredTime} hourRows={filteredHour} isHourly={isHourly} showGranularity={f.rangeId === 'all'} granularity={granularity} onGranularityChange={setGranularity} />
        </div>
        <div className="lg:w-80 2xl:w-96 shrink-0 max-lg:min-h-0 lg:relative">
          <div className="flex flex-col min-h-0 max-lg:h-auto lg:absolute lg:inset-0">
            <TopModels rows={filtered} onDrillModel={r => setTopModelDrill(r)} allDaily={M?.daily} />
          </div>
        </div>
      </div>

      {/* 热力图：独立区域（图表行下方全宽），格子尺寸随容器自适应 */}
      <Heatmap data={heatmapData} onSelect={setHeatmapDate} />
      {heatmapDate && <HeatmapDrillDialog date={heatmapDate} daily={M?.daily} timeRows={M?.time} hourRows={M?.hour} onClose={() => setHeatmapDate(null)} />}

      {topModelDrill && (
        <Modal
          open
          onCancel={() => setTopModelDrill(null)}
          title={null}
          centered
          width={{ xs: 520, md: 576, lg: 672, xl: 720 }}
          styles={{ body: { maxHeight: 'calc(85vh - 120px)', overflow: 'hidden', display: 'flex', flexDirection: 'column' } }}
          footer={null}
        >
          <h3 className="sr-only">{topModelDrill.model} 详情</h3>
          <p className="sr-only">模型用量详情</p>

          <div className="mb-4 shrink-0">
            <div className="text-xs text-muted-foreground mb-0.5">模型详情</div>
            <h3 className="text-sm font-semibold">{topModelDrill.model}</h3>
          </div>

          <div className="flex items-center gap-4 text-xs text-muted-foreground mb-4 shrink-0 flex-wrap">
            <span>总 Token <strong className="text-foreground">{compactCN(topModelDrill.total)}</strong></span>
            <span>费用 <strong className="text-foreground">${(topModelDrill.cost || 0).toFixed(2)}</strong></span>
            <span>活跃 <strong className="text-foreground">{topModelDrill.dayCount}</strong> 天</span>
          </div>

          <div className="flex-dialog-body min-h-0 overflow-y-auto scrollbar-subtle">
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
                        <div className="h-full" style={{ width: `${pct}%`, background: getSourceColor(s.source) }} />
                      </div>
                    </div>
                    <div className="text-right shrink-0">
                      <div className="text-xs font-semibold tabular-nums">{compactCN(s.total)}</div>
                      <div className="text-[10px] text-muted-foreground tabular-nums">{pct.toFixed(1)}%</div>
                    </div>
                  </div>
                );
              })}
            </div>
          )}
          </div>
        </Modal>
      )}
    </div>
  );
}
