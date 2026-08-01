import { useEffect, useState } from 'react';
import { Modal, Select } from 'antd';
import { HelpCircleIcon } from 'lucide-react';

/** 各供应商的帮助提示 */
const PROVIDER_HINTS = {
  opencode: {
    title: '如何获取配置信息？',
    steps: [
      { label: 'Auth Cookie', detail: '打开 opencode.ai 并登录 → 按 F12 打开 DevTools → Application 标签 → 左侧 Cookies → 点击 opencode.ai → 复制 auth 的值（Fe26.2** 开头的一长串）' },
      { label: 'Workspace ID', detail: '在 opencode.ai 进入你要查看的工作区 → 地址栏 URL 中 /workspace/ 后面的那串 ID（wrk_ 开头）' },
    ],
  },
  deepseek: {
    title: '如何获取 API Key？',
    steps: [
      { label: 'API Key', detail: '登录 platform.deepseek.com → API Keys → 创建或复制已有的 API Key（sk- 开头）' },
    ],
  },
  bigmodel: {
    title: '如何获取 Token？',
    steps: [
      { label: 'Authorization Token', detail: '打开 www.bigmodel.cn 并登录 → 按 F12 打开 DevTools → Network 标签 → 刷新页面 → 搜索 quota/limit → 复制请求头中 Authorization 的值（eyJ... 开头）' },
    ],
  },
};

export default function QuotaDialog({ open, onOpenChange, schemas, editCfg, onSave }) {
  const [provider, setProvider] = useState('');
  const [displayName, setDisplayName] = useState('');
  const [fields, setFields] = useState({});
  const [saving, setSaving] = useState(false);

  const isEdit = !!editCfg;

  // 初始化表单
  useEffect(() => {
    if (open) {
      if (editCfg) {
        setProvider(editCfg.provider);
        setDisplayName(editCfg.displayName || '');
        try {
          setFields(JSON.parse(editCfg.configJson || '{}'));
        } catch { setFields({}); }
      } else {
        setProvider(schemas?.[0]?.id || '');
        setDisplayName('');
        setFields({});
      }
    }
  }, [open, editCfg, schemas]);

  const currentSchema = schemas?.find(s => s.id === provider);

  const handleFieldChange = (key, value) => {
    setFields(f => ({ ...f, [key]: value }));
  };

  const handleSave = async () => {
    setSaving(true);
    try {
      const cfg = {
        id: editCfg?.id || 0,
        provider,
        plan: currentSchema?.planName || '',
        displayName,
        configJson: JSON.stringify(fields),
      };
      await onSave(cfg);
      onOpenChange(false);
    } finally {
      setSaving(false);
    }
  };

  const canSave = provider && currentSchema;

  return (
    <Modal
      open={open}
      onCancel={() => onOpenChange(false)}
      onOk={handleSave}
      okText={saving ? '保存中…' : '保存'}
      okButtonProps={{ disabled: !canSave || saving, size: 'small' }}
      cancelText="取消"
      cancelButtonProps={{ size: 'small' }}
      title={isEdit ? '编辑用量查询' : '添加用量查询'}
      centered
      width={{ xs: 448, md: 480, lg: 520 }}
    >
      <div className="space-y-4 py-2">
        {/* 供应商选择（编辑时不可改） */}
        <div className="space-y-1.5">
          <label className="text-xs font-medium text-muted-foreground">供应商</label>
          <Select
            value={provider}
            onChange={setProvider}
            disabled={isEdit}
            style={{ width: '100%' }}
            size="small"
            options={(schemas || []).map(s => ({ value: s.id, label: `${s.id} - ${s.planName}` }))}
          />
        </div>

        {/* 动态配置表单 */}
        {currentSchema?.fields?.map(f => (
          <div key={f.key} className="space-y-1.5">
            <label className="text-xs font-medium text-muted-foreground">{f.label}</label>
            <input
              type={f.type === 'password' ? 'password' : 'text'}
              value={fields[f.key] || ''}
              onChange={e => handleFieldChange(f.key, e.target.value)}
              placeholder={f.placeholder}
              className="w-full h-8 px-2 text-xs rounded-lg border border-input bg-transparent outline-none focus-visible:border-ring font-mono"
            />
          </div>
        ))}

        {/* 帮助提示 */}
        {!isEdit && PROVIDER_HINTS[provider] && (
          <details className="group">
            <summary className="flex items-center gap-1 text-xs text-muted-foreground cursor-pointer hover:text-foreground transition-colors">
              <HelpCircleIcon className="size-3.5" />
              如何获取这些信息？
            </summary>
            <div className="mt-2 p-3 rounded-lg bg-muted/50 space-y-2 text-xs text-muted-foreground">
              {PROVIDER_HINTS[provider].steps.map((s, i) => (
                <div key={i}>
                  <span className="font-medium text-foreground">{i + 1}. {s.label}：</span>
                  {s.detail}
                </div>
              ))}
            </div>
          </details>
        )}

        {/* 别名 */}
        <div className="space-y-1.5">
          <label className="text-xs font-medium text-muted-foreground">
            别名
            <span className="text-muted-foreground/60 ml-1">（可选，不填自动编号）</span>
          </label>
          <input
            value={displayName}
            onChange={e => setDisplayName(e.target.value)}
            placeholder="例：公司项目"
            className="w-full h-8 px-2 text-xs rounded-lg border border-input bg-transparent outline-none focus-visible:border-ring"
          />
        </div>
      </div>
    </Modal>
  );
}
