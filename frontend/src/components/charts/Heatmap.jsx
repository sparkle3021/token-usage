import React, { useMemo } from 'react';
import { Card, CardHeader, CardTitle, CardDescription, CardContent } from '../ui/card.jsx';
import * as U from '../../lib/utils.js';

export default function Heatmap({ rows, dates }) {
  const byDate = useMemo(() => {
    const m = new Map();
    for (const r of rows) m.set(r.usageDate, (m.get(r.usageDate) || 0) + (r.totalTokens || 0));
    return m;
  }, [rows]);

  const HOURLY = [0.005,0.003,0.002,0.001,0.001,0.003,0.008,0.025,0.045,0.075,0.092,0.082,0.055,0.078,0.092,0.088,0.080,0.060,0.045,0.038,0.045,0.040,0.025,0.012];
  const hsum = HOURLY.reduce((a, b) => a + b, 0);
  const hourly = HOURLY.map(v => v / hsum);

  const showDates = dates.slice(-28);
  const matrix = useMemo(() => showDates.map(d => {
    const total = byDate.get(d) || 0;
    return hourly.map(h => Math.round(total * h));
  }), [showDates, byDate]);

  const flat = matrix.flat();
  const max = Math.max(...flat, 1);
  const heatColor = (v) => {
    const t = Math.pow(v / max, 0.6);
    if (t < 0.02) return 'oklch(0.97 0.003 80)';
    return `oklch(${0.94 - t * 0.5} ${0.02 + t * 0.16} 265)`;
  };
  const hours = ['', '', '2', '', '4', '', '6', '', '8', '', '10', '', '12', '', '14', '', '16', '', '18', '', '20', '', '22', ''];

  return (
    <Card>
      <CardHeader>
        <div className="flex items-center justify-between">
          <div><CardTitle>使用热力图</CardTitle><CardDescription>最近 {showDates.length} 天 × 24 小时</CardDescription></div>
          <span className="flex items-center gap-1.5 text-xs text-muted-foreground">
            少<span className="inline-flex h-2 w-24 rounded overflow-hidden">{Array.from({length: 8}, (_, i) => <span key={i} style={{flex: 1, background: heatColor((i/7)*max)}} />)}</span>多
          </span>
        </div>
      </CardHeader>
      <CardContent>
        <div className="w-full overflow-x-auto">
          <div className="grid gap-0.5" style={{ gridTemplateColumns: `45px repeat(24, 1fr)`, minWidth: 580 }}>
            <div />{hours.map((h, i) => <div key={i} className="text-[10px] text-center text-muted-foreground">{h}</div>)}
            {showDates.map((d, di) => (
              <React.Fragment key={d}>
                <div className="text-[10px] text-right text-muted-foreground pr-1 leading-[18px]">{di % 3 === 0 ? d.slice(5) : ''}</div>
                {matrix[di].map((v, hi) => (
                  <div key={hi} className="h-[18px] rounded-sm cursor-pointer hover:outline hover:outline-1 hover:outline-foreground"
                    style={{ background: heatColor(v) }}
                    title={`${d} ${String(hi).padStart(2,'0')}:00 · ${U.compactCN(v)} tokens`} />
                ))}
              </React.Fragment>
            ))}
          </div>
        </div>
      </CardContent>
    </Card>
  );
}
