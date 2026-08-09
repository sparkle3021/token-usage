/**
 * 模型排行页：按总用量排名的模型卡片列表 + 模型详情视图（当前为纯前端模拟数据）。
 * 点击卡片进入详情，展示请求次数 / Token / 费用；过滤行暂只保留「时间范围」下拉。
 */

import { useEffect, useMemo, useRef, useState } from 'react';
import { Button, Card, Select, Skeleton } from 'antd';
import { ArrowLeftIcon } from 'lucide-react';
import { getModelIconUrl, getSourceColor } from '@/lib/iconMap.js';
import { oklchToHex } from '@/lib/oklch.js';
import { compact, numFmt } from '@/lib/formatters.js';
import KPI from '@/components/common/KPI.jsx';
import useECharts, { getChartTheme } from '@/lib/useECharts.js';

// 单位格式化：保留两位小数（K/M/B）
function fmtUsage(v) {
  if (v == null) return '—';
  const a = Math.abs(v);
  if (a >= 1e9) return (v / 1e9).toFixed(2) + 'B';
  if (a >= 1e6) return (v / 1e6).toFixed(2) + 'M';
  if (a >= 1e3) return (v / 1e3).toFixed(2) + 'K';
  return String(v);
}

// 时间范围筛选项（细节待老大确认）
const RANGE_OPTIONS = [
  { label: '近 7 天', value: 7 },
  { label: '近 30 天', value: 30 },
  { label: '近 90 天', value: 90 },
  { label: '近 1 年', value: 365 },
];

// ── 模拟数据（待后端接口接入后移除） ──────────────────────────
const MOCK_MODELS = [
  { name: 'DeepSeek-V4-Flash', totalTokens: 201_220_000 },
  { name: 'Claude Sonnet 4.6', totalTokens: 158_300_000 },
  { name: 'Gemini 2.5 Pro', totalTokens: 96_450_000 },
  { name: 'GPT-5', totalTokens: 52_120_000 },
  { name: 'Qwen3-Max', totalTokens: 18_760_000 },
  { name: 'Grok 4', totalTokens: 7_320_000 },
  { name: 'Kimi K2', totalTokens: 1_580_000 },
  { name: 'GLM-4.6', totalTokens: 640_000 },
];

// 前三名排名徽章配色：红/蓝/绿三色相分离，明暗主题下均醒目
const RANK_BADGE = [
  'bg-rose-500 text-white',
  'bg-sky-500 text-white',
  'bg-emerald-500 text-white',
];

// 确定性伪随机（seed 固定 → 每次渲染数据稳定）
function mulberry32(seed) {
  return function () {
    seed |= 0; seed = (seed + 0x6D2B79F5) | 0;
    let t = Math.imul(seed ^ (seed >>> 15), 1 | seed);
    t = (t + Math.imul(t ^ (t >>> 7), 61 | t)) ^ t;
    return ((t ^ (t >>> 14)) >>> 0) / 4294967296;
  };
}

function hashSeed(name) {
  let h = 0;
  for (let i = 0; i < name.length; i++) h = (h * 31 + name.charCodeAt(i)) >>> 0;
  return h;
}

// 近 days 天的日期标签（完整日期，x 轴显示时裁成 MM-DD）
function genDates(days) {
  const out = [];
  const now = new Date();
  for (let i = days - 1; i >= 0; i--) {
    const d = new Date(now); d.setDate(d.getDate() - i);
    out.push(`${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`);
  }
  return out;
}

// 生成请求/三种 Token 状态/费用日序列（带周期波动，视觉自然）
function genSeries(days, seed) {
  const rnd = mulberry32(seed);
  const requests = [], inputCache = [], inputNoCache = [], output = [], cost = [];
  const base = 200 + rnd() * 800;
  for (let i = 0; i < days; i++) {
    const wave = Math.sin(i / 3.5) * 0.3;
    const req = Math.max(5, Math.round(base * (1 + wave + (rnd() - 0.5) * 0.4)));
    requests.push(req);
    const inC = Math.round(req * (1500 + rnd() * 5000));
    const inN = Math.round(req * (800 + rnd() * 3000));
    const out = Math.round(req * (1200 + rnd() * 4000));
    inputCache.push(inC);
    inputNoCache.push(inN);
    output.push(out);
    cost.push(+((inC + inN + out) * (0.0000015 + rnd() * 0.000004)).toFixed(4));
  }
  return { requests, inputCache, inputNoCache, output, cost };
}

