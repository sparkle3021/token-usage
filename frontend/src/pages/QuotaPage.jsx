import { useState, useEffect, useCallback, useRef } from 'react';
import { Button } from '@/components/ui/button.jsx';
import { PlusIcon } from 'lucide-react';
import {
  listQuotaConfigs, getProviderSchemas, createQuotaConfig, updateQuotaConfig,
  deleteQuotaConfig, fetchQuota,
} from '../api/client.js';
import QuotaCard from '../components/QuotaCard.jsx';
import QuotaDialog from '../components/QuotaDialog.jsx';

export default function QuotaPage() {
  const [configs, setConfigs] = useState([]);
  const [schemas, setSchemas] = useState([]);
  const [quotaData, setQuotaData] = useState({}); // configId → QuotaData
  const [loading, setLoading] = useState(true);
  const [dlgOpen, setDlgOpen] = useState(false);
  const [editCfg, setEditCfg] = useState(null);
  const fetchIdRef = useRef(0);

  // 加载配置列表 + schema
  const load = useCallback(async () => {
    try {
      const [cfgs, schs] = await Promise.all([listQuotaConfigs(), getProviderSchemas()]);
      setConfigs(Array.isArray(cfgs) ? cfgs : []);
      setSchemas(Array.isArray(schs) ? schs : []);
      return Array.isArray(cfgs) ? cfgs : [];
    } catch { return []; }
  }, []);

  // 逐条拉取用量数据——哪个先回来先渲染
  const fetchAllIncremental = useCallback(async (cfgs, fetchId) => {
    // 逐个并发，每条返回即更新对应卡片
    cfgs.forEach(async (cfg) => {
      try {
        const d = await fetchQuota(cfg.id);
        if (fetchId !== fetchIdRef.current) return; // 过时的请求，丢弃
        if (d) setQuotaData(prev => ({ ...prev, [cfg.id]: d }));
      } catch { /* ignore */ }
    });
    // 全部完成后刷新配置以同步 is_valid
    try {
      await Promise.allSettled(cfgs.map(c => fetchQuota(c.id)));
      if (fetchId !== fetchIdRef.current) return;
      const cfgs = await listQuotaConfigs();
      if (Array.isArray(cfgs)) setConfigs(cfgs);
    } catch { /* ignore */ }
  }, []);

  // 初始化：先加载配置 → 立刻渲染卡片骨架 → 逐条拉取用量
  useEffect(() => {
    let cancelled = false;
    (async () => {
      setLoading(true);
      const cfgs = await load();
      if (cancelled) return;
      setLoading(false);
      if (cfgs.length > 0) {
        const id = ++fetchIdRef.current;
        fetchAllIncremental(cfgs, id);
      }
    })();
    return () => { cancelled = true; };
  }, []); // eslint-disable-line react-hooks/exhaustive-deps

  // 单条刷新
  const refreshOne = useCallback(async (cfg) => {
    try {
      const d = await fetchQuota(cfg.id);
      setQuotaData(prev => ({ ...prev, [cfg.id]: d }));
      const cfgs = await listQuotaConfigs();
      if (Array.isArray(cfgs)) setConfigs(cfgs);
    } catch { /* ignore */ }
  }, []);

  // 全量重新拉取（新建/编辑后）
  const refreshAll = useCallback(async () => {
    const cfgs = await load();
    if (cfgs.length > 0) {
      const id = ++fetchIdRef.current;
      fetchAllIncremental(cfgs, id);
    }
  }, [load, fetchAllIncremental]);

  // 自动刷新 5 分钟
  useEffect(() => {
    if (configs.length === 0) return;
    const t = setInterval(() => {
      const id = ++fetchIdRef.current;
      fetchAllIncremental(configs, id);
    }, 5 * 60 * 1000);
    return () => clearInterval(t);
  }, [configs, fetchAllIncremental]);

  const getSchema = (provider) => schemas.find(s => s.id === provider);

  const handleSave = async (cfg) => {
    try {
      if (cfg.id) {
        await updateQuotaConfig(cfg);
      } else {
        await createQuotaConfig(cfg);
      }
      await refreshAll();
      setEditCfg(null);
    } catch { /* ignore */ }
  };

  const handleEdit = (cfg) => { setEditCfg(cfg); setDlgOpen(true); };

  const handleDelete = async (cfg) => {
    if (!window.confirm(`确定删除 "${cfg.displayName || cfg.provider + '-' + cfg.plan + '-' + cfg.seq}"？`)) return;
    try {
      await deleteQuotaConfig(cfg.id);
      setQuotaData(prev => { const n = { ...prev }; delete n[cfg.id]; return n; });
      setConfigs(prev => prev.filter(c => c.id !== cfg.id));
    } catch { /* ignore */ }
  };

  const handleAdd = () => { setEditCfg(null); setDlgOpen(true); };

  if (loading) return (
    <div className="flex items-center justify-center h-64 text-sm text-muted-foreground">加载中…</div>
  );

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <h2 className="text-sm font-semibold">用量查询</h2>
        <Button size="sm" variant="default" className="h-8 text-xs" onClick={handleAdd}>
          <PlusIcon className="size-3.5 mr-1" />
          添加用量查询
        </Button>
      </div>

      {configs.length === 0 ? (
        <div className="flex flex-col items-center justify-center h-64 gap-3 text-muted-foreground">
          <p className="text-sm">暂无用量查询</p>
          <Button size="sm" onClick={handleAdd}>添加用量查询</Button>
        </div>
      ) : (
        <div className="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-3 gap-4">
          {configs.map(cfg => {
            const sch = getSchema(cfg.provider);
            const data = quotaData[cfg.id];
            return (
              <QuotaCard
                key={cfg.id}
                cfg={cfg}
                data={data}
                displayType={sch?.displayType}
                slotsLabels={sch?.slotsLabels}
                balanceLabel={sch?.balanceLabel}
                onEdit={() => handleEdit(cfg)}
                onDelete={() => handleDelete(cfg)}
                onRefresh={() => refreshOne(cfg)}
              />
            );
          })}
        </div>
      )}

      <QuotaDialog
        open={dlgOpen}
        onOpenChange={setDlgOpen}
        schemas={schemas}
        editCfg={editCfg}
        onSave={handleSave}
      />
    </div>
  );
}
