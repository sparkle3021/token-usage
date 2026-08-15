import { useCallback, useEffect, useState } from 'react';
import { Select, Input, Button, Segmented } from 'antd';
import { getMessage } from '@/lib/message.js';
import { detectCCSwitchDB, getAppVersion } from '@/api/client.js';
import SettingField from '@/components/common/SettingField.jsx';

/**
 * 通用档：自动同步间隔（即存）、CC-Switch 路径（草稿 + 检测 + 保存）、主题三态（即存）。
 * 受控展示：草稿由 SettingsPage 持有。
 */
export default function SettingsGeneral({ cfg, setCfg, persist, pref, setPref, localDevice, nameDrafts, setNameDrafts, savingDeviceId, onSaveDeviceName }) {
  const [detecting, setDetecting] = useState(false);
  const [savingAutoSync, setSavingAutoSync] = useState(false);
  const [savingPath, setSavingPath] = useState(false);
  const [version, setVersion] = useState('');

  const toast = getMessage();

  // 应用版本（关于信息）
  useEffect(() => {
    getAppVersion().then(setVersion).catch(() => {});
  }, []);

  // 自动同步间隔：选择即保存
  const saveAutoSync = useCallback(async (v) => {
    setSavingAutoSync(true);
    await persist({ autoSyncSeconds: Number(v) });
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
      const path = await detectCCSwitchDB();
      if (path) {
        setCfg(c => ({ ...c, ccSwitchDBPath: path }));
        toast?.success('已自动检测到数据库路径，点击"保存"生效');
      } else {
        toast?.info('未找到默认路径，请手动填写（默认 ~/.cc-switch/cc-switch.db）');
      }
    } catch (err) {
      console.error('[settings] detect db failed', err);
      toast?.error('检测失败，请检查路径或网络');
    } finally {
      setDetecting(false);
    }
  }, [setCfg, toast]);

  return (
    <div className="space-y-6 w-full">
      {localDevice && (
        <SettingField title="本机名" description="设置本机在看板中的展示名称，点击保存后生效。">
          <div className="flex flex-wrap items-center gap-2">
            <Input
              value={nameDrafts[localDevice.deviceId] ?? ''}
              onChange={e => setNameDrafts(dr => ({ ...dr, [localDevice.deviceId]: e.target.value }))}
              placeholder={localDevice.hostname || '设备名'}
              className="flex-1 min-w-0"
            />
            <Button className="shrink-0" onClick={() => onSaveDeviceName(localDevice.deviceId)} disabled={savingDeviceId === localDevice.deviceId}>
              {savingDeviceId === localDevice.deviceId ? '保存中…' : '保存'}
            </Button>
          </div>
        </SettingField>
      )}

      <SettingField title="自动同步配置" description="数据自动采集的间隔，立即生效。">
        <Select
          value={String(cfg.autoSyncSeconds)}
          onChange={saveAutoSync}
          disabled={savingAutoSync}
          style={{ width: '100%' }}
          options={[
            { value: '10', label: '10 秒' },
            { value: '30', label: '30 秒' },
            { value: '60', label: '1 分钟' },
            { value: '300', label: '5 分钟' },
            { value: '0', label: '不同步' },
          ]}
        />
      </SettingField>

      <SettingField title="CC-Switch 数据源路径" description="指向 cc-switch.db，用于导入代理日志，点击保存后生效。">
        <div className="flex flex-wrap gap-2">
          <Input
            value={cfg.ccSwitchDBPath}
            onChange={e => setCfg(c => ({ ...c, ccSwitchDBPath: e.target.value }))}
            placeholder="例: C:\Users\用户名\.cc-switch\cc-switch.db"
            className="flex-1 min-w-0 max-w-lg"
          />
          <Button className="shrink-0" onClick={detectDB} disabled={detecting}>
            {detecting ? '检测中…' : '检测'}
          </Button>
          <Button type="primary" className="shrink-0" onClick={savePath} disabled={savingPath}>
            {savingPath ? '保存中…' : '保存'}
          </Button>
        </div>
      </SettingField>

      <SettingField title="主题配置" description="选择应用的外观主题，立即生效。">
        <Segmented
          value={pref}
          onChange={setPref}
          options={[
            { label: '浅色', value: 'light' },
            { label: '深色', value: 'dark' },
            { label: '跟随系统', value: 'system' },
          ]}
        />
      </SettingField>

      <SettingField title="关于" description="本地优先的 AI Token 消耗看板 —— 离线分析、零上传、多工具聚合。">
        <div className="text-sm">
          版本 <span className="tabular-nums text-foreground/80">{version || '—'}</span>
        </div>
      </SettingField>
    </div>
  );
}
