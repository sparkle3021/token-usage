import { ranges, rangeDays } from '@/store/filterStore.jsx';
import { useState, useMemo } from 'react';
import { Card, Tabs, Segmented, Modal, Table, Button } from 'antd';
import { compact, compactCN, daysAgo, formatTs, numFmt as fmt } from '@/lib/formatters.js';
import { getModelIconUrl, getSourceColor } from '@/lib/iconMap.js';
import { filterDaily } from '@/lib/filters.js';
import SourceBadge from '@/components/common/SourceBadge.jsx';
import SourceIcon from '@/components/common/SourceIcon.jsx';
import MultiSelect from '@/components/common/MultiSelect.jsx';
import SourceDrillDialog from '@/components/dialogs/SourceDrillDialog.jsx';

export default function TablePage({ M, onRangeSwitch }) {
  const defaults = { rangeId: '30d', startDate: daysAgo(29), endDate: daysAgo(0), sources: new Set(), devices: new Set(), models: new Set() };
  const [f, setF] = useState(defaults);
  const [drill, setDrill] = useState(null);

  const allSources = useMemo(() => [...new Set(M.daily.map(r => r.source))].sort(), [M.daily]);
  const allModels = useMemo(() => [...new Set(M.daily.map(r => r.model))].filter(Boolean).sort(), [M.daily]);

  const filtered = useMemo(() => filterDaily(M.daily, f), [f, M.daily]);

  const setRange = (rangeId) => {
    if (rangeId === 'all') {
      const sorted = M.daily.map(x => x.usageDate).filter(Boolean).sort();
      setF({ ...f, rangeId, startDate: sorted[0] || daysAgo(0), endDate: sorted[sorted.length - 1] || daysAgo(0) });
    } else {
      const days = rangeDays[rangeId] || 30;
      setF({ ...f, rangeId, startDate: daysAgo(days - 1), endDate: daysAgo(0) });
    }
    // 切时间：本地立即渲染，后台异步补拉时间序列（不阻塞切换）
    onRangeSwitch?.(rangeDays[rangeId]);
  };

  const closeDrill = () => setDrill(null);

  return (
    <div className="flex flex-col min-h-0 flex-1 gap-4 h-full">
      {/* Filter Bar */}
      <Card className="p-3 shrink-0 overflow-visible" styles={{ body: { padding: 12 } }}>
        <div className="flex flex-wrap items-center gap-2">
          <span className="text-xs uppercase tracking-wider text-muted-foreground font-medium mr-1">时间</span>
          <Segmented
            value={f.rangeId}
            onChange={setRange}
            size="small"
            options={ranges.map(r => ({ label: r.label, value: r.id }))}
          />
        </div>
        <div className="flex flex-wrap items-center gap-1.5 mt-2">
          <span className="text-xs uppercase tracking-wider text-muted-foreground font-medium mr-1">来源</span>
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
        <ModelDetailModal drill={drill} onClose={closeDrill} />
      )}

      {/* 会话详情弹窗（内联） */}
      {drill?.kind === 'session-project' && <SessionProjectDialog drill={drill} onClose={closeDrill} />}
    </div>
  );
}

// 弹窗 body：flex 容器，标题/指标 shrink-0 固定，内容区 flex-dialog-body 承接滚动
const MODAL_BODY_STYLE = {
  maxHeight: 'calc(85vh - 120px)',
  overflow: 'hidden',
  display: 'flex',
  flexDirection: 'column',
};

