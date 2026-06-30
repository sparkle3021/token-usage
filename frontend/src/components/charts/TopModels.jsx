import React, { useMemo } from 'react';
import { Card, CardHeader, CardTitle, CardDescription, CardContent } from '../ui/card.jsx';
import * as U from '../../lib/utils.js';
import SourceBadge from '../SourceBadge.jsx';

export default function TopModels({ rows, onDrillModel }) {
  const list = useMemo(() => {
    const m = new Map();
    for (const r of rows) {
      if (!r.model) continue;
      if (!m.has(r.model)) m.set(r.model, { model: r.model, source: r.source, sources: new Map(), total: 0, cost: 0, count: 0 });
      const x = m.get(r.model);
      x.total += r.totalTokens || 0; x.cost += r.costUSD || 0; x.count += 1;
      x.sources.set(r.source, (x.sources.get(r.source) || 0) + (r.totalTokens || 0));
    }
    return [...m.values()]
      .map(x => {
        const srcArr = [...x.sources.entries()]
          .map(([source, total]) => ({ source, total }))
          .sort((a, b) => b.total - a.total);
        return { ...x, source: srcArr[0]?.source || x.source, sources: srcArr };
      })
      .sort((a, b) => b.total - a.total).slice(0, 8);
  }, [rows]);

  const max = list[0]?.total || 1;

  return (
    <Card className="flex-1 flex flex-col">
      <CardHeader>
        <div className="flex items-center justify-between">
          <div><CardTitle>Top 模型</CardTitle><CardDescription>按总 Token 排序 · {list.length} 个</CardDescription></div>
        </div>
      </CardHeader>
      <CardContent className="flex-1 overflow-y-auto scrollbar-subtle min-h-0">
        <div className="space-y-1.5">
          {list.map(m => (
            <div key={m.model} className="grid grid-cols-[1fr_auto] items-center gap-3 px-1.5 py-1.5 rounded-md cursor-pointer hover:bg-muted/50" onClick={() => onDrillModel?.(m)}>
              <div className="min-w-0">
                <div className="text-xs font-medium truncate">{m.model}</div>
                <div className="flex items-center gap-2 mt-0.5">
                  <div className="flex items-center gap-1 min-w-0">
                    <SourceBadge source={m.sources[0]?.source || m.source} />
                    {m.sources.length > 1 && <span className="text-[10px] text-muted-foreground shrink-0">+{m.sources.length - 1}</span>}
                  </div>
                  <span className="text-[10px] text-muted-foreground shrink-0">{m.count} 条</span>
                </div>
                <div className="h-1.5 rounded-full bg-muted mt-1.5 overflow-hidden" style={{ width: `${(m.total / max) * 100}%` }}>
                  <div className="h-full flex">
                    {m.sources.slice(0, 4).map(s => (
                      <div key={s.source} className="h-full transition-all" style={{ width: `${(s.total / m.total) * 100}%`, background: U.getSourceColor(s.source) }} />
                    ))}
                  </div>
                </div>
              </div>
              <div className="text-right shrink-0">
                <div className="text-xs font-semibold tabular-nums">{U.compactCN(m.total)}</div>
                <div className="text-[10px] text-muted-foreground">{m.cost > 0 ? '$' + m.cost.toFixed(2) : '—'}</div>
              </div>
            </div>
          ))}
        </div>
      </CardContent>
    </Card>
  );
}
