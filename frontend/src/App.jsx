import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import * as U from './lib/utils.js';
import { Card, CardContent } from './components/ui/card.jsx';
import { Badge } from './components/ui/badge.jsx';
import { Button } from './components/ui/button.jsx';
import { Tabs, TabsList, TabsTrigger } from './components/ui/tabs.jsx';

// Lazy-load chart/table components
import TrendChart from './components/charts/TrendChart.jsx';
import TopModels from './components/charts/TopModels.jsx';
import Heatmap from './components/charts/Heatmap.jsx';
import TablePanel from './components/tables/TablePanel.jsx';
import DrillDrawer from './components/tables/DrillDrawer.jsx';
import SourceIcon from './components/SourceIcon.jsx';

function App() {
  const [M, setM] = useState(null);
  const [loadError, setLoadError] = useState(null);
  const [refreshing, setRefreshing] = useState(false);
  const [collecting, setCollecting] = useState(false);
  const [collectStatus, setCollectStatus] = useState(null);
  const pollingRef = useRef(null);

  const loadData = useCallback(() => {
    setRefreshing(true);
    window.go.main.App.GetDashboardData()
      .then(data => {
        setM({ ...data, daily: data.daily || [], today: U.daysAgo(0) });
        setLoadError(null);
      })
      .catch(err => setLoadError(String(err)))
      .finally(() => setRefreshing(false));
  }, []);

  useEffect(() => { loadData(); }, [loadData]);

  useEffect(() => {
    const check = () => window.go.main.App.CollectStatus().then(s => {
      if (s.status === 'running') { setCollecting(true); setCollectStatus(s); }
      else if (s.status === 'ok' || s.status === 'error') { setCollecting(false); setCollectStatus(s); if (s.status === 'ok') loadData(); }
    }).catch(() => {});
    check();
    return () => { if (pollingRef.current) clearInterval(pollingRef.current); };
  }, [loadData]);

  const runCollect = useCallback(() => {
    setCollecting(true);
    setCollectStatus({ status: 'running', message: '正在采集本机用量…' });
    window.go.main.App.StartCollection();
    if (pollingRef.current) clearInterval(pollingRef.current);
    pollingRef.current = setInterval(() => {
      window.go.main.App.CollectStatus().then(s => {
        if (s.status !== 'running') {
          clearInterval(pollingRef.current);
          pollingRef.current = null;
          setCollecting(false);
          setCollectStatus(s);
          if (s.status === 'ok') loadData();
        }
      }).catch(() => {});
    }, 1500);
  }, [loadData]);

  if (loadError) return (
    <div className="flex flex-col items-center justify-center h-screen gap-4 text-muted-foreground">
      <p className="text-sm">加载失败：{loadError}</p>
      <Button onClick={loadData}>重试</Button>
    </div>
  );

  if (!M) return (
    <div className="flex flex-col items-center justify-center h-screen gap-3 text-muted-foreground">
      <div className="animate-spin w-8 h-8 border-2 border-foreground/20 border-t-foreground rounded-full" />
      <p className="text-sm">正在加载数据…</p>
    </div>
  );

  return <Dashboard M={M} refreshing={refreshing} collecting={collecting} collectStatus={collectStatus} onRefresh={loadData} onCollect={runCollect} />;
}

