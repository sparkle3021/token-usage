/**
 * oklch → sRGB hex 转换：echarts canvas 渲染专用。
 * zrender 的颜色解析不支持 oklch（parse 返回 undefined），
 * 而 DOM/CSS 场景（KPI/Heatmap/SourceBadge 等）可直接用 oklch，
 * 因此仅在组装 echarts option 时经由此函数把 oklch 转为 hex。
 * 算法参考 Björn Ottosson 的 oklab → sRGB 矩阵。
 */

// oklch 色值形如 "oklch(L C H)" 或 "oklch(L C H / a)"
const OKLCH_RE = /oklch\(\s*([\d.]+)\s+([\d.]+)\s+([\d.]+)(?:\s*\/\s*([\d.]+))?\s*\)/i;

function oklabToSrgbLinear(L, a, b) {
  const l_ = L + 0.3963377774 * a + 0.2158037573 * b;
  const m_ = L - 0.1055613458 * a - 0.0638541728 * b;
  const s_ = L - 0.0894841775 * a - 1.291485548 * b;
  const l = l_ ** 3, m = m_ ** 3, s = s_ ** 3;
  return {
    r: 4.0767416621 * l - 3.3077115913 * m + 0.2309699292 * s,
    g: -1.2684380046 * l + 2.6097574011 * m - 0.3413193965 * s,
    b: -0.0041960863 * l - 0.7034186147 * m + 1.707614701 * s,
  };
}

function toSrgb(c) {
  const c1 = Math.max(0, Math.min(1, c));
  return c1 <= 0.0031308 ? 12.92 * c1 : 1.055 * c1 ** (1 / 2.4) - 0.055;
}

function hex2(v) {
  return Math.round(v * 255).toString(16).padStart(2, '0');
}

/**
 * @param {string} color oklch 字符串
 * @returns {string} #rrggbb；非 oklch 或解析失败原样返回
 */
export function oklchToHex(color) {
  if (typeof color !== 'string') return color;
  const m = color.match(OKLCH_RE);
  if (!m) return color;
  const L = parseFloat(m[1]);
  const C = parseFloat(m[2]);
  const Hdeg = parseFloat(m[3]);
  const H = (Hdeg * Math.PI) / 180;
  const a = C * Math.cos(H);
  const b = C * Math.sin(H);
  const { r, g, b: bv } = oklabToSrgbLinear(L, a, b);
  return `#${hex2(toSrgb(r))}${hex2(toSrgb(g))}${hex2(toSrgb(bv))}`;
}
