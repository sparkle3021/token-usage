import React, { useState, useMemo } from 'react';
import { Card, CardHeader, CardContent } from '../ui/card.jsx';
import { Table, TableHeader, TableBody, TableRow, TableHead, TableCell } from '../ui/table.jsx';
import { Tabs, TabsList, TabsTrigger } from '../ui/tabs.jsx';
import * as U from '../../lib/utils.js';
import SourceBadge from '../SourceBadge.jsx';

export default function TablePanel({ daily = [], sessions = [], runs = [], onDrill, fullHeight = false }) {
  const [tab, setTab] = useState('sources');
  const [search, setSearch] = useState('');

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
    return [...m.values()].map(x => ({ ...x, modelCount: x.models.size }));
  }, [daily]);

  const byModel = useMemo(() => {
    const m = new Map();
    for (const r of (daily || [])) {
      if (!r.source || !r.model) continue;
      const k = `${r.source}::${r.model}`;
      if (!m.has(k)) m.set(k, { source: r.source, model: r.model, total: 0, input: 0, output: 0, cache: 0, cost: 0, days: new Set() });
      const x = m.get(k);
      x.total += r.totalTokens || 0; x.input += r.inputTokens || 0; x.output += r.outputTokens || 0; x.cache += r.cacheReadTokens || 0; x.cost += r.costUSD || 0;
      if (r.usageDate) x.days.add(r.usageDate);
    }
    return [...m.values()].map(x => ({ ...x, dayCount: x.days.size }));
  }, [daily]);

  const tabs = [
    { id: 'sources', label: '来源', count: bySource.length },
    { id: 'models', label: '模型', count: byModel.length },
    { id: 'sessions', label: '会话', count: (sessions || []).length },
    { id: 'runs', label: '采集', count: (runs || []).length },
  ];

  return (
    <Card className={fullHeight ? 'flex flex-col flex-1 min-h-0' : ''}>
      <CardHeader>
        <div className="flex items-center gap-3">
          <Tabs value={tab} onValueChange={v => { setTab(v); setSearch(''); }}>
            <TabsList>{tabs.map(t => <TabsTrigger key={t.id} value={t.id} className="text-xs">{t.label} <span className="opacity-55 ml-1">{t.count}</span></TabsTrigger>)}</TabsList>
          </Tabs>
          <div className="flex-1" />
          <input className="h-7 px-2.5 text-xs rounded-lg border bg-background w-36 outline-none focus:border-ring" placeholder="搜索..." value={search} onChange={e => setSearch(e.target.value)} />
        </div>
      </CardHeader>
      <CardContent className={`px-0 ${fullHeight ? 'flex-1 min-h-0' : ''}`}>
        {tab === 'sources' && <SourceTable rows={bySource} search={search} onDrill={onDrill} fullHeight={fullHeight} />}
        {tab === 'models' && <ModelTable rows={byModel} search={search} onDrill={onDrill} fullHeight={fullHeight} />}
        {tab === 'sessions' && <SessionTable rows={sessions || []} search={search} onDrill={onDrill} fullHeight={fullHeight} />}
        {tab === 'runs' && <RunTable rows={runs || []} search={search} onDrill={onDrill} fullHeight={fullHeight} />}
      </CardContent>
    </Card>
  );
}

// ── Data table wrapper ────────────────────────────────────────

