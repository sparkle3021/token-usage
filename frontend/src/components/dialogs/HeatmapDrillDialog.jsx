import { compact, compactCN } from '@/lib/formatters.js';
import { getSourceColor } from '@/lib/iconMap.js';
import { oklchToHex } from '@/lib/oklch.js';
import { useMemo, useRef } from 'react';
import { Modal, Card, Skeleton } from 'antd';
import useECharts, { getChartTheme } from '@/lib/useECharts.js';

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

    const covered = new Set(); // 仅用于 time→hour 优先级，避免双重计数
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
    // 纯日级来源（当天无任何 hour/time 数据）才把日总量兜底到当前小时
    const hourlySources = new Set();
    for (const r of timeRows || []) if (r.usageDate === date) hourlySources.add(r.source);
    for (const r of hourRows || []) if (r.usageDate === date) hourlySources.add(r.source);
    // 纯日级来源兜底：今天归当前小时（进行中）；历史日期（昨天等）已结束归 23 点
    const now = new Date();
    const todayLocal = `${now.getFullYear()}-${String(now.getMonth() + 1).padStart(2, '0')}-${String(now.getDate()).padStart(2, '0')}`;
    const fallbackHour = date === todayLocal ? now.getHours() : 23;
    for (const r of dayDaily) {
      if (hourlySources.has(r.source)) continue;
      add(fallbackHour, r.model, r.totalTokens || 0);
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

  const palette = seriesModels.map(m => oklchToHex(getSourceColor(m.key)));
  const hasHourly = hourlyData.some(pt => seriesModels.some(m => (pt[m.key] || 0) > 0));

  const optionRef = useRef(null);
  const { chartRef, setChartEl, dark, ready } = useECharts(optionRef, [hourlyData, seriesModels, palette, date]);
  const theme = getChartTheme(dark);

  const option = useMemo(() => {
    const xData = hourlyData.map(pt => pt.hour);
    const series = seriesModels.map((m, i) => ({
      name: m.label,
      type: 'bar',
      stack: 'a',
      data: hourlyData.map(pt => pt[m.key] || 0),
      itemStyle: { color: palette[i], borderRadius: [2, 2, 0, 0] },
      barMaxWidth: 16,
    }));
    return {
      grid: { top: 8, right: 8, bottom: 22, left: 48 },
      xAxis: {
        type: 'category',
        data: xData,
        axisLine: { show: false },
        axisTick: { show: false },
        axisLabel: { fontSize: 10, color: theme.axisText, interval: 3 },
      },
      yAxis: {
        type: 'value',
        axisLine: { show: false },
        axisTick: { show: false },
        splitLine: { lineStyle: { color: theme.grid } },
        // 与看板 TrendChart 一致：compact 缩写 + 字号 10.5；弹窗高度小，减少刻度颗粒度
        axisLabel: { fontSize: 10.5, color: theme.axisTick, formatter: (v) => compact(v) },
        splitNumber: 3,
      },
      tooltip: {
        trigger: 'axis',
        backgroundColor: 'var(--popover)',
        borderColor: 'var(--border)',
        borderRadius: 8,
        textStyle: { color: 'var(--popover-foreground)', fontSize: 12 },
        formatter: (params) => {
          const label = params[0]?.axisValue ?? '';
          const rows = params.filter(p => p.seriesName != null).map(p => `${p.marker}${p.seriesName}：${compactCN(p.value)}`).join('<br/>');
          return `<div style="color:var(--popover-foreground);font-size:12px">${date} ${label}<br/>${rows}</div>`;
        },
      },
      series,
    };
  }, [hourlyData, seriesModels, palette, theme, date]);
  optionRef.current = option;

  return (
    <Modal
      open
      onCancel={onClose}
      title={`${date} 用量详情`}
      centered
      width={{ xs: 640, md: 720, lg: 800, xl: 880 }}
      styles={{ body: { maxHeight: 'calc(85vh - 120px)', overflow: 'hidden', display: 'flex', flexDirection: 'column' } }}
      footer={null}
      afterOpenChange={(open) => { if (open) chartRef.current?.resize(); }}
    >
      <div className="text-xs text-muted-foreground mb-2 shrink-0">
        来源数 <strong className="text-foreground">{daySources.length}</strong>
        {topSource && <> · 峰值 <strong className="text-foreground">{topSource.source}</strong>（{compactCN(topSource.tokens)}）</>}
        · 总量 <strong className="text-foreground">{compactCN(dayTotal)}</strong>
      </div>

      <Card className="flex-1 min-h-0" styles={{ body: { padding: 16, height: '100%' } }}>
        {/* 图表 div 自身带尺寸（height:100% 在父 clamp 下可能解析失败→0 高→空白），并始终渲染避免 removeChild 冲突 */}
        <div className="relative" style={{ minHeight: 160, height: 'clamp(160px, 25vh, 250px)' }}>
          <div ref={setChartEl} style={{ minHeight: 160, height: 'clamp(160px, 25vh, 250px)' }} />
          {!ready && (
            <div className="absolute inset-0 overflow-hidden bg-background/50">
              <Skeleton active paragraph={{ rows: 3 }} />
            </div>
          )}
          {!hasHourly && (
            <div className="absolute inset-0 flex items-center justify-center text-muted-foreground text-sm">
              暂无小时级数据
            </div>
          )}
        </div>
      </Card>
    </Modal>
  );
}
