import { aggregateRows, compact, compactCN, computeActivityStats } from '@/lib/formatters.js';
import { getSourceColor, getSourceIconUrl } from '@/lib/iconMap.js';
import { oklchToHex } from '@/lib/oklch.js';
import { useMemo, useRef } from 'react';
import { Card, Button, Skeleton } from 'antd';
import useECharts, { getChartTheme } from '@/lib/useECharts.js';

const MODES = [{ id: 'stacked', label: '堆叠' }, { id: 'line', label: '折线' }, { id: 'bar', label: '柱状' }];
const GRANULARITIES = [{ id: 'daily', label: '日' }, { id: 'weekly', label: '周' }, { id: 'monthly', label: '月' }, { id: 'yearly', label: '年' }];

/** ECharts tooltip formatter：复刻 recharts CTooltip（来源图标 + 值 + 合计行） */
function tooltipHtml(params) {
  const label = params[0]?.axisValue ?? '';
  const total = params.reduce((s, p) => s + (p.value || 0), 0);
  const rows = params
    .filter(p => p.seriesName != null)
    .map(p => `
      <div class="flex items-center gap-2 mt-0.5">
        <img src="${getSourceIconUrl(p.seriesName)}" class="w-3 h-3 shrink-0" alt="" />
        <span class="text-muted-foreground">${p.seriesName}</span>
        <span class="font-semibold ml-auto tabular-nums">${compactCN(p.value)}</span>
      </div>`)
    .join('');
  const totalRow = params.length > 1
    ? `<div class="flex items-center justify-between gap-2 mt-1.5 pt-1.5 border-t font-semibold"><span>合计</span><span class="tabular-nums">${compactCN(total)}</span></div>`
    : '';
  return `<div class="bg-popover text-popover-foreground shadow-lg border rounded-lg p-2.5 text-xs"><div class="font-semibold mb-1.5">${label}</div>${rows}${totalRow}</div>`;
}

