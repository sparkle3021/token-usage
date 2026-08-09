/**
 * 顶栏组件：标题、页面切换、最后同步时间、同步按钮、设置对话框。
 */

import { Button, Segmented } from 'antd';
import { SettingsIcon, Sun, Moon } from 'lucide-react';

export default function Header({ page, setPage, lastSync, onCollect, collecting, refreshing, onOpenSettings, dark, setPref }) {
  return (
    <div className="flex items-center justify-between gap-4 pb-4 border-b flex-wrap">
      <div className="flex items-center gap-3">
        <div>
          <h1 className="text-base font-semibold">Token Usage</h1>
          <p className="text-xs text-muted-foreground">Token 消耗看板</p>
        </div>
        <div className="ml-6">
          <Segmented
            size="small"
            value={page}
            onChange={setPage}
            options={[
              { label: '看板', value: 'dashboard' },
              { label: '用量查询', value: 'quota' },
            ]}
          />
        </div>
      </div>
      <div className="flex items-center gap-2">
        <span className="text-xs text-muted-foreground whitespace-nowrap">最后同步 <strong>{lastSync}</strong></span>
        <Button type="primary" size="small" onClick={onCollect} disabled={collecting || refreshing}>
          {collecting ? '同步中' : '同步'}
        </Button>
        <Button size="small" className="h-8 w-8 p-0" icon={dark ? <Sun className="size-4" /> : <Moon className="size-4" />} onClick={() => setPref(dark ? 'light' : 'dark')} title={dark ? '切换到亮色' : '切换到暗色'} />
        <Button size="small" className="h-8 w-8 p-0" icon={<SettingsIcon className="size-4" />} onClick={onOpenSettings} title="设置" />
      </div>
    </div>
  );
}
