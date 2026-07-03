import claudeIcon from '../assets/icons/claude.svg';
import geminiIcon from '../assets/icons/gemini.svg';
import gptIcon from '../assets/icons/gpt.svg';
import hermesIcon from '../assets/icons/hermes.svg';
import openclawIcon from '../assets/icons/openclaw.svg';
import opencodeIcon from '../assets/icons/opencode.svg';

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

export default function SourceIcon({ name, className = 'w-3.5 h-3.5' }) {
  const url = getSourceIconUrl(name);
  if (!url) return null;
  return <img src={url} alt={name} className={`${className} shrink-0`} />;
}
