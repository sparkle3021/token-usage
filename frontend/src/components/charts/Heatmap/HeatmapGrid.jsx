import React from 'react';
import HeatmapCell from './HeatmapCell.jsx';
import { DAY_LABELS } from './constants.js';

/**
 * @param {Object} props
 * @param {import('./types.js').DayData[][]} props.weeks
 * @param {import('./types.js').MonthLabel[]} props.months
 * @param {number} props.cellSize
 * @param {number} props.gap
 * @param {(count: number) => string} props.getColor
 * @param {(date: string) => void} [props.onSelect]
 */
const HeatmapGrid = React.memo(function HeatmapGrid({ weeks, months, cellSize, gap, getColor, onSelect }) {
  const slotSize = cellSize + gap;
  const labelWidth = 32;

  if (weeks.length === 0) {
    return (
      <div className="flex items-center justify-center h-32 text-muted-foreground text-sm">
        暂无数据
      </div>
    );
  }

  return (
    <div className="w-full flex justify-center overflow-x-auto">
      <div className="inline-flex flex-col shrink-0" style={{ gap, '--cell-size': `${cellSize}px`, '--cell-gap': `${gap}px` }}>
        {/* Month labels */}
        <div className="flex text-[10px] text-muted-foreground" style={{ paddingLeft: labelWidth, height: 14, lineHeight: '14px', position: 'relative' }}>
          {months.map(m => (
            <div key={m.col} className="shrink-0 absolute" style={{ left: labelWidth + m.col * slotSize }}>
              {m.label}
            </div>
          ))}
        </div>

        {/* Day rows */}
        {[0, 1, 2, 3, 4, 5, 6].map(dow => (
          <div key={dow} className="flex" style={{ gap, alignItems: 'center', height: cellSize }}>
            {/* Day label */}
            <div
              className="text-[10px] text-muted-foreground text-right shrink-0"
              style={{ width: labelWidth, lineHeight: `${cellSize}px` }}
            >
              {DAY_LABELS[dow] || ''}
            </div>

            {/* Cells for this weekday across all weeks */}
            {weeks.map((week, wi) => {
              const day = week[dow];
              if (day.count === null) {
                return <div key={wi} style={{ width: cellSize, height: cellSize, flexShrink: 0 }} />;
              }
              return (
                <HeatmapCell
                  key={wi}
                  day={day}
                  size={cellSize}
                  getColor={getColor}
                  onSelect={onSelect}
                />
              );
            })}
          </div>
        ))}
      </div>
    </div>
  );
});

export default HeatmapGrid;
