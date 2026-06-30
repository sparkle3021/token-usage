import { useCallback, useEffect, useState } from 'react';
import {
  Dialog, DialogContent, DialogHeader, DialogTitle,
} from '@/components/ui/dialog.jsx';
import { Button } from '@/components/ui/button.jsx';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select.jsx';
import { SettingsIcon } from 'lucide-react';

const DEFAULTS = { autoSyncMinutes: 5, ccSwitchDBPath: '', ccSwitchEnabled: false, ccSwitchAutoSync: false };

export default function SettingsDialog({ onSettingsChange }) {
  const [open, setOpen] = useState(false);
  const [cfg, setCfg] = useState(null);
  const [saving, setSaving] = useState(false);
  const [loadErr, setLoadErr] = useState(null);
  const [detecting, setDetecting] = useState(false);
  const [detectMsg, setDetectMsg] = useState(null);

  // Load settings on open
  useEffect(() => {
    if (open) {
      setLoadErr(null);
      window.go.main.App.GetSettings().then(cfg => {
        console.log('[settings] loaded:', JSON.stringify(cfg));
        setCfg(cfg);
      }).catch(err => {
        console.error('[settings] load failed:', err);
        setLoadErr(String(err));
        setCfg({ ...DEFAULTS });
      });
    }
  }, [open]);

  const save = useCallback(async () => {
    if (!cfg) return;
    setSaving(true);
    try {
      await window.go.main.App.SaveSettings(cfg);
      if (onSettingsChange) onSettingsChange(cfg);
      setOpen(false);
    } catch (err) {
      console.error('[settings]', err);
    } finally {
      setSaving(false);
    }
  }, [cfg, onSettingsChange]);

  const detectDB = useCallback(async () => {
    setDetecting(true);
    setDetectMsg(null);
    try {
      const path = await window.go.main.App.DetectCCSwitchDB();
      if (path) {
        setCfg(c => ({ ...c, ccSwitchDBPath: path }));
        setDetectMsg({ type: 'success', text: '已自动检测到数据库路径' });
      } else {
        setDetectMsg({ type: 'info', text: '未找到默认路径，请手动填写' });
      }
    } catch (err) {
      setDetectMsg({ type: 'error', text: '检测失败：' + String(err) });
    } finally {
      setDetecting(false);
    }
  }, []);

  return (
    <>
      <Button size="sm" variant="outline" className="h-8 w-8 p-0" onClick={() => setOpen(true)}>
        <SettingsIcon className="size-4" />
      </Button>
      <Dialog open={open} onOpenChange={setOpen}>
        <DialogContent className="sm:max-w-md">
          <DialogHeader>
            <DialogTitle>设置</DialogTitle>
          </DialogHeader>

          {!cfg ? (
            <div className="py-8 text-center text-sm text-muted-foreground">
              {loadErr ? <p className="text-red-500">加载失败：{loadErr}</p> : '加载中…'}
            </div>
          ) : (
            <div className="space-y-4 py-2">
              {/* 同步间隔 */}
              <div className="space-y-1.5">
                <label className="text-xs font-medium text-muted-foreground">自动同步间隔</label>
                <Select value={String(cfg.autoSyncMinutes)} onValueChange={v => setCfg(c => ({ ...c, autoSyncMinutes: Number(v) }))}>
                  <SelectTrigger className="w-full h-8 text-xs">
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent side="bottom" align="start">
                    <SelectItem value="1">每 1 分钟</SelectItem>
                    <SelectItem value="5">每 5 分钟</SelectItem>
                    <SelectItem value="10">每 10 分钟</SelectItem>
                    <SelectItem value="15">每 15 分钟</SelectItem>
                    <SelectItem value="30">每 30 分钟</SelectItem>
                    <SelectItem value="0">不同步</SelectItem>
                  </SelectContent>
                </Select>
              </div>

              <div className="border-t pt-4">
                <h4 className="text-xs font-medium text-muted-foreground mb-3">CC-Switch 集成</h4>

                <div className="space-y-1.5 mb-3">
                  <label className="text-xs font-medium text-muted-foreground">数据库路径</label>
                  <div className="flex gap-2">
                    <input
                      value={cfg.ccSwitchDBPath}
                      onChange={e => setCfg(c => ({ ...c, ccSwitchDBPath: e.target.value }))}
                      placeholder="例: C:\Users\用户名\.cc-switch\cc-switch.db"
                      className="flex-1 h-8 px-2 text-xs rounded-lg border border-input bg-transparent outline-none focus-visible:border-ring"
                    />
                    <Button size="sm" variant="outline" className="h-8 text-xs shrink-0" onClick={detectDB} disabled={detecting}>
                      {detecting ? '检测中…' : '检测'}
                    </Button>
                  </div>
                  {detectMsg && (
                    <p className={`text-xs mt-1 ${detectMsg.type === 'success' ? 'text-green-600' : detectMsg.type === 'error' ? 'text-red-500' : 'text-muted-foreground'}`}>
                      {detectMsg.text}
                    </p>
                  )}
                </div>
              </div>
            </div>
          )}

          <div className="flex justify-end gap-2 pt-2 border-t">
            <Button size="sm" variant="outline" onClick={() => setOpen(false)}>取消</Button>
            <Button size="sm" onClick={save} disabled={saving || !cfg}>{saving ? '保存中…' : '保存'}</Button>
          </div>
        </DialogContent>
      </Dialog>
    </>
  );
}
