import React, { useMemo } from 'react';
import { BarChart, Bar, XAxis, YAxis, CartesianGrid, Tooltip, ResponsiveContainer, LineChart, Line } from 'recharts';
import { Card, CardHeader, CardTitle, CardDescription, CardContent } from '../ui/card.jsx';
import { Button } from '../ui/button.jsx';
import * as U from '../../lib/utils.js';

const MODES = [{ id: 'stacked', label: '堆叠' }, { id: 'line', label: '折线' }, { id: 'bar', label: '柱状' }];

export default function TrendChart({ rows, dates, sources, mode, onModeChange, totals }) {
  const byKey = useMemo(() => {
    const m = new Map();
    for (const r of rows) m.set(`${r.usageDate}::${r.source}`, (m.get(`${r.usageDate}::${r.source}`) || 0) + r.totalTokens);
    return m;
  }, [rows]);

  const chartData = useMemo(() => dates.map(d => {
    const pt = { date: d.slice(5) };
    for (const s of sources) pt[s] = byKey.get(`${d}::${s}`) || 0;
    return pt;
  }), [dates, sources, byKey]);

  const palette = sources.map(s => U.getSourceColor(s));

  return (
    <Card>
      <CardHeader>
        <div className="flex items-center justify-between gap-4">
          <div>
            <CardTitle>每日 Token 使用趋势</CardTitle>
            <CardDescription>{totals?.totalTokens != null ? `${U.compactCN(totals.totalTokens)} tokens · ${dates.length} 天` : ''}</CardDescription>
          </div>
          <div className="flex gap-0.5 bg-muted rounded-lg p-0.5">
            {MODES.map(m => (
              <Button key={m.id} size="xs" variant={mode === m.id ? 'default' : 'ghost'} onClick={() => onModeChange(m.id)}>{m.label}</Button>
            ))}
          </div>
        </div>
      </CardHeader>
      <CardContent>
        <div style={{ height: 320 }}>
          <ResponsiveContainer width="100%" height="100%">
            {mode === 'line' ? (
              <LineChart data={chartData}>
                <CartesianGrid strokeDasharray="3 3" stroke="oklch(0.93 0.004 80)" />
                <XAxis dataKey="date" tick={{ fontSize: 10.5, fill: 'oklch(0.55 0.005 80)' }} />
                <YAxis tick={{ fontSize: 10.5, fill: 'oklch(0.62 0.004 80)' }} tickFormatter={v => U.compact(v)} />
                <Tooltip content={<CTooltip />} />
                {sources.map((s, i) => (<Line key={s} type="monotone" dataKey={s} stroke={palette[i]} strokeWidth={2} dot={false} />))}
              </LineChart>
            ) : (
              <BarChart data={chartData}>
                <CartesianGrid strokeDasharray="3 3" stroke="oklch(0.93 0.004 80)" />
                <XAxis dataKey="date" tick={{ fontSize: 10.5, fill: 'oklch(0.55 0.005 80)' }} />
                <YAxis tick={{ fontSize: 10.5, fill: 'oklch(0.62 0.004 80)' }} tickFormatter={v => U.compact(v)} />
                <Tooltip content={<CTooltip />} />
                {sources.map((s, i) => (<Bar key={s} dataKey={s} stackId={mode === 'stacked' ? 'total' : undefined} fill={palette[i]} />))}
              </BarChart>
            )}
          </ResponsiveContainer>
        </div>
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
          <span className="w-2 h-2 rounded-sm" style={{ background: p.color }} />
          <span className="text-muted-foreground">{p.name}</span>
          <span className="font-semibold ml-auto tabular-nums">{U.compactCN(p.value)}</span>
        </div>
      ))}
    </div>
  );
}