function Dashboard({ M, refreshing, collecting, collectStatus, onRefresh, onCollect }) {
  const defaults = { rangeId: 'today', startDate: U.daysAgo(0), endDate: U.daysAgo(0), sources: new Set(), devices: new Set(), models: new Set(), compare: false };
  const [f, setF] = useState(defaults);
  const [trendMode, setTrendMode] = useState('stacked');
  const [drill, setDrill] = useState(null);

  const allSources = useMemo(() => [...new Set(M.daily.map(r => r.source))], [M.daily]);
  const allModels = useMemo(() => [...new Set(M.daily.map(r => r.model))].filter(Boolean), [M.daily]);

  const filtered = useMemo(() => {
    return U.filterDaily(M.daily, f);
  }, [f, M.daily]);

  const totals = useMemo(() => U.aggregateTotals(filtered), [filtered]);
  const dates = useMemo(() => U.rangeDates(f.startDate, f.endDate), [f]);

  const compareData = useMemo(() => {
    if (!f.compare) return { totals: null };
    const days = dates.length;
    const endStr = U.addDays(f.startDate, -1);
    const startStr = U.addDays(endStr, -(days - 1));
    return { totals: U.aggregateTotals(U.filterDaily(M.daily, { ...f, startDate: startStr, endDate: endStr })) };
  }, [f, dates, M.daily]);

  const dailyMap = useMemo(() => {
    const m = new Map();
    for (const r of filtered) m.set(r.usageDate, (m.get(r.usageDate) || 0) + r.totalTokens);
    return m;
  }, [filtered]);

  const sparkValues = useMemo(() => dates.map(d => dailyMap.get(d) || 0), [dates, dailyMap]);

  const sparkBy = useMemo(() => (key) => {
    const m = new Map();
    for (const r of filtered) m.set(r.usageDate, (m.get(r.usageDate) || 0) + (r[key] || 0));
    return dates.map(d => m.get(d) || 0);
  }, [filtered, dates]);

  const presentSources = useMemo(() => Array.from(f.sources.size ? f.sources : new Set(allSources)), [f.sources, allSources]);

  const setRange = (rangeId) => {
    if (rangeId === 'all') {
      const sorted = M.daily.map(x => x.usageDate).filter(Boolean).sort();
      setF({ ...f, rangeId, startDate: sorted[0] || U.daysAgo(0), endDate: sorted[sorted.length - 1] || U.daysAgo(0) });
    } else {
      const days = { today: 1, '7d': 7, '14d': 14, '30d': 30, '90d': 90 }[rangeId] || 30;
      setF({ ...f, rangeId, startDate: U.daysAgo(days - 1), endDate: U.daysAgo(0) });
    }
  };

  const lastSync = M.runs?.[0]?.collectedAt ? U.formatTs(M.runs[0].collectedAt) : '—';
  const ranges = [
    { id: 'today', label: '今天' }, { id: '7d', label: '7 天' }, { id: '14d', label: '14 天' },
    { id: '30d', label: '30 天' }, { id: '90d', label: '90 天' }, { id: 'all', label: '全部' },
  ];

  return (
    <div className="max-w-[1440px] mx-auto p-4 md:p-6 pb-16 space-y-4 font-sans">
      {/* Topbar */}
      <div className="flex items-center justify-between gap-4 pb-4 border-b">
        <div className="flex items-center gap-3">
          <div className="w-9 h-9 rounded-lg grid place-items-center text-white font-bold text-sm bg-gradient-to-br from-indigo-700 to-indigo-900">TS</div>
          <div>
            <h1 className="text-base font-semibold">Token Studio</h1>
            <p className="text-xs text-muted-foreground">个人 AI Token 消耗看板</p>
          </div>
        </div>
        <div className="flex items-center gap-2">
          {collectStatus && collectStatus.status === 'running' && <Badge variant="outline" className="text-indigo-600">采集中</Badge>}
          {collectStatus && collectStatus.status === 'ok' && <Badge variant="outline" className="text-green-600">采集完成</Badge>}
          {collectStatus && collectStatus.status === 'error' && <Badge variant="outline" className="text-red-500">采集失败</Badge>}
          <span className="text-xs text-muted-foreground whitespace-nowrap">最后同步 <strong>{lastSync}</strong></span>
          <Button size="sm" variant="default" onClick={onCollect} disabled={collecting || refreshing}>{collecting ? '采集中' : '采集'}</Button>
          <Button size="sm" variant="outline" onClick={onRefresh} disabled={refreshing}>{refreshing ? '同步中' : '刷新'}</Button>
        </div>
      </div>

      {/* Filter Bar */}
      <Card className="p-3 overflow-visible">
        <div className="flex flex-wrap items-center gap-2 mb-2">
          <span className="text-[10px] uppercase tracking-wider text-muted-foreground font-medium">时间</span>
          <Tabs value={f.rangeId} onValueChange={setRange}>
            <TabsList>{ranges.map(r => <TabsTrigger key={r.id} value={r.id} className="text-xs px-2.5">{r.label}</TabsTrigger>)}</TabsList>
          </Tabs>
        </div>
        <div className="flex flex-wrap items-center gap-1.5">
          <span className="text-[10px] uppercase tracking-wider text-muted-foreground font-medium mr-1">来源</span>
          {allSources.map(s => (
            <Badge key={s} variant={f.sources.has(s) ? 'default' : 'outline'} className="cursor-pointer text-xs gap-1"
              onClick={() => { const n = new Set(f.sources); n.has(s) ? n.delete(s) : n.add(s); setF({ ...f, sources: n }); }}>
              <SourceIcon name={s} className="w-3 h-3" />{s}
            </Badge>
          ))}
        </div>
        <div className="flex flex-wrap items-center gap-2 mt-2 pt-2 border-t">
          <MultiSelect items={allModels} selected={f.models} onChange={v => setF({ ...f, models: v })} placeholder="全部模型" />
          <div className="flex-1" />
          <Button size="sm" variant={f.compare ? 'default' : 'outline'} onClick={() => setF({ ...f, compare: !f.compare })}>
            对比
          </Button>
        </div>
      </Card>

      {/* KPI Cards */}
      <div className="grid grid-cols-2 sm:grid-cols-3 lg:grid-cols-6 gap-3">
        <KPI label="总 Token" value={U.compactCN(totals.totalTokens)} delta={U.deltaPct(totals.totalTokens, compareData.totals?.totalTokens)} spark={sparkValues} color="oklch(0.55 0.16 265)" />
        <KPI label="Input" value={U.compactCN(totals.inputTokens)} delta={U.deltaPct(totals.inputTokens, compareData.totals?.inputTokens)} spark={sparkBy('inputTokens')} color="oklch(0.62 0.13 240)" />
        <KPI label="Output" value={U.compactCN(totals.outputTokens)} delta={U.deltaPct(totals.outputTokens, compareData.totals?.outputTokens)} spark={sparkBy('outputTokens')} color="oklch(0.60 0.15 295)" />
        <KPI label="Cache" value={U.compactCN(totals.cacheReadTokens)} sub={`${totals.cacheHitRate.toFixed(0)}% 命中`} delta={U.deltaPct(totals.cacheReadTokens, compareData.totals?.cacheReadTokens)} spark={sparkBy('cacheReadTokens')} color="oklch(0.65 0.11 200)" />
        <KPI label="Reasoning" value={U.compactCN(totals.reasoningTokens)} delta={U.deltaPct(totals.reasoningTokens, compareData.totals?.reasoningTokens)} spark={sparkBy('reasoningOutputTokens')} color="oklch(0.65 0.12 150)" />
        <KPI label="费用" value={`$${(totals.costUSD || 0).toFixed(2)}`} delta={U.deltaPct(totals.costUSD, compareData.totals?.costUSD)} spark={sparkBy('costUSD')} color="oklch(0.72 0.14 75)" />
      </div>

      {/* Charts Grid */}
      <div className="grid grid-cols-12 gap-4">
        <div className="col-span-12 lg:col-span-8">
          <TrendChart rows={filtered} dates={dates} sources={presentSources} mode={trendMode} onModeChange={setTrendMode} totals={totals} />
        </div>
        <div className="col-span-12 lg:col-span-4">
          <TopModels rows={filtered} onDrillModel={r => setDrill({ kind: 'model', row: r })} />
        </div>
        <div className="col-span-12">
          <Heatmap rows={filtered} dates={dates} />
        </div>
        <div className="col-span-12">
          <TablePanel daily={filtered} sessions={M.sessions} runs={M.runs} onDrill={setDrill} />
        </div>
      </div>

      <DrillDrawer drill={drill} daily={M.daily} onClose={() => setDrill(null)} />
    </div>
  );
}

