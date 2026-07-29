import { useMemo, useState } from 'react';
import { Dialog, DialogContent, DialogTitle, DialogDescription } from '../ui/dialog.jsx';
import * as U from '../../lib/utils.js';
import SourceIcon from '../SourceIcon.jsx';
import SourceBadge from '../SourceBadge.jsx';
import { Table, TableHeader, TableBody, TableRow, TableHead, TableCell } from '../ui/table.jsx';

export default function DrillDialog({ drill, daily, sessions, onClose }) {
  const [sourceTab, setSourceTab] = useState('project');
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
    } else {
      title = row.projectPath || row.sessionId;
      sub = <div className="flex items-center gap-1"><SourceIcon name={row.source} className="w-3 h-3" />{row.source} · {row.device}</div>;
      filterFn = r => r.source === row.source && r.device === row.device && r.projectPath === row.projectPath;
    }

    const matching = daily.filter(filterFn);
    const totals = U.aggregateTotals(matching);

    const byDate = new Map();
    for (const r of matching) byDate.set(r.usageDate, (byDate.get(r.usageDate) || 0) + (r.totalTokens || 0));
    const dates = [...byDate.keys()].sort();
    const values = dates.map(d => byDate.get(d));

    // 来源视图：项目分布（来自 session_usage，含 project_path） + 模型分布（来自 daily）
    let projectBreakdown = null;
    let modelBreakdown = null;
    if (kind === 'source') {
      const pmap = new Map();
      for (const r of sessions || []) {
        if (r.source !== row.source || r.device !== row.device) continue;
        const proj = r.projectPath || '(默认)';
        pmap.set(proj, (pmap.get(proj) || 0) + (r.totalTokens || 0));
      }
      if (pmap.size > 0) {
        projectBreakdown = [...pmap.entries()].map(([project, total]) => ({ project, total })).sort((a, b) => b.total - a.total);
        const sum = projectBreakdown.reduce((s, x) => s + x.total, 0);
        for (const x of projectBreakdown) x.pct = (x.total / sum * 100).toFixed(1);
      }
      // 来源视图：模型分布（来自 daily，与表格显示一致）
      const modelMap = new Map();
      for (const r of matching) {
        if (!r.model) continue;
        modelMap.set(r.model, (modelMap.get(r.model) || 0) + (r.totalTokens || 0));
      }
      modelBreakdown = [...modelMap.entries()].map(([model, total]) => ({ model, total })).sort((a, b) => b.total - a.total);
      if (modelBreakdown?.length > 1) {
        const sum = modelBreakdown.reduce((s, x) => s + x.total, 0);
        for (const x of modelBreakdown) x.pct = (x.total / sum * 100).toFixed(1);
      }
    }

    // 模型视图：来源分布
    let sourceBreakdown = null;
    if (kind === 'model') {
      const srcMap = new Map();
      for (const r of matching) {
        srcMap.set(r.source, (srcMap.get(r.source) || 0) + (r.totalTokens || 0));
      }
      sourceBreakdown = [...srcMap.entries()].map(([source, total]) => ({ source, total })).sort((a, b) => b.total - a.total);
      if (sourceBreakdown.length > 1) {
        const sum = sourceBreakdown.reduce((s, x) => s + x.total, 0);
        for (const x of sourceBreakdown) x.pct = (x.total / sum * 100).toFixed(1);
      }
    }

    // 会话视图：项目内的模型用量
    let sessionModelBreakdown = null;
    if (kind === 'session') {
      const mm = new Map();
      for (const r of matching) {
        const m = r.model || '未知';
        if (!mm.has(m)) mm.set(m, { model: m, total: 0, input: 0, output: 0, cache: 0, cost: 0 });
        const x = mm.get(m);
        x.total += r.totalTokens || 0;
        x.input += r.inputTokens || 0;
        x.output += r.outputTokens || 0;
        x.cache += (r.cacheCreationTokens || 0) + (r.cacheReadTokens || 0);
        x.cost += r.costUSD || 0;
      }
      sessionModelBreakdown = [...mm.values()].sort((a, b) => b.total - a.total);
    }

    return { kind, row, title, sub, totals, dates, values, projectBreakdown, modelBreakdown, sourceBreakdown, sessionModelBreakdown };
  }, [drill, daily, sessions]);

  if (!detail) return null;

  return (
    <Dialog open={!!drill} onOpenChange={o => { if (!o) onClose(); }}>
      <DialogContent className="sm:max-w-lg max-h-[85vh] overflow-y-auto" showCloseButton>
        <DialogTitle className="sr-only">{detail.title}</DialogTitle>
        <DialogDescription className="sr-only">{detail.sub}</DialogDescription>

        <div className="flex items-center justify-between mb-4">
          <div>
            <div className="text-xs text-muted-foreground mb-0.5">
              {detail.kind === 'source' && '来源详情'}
              {detail.kind === 'model' && '模型详情'}
              {detail.kind === 'session' && '项目详情'}
            </div>
            <h3 className="text-sm font-semibold">{detail.title}</h3>
            <p className="text-xs text-muted-foreground">{detail.sub}</p>
          </div>
        </div>

        {/* 统计行 */}
        <div className="flex items-center gap-4 text-xs text-muted-foreground mb-4 flex-wrap">
          <span>活跃 <strong className="text-foreground">{detail.dates.length}</strong> 天</span>
          <span>总 Token <strong className="text-foreground">{U.compactCN(detail.totals.totalTokens)}</strong></span>
          <span>费用 <strong className="text-foreground">${(detail.totals.costUSD || 0).toFixed(2)}</strong></span>
          <span>缓存率 <strong className="text-foreground">{detail.totals.cacheHitRate.toFixed(1)}%</strong></span>
        </div>

        {/* 来源 → 项目分布 + 模型分布（按钮切换） */}
        {detail.kind === 'source' && (
          <div className="space-y-3">
            <div className="flex items-center gap-1 bg-muted rounded-lg p-0.5 w-fit">
              <button className={`px-2.5 py-1 text-xs rounded-md font-medium transition-colors ${sourceTab === 'project' ? 'bg-background shadow-sm text-foreground' : 'text-muted-foreground hover:text-foreground'}`} onClick={() => setSourceTab('project')}>项目分布</button>
              {detail.modelBreakdown?.length > 1 && (
                <button className={`px-2.5 py-1 text-xs rounded-md font-medium transition-colors ${sourceTab === 'model' ? 'bg-background shadow-sm text-foreground' : 'text-muted-foreground hover:text-foreground'}`} onClick={() => setSourceTab('model')}>模型分布</button>
              )}
            </div>
            {sourceTab === 'project' && detail.projectBreakdown?.length > 0 && (
              <DataTable rows={detail.projectBreakdown} cols={[
                { label: '项目', field: 'project' },
                { label: 'Token', field: 'total', right: true, render: v => U.compactCN(v) },
                { label: '占比', right: true, render: (_, r) => `${r.pct || '—'}%` },
              ]} />
            )}
            {sourceTab === 'model' && detail.modelBreakdown?.length > 1 && (
              <DataTable rows={detail.modelBreakdown} cols={[
                { label: '模型', field: 'model', mono: true },
                { label: 'Token', field: 'total', right: true, render: v => U.compactCN(v) },
                { label: '占比', right: true, render: (_, r) => `${r.pct || '—'}%` },
              ]} />
            )}
          </div>
        )}

        {/* 模型 → 来源分布 */}
        {detail.kind === 'model' && detail.sourceBreakdown?.length > 0 && (
          <DataTable rows={detail.sourceBreakdown} cols={[
            { label: '来源', field: 'source', render: v => <SourceBadge source={v} /> },
            { label: 'Token', field: 'total', right: true, render: v => U.compactCN(v) },
            { label: '占比', right: true, render: (_, r) => `${r.pct || '—'}%` },
          ]} />
        )}

        {/* 会话 → 模型用量清单 */}
        {detail.kind === 'session' && detail.sessionModelBreakdown?.length > 0 && (
          <DataTable rows={detail.sessionModelBreakdown} cols={[
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
}

/** 迷你数据表组件 */
function DataTable({ rows, cols }) {
  return (
    <Table>
      <TableHeader>
        <TableRow>
          {cols.map(c => (
            <TableHead key={c.label} className={c.right ? 'text-right text-[11px]' : 'text-[11px]'}>
              {c.label}
            </TableHead>
          ))}
        </TableRow>
      </TableHeader>
      <TableBody>
        {rows.map((r, i) => (
          <TableRow key={i}>
            {cols.map(c => (
              <TableCell key={c.label} className={c.right ? 'text-right tabular-nums text-xs' : 'text-xs'}>
                {c.render ? (typeof c.render === 'function' ? c.render(r[c.field], r) : c.render) : (c.mono ? <span className="font-mono">{r[c.field]}</span> : r[c.field])}
              </TableCell>
            ))}
          </TableRow>
        ))}
      </TableBody>
    </Table>
  );
}
