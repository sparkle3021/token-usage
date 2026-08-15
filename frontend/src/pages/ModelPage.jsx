/**
 * 模型排行页：按总用量排名的模型卡片列表 + 模型详情视图（后端真实数据）。
 * 点击卡片进入详情，展示请求次数 / Token / 费用；过滤行暂只保留「时间范围」下拉。
 */

import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { Button, Card, DatePicker, Select, Skeleton } from 'antd';
import { ArrowLeftIcon, RefreshCwIcon } from 'lucide-react';
import { getModelIconUrl, getSourceColor } from '@/lib/iconMap.js';
import { oklchToHex } from '@/lib/oklch.js';
import { compact, daysAgo, numFmt, rangeDates } from '@/lib/formatters.js';
import * as api from '@/api/client.js';
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

// 时间维度筛选项
const RANGE_OPTIONS = [
  { label: '今天', value: 'today' },
  { label: '昨天', value: 'yesterday' },
  { label: '近 7 天', value: 'last7' },
  { label: '近 30 天', value: 'last30' },
  { label: '近 90 天', value: 'last90' },
  { label: '自定义跨度', value: 'custom' },
  { label: '全部', value: 'all' },
];

// 解析时间维度 → 小时级（今天/昨天：{ isHourly, localDate }）或日级（{ isHourly, startDate, endDate }）
function resolveRange(id, custom) {
  switch (id) {
    case 'today': return { isHourly: true, localDate: daysAgo(0) };
    case 'yesterday': return { isHourly: true, localDate: daysAgo(1) };
    case 'last7': return { isHourly: false, startDate: daysAgo(6), endDate: daysAgo(0) };
    case 'last30': return { isHourly: false, startDate: daysAgo(29), endDate: daysAgo(0) };
    case 'last90': return { isHourly: false, startDate: daysAgo(89), endDate: daysAgo(0) };
    case 'all': return { isHourly: false, startDate: daysAgo(179), endDate: daysAgo(0) };
    case 'custom': {
      if (custom?.[0] && custom?.[1]) {
        return { isHourly: false, startDate: custom[0].format('YYYY-MM-DD'), endDate: custom[1].format('YYYY-MM-DD') };
      }
      return { isHourly: false, startDate: daysAgo(6), endDate: daysAgo(0) };
    }
    default: return { isHourly: false, startDate: daysAgo(6), endDate: daysAgo(0) };
  }
}

// 前三名排名徽章配色：红/蓝/绿三色相分离，明暗主题下均醒目
const RANK_BADGE = [
  'bg-rose-500 text-white',
  'bg-sky-500 text-white',
  'bg-emerald-500 text-white',
];

// 日级聚合：从 daily 按模型 + 日期区间聚合请求数/Token 三态/费用，缺失日期补 0
function buildDailySeries(daily, model, startDate, endDate) {
  const dates = rangeDates(startDate, endDate);
  const map = new Map();
  for (const r of daily) {
    if (r.model !== model) continue;
    const e = map.get(r.usageDate) || { requests: 0, inputCache: 0, inputNoCache: 0, output: 0, cost: 0 };
    e.requests += r.requestCount || 0;
    e.inputCache += r.cacheReadTokens || 0;
    e.inputNoCache += r.inputTokens || 0;
    e.output += r.outputTokens || 0;
    e.cost += r.costUSD || 0;
    map.set(r.usageDate, e);
  }
  const xData = dates;
  const requests = [], inputCache = [], inputNoCache = [], output = [], cost = [];
  for (const d of dates) {
    const e = map.get(d);
    requests.push(e?.requests ?? 0);
    inputCache.push(e?.inputCache ?? 0);
    inputNoCache.push(e?.inputNoCache ?? 0);
    output.push(e?.output ?? 0);
    cost.push(e?.cost ?? 0);
  }
  return { xData, requests, inputCache, inputNoCache, output, cost };
}