function ModelDetailModal({ drill, onClose }) {
  return (
    <Modal
      open
      onCancel={onClose}
      title={null}
      centered
      width={{ xs: 520, md: 576, lg: 672, xl: 720 }}
      styles={{ body: MODAL_BODY_STYLE }}
      footer={null}
    >
      <h3 className="sr-only">{drill.row.model} 详情</h3>
      <p className="sr-only">模型用量详情</p>

      <div className="mb-4 shrink-0">
        <div className="text-xs text-muted-foreground mb-0.5">模型详情</div>
        <h3 className="text-sm font-semibold font-mono text-[11px]">{drill.row.model}</h3>
      </div>

      <div className="flex items-center gap-4 text-xs text-muted-foreground mb-4 shrink-0 flex-wrap">
        <span>活跃 <strong className="text-foreground">{drill.row.dayCount}</strong> 天</span>
        <span>总 Token <strong className="text-foreground">{compactCN(drill.row.total)}</strong></span>
        <span>费用 <strong className="text-foreground">${(drill.row.cost || 0).toFixed(2)}</strong></span>
      </div>

      <div className="flex-dialog-body min-h-0 overflow-y-auto scrollbar-subtle">
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
                      <div className="h-full" style={{ width: `${pct}%`, background: getSourceColor(s.source) }} />
                    </div>
                  </div>
                  <div className="text-right shrink-0">
                    <div className="text-xs font-semibold tabular-nums">{compactCN(s.total)}</div>
                    <div className="text-[10px] text-muted-foreground tabular-nums">{pct.toFixed(1)}%</div>
                  </div>
                </div>
              );
            })}
          </div>
        )}
      </div>
    </Modal>
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

  const columns = [
    {
      title: '会话',
      dataIndex: 'sessionId',
      key: 'sessionId',
      ellipsis: true,
      render: v => <span className="text-xs font-mono">{String(v).split('/').slice(-1)[0]}</span>,
    },
    {
      title: '活动',
      dataIndex: 'lastTs',
      key: 'lastTs',
      width: 132,
      render: v => <span className="text-xs text-muted-foreground">{formatTs(v)}</span>,
    },
    {
      title: 'Total',
      dataIndex: 'totalTokens',
      key: 'totalTokens',
      align: 'right',
      width: 88,
      render: v => <span className="text-xs tabular-nums">{compactCN(v || 0)}</span>,
    },
    {
      title: '费用',
      dataIndex: 'costUSD',
      key: 'costUSD',
      align: 'right',
      width: 84,
      render: v => <span className="text-xs tabular-nums">{(v || 0) > 0 ? `$${v.toFixed(2)}` : '—'}</span>,
    },
  ];

  return (
    <Modal
      open
      onCancel={onClose}
      title={null}
      centered
      width={{ xs: 520, md: 576, lg: 672, xl: 720 }}
      styles={{ body: MODAL_BODY_STYLE }}
      footer={null}
    >
      <h3 className="sr-only">{row.projectPath}</h3>
      <p className="sr-only">项目会话用量详情</p>

      <div className="mb-4 shrink-0">
        <div className="text-xs text-muted-foreground mb-0.5">项目详情</div>
        <h3 className="text-sm font-semibold font-mono">{row.projectPath || '—'}</h3>
        <p className="text-xs text-muted-foreground">{row.source} · {row.device}</p>
      </div>

      <div className="flex items-center gap-4 text-xs text-muted-foreground mb-4 shrink-0 flex-wrap">
        <span>总 Token <strong className="text-foreground">{compactCN(row.total || 0)}</strong></span>
        <span>费用 <strong className="text-foreground">${(row.cost || 0).toFixed(2)}</strong></span>
        <span>会话数 <strong className="text-foreground">{row.sessionCount || 0}</strong></span>
        <span>活跃 <strong className="text-foreground">{formatTs(row.lastTs)}</strong></span>
      </div>

      <div className="flex-dialog-body min-h-0 overflow-y-auto scrollbar-subtle">
        {sessions.length > 0 && (
          <Table
            columns={columns}
            dataSource={visible}
            rowKey="sessionId"
            size="small"
            pagination={false}
            sticky
            locale={{ emptyText: '暂无数据' }}
          />
        )}
      </div>
      {pageCount > 1 && (
        <div className="shrink-0 flex items-center justify-between pt-2">
          <span className="text-[11px] text-muted-foreground">共 {sessions.length} 条 · 第 {curPage}/{pageCount} 页</span>
          <div className="flex gap-1">
            <Button size="small" disabled={curPage <= 1} onClick={() => setPage(curPage - 1)}>上一页</Button>
            <Button size="small" disabled={curPage >= pageCount} onClick={() => setPage(curPage + 1)}>下一页</Button>
          </div>
        </div>
      )}
    </Modal>
  );
}

