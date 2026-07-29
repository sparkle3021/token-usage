import { useState, useEffect } from 'react';
import { Button } from '@/components/ui/button.jsx';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card.jsx';
import { Skeleton } from '@/components/ui/skeleton.jsx';
import { PencilIcon, Trash2Icon, RefreshCwIcon } from 'lucide-react';
import { getSourceIconUrl } from '../lib/iconMap.js';

/** 格式化倒计时 */
function fmtCountdown(sec) {
  if (sec <= 0) return '重置中…';
  const h = Math.floor(sec / 3600);
  const m = Math.floor((sec % 3600) / 60);
  const s = sec % 60;
  if (h > 0) return `剩 ${h}h${m}m`;
  if (m > 0) return `剩 ${m}m${s}s`;
  return `剩 ${s}s`;
}

/** 格式化时间 */
function fmtTime(ts) {
  if (!ts) return '—';
  const d = new Date(ts);
  const pad = n => String(n).padStart(2, '0');
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}:${pad(d.getSeconds())}`;
}

/** 百分比颜色 */
function pctColor(pct) {
  if (pct < 60) return '#10b981';
  if (pct < 80) return '#f59e0b';
  return '#ef4444';
}

/** SVG 圆环进度条 */
function RingGauge({ pct, size = 80, strokeWidth = 7 }) {
  const radius = (size - strokeWidth) / 2;
  const circumference = 2 * Math.PI * radius;
  const offset = circumference - (Math.min(pct, 100) / 100) * circumference;
  const color = pctColor(pct);

  return (
    <svg width={size} height={size} className="shrink-0">
      <circle cx={size / 2} cy={size / 2} r={radius} fill="none" stroke="currentColor" className="text-muted/20" strokeWidth={strokeWidth} />
      <circle cx={size / 2} cy={size / 2} r={radius} fill="none" stroke={color} strokeWidth={strokeWidth}
        strokeDasharray={circumference} strokeDashoffset={offset}
        strokeLinecap="round" transform={`rotate(-90 ${size / 2} ${size / 2})`}
        style={{ transition: 'stroke-dashoffset 1s ease' }} />
      <text x={size / 2} y={size / 2} textAnchor="middle" dominantBaseline="central"
        className="fill-foreground text-xs font-bold tabular-nums" style={{ fontSize: size * 0.2 }}>{pct}%</text>
    </svg>
  );
}

/** Quota 类型卡片：多个圆环进度条 */
function QuotaSlots({ slots }) {
  return (
    <div className="flex items-center justify-center gap-10 flex-wrap">
      {slots.map((s, i) => (
        <div key={i} className="flex flex-col items-center gap-1.5">
          <RingGauge pct={s.usagePercent} />
          <span className="text-xs text-muted-foreground">{s.label}</span>
          <span className="text-xs text-muted-foreground tabular-nums">{fmtCountdown(s.resetInSec)}</span>
        </div>
      ))}
    </div>
  );
}

/** Skeleton 骨架屏 */
function CardSkeleton({ displayType }) {
  return (
    <Card>
      <CardHeader className="pb-1">
        <div className="flex items-center justify-between">
          <div className="flex items-center gap-1.5">
            <Skeleton className="size-4 rounded" />
            <Skeleton className="h-4 w-28" />
          </div>
          <div className="flex items-center gap-0.5">
            <Skeleton className="size-7 rounded" />
            <Skeleton className="size-7 rounded" />
            <Skeleton className="size-7 rounded" />
          </div>
        </div>
      </CardHeader>
      <CardContent>
        {displayType === 'quota' ? (
          <div className="flex items-center justify-center gap-10">
            {[1, 2, 3].map(i => (
              <div key={i} className="flex flex-col items-center gap-1.5">
                <Skeleton className="size-20 rounded-full" />
                <Skeleton className="h-3 w-16" />
                <Skeleton className="h-3 w-12" />
              </div>
            ))}
          </div>
        ) : (
          <div className="space-y-2 py-2">
            <Skeleton className="h-5 w-40 mx-auto" />
            <Skeleton className="h-4 w-32 mx-auto" />
          </div>
        )}
      </CardContent>
    </Card>
  );
}

export default function QuotaCard({ cfg, data, displayType, slotsLabels, balanceLabel, onEdit, onDelete, onRefresh }) {
  const [localData, setLocalData] = useState(data);
  const loading = !data;

  useEffect(() => { setLocalData(data); }, [data]);

  // 倒计时每秒自减
  useEffect(() => {
    const t = setInterval(() => {
      setLocalData(d => {
        if (!d || !d.slots) return d;
        return {
          ...d,
          slots: d.slots.map(s => ({
            ...s,
            resetInSec: Math.max(0, (s.resetInSec || 0) - 1),
          })),
        };
      });
    }, 1000);
    return () => clearInterval(t);
  }, []);

  const getTitle = () => {
    const alias = cfg.displayName || cfg.seq;
    return `${cfg.provider}-${cfg.plan}：${alias}`;
  };

  if (loading) return <CardSkeleton displayType={displayType} />;

  const isError = localData?.error;

  return (
    <Card className={isError ? 'border-red-200 bg-red-50/50' : ''}>
      <CardHeader className="pb-1">
        <div className="flex items-center justify-between">
          <CardTitle className="text-sm font-medium flex items-center gap-1.5">
            <img src={getSourceIconUrl(cfg.provider)} alt={cfg.provider} className="size-4 shrink-0" />
            {getTitle()}
          </CardTitle>
          <div className="flex items-center gap-0.5">
            <Button size="icon" variant="ghost" className="size-7" onClick={onRefresh}>
              <RefreshCwIcon className="size-3.5" />
            </Button>
            <Button size="icon" variant="ghost" className="size-7" onClick={onEdit}>
              <PencilIcon className="size-3.5" />
            </Button>
            <Button size="icon" variant="ghost" className="size-7 text-red-500" onClick={onDelete}>
              <Trash2Icon className="size-3.5" />
            </Button>
          </div>
        </div>
      </CardHeader>
      <CardContent>
        {isError ? (
          <p className="text-xs text-red-500 text-center py-4">{localData.error}</p>
        ) : displayType === 'quota' ? (
          <QuotaSlots slots={localData?.slots || []} />
        ) : displayType === 'balance' ? (
          <div className="text-sm text-center py-2 space-y-1">
            <p>
              <span className="text-muted-foreground">可用余额：</span>
              <span className="font-bold tabular-nums">
                                {localData && typeof localData.balance === 'number' ? localData.balance.toFixed(2) : '—'}
              </span>
            </p>
            <p className="text-xs text-muted-foreground">
              查询时间：{fmtTime(localData?.fetchedAt)}
            </p>
          </div>
        ) : (
          <p className="text-xs text-muted-foreground text-center py-4">不支持的展示类型</p>
        )}
      </CardContent>
    </Card>
  );
}