/** 单模型趋势图卡片（echarts 封装） */
function MiniTrendCard({ title, xData, values, color, valueFmt, areaColor, areaOpacity = 0.08, lineColor }) {
  const optionRef = useRef(null);
  const { setChartEl, dark, ready } = useECharts(optionRef, [xData, values]);
  const theme = getChartTheme(dark);

  const option = useMemo(() => ({
    grid: { top: 8, right: 16, bottom: 22, left: 48 },
    xAxis: {
      type: 'category', data: xData,
      axisLine: { show: false }, axisTick: { show: false },
      axisLabel: { fontSize: 10.5, color: theme.axisText },
    },
    yAxis: {
      type: 'value',
      axisLine: { show: false }, axisTick: { show: false },
      splitLine: { lineStyle: { color: theme.grid } },
      axisLabel: { fontSize: 10.5, color: theme.axisTick, formatter: valueFmt },
      splitNumber: 4,
    },
    tooltip: {
      trigger: 'axis',
      backgroundColor: 'transparent', borderWidth: 0, padding: 0,
      formatter: (params) => {
        const p = params[0];
        return `<div class="bg-popover text-popover-foreground shadow-lg border rounded-lg p-2 text-xs"><span class="font-semibold">${p.axisValue}</span><span class="ml-2 tabular-nums font-semibold">${valueFmt(p.value)}</span></div>`;
      },
    },
    series: [{
      type: 'line', data: values, smooth: true, showSymbol: false,
      lineStyle: { width: 2, color: lineColor || color }, itemStyle: { color: lineColor || color },
      areaStyle: { color: areaColor || color, opacity: areaOpacity },
    }],
  }), [xData, values, theme, color, valueFmt, areaColor, areaOpacity, lineColor]);
  optionRef.current = option;

  return (
    <Card styles={{ body: { padding: 16 } }}>
      <div className="text-sm font-semibold mb-3">{title}</div>
      <div className="relative">
        <div ref={setChartEl} style={{ height: 260 }} />
        {!ready && (
          <div className="absolute inset-0 overflow-hidden bg-background/50">
            <Skeleton active paragraph={{ rows: 4 }} />
          </div>
        )}
      </div>
    </Card>
  );
}

/** Token 堆叠柱状图：输入（命中缓存）/ 输入（未命中缓存）/ 输出 */
const TOKEN_SERIES = [
  { key: 'inputCache', name: '输入（命中缓存）', color: '#aadafa' },
  { key: 'inputNoCache', name: '输入（未命中缓存）', color: '#70b1f8' },
  { key: 'output', name: '输出', color: '#2d6feb' },
];

function TokenBarCard({ xData, inputCache, inputNoCache, output }) {
  const optionRef = useRef(null);
  const { setChartEl, dark, ready } = useECharts(optionRef, [xData, inputCache, inputNoCache, output]);
  const theme = getChartTheme(dark);

  const option = useMemo(() => {
    return {
      grid: { top: 8, right: 16, bottom: 22, left: 48 },
      xAxis: {
        type: 'category', data: xData,
        axisLine: { show: false }, axisTick: { show: false },
        axisLabel: { fontSize: 10.5, color: theme.axisText, formatter: (v) => v.slice(5) },
      },
      yAxis: {
        type: 'value',
        axisLine: { show: false }, axisTick: { show: false },
        splitLine: { lineStyle: { color: theme.grid } },
        axisLabel: { fontSize: 10.5, color: theme.axisTick, formatter: compact },
        splitNumber: 4,
      },
      tooltip: {
        trigger: 'axis',
        backgroundColor: 'transparent', borderWidth: 0, padding: 0,
        formatter: (params) => {
          const date = params[0]?.axisValue ?? '';
          const sum = params.reduce((s, p) => s + (p.value || 0), 0);
          const rows = params.map(p => `
            <div class="flex items-center justify-between gap-6 mt-1">
              <span class="flex items-center gap-1.5">
                <span class="inline-block w-2.5 h-2.5 rounded-sm shrink-0" style="background:${p.color}"></span>
                <span class="text-muted-foreground">${p.seriesName}</span>
              </span>
              <span class="font-semibold tabular-nums">${numFmt.format(p.value || 0)}</span>
            </div>`).join('');
          return `<div class="bg-popover text-popover-foreground shadow-lg border rounded-lg p-2.5 text-xs">
            <div class="flex items-center justify-between gap-6">
              <span class="font-semibold">${date}</span>
              <span class="font-semibold tabular-nums">${numFmt.format(sum)}</span>
            </div>
            ${rows}
          </div>`;
        },
      },
      series: TOKEN_SERIES.map(s => ({
        name: s.name,
        type: 'bar',
        stack: 'total',
        data: s.key === 'inputCache' ? inputCache : s.key === 'inputNoCache' ? inputNoCache : output,
        itemStyle: { color: s.color },
        barMaxWidth: 24,
      })),
    };
  }, [xData, inputCache, inputNoCache, output, theme]);
  optionRef.current = option;

  return (
    <Card styles={{ body: { padding: 16 } }}>
      <div className="text-sm font-semibold mb-3">Tokens</div>
      <div className="relative">
        <div ref={setChartEl} style={{ height: 260 }} />
        {!ready && (
          <div className="absolute inset-0 overflow-hidden bg-background/50">
            <Skeleton active paragraph={{ rows: 4 }} />
          </div>
        )}
      </div>
    </Card>
  );
}

