import { getSourceIconUrl } from '@/lib/iconMap.js';

export default function SourceIcon({ name, className = 'w-3.5 h-3.5' }) {
  const url = getSourceIconUrl(name);
  if (!url) return null;
  return <img src={url} alt={name} className={`${className} shrink-0`} />;
}
