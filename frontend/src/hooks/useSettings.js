import { useEffect, useCallback, useState } from 'react';
import * as api from '@/api/client.js';
import { EventsOn } from '../../wailsjs/runtime/runtime.js';

/**
 * 设置管理与事件监听 Hook。
 * 自动挂载时加载设置并同步自动同步间隔，监听 collection:done 事件触发数据刷新。
 * @param {function} [onDataChange] collection:done 事件触发时的数据刷新回调
 * @returns {{ autoSync: number, handleSettingsChange: (cfg) => void }}
 */
export function useSettings(onDataChange) {
  const [autoSync, setAutoSync] = useState(30);

  useEffect(() => {
    api.getSettings().then(cfg => {
      setAutoSync(cfg.autoSyncSeconds || 30);
      api.setAutoSyncInterval(cfg.autoSyncSeconds || 30);
    }).catch(() => {
      api.setAutoSyncInterval(30);
    });
  }, []);

  useEffect(() => {
    const cancel = EventsOn('collection:done', () => {
      if (onDataChange) onDataChange();
    });
    return () => cancel();
  }, [onDataChange]);

  const handleSettingsChange = useCallback((cfg) => {
    setAutoSync(cfg.autoSyncSeconds);
  }, []);

  return { autoSync, handleSettingsChange };
}
