import React, { useMemo } from 'react';
import { BarChart, Bar, XAxis, YAxis, CartesianGrid, Tooltip, ResponsiveContainer } from 'recharts';
import { Dialog, DialogContent, DialogHeader, DialogTitle } from '../../ui/dialog.jsx';
import { Card, CardContent } from '../../ui/card.jsx';
import SourceBadge from '../../SourceBadge.jsx';
import * as U from '../../../lib/utils.js';

export default function HeatmapDrillDialog({ date, daily, timeRows, hourRows, onClose }) {
  const dayDaily = useMemo(() => {
    if (!daily || !date) return [];
    return daily.filter(r => r.usageDate === date);
  }, [daily, date]);

  const daySources = useMemo(() => {
    return [...new Set(dayDaily.map(r => r.source))];
  }, [dayDaily]);

  const dayTotals = useMemo(() => U.aggregateTotals(dayDaily), [dayDaily]);

  // Hourly data from timeRows + hourRows + daily fallback
  const hourlyData = useMemo(() => {
    if (!date) return [];
    const byHour = new Map();

    // 1) event-level time usage
    for (const r of timeRows || []) {
      if (r.usageDate !== date) continue;
      const d = new Date(r.eventTime);
      if (isNaN(d.getTime())) continue;
      const hour = String(d.getHours()).padStart(2, '0');
      byHour.set(`${hour}::${r.source}`, (byHour.get(`${hour}::${r.source}`) || 0) + r.totalTokens);
    }

    // 2) hour-level usage — fill hours not covered by timeRows
    for (const r of hourRows || []) {
      if (r.usageDate !== date) continue;
      const hour = String(r.hour).padStart(2, '0');
      const key = `${hour}::${r.source}`;
      if (byHour.has(key)) continue;
      byHour.set(key, (byHour.get(key) || 0) + r.totalTokens);
    }

    // 3) daily-only sources — put into current hour
    const currentHour = String(new Date().getHours()).padStart(2, '0');
    for (const r of dayDaily) {
      const key = `${currentHour}::${r.source}`;
      if (byHour.has(key)) continue;
      byHour.set(key, (byHour.get(key) || 0) + r.totalTokens);
    }

    return Array.from({ length: 24 }, (_, h) => {
      const hourStr = String(h).padStart(2, '0');
      const pt = { hour: `${hourStr}:00` };
      for (const s of daySources) pt[s] = byHour.get(`${hourStr}::${s}`) || 0;
      return pt;
    });
  }, [timeRows, hourRows, date, daySources, dayDaily]);

  // Top models for the day
  const topModels = useMemo(() => {
    const m = new Map();
    for (const r of dayDaily) {
      if (!r.model) continue;
      if (!m.has(r.model)) m.set(r.model, { model: r.model, total: 0, cost: 0, source: r.source });
      const x = m.get(r.model);
      x.total += r.totalTokens || 0;
      x.cost += r.costUSD || 0;
    }
    return [...m.values()].sort((a, b) => b.total - a.total).slice(0, 8);
  }, [dayDaily]);

  const palette = daySources.map(s => U.getSourceColor(s));
  const hasHourly = hourlyData.some(pt => daySources.some(s => pt[s] > 0));

  return (
    <Dialog open onOpenChange={o => { if (!o) onClose(); }}>
      <DialogContent className="sm:max-w-2xl max-h-[85vh] overflow-y-auto">
        <DialogHeader>
          <DialogTitle>{date} 用量详情</DialogTitle>
        </DialogHeader>

        {/* Summary bar */}
        <div className="flex items-center gap-4 text-sm text-muted-foreground mb-2">
          <span>总用量: <strong className="text-foreground">{U.compactCN(dayTotals.totalTokens)}</strong></span>
          <span>费用: <strong className="text-foreground">${(dayTotals.costUSD || 0).toFixed(2)}</strong></span>
          <span>模型: <strong className="text-foreground">{topModels.length}</strong></span>
        </div>

        {/* Hourly trend */}
        <Card>
          <CardContent className="pt-4">
            <h4 className="text-sm font-medium mb-3">Token 使用趋势（24 小时）</h4>
            {hasHourly ? (
              <div style={{ height: 200 }}>
                <ResponsiveContainer width="100%" height="100%">
                  <BarChart data={hourlyData} margin={{ top: 4, right: 4, bottom: 0, left: -12 }}>
                    <CartesianGrid strokeDasharray="3 3" stroke="var(--color-border)" />
                    <XAxis dataKey="hour" tick={{ fontSize: 10 }} interval={3} />
                    <YAxis tick={{ fontSize: 10 }} tickFormatter={v => U.compactCN(v)} width={50} />
                    <Tooltip
                      formatter={(v, name) => [U.compactCN(v), name]}
                      labelFormatter={label => `${date} ${label}`}
                    />
                    {daySources.map((s, i) => (
                      <Bar key={s} dataKey={s} stackId="a" fill={palette[i % palette.length]} radius={[2, 2, 0, 0]} />
                    ))}
                  </BarChart>
                </ResponsiveContainer>
              </div>
            ) : (
              <div className="flex items-center justify-center h-32 text-muted-foreground text-sm">
                暂无小时级数据
              </div>
            )}
          </CardContent>
        </Card>

        {/* Top models */}
        <Card>
          <CardContent className="pt-4">
            <h4 className="text-sm font-medium mb-3">Top 模型</h4>
            {topModels.length > 0 ? (
              <div className="space-y-2">
                {topModels.map((m, i) => (
                  <div key={m.model} className="flex items-center justify-between gap-3 px-2 py-1.5 rounded-md bg-muted/30">
                    <div className="flex items-center gap-2 min-w-0 flex-1">
                      <span className="text-xs text-muted-foreground w-4 shrink-0 text-right">#{i + 1}</span>
                      <SourceBadge source={m.source} />
                      <span className="text-xs font-medium truncate">{m.model}</span>
                    </div>
                    <div className="text-right shrink-0">
                      <span className="text-xs font-semibold tabular-nums">{U.compactCN(m.total)}</span>
                      {m.cost > 0 && <span className="text-[10px] text-muted-foreground ml-2">${m.cost.toFixed(2)}</span>}
                    </div>
                  </div>
                ))}
              </div>
            ) : (
              <div className="text-sm text-muted-foreground">无数据</div>
            )}
          </CardContent>
        </Card>
      </DialogContent>
    </Dialog>
  );
}