// ── 数据明细表格（antd Table columns）──────────────────────────

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

  const tabs = [
    { key: 'sources', label: '来源', count: bySource.length },
    { key: 'models', label: '模型', count: byModel.length },
    { key: 'sessions', label: '会话', count: sessionProjectRows.length },
  ];

  return (
    <Card className={fullHeight ? 'flex flex-col flex-1 min-h-0' : ''} styles={{ body: { padding: 0 } }}>
      <div className="px-4 pt-3">
        <Tabs
          activeKey={tab}
          onChange={setTab}
          size="small"
          items={tabs.map(t => ({ key: t.key, label: <span className="text-xs">{t.label} <span className="opacity-55 ml-1">{t.count}</span></span> }))}
        />
      </div>
      <div className={fullHeight ? 'flex flex-col flex-1 min-h-0' : ''}>
        {tab === 'sources' && (
          <SourceTable rows={bySource} onDrill={r => onDrill?.({ kind: 'source-model', row: r })} />
        )}
        {tab === 'models' && (
          <ModelTable rows={byModel} onDrill={r => onDrill?.({ kind: 'model', row: r })} />
        )}
        {tab === 'sessions' && (
          <SessionTable rows={sessionProjectRows} onDrill={r => onDrill?.({ kind: 'session-project', row: r })} />
        )}
      </div>
    </Card>
  );
}

// ── 纯展示表格（antd Table）──

function SourceTable({ rows, onDrill }) {
  const columns = [
    {
      title: '来源',
      key: 'source',
      render: (_, r) => <SourceBadge source={r.source || 'unknown'} />,
    },
    {
      title: '设备',
      dataIndex: 'device',
      key: 'device',
      render: v => <span className={`${TD.cell} text-muted-foreground text-[11px]`}>{v}</span>,
    },
    {
      title: '模型',
      dataIndex: 'modelCount',
      key: 'modelCount',
      align: 'right',
      render: v => <span className={TD.cell}>{v}</span>,
    },
    {
      title: 'Total',
      dataIndex: 'total',
      key: 'total',
      align: 'right',
      render: v => <span className={`${TD.cell} font-semibold`}>{fmt.format(v || 0)}</span>,
    },
    {
      title: 'Input',
      dataIndex: 'input',
      key: 'input',
      align: 'right',
      render: v => <span className={TD.cell}>{compact(v)}</span>,
    },
    {
      title: 'Output',
      dataIndex: 'output',
      key: 'output',
      align: 'right',
      render: v => <span className={TD.cell}>{compact(v)}</span>,
    },
    {
      title: 'Cache',
      dataIndex: 'cache',
      key: 'cache',
      align: 'right',
      render: v => <span className={TD.cell}>{compact(v)}</span>,
    },
    {
      title: '费用',
      dataIndex: 'cost',
      key: 'cost',
      align: 'right',
      render: v => <span className={TD.cell}>{v > 0 ? <span className="text-amber-600">${v.toFixed(2)}</span> : '—'}</span>,
    },
    {
      title: '操作',
      key: 'action',
      fixed: 'end',
      align: 'right',
      render: (_, r) => <RowLink onClick={() => onDrill(r)}>模型分布</RowLink>,
    },
  ];
  return <Table columns={columns} dataSource={rows} rowKey={(r) => `${r.source}::${r.device}`} size="small" pagination={{ pageSize: 10 }} locale={{ emptyText: '暂无数据' }} scroll={{ x: 'max-content' }} />;
}

