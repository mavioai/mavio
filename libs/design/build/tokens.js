// Reads the token sets of tokens/ with Style Dictionary, which resolves
// their references, into plain values the emitters write for each language.
import StyleDictionary from 'style-dictionary';

const BASE = ['palette', 'spacing', 'radius', 'elevation', 'motion'];
export const THEMES = ['dark', 'light'];
export const TYPE_SCALES = ['standard', 'tv'];
export const FORM_FACTORS = ['phone', 'tablet', 'desktop', 'tv'];

// Text color pairs that must meet WCAG AA (4.5:1) in every theme.
const CONTRAST_PAIRS = [
  ['textPrimary', 'background'],
  ['textSecondary', 'background'],
  ['textTertiary', 'background'],
  ['textPrimary', 'surface'],
  ['textSecondary', 'surface'],
  ['textPrimary', 'surfaceOverlay'],
  ['onAccent', 'accent'],
  ['onAccent', 'accentPressed'],
  ['onButton', 'button'],
  ['accentText', 'background'],
  ['accentText', 'surface'],
];

async function resolve(names) {
  const sd = new StyleDictionary({
    source: names.map((n) => `tokens/${n}.json`),
    log: { warnings: 'error', verbosity: 'verbose' },
    platforms: { raw: { transforms: [] } },
  });
  return sd.exportPlatform('raw');
}

// The tokens of a group, keyed by name, with their resolved values.
function leaves(group, convert) {
  const out = {};
  for (const [name, token] of Object.entries(group)) {
    if (name.startsWith('$')) continue;
    out[name] = convert(token.$value, token);
  }
  return out;
}

function color(v) {
  const [r, g, b] = v.components.map((c) => Math.round(c * 255));
  return { r, g, b, a: v.alpha ?? 1 };
}

function px(v) {
  if (v.unit !== 'px') throw new Error(`unit ${v.unit}, want px`);
  return v.value;
}

function ms(v) {
  if (v.unit !== 'ms') throw new Error(`unit ${v.unit}, want ms`);
  return v.value;
}

function luminance({ r, g, b }) {
  const lin = (c) => {
    const s = c / 255;
    return s <= 0.03928 ? s / 12.92 : ((s + 0.055) / 1.055) ** 2.4;
  };
  return 0.2126 * lin(r) + 0.7152 * lin(g) + 0.0722 * lin(b);
}

export function contrast(a, b) {
  const [hi, lo] = [luminance(a), luminance(b)].sort((x, y) => y - x);
  return (hi + 0.05) / (lo + 0.05);
}

export async function readTokens() {
  const base = await resolve(BASE);
  const tokens = {
    palette: Object.fromEntries(
      Object.entries(base.palette)
        .filter(([k]) => !k.startsWith('$'))
        .map(([k, scale]) => [k, leaves(scale, color)]),
    ),
    spacing: leaves(base.spacing, px),
    radius: leaves(base.radius, px),
    elevation: leaves(base.elevation, (v) => ({
      color: color(v.color),
      x: px(v.offsetX),
      y: px(v.offsetY),
      blur: px(v.blur),
      spread: px(v.spread),
    })),
    duration: leaves(base.motion.duration, ms),
    easing: leaves(base.motion.easing, (v) => v),
    themes: {},
    typography: {},
    layout: {},
  };
  for (const theme of THEMES) {
    const t = await resolve(['palette', `color.${theme}`]);
    tokens.themes[theme] = leaves(t.color, color);
  }
  for (const scale of TYPE_SCALES) {
    const t = await resolve([`typography.${scale === 'standard' ? 'default' : scale}`]);
    tokens.typography[scale] = leaves(t.typography, (v) => ({
      size: px(v.fontSize),
      lineHeight: Math.round(px(v.fontSize) * v.lineHeight),
      weight: v.fontWeight,
      tracking: px(v.letterSpacing),
    }));
  }
  for (const ff of FORM_FACTORS) {
    const t = await resolve([`layout.${ff}`]);
    tokens.layout[ff] = leaves(t.layout, (v, token) => (token.$type === 'dimension' ? px(v) : v));
  }
  checkContrast(tokens.themes);
  return tokens;
}

function checkContrast(themes) {
  const failures = [];
  for (const [theme, colors] of Object.entries(themes)) {
    for (const [fg, bg] of CONTRAST_PAIRS) {
      const ratio = contrast(colors[fg], colors[bg]);
      if (ratio < 4.5) failures.push(`${theme}: ${fg} on ${bg} is ${ratio.toFixed(2)}:1, want at least 4.5:1`);
    }
  }
  if (failures.length) throw new Error(`contrast below WCAG AA:\n${failures.join('\n')}`);
}
