import React, { useMemo } from 'react';
import { BarChart, Bar, XAxis, YAxis, CartesianGrid, Tooltip, ResponsiveContainer, LineChart, Line } from 'recharts';
import { Card, CardHeader, CardTitle, CardDescription, CardContent } from '../ui/card.jsx';
import { Button } from '../ui/button.jsx';
import * as U from '../../lib/utils.js';
import SourceIcon from '../SourceIcon.jsx';

const MODES = [{ id: 'stacked', label: '堆叠' }, { id: 'line', label: '折线' }, { id: 'bar', label: '柱状' }];

export default function TrendChart({ rows, dates, sources, mode, onModeChange, totals, timeRows, hourRows, isHourly }) {
  const byKey = useMemo(() => {
    const m = new Map();
    if (isHourly && (timeRows?.length || hourRows?.length)) {
      const todayStr = dates[0];

      // 1) event-level time usage (Claude Code, Codex, Gemini)
      for (const r of timeRows || []) {
        if (r.usageDate !== todayStr) continue;
        const d = new Date(r.eventTime);
        if (isNaN(d.getTime())) continue;
        const hour = String(d.getHours()).padStart(2, '0');
        const key = `${hour}::${r.source}`;
        m.set(key, (m.get(key) || 0) + r.totalTokens);
      }

      // 2) hour-level usage — fill hours not already covered by timeRows
      for (const r of hourRows || []) {
        if (r.usageDate !== todayStr) continue;
        const hour = String(r.hour).padStart(2, '0');
        const key = `${hour}::${r.source}`;
        if (m.has(key)) continue;
        m.set(key, (m.get(key) || 0) + r.totalTokens);
      }

      // 3) daily-only sources — put into current hour
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

  const chartData = useMemo(() => {
    if (hasHourly) {
      return Array.from({ length: 24 }, (_, h) => {
        const hourStr = String(h).padStart(2, '0');
        const pt = { hour: `${hourStr}:00` };
        for (const s of sources) pt[s] = byKey.get(`${hourStr}::${s}`) || 0;
        return pt;
      });
    }
    return dates.map(d => {
      const pt = { date: d.slice(5) };
      for (const s of sources) pt[s] = byKey.get(`${d}::${s}`) || 0;
      return pt;
    });
  }, [dates, sources, byKey, hasHourly]);

  const activeSources = useMemo(
    () => sources.filter(s => chartData.some(pt => pt[s] > 0)),
    [sources, chartData],
  );

  const palette = activeSources.map(s => U.getSourceColor(s));

  return (
    <Card>
      <CardHeader>
        <div className="flex items-center justify-between gap-4">
          <div>
            <CardTitle>Token 使用趋势</CardTitle>
            <CardDescription>{totals?.totalTokens != null ? `${U.compactCN(totals.totalTokens)} tokens · ${hasHourly ? '24 小时' : dates.length + ' 天'}` : ''}</CardDescription>
          </div>
          <div className="flex gap-0.5 bg-muted rounded-lg p-0.5">
            {MODES.map(m => (
              <Button key={m.id} size="xs" variant={mode === m.id ? 'default' : 'ghost'} onClick={() => onModeChange(m.id)}>{m.label}</Button>
            ))}
          </div>
        </div>
      </CardHeader>
      <CardContent>
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
                <XAxis dataKey={hasHourly ? 'hour' : 'date'} tick={{ fontSize: 10.5, fill: 'oklch(0.55 0.005 80)' }} />
                <YAxis tick={{ fontSize: 10.5, fill: 'oklch(0.62 0.004 80)' }} tickFormatter={v => U.compact(v)} />
                <Tooltip content={<CTooltip />} />
                {activeSources.map((s, i) => (<Line key={s} type="monotone" dataKey={s} stroke={palette[i]} strokeWidth={2} dot={false} />))}
              </LineChart>
            ) : (
              <BarChart data={chartData}>
                <CartesianGrid strokeDasharray="3 3" stroke="oklch(0.93 0.004 80)" />
                <XAxis dataKey={hasHourly ? 'hour' : 'date'} tick={{ fontSize: 10.5, fill: 'oklch(0.55 0.005 80)' }} />
                <YAxis tick={{ fontSize: 10.5, fill: 'oklch(0.62 0.004 80)' }} tickFormatter={v => U.compact(v)} />
                <Tooltip content={<CTooltip />} />
                {activeSources.map((s, i) => (<Bar key={s} dataKey={s} stackId={mode === 'stacked' ? 'total' : undefined} fill={palette[i]} />))}
              </BarChart>
            )}
          </ResponsiveContainer>
        </div>
        )}
      </CardContent>
    </Card>
  );
}

function CTooltip({ active, payload, label }) {
  if (!active || !payload) return null;
  return (
    <div className="bg-white shadow-lg border rounded-lg p-2.5 text-xs">
      <div className="font-semibold mb-1 text-foreground">{label}</div>
      {payload.map(p => (
        <div key={p.name} className="flex items-center gap-2 mt-0.5">
          <SourceIcon name={p.name} className="w-3 h-3" />
          <span className="text-muted-foreground">{p.name}</span>
          <span className="font-semibold ml-auto tabular-nums">{U.compactCN(p.value)}</span>
        </div>
      ))}
    </div>
  );
}
