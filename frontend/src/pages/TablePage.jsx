import { ranges, rangeDays } from '../store/filterStore.jsx';
import { useState, useMemo } from 'react';
import { Card, CardHeader, CardContent } from '../components/ui/card.jsx';
import { Tabs, TabsList, TabsTrigger } from '../components/ui/tabs.jsx';
import { Dialog, DialogContent, DialogTitle, DialogDescription } from '../components/ui/dialog.jsx';
import { Table, TableHeader, TableBody, TableRow, TableHead, TableCell } from '../components/ui/table.jsx';
import SourceBadge from '../components/SourceBadge.jsx';
import SourceIcon from '../components/SourceIcon.jsx';
import { Button } from '../components/ui/button.jsx';
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
      <Card className="p-3 shrink-0 overflow-visible">
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
        <DataTablePanel daily={filtered} sessionAgg={M.sessionAgg} onDrill={setDrill} fullHeight allDaily={M.daily} dateRange={[f.startDate, f.endDate]} />
      </div>

      {/* 来源模型分布弹窗 */}
      {drill?.kind === 'source-model' && (
        <SourceDrillDialog drill={drill} daily={filtered} allDaily={M.daily} onClose={closeDrill} />
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
              <div className="space-y-1.5">
                {drill.row.sources.map(s => {
                  const pct = (s.total / drill.row.total * 100);
                  return (
                    <div key={s.source} className="grid grid-cols-[1fr_auto] items-center gap-3 px-1.5 py-1.5">
                      <div className="min-w-0">
                        <div className="flex items-center gap-1.5">
                          <SourceIcon name={s.source} className="w-3.5 h-3.5 shrink-0" />
                          <span className="text-xs font-medium truncate">{s.source}</span>
                        </div>
                        <div className="h-1.5 rounded-full bg-muted mt-1.5 overflow-hidden">
                          <div className="h-full" style={{ width: `${pct}%`, background: U.getSourceColor(s.source) }} />
                        </div>
                      </div>
                      <div className="text-right shrink-0">
                        <div className="text-xs font-semibold tabular-nums">{U.compactCN(s.total)}</div>
                        <div className="text-[10px] text-muted-foreground tabular-nums">{pct.toFixed(1)}%</div>
                      </div>
                    </div>
                  );
                })}
              </div>
            )}
          </DialogContent>
        </Dialog>
      )}

      {/* 会话详情弹窗（内联） */}
      {drill?.kind === 'session-project' && <SessionProjectDialog drill={drill} onClose={closeDrill} />}
    </div>
  );
}

// 项目会话弹窗：展示该项目（source+device+projectPath）下的会话列表，独立分页。
const PAGE_SIZE = 15;

