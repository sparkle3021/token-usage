import React, { useRef, useState, useEffect, useMemo } from 'react';
import { Card, CardHeader, CardTitle, CardContent } from '../../ui/card.jsx';
import HeatmapGrid from './HeatmapGrid.jsx';
import HeatmapLegend from './HeatmapLegend.jsx';
import { useHeatmap } from './hooks.js';
import { getContributionColor } from './utils.js';
import { DEFAULT_THEME, CELL_SIZE, GAP } from './constants.js';

const LABEL_WIDTH = 32;
const MIN_CELL = 8;
const MAX_CELL = 20;

/**
 * GitHub-style contribution heatmap with adaptive cell sizing.
 *
 * @param {import('./types.js').HeatmapProps} props
 */
export default function Heatmap({
  data = [],
  startDate,
  endDate,
  cellSize: fixedSize,
  gap = GAP,
  onSelect,
  className = '',
  theme = DEFAULT_THEME,
}) {
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
    <Card className={className}>
      <CardHeader>
        <div className="flex items-center justify-between">
          <CardTitle>Token 消耗热力图</CardTitle>
          <HeatmapLegend theme={theme} cellSize={cellSize} />
        </div>
      </CardHeader>
      <CardContent>
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
      </CardContent>
    </Card>
  );
}
