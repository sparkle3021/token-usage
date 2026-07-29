import { useMemo, useState } from 'react';
import { Dialog, DialogContent, DialogTitle, DialogDescription } from '../../components/ui/dialog.jsx';
import * as U from '../../lib/utils.js';
import SourceIcon from '../../components/SourceIcon.jsx';
import DataTable from '../../components/common/DataTable.jsx';

/**
 * Table 页面来源行点击 → 专用弹窗。
 * 展示该来源的项目分布（来自 sessions）和模型分布（来自 daily）。
 */
export default function SourceDrillDialog({ drill, daily, sessions, onClose }) {
  const [tab, setTab] = useState('project');

  const detail = useMemo(() => {
    const { row } = drill;
    const filterFn = r => r.source === row.source && r.device === row.device;
    const matching = daily.filter(filterFn);
    const totals = U.aggregateTotals(matching);

    // 项目分布（来自 sessions）
    const pmap = new Map();
    for (const r of sessions || []) {
      if (r.source !== row.source || r.device !== row.device) continue;
      const proj = r.projectPath || '(默认)';
      pmap.set(proj, (pmap.get(proj) || 0) + (r.totalTokens || 0));
    }
    let projectBreakdown = null;
    if (pmap.size > 0) {
      projectBreakdown = [...pmap.entries()].map(([project, total]) => ({ project, total })).sort((a, b) => b.total - a.total);
      const sum = projectBreakdown.reduce((s, x) => s + x.total, 0);
      for (const x of projectBreakdown) x.pct = (x.total / sum * 100).toFixed(1);
    }

    // 模型分布（来自 daily）
    const modelMap = new Map();
    for (const r of matching) {
      if (!r.model) continue;
      modelMap.set(r.model, (modelMap.get(r.model) || 0) + (r.totalTokens || 0));
    }
    let modelBreakdown = [...modelMap.entries()].map(([model, total]) => ({ model, total })).sort((a, b) => b.total - a.total);
    if (modelBreakdown.length > 1) {
      const sum = modelBreakdown.reduce((s, x) => s + x.total, 0);
      for (const x of modelBreakdown) x.pct = (x.total / sum * 100).toFixed(1);
    }

    const dates = [...new Set(matching.map(r => r.usageDate))].sort();

    return {
      title: <div className="flex items-center gap-1.5"><SourceIcon name={row.source} className="w-4 h-4" />{row.source}</div>,
      sub: row.device,
      dates,
      totals,
      projectBreakdown,
      modelBreakdown,
    };
  }, [drill, daily, sessions]);

  return (
    <Dialog open onOpenChange={o => { if (!o) onClose(); }}>
      <DialogContent className="sm:max-w-lg max-h-[85vh] overflow-y-auto" showCloseButton>
        <DialogTitle className="sr-only">{drill.row.source}</DialogTitle>
        <DialogDescription className="sr-only">{drill.row.device}</DialogDescription>

        <div className="flex items-center justify-between mb-4">
          <div>
            <div className="text-xs text-muted-foreground mb-0.5">来源详情</div>
            <h3 className="text-sm font-semibold">{detail.title}</h3>
            <p className="text-xs text-muted-foreground">{detail.sub}</p>
          </div>
        </div>

        <div className="flex items-center gap-4 text-xs text-muted-foreground mb-4 flex-wrap">
          <span>活跃 <strong className="text-foreground">{detail.dates.length}</strong> 天</span>
          <span>总 Token <strong className="text-foreground">{U.compactCN(detail.totals.totalTokens)}</strong></span>
          <span>费用 <strong className="text-foreground">${(detail.totals.costUSD || 0).toFixed(2)}</strong></span>
          <span>缓存率 <strong className="text-foreground">{detail.totals.cacheHitRate.toFixed(1)}%</strong></span>
        </div>

        <div className="space-y-3">
          <div className="flex items-center gap-1 bg-muted rounded-lg p-0.5 w-fit">
            <button className={`px-2.5 py-1 text-xs rounded-md font-medium transition-colors ${tab === 'project' ? 'bg-background shadow-sm text-foreground' : 'text-muted-foreground hover:text-foreground'}`} onClick={() => setTab('project')}>项目分布</button>
            {detail.modelBreakdown?.length > 1 && (
              <button className={`px-2.5 py-1 text-xs rounded-md font-medium transition-colors ${tab === 'model' ? 'bg-background shadow-sm text-foreground' : 'text-muted-foreground hover:text-foreground'}`} onClick={() => setTab('model')}>模型分布</button>
            )}
          </div>
          {tab === 'project' && detail.projectBreakdown?.length > 0 && (
            <DataTable rows={detail.projectBreakdown} cols={[
              { label: '项目', field: 'project' },
              { label: 'Token', field: 'total', right: true, render: v => U.compactCN(v) },
              { label: '占比', right: true, render: (_, r) => `${r.pct || '—'}%` },
            ]} />
          )}
          {tab === 'model' && detail.modelBreakdown?.length > 1 && (
            <DataTable rows={detail.modelBreakdown} cols={[
              { label: '模型', field: 'model', mono: true },
              { label: 'Token', field: 'total', right: true, render: v => U.compactCN(v) },
              { label: '占比', right: true, render: (_, r) => `${r.pct || '—'}%` },
            ]} />
          )}
        </div>
      </DialogContent>
    </Dialog>
  );
}
