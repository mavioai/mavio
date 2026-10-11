// Writes gen/kotlin: MavioTokens.kt (Jetpack Compose), and res/ with the
// icons as vector drawables and the strings for the Android apps.
import { HEADER, hex, num, pascal, snake, write } from './util.js';
import { argsOf, entriesFor, formParts, SOURCE_LOCALE } from './strings.js';
import { VIEWBOX } from './icons.js';

export const KOTLIN_PACKAGE = 'dev.mavio.design';

function color({ r, g, b, a }) {
  const argb = hex({ r, g, b, a: 1 }).slice(1);
  const alpha = Math.round(a * 255).toString(16).padStart(2, '0').toUpperCase();
  return `Color(0x${alpha}${argb})`;
}

const f = (n) => `${num(n)}f`;

function dataClass(name, doc, fields, type, instances, instanceName) {
  const lines = [`/** ${doc} */`, '@Immutable', `data class ${name}(`];
  lines.push(...fields.map((x) => `    val ${x}: ${typeof type === 'function' ? type(x) : type},`), ')');
  for (const [inst, values] of Object.entries(instances)) {
    lines.push('', `val ${instanceName(inst)} = ${name}(`, ...fields.map((x) => `    ${x} = ${values[x]},`), ')');
  }
  return lines.join('\n');
}

function object(name, doc, o, fn, key = (k) => k) {
  return [`/** ${doc} */`, `object ${name} {`, ...Object.entries(o).map(([k, v]) => `    val ${key(k)} = ${fn(v)}`), '}'].join('\n');
}

function mapValues(o, fn) {
  return Object.fromEntries(Object.entries(o).map(([k, v]) => [k, fn(v)]));
}

function tokensKt(t) {
  const typeStyle = (s) =>
    `MavioTypeStyle(size = ${s.size}.sp, lineHeight = ${s.lineHeight}.sp, weight = FontWeight.W${s.weight}, tracking = ${num(s.tracking)}.sp)`;
  const shadow = (s) => `MavioShadow(color = ${color(s.color)}, x = ${s.x}.dp, y = ${s.y}.dp, blur = ${s.blur}.dp, spread = ${s.spread}.dp)`;
  const layoutFields = Object.keys(t.layout.phone);
  const layoutValues = mapValues(t.layout, (l) =>
    Object.fromEntries(Object.entries(l).map(([k, v]) => [k, k === 'focusScale' ? f(v) : `${v}.dp`])),
  );
  return `${HEADER('//')}
package ${KOTLIN_PACKAGE}

import androidx.compose.animation.core.CubicBezierEasing
import androidx.compose.runtime.Immutable
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.Dp
import androidx.compose.ui.unit.TextUnit
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp

${dataClass('MavioColors', 'The semantic colors of a theme.', Object.keys(t.themes.dark), 'Color', mapValues(t.themes, (c) => mapValues(c, color)), (n) => `${pascal(n)}Colors`)}

/** A style of the type scale; the family is the system font. */
@Immutable
data class MavioTypeStyle(
    val size: TextUnit,
    val lineHeight: TextUnit,
    val weight: FontWeight,
    val tracking: TextUnit,
)

${dataClass('MavioTypography', 'A type scale: standard for phones and tablets, tv for Android TV.', Object.keys(t.typography.standard), 'MavioTypeStyle', mapValues(t.typography, (s) => mapValues(s, typeStyle)), (n) => `${n === 'tv' ? 'Tv' : pascal(n)}Typography`)}

${object('MavioSpacing', 'Spacing steps.', t.spacing, (v) => `${v}.dp`, (k) => `s${k}`)}

${object('MavioRadius', 'Corner radii.', t.radius, (v) => `${v}.dp`)}

/** A shadow. */
@Immutable
data class MavioShadow(
    val color: Color,
    val x: Dp,
    val y: Dp,
    val blur: Dp,
    val spread: Dp,
)

${object('MavioElevation', 'The shadows of raised surfaces.', t.elevation, shadow)}

${object('MavioDuration', 'Durations, in milliseconds.', t.duration, (v) => String(v))}

${object('MavioEasing', 'The easing curves of motion.', t.easing, (e) => `CubicBezierEasing(${e.map(f).join(', ')})`)}

${dataClass('MavioLayout', 'The layout values of a form factor (focusScale is a factor).', layoutFields, (x) => (x === 'focusScale' ? 'Float' : 'Dp'), layoutValues, (n) => `${n === 'tv' ? 'Tv' : pascal(n)}Layout`)}
`;
}

function drawable(paths) {
  return `<?xml version="1.0" encoding="utf-8"?>
<!-- ${HEADER('').trim()} -->
<vector xmlns:android="http://schemas.android.com/apk/res/android"
    android:width="24dp"
    android:height="24dp"
    android:viewportWidth="${VIEWBOX}"
    android:viewportHeight="${VIEWBOX}">
${paths.map((d) => `    <path\n        android:fillColor="#FF000000"\n        android:pathData="${d}" />`).join('\n')}
</vector>
`;
}

function escapeXml(s) {
  return s
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/\\/g, '\\\\')
    .replace(/'/g, "\\'")
    .replace(/"/g, '\\"')
    .replace(/^([@?])/, '\\$1');
}

// The text of a message's parts in Android format specifiers: arguments are
// %N$s, the count %d.
function format(ps, args) {
  return ps
    .map((p) => {
      if (typeof p === 'string') return escapeXml(args.length ? p.replace(/%/g, '%%') : p);
      if (p.count) return args.length === 1 ? '%d' : `%${args.indexOf(p.arg ?? args[0]) + 1}$d`;
      return args.length === 1 ? '%s' : `%${args.indexOf(p.arg) + 1}$s`;
    })
    .join('');
}

function stringsXml(s, locale) {
  const lines = ['<?xml version="1.0" encoding="utf-8"?>', `<!-- ${HEADER('').trim()} -->`, '<resources>'];
  for (const { key, description } of entriesFor(s, 'android')) {
    const msg = s.messages[locale][key];
    const args = argsOf(msg);
    const name = snake(key);
    if (locale === SOURCE_LOCALE) lines.push(`    <!-- ${description.replace(/--/g, '—')} -->`);
    if (msg.plural) {
      lines.push(`    <plurals name="${name}">`);
      for (const [form, ps] of Object.entries(formParts(msg))) lines.push(`        <item quantity="${form}">${format(ps, args)}</item>`);
      lines.push('    </plurals>');
    } else {
      lines.push(`    <string name="${name}">${format(msg.parts, args)}</string>`);
    }
  }
  lines.push('</resources>', '');
  return lines.join('\n');
}

// The resource directory qualifier of a BCP 47 locale (values-b+zh+Hans).
function qualifier(locale) {
  return locale === SOURCE_LOCALE ? 'values' : `values-b+${locale.split('-').join('+')}`;
}

export function emitKotlin(tokens, icons, strings) {
  write(`gen/kotlin/${KOTLIN_PACKAGE.replace(/\./g, '/')}/MavioTokens.kt`, tokensKt(tokens));
  for (const [name, weights] of Object.entries(icons)) {
    for (const [weight, paths] of Object.entries(weights)) {
      const file = `ic_${snake(name)}${weight === 'regular' ? '' : '_' + weight}`;
      write(`gen/kotlin/res/drawable/${file}.xml`, drawable(paths));
    }
  }
  for (const locale of strings.locales) write(`gen/kotlin/res/${qualifier(locale)}/strings.xml`, stringsXml(strings, locale));
}
