/**
 * 应用入口：编排数据流、采集状态、设置管理。
 * 使用 FilterProvider 包裹全局过滤器，DashboardPage 与 TablePage 共享同一数据源。
 * ConfigProvider 由 dark 状态驱动 antd 明暗主题（默认主题，仅 algorithm 切换）。
 */

import { Component, useState, useCallback, useEffect } from 'react';
import { App as AntdApp, Button, ConfigProvider, theme as antdTheme } from 'antd';
import { useDashboardData } from '@/hooks/useDashboardData.js';
import { useCollection } from '@/hooks/useCollection.js';
import { useSettings } from '@/hooks/useSettings.js';
import { FilterProvider } from '@/store/filterStore.jsx';
import { formatTs } from '@/lib/formatters.js';
import { setMessageApi } from '@/lib/message.js';
import { clearAllData } from '@/api/client.js';
import Header from '@/components/layout/Header.jsx';
import DashboardPage from '@/pages/DashboardPage.jsx';
import QuotaPage from '@/pages/QuotaPage.jsx';
import { WindowSetDarkTheme, WindowSetLightTheme, WindowSetBackgroundColour } from '../wailsjs/runtime/runtime.js';

const THEME_KEY = 'app-theme';

// 全局错误边界：渲染异常时展示错误而非白屏，便于定位（频繁切时间范围等场景）。
class ErrorBoundary extends Component {
  constructor(props) {
    super(props);
    this.state = { error: null, stack: null };
  }
  static getDerivedStateFromError(error) {
    return { error };
  }
  componentDidCatch(error, errorInfo) {
    this.setState({ stack: errorInfo.componentStack });
  }
  render() {
    if (this.state.error) {
      const msg = this.state.error && (this.state.error.stack || this.state.error.message || String(this.state.error));
      return (
        <div className="flex flex-col items-center justify-center h-screen gap-3 p-8 text-center">
          <h2 className="text-base font-semibold text-red-600">页面渲染出错</h2>
          <pre className="text-xs text-muted-foreground whitespace-pre-wrap text-left max-w-2xl bg-muted/40 p-4 rounded-lg overflow-auto max-h-48">{msg}</pre>
          {this.state.stack && (
            <pre className="text-xs text-foreground/70 whitespace-pre-wrap text-left max-w-2xl bg-muted/40 p-4 rounded-lg overflow-auto max-h-48">组件栈：{this.state.stack}</pre>
          )}
          <button
            className="px-3 py-1 text-xs rounded-md bg-foreground/90 text-background"
            onClick={() => { this.setState({ error: null, stack: null }); window.location.reload(); }}
          >
            重新加载
          </button>
        </div>
      );
    }
    return this.props.children;
  }
}

function AppContent({ dark, onToggleDark }) {
  const { message } = AntdApp.useApp();
  setMessageApi(message);
  const [page, setPage] = useState('dashboard');

  const { M, loadError, refreshing, fetchData, fetchTimeSeries, allSources, allModels, heatmapData } = useDashboardData();
  const [lastSyncTs, setLastSyncTs] = useState(null);

  // 同步完成（手动采集轮询结束 / 后端自动同步 collection:done）统一在此记录时间，
  // 作为 Header「最后同步」显示来源，不查库。
  const onDataChange = useCallback(() => {
    setLastSyncTs(Date.now());
    fetchData(true);
  }, [fetchData]);

  const { collecting, runCollect, runFullCollect } = useCollection(onDataChange);
  const { handleSettingsChange } = useSettings(onDataChange);

  const onClearData = useCallback(() => {
    clearAllData().then(() => fetchData(true)).catch(() => {});
  }, [fetchData]);

  const lastSync = lastSyncTs ? formatTs(new Date(lastSyncTs).toISOString()) : '—';

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
        ) : (
          <QuotaPage />
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
          <ErrorBoundary>
            <AppContent dark={dark} onToggleDark={() => setDark(d => !d)} />
          </ErrorBoundary>
        </FilterProvider>
      </AntdApp>
    </ConfigProvider>
  );
}
