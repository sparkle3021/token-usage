import { Input, Button } from 'antd';
import SettingField from '@/components/common/SettingField.jsx';

/**
 * 设备档：每台设备以三段式呈现（设备标识 + 说明 + 展示名输入与保存按钮）。
 * 受控展示：nameDrafts 草稿与保存动作由 SettingsPage 持有。
 */
export default function SettingsDevices({ devices, nameDrafts, setNameDrafts, savingDeviceId, onSave }) {
  const others = devices.filter(d => !d.isLocal);
  if (others.length === 0) {
    return <p className="text-sm text-muted-foreground">暂无其他设备</p>;
  }

  return (
    <div className="space-y-6 w-full">
      {others.map(d => (
        <SettingField
          key={d.deviceId}
          title={`${d.hostname || '未知设备'}${d.isLocal ? ' · 本机' : ''}`}
          description="自定义看板上的展示名称，点击保存后生效。"
        >
          <div className="flex items-center gap-2">
            <Input
              value={nameDrafts[d.deviceId] ?? ''}
              onChange={e => setNameDrafts(dr => ({ ...dr, [d.deviceId]: e.target.value }))}
              placeholder={d.hostname || '设备名'}
              className="flex-1 min-w-0"
            />
            <Button
              className="shrink-0"
              onClick={() => onSave(d.deviceId)}
              disabled={savingDeviceId === d.deviceId}
            >
              {savingDeviceId === d.deviceId ? '保存中…' : '保存'}
            </Button>
          </div>
        </SettingField>
      ))}
    </div>
  );
}
