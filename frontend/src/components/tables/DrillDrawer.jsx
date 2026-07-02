import React, { useMemo } from 'react';
import { Dialog, DialogContent, DialogTitle, DialogDescription } from '../ui/dialog.jsx';
import * as U from '../../lib/utils.js';
import SourceIcon from '../SourceIcon.jsx';
import SourceBadge from '../SourceBadge.jsx';

export default function DrillDrawer({ drill, daily, timeRows, onClose }) {
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
      const srcList = row.sources || [{ source: row.source, total: row.total }];
      sub = (
        <div className="flex items-center gap-1.5 flex-wrap">
          {srcList.map(s => (
            <span key={s.source} className="flex items-center gap-1 text-xs">
              <SourceIcon name={s.source} className="w-3 h-3" />
              {s.source}
            </span>
          ))}
        </div>
      );
      filterFn = r => r.model === row.model;
    } else if (kind === 'session') {
      title = row.projectPath || row.sessionId;
      sub = <div className="flex items-center gap-1"><SourceIcon name={row.source} className="w-3 h-3" />{row.source} · {row.device}</div>;
      filterFn = null;
    } else {
      title = <div className="flex items-center gap-1.5"><SourceIcon name={row.source} className="w-4 h-4" />采集: {row.source}</div>;
      sub = U.formatTs(row.collectedAt);
      filterFn = () => false;
    }

    let totals, dates, values, sourceBreakdown, projectSet, modelBreakdown;

    if (kind === 'session' && timeRows) {
      // Use timeRows for project-level data (daily_usage has no project_path)
      const proj = row.projectPath;
      const projRows = timeRows.filter(t => t.projectPath === proj);
      totals = U.aggregateTotals(projRows);
      const byDate = new Map();
      for (const r of projRows) byDate.set(r.usageDate, (byDate.get(r.usageDate) || 0) + (r.totalTokens || 0));
      dates = [...byDate.keys()].sort();
      values = dates.map(d => byDate.get(d));
      const srcMap = new Map();
      for (const r of projRows) srcMap.set(r.source, (srcMap.get(r.source) || 0) + (r.totalTokens || 0));
      sourceBreakdown = [...srcMap.entries()].map(([source, total]) => ({ source, total })).sort((a, b) => b.total - a.total);
      projectSet = new Set([proj]);
      // Model breakdown from same timeRows
      const modelMap = new Map();
      for (const t of projRows) {
        const key = t.model || '未知';
        if (!modelMap.has(key)) modelMap.set(key, { model: key, total: 0, input: 0, output: 0, cache: 0, cost: 0 });
        const m = modelMap.get(key);
        m.total += t.totalTokens || 0;
        m.input += t.inputTokens || 0;
        m.output += t.outputTokens || 0;
        m.cache += (t.cacheCreationTokens || 0) + (t.cacheReadTokens || 0);
        m.cost += t.costUSD || 0;
      }
      modelBreakdown = [...modelMap.values()].sort((a, b) => b.total - a.total);
    } else {
      const matching = daily.filter(filterFn);
      totals = U.aggregateTotals(matching);
      const byDate = new Map();
      for (const r of matching) byDate.set(r.usageDate, (byDate.get(r.usageDate) || 0) + (r.totalTokens || 0));
      dates = [...byDate.keys()].sort();
      values = dates.map(d => byDate.get(d));
      const srcMap = new Map();
      for (const r of matching) srcMap.set(r.source, (srcMap.get(r.source) || 0) + (r.totalTokens || 0));
      sourceBreakdown = [...srcMap.entries()].map(([source, total]) => ({ source, total })).sort((a, b) => b.total - a.total);
      projectSet = new Set();
      for (const r of matching) if (r.projectPath) projectSet.add(r.projectPath);
      modelBreakdown = null;
    }

    const count = kind === 'session' ? (dates.length || 0) : matching.length;
    return { kind, row, title, sub, totals, dates, values, count, sourceBreakdown, projectCount: projectSet.size, modelBreakdown };
  }, [drill, daily, timeRows]);

  const open = !!drill;

  return (
    <Dialog open={open} onOpenChange={o => { if (!o) onClose(); }}>
      {detail && (
        <DialogContent className="sm:max-w-lg max-h-[85vh] overflow-y-auto" showCloseButton>
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
          </div>

          {detail.kind !== 'run' ? (
            <>
              <div className="grid grid-cols-4 gap-2 mb-4">
                <div className="p-2.5 bg-muted rounded-lg">
                  <div className="text-[10px] uppercase text-muted-foreground">总 Token</div>
                  <div className="text-lg font-semibold tabular-nums">{U.compactCN(detail.totals.totalTokens)}</div>
                </div>
                <div className="p-2.5 bg-muted rounded-lg">
                  <div className="text-[10px] uppercase text-muted-foreground">费用</div>
                  <div className="text-lg font-semibold tabular-nums">{detail.totals.costUSD > 0 ? '$' + detail.totals.costUSD.toFixed(2) : '—'}</div>
                </div>
                <div className="p-2.5 bg-muted rounded-lg">
                  <div className="text-[10px] uppercase text-muted-foreground">活跃</div>
                  <div className="text-lg font-semibold tabular-nums">{detail.dates.length}<span className="text-xs text-muted-foreground font-normal"> 天</span></div>
                </div>
                <div className="p-2.5 bg-muted rounded-lg">
                  <div className="text-[10px] uppercase text-muted-foreground">项目</div>
                  <div className="text-lg font-semibold tabular-nums">{detail.projectCount || <span className="text-muted-foreground font-normal">—</span>}</div>
                </div>
              </div>

              {detail.kind === 'session' && detail.modelBreakdown && (
                <div className="mb-4">
                  <h4 className="text-[10px] font-semibold uppercase text-muted-foreground mb-2">模型用量</h4>
                  <div className="space-y-2">
                    {detail.modelBreakdown.map(m => (
                      <div key={m.model} className="p-2.5 bg-muted rounded-lg">
                        <div className="flex items-center justify-between mb-1.5">
                          <span className="text-xs font-mono font-medium">{m.model}</span>
                          <span className="text-xs tabular-nums font-semibold">{U.compactCN(m.total)}</span>
                        </div>
                        <div className="flex items-center gap-2 text-[10px] text-muted-foreground">
                          <span>Input {U.compact(m.input)}</span>
                          <span>Output {U.compact(m.output)}</span>
                          <span>Cache {U.compact(m.cache)}</span>
                          {m.cost > 0 && <span className="text-amber-600">${m.cost.toFixed(2)}</span>}
                        </div>
                      </div>
                    ))}
                  </div>
                </div>
              )}

              {detail.kind === 'model' && detail.sourceBreakdown.length > 1 && (
                <div className="mb-4">
                  <h4 className="text-[10px] font-semibold uppercase text-muted-foreground mb-2">来源分布</h4>
                  <div className="space-y-1.5">
                    {detail.sourceBreakdown.map(s => (
                      <div key={s.source} className="flex items-center gap-2 text-xs">
                        <span className="w-20 shrink-0"><SourceBadge source={s.source} /></span>
                        <div className="flex-1 h-2 rounded-full bg-muted overflow-hidden">
                          <div className="h-full rounded-full transition-all" style={{ width: `${(s.total / detail.totals.totalTokens) * 100}%`, background: U.getSourceColor(s.source) }} />
                        </div>
                        <span className="tabular-nums font-medium w-16 text-right">{U.compactCN(s.total)}</span>
                        <span className="text-muted-foreground w-10 text-right">{(s.total / detail.totals.totalTokens * 100).toFixed(1)}%</span>
                      </div>
                    ))}
                  </div>
                </div>
              )}

              {detail.dates.length > 1 && <SparkLine dates={detail.dates} values={detail.values} />}

              <div className="mt-4">
                <h4 className="text-[10px] font-semibold uppercase text-muted-foreground mb-2">分布</h4>
                <div className="divide-y divide-dashed text-xs">
                  <Row label="Input" value={U.fmt.format(detail.totals.inputTokens)} />
                  <Row label="Output" value={U.fmt.format(detail.totals.outputTokens)} />
                  <Row label="Cache Read" value={U.fmt.format(detail.totals.cacheReadTokens)} />
                  <Row label="Cache Creation" value={U.fmt.format(detail.totals.cacheCreationTokens)} />
                  <Row label="Reasoning" value={U.fmt.format(detail.totals.reasoningTokens)} />
                  <Row label="缓存命中率" value={detail.totals.cacheHitRate.toFixed(2) + '%'} />
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
