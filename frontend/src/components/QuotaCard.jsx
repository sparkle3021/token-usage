import { useState, useEffect } from 'react';
import { Button, Card, Skeleton } from 'antd';
import { PencilIcon, Trash2Icon, RefreshCwIcon } from 'lucide-react';
import { getSourceIconUrl } from '../lib/iconMap.js';

const PROVIDER_NAMES = {
  opencode: 'OpenCode',
  deepseek: 'DeepSeek',
  bigmodel: 'BigModel',
};

/** 格式化倒计时：-1 表示待使用，0 表示重置中，>0 正常倒计时 */
function fmtCountdown(sec) {
  if (sec === -1) return '待使用';
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
    <Card styles={{ body: { padding: 16 } }}>
      <div className="flex items-center justify-between mb-3">
        <Skeleton active title={{ width: 120 }} paragraph={false} />
        <div className="flex items-center gap-1">
          <Skeleton.Button active size="small" shape="circle" />
          <Skeleton.Button active size="small" shape="circle" />
          <Skeleton.Button active size="small" shape="circle" />
        </div>
      </div>
      {displayType === 'quota' ? (
        <div className="flex items-center justify-center gap-10">
          {[1, 2, 3].map(i => (
            <div key={i} className="flex flex-col items-center gap-1.5">
              <Skeleton.Avatar active size={80} shape="circle" />
              <Skeleton active title={false} paragraph={{ rows: 2, width: ['80%', '60%'] }} />
            </div>
          ))}
        </div>
      ) : (
        <Skeleton active title={false} paragraph={{ rows: 2 }} />
      )}
    </Card>
  );
}

export default function QuotaCard({ cfg, data, displayType, slotsLabels, balanceLabel, onEdit, onDelete, onRefresh }) {
  const [localData, setLocalData] = useState(data);
  const loading = !data;

  useEffect(() => { setLocalData(data); }, [data]);

  // 倒计时每秒自减（-1 表示待使用，不递减）
  useEffect(() => {
    const t = setInterval(() => {
      setLocalData(d => {
        if (!d || !d.slots) return d;
        return {
          ...d,
          slots: d.slots.map(s => ({
            ...s,
            resetInSec: s.resetInSec > 0 ? s.resetInSec - 1 : s.resetInSec,
          })),
        };
      });
    }, 1000);
    return () => clearInterval(t);
  }, []);

  const getTitle = () => {
    const alias = cfg.displayName || cfg.seq;
    const provider = PROVIDER_NAMES[cfg.provider] || cfg.provider;
    return `${provider} ${cfg.plan} - ${alias}`;
  };

  if (loading) return <CardSkeleton displayType={displayType} />;

  // Token 已过期
  if (cfg.isValid === false) {
    return (
      <Card className="border-red-200 bg-red-50/50" styles={{ body: { padding: 16 } }}>
        <div className="flex items-center justify-between mb-2">
          <div className="text-sm font-medium flex items-center gap-1.5">
            <img src={getSourceIconUrl(cfg.provider)} alt={cfg.provider} className="size-4 shrink-0" />
            {getTitle()}
          </div>
          <div className="flex items-center gap-0.5">
            <Button type="text" size="small" className="size-7" icon={<PencilIcon className="size-3.5" />} onClick={onEdit} />
            <Button type="text" size="small" className="size-7 text-red-500" icon={<Trash2Icon className="size-3.5" />} onClick={onDelete} />
          </div>
        </div>
        <div className="text-center py-4">
          <p className="text-xs text-red-500 mb-2">Token 已过期，请更新</p>
          <Button size="small" className="h-7 text-xs" onClick={onEdit}>更新配置</Button>
        </div>
      </Card>
    );
  }

  const isError = localData?.error;

  return (
    <Card className={isError ? 'border-red-200 bg-red-50/50' : ''} styles={{ body: { padding: 16 } }}>
      <div className="flex items-center justify-between mb-3">
        <div className="text-sm font-medium flex items-center gap-1.5">
          <img src={getSourceIconUrl(cfg.provider)} alt={cfg.provider} className="size-4 shrink-0" />
          {getTitle()}
        </div>
        <div className="flex items-center gap-0.5">
          <Button type="text" size="small" className="size-7" icon={<RefreshCwIcon className="size-3.5" />} onClick={onRefresh} />
          <Button type="text" size="small" className="size-7" icon={<PencilIcon className="size-3.5" />} onClick={onEdit} />
          <Button type="text" size="small" className="size-7 text-red-500" icon={<Trash2Icon className="size-3.5" />} onClick={onDelete} />
        </div>
      </div>
      <div>
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
                {localData?.balanceDetails?.[0]?.currency ? ` ${localData.balanceDetails[0].currency}` : ''}
              </span>
            </p>
            {localData?.balanceDetails?.[0] && (
              <div className="text-xs tabular-nums space-y-0.5">
                <p className="flex items-center justify-center gap-2">
                  <span className="w-16 text-right text-muted-foreground">充值余额：</span>
                  <span className="w-16 text-left font-medium text-foreground">{(localData.balanceDetails[0].toppedUp ?? 0).toFixed(2)}</span>
                </p>
                <p className="flex items-center justify-center gap-2">
                  <span className="w-16 text-right text-muted-foreground">赠金余额：</span>
                  <span className="w-16 text-left font-medium text-foreground">{(localData.balanceDetails[0].granted ?? 0).toFixed(2)}</span>
                </p>
              </div>
            )}
            <p className="text-xs text-muted-foreground">
              查询时间：{fmtTime(localData?.fetchedAt)}
            </p>
          </div>
        ) : (
          <p className="text-xs text-muted-foreground text-center py-4">不支持的展示类型</p>
        )}
      </div>
    </Card>
  );
}
