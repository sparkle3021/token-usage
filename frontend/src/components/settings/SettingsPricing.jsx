import { useCallback, useEffect, useMemo, useState } from 'react';
import { Button, Form, Input, Modal, Table } from 'antd';
import { PencilIcon, RefreshCwIcon, SearchIcon, Trash2Icon } from 'lucide-react';
import { getMessage } from '@/lib/message.js';
import { formatTs } from '@/lib/formatters.js';
import { listModelPricing, updateModelPricing, deleteModelPricing, getPricingMeta, updatePricing } from '@/api/client.js';
import SettingField from '@/components/common/SettingField.jsx';
import ConfirmDialog from '@/components/common/ConfirmDialog.jsx';

// 费率展示：科学计数法（价格量级 e-6 ~ e-7，普通小数不可读）
function fmtRate(v) {
  if (v == null || v === 0) return '—';
  return v.toExponential(3);
}

// 数字输入校验（支持科学计数法如 1.4e-7）
const rateRules = [
  { required: true, message: '请输入价格' },
  { pattern: /^[+-]?(\d+\.?\d*|\.\d+)([eE][+-]?\d+)?$/, message: '请输入有效数字，如 1.4e-7' },
];

/**
 * 设置-价格管理：model_pricing 表的查看/搜索/编辑/删除 + 拉取更新。
 * 唯一价格源：拉取（UPSERT 覆盖，含手动修改）；改价立即生效，仅影响之后采集。
 */
