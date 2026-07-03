/**
 * 顶栏组件：标题、页面切换、最后同步时间、同步按钮、导入/设置对话框。
 */

import { Button } from '../ui/button.jsx';
import ImportDialog from '../ImportDialog.jsx';
import SettingsDialog from '../SettingsDialog.jsx';

export default function Header({ page, setPage, lastSync, onCollect, collecting, refreshing, onRefresh, onClearData, onSettingsChange }) {
  return (
    <div className="flex items-center justify-between gap-4 pb-4 border-b">
      <div className="flex items-center gap-3">
        <div>
          <h1 className="text-base font-semibold">Token Usage</h1>
          <p className="text-xs text-muted-foreground">Token 消耗看板</p>
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
        <SettingsDialog onSettingsChange={onSettingsChange} onClear={onClearData} />
      </div>
    </div>
  );
}
