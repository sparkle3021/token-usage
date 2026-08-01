import React from 'react';
import { Tooltip } from 'antd';
import { formatTooltip } from '@/components/charts/Heatmap/utils.js';

/**
 * @param {Object} props
 * @param {{{ date: string, count: number, isToday: boolean }}} props.day
 * @param {number} props.size
 * @param {(count: number) => string} props.getColor
 * @param {(date: string) => void} [props.onSelect]
 */
const HeatmapCell = React.memo(function HeatmapCell({ day, size, getColor, onSelect }) {
  const { date, count, isToday: today } = day;
  const color = getColor(count);
  const tooltipText = formatTooltip(date, count ?? 0);

  return (
    <Tooltip
      title={<pre className="text-center whitespace-pre-line leading-tight">{tooltipText}</pre>}
      placement="top"
    >
      <div
        className="cursor-pointer transition-none"
        style={{
          width: size,
          height: size,
          backgroundColor: color,
          borderRadius: Math.max(1, size * 0.12) + 'px',
          ...(today ? { border: '1px solid rgba(255,255,255,0.4)', outline: '1px solid rgba(255,255,255,0.2)' } : {}),
        }}
        onClick={() => onSelect?.(date)}
        role="button"
        tabIndex={0}
        onKeyDown={(e) => { if (e.key === 'Enter') onSelect?.(date); }}
      />
    </Tooltip>
  );
});

export default HeatmapCell;
