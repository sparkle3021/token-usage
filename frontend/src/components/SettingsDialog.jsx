import { useCallback, useEffect, useState } from 'react';
import { Modal, Select, Button } from 'antd';
import { getMessage } from '@/lib/message.js';
import { SettingsIcon, TriangleAlertIcon } from 'lucide-react';

const DEFAULTS = { autoSyncMinutes: 5, ccSwitchDBPath: '', ccSwitchEnabled: false, ccSwitchAutoSync: false };

// 操作确认弹窗：统一交互——说明 + 取消/确认，确认后执行并 toast 反馈。
function ConfirmDialog({ title, description, danger, confirmText, busyText, busy, onConfirm, onCancel }) {
  return (
    <Modal
      open
      onCancel={() => { if (!busy) onCancel(); }}
      onOk={onConfirm}
      okText={busy ? busyText : confirmText}
      okButtonProps={{ danger, disabled: busy, loading: busy }}
      cancelText="取消"
      cancelButtonProps={{ disabled: busy }}
      mask={{ closable: !busy }}
      keyboard={!busy}
      closable={!busy}
      centered
      title={<span className={danger ? 'text-red-600' : ''}>{title}</span>}
      width={{ xs: 416, md: 448 }}
    >
      <p className="text-xs text-muted-foreground leading-relaxed">{description}</p>
    </Modal>
  );
}

