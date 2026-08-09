/**
 * 模型数据页：按总用量排名的模型卡片列表（当前为纯前端模拟数据）。
 * 卡片背景使用模型 logo，置于背景右方。
 */

import { Card } from 'antd';
import { getModelIconUrl } from '@/lib/iconMap.js';

// 单位格式化：保留两位小数（K/M/B）
function fmtUsage(v) {
  if (v == null) return '—';
  const a = Math.abs(v);
  if (a >= 1e9) return (v / 1e9).toFixed(2) + 'B';
  if (a >= 1e6) return (v / 1e6).toFixed(2) + 'M';
  if (a >= 1e3) return (v / 1e3).toFixed(2) + 'K';
  return String(v);
}

// ── 模拟数据（待后端接口接入后移除） ──────────────────────────
const MOCK_MODELS = [
  { name: 'DeepSeek-V4-Flash', totalTokens: 201_220_000 },
  { name: 'Claude Sonnet 4.6', totalTokens: 158_300_000 },
  { name: 'Gemini 2.5 Pro', totalTokens: 96_450_000 },
  { name: 'GPT-5', totalTokens: 52_120_000 },
  { name: 'Qwen3-Max', totalTokens: 18_760_000 },
  { name: 'Grok 4', totalTokens: 7_320_000 },
  { name: 'Kimi K2', totalTokens: 1_580_000 },
  { name: 'GLM-4.6', totalTokens: 640_000 },
];

// 前三名排名徽章配色：红/蓝/绿三色相分离，明暗主题下均醒目
const RANK_BADGE = [
  'bg-rose-500 text-white',
  'bg-sky-500 text-white',
  'bg-emerald-500 text-white',
];

export default function ModelPage() {
  // 按总用量降序排名
  const ranked = [...MOCK_MODELS].sort((a, b) => b.totalTokens - a.totalTokens);

  return (
    <div className="space-y-4 h-full min-h-0">
      <h2 className="text-sm font-semibold">模型排行</h2>
      <div className="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-3 gap-4">
        {ranked.map((m, i) => (
          <Card
            key={m.name}
            className="relative overflow-hidden"
            styles={{ body: { padding: 16 } }}
          >
            <img
              src={getModelIconUrl(m.name)}
              alt=""
              aria-hidden="true"
              className="pointer-events-none absolute right-3 top-1/2 -translate-y-1/2 h-[88%] w-auto max-w-[55%] object-contain opacity-[0.08] brightness-0 dark:opacity-[0.18] dark:brightness-0 dark:invert"
            />
            <div className="relative z-10 flex flex-col gap-6">
              <div className="flex items-center gap-2">
                <span className={`flex items-center justify-center h-6 min-w-6 px-1.5 rounded-md text-xs font-bold tabular-nums ${RANK_BADGE[i] || 'bg-muted text-muted-foreground'}`}>
                  #{i + 1}
                </span>
                <span className="text-sm font-semibold truncate">{m.name}</span>
              </div>
              <div>
                <div className="text-2xl font-bold tabular-nums leading-none">{fmtUsage(m.totalTokens)}</div>
              </div>
            </div>
          </Card>
        ))}
      </div>
    </div>
  );
}