export default function TrendChart({ rows, dates, sources, mode, onModeChange, totals, timeRows, hourRows, isHourly, showGranularity = false, granularity = 'daily', onGranularityChange }) {
  const byKey = useMemo(() => {
    const m = new Map();
    if (isHourly && (timeRows?.length || hourRows?.length)) {
      const todayStr = dates[0];
      // hour_usage 优先（权威聚合：time_usage 汇总 + CC-Switch 直接写入），
      // time_usage 兜底补缺——避免 time_usage 有缺口时图表少数据
      for (const r of hourRows || []) {
        if (r.usageDate !== todayStr) continue;
        const hour = String(r.hour).padStart(2, '0');
        const key = `${hour}::${r.source}`;
        m.set(key, (m.get(key) || 0) + r.totalTokens);
      }
      for (const r of timeRows || []) {
        if (r.usageDate !== todayStr) continue;
        const d = new Date(r.eventTime);
        if (isNaN(d.getTime())) continue;
        const hour = String(d.getHours()).padStart(2, '0');
        const key = `${hour}::${r.source}`;
        if (m.has(key)) continue;
        m.set(key, (m.get(key) || 0) + r.totalTokens);
      }
      // 纯日级来源兜底：今天归当前小时（进行中）；昨天已结束归 23 点（有小时数据的来源不兜底）
      const now = new Date();
      const todayLocal = `${now.getFullYear()}-${String(now.getMonth() + 1).padStart(2, '0')}-${String(now.getDate()).padStart(2, '0')}`;
      const fallbackHour = String(todayStr === todayLocal ? now.getHours() : 23).padStart(2, '0');
      const hourlySources = new Set();
      for (const r of hourRows || []) if (r.usageDate === todayStr) hourlySources.add(r.source);
      for (const r of timeRows || []) if (r.usageDate === todayStr) hourlySources.add(r.source);
      for (const r of rows) {
        if (r.usageDate !== todayStr || hourlySources.has(r.source)) continue;
        const key = `${fallbackHour}::${r.source}`;
        m.set(key, (m.get(key) || 0) + r.totalTokens);
      }
    } else {
      for (const r of rows) {
        const key = `${r.usageDate}::${r.source}`;
        m.set(key, (m.get(key) || 0) + r.totalTokens);
      }
    }
    return m;
  }, [rows, timeRows, hourRows, isHourly, dates]);

  const hasHourly = isHourly && (!!timeRows?.length || !!hourRows?.length);

  // Aggregation & chart data
  const { chartData, aggBuckets, aggUnit, aggCount } = useMemo(() => {
    if (hasHourly) {
      const data = Array.from({ length: 24 }, (_, h) => {
        const hourStr = String(h).padStart(2, '0');
        const pt = { hour: `${hourStr}:00` };
        for (const s of sources) pt[s] = byKey.get(`${hourStr}::${s}`) || 0;
        return pt;
      });
      return { chartData: data, aggBuckets: null, aggUnit: '小时', aggCount: 24 };
    }

    if (granularity === 'daily') {
      const data = dates.map(d => {
        const pt = { date: d.slice(5) };
        for (const s of sources) pt[s] = byKey.get(`${d}::${s}`) || 0;
        return pt;
      });
      return { chartData: data, aggBuckets: null, aggUnit: '天', aggCount: dates.length };
    }

    const buckets = aggregateRows(rows, dates, granularity);
    const labelKey = granularity === 'weekly' ? 'week' : granularity === 'monthly' ? 'month' : 'year';
    const data = [...buckets.entries()].map(([label, sourceMap]) => {
      const pt = { [labelKey]: label };
      for (const s of sources) pt[s] = sourceMap.get(s) || 0;
      return pt;
    });
    const unitMap = { weekly: '周', monthly: '个月', yearly: '年' };
    return { chartData: data, aggBuckets: buckets, aggUnit: unitMap[granularity], aggCount: data.length };
  }, [rows, dates, sources, byKey, hasHourly, granularity]);

  // Activity stats
  const activityStats = useMemo(() => {
    if (!aggBuckets) {
      // daily: compute from byKey/dates
      const dateSet = new Set(rows.map(r => r.usageDate));
      const active = dateSet.size;
      let longestGap = 0, currentGap = 0;
      for (const date of dates) {
        if (dateSet.has(date)) { currentGap = 0; }
        else { currentGap++; longestGap = Math.max(longestGap, currentGap); }
      }
      return { active, longestGap };
    }
    return computeActivityStats(aggBuckets);
  }, [aggBuckets, rows, dates]);

  const activeSources = useMemo(
    () => sources.filter(s => chartData.some(pt => pt[s] > 0)),
    [sources, chartData],
  );

  const palette = activeSources.map(s => oklchToHex(getSourceColor(s)));
  const dataKey = chartData[0]?.hour != null ? 'hour' : chartData[0]?.week != null ? 'week' : chartData[0]?.month != null ? 'month' : chartData[0]?.year != null ? 'year' : 'date';

  const descParts = [
    totals?.totalTokens != null ? `${compactCN(totals.totalTokens)} tokens` : '',
    hasHourly ? '24 小时' : `${aggCount} ${aggUnit}`,
    granularity !== 'daily' && !hasHourly ? `按${granularity === 'weekly' ? '周' : granularity === 'monthly' ? '月' : '年'}聚合` : '',
    !hasHourly ? `活跃 ${activityStats.active} ${aggUnit}` : '',
    !hasHourly && activityStats.longestGap > 0 ? `最长空白 ${activityStats.longestGap} ${aggUnit}` : '',
  ].filter(Boolean).join(' · ');

  const optionRef = useRef(null);
  const { setChartEl, dark, ready } = useECharts(optionRef, [chartData, palette, activeSources, mode, dataKey]);
  const theme = getChartTheme(dark);

  const option = useMemo(() => {
    const xData = chartData.map(pt => pt[dataKey]);
    const series = activeSources.map((s, i) => ({
      name: s,
      type: mode === 'line' ? 'line' : 'bar',
      stack: mode === 'stacked' ? 'total' : undefined,
      data: chartData.map(pt => pt[s] || 0),
      itemStyle: { color: palette[i], borderRadius: mode === 'line' ? undefined : [2, 2, 0, 0] },
      barMaxWidth: 24,
      lineStyle: { width: 2 },
      smooth: mode === 'line',
      showSymbol: false,
    }));
    return {
      grid: { top: 8, right: 8, bottom: 22, left: 44 },
      xAxis: {
        type: 'category',
        data: xData,
        axisLine: { show: false },
        axisTick: { show: false },
        axisLabel: { fontSize: 10.5, color: theme.axisText },
      },
      yAxis: {
        type: 'value',
        axisLine: { show: false },
        axisTick: { show: false },
        splitLine: { lineStyle: { color: theme.grid } },
        axisLabel: { fontSize: 10.5, color: theme.axisTick, formatter: (v) => compact(v) },
        splitNumber: 4,
      },
      tooltip: {
        trigger: 'axis',
        backgroundColor: 'transparent',
        borderWidth: 0,
        padding: 0,
        formatter: (params) => tooltipHtml(params),
        extraCssText: 'box-shadow:none;',
      },
      series,
    };
  }, [chartData, palette, activeSources, mode, dataKey, theme]);
  optionRef.current = option;

  return (
    <Card styles={{ body: { padding: 16 } }}>
      <div className="flex flex-col gap-2 mb-4">
        <div className="flex items-center justify-between gap-4">
          <div>
            <div className="text-sm font-semibold">Token 使用趋势</div>
            <div className="text-xs text-muted-foreground">{descParts}</div>
          </div>
          <div className="flex items-center gap-2 shrink-0">
            {/* Granularity selector：仅在「全部」范围显示（日/周/月/年聚合只对长跨度有意义），其余范围固定日粒度 */}
            {!hasHourly && showGranularity && (
              <div className="flex gap-0.5 bg-muted rounded-lg p-0.5">
                {GRANULARITIES.filter(g => {
                  if (g.id === 'yearly') return dates.length > 730;
                  return true;
                }).map(g => (
                  <Button key={g.id} size="small" type={granularity === g.id ? 'primary' : 'text'}
                    className="h-6 text-xs px-2" onClick={() => onGranularityChange?.(g.id)}>{g.label}</Button>
                ))}
              </div>
            )}
            <div className="flex gap-0.5 bg-muted rounded-lg p-0.5">
              {MODES.map(m => (
                <Button key={m.id} size="small" type={mode === m.id ? 'primary' : 'text'} className="h-6 text-xs px-2" onClick={() => onModeChange(m.id)}>{m.label}</Button>
              ))}
            </div>
          </div>
        </div>
      </div>
      <div className="relative" style={{ minHeight: 280, height: 'clamp(280px, 35vh, 400px)' }}>
        {/* 图表 div 自身带尺寸（height:100% 在父 clamp 下可能解析失败→0 高→空白），并始终渲染避免 removeChild 冲突 */}
        <div ref={setChartEl} style={{ minHeight: 280, height: 'clamp(280px, 35vh, 400px)' }} />
        {!ready && (
          <div className="absolute inset-0 overflow-hidden bg-background/50">
            <Skeleton active paragraph={{ rows: 5 }} />
          </div>
        )}
        {activeSources.length === 0 && (
          <div className="absolute inset-0 flex items-center justify-center">
            <span className="text-sm text-muted-foreground">当前时间范围内无数据</span>
          </div>
        )}
      </div>
    </Card>
  );
}
