/**
 * 应用入口：编排数据流、采集状态、设置管理。
 * 使用 FilterProvider 包裹全局过滤器，DashboardPage 与 TablePage 共享同一数据源。
 * ConfigProvider 由 dark 状态驱动 antd 明暗主题（默认主题，仅 algorithm 切换）。
 */

import { useState, useCallback, useEffect } from 'react';
import { App as AntdApp, Button, ConfigProvider, theme as antdTheme } from 'antd';
import { useDashboardData } from './hooks/useDashboardData.js';
import { useCollection } from './hooks/useCollection.js';
import { useSettings } from './hooks/useSettings.js';
import { FilterProvider } from './store/filterStore.jsx';
import { formatTs } from './lib/formatters.js';
import { setMessageApi } from './lib/message.js';
import Header from './components/layout/Header.jsx';
import DashboardPage from './pages/DashboardPage.jsx';
import TablePage from './pages/TablePage.jsx';
import QuotaPage from './pages/QuotaPage.jsx';
import { WindowSetDarkTheme, WindowSetLightTheme, WindowSetBackgroundColour } from '../wailsjs/runtime/runtime.js';

const THEME_KEY = 'app-theme';

function AppContent({ dark, onToggleDark }) {
  const { message } = AntdApp.useApp();
  setMessageApi(message);
  const [page, setPage] = useState('dashboard');

  const { M, loadError, refreshing, fetchData, fetchTimeSeries, allSources, allModels, heatmapData } = useDashboardData();
  const { collecting, runCollect, runFullCollect } = useCollection(fetchData);
  const { handleSettingsChange } = useSettings(fetchData);

  const onClearData = useCallback(() => {
    window.go.main.App.ClearAllData().then(() => fetchData(true)).catch(() => {});
  }, [fetchData]);

  const lastSync = M?.runs?.[0]?.collectedAt ? formatTs(M.runs[0].collectedAt) : '—';

  if (loadError) return (
    <div className="flex flex-col items-center justify-center h-screen gap-4 text-muted-foreground">
      <p className="text-sm">加载失败：{loadError}</p>
      <Button type="primary" onClick={() => fetchData(false)}>重试</Button>
    </div>
  );

  if (!M) return (
    <div className="flex flex-col items-center justify-center h-screen gap-3 text-muted-foreground">
      <div className="animate-spin w-8 h-8 border-2 border-foreground/20 border-t-foreground rounded-full" />
      <p className="text-sm">正在加载数据…</p>
    </div>
  );

  return (
    <div className="max-w-[1440px] mx-auto p-4 md:p-6 pb-16 font-sans flex flex-col h-screen overflow-hidden">
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
        onFullSync={runFullCollect}
        fullSyncing={collecting}
        dark={dark}
        onToggleDark={onToggleDark}
      />
      <div className="flex-1 min-h-0 overflow-y-auto scrollbar-none pt-4">
        {page === 'dashboard' ? (
          <DashboardPage
            M={M}
            allSources={allSources}
            allModels={allModels}
            heatmapData={heatmapData}
            onRangeSwitch={fetchTimeSeries}
          />
        ) : page === 'quota' ? (
          <QuotaPage />
        ) : (
          <TablePage M={M} onRangeSwitch={fetchTimeSeries} />
        )}
      </div>
    </div>
  );
}

export default function App() {
  const [dark, setDark] = useState(() => {
    try { return localStorage.getItem(THEME_KEY) === 'dark'; } catch { return false; }
  });
  useEffect(() => {
    document.documentElement.classList.toggle('dark', dark);
    try { localStorage.setItem(THEME_KEY, dark ? 'dark' : 'light'); } catch { /* ignore */ }
    // 同步 Wails 窗口标题栏主题与背景色（#f3f3f3 亮 / #202020 暗）
    try {
      if (dark) WindowSetDarkTheme(); else WindowSetLightTheme();
      WindowSetBackgroundColour(dark ? 32 : 243, dark ? 32 : 243, dark ? 32 : 243, 1);
    } catch { /* ignore */ }
  }, [dark]);

  return (
    <ConfigProvider theme={{ algorithm: dark ? antdTheme.darkAlgorithm : antdTheme.defaultAlgorithm }} button={{ autoInsertSpace: false }}>
      <AntdApp>
        <FilterProvider>
          <AppContent dark={dark} onToggleDark={() => setDark(d => !d)} />
        </FilterProvider>
      </AntdApp>
    </ConfigProvider>
  );
}