export default function SettingsDialog({ onSettingsChange, onClear, onFullSync, fullSyncing }) {
  const [open, setOpen] = useState(false);
  const [cfg, setCfg] = useState(null);
  const [loadErr, setLoadErr] = useState(null);
  const [detecting, setDetecting] = useState(false);
  const [savingAutoSync, setSavingAutoSync] = useState(false);
  const [savingPath, setSavingPath] = useState(false);

  // 当前待确认的操作：null | 'fullSync' | 'updatePricing' | 'clear'
  const [confirmAction, setConfirmAction] = useState(null);
  const [busy, setBusy] = useState(false);

  const toast = getMessage();

  // Load settings on open
  useEffect(() => {
    if (open) {
      setLoadErr(null);
      window.go.main.App.GetSettings().then(cfg => {
        setCfg(cfg);
      }).catch(err => {
        setLoadErr(String(err));
        setCfg({ ...DEFAULTS });
      });
    }
  }, [open]);

  // 保存配置（指定改动的字段），立即持久化到 app_config 并应用运行时。
  const persist = useCallback(async (patch) => {
    const next = { ...cfg, ...patch };
    setCfg(next);
    try {
      await window.go.main.App.SaveSettings(next);
      if (onSettingsChange) onSettingsChange(next);
      return true;
    } catch (err) {
      toast?.error('保存失败', err ? `详情：${String(err)}` : undefined);
      return false;
    }
  }, [cfg, onSettingsChange, toast]);

  // 自动同步间隔：选择即保存
  const saveAutoSync = useCallback(async (v) => {
    setSavingAutoSync(true);
    await persist({ autoSyncMinutes: Number(v) });
    setSavingAutoSync(false);
  }, [persist]);

  // CC-Switch 路径：点"保存"按钮提交
  const savePath = useCallback(async () => {
    setSavingPath(true);
    const ok = await persist({});
    if (ok) toast?.success('CC-Switch 路径已保存');
    setSavingPath(false);
  }, [persist, toast]);

  const detectDB = useCallback(async () => {
    setDetecting(true);
    try {
      const path = await window.go.main.App.DetectCCSwitchDB();
      if (path) {
        setCfg(c => ({ ...c, ccSwitchDBPath: path }));
        toast?.success('已自动检测到数据库路径，点击"保存"生效');
      } else {
        toast?.info('未找到默认路径，请手动填写', '默认路径为 ~/.cc-switch/cc-switch.db');
      }
    } catch (err) {
      toast?.error('检测失败', `详情：${String(err)}`);
    } finally {
      setDetecting(false);
    }
  }, [toast]);

  // 操作执行：确认弹窗 → 执行 → toast 反馈
  const confirmAndRun = useCallback(async () => {
    setBusy(true);
    try {
      switch (confirmAction) {
        case 'fullSync': {
          await onFullSync();
          toast?.success('全量同步已启动', '重读所有数据源，可在顶栏查看进度');
          break;
        }
        case 'updatePricing': {
          const result = await window.go.main.App.UpdatePricing();
          if (result.error) {
            toast?.error('价格更新失败', `详情：${result.error}`);
          } else {
            toast?.success('价格更新成功', result.message || `LiteLLM ${result.litellm} 条`);
          }
          break;
        }
        case 'clear': {
          await window.go.main.App.ClearAllData();
          if (onClear) onClear();
          toast?.success('已清除所有历史数据');
          break;
        }
      }
      setConfirmAction(null);
    } catch (err) {
      toast?.error('操作失败', `详情：${String(err)}`);
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
    updatePricing: {
      title: '更新价格数据',
      danger: false,
      confirmText: '更新',
      busyText: '更新中…',
      description: '将从远程源（LiteLLM）拉取最新模型定价并重载。确定执行？',
    },
    clear: {
      title: '清除所有历史数据',
      danger: true,
      confirmText: '确认清除',
      busyText: '清除中…',
      description: '将清除所有用量数据、采集记录和缓存（含 CC-Switch 检查点），此操作不可撤销。确定继续？',
    },
  };

  const confirm = operationDescs[confirmAction];

  return (
    <>
      <Button size="small" className="h-8 w-8 p-0" icon={<SettingsIcon className="size-4" />} onClick={() => setOpen(true)} />
      <Modal
        open={open}
        onCancel={() => setOpen(false)}
        title="设置"
        centered
        footer={
          <div className="flex justify-end">
            <Button size="small" onClick={() => setOpen(false)}>关闭</Button>
          </div>
        }
        width={{ xs: 480, md: 520, lg: 560 }}
      >
        {!cfg ? (
          <div className="py-8 text-center text-sm text-muted-foreground">
            {loadErr ? <p className="text-red-500">加载失败：{loadErr}</p> : '加载中…'}
          </div>
        ) : (
          <div className="space-y-4 py-2">
            {/* 常规配置 */}
            <div>
              <h4 className="text-xs font-medium text-muted-foreground mb-3">常规配置</h4>
              <div className="space-y-3">
                <div className="space-y-1.5">
                  <label className="text-xs font-medium text-muted-foreground">自动同步间隔</label>
                  <Select
                    value={String(cfg.autoSyncMinutes)}
                    onChange={saveAutoSync}
                    disabled={savingAutoSync}
                    style={{ width: '100%' }}
                    size="small"
                    options={[
                      { value: '1', label: '每 1 分钟' },
                      { value: '5', label: '每 5 分钟' },
                      { value: '10', label: '每 10 分钟' },
                      { value: '15', label: '每 15 分钟' },
                      { value: '30', label: '每 30 分钟' },
                      { value: '0', label: '不同步' },
                    ]}
                  />
                  {savingAutoSync && <p className="text-xs text-muted-foreground">保存中…</p>}
                </div>
                <div className="space-y-1.5">
                  <label className="text-xs font-medium text-muted-foreground">CC-Switch 数据库路径</label>
                  <div className="flex gap-2">
                    <input
                      value={cfg.ccSwitchDBPath}
                      onChange={e => setCfg(c => ({ ...c, ccSwitchDBPath: e.target.value }))}
                      placeholder="例: C:\Users\用户名\.cc-switch\cc-switch.db"
                      className="flex-1 h-8 px-2 text-xs rounded-lg border border-input bg-transparent outline-none focus-visible:border-ring"
                    />
                    <Button size="small" className="h-8 text-xs shrink-0" onClick={detectDB} disabled={detecting}>
                      {detecting ? '检测中…' : '检测'}
                    </Button>
                    <Button type="primary" size="small" className="h-8 text-xs shrink-0" onClick={savePath} disabled={savingPath}>
                      {savingPath ? '保存中…' : '保存'}
                    </Button>
                  </div>
                </div>
              </div>
            </div>

            {/* 数据操作 */}
            <div className="border-t pt-4">
              <h4 className="text-xs font-medium text-muted-foreground mb-3">数据操作</h4>
              <div className="space-y-2">
                <div className="flex items-center justify-between gap-2">
                  <div>
                    <p className="text-xs font-medium">全量同步</p>
                    <p className="text-xs text-muted-foreground">重读所有数据源，耗时较长</p>
                  </div>
                  <Button size="small" className="h-8 text-xs shrink-0" onClick={() => setConfirmAction('fullSync')} disabled={fullSyncing}>
                    {fullSyncing ? '同步中…' : '执行'}
                  </Button>
                </div>
                <div className="flex items-center justify-between gap-2">
                  <div>
                    <p className="text-xs font-medium">更新价格数据</p>
                    <p className="text-xs text-muted-foreground">从 LiteLLM 拉取最新定价</p>
                  </div>
                  <Button size="small" className="h-8 text-xs shrink-0" onClick={() => setConfirmAction('updatePricing')}>
                    执行
                  </Button>
                </div>
                <div className="flex items-center justify-between gap-2">
                  <div>
                    <p className="text-xs font-medium text-red-600">清除所有历史数据</p>
                    <p className="text-xs text-muted-foreground">删除全部用量与缓存，不可撤销</p>
                  </div>
                  <Button size="small" className="h-8 text-xs shrink-0 border-red-300 text-red-500 hover:bg-red-50" onClick={() => setConfirmAction('clear')}>
                    <TriangleAlertIcon className="size-3 mr-1" />
                    执行
                  </Button>
                </div>
              </div>
            </div>
          </div>
        )}
      </Modal>

      {/* 操作确认弹窗 */}
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
