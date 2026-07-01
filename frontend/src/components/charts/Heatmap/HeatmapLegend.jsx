import React from 'react';
import { DEFAULT_THEME, CELL_SIZE } from './constants.js';

/**
 * @param {Object} props
 * @param {import('./types.js').HeatmapTheme} [props.theme]
 * @param {number} [props.cellSize]
 */
const HeatmapLegend = React.memo(function HeatmapLegend({ theme = DEFAULT_THEME, cellSize = CELL_SIZE }) {
  const levels = [theme.level1, theme.level2, theme.level3, theme.level4, theme.level5];

  return (
    <div className="flex items-center gap-1 text-[10px] text-muted-foreground justify-end mt-1">
      少
      {levels.map((color, i) => (
        <span
          key={i}
          className="inline-block"
          style={{ width: cellSize, height: cellSize, backgroundColor: color, borderRadius: Math.max(1, cellSize * 0.12) + 'px' }}
        />
      ))}
      多
    </div>
  );
});

export default HeatmapLegend;
