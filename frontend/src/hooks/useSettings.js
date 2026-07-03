import { useEffect, useCallback, useState } from 'react';
import * as api from '../api/client.js';
import { EventsOn } from '../../wailsjs/runtime/runtime.js';

/**
 * 设置管理与事件监听 Hook。
 * 自动挂载时加载设置并同步自动同步间隔，监听 collection:done 事件触发数据刷新。
 * @param {function} [onDataChange] collection:done 事件触发时的数据刷新回调
 * @returns {{ autoSync: number, handleSettingsChange: (cfg) => void }}
 */
export function useSettings(onDataChange) {
  const [autoSync, setAutoSync] = useState(5);

  useEffect(() => {
    api.getSettings().then(cfg => {
      setAutoSync(cfg.autoSyncMinutes || 5);
      api.setAutoSyncInterval(cfg.autoSyncMinutes || 5);
    }).catch(() => {
      api.setAutoSyncInterval(5);
    });
  }, []);

  useEffect(() => {
    const cancel = EventsOn('collection:done', () => {
      if (onDataChange) onDataChange();
    });
    return () => cancel();
  }, [onDataChange]);

  const handleSettingsChange = useCallback((cfg) => {
    setAutoSync(cfg.autoSyncMinutes);
  }, []);

  return { autoSync, handleSettingsChange };
}
