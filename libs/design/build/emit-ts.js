// Writes gen/ts: tokens.ts, tokens.css, icons.ts and strings.ts for the web UI.
import { HEADER, hex, write } from './util.js';
import { entriesFor } from './strings.js';

const BREAKPOINTS = { tablet: 600, desktop: 1200 }; // CSS px; below tablet is phone

const json = (v) => JSON.stringify(v, null, 2);

function rgba({ r, g, b, a }) {
  return a === 1 ? hex({ r, g, b, a }) : `rgba(${r}, ${g}, ${b}, ${a})`;
}

function shadow(s) {
  return `${s.x}px ${s.y}px ${s.blur}px ${s.spread}px ${rgba(s.color)}`;
}

function kebab(s) {
  return s.replace(/[A-Z]/g, (c) => '-' + c.toLowerCase());
}

function mapValues(o, f) {
  return Object.fromEntries(Object.entries(o).map(([k, v]) => [k, f(v)]));
}

function tokensTs(t) {
  return `${HEADER('//')}
/** The semantic colors of each theme, as CSS colors. */
export const themes = ${json(mapValues(t.themes, (c) => mapValues(c, rgba)))} as const;

export type Theme = keyof typeof themes;
export type ColorName = keyof (typeof themes)['dark'];

/** The palette the semantic colors reference. */
export const palette = ${json(mapValues(t.palette, (s) => mapValues(s, rgba)))} as const;

/** The type scales, in CSS px; the family is the platform's system font. */
export const typography = ${json(t.typography)} as const;

/** Spacing steps, in CSS px. */
export const spacing = ${json(t.spacing)} as const;

/** Corner radii, in CSS px. */
export const radius = ${json(t.radius)} as const;

/** Shadows, as CSS box-shadow values. */
export const elevation = ${json(mapValues(t.elevation, shadow))} as const;

/** Durations, in milliseconds. */
export const duration = ${json(t.duration)} as const;

/** Easing curves, as cubic Bézier control points. */
export const easing = ${json(t.easing)} as const;

/** Layout values of each form factor, in CSS px (focusScale is a factor). */
export const layout = ${json(t.layout)} as const;

/** The smallest viewport widths, in CSS px, of the tablet and desktop layouts. */
export const breakpoints = ${json(BREAKPOINTS)} as const;
`;
}

function vars(prefix, o, unit = '') {
  return Object.entries(o).map(([k, v]) => `  --mavio-${prefix}-${kebab(k)}: ${v}${unit};`);
}

function themeVars(colors) {
  return vars('color', mapValues(colors, rgba)).join('\n');
}

function layoutVars(l) {
  return Object.entries(l)
    .map(([k, v]) => `  --mavio-layout-${kebab(k)}: ${v}${k === 'focusScale' ? '' : 'px'};`)
    .join('\n');
}

function tokensCss(t) {
  const type = Object.entries(t.typography.standard).flatMap(([k, s]) => [
    `  --mavio-type-${kebab(k)}-size: ${s.size}px;`,
    `  --mavio-type-${kebab(k)}-line-height: ${s.lineHeight}px;`,
    `  --mavio-type-${kebab(k)}-weight: ${s.weight};`,
  ]);
  return `${HEADER('/*', ' */')}
/* The dark theme is the default; the light theme follows the system unless
   data-theme on the root element picks one. */
:root {
  --mavio-font-family: Inter, system-ui, sans-serif;
${themeVars(t.themes.dark)}
${type.join('\n')}
${vars('spacing', t.spacing, 'px').join('\n')}
${vars('radius', t.radius, 'px').join('\n')}
${vars('elevation', mapValues(t.elevation, shadow)).join('\n')}
${vars('duration', t.duration, 'ms').join('\n')}
${vars('easing', mapValues(t.easing, (e) => `cubic-bezier(${e.join(', ')})`)).join('\n')}
${layoutVars(t.layout.phone)}
}

@media (prefers-color-scheme: light) {
  :root:not([data-theme='dark']) {
${themeVars(t.themes.light).replace(/^/gm, '  ')}
  }
}

:root[data-theme='light'] {
${themeVars(t.themes.light)}
}

@media (min-width: ${BREAKPOINTS.tablet}px) {
  :root {
${layoutVars(t.layout.tablet).replace(/^/gm, '  ')}
  }
}

@media (min-width: ${BREAKPOINTS.desktop}px) {
  :root {
${layoutVars(t.layout.desktop).replace(/^/gm, '  ')}
  }
}
`;
}

function iconsTs(icons) {
  return `${HEADER('//')}
/** The side of the square view box of every icon's paths. */
export const iconViewBox = 256;

/** The path data of each icon, in its Regular and Fill weights. */
export const icons = ${json(icons)} as const;

export type IconName = keyof typeof icons;
export type IconWeight = 'regular' | 'fill';
`;
}

function stringsTs(s) {
  const entries = entriesFor(s, 'web');
  const keys = entries.map((e) => e.key);
  const messages = Object.fromEntries(
    s.locales.map((l) => [l, Object.fromEntries(keys.map((k) => [k, s.messages[l][k].text]))]),
  );
  return `${HEADER('//')}
/** The locale the strings are written in, and the fallback of the others. */
export const sourceLocale = 'en';

/** The locales of the strings. */
export const locales = ${json(s.locales)} as const;

export type Locale = (typeof locales)[number];

/** The keys of the strings of the web UI. */
export type MessageKey =
${keys.map((k) => `  | '${k}'`).join('\n')};

/** The ICU MessageFormat strings of each locale. */
export const messages: { readonly [L in Locale]: { readonly [K in MessageKey]: string } } = ${json(messages)};
`;
}

export function emitTs(tokens, icons, strings) {
  write('gen/ts/tokens.ts', tokensTs(tokens));
  write('gen/ts/tokens.css', tokensCss(tokens));
  write('gen/ts/icons.ts', iconsTs(icons));
  write('gen/ts/strings.ts', stringsTs(strings));
}