/** 模型详情视图 */
function ModelDetail({ model, onBack }) {
  const [days, setDays] = useState(30);
  const color = useMemo(() => oklchToHex(getSourceColor(model)), [model]);

  const { xData, requests, inputCache, inputNoCache, output, cost } = useMemo(() => {
    const s = genSeries(days, hashSeed(model));
    return { xData: genDates(days), ...s };
  }, [days, model]);

  const totalReq = useMemo(() => requests.reduce((a, b) => a + b, 0), [requests]);
  const totalTok = useMemo(
    () => inputCache.reduce((a, b) => a + b, 0) + inputNoCache.reduce((a, b) => a + b, 0) + output.reduce((a, b) => a + b, 0),
    [inputCache, inputNoCache, output],
  );

  return (
    <div className="flex flex-col gap-4 h-full min-h-0">
      <div className="flex items-center gap-2">
        <Button size="small" className="h-8" icon={<ArrowLeftIcon className="size-4" />} onClick={onBack}>返回</Button>
        <div className="flex items-center gap-2">
          <img src={getModelIconUrl(model)} alt="" className="size-5 shrink-0 brightness-0 dark:brightness-0 dark:invert" />
          <h2 className="text-sm font-semibold">{model}</h2>
        </div>
      </div>

      <div className="flex items-center gap-2">
        <span className="text-xs text-muted-foreground">时间范围</span>
        <Select
          size="small"
          value={days}
          onChange={setDays}
          options={RANGE_OPTIONS}
          style={{ width: 140 }}
        />
      </div>

      <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
        <KPI label="请求次数" value={numFmt.format(totalReq)} />
        <KPI label="Tokens" value={numFmt.format(totalTok)} />
      </div>

      <div className="grid grid-cols-1 lg:grid-cols-2 gap-4">
        <MiniTrendCard title="调用次数" xData={xData} values={requests} color={color} valueFmt={(v) => compact(v)} areaColor="#a1c6f9" areaOpacity={0.4} lineColor="#417ceb" />
        <TokenBarCard xData={xData} inputCache={inputCache} inputNoCache={inputNoCache} output={output} />
      </div>

      <MiniTrendCard title="费用" xData={xData} values={cost} color="oklch(0.72 0.14 75)" valueFmt={(v) => '$' + Number(v).toFixed(2)} />
    </div>
  );
}

/** 排行列表视图 */
function RankList({ onSelect }) {
  const ranked = useMemo(() => [...MOCK_MODELS].sort((a, b) => b.totalTokens - a.totalTokens), []);
  return (
    <div className="flex flex-col gap-4 h-full min-h-0">
      <h2 className="text-sm font-semibold">模型排行</h2>
      <div className="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-3 gap-4">
        {ranked.map((m, i) => (
          <Card
            key={m.name}
            className="relative overflow-hidden cursor-pointer transition-shadow hover:shadow-md"
            styles={{ body: { padding: 16 } }}
            onClick={() => onSelect(m.name)}
          >
            <img
              src={getModelIconUrl(m.name)}
              alt=""
              aria-hidden="true"
              className="pointer-events-none absolute right-3 top-1/2 -translate-y-1/2 h-[88%] w-auto max-w-[55%] object-contain opacity-[0.08] brightness-0 dark:opacity-[0.18] dark:brightness-0 dark:invert"
            />
            <div className="relative z-10 flex flex-col gap-6">
              <div className="flex items-center gap-2">
                <span className={`flex items-center justify-center h-6 min-w-6 px-1.5 rounded-md text-xs font-bold tabular-nums ${RANK_BADGE[i] || 'bg-muted text-muted-foreground'}`}>
                  #{i + 1}
                </span>
                <span className="text-sm font-semibold truncate">{m.name}</span>
              </div>
              <div>
                <div className="text-2xl font-bold tabular-nums leading-none">{fmtUsage(m.totalTokens)}</div>
              </div>
            </div>
          </Card>
        ))}
      </div>
    </div>
  );
}

export default function ModelPage() {
  const [selected, setSelected] = useState(null);
  const rootRef = useRef(null);

  // 视图切换时复位最近的可滚动容器到顶部（详情/列表都从顶部开始）
  useEffect(() => {
    let el = rootRef.current;
    while (el && el !== document.body) {
      const oy = getComputedStyle(el).overflowY;
      if (/auto|scroll/.test(oy) && el.scrollHeight > el.clientHeight) { el.scrollTo(0, 0); break; }
      el = el.parentElement;
    }
  }, [selected]);

  return (
    <div ref={rootRef} className="h-full min-h-0">
      {selected
        ? <ModelDetail model={selected} onBack={() => setSelected(null)} />
        : <RankList onSelect={setSelected} />}
    </div>
  );
}
