/**
 * 应用入口：编排数据流、采集状态、设置管理。
 * 使用 FilterProvider 包裹全局过滤器，DashboardPage 与 TablePage 共享同一数据源。
 */

import { useState, useCallback } from 'react';
import { useDashboardData } from './hooks/useDashboardData.js';
import { useCollection } from './hooks/useCollection.js';
import { useSettings } from './hooks/useSettings.js';
import { FilterProvider } from './store/filterStore.jsx';
import { formatTs } from './lib/formatters.js';
import { Button } from './components/ui/button.jsx';
import Header from './components/layout/Header.jsx';
import DashboardPage from './pages/DashboardPage.jsx';
import TablePage from './pages/TablePage.jsx';

function AppContent() {
  const [page, setPage] = useState('dashboard');
  const { M, loadError, refreshing, fetchData, allSources, allModels, heatmapData } = useDashboardData();
  const { collecting, runCollect } = useCollection(fetchData);
  const { handleSettingsChange } = useSettings(fetchData);

  const onClearData = useCallback(() => {
    window.go.main.App.ClearAllData().then(() => fetchData(true)).catch(() => {});
  }, [fetchData]);

  const lastSync = M?.runs?.[0]?.collectedAt ? formatTs(M.runs[0].collectedAt) : '—';

  if (loadError) return (
    <div className="flex flex-col items-center justify-center h-screen gap-4 text-muted-foreground">
      <p className="text-sm">加载失败：{loadError}</p>
      <Button onClick={() => fetchData(false)}>重试</Button>
    </div>
  );

  if (!M) return (
    <div className="flex flex-col items-center justify-center h-screen gap-3 text-muted-foreground">
      <div className="animate-spin w-8 h-8 border-2 border-foreground/20 border-t-foreground rounded-full" />
      <p className="text-sm">正在加载数据…</p>
    </div>
  );

  return (
    <div className="max-w-[1440px] mx-auto p-4 md:p-6 pb-16 space-y-4 font-sans">
      <Header
        page={page}
        setPage={setPage}
        lastSync={lastSync}
        onCollect={runCollect}
        collecting={collecting}
        refreshing={refreshing}
        onRefresh={fetchData}
        onClearData={onClearData}
        onSettingsChange={handleSettingsChange}
      />
      {page === 'dashboard' ? (
        <DashboardPage
          M={M}
          allSources={allSources}
          allModels={allModels}
          heatmapData={heatmapData}
          onRefresh={fetchData}
        />
      ) : (
        <TablePage M={M} onRefresh={fetchData} />
      )}
    </div>
  );
}

export default function App() {
  return (
    <FilterProvider>
      <AppContent />
    </FilterProvider>
  );
}
