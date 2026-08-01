/**
 * 过滤器行组件：时间范围 Segmented、来源标签选择、模型多选、对比切换。
 */

import { ranges } from '@/store/filterStore.jsx';
import { Card, Segmented, Button } from 'antd';
import SourceBadge from '@/components/common/SourceBadge.jsx';
import MultiSelect from '@/components/common/MultiSelect.jsx';

export default function FilterBar({ f, allSources, allModels, onSetRange, onToggleSource, onSetModels, onToggleCompare }) {
  return (
    <Card className="p-3 overflow-visible" styles={{ body: { padding: 12 } }}>
      <div className="flex flex-wrap items-center gap-2 mb-2">
        <span className="text-xs uppercase tracking-wider text-muted-foreground font-medium">时间</span>
        <Segmented
          value={f.rangeId}
          onChange={onSetRange}
          size="small"
          options={ranges.map(r => ({ label: r.label, value: r.id }))}
        />
      </div>
      <div className="flex flex-wrap items-center gap-1.5">
        <span className="text-xs uppercase tracking-wider text-muted-foreground font-medium mr-1">来源</span>
        {allSources.map(s => (
          <SourceBadge key={s} source={s} selected={f.sources.has(s)} onClick={() => onToggleSource(s)} />
        ))}
      </div>
      <div className="flex flex-wrap items-center gap-2 mt-2 pt-2 border-t">
        <MultiSelect items={allModels} selected={f.models} onChange={onSetModels} placeholder="全部模型" />
        <div className="flex-1" />
        <Button size="small" type={f.compare ? 'primary' : 'default'} onClick={onToggleCompare}>
          对比
        </Button>
      </div>
    </Card>
  );
}
