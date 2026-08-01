import { compactCN } from '@/lib/formatters.js';
import { getModelIconUrl, getSourceColor } from '@/lib/iconMap.js';
import React, { useMemo } from 'react';
import { Card } from 'antd';

export default function TopModels({ rows, onDrillModel, allDaily = [] }) {
  const list = useMemo(() => {
    const m = new Map();
    for (const r of rows) {
      if (!r.model) continue;
      if (!m.has(r.model)) m.set(r.model, { model: r.model, source: r.source, sources: new Map(), total: 0, cost: 0 });
      const x = m.get(r.model);
      x.total += r.totalTokens || 0; x.cost += r.costUSD || 0;
      x.sources.set(r.source, (x.sources.get(r.source) || 0) + (r.totalTokens || 0));
    }
    // 活跃天数按全量 daily 统计（模型的固有属性，不随筛选范围变化）
    const allDays = new Map();
    for (const r of (allDaily || [])) {
      if (!r.model || !r.usageDate) continue;
      if (!allDays.has(r.model)) allDays.set(r.model, new Set());
      allDays.get(r.model).add(r.usageDate);
    }
    return [...m.values()]
      .map(x => {
        const srcArr = [...x.sources.entries()]
          .map(([source, total]) => ({ source, total }))
          .sort((a, b) => b.total - a.total);
        return { ...x, dayCount: allDays.get(x.model)?.size || 0, source: srcArr[0]?.source || x.source, sources: srcArr };
      })
      .sort((a, b) => b.total - a.total).slice(0, 8);
  }, [rows, allDaily]);

  const max = list[0]?.total || 1;

  return (
    <Card className="flex-1 flex flex-col" styles={{ body: { padding: 16, display: 'flex', flexDirection: 'column', flex: 1, minHeight: 0 } }}>
      <div className="flex items-center justify-between mb-3">
        <div><div className="text-sm font-semibold">Top 模型</div><div className="text-xs text-muted-foreground">按总 Token 排序 · {list.length} 个</div></div>
      </div>
      <div className="flex-1 overflow-y-auto scrollbar-subtle min-h-0">
        <div className="space-y-1.5">
          {list.map(m => (
            <div key={m.model} className="grid grid-cols-[1fr_auto] items-center gap-3 px-1.5 py-1.5 rounded-md cursor-pointer hover:bg-muted/50" onClick={() => onDrillModel?.(m)}>
              <div className="min-w-0">
                <div className="flex items-center gap-1.5">
                  {getModelIconUrl(m.model) && <img src={getModelIconUrl(m.model)} className="w-4 h-4 shrink-0" alt="" />}
                  <span className="text-xs font-medium truncate">{m.model}</span>
                </div>
                <div className="h-1.5 rounded-full bg-muted mt-1.5 overflow-hidden" style={{ width: `${(m.total / max) * 100}%` }}>
                  <div className="h-full flex">
                    {m.sources.slice(0, 4).map(s => (
                      <div key={s.source} className="h-full transition-all" style={{ width: `${(s.total / m.total) * 100}%`, background: getSourceColor(s.source) }} />
                    ))}
                  </div>
                </div>
              </div>
              <div className="text-right shrink-0">
                <div className="text-xs font-semibold tabular-nums">{compactCN(m.total)}</div>
                <div className="text-[10px] text-muted-foreground">{m.cost > 0 ? '$' + m.cost.toFixed(2) : '—'}</div>
              </div>
            </div>
          ))}
        </div>
      </div>
    </Card>
  );
}
