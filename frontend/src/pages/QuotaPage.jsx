import { useState, useEffect, useCallback } from 'react';
import { Button } from '@/components/ui/button.jsx';
import { PlusIcon } from 'lucide-react';
import {
  listQuotaConfigs, getProviderSchemas, createQuotaConfig, updateQuotaConfig,
  deleteQuotaConfig, fetchAllQuota, fetchQuota,
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

  // 加载配置列表 + schema
  const load = useCallback(async () => {
    try {
      const [cfgs, schs] = await Promise.all([listQuotaConfigs(), getProviderSchemas()]);
      setConfigs(Array.isArray(cfgs) ? cfgs : []);
      setSchemas(Array.isArray(schs) ? schs : []);
    } catch { /* ignore */ }
    setLoading(false);
  }, []);

  // 拉取所有用量数据，之后刷新配置以同步 is_valid 状态
  const refreshAll = useCallback(async () => {
    if (configs.length === 0) return;
    try {
      const [dataList, cfgs] = await Promise.all([fetchAllQuota(), listQuotaConfigs()]);
      if (Array.isArray(dataList)) {
        const map = {};
        for (const d of dataList) map[d.configId] = d;
        setQuotaData(map);
      }
      if (Array.isArray(cfgs)) setConfigs(cfgs);
    } catch { /* ignore */ }
  }, [configs.length]);

  // 初始化
  useEffect(() => { load(); }, [load]);

  // 配置变更后重新拉取
  useEffect(() => {
    if (configs.length > 0) refreshAll();
  }, [configs.length, refreshAll]);

  // 自动刷新 5 分钟
  useEffect(() => {
    if (configs.length === 0) return;
    const t = setInterval(refreshAll, 5 * 60 * 1000);
    return () => clearInterval(t);
  }, [configs.length, refreshAll]);

  // 获取特定配置的 schema
  const getSchema = (provider) => schemas.find(s => s.id === provider);

  // 保存（新建 or 编辑）
  const handleSave = async (cfg) => {
    try {
      if (cfg.id) {
        await updateQuotaConfig(cfg);
      } else {
        await createQuotaConfig(cfg);
      }
      await load(); // 重新加载配置列表
      setEditCfg(null);
    } catch { /* ignore */ }
  };

  // 编辑
  const handleEdit = (cfg) => {
    setEditCfg(cfg);
    setDlgOpen(true);
  };

  // 删除
  const handleDelete = async (cfg) => {
    if (!window.confirm(`确定删除 "${cfg.displayName || cfg.provider + '-' + cfg.plan + '-' + cfg.seq}"？`)) return;
    try {
      await deleteQuotaConfig(cfg.id);
      await load();
    } catch { /* ignore */ }
  };

  // 单个刷新
  const handleRefreshOne = async (cfg) => {
    try {
      const d = await fetchQuota(cfg.id);
      setQuotaData(prev => ({ ...prev, [cfg.id]: d }));
      // 同步 is_valid 状态
      const cfgs = await listQuotaConfigs();
      if (Array.isArray(cfgs)) setConfigs(cfgs);
    } catch { /* ignore */ }
  };

  const handleAdd = () => {
    setEditCfg(null);
    setDlgOpen(true);
  };

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
                onRefresh={() => handleRefreshOne(cfg)}
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