export default function SettingsPricing() {
  const [rows, setRows] = useState([]);
  const [meta, setMeta] = useState(null);
  const [keyword, setKeyword] = useState('');
  const [loading, setLoading] = useState(false);
  const [updating, setUpdating] = useState(false);
  const [confirmUpdate, setConfirmUpdate] = useState(false);
  const [editRow, setEditRow] = useState(null);
  const [deleteRow, setDeleteRow] = useState(null);
  const [form] = Form.useForm();
  const toast = getMessage();

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const [list, m] = await Promise.all([listModelPricing(), getPricingMeta()]);
      setRows(list || []);
      setMeta(m || null);
    } catch (err) {
      console.error('[pricing] load failed', err);
      toast?.error('价格数据加载失败');
    } finally {
      setLoading(false);
    }
  }, [toast]);

  useEffect(() => { load(); }, [load]);

  const filtered = useMemo(() => {
    const kw = keyword.trim().toLowerCase();
    if (!kw) return rows;
    return rows.filter(r => r.modelKey.toLowerCase().includes(kw));
  }, [rows, keyword]);

  // 拉取更新（确认后执行，成功后刷新列表）
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

  // 打开编辑弹窗并回填当前值（字符串形式便于科学计数法输入）
  const openEdit = useCallback((r) => {
    setEditRow(r);
    form.setFieldsValue({
      inputRate: String(r.inputRate ?? 0),
      outputRate: String(r.outputRate ?? 0),
      cacheReadRate: String(r.cacheReadRate ?? 0),
      cacheWriteRate: String(r.cacheWriteRate ?? 0),
    });
  }, [form]);

  // 保存编辑：写库 + 引擎同步（立即生效）
  const handleSaveEdit = useCallback(async () => {
    if (!editRow) return;
    try {
      const values = await form.validateFields();
      await updateModelPricing({
        modelKey: editRow.modelKey,
        inputRate: parseFloat(values.inputRate) || 0,
        outputRate: parseFloat(values.outputRate) || 0,
        cacheReadRate: parseFloat(values.cacheReadRate) || 0,
        cacheWriteRate: parseFloat(values.cacheWriteRate) || 0,
      });
      toast?.success(`已更新 ${editRow.modelKey} 价格`);
      setEditRow(null);
      await load();
    } catch (err) {
      if (err?.errorFields) return; // 表单校验失败，留在弹窗
      console.error('[pricing] save failed', err);
      toast?.error('保存失败，请重试');
    }
  }, [editRow, form, toast, load]);

  // 删除：手动添加/残留模型清理
  const handleDelete = useCallback(async () => {
    if (!deleteRow) return;
    try {
      await deleteModelPricing(deleteRow.modelKey);
      toast?.success(`已删除 ${deleteRow.modelKey}`);
      setDeleteRow(null);
      await load();
    } catch (err) {
      console.error('[pricing] delete failed', err);
      toast?.error('删除失败，请重试');
    }
  }, [deleteRow, toast, load]);

  const columns = [
    { title: '模型', dataIndex: 'modelKey', key: 'modelKey', ellipsis: true, width: '30%' },
    { title: 'Input $/tok', dataIndex: 'inputRate', key: 'inputRate', render: fmtRate, align: 'right' },
    { title: 'Output $/tok', dataIndex: 'outputRate', key: 'outputRate', render: fmtRate, align: 'right' },
    { title: 'Cache Read $/tok', dataIndex: 'cacheReadRate', key: 'cacheReadRate', render: fmtRate, align: 'right' },
    { title: 'Cache Write $/tok', dataIndex: 'cacheWriteRate', key: 'cacheWriteRate', render: fmtRate, align: 'right' },
    {
      title: '操作', key: 'actions', width: 96, align: 'center',
      render: (_, r) => (
        <div className="flex justify-center gap-1">
          <Button size="small" icon={<PencilIcon className="size-3" />} onClick={() => openEdit(r)} aria-label="编辑" />
          <Button size="small" danger icon={<Trash2Icon className="size-3" />} onClick={() => setDeleteRow(r)} aria-label="删除" />
        </div>
      ),
    },
  ];

  return (
    <div className="space-y-4 max-w-3xl">
      <SettingField title="模型价格" description="唯一价格源（LiteLLM 快照 + 手动调整）。拉取更新会覆盖手动修改；修改后立即生效，仅影响之后采集的消息。">
        <div className="flex flex-wrap items-center gap-3">
          <Button type="primary" icon={<RefreshCwIcon className="size-3" />} loading={updating} onClick={() => setConfirmUpdate(true)}>
            更新价格
          </Button>
          <span className="text-xs text-muted-foreground">
            {meta
              ? `上次拉取 ${meta.fetchedAt ? formatTs(meta.fetchedAt) : '—'} · 共 ${meta.count} 个模型`
              : '加载中…'}
          </span>
        </div>
      </SettingField>

      <Input
        allowClear
        prefix={<SearchIcon className="size-3 text-muted-foreground" />}
        placeholder="搜索模型…"
        value={keyword}
        onChange={e => setKeyword(e.target.value)}
        className="max-w-xs"
      />

      <Table
        rowKey="modelKey"
        size="small"
        loading={loading}
        columns={columns}
        dataSource={filtered}
        pagination={{ pageSize: 50, showSizeChanger: false }}
      />

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
          description={`确定删除「${deleteRow.modelKey}」的价格记录？之后该模型将按 0 费用计算。`}
          confirmText="删除"
          busyText="删除中…"
          busy={false}
          onConfirm={handleDelete}
          onCancel={() => setDeleteRow(null)}
        />
      )}

      <Modal
        open={!!editRow}
        title={`编辑价格：${editRow?.modelKey || ''}`}
        onOk={handleSaveEdit}
        onCancel={() => setEditRow(null)}
        okText="保存"
        cancelText="取消"
        centered
        width={420}
      >
        <Form form={form} layout="vertical" className="pt-2">
          <Form.Item name="inputRate" label="Input（$/tok）" rules={rateRules}>
            <Input placeholder="如 1.4e-7" />
          </Form.Item>
          <Form.Item name="outputRate" label="Output（$/tok）" rules={rateRules}>
            <Input placeholder="如 2.8e-7" />
          </Form.Item>
          <Form.Item name="cacheReadRate" label="Cache Read（$/tok）" rules={rateRules}>
            <Input placeholder="如 2.8e-9" />
          </Form.Item>
          <Form.Item name="cacheWriteRate" label="Cache Write（$/tok）" rules={rateRules}>
            <Input placeholder="如 0" />
          </Form.Item>
        </Form>
      </Modal>
    </div>
  );
}
