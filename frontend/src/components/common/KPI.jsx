/**
 * KPI 卡片组件，展示指标数值、环比变化和迷你趋势图（SparkLine）。
 */

import { Card } from 'antd';

/** 环比变化标，绿色 ↑（增长）、红色 ↓（下降）、灰色 ·（持平） */
function DeltaBadge({ value }) {
  return (
    <span className={`inline-flex items-center gap-0.5 font-semibold text-[11px] px-1 rounded ${value > 0.05 ? 'text-green-700 bg-green-100 dark:text-green-400 dark:bg-green-950/60' : value < -0.05 ? 'text-red-700 bg-red-100 dark:text-red-400 dark:bg-red-950/60' : 'text-muted-foreground bg-muted'}`}>
      {`${value > 0.05 ? '↑' : value < -0.05 ? '↓' : '·'} ${Math.abs(value).toFixed(1)}%`}
    </span>
  );
}

/** 迷你趋势图 SVG，自适应宽度，20% 透明填充 */
function SparkLine({ values, color }) {
  const w = 100, h = 28;
  const max = Math.max(...values, 1);
  const pts = values.map((v, i) => [(i / (values.length - 1 || 1)) * w, h - (v / max) * (h - 2) - 1]);
  const d = pts.map((p, i) => `${i === 0 ? 'M' : 'L'}${p[0]},${p[1]}`).join(' ');
  return (
    <svg className="absolute right-0 bottom-0 w-full pointer-events-none opacity-20" viewBox={`0 0 ${w} ${h}`} preserveAspectRatio="none" style={{ height: 28 }}>
      <path d={d + ` L${w},${h} L0,${h} Z`} fill={color} opacity="0.12" />
      <path d={d} fill="none" stroke={color} strokeWidth="1" />
    </svg>
  );
}

/**
 * @param {{ label: string, value: string, sub?: string, delta?: number, subDelta?: number, spark?: number[], color?: string }} props
 */
export default function KPI({ label, value, sub, delta, subDelta, spark, color }) {
  return (
    <Card className="relative overflow-hidden" styles={{ body: { padding: 16 } }}>
      <div className="text-[11px] text-muted-foreground font-medium mb-1.5">{label}</div>
        {sub ? (
          <>
            <div className="flex items-baseline gap-3">
              <div className="text-xl font-semibold tabular-nums leading-tight">{value}</div>
              {delta != null && <DeltaBadge value={delta} />}
            </div>
            <div className="flex items-center gap-3 mt-1.5 text-xs text-muted-foreground">
              <span className="truncate">{sub}</span>
              {subDelta != null && <DeltaBadge value={subDelta} />}
            </div>
          </>
        ) : (
          <>
            <div className="text-xl font-semibold tabular-nums leading-tight">{value}</div>
            <div className="flex items-center gap-1.5 mt-2 text-xs text-muted-foreground">
              {delta != null && <DeltaBadge value={delta} />}
            </div>
          </>
        )}
        {spark && spark.length > 1 && <SparkLine values={spark} color={color} />}
    </Card>
  );
}