// 小时级聚合：从 hour 按模型 + 本地日聚合成 24 小时序列
function buildHourlySeries(hour, model, localDate) {
  const acc = Array.from({ length: 24 }, () => ({ requests: 0, inputCache: 0, inputNoCache: 0, output: 0, cost: 0 }));
  for (const r of hour) {
    if (r.model !== model || r.usageDate !== localDate) continue;
    if (r.hour < 0 || r.hour > 23) continue;
    const e = acc[r.hour];
    e.requests += r.requestCount || 0;
    e.inputCache += r.cacheReadTokens || 0;
    e.inputNoCache += r.inputTokens || 0;
    e.output += r.outputTokens || 0;
    e.cost += r.costUSD || 0;
  }
  return {
    xData: Array.from({ length: 24 }, (_, h) => `${String(h).padStart(2, '0')}:00`),
    requests: acc.map(e => e.requests),
    inputCache: acc.map(e => e.inputCache),
    inputNoCache: acc.map(e => e.inputNoCache),
    output: acc.map(e => e.output),
    cost: acc.map(e => e.cost),
  };
}

/** 单模型趋势图卡片（echarts 封装） */
function MiniTrendCard({ title, xData, values, color, valueFmt, areaColor, areaOpacity = 0.08, lineColor, tooltipLabel, tooltipFmt, hourly = false }) {
  const optionRef = useRef(null);
  const { setChartEl, dark, ready } = useECharts(optionRef, [xData, values, hourly]);
  const theme = getChartTheme(dark);

  // 小时模式下 tooltip 首行显示时间跨度（01:00~02:00），否则用 x 轴原值
  const spanLabel = (axisValue) => {
    if (!hourly) return axisValue;
    const h = parseInt(axisValue, 10);
    return `${String(h).padStart(2, '0')}:00~${String(h + 1).padStart(2, '0')}:00`;
  };

  const option = useMemo(() => ({
    grid: { top: 8, right: 16, bottom: 22, left: 48 },
    xAxis: {
      type: 'category', data: xData,
      axisLine: { show: false }, axisTick: { show: false },
      axisLabel: { fontSize: 10.5, color: theme.axisText, formatter: hourly ? undefined : (v) => v.slice(5) },
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
      confine: true,
      backgroundColor: 'transparent', borderWidth: 0, padding: 0,
      formatter: (params) => {
        const p = params[0];
        if (tooltipLabel) {
          return `<div class="bg-popover text-popover-foreground shadow-lg border rounded-lg p-2.5 text-xs">
            <div class="font-semibold mb-1">${spanLabel(p.axisValue)}</div>
            <div class="flex items-center justify-between gap-6">
              <span class="text-muted-foreground">${tooltipLabel}</span>
              <span class="font-semibold tabular-nums">${(tooltipFmt || valueFmt)(p.value)}</span>
            </div>
          </div>`;
        }
        return `<div class="bg-popover text-popover-foreground shadow-lg border rounded-lg p-2 text-xs"><span class="font-semibold">${spanLabel(p.axisValue)}</span><span class="ml-2 tabular-nums font-semibold">${valueFmt(p.value)}</span></div>`;
      },
    },
    series: [{
      type: 'line', data: values, smooth: true, showSymbol: false,
      lineStyle: { width: 2, color: lineColor || color }, itemStyle: { color: lineColor || color },
      areaStyle: { color: areaColor || color, opacity: areaOpacity },
    }],
  }), [xData, values, theme, color, valueFmt, areaColor, areaOpacity, lineColor, tooltipLabel, tooltipFmt, hourly]); // eslint-disable-line react-hooks/exhaustive-deps -- spanLabel 为纯函数，hourly 已入依赖
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

/** Token 堆叠柱状图：输出（底部）/ 输入（未命中缓存）/ 输入（命中缓存，顶部）——自下而上深到浅 */
const TOKEN_SERIES = [
  { key: 'output', name: '输出', color: '#2d6feb' },
  { key: 'inputNoCache', name: '输入（未命中缓存）', color: '#70b1f8' },
  { key: 'inputCache', name: '输入（命中缓存）', color: '#aadafa' },
];
// tooltip 展示顺序固定：命中缓存 → 未命中缓存 → 输出
const TOOLTIP_ORDER = { '输入（命中缓存）': 0, '输入（未命中缓存）': 1, '输出': 2 };

function TokenBarCard({ xData, inputCache, inputNoCache, output, hourly = false }) {
  const optionRef = useRef(null);
  const { setChartEl, dark, ready } = useECharts(optionRef, [xData, inputCache, inputNoCache, output, hourly]);
  const theme = getChartTheme(dark);

  // 小时模式下 tooltip 首行显示时间跨度（01:00~02:00），否则用 x 轴原值
  const spanLabel = (axisValue) => {
    if (!hourly) return axisValue;
    const h = parseInt(axisValue, 10);
    return `${String(h).padStart(2, '0')}:00~${String(h + 1).padStart(2, '0')}:00`;
  };

  const option = useMemo(() => {
    return {
      grid: { top: 8, right: 16, bottom: 22, left: 48 },
      xAxis: {
        type: 'category', data: xData,
        axisLine: { show: false }, axisTick: { show: false },
        axisLabel: { fontSize: 10.5, color: theme.axisText, formatter: hourly ? undefined : (v) => v.slice(5) },
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
        confine: true,
        backgroundColor: 'transparent', borderWidth: 0, padding: 0,
        formatter: (params) => {
          const ordered = [...params].sort((a, b) => TOOLTIP_ORDER[a.seriesName] - TOOLTIP_ORDER[b.seriesName]);
          const date = spanLabel(ordered[0]?.axisValue ?? '');
          const sum = ordered.reduce((s, p) => s + (p.value || 0), 0);
          const rows = ordered.map(p => `
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
        barMaxWidth: 32,
      })),
    };
  }, [xData, inputCache, inputNoCache, output, theme, hourly]); // eslint-disable-line react-hooks/exhaustive-deps -- spanLabel 为纯函数，hourly 已入依赖
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
  const [rangeId, setRangeId] = useState('last7');
  const [customRange, setCustomRange] = useState(null);
  const [data, setData] = useState(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(null);
  const [refreshing, setRefreshing] = useState(false);
  const color = useMemo(() => oklchToHex(getSourceColor(model)), [model]);

  // 跨天回滚：本地日期变化时更新 todayKey，驱动 series 重算 resolveRange，
  // 避免详情页持续开启跨天后「今天/昨天」仍停留在旧日期（与看板跨天修复同因）。
  const [todayKey, setTodayKey] = useState(() => daysAgo(0));
  useEffect(() => {
    const timer = setInterval(() => {
      const today = daysAgo(0);
      if (today !== todayKey) setTodayKey(today);
    }, 30 * 1000);
    return () => clearInterval(timer);
  }, [todayKey]);

  // 独立拉取该模型时间序列（日级 + 小时级）。silent 刷新保留旧数据，仅按钮转圈
  const load = useCallback((silent = false) => {
    if (!silent) { setData(null); setError(null); setLoading(true); }
    else setError(null);
    setRefreshing(true);
    api.getModelSeries(model)
      .then(d => setData(d || { daily: [], hour: [] }))
      .catch(err => { if (!silent) { setError(String(err)); setLoading(false); } })
      .finally(() => { setRefreshing(false); if (!silent) setLoading(false); });
  }, [model]);

  useEffect(() => { load(false); }, [load]);

  // 今天/昨天为小时维度（24 小时）
  const isHourly = rangeId === 'today' || rangeId === 'yesterday';

  const series = useMemo(() => {
    if (loading || !data) return null;
    const r = resolveRange(rangeId, customRange);
    if (r.isHourly) return buildHourlySeries(data.hour || [], model, r.localDate);
    // 「全部」维度裁剪到该模型实际数据范围，避免数据范围外出现无意义空白（真实历史缺口仍保留）
    let start = r.startDate, end = r.endDate;
    if (rangeId === 'all') {
      const dates = (data.daily || []).filter(x => x.model === model).map(x => x.usageDate).filter(Boolean);
      if (dates.length) {
        start = dates.reduce((a, b) => (a < b ? a : b));
        end = dates.reduce((a, b) => (a > b ? a : b));
      }
    }
    return buildDailySeries(data.daily || [], model, start, end);
  }, [loading, data, rangeId, customRange, model, todayKey]); // eslint-disable-line react-hooks/exhaustive-deps -- todayKey 跨天回滚时驱动重算，resolveRange 内部按当前日期求值

  const { xData, requests, inputCache, inputNoCache, output, cost } = series || { xData: [], requests: [], inputCache: [], inputNoCache: [], output: [], cost: [] };
  const totalReq = useMemo(() => requests.reduce((a, b) => a + b, 0), [requests]);
  const totalTok = useMemo(
    () => inputCache.reduce((a, b) => a + b, 0) + inputNoCache.reduce((a, b) => a + b, 0) + output.reduce((a, b) => a + b, 0),
    [inputCache, inputNoCache, output],
  );
  const totalCost = useMemo(() => cost.reduce((a, b) => a + b, 0), [cost]);

  return (
    <div className="flex flex-col gap-4 h-full min-h-0">
      <div className="flex items-center justify-between gap-2">
        <div className="flex items-center gap-2">
          <Button icon={<ArrowLeftIcon className="size-4" />} onClick={onBack}>返回</Button>
          <div className="flex items-center gap-2">
            <img src={getModelIconUrl(model)} alt="" className="size-5 shrink-0 brightness-0 dark:brightness-0 dark:invert" />
            <h2 className="text-sm font-semibold">{model}</h2>
          </div>
        </div>
        <div className="flex items-center gap-2">
          <Select
            value={rangeId}
            onChange={setRangeId}
            options={RANGE_OPTIONS}
            style={{ width: 140 }}
          />
          {rangeId === 'custom' && (
            <DatePicker.RangePicker
              value={customRange}
              onChange={setCustomRange}
              allowClear
              format="YYYY-MM-DD"
            />
          )}
          <Button icon={<RefreshCwIcon className="size-4" />} onClick={() => load(true)} loading={refreshing}>
            刷新
          </Button>
        </div>
      </div>

      {loading ? (
        <div className="flex flex-col gap-4">
          <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
            {[0, 1].map(i => (
              <Card key={i} styles={{ body: { padding: 16 } }}>
                <Skeleton active paragraph={{ rows: 2 }} />
              </Card>
            ))}
          </div>
          <Card styles={{ body: { padding: 16 } }}>
            <Skeleton active paragraph={{ rows: 4 }} />
          </Card>
          <Card styles={{ body: { padding: 16 } }}>
            <Skeleton active paragraph={{ rows: 4 }} />
          </Card>
        </div>
      ) : error ? (
        <div className="flex flex-col items-center justify-center h-64 gap-3 text-muted-foreground">
          <p className="text-sm">详情数据加载失败：{error}</p>
          <Button size="small" onClick={() => load(false)}>重试</Button>
        </div>
      ) : (
        <>
          <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
            <KPI label="请求次数" value={numFmt.format(totalReq)} />
            <KPI label="Tokens" value={numFmt.format(totalTok)} />
          </div>

          <div className="grid grid-cols-1 lg:grid-cols-2 gap-4">
            <MiniTrendCard title="请求次数" xData={xData} values={requests} color={color} valueFmt={(v) => compact(v)} areaColor="#a1c6f9" areaOpacity={0.4} lineColor="#417ceb" tooltipLabel="请求次数" tooltipFmt={numFmt.format} hourly={isHourly} />
            <TokenBarCard xData={xData} inputCache={inputCache} inputNoCache={inputNoCache} output={output} hourly={isHourly} />
          </div>

          <MiniTrendCard title={`消费金额 $${totalCost.toFixed(2)}`} xData={xData} values={cost} color={oklchToHex('oklch(0.72 0.14 75)')} valueFmt={(v) => '$' + Number(v ?? 0).toFixed(2)} tooltipLabel="费用" tooltipFmt={(v) => '$' + Number(v ?? 0).toFixed(2)} hourly={isHourly} />
        </>
      )}
    </div>
  );
}

/** 排行列表视图 */
function RankList({ onSelect }) {
  const [ranking, setRanking] = useState(null);
  const [error, setError] = useState(null);
  const [refreshing, setRefreshing] = useState(false);
  const load = useCallback((silent = false) => {
    // silent 刷新保留旧数据（仅按钮 loading），避免骨架屏闪烁；首次/重试清空显示骨架
    if (!silent) setRanking(null);
    setError(null);
    setRefreshing(true);
    api.getModelRanking()
      .then(list => setRanking(Array.isArray(list) ? list : []))
      .catch(err => { if (!silent) setError(String(err)); })
      .finally(() => setRefreshing(false));
  }, []);

  useEffect(() => { load(false); }, [load]);

  const loading = ranking === null && !error;
  const list = useMemo(() => [...(ranking ?? [])].sort((a, b) => b.totalTokens - a.totalTokens), [ranking]);

  return (
    <div className="flex flex-col gap-4 h-full min-h-0">
      {/* 基础骨架：标题固定渲染，不随数据状态变化 */}
      <div className="flex items-center justify-between">
        <h2 className="text-sm font-semibold">模型排行</h2>
        <Button size="small" className="h-7" icon={<RefreshCwIcon className="size-3.5" />} onClick={() => load(true)} loading={refreshing}>
          刷新
        </Button>
      </div>
      {loading ? (
        <div className="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-3 2xl:grid-cols-4 gap-4">
          {Array.from({ length: 6 }).map((_, i) => (
            <Card key={i} styles={{ body: { padding: 16 } }}>
              <Skeleton active paragraph={{ rows: 2 }} />
            </Card>
          ))}
        </div>
      ) : error ? (
        <div className="flex flex-col items-center justify-center h-64 gap-3 text-muted-foreground">
          <p className="text-sm">排行数据加载失败：{error}</p>
          <Button size="small" onClick={() => load(false)}>重试</Button>
        </div>
      ) : list.length === 0 ? (
        <div className="flex flex-col items-center justify-center h-64 gap-3 text-muted-foreground">
          <p className="text-sm">暂无模型数据</p>
        </div>
      ) : (
        <div className="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-3 2xl:grid-cols-4 gap-4">
          {list.map((m, i) => (
            <Card
              key={m.model}
              className="relative overflow-hidden cursor-pointer transition-shadow hover:shadow-md"
              styles={{ body: { padding: 16 } }}
              onClick={() => onSelect(m.model)}
            >
              <img
                src={getModelIconUrl(m.model)}
                alt=""
                aria-hidden="true"
                className="pointer-events-none absolute right-3 top-1/2 -translate-y-1/2 h-[88%] w-auto max-w-[55%] object-contain opacity-[0.08] brightness-0 dark:opacity-[0.18] dark:brightness-0 dark:invert"
              />
              <div className="relative z-10 flex flex-col gap-6">
                <div className="flex items-center gap-2">
                  <span className={`flex items-center justify-center h-6 min-w-6 px-1.5 rounded-md text-xs font-bold tabular-nums ${RANK_BADGE[i] || 'bg-muted text-muted-foreground'}`}>
                    #{i + 1}
                  </span>
                  <span className="text-sm font-semibold truncate">{m.model}</span>
                </div>
                <div>
                  <div className="text-2xl font-bold tabular-nums leading-none">{fmtUsage(m.totalTokens)}</div>
                </div>
              </div>
            </Card>
          ))}
        </div>
      )}
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
