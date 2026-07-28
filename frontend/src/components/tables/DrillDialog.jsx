import { useMemo } from 'react';
import { Dialog, DialogContent, DialogTitle, DialogDescription } from '../ui/dialog.jsx';
import * as U from '../../lib/utils.js';
import SourceIcon from '../SourceIcon.jsx';
import SourceBadge from '../SourceBadge.jsx';

export default function DrillDialog({ drill, daily, timeRows, onClose }) {
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

    let totals, dates, values, sourceBreakdown, projectSet, modelBreakdown, matchingCount;

    if (kind === 'session' && timeRows) {
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
      matchingCount = dates.length || 0;
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
      matchingCount = matching.length;
    }

    // Project breakdown for source view
    let projectBreakdown = null;
    if (kind === 'source' && timeRows) {
      const pmap = new Map();
      for (const t of timeRows) {
        if (t.source !== row.source || t.device !== row.device) continue;
        const proj = t.projectPath || '(默认)';
        pmap.set(proj, (pmap.get(proj) || 0) + (t.totalTokens || 0));
      }
      if (pmap.size > 0) {
        projectBreakdown = [...pmap.entries()].map(([project, total]) => ({ project, total })).sort((a, b) => b.total - a.total);
      }
    }
    // Model breakdown for source view
    let sourceModelBreakdown = null;
    if (kind === 'source') {
      const modelMap = new Map();
      for (const r of (timeRows || [])) {
        if (r.source !== row.source || r.device !== row.device) continue;
        const key = r.model || '未知';
        modelMap.set(key, (modelMap.get(key) || 0) + (r.totalTokens || 0));
      }
      // Also check daily for sources not in timeRows
      for (const r of daily) {
        if (r.source !== row.source || r.device !== row.device) continue;
        if (!r.model) continue;
        const key = r.model;
        if (!modelMap.has(key)) modelMap.set(key, r.totalTokens || 0);
      }
      if (modelMap.size > 1) {
        sourceModelBreakdown = [...modelMap.entries()].map(([model, total]) => ({ model, total })).sort((a, b) => b.total - a.total);
      }
    }

    const count = matchingCount;
    return { kind, row, title, sub, totals, dates, values, count, sourceBreakdown, projectCount: projectSet.size, projectBreakdown, sourceModelBreakdown, modelBreakdown };
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
              <div className="text-xs text-muted-foreground mb-0.5">
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
              {/* 来源视图：增量信息 */}
              {detail.kind === 'source' && (
                <div className="space-y-4">
                  <div className="flex items-center gap-3 text-xs text-muted-foreground">
                    <span>活跃 <strong className="text-foreground">{detail.dates.length}</strong> 天</span>
                    <span>项目 <strong className="text-foreground">{detail.projectCount || 0}</strong> 个</span>
                    <span>缓存命中率 <strong className="text-foreground">{detail.totals.cacheHitRate.toFixed(1)}%</strong></span>
                  </div>

                  {detail.projectBreakdown && detail.projectBreakdown.length > 0 && (
                    <div>
                      <h4 className="text-xs font-semibold text-muted-foreground mb-2">项目分布</h4>
                      <div className="space-y-1.5">
                        {detail.projectBreakdown.map(p => (
                          <div key={p.project} className="flex items-center gap-2 text-xs">
                            <span className="font-mono truncate flex-1 min-w-0">{p.project}</span>
                            <div className="flex-1 h-2 rounded-full bg-muted overflow-hidden max-w-[120px]">
                              <div className="h-full rounded-full transition-all" style={{ width: `${(p.total / detail.projectBreakdown[0].total) * 100}%`, background: U.getSourceColor(detail.row.source) }} />
                            </div>
                            <span className="tabular-nums font-medium w-16 text-right shrink-0">{U.compactCN(p.total)}</span>
                          </div>
                        ))}
                      </div>
                    </div>
                  )}

                  {detail.sourceModelBreakdown && detail.sourceModelBreakdown.length > 1 && (
                    <div>
                      <h4 className="text-xs font-semibold text-muted-foreground mb-2">模型分布</h4>
                      <div className="space-y-1.5">
                        {detail.sourceModelBreakdown.map(m => (
                          <div key={m.model} className="flex items-center gap-2 text-xs">
                            <span className="font-mono truncate flex-1 min-w-0">{m.model}</span>
                            <div className="flex-1 h-2 rounded-full bg-muted overflow-hidden max-w-[120px]">
                              <div className="h-full rounded-full transition-all" style={{ width: `${(m.total / detail.sourceModelBreakdown[0].total) * 100}%`, background: U.getSourceColor(detail.row.source) }} />
                            </div>
                            <span className="tabular-nums font-medium w-16 text-right shrink-0">{U.compactCN(m.total)}</span>
                          </div>
                        ))}
                      </div>
                    </div>
                  )}
                </div>
              )}

              {/* 模型视图：来源分布（独家） */}
              {detail.kind === 'model' && (
                <div className="space-y-4">
                  <div className="flex items-center gap-3 text-xs text-muted-foreground">
                    <span>活跃 <strong className="text-foreground">{detail.dates.length}</strong> 天</span>
                    <span>费用 <strong className="text-foreground">${(detail.totals.costUSD || 0).toFixed(2)}</strong></span>
                  </div>

                  {detail.sourceBreakdown.length > 1 ? (
                    <div>
                      <h4 className="text-xs font-semibold text-muted-foreground mb-2">来源分布</h4>
                      <div className="space-y-1.5">
                        {detail.sourceBreakdown.map(s => (
                          <div key={s.source} className="flex items-center gap-2 text-xs">
                            <span className="w-[100px] shrink-0"><SourceBadge source={s.source} /></span>
                            <div className="flex-1 h-2 rounded-full bg-muted overflow-hidden">
                              <div className="h-full rounded-full transition-all" style={{ width: `${(s.total / detail.totals.totalTokens) * 100}%`, background: U.getSourceColor(s.source) }} />
                            </div>
                            <span className="tabular-nums font-medium w-16 text-right">{U.compactCN(s.total)}</span>
                            <span className="text-muted-foreground w-10 text-right">{(s.total / detail.totals.totalTokens * 100).toFixed(1)}%</span>
                          </div>
                        ))}
                      </div>
                    </div>
                  ) : (
                    <p className="text-xs text-muted-foreground">单一来源：{detail.sourceBreakdown[0]?.source}</p>
                  )}
                </div>
              )}

              {/* 会话视图：模型用量清单 */}
              {detail.kind === 'session' && (
                <div className="space-y-4">
                  <div className="text-xs text-muted-foreground">
                    活跃 <strong className="text-foreground">{detail.dates.length}</strong> 天 · 总 Token <strong className="text-foreground">{U.compactCN(detail.totals.totalTokens)}</strong>
                  </div>

                  {detail.modelBreakdown && (
                    <div>
                      <h4 className="text-xs font-semibold text-muted-foreground mb-2">模型用量</h4>
                      <div className="space-y-2">
                        {detail.modelBreakdown.map(m => (
                          <div key={m.model} className="p-2.5 bg-muted rounded-lg">
                            <div className="flex items-center justify-between mb-1.5">
                              <span className="text-xs font-mono font-medium">{m.model}</span>
                              <span className="text-xs tabular-nums font-semibold">{U.compactCN(m.total)}</span>
                            </div>
                            <div className="flex items-center gap-2 text-xs text-muted-foreground">
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
                </div>
              )}

              {/* 采集 run 视图 */}
              {detail.kind === 'run' && (
                <div className="p-3 bg-muted rounded-lg text-xs">{detail.row.message || '—'}</div>
              )}
            </>
          ) : (
            <div className="p-3 bg-muted rounded-lg text-xs">{detail.row.message || '—'}</div>
          )}
        </DialogContent>
      )}
    </Dialog>
  );
}