// ── KPI Card ──────────────────────────────────────────────────

function KPI({ label, value, sub, delta, spark, color }) {
  return (
    <Card className="relative overflow-hidden">
      <CardContent className="pt-4 px-4 pb-4">
        <div className="text-[11px] text-muted-foreground font-medium mb-1.5">{label}</div>
        <div className="text-xl font-semibold tabular-nums leading-tight">{value}</div>
        <div className="flex items-center gap-1.5 mt-2 text-xs text-muted-foreground">
          {delta != null && (
            <span className={`inline-flex items-center gap-0.5 font-semibold text-[11px] px-1 rounded ${delta > 0.05 ? 'text-green-600 bg-green-50' : delta < -0.05 ? 'text-red-500 bg-red-50' : 'text-muted-foreground bg-muted'}`}>
              {delta > 0.05 ? '↑' : delta < -0.05 ? '↓' : '·'} {Math.abs(delta).toFixed(1)}%
            </span>
          )}
          <span className="truncate">{sub || ''}</span>
        </div>
        {spark && spark.length > 1 && <SparkLine values={spark} color={color} />}
      </CardContent>
    </Card>
  );
}

function SparkLine({ values, color }) {
  const w = 100, h = 28;
  const max = Math.max(...values, 1);
  const pts = values.map((v, i) => [(i / (values.length - 1 || 1)) * w, h - (v / max) * (h - 2) - 1]);
  const d = pts.map((p, i) => `${i === 0 ? 'M' : 'L'}${p[0]},${p[1]}`).join(' ');
  return (
    <svg className="absolute right-0 bottom-0 w-full pointer-events-none opacity-20" viewBox={`0 0 ${w} ${h}`} preserveAspectRatio="none" style={{ height: 28 }}>
      <path d={d + ` L${w},${h} L0,${h} Z`} fill={color} opacity="0.12" />
      <path d={d} fill="none" stroke={color} strokeWidth="1.5" />
    </svg>
  );
}

