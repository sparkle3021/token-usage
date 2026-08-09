import { useState } from 'react';
import { Button, Dropdown, Checkbox } from 'antd';

export default function MultiSelect({ items, selected, onChange, placeholder }) {
  const [open, setOpen] = useState(false);

  const label = selected.size === 0 ? placeholder : selected.size === 1 ? [...selected][0] : `${selected.size} 项`;

  const content = (
    <div className="min-w-[180px] bg-popover border rounded-lg shadow-lg p-1.5 max-h-64 overflow-y-auto">
      {selected.size > 0 && (
        <Button type="text" size="small" className="w-full justify-start text-indigo-500 mb-0.5" onClick={() => onChange(new Set())}>
          清除
        </Button>
      )}
      <Checkbox.Group
        value={[...selected]}
        onChange={(values) => onChange(new Set(values))}
        className="flex flex-col gap-0.5"
      >
        {(items || []).map(o => (
          <Checkbox key={o} value={o} className="!m-0 text-xs w-full px-2 py-1 rounded hover:bg-muted">
            {o}
          </Checkbox>
        ))}
      </Checkbox.Group>
    </div>
  );

  return (
    <Dropdown open={open} onOpenChange={setOpen} trigger={['click']} placement="bottomLeft" popupRender={() => content}>
      <Button size="small" className="text-xs">{label}</Button>
    </Dropdown>
  );
}
