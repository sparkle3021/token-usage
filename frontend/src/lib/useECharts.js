/**
 * ECharts 自封装 hook：按需注册 + init/resize/dispose + 明暗主题同步。
 * 主题跟随 html.dark class（App 单点驱动），与 Heatmap 的 MutationObserver 模式一致。
 * 容器用回调 ref 接收，天然处理「空态 → 有数据」的动态挂载，无需手动管理元素出现时机。
 */

import { useCallback, useEffect, useRef, useState } from 'react';
import * as echarts from 'echarts/core';
import { BarChart, LineChart } from 'echarts/charts';
import { GridComponent, TooltipComponent } from 'echarts/components';
import { CanvasRenderer } from 'echarts/renderers';

echarts.use([BarChart, LineChart, GridComponent, TooltipComponent, CanvasRenderer]);

// 图表明暗色板：浅色沿用既有 recharts 硬编码值，暗色按 Tailwind 暗色语义变量近似定值。
// 轴/文字颜色随 html.dark 切换（notMerge 重设），系列颜色由调用方 getSourceColor 提供、不动。
export const CHART_THEME = {
  light: {
    grid: '#e7e5e0',
    axisText: '#8c8a86',
    axisTick: '#9d9b97',
  },
  dark: {
    grid: '#2e2e2e',
    axisText: '#9a9a9a',
    axisTick: '#7c7c7c',
  },
};

export function getChartTheme(dark) {
  return dark ? CHART_THEME.dark : CHART_THEME.light;
}

export function isDarkTheme() {
  return typeof document !== 'undefined' && document.documentElement.classList.contains('dark');
}

/**
 * @param {{ current: object|null }} optionRef option 容器 ref（调用方在渲染时写入最新 option，绕开「option 依赖 dark」的循环）
 * @param {any[]} [deps] setOption 的依赖数组
 * @returns {{ chartRef: import('react').RefObject, setChartEl: (el: HTMLElement|null) => void, dark: boolean }}
 */
export default function useECharts(optionRef, deps = []) {
  const chartRef = useRef(null);
  const observerRef = useRef(null);
  const [dark, setDark] = useState(isDarkTheme);
  const [ready, setReady] = useState(false);

  // 容器回调 ref：挂载 → init + 立即 setOption；卸载 → 延迟 dispose（避免与 React removeChild 冲突）。
  // init 后立即 setOption 是必需的：Modal 中若依赖 useEffect 的 deps 触发，首次因 chart 为 null 被跳过 → 图表空白。
  const setChartEl = useCallback((el) => {
    if (!el) {
      observerRef.current?.disconnect();
      observerRef.current = null;
      const chart = chartRef.current;
      chartRef.current = null;
      setReady(false);
      // 延迟到 React commit 之后 dispose：避免 echarts 移除 canvas 与 React removeChild 冲突
      // （频繁切时间范围时图表容器反复卸载/挂载，同步 dispose 会触发 removeChild NotFoundError）
      if (chart) setTimeout(() => chart.dispose(), 0);
      return;
    }
    if (chartRef.current) return;
    let chart;
    try {
      chart = echarts.init(el);
    } catch {
      return;
    }
    chartRef.current = chart;
    const observer = new ResizeObserver(() => chart.resize());
    observer.observe(el);
    observerRef.current = observer;
    // 立即渲染已有 option，消除「init 晚于 effect 导致 setOption 被跳过」的空白
    if (optionRef.current) chart.setOption(optionRef.current, { notMerge: true });
    setReady(true);
    // 多时机 resize 兜底：Modal 动画期容器可能 0 尺寸，尺寸就绪后重绘既有内容（resize 会重绘已 setOption 的数据）
    const ensure = () => { if (chartRef.current === chart) chart.resize(); };
    requestAnimationFrame(ensure);
    setTimeout(ensure, 100);
    setTimeout(ensure, 400);
  }, []); // eslint-disable-line react-hooks/exhaustive-deps -- optionRef 为稳定 ref，无需纳入依赖

  // 主题切换：监听 html.dark class
  useEffect(() => {
    const el = document.documentElement;
    const observer = new MutationObserver(() => setDark(el.classList.contains('dark')));
    observer.observe(el, { attributes: true, attributeFilter: ['class'] });
    return () => observer.disconnect();
  }, []);

  // 数据/主题更新：dark 或 deps 变化时重设 option
  useEffect(() => {
    const chart = chartRef.current;
    if (!chart || !optionRef.current) return;
    try {
      chart.setOption(optionRef.current, { notMerge: true });
    } catch {
      // 忽略 echarts 渲染异常，避免中断 React 渲染导致白屏
    }
  }, [dark, ...deps]); // eslint-disable-line react-hooks/exhaustive-deps

  return { chartRef, setChartEl, dark, ready };
}