function DTable({ rows, columns, sortField = 'total', search, fullHeight, onDrill }) {
  const [sortBy, setSortBy] = useState({ field: sortField, dir: 'desc' });
  const filtered = useMemo(() => {
    if (!search) return rows;
    const q = search.toLowerCase();
    return rows.filter(r => columns.some(c => String(typeof c.val === 'function' ? c.val(r) : r?.[c.field] ?? '').toLowerCase().includes(q)));
  }, [rows, columns, search]);
  const sorted = useMemo(() => [...filtered].sort((a, b) => {
    const va = typeof columns.find(c => c.field === sortBy.field)?.val === 'function'
      ? columns.find(c => c.field === sortBy.field)?.val(a) : a?.[sortBy.field];
    const vb = typeof columns.find(c => c.field === sortBy.field)?.val === 'function'
      ? columns.find(c => c.field === sortBy.field)?.val(b) : b?.[sortBy.field];
    return typeof va === 'number' ? (sortBy.dir === 'asc' ? va - vb : vb - va) : 0;
  }), [filtered, sortBy, columns]);

  return (
    <div className={`overflow-x-auto overflow-y-auto ${fullHeight ? 'flex-1 min-h-0' : 'max-h-[400px]'}`}>
      <Table>
        <TableHeader>
          <TableRow>{columns.map(c => (
            <TableHead key={c.field || c.label} className={`text-[11px] uppercase tracking-wider cursor-pointer ${sortBy.field === c.field ? 'text-foreground' : 'text-muted-foreground'}`}
              style={{ textAlign: c.right ? 'right' : 'left' }}
              onClick={() => setSortBy(p => p.field === c.field ? { field: c.field, dir: p.dir === 'asc' ? 'desc' : 'asc' } : { field: c.field, dir: 'desc' })}>
              {c.label}
            </TableHead>
          ))}</TableRow>
        </TableHeader>
        <TableBody>
          {sorted.length === 0 && <TableRow><TableCell colSpan={columns.length} className="text-center py-8 text-muted-foreground">暂无数据</TableCell></TableRow>}
          {sorted.map((r, i) => (
            <TableRow key={i} onClick={() => onDrill?.(r)}>
              {columns.map(c => <TableCell key={c.field || c.label} className="text-xs tabular-nums" style={{ textAlign: c.right ? 'right' : 'left' }}>
                {c.render ? c.render(r) : (typeof c.val === 'function' ? c.val(r) : r?.[c.field])}
              </TableCell>)}
            </TableRow>
          ))}
        </TableBody>
      </Table>
    </div>
  );
}

// ── Source Table ───────────────────────────────────────────────

function SourceTable({ rows, search, onDrill, fullHeight }) {
  const cols = [
    { field: 'source', label: '来源', render: r => <SourceBadgeLabel s={r.source} /> },
    { field: 'device', label: '设备', render: r => <span className="text-muted-foreground text-[11px]">{r.device}</span> },
    { field: 'modelCount', label: '模型', right: true },
    { field: 'total', label: 'Total', right: true, render: r => <span className="font-semibold">{U.fmt.format(r.total || 0)}</span> },
    { field: 'input', label: 'Input', right: true, render: r => U.compact(r.input) },
    { field: 'output', label: 'Output', right: true, render: r => U.compact(r.output) },
    { field: 'cache', label: 'Cache', right: true, render: r => U.compact(r.cache) },
    { field: 'cost', label: '费用', right: true, render: r => (r.cost || 0) > 0 ? <span className="text-amber-600">${(r.cost || 0).toFixed(2)}</span> : '—' },
  ];
  return <DTable rows={rows} columns={cols} search={search} sortField="total" fullHeight={fullHeight} onDrill={r => onDrill?.({ kind: 'source', row: r })} />;
}

function ModelTable({ rows, search, onDrill, fullHeight }) {
  const cols = [
    { field: 'source', label: '来源', render: r => <SourceBadgeLabel s={r.source} /> },
    { field: 'model', label: '模型', render: r => <span className="font-mono text-[11px]">{r.model}</span> },
    { field: 'dayCount', label: '活跃天', right: true },
    { field: 'total', label: 'Total', right: true, render: r => <span className="font-semibold">{U.fmt.format(r.total || 0)}</span> },
    { field: 'input', label: 'Input', right: true, render: r => U.compact(r.input) },
    { field: 'output', label: 'Output', right: true, render: r => U.compact(r.output) },
    { field: 'cost', label: '费用', right: true, render: r => (r.cost || 0) > 0 ? <span className="text-amber-600">${(r.cost || 0).toFixed(2)}</span> : '—' },
  ];
  return <DTable rows={rows} columns={cols} search={search} sortField="total" fullHeight={fullHeight} onDrill={r => onDrill?.({ kind: 'model', row: r })} />;
}

