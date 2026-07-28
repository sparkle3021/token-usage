import { useMemo } from 'react';
import { BarChart, Bar, XAxis, YAxis, CartesianGrid, Tooltip, ResponsiveContainer } from 'recharts';
import { Dialog, DialogContent, DialogHeader, DialogTitle } from '../../ui/dialog.jsx';
import { Card, CardContent } from '../../ui/card.jsx';
import * as U from '../../../lib/utils.js';

export default function HeatmapDrillDialog({ date, daily, timeRows, hourRows, onClose }) {
  const dayDaily = useMemo(() => {
    if (!daily || !date) return [];
    return daily.filter(r => r.usageDate === date);
  }, [daily, date]);

  const daySources = useMemo(() => {
    return [...new Set(dayDaily.map(r => r.source))];
  }, [dayDaily]);

  // Top model for the day (just name, not full list)
  const topModel = useMemo(() => {
    const m = new Map();
    for (const r of dayDaily) {
      if (!r.model) continue;
      m.set(r.model, (m.get(r.model) || 0) + (r.totalTokens || 0));
    }
    const sorted = [...m.entries()].sort((a, b) => b[1] - a[1]);
    return sorted.length > 0 ? { name: sorted[0][0], tokens: sorted[0][1], count: sorted.length } : null;
  }, [dayDaily]);

  // Hourly data from timeRows + hourRows + daily fallback
  const hourlyData = useMemo(() => {
    if (!date) return [];
    const byHour = new Map();

    for (const r of timeRows || []) {
      if (r.usageDate !== date) continue;
      const d = new Date(r.eventTime);
      if (isNaN(d.getTime())) continue;
      const hour = String(d.getHours()).padStart(2, '0');
      byHour.set(`${hour}::${r.source}`, (byHour.get(`${hour}::${r.source}`) || 0) + r.totalTokens);
    }

    for (const r of hourRows || []) {
      if (r.usageDate !== date) continue;
      const hour = String(r.hour).padStart(2, '0');
      const key = `${hour}::${r.source}`;
      if (byHour.has(key)) continue;
      byHour.set(key, (byHour.get(key) || 0) + r.totalTokens);
    }

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

  const palette = daySources.map(s => U.getSourceColor(s));
  const hasHourly = hourlyData.some(pt => daySources.some(s => pt[s] > 0));

  return (
    <Dialog open onOpenChange={o => { if (!o) onClose(); }}>
      <DialogContent className="sm:max-w-2xl max-h-[85vh] overflow-y-auto">
        <DialogHeader>
          <DialogTitle>{date} 用量详情</DialogTitle>
        </DialogHeader>

        {topModel && (
          <div className="text-xs text-muted-foreground mb-2">
            模型数 <strong className="text-foreground">{topModel.count}</strong> · 峰值 <strong className="text-foreground">{topModel.name}</strong>（{U.compactCN(topModel.tokens)}）
          </div>
        )}

        <Card>
          <CardContent className="pt-4">
            <h4 className="text-sm font-medium mb-3">Token 使用趋势（24 小时）</h4>
            {hasHourly ? (
              <div style={{ minHeight: 160, height: 'clamp(160px, 25vh, 250px)' }}>
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
      </DialogContent>
    </Dialog>
  );
}
