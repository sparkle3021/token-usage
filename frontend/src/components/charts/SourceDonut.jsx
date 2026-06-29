import React, { useMemo } from 'react';
import { PieChart, Pie, Cell, ResponsiveContainer } from 'recharts';
import { Card, CardHeader, CardTitle, CardDescription, CardContent } from '../ui/card.jsx';
import * as U from '../../lib/utils.js';
import SourceIcon from '../SourceIcon.jsx';

export default function SourceDonut({ rows, focused, onFocusSource }) {
  const data = useMemo(() => {
    const sources = [...new Set(rows.map(r => r.source))];
    const items = sources.map(src => {
      let v = 0;
      for (const r of rows) if (r.source === src) v += r.totalTokens;
      return { name: src, value: v, color: U.getSourceColor(src) };
    }).sort((a, b) => b.value - a.value);
    return items;
  }, [rows]);

  const sum = data.reduce((s, d) => s + d.value, 0);
  const topPct = data[0] && sum ? ((data[0].value / sum) * 100).toFixed(0) : 0;

  return (
    <Card>
      <CardHeader>
        <div className="flex items-center justify-between">
          <CardTitle>来源占比</CardTitle>
          <CardDescription className="text-right max-w-[160px]">点击聚焦 · 顶部贡献 {topPct}%</CardDescription>
        </div>
      </CardHeader>
      <CardContent>
        <div className="flex flex-col items-center gap-3">
          <div className="relative" style={{ width: 220, height: 220 }}>
            <ResponsiveContainer width="100%" height="100%">
              <PieChart>
                <Pie data={data} cx="50%" cy="50%" innerRadius={50} outerRadius={85} dataKey="value" strokeWidth={2}>
                  {data.map(d => (<Cell key={d.name} fill={d.color} opacity={focused && focused !== d.name ? 0.25 : 1} />))}
                </Pie>
              </PieChart>
            </ResponsiveContainer>
            <div className="absolute inset-0 flex flex-col items-center justify-center pointer-events-none">
              <div className="text-[10px] text-muted-foreground uppercase tracking-wider">合计</div>
              <div className="text-xl font-semibold tabular-nums">{U.compactCN(sum)}</div>
            </div>
          </div>
          <div className="w-full space-y-1">
            {data.map(d => (
              <div key={d.name}
                className={`flex items-center gap-2 px-2 py-1 rounded-md cursor-pointer hover:bg-muted/50 ${focused && focused !== d.name ? 'opacity-40' : ''}`}
                onClick={() => onFocusSource(focused === d.name ? null : d.name)}>
                <SourceIcon name={d.name} className="w-4 h-4 shrink-0" />
                <span className="text-xs truncate flex-1">{d.name}</span>
                <span className="text-xs text-muted-foreground tabular-nums">{U.compactCN(d.value)}</span>
                <span className="text-xs font-semibold tabular-nums w-10 text-right">{sum ? ((d.value / sum) * 100).toFixed(1) : 0}%</span>
              </div>
            ))}
          </div>
        </div>
      </CardContent>
    </Card>
  );
}
