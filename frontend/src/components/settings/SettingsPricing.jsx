import { useCallback, useEffect, useMemo, useState } from 'react';
import { Button, Col, Form, Input, InputNumber, Modal, Row, Segmented, Table } from 'antd';
import { PencilIcon, RefreshCwIcon, SearchIcon, Trash2Icon } from 'lucide-react';
import { getMessage } from '@/lib/message.js';
import { compactCN, formatTs } from '@/lib/formatters.js';
import { listModelPricing, updateModelPricing, deleteModelPricing, getPricingMeta, getModelRanking, updatePricing } from '@/api/client.js';
import SettingField from '@/components/common/SettingField.jsx';
import ConfirmDialog from '@/components/common/ConfirmDialog.jsx';

// 价格展示/输入统一为 $/1M tokens（每百万 token 价格，业界惯例）：rate(每token) × 1e6
const PER_M = 1e6;

// 简短名：LiteLLM key 形如 tensormesh/deepseek-ai/DeepSeek-V4-Flash，取最后一段便于阅读
function bareName(key) {
  return String(key || '').split('/').pop() || key;
}

// 纯数字格式化（$/M 单位已在页面说明中声明，单元格不重复单位）
function fmtNum(rate) {
  if (rate == null || rate <= 0) return null;
  const perM = rate * PER_M;
  if (perM >= 100) return perM.toFixed(0);
  if (perM >= 1) return perM.toFixed(2);
  if (perM >= 0.01) return perM.toFixed(3);
  return perM.toPrecision(2);
}

// 未设置价格的行展示
function PriceCell({ rate }) {
  const s = fmtNum(rate);
  if (s === null) {
    return <span className="text-muted-foreground/50">未设置</span>;
  }
  return <span className="tabular-nums">{s}</span>;
}

/**
 * 编辑价格弹窗：$/1M tokens 单位输入（与列表展示一致），保存时换算回每 token 入库。
 */
function EditPriceModal({ row, onClose, onSaved }) {
  const [form] = Form.useForm();
  const [saving, setSaving] = useState(false);
  const toast = getMessage();
  const isNew = row.inputRate == null;

  const handleOk = useCallback(async () => {
    try {
      const v = await form.validateFields();
      setSaving(true);
      await updateModelPricing({
        modelKey: row.modelKey,
        inputRate: (v.inputPerM || 0) / PER_M,
        outputRate: (v.outputPerM || 0) / PER_M,
        cacheReadRate: (v.cacheReadPerM || 0) / PER_M,
        cacheWriteRate: (v.cacheWritePerM || 0) / PER_M,
      });
      toast?.success(isNew ? `已为 ${row.modelKey} 设置价格` : `已更新 ${row.modelKey} 价格`);
      onSaved();
    } catch (err) {
      if (err?.errorFields) return; // 表单校验失败
      console.error('[pricing] save failed', err);
      toast?.error('保存失败，请重试');
    } finally {
      setSaving(false);
    }
  }, [form, row, toast, onSaved, isNew]);

  return (
    <Modal
      open
      title={isNew ? `设置价格：${bareName(row.modelKey)}` : `编辑价格：${bareName(row.modelKey)}`}
      onOk={handleOk}
      onCancel={onClose}
      okText="保存"
      cancelText="取消"
      confirmLoading={saving}
      centered
      width="min(560px, calc(100vw - 48px))"
    >
      <div className="text-[11px] text-muted-foreground/70 mb-2 truncate" title={row.modelKey}>
        {row.modelKey}
      </div>
      <div className="text-xs text-muted-foreground mb-3">单位：美元 / 100 万 tokens。输入（缓存写入）仅部分供应商（如 Claude）单独计费，其余已含在输入（缓存未命中）价中。</div>
      <Form form={form} layout="horizontal" initialValues={{
        inputPerM: row.inputRate != null ? row.inputRate * PER_M : 0,
        outputPerM: row.outputRate != null ? row.outputRate * PER_M : 0,
        cacheReadPerM: row.cacheReadRate != null ? row.cacheReadRate * PER_M : 0,
        cacheWritePerM: row.cacheWriteRate != null ? row.cacheWriteRate * PER_M : 0,
      }}>
        {/* antd 标准两列表单：Row/Col 分列，Form.Item label 统一列宽，天然对齐不错位 */}
        <Row gutter={16}>
          <Col span={12}>
            <Form.Item name="inputPerM" label="输入（缓存未命中）" labelCol={{ flex: '118px' }} wrapperCol={{ flex: 1 }} rules={[{ required: true, message: '请输入' }]}>
              <InputNumber min={0} className="w-full" placeholder="0.28" />
            </Form.Item>
          </Col>
          <Col span={12}>
            <Form.Item name="cacheReadPerM" label="输入（缓存命中）" labelCol={{ flex: '118px' }} wrapperCol={{ flex: 1 }} rules={[{ required: true, message: '请输入' }]}>
              <InputNumber min={0} className="w-full" placeholder="0.0028" />
            </Form.Item>
          </Col>
          <Col span={12}>
            <Form.Item name="outputPerM" label="输出" labelCol={{ flex: '118px' }} wrapperCol={{ flex: 1 }} rules={[{ required: true, message: '请输入' }]}>
              <InputNumber min={0} className="w-full" placeholder="0.56" />
            </Form.Item>
          </Col>
          <Col span={12}>
            <Form.Item name="cacheWritePerM" label="输入（缓存写入）" labelCol={{ flex: '118px' }} wrapperCol={{ flex: 1 }} rules={[{ required: true, message: '请输入' }]}>
              <InputNumber min={0} className="w-full" placeholder="0" />
            </Form.Item>
          </Col>
        </Row>
      </Form>
    </Modal>
  );
}

