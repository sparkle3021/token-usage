import * as U from '../lib/utils.js';
import SourceIcon from './SourceIcon.jsx';

export default function SourceBadge({ source, selected, onClick, size = 'sm' }) {
  const color = U.getSourceColor(source || '');
  const sm = size === 'sm';

  const base = `inline-flex items-center rounded font-medium whitespace-nowrap
    ${sm ? 'text-[10px] px-1 py-0 h-4 gap-1' : 'text-xs px-1.5 py-0.5 h-5 gap-1.5'}
    ${onClick ? 'cursor-pointer transition-all' : ''}`;

  const iconSize = sm ? 'w-2.5 h-2.5' : 'w-3 h-3';

  if (selected != null) {
    // Clickable filter badge with selected/unselected state
    const stateCls = selected
      ? 'text-white border-transparent'
      : 'bg-transparent border';
    return (
      <span
        className={`${base} ${stateCls}`}
        style={{
          borderColor: selected ? 'transparent' : color,
          backgroundColor: selected ? color : 'transparent',
          color: selected ? '#fff' : color,
        }}
        onClick={onClick}
      >
        <SourceIcon name={source} className={iconSize} />
        {source}
      </span>
    );
  }

  // Display-only badge (TopModels style)
  return (
    <span
      className={`${base} bg-transparent border`}
      style={{ borderColor: color, color }}
      onClick={onClick}
    >
      <SourceIcon name={source} className={iconSize} />
      {source || 'unknown'}
    </span>
  );
}