function SessionProjectDialog({ drill, onClose }) {
  const row = drill.row;
  const sessions = useMemo(() => [...(row?.sessions || [])].sort((a, b) => (b.totalTokens || 0) - (a.totalTokens || 0)), [row?.sessions]);
  const [page, setPage] = useState(1);
  const pageCount = Math.max(1, Math.ceil(sessions.length / PAGE_SIZE));
  const curPage = Math.min(page, pageCount);
  const visible = sessions.slice((curPage - 1) * PAGE_SIZE, curPage * PAGE_SIZE);

  return (
    <Dialog open onOpenChange={o => { if (!o) onClose(); }}>
      <DialogContent className="sm:max-w-lg max-h-[85vh] overflow-y-auto" showCloseButton>
        <DialogTitle className="sr-only">{row.projectPath}</DialogTitle>
        <DialogDescription className="sr-only">项目会话用量详情</DialogDescription>

        <div className="mb-4">
          <div className="text-xs text-muted-foreground mb-0.5">项目详情</div>
          <h3 className="text-sm font-semibold font-mono">{row.projectPath || '—'}</h3>
          <p className="text-xs text-muted-foreground">{row.source} · {row.device}</p>
        </div>

        <div className="flex items-center gap-4 text-xs text-muted-foreground mb-4 flex-wrap">
          <span>总 Token <strong className="text-foreground">{U.compactCN(row.total || 0)}</strong></span>
          <span>费用 <strong className="text-foreground">${(row.cost || 0).toFixed(2)}</strong></span>
          <span>会话数 <strong className="text-foreground">{row.sessionCount || 0}</strong></span>
          <span>活跃 <strong className="text-foreground">{U.formatTs(row.lastTs)}</strong></span>
        </div>

        {sessions.length > 0 && (
          <>
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead className="text-[11px] uppercase tracking-wider text-muted-foreground">会话</TableHead>
                  <TableHead className="text-[11px] uppercase tracking-wider text-muted-foreground">活动</TableHead>
                  <TableHead className="text-[11px] uppercase tracking-wider text-muted-foreground text-right">Total</TableHead>
                  <TableHead className="text-[11px] uppercase tracking-wider text-muted-foreground text-right">费用</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {visible.map(s => (
                  <TableRow key={s.sessionId}>
                    <TableCell className="text-xs font-mono">{String(s.sessionId).split('/').slice(-1)[0]}</TableCell>
                    <TableCell className="text-xs text-muted-foreground">{U.formatTs(s.lastTs)}</TableCell>
                    <TableCell className="text-xs tabular-nums text-right">{U.compactCN(s.totalTokens || 0)}</TableCell>
                    <TableCell className="text-xs tabular-nums text-right">{(s.costUSD || 0) > 0 ? `$${s.costUSD.toFixed(2)}` : '—'}</TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
            {pageCount > 1 && (
              <div className="flex items-center justify-between pt-2">
                <span className="text-[11px] text-muted-foreground">共 {sessions.length} 条 · 第 {curPage}/{pageCount} 页</span>
                <div className="flex gap-1">
                  <Button size="xs" variant="outline" disabled={curPage <= 1} onClick={() => setPage(curPage - 1)}>上一页</Button>
                  <Button size="xs" variant="outline" disabled={curPage >= pageCount} onClick={() => setPage(curPage + 1)}>下一页</Button>
                </div>
              </div>
            )}
          </>
        )}
      </DialogContent>
    </Dialog>
  );
}

// ── 数据明细表格（shadcn Table 内联）──────────────────────────

const TD = {
  head: "text-[11px] uppercase tracking-wider text-muted-foreground whitespace-nowrap",
  cell: "text-xs tabular-nums py-2 px-2 whitespace-nowrap",
};

function RowLink({ onClick, children }) {
  return (
    <button className="text-xs text-primary underline-offset-4 hover:underline" onClick={onClick}>
      {children}
    </button>
  );
}

function DataTablePanel({ daily = [], sessionAgg = [], onDrill, fullHeight = false, allDaily = [], dateRange = null }) {
  const [tab, setTab] = useState('sources');

  // ── 来源聚合（同 TablePanel bySource）──
  const bySource = useMemo(() => {
    const m = new Map();
    for (const r of (daily || [])) {
      if (!r.source) continue;
      const k = `${r.source}::${r.device || ''}`;
      if (!m.has(k)) m.set(k, { source: r.source, device: r.device || '', total: 0, input: 0, output: 0, cache: 0, cost: 0, models: new Set() });
      const x = m.get(k);
      x.total += r.totalTokens || 0; x.input += r.inputTokens || 0; x.output += r.outputTokens || 0; x.cache += r.cacheReadTokens || 0; x.cost += r.costUSD || 0;
      if (r.model) x.models.add(r.model);
    }
    return [...m.values()].map(x => ({ ...x, modelCount: x.models.size })).sort((a, b) => b.total - a.total);
  }, [daily]);

  // ── 模型聚合（同 TablePanel byModel，活跃天全量）──
  const byModel = useMemo(() => {
    const m = new Map();
    for (const r of (daily || [])) {
      if (!r.model) continue;
      if (!m.has(r.model)) m.set(r.model, { model: r.model, total: 0, input: 0, output: 0, cache: 0, cost: 0, sources: new Map() });
      const x = m.get(r.model);
      x.total += r.totalTokens || 0; x.input += r.inputTokens || 0; x.output += r.outputTokens || 0; x.cache += r.cacheReadTokens || 0; x.cost += r.costUSD || 0;
      if (r.source) x.sources.set(r.source, (x.sources.get(r.source) || 0) + (r.totalTokens || 0));
    }
    const allDays = new Map();
    for (const r of (allDaily || [])) {
      if (!r.model || !r.usageDate) continue;
      if (!allDays.has(r.model)) allDays.set(r.model, new Set());
      allDays.get(r.model).add(r.usageDate);
    }
    return [...m.values()].map(x => {
      const srcArr = [...x.sources.entries()].map(([source, total]) => ({ source, total })).sort((a, b) => b.total - a.total);
      return { ...x, sources: srcArr, source: srcArr[0]?.source || '', dayCount: allDays.get(x.model)?.size || 0 };
    }).sort((a, b) => b.total - a.total);
  }, [daily, allDaily]);

  // ── 会话聚合：按 lastTs 过滤后聚合为项目行 ──
  const sessionProjectRows = useMemo(() => {
    const filtered = (sessionAgg || []).filter(r => {
      if (!dateRange || !dateRange[0] || !dateRange[1]) return true;
      const d = r.lastTs?.slice(0, 10);
      return d && d >= dateRange[0] && d <= dateRange[1];
    });
    const m = new Map();
    for (const r of filtered) {
      const key = `${r.source}::${r.device || ''}::${r.projectPath || ''}`;
      if (!m.has(key)) m.set(key, {
        source: r.source, device: r.device || '', projectPath: r.projectPath || '',
        sessionCount: 0, total: 0, input: 0, output: 0, cost: 0, lastTs: '', sessions: [],
      });
      const x = m.get(key);
      x.sessionCount++;
      x.total += r.totalTokens || 0; x.input += r.inputTokens || 0; x.output += r.outputTokens || 0; x.cost += r.costUSD || 0;
      if (r.lastTs > x.lastTs) x.lastTs = r.lastTs;
      x.sessions.push(r);
    }
    return [...m.values()].map(({ sessions, ...rest }) => ({ ...rest, sessions })).sort((a, b) => b.total - a.total);
  }, [sessionAgg, dateRange]);

  const sessionMinDate = useMemo(() => {
    const dates = (sessionAgg || []).map(s => s.lastTs?.slice(0, 10)).filter(Boolean).sort();
    return dates[0] || null;
  }, [sessionAgg]);

  const tabs = [
    { id: 'sources', label: '来源', count: bySource.length },
    { id: 'models', label: '模型', count: byModel.length },
    { id: 'sessions', label: '会话', count: sessionProjectRows.length },
  ];

  return (
    <Card className={fullHeight ? 'flex flex-col flex-1 min-h-0' : ''}>
      <CardHeader>
        <Tabs value={tab} onValueChange={setTab}>
          <TabsList>{tabs.map(t => <TabsTrigger key={t.id} value={t.id} className="text-xs">{t.label} <span className="opacity-55 ml-1">{t.count}</span></TabsTrigger>)}</TabsList>
        </Tabs>
      </CardHeader>
      <CardContent className={`px-0 ${fullHeight ? 'flex flex-col flex-1 min-h-0' : ''}`}>
        {tab === 'sessions' && sessionMinDate && (
          <div className="px-4 pt-1 pb-1 text-[10px] text-muted-foreground shrink-0">数据范围 {sessionMinDate} 起</div>
        )}
        {tab === 'sources' && (
          <SourceTable rows={bySource} onDrill={r => onDrill?.({ kind: 'source-model', row: r })} />
        )}
        {tab === 'models' && (
          <ModelTable rows={byModel} onDrill={r => onDrill?.({ kind: 'model', row: r })} />
        )}
        {tab === 'sessions' && (
          <SessionTable rows={sessionProjectRows} onDrill={r => onDrill?.({ kind: 'session-project', row: r })} />
        )}
      </CardContent>
    </Card>
  );
}

// ── 纯展示表格（shadcn Table，无排序搜索）──

function BasicTable({ head, rows, children, fullHeight, stickyLast = false }) {
  const stickyHead = "sticky right-0 bg-card z-10";
  return (
    <div className={fullHeight ? 'relative flex-1 min-h-0' : ''}>
      <div className={`${fullHeight ? 'absolute inset-0 overflow-auto' : 'max-h-[400px] overflow-auto'}`}>
        <Table>
          <TableHeader>
            <TableRow>{head.map((h, i) => (
              <TableHead key={h} className={`${TD.head} ${stickyLast && i === head.length - 1 ? stickyHead : ''}`}>{h}</TableHead>
            ))}</TableRow>
          </TableHeader>
          <TableBody>
            {rows.length === 0 && <TableRow><TableCell colSpan={head.length + 1} className="text-center py-8 text-muted-foreground">暂无数据</TableCell></TableRow>}
            {rows.map((r, i) => <TableRow key={i}>{children(r)}</TableRow>)}
          </TableBody>
        </Table>
      </div>
    </div>
  );
}

function SourceTable({ rows, onDrill }) {
  const head = ['来源', '设备', '模型', 'Total', 'Input', 'Output', 'Cache', '费用', ''];
  return (
    <BasicTable head={head} rows={rows} fullHeight stickyLast>
      {r => <>
        <TableCell className={TD.cell}><SourceBadge source={r.source || 'unknown'} /></TableCell>
        <TableCell className={`${TD.cell} text-muted-foreground text-[11px]`}>{r.device}</TableCell>
        <TableCell className={`${TD.cell} text-right`}>{r.modelCount}</TableCell>
        <TableCell className={`${TD.cell} text-right font-semibold`}>{U.fmt.format(r.total || 0)}</TableCell>
        <TableCell className={`${TD.cell} text-right`}>{U.compact(r.input)}</TableCell>
        <TableCell className={`${TD.cell} text-right`}>{U.compact(r.output)}</TableCell>
        <TableCell className={`${TD.cell} text-right`}>{U.compact(r.cache)}</TableCell>
        <TableCell className={`${TD.cell} text-right`}>{r.cost > 0 ? <span className="text-amber-600">${r.cost.toFixed(2)}</span> : '—'}</TableCell>
        <TableCell className="sticky right-0 bg-card z-10 py-0 text-right"><RowLink onClick={() => onDrill(r)}>模型分布</RowLink></TableCell>
      </>}
    </BasicTable>
  );
}

function ModelTable({ rows, onDrill }) {
  const head = ['模型', '活跃天', 'Total', 'Input', 'Output', '费用', ''];
  return (
    <BasicTable head={head} rows={rows} fullHeight stickyLast>
      {r => <>
        <TableCell className={TD.cell}>
          <span className="inline-flex items-center gap-1.5 font-mono text-[11px]">
            {U.getModelIconUrl(r.model) && <img src={U.getModelIconUrl(r.model)} className="w-3.5 h-3.5 shrink-0" alt="" />}
            {r.model}
          </span>
        </TableCell>
        <TableCell className={`${TD.cell} text-right`}>{r.dayCount}</TableCell>
        <TableCell className={`${TD.cell} text-right font-semibold`}>{U.fmt.format(r.total || 0)}</TableCell>
        <TableCell className={`${TD.cell} text-right`}>{U.compact(r.input)}</TableCell>
        <TableCell className={`${TD.cell} text-right`}>{U.compact(r.output)}</TableCell>
        <TableCell className={`${TD.cell} text-right`}>{r.cost > 0 ? <span className="text-amber-600">${r.cost.toFixed(2)}</span> : '—'}</TableCell>
        <TableCell className="sticky right-0 bg-card z-10 py-0 text-right"><RowLink onClick={() => onDrill(r)}>详情</RowLink></TableCell>
      </>}
    </BasicTable>
  );
}

function SessionTable({ rows, onDrill }) {
  const head = ['来源', '项目', '会话', '活动', 'Input', 'Output', 'Total', '费用', ''];
  return (
    <BasicTable head={head} rows={rows} fullHeight stickyLast>
      {r => <>
        <TableCell className={TD.cell}><SourceBadge source={r.source || 'unknown'} /></TableCell>
        <TableCell className={`${TD.cell} font-mono text-[11px]`} title={r.projectPath}>{r.projectPath || '—'}</TableCell>
        <TableCell className={`${TD.cell} text-right text-muted-foreground text-[11px]`}>{r.sessionCount || 0}</TableCell>
        <TableCell className={`${TD.cell} text-muted-foreground text-[11px]`}>{U.formatTs(r.lastTs)}</TableCell>
        <TableCell className={`${TD.cell} text-right`}>{U.compact(r.input)}</TableCell>
        <TableCell className={`${TD.cell} text-right`}>{U.compact(r.output)}</TableCell>
        <TableCell className={`${TD.cell} text-right font-semibold`}>{U.fmt.format(r.total || 0)}</TableCell>
        <TableCell className={`${TD.cell} text-right`}>{r.cost > 0 ? <span className="text-amber-600">${r.cost.toFixed(2)}</span> : '—'}</TableCell>
        <TableCell className="sticky right-0 bg-card z-10 py-0 text-right"><RowLink onClick={() => onDrill(r)}>详情</RowLink></TableCell>
      </>}
    </BasicTable>
  );
}
