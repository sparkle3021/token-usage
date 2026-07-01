import { useState, useRef, useEffect } from 'react';
import { Button } from './ui/button.jsx';

export default function MultiSelect({ items, selected, onChange, placeholder }) {
  const [open, setOpen] = useState(false);
  const ref = useRef(null);
  useEffect(() => {
    const f = (e) => { if (ref.current && !ref.current.contains(e.target)) setOpen(false); };
    document.addEventListener('mousedown', f);
    return () => document.removeEventListener('mousedown', f);
  }, []);

  return (
    <div ref={ref} className="relative inline-block">
      <Button size="sm" variant="outline" onClick={() => setOpen(o => !o)} className="text-xs">
        {selected.size === 0 ? placeholder : selected.size === 1 ? [...selected][0] : `${selected.size} 项`}
      </Button>
      {open && (
        <div className="absolute top-full left-0 mt-1 z-30 min-w-[180px] bg-popover border rounded-lg shadow-lg p-1.5 max-h-64 overflow-y-auto">
          {selected.size > 0 && <Button size="xs" variant="ghost" className="w-full justify-start text-indigo-500 mb-0.5" onClick={() => onChange(new Set())}>清除</Button>}
          {(items || []).map(o => (
            <button key={o} className={`w-full text-left px-2 py-1 text-xs rounded flex items-center gap-2 hover:bg-muted ${selected.has(o) ? 'font-medium' : ''}`}
              onClick={() => { const n = new Set(selected); n.has(o) ? n.delete(o) : n.add(o); onChange(n); }}>
              <input type="checkbox" className="accent-indigo-500" checked={selected.has(o)} readOnly />
              {o}
            </button>
          ))}
        </div>
      )}
    </div>
  );
}
