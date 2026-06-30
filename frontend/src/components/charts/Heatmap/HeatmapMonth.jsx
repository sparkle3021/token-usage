import React from 'react';

/**
 * @param {Object} props
 * @param {import('./types.js').MonthLabel[]} props.months
 * @param {number} props.slotSize
 */
const HeatmapMonth = React.memo(function HeatmapMonth({ months, slotSize }) {
  if (months.length === 0) return null;

  return (
    <div className="flex text-[10px] text-muted-foreground" style={{ marginBottom: 2, height: 14, lineHeight: '14px' }}>
      {months.map(m => (
        <div
          key={m.col}
          className="shrink-0"
          style={{ width: slotSize, marginLeft: m.col * slotSize, position: 'absolute' }}
        >
          {m.label}
        </div>
      ))}
      {/* Spacer to maintain height */}
      <div className="invisible">_</div>
    </div>
  );
});

export default HeatmapMonth;
