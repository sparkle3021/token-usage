import { ranges, rangeDays } from '../store/filterStore.jsx';
import { useState, useMemo } from 'react';
import { Card } from '../components/ui/card.jsx';
import { Tabs, TabsList, TabsTrigger } from '../components/ui/tabs.jsx';
import { Dialog, DialogContent, DialogTitle, DialogDescription } from '../components/ui/dialog.jsx';
import TablePanel from '../components/tables/TablePanel.jsx';
import SourceBadge from '../components/SourceBadge.jsx';
import DataTable from '../components/common/DataTable.jsx';
import MultiSelect from '../components/MultiSelect.jsx';
import * as U from '../lib/utils.js';
import SourceDrillDialog from './table/SourceDrillDialog.jsx';

export default function TablePage({ M, onRefresh }) {
  const defaults = { rangeId: '30d', startDate: U.daysAgo(29), endDate: U.daysAgo(0), sources: new Set(), devices: new Set(), models: new Set() };
  const [f, setF] = useState(defaults);
  const [drill, setDrill] = useState(null);

  const allSources = useMemo(() => [...new Set(M.daily.map(r => r.source))].sort(), [M.daily]);
  const allModels = useMemo(() => [...new Set(M.daily.map(r => r.model))].filter(Boolean).sort(), [M.daily]);

  const filtered = useMemo(() => U.filterDaily(M.daily, f), [f, M.daily]);

  const setRange = (rangeId) => {
    if (rangeId === 'all') {
      const sorted = M.daily.map(x => x.usageDate).filter(Boolean).sort();
      setF({ ...f, rangeId, startDate: sorted[0] || U.daysAgo(0), endDate: sorted[sorted.length - 1] || U.daysAgo(0) });
    } else {
      const days = rangeDays[rangeId] || 30;
      setF({ ...f, rangeId, startDate: U.daysAgo(days - 1), endDate: U.daysAgo(0) });
    }
    if (onRefresh) onRefresh();
  };

  const closeDrill = () => setDrill(null);

  return (
    <div className="flex flex-col min-h-0 flex-1 space-y-4">
      {/* Filter Bar */}
      <Card className="p-3 shrink-0">
        <div className="flex flex-wrap items-center gap-2">
          <span className="text-[10px] uppercase tracking-wider text-muted-foreground font-medium mr-1">时间</span>
          <Tabs value={f.rangeId} onValueChange={setRange}>
            <TabsList>{ranges.map(r => <TabsTrigger key={r.id} value={r.id} className="text-xs px-2.5">{r.label}</TabsTrigger>)}</TabsList>
          </Tabs>
        </div>
        <div className="flex flex-wrap items-center gap-1.5 mt-2">
          <span className="text-[10px] uppercase tracking-wider text-muted-foreground font-medium mr-1">来源</span>
          {allSources.map(s => (
            <SourceBadge key={s} source={s} selected={f.sources.has(s)} onClick={() => { const n = new Set(f.sources); n.has(s) ? n.delete(s) : n.add(s); setF({ ...f, sources: n }); }} />
          ))}
        </div>
        <div className="flex items-center gap-2 mt-2 pt-2 border-t">
          <MultiSelect items={allModels} selected={f.models} onChange={v => setF({ ...f, models: v })} placeholder="全部模型" />
        </div>
      </Card>

      {/* Table */}
      <div className="flex flex-col flex-1 min-h-0">
        <TablePanel daily={filtered} sessions={M.sessions} onDrill={setDrill} fullHeight />
      </div>

      {/* 来源详情弹窗 */}
      {drill?.kind === 'source' && (
        <SourceDrillDialog drill={drill} daily={filtered} sessions={M.sessions} onClose={closeDrill} />
      )}

      {/* 模型详情弹窗（内联） */}
      {drill?.kind === 'model' && (
        <Dialog open onOpenChange={o => { if (!o) closeDrill(); }}>
          <DialogContent className="sm:max-w-lg max-h-[85vh] overflow-y-auto" showCloseButton>
            <DialogTitle className="sr-only">{drill.row.model} 详情</DialogTitle>
            <DialogDescription className="sr-only">模型用量详情</DialogDescription>

            <div className="mb-4">
              <div className="text-xs text-muted-foreground mb-0.5">模型详情</div>
              <h3 className="text-sm font-semibold font-mono text-[11px]">{drill.row.model}</h3>
            </div>

            <div className="flex items-center gap-4 text-xs text-muted-foreground mb-4 flex-wrap">
              <span>活跃 <strong className="text-foreground">{drill.row.dayCount}</strong> 天</span>
              <span>总 Token <strong className="text-foreground">{U.compactCN(drill.row.total)}</strong></span>
              <span>费用 <strong className="text-foreground">${(drill.row.cost || 0).toFixed(2)}</strong></span>
            </div>

            {drill.row.sources?.length > 0 && (
              <DataTable rows={drill.row.sources.map(s => ({
                ...s,
                pct: (s.total / drill.row.total * 100).toFixed(1),
              }))} cols={[
                { label: '来源', field: 'source', render: v => <SourceBadge source={v} /> },
                { label: 'Token', field: 'total', right: true, render: v => U.compactCN(v) },
                { label: '占比', right: true, render: (_, r) => `${r.pct || '—'}%` },
              ]} />
            )}
          </DialogContent>
        </Dialog>
      )}

      {/* 会话详情弹窗（内联） */}
      {drill?.kind === 'session' && (() => {
        const matching = filtered.filter(r =>
          r.source === drill.row.source && r.device === drill.row.device && r.projectPath === drill.row.projectPath
        );
        const totals = U.aggregateTotals(matching);
        const mm = new Map();
        for (const r of matching) {
          const m = r.model || '未知';
          if (!mm.has(m)) mm.set(m, { model: m, total: 0, input: 0, output: 0, cache: 0, cost: 0 });
          const x = mm.get(m);
          x.total += r.totalTokens || 0; x.input += r.inputTokens || 0;
          x.output += r.outputTokens || 0;
          x.cache += (r.cacheCreationTokens || 0) + (r.cacheReadTokens || 0);
          x.cost += r.costUSD || 0;
        }
        const modelBreakdown = [...mm.values()].sort((a, b) => b.total - a.total);

        return (
          <Dialog open onOpenChange={o => { if (!o) closeDrill(); }}>
            <DialogContent className="sm:max-w-lg max-h-[85vh] overflow-y-auto" showCloseButton>
              <DialogTitle className="sr-only">{drill.row.projectPath || drill.row.sessionId}</DialogTitle>
              <DialogDescription className="sr-only">项目/会话用量详情</DialogDescription>

              <div className="mb-4">
                <div className="text-xs text-muted-foreground mb-0.5">项目详情</div>
                <h3 className="text-sm font-semibold">{drill.row.projectPath || drill.row.sessionId}</h3>
                <p className="text-xs text-muted-foreground">{drill.row.source} · {drill.row.device}</p>
              </div>

              <div className="flex items-center gap-4 text-xs text-muted-foreground mb-4 flex-wrap">
                <span>总 Token <strong className="text-foreground">{U.compactCN(totals.totalTokens)}</strong></span>
                <span>费用 <strong className="text-foreground">${(totals.costUSD || 0).toFixed(2)}</strong></span>
                <span>缓存率 <strong className="text-foreground">{totals.cacheHitRate.toFixed(1)}%</strong></span>
                <span>会话数 <strong className="text-foreground">{drill.row.sessionCount || 1}</strong></span>
              </div>

              {modelBreakdown.length > 0 && (
                <DataTable rows={modelBreakdown} cols={[
                  { label: '模型', field: 'model', mono: true },
                  { label: 'Input', field: 'input', right: true, render: v => U.compact(v) },
                  { label: 'Output', field: 'output', right: true, render: v => U.compact(v) },
                  { label: 'Cache', field: 'cache', right: true, render: v => U.compact(v) },
                  { label: '费用', field: 'cost', right: true, render: v => v > 0 ? `$${v.toFixed(2)}` : '—' },
                  { label: 'Total', field: 'total', right: true, render: v => U.compactCN(v) },
                ]} />
              )}
            </DialogContent>
          </Dialog>
        );
      })()}
    </div>
  );
}
