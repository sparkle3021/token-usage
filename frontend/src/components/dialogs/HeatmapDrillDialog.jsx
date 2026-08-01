import { compactCN } from '@/lib/formatters.js';
import { getSourceColor } from '@/lib/iconMap.js';
import { useMemo } from 'react';
import { BarChart, Bar, XAxis, YAxis, CartesianGrid, Tooltip, ResponsiveContainer } from 'recharts';
import { Modal, Card } from 'antd';

const OTHER_KEY = '__other__';
const TOP_N = 6;

export default function HeatmapDrillDialog({ date, daily, timeRows, hourRows, onClose }) {
  const dayDaily = useMemo(() => {
    if (!daily || !date) return [];
    return daily.filter(r => r.usageDate === date);
  }, [daily, date]);

  // ── 头部：来源维度 + 当日总量 ──
  const daySources = useMemo(() => [...new Set(dayDaily.map(r => r.source))], [dayDaily]);

  const topSource = useMemo(() => {
    const m = new Map();
    for (const r of dayDaily) m.set(r.source, (m.get(r.source) || 0) + (r.totalTokens || 0));
    let best = null;
    for (const [source, tokens] of m) {
      if (!best || tokens > best.tokens) best = { source, tokens };
    }
    return best;
  }, [dayDaily]);

  const dayTotal = useMemo(() => dayDaily.reduce((s, r) => s + (r.totalTokens || 0), 0), [dayDaily]);

  // ── 图表：模型维度 + top-N 归并 ──
  const seriesModels = useMemo(() => {
    const m = new Map();
    for (const r of dayDaily) {
      const model = r.model || '未知';
      m.set(model, (m.get(model) || 0) + (r.totalTokens || 0));
    }
    const sorted = [...m.entries()].sort((a, b) => b[1] - a[1]);
    if (sorted.length === 0) return [];
    if (sorted.length <= TOP_N) return sorted.map(([model]) => ({ key: model, label: model }));
    return [
      ...sorted.slice(0, TOP_N).map(([model]) => ({ key: model, label: model })),
      { key: OTHER_KEY, label: '其他' },
    ];
  }, [dayDaily]);

  const hourlyData = useMemo(() => {
    if (!date) return [];
    const hourModelMap = new Map();
    const add = (h, model, tokens) => {
      let mm = hourModelMap.get(h);
      if (!mm) { mm = new Map(); hourModelMap.set(h, mm); }
      mm.set(model, (mm.get(model) || 0) + tokens);
    };

    const covered = new Set();
    for (const r of timeRows || []) {
      if (r.usageDate !== date) continue;
      covered.add(r.source);
      const d = new Date(r.eventTime);
      if (isNaN(d.getTime())) continue;
      add(d.getHours(), r.model, r.totalTokens || 0);
    }
    for (const r of hourRows || []) {
      if (r.usageDate !== date || covered.has(r.source)) continue;
      add(r.hour, r.model, r.totalTokens || 0);
    }
    const currentHour = new Date().getHours();
    for (const r of dayDaily) {
      if (covered.has(r.source)) continue;
      add(currentHour, r.model, r.totalTokens || 0);
    }

    const topSet = new Set(seriesModels.filter(m => m.key !== OTHER_KEY).map(m => m.key));
    const hasOther = seriesModels.some(m => m.key === OTHER_KEY);

    return Array.from({ length: 24 }, (_, h) => {
      const hourStr = String(h).padStart(2, '0');
      const pt = { hour: `${hourStr}:00` };
      let other = 0;
      for (const [model, tokens] of hourModelMap.get(h) || []) {
        if (topSet.has(model)) pt[model] = (pt[model] || 0) + tokens;
        else other += tokens;
      }
      if (hasOther && other > 0) pt[OTHER_KEY] = other;
      return pt;
    });
  }, [timeRows, hourRows, date, seriesModels, dayDaily]);

  const palette = seriesModels.map(m => getSourceColor(m.key));
  const hasHourly = hourlyData.some(pt => seriesModels.some(m => (pt[m.key] || 0) > 0));

  return (
    <Modal
      open
      onCancel={onClose}
      title={`${date} 用量详情`}
      centered
      width={{ xs: 640, md: 720, lg: 800, xl: 880 }}
      styles={{ body: { maxHeight: 'calc(85vh - 120px)', overflow: 'hidden', display: 'flex', flexDirection: 'column' } }}
      footer={null}
    >
      <div className="text-xs text-muted-foreground mb-2 shrink-0">
        来源数 <strong className="text-foreground">{daySources.length}</strong>
        {topSource && <> · 峰值 <strong className="text-foreground">{topSource.source}</strong>（{compactCN(topSource.tokens)}）</>}
        · 总量 <strong className="text-foreground">{compactCN(dayTotal)}</strong>
      </div>

      <Card className="flex-1 min-h-0" styles={{ body: { padding: 16, height: '100%' } }}>
        <h4 className="text-sm font-medium mb-3">Token 使用趋势（24 小时）</h4>
        {hasHourly ? (
          <div style={{ minHeight: 160, height: 'clamp(160px, 25vh, 250px)' }}>
            <ResponsiveContainer width="100%" height="100%">
              <BarChart data={hourlyData} margin={{ top: 4, right: 4, bottom: 0, left: -12 }}>
                <CartesianGrid strokeDasharray="3 3" stroke="var(--color-border)" />
                <XAxis dataKey="hour" tick={{ fontSize: 10 }} interval={3} />
                <YAxis tick={{ fontSize: 10 }} tickFormatter={v => compactCN(v)} width={50} />
                <Tooltip
                  formatter={(v, name) => [compactCN(v), name]}
                  labelFormatter={label => `${date} ${label}`}
                  contentStyle={{
                    background: 'var(--popover)',
                    color: 'var(--popover-foreground)',
                    border: '1px solid var(--border)',
                    borderRadius: 8,
                    fontSize: 12,
                  }}
                />
                {seriesModels.map((m, i) => (
                  <Bar key={m.key} name={m.label} dataKey={m.key} stackId="a" fill={palette[i % palette.length]} radius={[2, 2, 0, 0]} />
                ))}
              </BarChart>
            </ResponsiveContainer>
          </div>
        ) : (
          <div className="flex items-center justify-center h-32 text-muted-foreground text-sm">
            暂无小时级数据
          </div>
        )}
      </Card>
    </Modal>
  );
}
