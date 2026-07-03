import React from 'react';
import { Card, CardHeader, CardTitle, CardDescription, CardContent } from '../ui/card.jsx';
import * as U from '../../lib/utils.js';

export default function Gauge({ rate, cacheRead, cacheCreation, prevRate }) {
  const r = Math.max(0, Math.min(100, rate));
  const C = Math.PI * 70;
  const dash = (r / 100) * C;
  const delta = U.deltaPct(rate, prevRate);

  return (
    <Card>
      <CardHeader>
        <div className="flex items-center justify-between">
          <div><CardTitle>缓存命中率</CardTitle><CardDescription>cache_read / total</CardDescription></div>
          {delta != null && (
            <span className={`inline-flex items-center gap-0.5 font-semibold text-xs px-1 rounded ${delta > 0.05 ? 'text-green-600 bg-green-50' : delta < -0.05 ? 'text-red-500 bg-red-50' : 'text-muted-foreground bg-muted'}`}>
              {delta > 0.05 ? '↑' : delta < -0.05 ? '↓' : '·'} {Math.abs(delta).toFixed(1)}%
            </span>
          )}
        </div>
      </CardHeader>
      <CardContent>
        <div className="flex flex-col items-center py-2">
          <svg viewBox="0 0 180 100" className="w-40 max-w-full h-auto">
            <path d="M 10 90 A 80 80 0 0 1 170 90" stroke="oklch(0.93 0.004 80)" strokeWidth="14" fill="none" strokeLinecap="round"/>
            <path d="M 10 90 A 80 80 0 0 1 170 90" stroke="url(#gg)" strokeWidth="14" fill="none" strokeLinecap="round" strokeDasharray={`${dash} ${C}`} style={{ transition: 'stroke-dasharray 0.6s' }} />
            <defs><linearGradient id="gg" x1="0" y1="0" x2="1" y2="0"><stop offset="0%" stopColor="oklch(0.65 0.13 200)"/><stop offset="100%" stopColor="oklch(0.55 0.16 265)"/></linearGradient></defs>
          </svg>
          <div className="-mt-11 text-center">
            <span className="text-2xl font-semibold tabular-nums">{r.toFixed(1)}</span><span className="text-sm text-muted-foreground ml-0.5">%</span>
          </div>
          <div className="flex gap-4 mt-3 text-xs text-muted-foreground">
            <span>读取 <b className="text-foreground">{U.compactCN(cacheRead)}</b></span>
            <span>创建 <b className="text-foreground">{U.compactCN(cacheCreation)}</b></span>
          </div>
        </div>
      </CardContent>
    </Card>
  );
}