// ── MultiSelect ───────────────────────────────────────────────

function MultiSelect({ items, selected, onChange, placeholder }) {
  const [open, setOpen] = useState(false);
  const ref = useRef(null);
  useEffect(() => {
    const f = (e) => { if (ref.current && !ref.current.contains(e.target)) setOpen(false); };
    document.addEventListener('mousedown', f);
    return () => document.removeEventListener('mousedown', f);
  }, []);

  return (
    <div ref={ref} className="relative inline-block">
      <Button size="sm" variant="outline" onClick={() => setOpen(o => !o)} className="text-xs">
        {selected.size === 0 ? placeholder : selected.size === 1 ? [...selected][0] : `${selected.size} 项`}
      </Button>
      {open && (
        <div className="absolute top-full left-0 mt-1 z-30 min-w-[180px] bg-popover border rounded-lg shadow-lg p-1.5 max-h-64 overflow-y-auto">
          {selected.size > 0 && <Button size="xs" variant="ghost" className="w-full justify-start text-indigo-500 mb-0.5" onClick={() => onChange(new Set())}>清除</Button>}
          {(items || []).map(o => (
            <button key={o} className={`w-full text-left px-2 py-1 text-xs rounded flex items-center gap-2 hover:bg-muted ${selected.has(o) ? 'font-medium' : ''}`}
              onClick={() => { const n = new Set(selected); n.has(o) ? n.delete(o) : n.add(o); onChange(n); }}>
              <input type="checkbox" className="accent-indigo-500" checked={selected.has(o)} readOnly />
              {o}
            </button>
          ))}
        </div>
      )}
    </div>
  );
}

export default App;
