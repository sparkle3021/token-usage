import React, { useMemo } from 'react';
import { Dialog, DialogContent, DialogTitle, DialogDescription } from '../ui/dialog.jsx';
import { Button } from '../ui/button.jsx';
import { XIcon } from 'lucide-react';
import * as U from '../../lib/utils.js';
import SourceIcon from '../SourceIcon.jsx';

export default function DrillDrawer({ drill, daily, onClose }) {
  const detail = useMemo(() => {
    if (!drill) return null;
    const { kind, row } = drill;
    let title, sub, filterFn;
    if (kind === 'source') {
      title = <div className="flex items-center gap-1.5"><SourceIcon name={row.source} className="w-4 h-4" />{row.source}</div>;
      sub = row.device;
      filterFn = r => r.source === row.source && r.device === row.device;
    } else if (kind === 'model') {
      title = row.model;
      sub = <div className="flex items-center gap-1"><SourceIcon name={row.source} className="w-3 h-3" />{row.source}</div>;
      filterFn = r => r.source === row.source && r.model === row.model;
    } else if (kind === 'session') {
      title = row.projectPath || row.sessionId;
      sub = <div className="flex items-center gap-1"><SourceIcon name={row.source} className="w-3 h-3" />{row.source} · {row.device}</div>;
      filterFn = r => r.source === row.source;
    } else {
      title = <div className="flex items-center gap-1.5"><SourceIcon name={row.source} className="w-4 h-4" />采集: {row.source}</div>;
      sub = U.formatTs(row.collectedAt);
      filterFn = () => false;
    }

    const matching = daily.filter(filterFn);
    const totals = U.aggregateTotals(matching);
    const byDate = new Map();
    for (const r of matching) byDate.set(r.usageDate, (byDate.get(r.usageDate) || 0) + (r.totalTokens || 0));
    const dates = [...byDate.keys()].sort();
    const values = dates.map(d => byDate.get(d));
    return { kind, row, title, sub, totals, dates, values, count: matching.length };
  }, [drill, daily]);

  const open = !!drill;

  return (
    <Dialog open={open} onOpenChange={o => { if (!o) onClose(); }}>
      {detail && (
        <DialogContent className="fixed top-0 right-0 bottom-0 left-auto w-[500px] max-w-[92vw] translate-x-0 translate-y-0 rounded-none rounded-l-xl data-open:animate-in data-open:slide-in-from-right data-closed:animate-out data-closed:slide-out-to-right overflow-auto">
          <DialogTitle className="sr-only">{detail.title}</DialogTitle>
          <DialogDescription className="sr-only">{detail.sub}</DialogDescription>

          <div className="flex items-center justify-between mb-4">
            <div>
              <div className="text-[10px] text-muted-foreground uppercase tracking-wider mb-0.5">
                {detail.kind === 'source' && '来源详情'}
                {detail.kind === 'model' && '模型详情'}
                {detail.kind === 'session' && '项目详情'}
                {detail.kind === 'run' && '采集详情'}
              </div>
              <h3 className="text-sm font-semibold">{detail.title}</h3>
              <p className="text-xs text-muted-foreground">{detail.sub}</p>
            </div>
            <Button size="icon-sm" variant="ghost" onClick={onClose}><XIcon className="size-4" /></Button>
          </div>

          {detail.kind !== 'run' ? (
            <>
              <div className="grid grid-cols-3 gap-2 mb-4">
                <div className="p-2.5 bg-muted rounded-lg">
                  <div className="text-[10px] uppercase text-muted-foreground">Total</div>
                  <div className="text-lg font-semibold tabular-nums">{U.compactCN(detail.totals.totalTokens)}</div>
                </div>
                <div className="p-2.5 bg-muted rounded-lg">
                  <div className="text-[10px] uppercase text-muted-foreground">费用</div>
                  <div className="text-lg font-semibold tabular-nums">{detail.totals.costUSD > 0 ? '$' + detail.totals.costUSD.toFixed(2) : '—'}</div>
                </div>
                <div className="p-2.5 bg-muted rounded-lg">
                  <div className="text-[10px] uppercase text-muted-foreground">活跃</div>
                  <div className="text-lg font-semibold tabular-nums">{detail.dates.length} 天</div>
                </div>
              </div>

              {detail.dates.length > 1 && <SparkLine dates={detail.dates} values={detail.values} />}

              <div className="mt-4">
                <h4 className="text-[10px] font-semibold uppercase text-muted-foreground mb-2">分布</h4>
                <div className="divide-y divide-dashed text-xs">
                  <Row label="Input" value={U.fmt.format(detail.totals.inputTokens)} />
                  <Row label="Output" value={U.fmt.format(detail.totals.outputTokens)} />
                  <Row label="Cache Read" value={U.fmt.format(detail.totals.cacheReadTokens)} />
                  <Row label="Cache Creation" value={U.fmt.format(detail.totals.cacheCreationTokens)} />
                  <Row label="Reasoning" value={U.fmt.format(detail.totals.reasoningTokens)} />
                  <Row label="缓存命中率" value={detail.totals.cacheHitRate.toFixed(1) + '%'} />
                </div>
              </div>
            </>
          ) : (
            <div className="p-3 bg-muted rounded-lg text-xs">{detail.row.message || '—'}</div>
          )}
        </DialogContent>
      )}
    </Dialog>
  );
}

function Row({ label, value }) {
  return (
    <div className="flex justify-between items-center py-2">
      <span className="text-xs text-muted-foreground">{label}</span>
      <span className="tabular-nums text-xs font-medium">{value}</span>
    </div>
  );
}

function SparkLine({ dates, values }) {
  const w = 480, h = 100;
  const max = Math.max(...values, 1);
  const pts = values.map((v, i) => [16 + (i / Math.max(1, values.length - 1)) * (w - 32), h - 16 - (v / max) * (h - 32)]);
  const d = pts.map((p, i) => `${i === 0 ? 'M' : 'L'}${p[0]},${p[1]}`).join(' ');
  return (
    <svg viewBox={`0 0 ${w} ${h}`} className="w-full block" style={{ height: 100 }}>
      <defs><linearGradient id="sg" x1="0" y1="0" x2="0" y2="1"><stop offset="0%" stopColor="oklch(0.55 0.16 265)" stopOpacity="0.25"/><stop offset="100%" stopColor="oklch(0.55 0.16 265)" stopOpacity="0"/></linearGradient></defs>
      <path d={d + ` L${w-16},${h-16} L16,${h-16} Z`} fill="url(#sg)"/>
      <path d={d} fill="none" stroke="oklch(0.55 0.16 265)" strokeWidth="2"/>
      <text x={16} y={h - 2} fontSize="9" fill="oklch(0.62 0.005 80)">{dates[0]}</text>
      <text x={w - 16} y={h - 2} textAnchor="end" fontSize="9" fill="oklch(0.62 0.005 80)">{dates[dates.length - 1]}</text>
    </svg>
  );
}
