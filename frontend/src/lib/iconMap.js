/**
 * 图标与颜色映射。
 * 根据来源名称或模型名称返回对应 SVG 图标 URL 和 oklch 颜色值。
 */

import claudeIcon from '../assets/icons/claude.svg';
import geminiIcon from '../assets/icons/gemini.svg';
import gptIcon from '../assets/icons/gpt.svg';
import hermesIcon from '../assets/icons/hermes.svg';
import openclawIcon from '../assets/icons/openclaw.svg';
import opencodeIcon from '../assets/icons/opencode.svg';

import modelClaude from '../assets/models/claude.svg';
import modelDeepseek from '../assets/models/deepseek.svg';
import modelDoubao from '../assets/models/doubao.svg';
import modelGemini from '../assets/models/gemini.svg';
import modelGrok from '../assets/models/grok.svg';
import modelHunyuan from '../assets/models/hunyuan.svg';
import modelKimi from '../assets/models/kimi.svg';
import modelMinimax from '../assets/models/minimax.svg';
import modelOllama from '../assets/models/ollama.svg';
import modelOpenai from '../assets/models/gpt.svg';
import modelQwen from '../assets/models/qwen.svg';
import modelXiaomi from '../assets/models/xiaomimimo.svg';
import modelZhipu from '../assets/models/zhipu.svg';

const SOURCE_ICONS = {
  'Claude Code': claudeIcon,
  'claude-desktop': claudeIcon,
  'Codex CLI': gptIcon,
  'Gemini CLI': geminiIcon,
  'Hermes Agent': hermesIcon,
  'OpenClaw': openclawIcon,
  'OpenCode': opencodeIcon,
};

export function getSourceIconUrl(name) {
  return SOURCE_ICONS[name] || null;
}

const MODEL_ICON_RULES = [
  { test: n => /^claude\b/i.test(n), icon: modelClaude },
  { test: n => /^deepseek\b/i.test(n), icon: modelDeepseek },
  { test: n => /^doubao\b/i.test(n), icon: modelDoubao },
  { test: n => /^gemini\b/i.test(n), icon: modelGemini },
  { test: n => /^grok\b/i.test(n), icon: modelGrok },
  { test: n => /^hunyuan\b|混元/i.test(n), icon: modelHunyuan },
  { test: n => /^kimi\b/i.test(n), icon: modelKimi },
  { test: n => /^minimax\b/i.test(n), icon: modelMinimax },
  { test: n => /^ollama\b/i.test(n), icon: modelOllama },
  { test: n => /^(gpt|o1|o3|chatgpt)\b/i.test(n), icon: modelOpenai },
  { test: n => /^qwen\b|通义/i.test(n), icon: modelQwen },
  { test: n => /^mimo\b|^xiaomi\b|小米/i.test(n), icon: modelXiaomi },
  { test: n => /^glm\b|^zhipu\b|智谱/i.test(n), icon: modelZhipu },
];

export function getModelIconUrl(name) {
  if (!name) return null;
  for (const { test, icon } of MODEL_ICON_RULES) {
    if (test(name)) return icon;
  }
  return null;
}

const PALETTE = {
  'Claude Code': 'oklch(0.654 0.147 38.2)',
  'claude-desktop': 'oklch(0.654 0.147 38.2)',
  'Codex CLI': 'oklch(0.472 0.282 270.1)',
  'Hermes Agent': 'oklch(0.58 0.14 240)',
  'OpenClaw': 'oklch(0.549 0.187 25.9)',
  'OpenCode': 'oklch(0.65 0.216 271.6)',
  'Gemini CLI': 'oklch(0.529 0.223 295.0)',
  'Cursor': 'oklch(0.68 0.12 220)',
  'Aider': 'oklch(0.65 0.13 155)',
  'Amp': 'oklch(0.62 0.16 20)',
};

const FALLBACK = [
  'oklch(0.55 0.16 265)', 'oklch(0.60 0.15 295)', 'oklch(0.65 0.11 200)',
  'oklch(0.72 0.14 75)',  'oklch(0.65 0.12 150)', 'oklch(0.62 0.16 20)',
  'oklch(0.58 0.14 240)', 'oklch(0.63 0.14 330)', 'oklch(0.68 0.12 220)',
];

export function getSourceColor(name) {
  if (!name) return 'var(--muted)';
  if (PALETTE[name]) return PALETTE[name];
  let h = 0;
  for (let i = 0; i < name.length; i++) h = (h * 31 + name.charCodeAt(i)) >>> 0;
  return FALLBACK[h % FALLBACK.length];
}
