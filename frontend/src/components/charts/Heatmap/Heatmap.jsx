import React, { useRef, useState, useEffect, useMemo } from 'react';
import { Card } from 'antd';
import HeatmapGrid from '@/components/charts/Heatmap/HeatmapGrid.jsx';
import HeatmapLegend from '@/components/charts/Heatmap/HeatmapLegend.jsx';
import { useHeatmap } from '@/components/charts/Heatmap/hooks.js';
import { getContributionColor } from '@/components/charts/Heatmap/utils.js';
import { DEFAULT_THEME, DARK_THEME, CELL_SIZE, GAP } from '@/components/charts/Heatmap/constants.js';

const LABEL_WIDTH = 32;
const MIN_CELL = 8;
const MAX_CELL = 20;

/**
 * GitHub-style contribution heatmap with adaptive cell sizing.
 *
 * @param {Object} props
 */
export default function Heatmap({
  data = [],
  startDate,
  endDate,
  cellSize: fixedSize,
  gap = GAP,
  onSelect,
  className = '',
  theme: themeProp,
}) {
  // 明暗主题跟随 html.dark class（App 单点驱动），避免 props 穿透
  const [isDark, setIsDark] = useState(() => typeof document !== 'undefined' && document.documentElement.classList.contains('dark'));
  useEffect(() => {
    const el = document.documentElement;
    const observer = new MutationObserver(() => setIsDark(el.classList.contains('dark')));
    observer.observe(el, { attributes: true, attributeFilter: ['class'] });
    return () => observer.disconnect();
  }, []);
  const theme = themeProp || (isDark ? DARK_THEME : DEFAULT_THEME);
  const containerRef = useRef(null);
  const [containerWidth, setContainerWidth] = useState(0);

  const { weeks, months } = useHeatmap(data, startDate, endDate);

  // Measure container width for adaptive sizing
  useEffect(() => {
    const el = containerRef.current;
    if (!el) return;

    const measure = () => setContainerWidth(el.clientWidth);
    const observer = new ResizeObserver(measure);
    observer.observe(el);
    measure();

    return () => observer.disconnect();
  }, []);

  // Compute cell size: fixed > adaptive > default fallback
  const cellSize = useMemo(() => {
    if (fixedSize) return fixedSize;
    if (!containerWidth || weeks.length === 0) return CELL_SIZE;

    const available = containerWidth - LABEL_WIDTH - (weeks.length * gap);
    const computed = Math.floor(available / weeks.length);
    return Math.max(MIN_CELL, Math.min(MAX_CELL, computed));
  }, [containerWidth, fixedSize, weeks.length, gap]);

  // Color function using exponential thresholds
  const getColor = useMemo(() => {
    return (count) => getContributionColor(count, theme);
  }, [theme]);

  return (
    <Card className={className} styles={{ body: { padding: 16 } }}>
      <div className="flex items-center justify-between mb-3">
        <div className="text-sm font-semibold">Token 消耗热力图</div>
        <HeatmapLegend theme={theme} cellSize={cellSize} />
      </div>
      <div ref={containerRef}>
        <HeatmapGrid
          weeks={weeks}
          months={months}
          cellSize={cellSize}
          gap={gap}
          getColor={getColor}
          onSelect={onSelect}
        />
      </div>
    </Card>
  );
}
