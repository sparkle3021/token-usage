import React, { useMemo } from 'react';
import { Card, CardHeader, CardTitle, CardDescription, CardContent } from '../ui/card.jsx';
import * as U from '../../lib/utils.js';

export default function TopModels({ rows, onDrillModel }) {
  const list = useMemo(() => {
    const m = new Map();
    for (const r of rows) {
      if (!r.model) continue;
      if (!m.has(r.model)) m.set(r.model, { model: r.model, source: r.source, total: 0, cost: 0, count: 0 });
      const x = m.get(r.model);
      x.total += r.totalTokens || 0; x.cost += r.costUSD || 0; x.count += 1;
    }
    return [...m.values()].sort((a, b) => b.total - a.total).slice(0, 8);
  }, [rows]);

  const max = list[0]?.total || 1;

  return (
    <Card>
      <CardHeader>
        <div className="flex items-center justify-between">
          <div><CardTitle>Top 模型</CardTitle><CardDescription>按总 Token 排序 · {list.length} 个</CardDescription></div>
        </div>
      </CardHeader>
      <CardContent>
        <div className="space-y-1.5">
          {list.map(m => (
            <div key={m.model} className="grid grid-cols-[1fr_auto] items-center gap-3 px-1.5 py-1.5 rounded-md cursor-pointer hover:bg-muted/50" onClick={() => onDrillModel?.(m)}>
              <div className="min-w-0">
                <div className="text-xs font-medium truncate">{m.model}</div>
                <div className="flex items-center gap-2 mt-0.5">
                  <Badge variant="outline" className="text-[10px] px-1 py-0 h-4 gap-1" style={{ borderColor: U.getSourceColor(m.source), color: U.getSourceColor(m.source) }}>{m.source}</Badge>
                  <span className="text-[10px] text-muted-foreground">{m.count} 条</span>
                </div>
                <div className="h-1.5 rounded-full bg-muted mt-1.5 overflow-hidden">
                  <div className="h-full rounded-full transition-all" style={{ width: `${(m.total / max) * 100}%`, background: U.getSourceColor(m.source) }} />
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

function Badge({ variant, className, style, children, ...props }) {
  const cls = variant === 'outline' ? 'border bg-transparent text-foreground' : 'bg-primary text-primary-foreground';
  return <span className={`inline-flex items-center px-1.5 py-0.5 rounded text-xs font-medium ${cls} ${className || ''}`} style={style} {...props}>{children}</span>;
}
