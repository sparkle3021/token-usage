import { useMemo } from 'react';
import { Dialog, DialogContent, DialogTitle, DialogDescription } from '../../components/ui/dialog.jsx';
import * as U from '../../lib/utils.js';
import SourceIcon from '../../components/SourceIcon.jsx';

/**
 * Table 页面来源行「模型分布」按钮点击 → 专用弹窗。
 * 展示该来源（source+device）在过滤时间范围内的模型分布，
 * 与看板 TopModels 同款进度条：来源细分堆叠条 + 总 Token + 占比。
 */
export default function SourceDrillDialog({ drill, daily, allDaily = [], onClose }) {
  const { row } = drill;

  const detail = useMemo(() => {
    const matching = (daily || []).filter(r => r.source === row.source && r.device === row.device);
    const totals = U.aggregateTotals(matching);
    // 活跃天数按全量 daily 统计（来源的固有属性，不随筛选范围变化）
    const dates = new Set((allDaily || []).filter(r => r.source === row.source && r.device === row.device && r.usageDate).map(r => r.usageDate));

    // 模型维度聚合（该来源下的各模型 Token 占比）
    const modelMap = new Map();
    for (const r of matching) {
      if (!r.model) continue;
      modelMap.set(r.model, (modelMap.get(r.model) || 0) + (r.totalTokens || 0));
    }
    const list = [...modelMap.entries()].map(([model, total]) => ({ model, total })).sort((a, b) => b.total - a.total);

    return { totals, dates, list, sourceTotal: matching.reduce((s, r) => s + (r.totalTokens || 0), 0) };
  }, [daily, allDaily, row]);

  return (
    <Dialog open onOpenChange={o => { if (!o) onClose(); }}>
      <DialogContent className="sm:max-w-lg max-h-[85vh] overflow-y-auto" showCloseButton>
        <DialogTitle className="sr-only">{row.source} 模型分布</DialogTitle>
        <DialogDescription className="sr-only">{row.device} 模型用量分布</DialogDescription>

        <div className="mb-4">
          <div className="text-xs text-muted-foreground mb-0.5">模型分布</div>
          <h3 className="text-sm font-semibold flex items-center gap-1.5"><SourceIcon name={row.source} className="w-4 h-4" />{row.source}<span className="text-xs text-muted-foreground font-normal">{row.device}</span></h3>
        </div>

        <div className="flex items-center gap-4 text-xs text-muted-foreground mb-4 flex-wrap">
          <span>活跃 <strong className="text-foreground">{detail.dates.size}</strong> 天</span>
          <span>总 Token <strong className="text-foreground">{U.compactCN(detail.totals.totalTokens)}</strong></span>
          <span>费用 <strong className="text-foreground">${(detail.totals.costUSD || 0).toFixed(2)}</strong></span>
          <span>缓存命中率 <strong className="text-foreground">{detail.totals.cacheHitRate.toFixed(1)}%</strong></span>
        </div>

        {detail.list.length > 0 ? (
          <div className="space-y-1.5">
            {detail.list.map(m => {
              const pct = detail.sourceTotal > 0 ? (m.total / detail.sourceTotal * 100) : 0;
              return (
                <div key={m.model} className="grid grid-cols-[1fr_auto] items-center gap-3 px-1.5 py-1.5">
                  <div className="min-w-0">
                    <div className="flex items-center gap-1.5">
                      {U.getModelIconUrl(m.model) && <img src={U.getModelIconUrl(m.model)} className="w-3.5 h-3.5 shrink-0" alt="" />}
                      <span className="text-xs font-medium truncate">{m.model}</span>
                    </div>
                    <div className="h-1.5 rounded-full bg-muted mt-1.5 overflow-hidden">
                      <div className="h-full" style={{ width: `${pct}%`, background: U.getSourceColor(row.source) }} />
                    </div>
                  </div>
                  <div className="text-right shrink-0">
                    <div className="text-xs font-semibold tabular-nums">{U.compactCN(m.total)}</div>
                    <div className="text-[10px] text-muted-foreground tabular-nums">{pct.toFixed(1)}%</div>
                  </div>
                </div>
              );
            })}
          </div>
        ) : (
          <div className="flex items-center justify-center h-24 text-muted-foreground text-sm">暂无模型数据</div>
        )}
      </DialogContent>
    </Dialog>
  );
}
