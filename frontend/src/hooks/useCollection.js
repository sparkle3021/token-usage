import { useState, useRef, useCallback } from 'react';
import * as api from '../api/client.js';

const POLL_TIMEOUT = 30000;
const FULL_POLL_TIMEOUT = 60000;

/**
 * 采集触发与轮询 Hook。
 * runCollect 启动增量采集，runFullCollect 启动全量采集，之后每 1.5s 轮询状态，
 * 直到采集结束或超时。
 * @param {function} [onDataChange] 采集完成后的数据刷新回调
 * @returns {{ collecting: boolean, runCollect: () => void, runFullCollect: () => void }}
 */
export function useCollection(onDataChange) {
  const [collecting, setCollecting] = useState(false);
  const pollingRef = useRef(null);

  const stopPolling = useCallback(() => {
    if (pollingRef.current) {
      clearInterval(pollingRef.current);
      pollingRef.current = null;
    }
  }, []);

  const poll = useCallback((timeout) => {
    const startTime = Date.now();
    pollingRef.current = setInterval(() => {
      api.collectStatus().then(s => {
        const elapsed = Date.now() - startTime;
        if (s.status !== 'running' || elapsed > timeout) {
          stopPolling();
          setCollecting(false);
          if (s.status === 'ok' && onDataChange) onDataChange();
        }
      }).catch(() => {
        stopPolling();
        setCollecting(false);
      });
    }, 1500);
  }, [stopPolling, onDataChange]);

  const start = useCallback((fn, timeout) => {
    setCollecting(true);
    fn();
    stopPolling();
    poll(timeout);
  }, [stopPolling, poll]);

  const runCollect = useCallback(() => {
    start(() => api.startCollection(), POLL_TIMEOUT);
  }, [start]);

  const runFullCollect = useCallback(() => {
    start(() => api.startFullCollection(), FULL_POLL_TIMEOUT);
  }, [start]);

  return { collecting, runCollect, runFullCollect };
}
