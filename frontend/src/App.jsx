import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import * as U from './lib/utils.js';
import { Card, CardContent } from './components/ui/card.jsx';
import { Button } from './components/ui/button.jsx';
import { Tabs, TabsList, TabsTrigger } from './components/ui/tabs.jsx';

// Lazy-load components
import TrendChart from './components/charts/TrendChart.jsx';
import TopModels from './components/charts/TopModels.jsx';
import Heatmap from './components/charts/Heatmap/Heatmap.jsx';
import DrillDrawer from './components/tables/DrillDrawer.jsx';
import SourceBadge from './components/SourceBadge.jsx';
import TablePage from './pages/TablePage.jsx';
import MultiSelect from './components/MultiSelect.jsx';
import SettingsDialog from './components/SettingsDialog.jsx';
import ImportDialog from './components/ImportDialog.jsx';

function App() {
  const [M, setM] = useState(null);
  const [loadError, setLoadError] = useState(null);
  const [refreshing, setRefreshing] = useState(false);
  const [collecting, setCollecting] = useState(false);
  const pollingRef = useRef(null);
  const [page, setPage] = useState('dashboard');

  const loadData = useCallback(() => {
    setRefreshing(true);
    Promise.all([
      window.go.main.App.GetDashboardData(),
      window.go.main.App.GetTimeSeriesData()
    ])
      .then(([data, tsData]) => {
        setM({ ...data, daily: data.daily || [], today: U.daysAgo(0), time: tsData.time || [] });
        setLoadError(null);
      })
      .catch(err => setLoadError(String(err)))
      .finally(() => setRefreshing(false));
  }, []);

  useEffect(() => { loadData(); }, [loadData]);

  // Load settings on mount
  useEffect(() => {
    try {
      window.go.main.App.GetSettings().then(cfg => {
        console.log('[settings] loaded:', JSON.stringify(cfg));
        window.go.main.App.SetAutoSyncInterval(cfg.autoSyncMinutes || 5);
      }).catch(err => {
        console.error('[settings] .catch:', err);
        window.go.main.App.SetAutoSyncInterval(5);
      });
    } catch(err) {
      console.error('[settings] try/catch:', err);
    }
  }, []);

  useEffect(() => {
    const check = () => window.go.main.App.CollectStatus().then(s => {
      if (s.status === 'running') { setCollecting(true); }
      else if (s.status === 'ok' || s.status === 'error') { setCollecting(false); if (s.status === 'ok') loadData(); }
    }).catch(() => {});
    check();
    return () => { if (pollingRef.current) clearInterval(pollingRef.current); };
  }, [loadData]);

  const runCollect = useCallback(() => {
    setCollecting(true);
    window.go.main.App.StartCollection();
    if (pollingRef.current) clearInterval(pollingRef.current);
    pollingRef.current = setInterval(() => {
      window.go.main.App.CollectStatus().then(s => {
        if (s.status !== 'running') {
          clearInterval(pollingRef.current);
          pollingRef.current = null;
          setCollecting(false);
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

  return <Dashboard M={M} refreshing={refreshing} collecting={collecting} onRefresh={loadData} onCollect={runCollect} page={page} setPage={setPage} />;
}

function Dashboard({ M, refreshing, collecting, onRefresh, onCollect, page, setPage }) {
  const defaults = { rangeId: 'today', startDate: U.daysAgo(0), endDate: U.daysAgo(0), sources: new Set(), devices: new Set(), models: new Set(), compare: true };
  const [f, setF] = useState(defaults);
  const [trendMode, setTrendMode] = useState('stacked');
  const [drill, setDrill] = useState(null);
  const [autoSync, setAutoSync] = useState(5);

  const handleSettingsChange = useCallback((cfg) => {
    setAutoSync(cfg.autoSyncMinutes);
  }, []);

  const allSources = useMemo(() => [...new Set(M.daily.map(r => r.source))], [M.daily]);
  const allModels = useMemo(() => [...new Set(M.daily.map(r => r.model))].filter(Boolean).sort(), [M.daily]);

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

  const heatmapData = useMemo(() => {
    const m = new Map();
    for (const r of M.daily) m.set(r.usageDate, (m.get(r.usageDate) || 0) + (r.totalTokens || 0));
    return [...m.entries()].map(([date, count]) => ({ date, count }));
  }, [M.daily]);

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
          <div className="ml-6 flex items-center gap-1 bg-muted rounded-lg p-0.5">
            <button className={`px-3 py-1 text-xs rounded-md font-medium transition-colors ${page === 'dashboard' ? 'bg-background shadow-sm text-foreground' : 'text-muted-foreground hover:text-foreground'}`} onClick={() => setPage('dashboard')}>看板</button>
            <button className={`px-3 py-1 text-xs rounded-md font-medium transition-colors ${page === 'table' ? 'bg-background shadow-sm text-foreground' : 'text-muted-foreground hover:text-foreground'}`} onClick={() => setPage('table')}>数据明细</button>
          </div>
        </div>
        <div className="flex items-center gap-2">
          <span className="text-xs text-muted-foreground whitespace-nowrap">最后同步 <strong>{lastSync}</strong></span>
          <Button size="sm" variant="default" onClick={onCollect} disabled={collecting || refreshing}>
            {collecting ? '同步中' : '同步'}
          </Button>
          <ImportDialog onRefresh={onRefresh} />
          <SettingsDialog onSettingsChange={handleSettingsChange} />
        </div>
      </div>

      {/* Page content */}
      {page === 'dashboard' ? (
        <>
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
                <SourceBadge key={s} source={s} selected={f.sources.has(s)} onClick={() => { const n = new Set(f.sources); n.has(s) ? n.delete(s) : n.add(s); setF({ ...f, sources: n }); }} />
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
            <KPI label="Cache" value={U.compactCN(totals.cacheReadTokens)} sub={`${totals.cacheHitRate.toFixed(2)}% 命中`} delta={U.deltaPct(totals.cacheReadTokens, compareData.totals?.cacheReadTokens)} spark={sparkBy('cacheReadTokens')} color="oklch(0.65 0.11 200)" />
            <KPI label="Reasoning" value={U.compactCN(totals.reasoningTokens)} delta={U.deltaPct(totals.reasoningTokens, compareData.totals?.reasoningTokens)} spark={sparkBy('reasoningOutputTokens')} color="oklch(0.65 0.12 150)" />
            <KPI label="费用" value={`$${(totals.costUSD || 0).toFixed(2)}`} delta={U.deltaPct(totals.costUSD, compareData.totals?.costUSD)} spark={sparkBy('costUSD')} color="oklch(0.72 0.14 75)" />
          </div>

          {/* Charts Row */}
          <div className="flex flex-col lg:flex-row gap-4">
            <div className="flex-1 min-w-0">
              <TrendChart rows={filtered} dates={dates} sources={presentSources} mode={trendMode} onModeChange={setTrendMode} totals={totals} timeRows={M.time} isHourly={f.rangeId === 'today'} />
            </div>
            <div className="lg:w-80 2xl:w-96 shrink-0 max-lg:min-h-[260px] lg:relative">
              <div className="flex flex-col min-h-0 lg:absolute lg:inset-0">
                <TopModels rows={filtered} onDrillModel={r => setDrill({ kind: 'model', row: r })} />
              </div>
            </div>
          </div>

          {/* Heatmap */}
          <Heatmap data={heatmapData} />

          <DrillDrawer drill={drill} daily={M.daily} timeRows={M.time} onClose={() => setDrill(null)} />
        </>
      ) : (
        <TablePage M={M} />
      )}
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

export default App;
