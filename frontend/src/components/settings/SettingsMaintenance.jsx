import { useCallback, useState } from 'react';
import { Button } from 'antd';
import { TriangleAlertIcon } from 'lucide-react';
import { getMessage } from '@/lib/message.js';
import { clearAllData, exportData, importData } from '@/api/client.js';
import SettingField from '@/components/common/SettingField.jsx';
import ConfirmDialog from '@/components/common/ConfirmDialog.jsx';

/**
 * 维护档：数据操作（导出/导入/全量同步/更新价格/清除）。
 * 导出/导入直调；全量同步/更新价格/清除经确认弹窗执行，结果统一 toast。
 */
export default function SettingsMaintenance({ onClear, onFullSync, fullSyncing }) {
  const [exporting, setExporting] = useState(false);
  // 当前待确认的操作：null | 'fullSync' | 'updatePricing' | 'clear' | 'import'
  const [confirmAction, setConfirmAction] = useState(null);
  const [busy, setBusy] = useState(false);

  const toast = getMessage();

  // 导出用量数据（hour/session + 设备映射 → 用户选择路径的 JSON 文件）
  const handleExport = useCallback(async () => {
    setExporting(true);
    try {
      const path = await exportData();
      if (path) {
        toast?.success(`已导出：${path}`);
      }
      // path 为空 = 用户在对话框取消，静默
    } catch (err) {
      console.error('[settings] export failed', err);
      toast?.error('导出失败，请重试');
    } finally {
      setExporting(false);
    }
  }, [toast]);

  // 操作执行：确认弹窗 → 执行 → toast 反馈
  const confirmAndRun = useCallback(async () => {
    setBusy(true);
    try {
      switch (confirmAction) {
        case 'fullSync': {
          await onFullSync();
          toast?.success('全量同步已启动，重读所有数据源，可在顶栏查看进度');
          break;
        }
        case 'clear': {
          await clearAllData();
          if (onClear) onClear();
          toast?.success('已清除所有历史数据');
          break;
        }
        case 'import': {
          const result = await importData();
          if (result) {
            toast?.success(`已导入：${result.hours} 小时行 / ${result.daily} 日行 / ${result.sessions} 会话行，新增 ${result.devices} 个设备`);
          }
          break;
        }
      }
      setConfirmAction(null);
    } catch (err) {
      console.error('[settings] operation failed', err);
      toast?.error('操作失败，请重试');
    } finally {
      setBusy(false);
    }
  }, [confirmAction, onFullSync, onClear, toast]);

  const operationDescs = {
    fullSync: {
      title: '全量同步',
      danger: false,
      confirmText: '开始同步',
      busyText: '同步中…',
      description: '将重读所有数据源（Claude Code / Codex / Gemini / OpenClaw / CC-Switch / OpenCode 等），耗时较长。确定执行？',
    },
    clear: {
      title: '清除所有历史数据',
      danger: true,
      confirmText: '确认清除',
      busyText: '清除中…',
      description: '将清除所有用量数据、采集记录和缓存（含 CC-Switch 检查点），此操作不可撤销。确定继续？',
    },
    import: {
      title: '导入数据',
      danger: false,
      confirmText: '选择文件导入',
      busyText: '导入中…',
      description: '将选择导出的 JSON 文件，合并其用量数据与设备映射到本机。重复导入不会重复计数。确定继续？',
    },
  };

  const confirm = operationDescs[confirmAction];

  return (
    <>
      <div className="space-y-6 max-w-xl">
        <SettingField title="导入 / 导出数据" description="导出用量为 JSON 供跨设备合并；从导出 JSON 合并用量与设备，重复导入不会重复计数。">
          <div className="flex gap-2">
            <Button onClick={handleExport} disabled={exporting}>
              {exporting ? '导出中…' : '导出'}
            </Button>
            <Button onClick={() => setConfirmAction('import')}>
              导入
            </Button>
          </div>
        </SettingField>

        <SettingField title="全量同步" description="重读所有数据源，耗时较长。">
          <Button type="primary" onClick={() => setConfirmAction('fullSync')} disabled={fullSyncing}>
            {fullSyncing ? '同步中…' : '执行'}
          </Button>
        </SettingField>

        <SettingField title="清除所有历史数据" description="删除全部用量、采集记录与缓存，不可撤销。">
          <Button danger onClick={() => setConfirmAction('clear')}>
            <TriangleAlertIcon className="size-3 mr-1" />
            执行
          </Button>
        </SettingField>
      </div>

      {confirm && (
        <ConfirmDialog
          title={confirm.title}
          description={confirm.description}
          danger={confirm.danger}
          confirmText={confirm.confirmText}
          busyText={confirm.busyText}
          busy={busy}
          onConfirm={confirmAndRun}
          onCancel={() => setConfirmAction(null)}
        />
      )}
    </>
  );
}