/**
 * 设置-价格管理：model_pricing 表查看/搜索/编辑/删除 + 拉取更新。
 * 双视图：使用中（按实际用量排序，无分页）｜全部（搜索 + 分页）。
 * 唯一价格源：拉取（UPSERT 覆盖，含手动修改）；改价立即生效，仅影响之后采集。
 */
export default function SettingsPricing() {
  const [rows, setRows] = useState([]);
  const [ranking, setRanking] = useState([]);
  const [meta, setMeta] = useState(null);
  const [view, setView] = useState('active');
  const [keyword, setKeyword] = useState('');
  const [loading, setLoading] = useState(false);
  const [updating, setUpdating] = useState(false);
  const [confirmUpdate, setConfirmUpdate] = useState(false);
  const [editRow, setEditRow] = useState(null);
  const [deleteRow, setDeleteRow] = useState(null);
  const toast = getMessage();

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const [list, m, rank] = await Promise.all([listModelPricing(), getPricingMeta(), getModelRanking()]);
      setRows(list || []);
      setMeta(m || null);
      setRanking(rank || []);
    } catch (err) {
      console.error('[pricing] load failed', err);
      toast?.error('价格数据加载失败');
    } finally {
      setLoading(false);
    }
  }, [toast]);

  useEffect(() => { load(); }, [load]);

  // 价格表索引：原始 key 与 bare key（openai/gpt-4o → gpt-4o）均可命中，
  // 用于把「使用中」模型的归一化名称匹配到价格行。
  const priceMap = useMemo(() => {
    const m = new Map();
    for (const r of rows) {
      m.set(r.modelKey.toLowerCase(), r);
      m.set(r.modelKey.split('/').pop().toLowerCase(), r);
    }
    return m;
  }, [rows]);

  // 使用中模型行：实际用过的模型（totalTokens>0）合并价格，按模型名排序；无价格行标「未设置」
  const activeRows = useMemo(() => {
    return ranking
      .filter(r => r.totalTokens > 0)
      .map(r => {
        const p = priceMap.get(r.model.toLowerCase());
        return {
          modelKey: r.model,
          totalTokens: r.totalTokens,
          inputRate: p?.inputRate ?? null,
          outputRate: p?.outputRate ?? null,
          cacheReadRate: p?.cacheReadRate ?? null,
          cacheWriteRate: p?.cacheWriteRate ?? null,
          hasPrice: !!(p && (p.inputRate > 0 || p.outputRate > 0)),
          priceRow: p || null,
        };
      })
      .sort((a, b) => a.modelKey.localeCompare(b.modelKey));
  }, [ranking, priceMap]);

  // 全部视图：搜索过滤
  const filteredAll = useMemo(() => {
    const kw = keyword.trim().toLowerCase();
    if (!kw) return rows;
    return rows.filter(r => r.modelKey.toLowerCase().includes(kw));
  }, [rows, keyword]);

  // 拉取更新
  const handleUpdate = useCallback(async () => {
    setUpdating(true);
    try {
      const result = await updatePricing();
      if (result.error) {
        toast?.error('价格更新失败：' + result.error);
      } else {
        toast?.success('价格更新成功，' + (result.message || `LiteLLM ${result.litellm} 条`));
      }
      await load();
    } catch (err) {
      console.error('[pricing] update failed', err);
      toast?.error('价格更新失败，请重试');
    } finally {
      setUpdating(false);
      setConfirmUpdate(false);
    }
  }, [toast, load]);

  // 打开编辑：有价格 → 编辑；无价格（使用中视图缺失）→ 新增。
  // 「使用中」行带 priceRow（匹配到的价格行）；「全部」行本身即价格行，直接取用。
  const openEdit = useCallback((r) => {
    const p = r.priceRow || r;
    setEditRow({
      modelKey: p.modelKey || r.modelKey,
      inputRate: p.inputRate ?? null,
      outputRate: p.outputRate ?? null,
      cacheReadRate: p.cacheReadRate ?? null,
      cacheWriteRate: p.cacheWriteRate ?? null,
    });
  }, []);

  const handleDelete = useCallback(async () => {
    if (!deleteRow) return;
    try {
      await deleteModelPricing(deleteRow);
      toast?.success(`已删除 ${deleteRow} 的价格`);
      setDeleteRow(null);
      await load();
    } catch (err) {
      console.error('[pricing] delete failed', err);
      toast?.error('删除失败，请重试');
    }
  }, [deleteRow, toast, load]);

  // 两视图共用的价格列
  const rateColumns = [
    { title: '输入（缓存未命中）', dataIndex: 'inputRate', key: 'inputRate', align: 'right', render: (v) => <PriceCell rate={v} /> },
    { title: '输入（缓存命中）', dataIndex: 'cacheReadRate', key: 'cacheReadRate', align: 'right', render: (v) => <PriceCell rate={v} /> },
    { title: '输出', dataIndex: 'outputRate', key: 'outputRate', align: 'right', render: (v) => <PriceCell rate={v} /> },
    { title: '输入（缓存写入）', dataIndex: 'cacheWriteRate', key: 'cacheWriteRate', align: 'right', render: (v) => <PriceCell rate={v} /> },
  ];

  const actionColumn = {
    title: '操作', key: 'actions', width: 92, align: 'center',
    render: (_, r) => {
      const edit = () => openEdit(r);
      return (
        <div className="flex justify-center gap-1">
          <Button size="small" icon={<PencilIcon className="size-3" />} onClick={edit} aria-label="编辑" />
          {r.hasPrice && (
            <Button size="small" danger icon={<Trash2Icon className="size-3" />} onClick={() => setDeleteRow(r.priceRow?.modelKey || r.modelKey)} aria-label="删除" />
          )}
        </div>
      );
    },
  };

  const activeColumns = [
    {
      title: '模型', dataIndex: 'modelKey', key: 'modelKey', ellipsis: true,
      render: (v, r) => (
        <div className="min-w-0">
          <div className="font-medium truncate">{v}</div>
          <div className="text-[11px] text-muted-foreground/70">{compactCN(r.totalTokens)} tokens</div>
        </div>
      ),
    },
    ...rateColumns,
    actionColumn,
  ];

  const allColumns = [
    {
      title: '模型', dataIndex: 'modelKey', key: 'modelKey', width: '30%',
      render: (v) => (
        <div className="min-w-0">
          <div className="font-medium truncate">{bareName(v)}</div>
          <div className="text-[11px] text-muted-foreground/70 truncate" title={v}>{v}</div>
        </div>
      ),
    },
    ...rateColumns,
    actionColumn,
  ];

  return (
    <div className="space-y-4 w-full">
      <SettingField title="模型价格" description="统一价格源（LiteLLM 快照 + 手动调整），下表价格单位均为美元 / 100 万 tokens。拉取更新会覆盖手动修改；修改后立即生效，仅影响之后采集的消息。">
        <div className="flex flex-wrap items-center gap-3">
          <Button type="primary" size="middle" icon={<RefreshCwIcon className="size-4" />} loading={updating} onClick={() => setConfirmUpdate(true)}>
            更新价格
          </Button>
          <span className="text-xs text-muted-foreground">
            {meta
              ? `上次拉取 ${meta.fetchedAt ? formatTs(meta.fetchedAt) : '—'} · 共 ${meta.count} 个模型`
              : '加载中…'}
          </span>
        </div>
      </SettingField>

      <div className="flex flex-wrap items-center gap-3">
        <Segmented
          value={view}
          onChange={setView}
          options={[
            { label: '使用中', value: 'active' },
            { label: '全部', value: 'all' },
          ]}
          size="small"
        />
        {view === 'all' && (
          <Input
            allowClear
            prefix={<SearchIcon className="size-3 text-muted-foreground" />}
            placeholder="搜索模型…"
            value={keyword}
            onChange={e => setKeyword(e.target.value)}
            className="max-w-xs"
          />
        )}
      </div>

      {view === 'active' ? (
        <Table
          rowKey="modelKey"
          size="small"
          loading={loading}
          columns={activeColumns}
          dataSource={activeRows}
          pagination={false}
          scroll={{ x: 'max-content' }}
          locale={{ emptyText: '暂无使用中的模型，采集数据后会自动出现' }}
        />
      ) : (
        <Table
          rowKey="modelKey"
          size="small"
          loading={loading}
          columns={allColumns}
          dataSource={filteredAll}
          pagination={{ pageSize: 50, showSizeChanger: false, showLessItems: true, showTotal: (t) => `共 ${t} 个模型` }}
          scroll={{ x: 'max-content' }}
        />
      )}

      {confirmUpdate && (
        <ConfirmDialog
          title="更新价格数据"
          description="将从 LiteLLM 拉取最新模型定价并覆盖现有数据（含手动修改）。确定执行？"
          confirmText="更新"
          busyText="更新中…"
          busy={updating}
          onConfirm={handleUpdate}
          onCancel={() => setConfirmUpdate(false)}
        />
      )}

      {deleteRow && (
        <ConfirmDialog
          title="删除价格"
          danger
          description={`确定删除「${deleteRow}」的价格记录？之后该模型将按 0 费用计算。`}
          confirmText="删除"
          busyText="删除中…"
          busy={false}
          onConfirm={handleDelete}
          onCancel={() => setDeleteRow(null)}
        />
      )}

      {editRow && (
        <EditPriceModal
          row={editRow}
          onClose={() => setEditRow(null)}
          onSaved={() => { setEditRow(null); load(); }}
        />
      )}
    </div>
  );
}