function SessionTable({ rows, search, onDrill, fullHeight }) {
  const aggregated = useMemo(() => {
    const m = new Map();
    for (const r of (rows || [])) {
      const key = r.sessionId;
      if (!key) continue;
      if (!m.has(key)) m.set(key, { ...r, modelList: new Set() });
      const agg = m.get(key);
      // Merge token counts
      agg.inputTokens += r.inputTokens || 0;
      agg.outputTokens += r.outputTokens || 0;
      agg.cacheCreationTokens += r.cacheCreationTokens || 0;
      agg.cacheReadTokens += r.cacheReadTokens || 0;
      agg.reasoningOutputTokens += r.reasoningOutputTokens || 0;
      agg.totalTokens += r.totalTokens || 0;
      agg.costUSD += r.costUSD || 0;
      // Track unique models
      if (r.model) agg.modelList.add(r.model);
      // Keep latest lastActivity
      if (r.lastActivity > agg.lastActivity) agg.lastActivity = r.lastActivity;
    }
    return [...m.values()].map(r => ({ ...r, modelCount: r.modelList.size, modelList: undefined }));
  }, [rows]);

  const cols = [
    { field: 'source', label: '来源', width: 100, render: r => <SourceBadgeLabel s={r?.source} /> },
    { field: 'projectPath', label: '项目', render: r => <span className="font-mono text-[11px]" title={r?.sessionId}>{r?.projectPath || (r?.sessionId ? String(r.sessionId).split('/').slice(-1)[0] : '—')}</span> },
    { field: 'lastActivity', label: '活动', render: r => <span className="text-muted-foreground text-[11px]">{r?.lastActivity || '—'}</span> },
    { field: 'inputTokens', label: 'Input', right: true, render: r => U.compact(r?.inputTokens) },
    { field: 'outputTokens', label: 'Output', right: true, render: r => U.compact(r?.outputTokens) },
    { field: 'totalTokens', label: 'Total', right: true, render: r => <span className="font-semibold">{U.fmt.format(r?.totalTokens || 0)}</span> },
    { field: 'costUSD', label: '费用', right: true, render: r => (r?.costUSD || 0) > 0 ? <span className="text-amber-600">${(r.costUSD || 0).toFixed(2)}</span> : '—' },
  ];
  return <DTable rows={aggregated} columns={cols} search={search} sortField="totalTokens" fullHeight={fullHeight} onDrill={r => onDrill?.({ kind: 'session', row: r })} />;
}

function RunTable({ rows, fullHeight }) {
  return (
    <div className={`overflow-x-auto overflow-y-auto ${fullHeight ? 'flex-1 min-h-0' : 'max-h-[400px]'}`}>
      <Table>
        <TableHeader>
          <TableRow>
            <TableHead className="text-[11px] uppercase tracking-wider">时间</TableHead>
            <TableHead className="text-[11px] uppercase tracking-wider">来源</TableHead>
            <TableHead className="text-[11px] uppercase tracking-wider">状态</TableHead>
            <TableHead className="text-[11px] uppercase tracking-wider">说明</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {(rows || []).length === 0 && <TableRow><TableCell colSpan={4} className="text-center py-8 text-muted-foreground">暂无采集记录</TableCell></TableRow>}
          {(rows || []).map((r, i) => (
            <TableRow key={i} onClick={() => onDrill?.({ kind: 'run', row: r })}>
              <TableCell className="text-xs font-mono text-muted-foreground whitespace-nowrap">{U.formatTs(r?.collectedAt)}</TableCell>
              <TableCell className="text-xs"><SourceBadgeLabel s={r?.source} /></TableCell>
              <TableCell className="text-xs"><StatusBadge s={r?.status} /></TableCell>
              <TableCell className="text-xs text-muted-foreground max-w-[300px] truncate" title={r?.message}>{r?.message || ''}</TableCell>
            </TableRow>
          ))}
        </TableBody>
      </Table>
    </div>
  );
}

function SourceBadgeLabel({ s }) {
  return <SourceBadge source={s || 'unknown'} />;
}

function StatusBadge({ s }) {
  const cls = s === 'ok' ? 'bg-green-50 text-green-700' : s === 'error' ? 'bg-red-50 text-red-600' : 'bg-muted text-muted-foreground';
  return <span className={`inline-flex px-1.5 py-0.5 rounded text-[10px] font-medium ${cls}`}>{s || '—'}</span>;
}
