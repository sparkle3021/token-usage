import { getDevices, getSettings, renameDevice, saveSettings } from '@/api/client.js';
import SettingsGeneral from '@/components/settings/SettingsGeneral.jsx';
import SettingsMaintenance from '@/components/settings/SettingsMaintenance.jsx';
import SettingsPricing from '@/components/settings/SettingsPricing.jsx';
import { getMessage } from '@/lib/message.js';
import { Button, Segmented } from 'antd';
import { ArrowLeftIcon } from 'lucide-react';
import { useCallback, useEffect, useState } from 'react';

const DEFAULTS = { autoSyncSeconds: 30, ccSwitchDBPath: '', ccSwitchEnabled: false, ccSwitchAutoSync: false };

const TABS = [
  { label: '通用', value: 'general' },
  { label: '价格', value: 'pricing' },
  { label: '维护', value: 'maintenance' },
];

/**
 * 设置页面：顶部「返回 + 标题」导航 + Segmented 分区切换 + 内容区。
 * 草稿状态（CC-Switch 路径、设备展示名）全部持有于此，分区组件受控化，切档不丢草稿。
 */
export default function SettingsPage({ onBack, pref, setPref, handleSettingsChange, onClear, onFullSync, fullSyncing }) {
  const [tab, setTab] = useState('general');
  const [cfg, setCfg] = useState(null);
  const [loadErr, setLoadErr] = useState(null);
  const [devices, setDevices] = useState([]);
  const [nameDrafts, setNameDrafts] = useState({});
  const [savingDeviceId, setSavingDeviceId] = useState(null);

  const toast = getMessage();

  // 挂载时加载设置与设备（页面进入即拉取，替换原弹窗的 open 触发）
  useEffect(() => {
    setLoadErr(null);
    getSettings().then(setCfg).catch(err => {
      setLoadErr(String(err));
      setCfg({ ...DEFAULTS });
    });
    getDevices().then(list => {
      setDevices(list || []);
      const drafts = {};
      for (const d of list || []) drafts[d.deviceId] = d.displayName || d.hostname || '';
      setNameDrafts(drafts);
    }).catch(() => {});
  }, []);

  // 保存配置（指定改动的字段），立即持久化到 app_config 并应用运行时。
  const persist = useCallback(async (patch) => {
    const next = { ...cfg, ...patch };
    setCfg(next);
    try {
      await saveSettings(next);
      if (handleSettingsChange) handleSettingsChange(next);
      return true;
    } catch (err) {
      console.error('[settings] save failed', err);
      toast?.error('保存失败，请重试');
      return false;
    }
  }, [cfg, handleSettingsChange, toast]);

  // 保存设备展示名（仅改 devices.display_name，不触碰用量数据）
  const saveDeviceName = useCallback(async (deviceId) => {
    const name = (nameDrafts[deviceId] || '').trim();
    if (!name) {
      toast?.warning('设备名不能为空');
      return;
    }
    setSavingDeviceId(deviceId);
    try {
      await renameDevice(deviceId, name);
      setDevices(ds => ds.map(d => d.deviceId === deviceId ? { ...d, displayName: name } : d));
      toast?.success('设备名已更新');
    } catch (err) {
      console.error('[settings] rename device failed', err);
      toast?.error('重命名失败，请重试');
    } finally {
      setSavingDeviceId(null);
    }
  }, [nameDrafts, toast]);

  return (
    <div className="flex flex-col h-full min-h-0">
      {/* 顶部导航：返回 + 标题 */}
      <div className="flex items-center gap-3">
        <Button className="h-8 w-8 p-0" icon={<ArrowLeftIcon className="size-4" />} onClick={onBack} />
        <h1 className="text-base font-semibold">设置</h1>
      </div>

      {/* 设置类型过滤：宽度拉满 */}
      <div className="py-4 w-full">
        <Segmented
          block
          value={tab}
          onChange={setTab}
          options={TABS}
          styles={{ root: { padding: 4 }, item: { marginInline: 4, paddingBlock: 2 } }}
        />
      </div>

      {/* 内容区（滚动承载于此） */}
      <div className="flex-1 min-h-0 overflow-y-auto scrollbar-none pb-4">
        {!cfg ? (
          <div className="py-8 text-center text-sm text-muted-foreground">
            {loadErr ? <p className="text-red-500">加载失败：{loadErr}</p> : '加载中…'}
          </div>
        ) : (
          <>
            {tab === 'general' && (
              <SettingsGeneral
                cfg={cfg}
                setCfg={setCfg}
                persist={persist}
                pref={pref}
                setPref={setPref}
                localDevice={devices.find(d => d.isLocal)}
                nameDrafts={nameDrafts}
                setNameDrafts={setNameDrafts}
                savingDeviceId={savingDeviceId}
                onSaveDeviceName={saveDeviceName}
              />
            )}
            {tab === 'pricing' && (
              <SettingsPricing />
            )}
            {tab === 'maintenance' && (
              <SettingsMaintenance onClear={onClear} onFullSync={onFullSync} fullSyncing={fullSyncing} />
            )}
          </>
        )}
      </div>
    </div>
  );
}
