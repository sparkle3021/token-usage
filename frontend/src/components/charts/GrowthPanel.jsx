import React, { useMemo } from 'react';
import { Card, CardHeader, CardTitle, CardDescription, CardContent } from '../ui/card.jsx';
import * as U from '../../lib/utils.js';

export default function GrowthPanel({ totalsByDay }) {
  const stats = useMemo(() => {
    const entries = [...(totalsByDay?.entries?.() || [])].sort((a, b) => String(a[0]).localeCompare(String(b[0])));
    const values = entries.map(d => d[1]);
    const n = values.length;
    const today = values[n - 1] || 0;
    const dod = n >= 2 ? U.deltaPct(today, values[n - 2] || 0) : null;
    const last7 = values.slice(-7).reduce((s, v) => s + v, 0);
    const wow = n >= 14 ? U.deltaPct(last7, values.slice(-14, -7).reduce((s, v) => s + v, 0)) : null;
    const last30 = values.slice(-30).reduce((s, v) => s + v, 0);
    const mom = n >= 60 ? U.deltaPct(last30, values.slice(-60, -30).reduce((s, v) => s + v, 0)) : null;
    let bestIdx = 0;
    values.forEach((v, i) => { if (v > values[bestIdx]) bestIdx = i; });
    const avg = n ? Math.round(values.reduce((s, v) => s + v, 0) / n) : 0;
    return { dod, wow, mom, avg, today, last7, last30, bestDate: entries[bestIdx]?.[0], bestVal: values[bestIdx] };
  }, [totalsByDay]);

  return (
    <Card>
      <CardHeader><CardTitle>环比与趋势</CardTitle><CardDescription>基于当前筛选周期</CardDescription></CardHeader>
      <CardContent>
        <div className="space-y-1.5">
          <Stat label="日环比 DoD" value={stats.dod} sub={`今日 ${U.compactCN(stats.today)}`} />
          <Stat label="周环比 WoW" value={stats.wow} sub={`7 日 ${U.compactCN(stats.last7)}`} />
          <Stat label="月环比 MoM" value={stats.mom} sub={`30 日 ${U.compactCN(stats.last30)}`} />
          <Stat label="日均" value={null} sub={U.compactCN(stats.avg)} subUnit="tokens/天" />
        </div>
        {stats.bestDate && (
          <div className="mt-3 p-2.5 bg-muted rounded-lg text-[11px] flex items-center gap-2">
            <span className="text-amber-500 shrink-0">★</span>
            <span>峰值 <b>{stats.bestDate}</b> · {U.compactCN(stats.bestVal)} tokens</span>
          </div>
        )}
      </CardContent>
    </Card>
  );
}

function Stat({ label, value, sub, subUnit }) {
  return (
    <div className="p-2.5 bg-muted rounded-lg flex items-center justify-between gap-2">
      <div className="min-w-0">
        <div className="text-[10px] text-muted-foreground uppercase tracking-wider">{label}</div>
        <div className="text-[11px] text-muted-foreground truncate">{value != null ? sub : (subUnit || '')}</div>
      </div>
      <div className={`text-base font-semibold tabular-nums shrink-0 ${value == null ? 'text-foreground' : value > 0 ? 'text-green-600' : value < 0 ? 'text-red-500' : 'text-foreground'}`}>
        {value != null ? `${value > 0 ? '+' : ''}${value.toFixed(1)}%` : sub}
      </div>
    </div>
  );
}
