import { useState, useRef, useCallback } from 'react';
import * as api from '../api/client.js';

const POLL_TIMEOUT = 30000;

/**
 * 采集触发与轮询 Hook。
 * runCollect 启动采集后每 1.5s 轮询状态，直到采集结束或超时。
 * @param {function} [onDataChange] 采集完成后的数据刷新回调
 * @returns {{ collecting: boolean, runCollect: () => void }}
 */
export function useCollection(onDataChange) {
  const [collecting, setCollecting] = useState(false);
  const pollingRef = useRef(null);

  const runCollect = useCallback(() => {
    setCollecting(true);
    api.startCollection();
    if (pollingRef.current) clearInterval(pollingRef.current);
    const startTime = Date.now();
    pollingRef.current = setInterval(() => {
      api.collectStatus().then(s => {
        const elapsed = Date.now() - startTime;
        if (s.status !== 'running' || elapsed > POLL_TIMEOUT) {
          clearInterval(pollingRef.current);
          pollingRef.current = null;
          setCollecting(false);
          if (s.status === 'ok' && onDataChange) onDataChange();
        }
      }).catch(() => {
        clearInterval(pollingRef.current);
        pollingRef.current = null;
        setCollecting(false);
      });
    }, 1500);
  }, [onDataChange]);

  return { collecting, runCollect };
}
