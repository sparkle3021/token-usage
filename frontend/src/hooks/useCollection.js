import { useState, useRef, useCallback } from 'react';
import * as api from '../api/client.js';

/**
 * 采集触发与轮询 Hook。
 * runCollect 启动采集后每 1.5s 轮询状态，直到采集结束。
 * @param {function} [onDataChange] 采集完成后的数据刷新回调（非必需，事件监听也会触发刷新）
 * @returns {{ collecting: boolean, runCollect: () => void }}
 */
export function useCollection(onDataChange) {
  const [collecting, setCollecting] = useState(false);
  const pollingRef = useRef(null);

  const runCollect = useCallback(() => {
    setCollecting(true);
    api.startCollection();
    if (pollingRef.current) clearInterval(pollingRef.current);
    pollingRef.current = setInterval(() => {
      api.collectStatus().then(s => {
        if (s.status !== 'running') {
          clearInterval(pollingRef.current);
          pollingRef.current = null;
          setCollecting(false);
          if (s.status === 'ok' && onDataChange) onDataChange();
        }
      }).catch(() => {});
    }, 1500);
  }, [onDataChange]);

  return { collecting, runCollect };
}
