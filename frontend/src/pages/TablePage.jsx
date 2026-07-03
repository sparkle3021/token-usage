import { useState, useMemo } from 'react';
import { Card } from '../components/ui/card.jsx';
import { Tabs, TabsList, TabsTrigger } from '../components/ui/tabs.jsx';
import TablePanel from '../components/tables/TablePanel.jsx';
import DrillDrawer from '../components/tables/DrillDrawer.jsx';
import SourceBadge from '../components/SourceBadge.jsx';
import MultiSelect from '../components/MultiSelect.jsx';
import * as U from '../lib/utils.js';

export default function TablePage({ M, onRefresh }) {
  const defaults = { rangeId: '30d', startDate: U.daysAgo(29), endDate: U.daysAgo(0), sources: new Set(), devices: new Set(), models: new Set() };
  const [f, setF] = useState(defaults);
  const [drill, setDrill] = useState(null);

  const allSources = useMemo(() => [...new Set(M.daily.map(r => r.source))].sort(), [M.daily]);
  const allModels = useMemo(() => [...new Set(M.daily.map(r => r.model))].filter(Boolean).sort(), [M.daily]);

  const filtered = useMemo(() => U.filterDaily(M.daily, f), [f, M.daily]);

  const setRange = (rangeId) => {
    if (rangeId === 'all') {
      const sorted = M.daily.map(x => x.usageDate).filter(Boolean).sort();
      setF({ ...f, rangeId, startDate: sorted[0] || U.daysAgo(0), endDate: sorted[sorted.length - 1] || U.daysAgo(0) });
    } else {
      const days = { today: 1, '7d': 7, '14d': 14, '30d': 30, '90d': 90 }[rangeId] || 30;
      setF({ ...f, rangeId, startDate: U.daysAgo(days - 1), endDate: U.daysAgo(0) });
    }
    if (onRefresh) onRefresh();
  };

  const ranges = [
    { id: 'today', label: '今天' }, { id: '7d', label: '7 天' }, { id: '14d', label: '14 天' },
    { id: '30d', label: '30 天' }, { id: '90d', label: '90 天' }, { id: 'all', label: '全部' },
  ];

  return (
    <div className="flex flex-col min-h-0 flex-1 space-y-4">
      {/* Filter Bar */}
      <Card className="p-3 shrink-0">
        <div className="flex flex-wrap items-center gap-2">
          <span className="text-[10px] uppercase tracking-wider text-muted-foreground font-medium mr-1">时间</span>
          <Tabs value={f.rangeId} onValueChange={setRange}>
            <TabsList>{ranges.map(r => <TabsTrigger key={r.id} value={r.id} className="text-xs px-2.5">{r.label}</TabsTrigger>)}</TabsList>
          </Tabs>
        </div>
        <div className="flex flex-wrap items-center gap-1.5 mt-2">
          <span className="text-[10px] uppercase tracking-wider text-muted-foreground font-medium mr-1">来源</span>
          {allSources.map(s => (
            <SourceBadge key={s} source={s} selected={f.sources.has(s)} onClick={() => { const n = new Set(f.sources); n.has(s) ? n.delete(s) : n.add(s); setF({ ...f, sources: n }); }} />
          ))}
        </div>
        <div className="flex items-center gap-2 mt-2 pt-2 border-t">
          <MultiSelect items={allModels} selected={f.models} onChange={v => setF({ ...f, models: v })} placeholder="全部模型" />
        </div>
      </Card>

      {/* Table */}
      <div className="flex flex-col flex-1 min-h-0">
        <TablePanel daily={filtered} sessions={M.sessions} onDrill={setDrill} fullHeight />
      </div>

      <DrillDrawer drill={drill} daily={M.daily} timeRows={M.time} onClose={() => setDrill(null)} />
    </div>
  );
}
