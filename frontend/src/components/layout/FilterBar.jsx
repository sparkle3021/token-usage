/**
 * 过滤器行组件：时间范围 Tabs、来源标签选择、模型多选、对比切换。
 */

import { Card } from '../ui/card.jsx';
import { Tabs, TabsList, TabsTrigger } from '../ui/tabs.jsx';
import { Button } from '../ui/button.jsx';
import SourceBadge from '../SourceBadge.jsx';
import MultiSelect from '../MultiSelect.jsx';

const ranges = [
  { id: 'today', label: '今天' }, { id: '7d', label: '7 天' }, { id: '14d', label: '14 天' },
  { id: '30d', label: '30 天' }, { id: '90d', label: '90 天' }, { id: 'all', label: '全部' },
];

export default function FilterBar({ f, allSources, allModels, onSetRange, onToggleSource, onSetModels, onToggleCompare }) {
  return (
    <Card className="p-3 overflow-visible">
      <div className="flex flex-wrap items-center gap-2 mb-2">
        <span className="text-[10px] uppercase tracking-wider text-muted-foreground font-medium">时间</span>
        <Tabs value={f.rangeId} onValueChange={onSetRange}>
          <TabsList>{ranges.map(r => <TabsTrigger key={r.id} value={r.id} className="text-xs px-2.5">{r.label}</TabsTrigger>)}</TabsList>
        </Tabs>
      </div>
      <div className="flex flex-wrap items-center gap-1.5">
        <span className="text-[10px] uppercase tracking-wider text-muted-foreground font-medium mr-1">来源</span>
        {allSources.map(s => (
          <SourceBadge key={s} source={s} selected={f.sources.has(s)} onClick={() => onToggleSource(s)} />
        ))}
      </div>
      <div className="flex flex-wrap items-center gap-2 mt-2 pt-2 border-t">
        <MultiSelect items={allModels} selected={f.models} onChange={onSetModels} placeholder="全部模型" />
        <div className="flex-1" />
        <Button size="sm" variant={f.compare ? 'default' : 'outline'} onClick={onToggleCompare}>
          对比
        </Button>
      </div>
    </Card>
  );
}
