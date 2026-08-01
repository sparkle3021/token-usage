import { useMemo } from 'react';
import { BarChart, Bar, XAxis, YAxis, CartesianGrid, Tooltip, ResponsiveContainer, LineChart, Line } from 'recharts';
import { Card, Button } from 'antd';
import * as U from '../../lib/utils.js';
import SourceIcon from '../SourceIcon.jsx';

const MODES = [{ id: 'stacked', label: '堆叠' }, { id: 'line', label: '折线' }, { id: 'bar', label: '柱状' }];
const GRANULARITIES = [{ id: 'daily', label: '日' }, { id: 'weekly', label: '周' }, { id: 'monthly', label: '月' }, { id: 'yearly', label: '年' }];

export default function TrendChart({ rows, dates, sources, mode, onModeChange, totals, timeRows, hourRows, isHourly, granularity = 'daily', onGranularityChange }) {
  const byKey = useMemo(() => {
    const m = new Map();
    if (isHourly && (timeRows?.length || hourRows?.length)) {
      const todayStr = dates[0];
      for (const r of timeRows || []) {
        if (r.usageDate !== todayStr) continue;
        const d = new Date(r.eventTime);
        if (isNaN(d.getTime())) continue;
        const hour = String(d.getHours()).padStart(2, '0');
        const key = `${hour}::${r.source}`;
        m.set(key, (m.get(key) || 0) + r.totalTokens);
      }
      for (const r of hourRows || []) {
        if (r.usageDate !== todayStr) continue;
        const hour = String(r.hour).padStart(2, '0');
        const key = `${hour}::${r.source}`;
        if (m.has(key)) continue;
        m.set(key, (m.get(key) || 0) + r.totalTokens);
      }
      const currentHour = String(new Date().getHours()).padStart(2, '0');
      for (const r of rows) {
        if (r.usageDate !== todayStr) continue;
        const key = `${currentHour}::${r.source}`;
        if (m.has(key)) continue;
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

    const buckets = U.aggregateRows(rows, dates, granularity);
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
    return U.computeActivityStats(aggBuckets);
  }, [aggBuckets, rows, dates]);

  const activeSources = useMemo(
    () => sources.filter(s => chartData.some(pt => pt[s] > 0)),
    [sources, chartData],
  );

  const palette = activeSources.map(s => U.getSourceColor(s));
  const dataKey = chartData[0]?.hour != null ? 'hour' : chartData[0]?.week != null ? 'week' : chartData[0]?.month != null ? 'month' : chartData[0]?.year != null ? 'year' : 'date';

  const descParts = [
    totals?.totalTokens != null ? `${U.compactCN(totals.totalTokens)} tokens` : '',
    hasHourly ? '24 小时' : `${aggCount} ${aggUnit}`,
    granularity !== 'daily' && !hasHourly ? `按${granularity === 'weekly' ? '周' : granularity === 'monthly' ? '月' : '年'}聚合` : '',
    !hasHourly ? `活跃 ${activityStats.active} ${aggUnit}` : '',
    !hasHourly && activityStats.longestGap > 0 ? `最长空白 ${activityStats.longestGap} ${aggUnit}` : '',
  ].filter(Boolean).join(' · ');

  return (
    <Card styles={{ body: { padding: 16 } }}>
      <div className="flex flex-col gap-2 mb-4">
        <div className="flex items-center justify-between gap-4">
          <div>
            <div className="text-sm font-semibold">Token 使用趋势</div>
            <div className="text-xs text-muted-foreground">{descParts}</div>
          </div>
          <div className="flex items-center gap-2 shrink-0">
            {/* Granularity selector (hidden in hourly mode) */}
            {!hasHourly && (
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
      <div>
        {activeSources.length === 0 ? (
          <div className="flex items-center justify-center" style={{ minHeight: 280, height: 'clamp(280px, 35vh, 400px)' }}>
            <span className="text-sm text-muted-foreground">当前时间范围内无数据</span>
          </div>
        ) : (
        <div style={{ minHeight: 280, height: 'clamp(280px, 35vh, 400px)' }}>
          <ResponsiveContainer width="100%" height="100%">
            {mode === 'line' ? (
              <LineChart data={chartData}>
                <CartesianGrid strokeDasharray="3 3" stroke="oklch(0.93 0.004 80)" />
                <XAxis dataKey={dataKey} tick={{ fontSize: 10.5, fill: 'oklch(0.55 0.005 80)' }} />
                <YAxis tick={{ fontSize: 10.5, fill: 'oklch(0.62 0.004 80)' }} tickFormatter={v => U.compact(v)} />
                <Tooltip content={<CTooltip />} />
                {activeSources.map((s, i) => (<Line key={s} type="monotone" dataKey={s} stroke={palette[i]} strokeWidth={2} dot={false} />))}
              </LineChart>
            ) : (
              <BarChart data={chartData}>
                <CartesianGrid strokeDasharray="3 3" stroke="oklch(0.93 0.004 80)" />
                <XAxis dataKey={dataKey} tick={{ fontSize: 10.5, fill: 'oklch(0.55 0.005 80)' }} />
                <YAxis tick={{ fontSize: 10.5, fill: 'oklch(0.62 0.004 80)' }} tickFormatter={v => U.compact(v)} />
                <Tooltip content={<CTooltip />} />
                {activeSources.map((s, i) => (<Bar key={s} dataKey={s} stackId={mode === 'stacked' ? 'total' : undefined} fill={palette[i]} />))}
              </BarChart>
            )}
          </ResponsiveContainer>
        </div>
        )}
      </div>
    </Card>
  );
}

function CTooltip({ active, payload, label }) {
  if (!active || !payload) return null;
  const total = payload.reduce((s, p) => s + (p.value || 0), 0);
  return (
    <div className="bg-popover text-popover-foreground shadow-lg border rounded-lg p-2.5 text-xs">
      <div className="font-semibold mb-1.5">{label}</div>
      {payload.map(p => (
        <div key={p.name} className="flex items-center gap-2 mt-0.5">
          <SourceIcon name={p.name} className="w-3 h-3" />
          <span className="text-muted-foreground">{p.name}</span>
          <span className="font-semibold ml-auto tabular-nums">{U.compactCN(p.value)}</span>
        </div>
      ))}
      {payload.length > 1 && (
        <div className="flex items-center justify-between gap-2 mt-1.5 pt-1.5 border-t font-semibold">
          <span>合计</span>
          <span className="tabular-nums">{U.compactCN(total)}</span>
        </div>
      )}
    </div>
  );
}