function ModelTable({ rows, onDrill }) {
  const columns = [
    {
      title: '模型',
      key: 'model',
      render: (_, r) => (
        <span className="inline-flex items-center gap-1.5 font-mono text-[11px]">
          {getModelIconUrl(r.model) && <img src={getModelIconUrl(r.model)} className="w-3.5 h-3.5 shrink-0" alt="" />}
          {r.model}
        </span>
      ),
    },
    {
      title: '活跃天',
      dataIndex: 'dayCount',
      key: 'dayCount',
      align: 'right',
      render: v => <span className={TD.cell}>{v}</span>,
    },
    {
      title: 'Total',
      dataIndex: 'total',
      key: 'total',
      align: 'right',
      render: v => <span className={`${TD.cell} font-semibold`}>{fmt.format(v || 0)}</span>,
    },
    {
      title: 'Input',
      dataIndex: 'input',
      key: 'input',
      align: 'right',
      render: v => <span className={TD.cell}>{compact(v)}</span>,
    },
    {
      title: 'Output',
      dataIndex: 'output',
      key: 'output',
      align: 'right',
      render: v => <span className={TD.cell}>{compact(v)}</span>,
    },
    {
      title: '费用',
      dataIndex: 'cost',
      key: 'cost',
      align: 'right',
      render: v => <span className={TD.cell}>{v > 0 ? <span className="text-amber-600">${v.toFixed(2)}</span> : '—'}</span>,
    },
    {
      title: '操作',
      key: 'action',
      fixed: 'end',
      align: 'right',
      render: (_, r) => <RowLink onClick={() => onDrill(r)}>详情</RowLink>,
    },
  ];
  return <Table columns={columns} dataSource={rows} rowKey="model" size="small" pagination={{ pageSize: 10 }} locale={{ emptyText: '暂无数据' }} scroll={{ x: 'max-content' }} />;
}

function SessionTable({ rows, onDrill }) {
  const columns = [
    {
      title: '来源',
      key: 'source',
      render: (_, r) => <SourceBadge source={r.source || 'unknown'} />,
    },
    {
      title: '项目',
      dataIndex: 'projectPath',
      key: 'projectPath',
      render: (v) => <span className={`${TD.cell} font-mono text-[11px]`} title={v}>{v || '—'}</span>,
    },
    {
      title: '会话',
      dataIndex: 'sessionCount',
      key: 'sessionCount',
      align: 'right',
      render: v => <span className={`${TD.cell} text-muted-foreground text-[11px]`}>{v || 0}</span>,
    },
    {
      title: '活动',
      dataIndex: 'lastTs',
      key: 'lastTs',
      render: v => <span className={`${TD.cell} text-muted-foreground text-[11px]`}>{formatTs(v)}</span>,
    },
    {
      title: 'Input',
      dataIndex: 'input',
      key: 'input',
      align: 'right',
      render: v => <span className={TD.cell}>{compact(v)}</span>,
    },
    {
      title: 'Output',
      dataIndex: 'output',
      key: 'output',
      align: 'right',
      render: v => <span className={TD.cell}>{compact(v)}</span>,
    },
    {
      title: 'Total',
      dataIndex: 'total',
      key: 'total',
      align: 'right',
      render: v => <span className={`${TD.cell} font-semibold`}>{fmt.format(v || 0)}</span>,
    },
    {
      title: '费用',
      dataIndex: 'cost',
      key: 'cost',
      align: 'right',
      render: v => <span className={TD.cell}>{v > 0 ? <span className="text-amber-600">${v.toFixed(2)}</span> : '—'}</span>,
    },
    {
      title: '操作',
      key: 'action',
      fixed: 'end',
      align: 'right',
      render: (_, r) => <RowLink onClick={() => onDrill(r)}>详情</RowLink>,
    },
  ];
  return <Table columns={columns} dataSource={rows} rowKey={(r) => `${r.source}::${r.device}::${r.projectPath}`} size="small" pagination={{ pageSize: 10 }} locale={{ emptyText: '暂无数据' }} scroll={{ x: 'max-content' }} />;
}
