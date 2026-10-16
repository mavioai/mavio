// Writes gen/swift: MavioTokens.swift (SwiftUI), Icons.xcassets,
// Localizable.xcstrings and MavioStrings.swift, its typed accessors, for
// the Apple apps.
import { HEADER, num, pascal, write } from './util.js';
import { argsOf, entriesFor, formParts, SOURCE_LOCALE } from './strings.js';
import { VIEWBOX } from './icons.js';

const WEIGHTS = { 400: '.regular', 500: '.medium', 600: '.semibold', 700: '.bold' };

function color({ r, g, b, a }) {
  return `Color(.sRGB, red: ${num(r / 255)}, green: ${num(g / 255)}, blue: ${num(b / 255)}, opacity: ${num(a)})`;
}

function struct(name, doc, fields, type, instances) {
  const lines = [`/// ${doc}`, `public struct ${name}: Sendable {`];
  for (const f of fields) lines.push(`    public let ${f}: ${type}`);
  for (const [inst, values] of Object.entries(instances)) {
    const args = fields.map((f) => `        ${f}: ${values[f]}`).join(',\n');
    lines.push('', `    public static let ${inst} = ${name}(`, args, '    )');
  }
  lines.push('}');
  return lines.join('\n');
}

function mapValues(o, f) {
  return Object.fromEntries(Object.entries(o).map(([k, v]) => [k, f(v)]));
}

function tokensSwift(t) {
  const typeStyle = (s) =>
    `MavioTypeStyle(size: ${s.size}, lineHeight: ${s.lineHeight}, weight: ${WEIGHTS[s.weight]}, tracking: ${num(s.tracking)})`;
  const shadow = (s) => `MavioShadow(color: ${color(s.color)}, x: ${s.x}, y: ${s.y}, blur: ${s.blur}, spread: ${s.spread})`;
  const constants = (name, doc, o, type, f = (v) => num(v), key = (k) => k) =>
    [`/// ${doc}`, `public enum ${name} {`, ...Object.entries(o).map(([k, v]) => `    public static let ${key(k)}: ${type} = ${f(v)}`), '}'].join('\n');
  return `${HEADER('//')}
import SwiftUI

${struct('MavioColors', 'The semantic colors of a theme.', Object.keys(t.themes.dark), 'Color', mapValues(t.themes, (c) => mapValues(c, color)))}

/// A style of the type scale; the family is the system font.
public struct MavioTypeStyle: Sendable {
    public let size: CGFloat
    public let lineHeight: CGFloat
    public let weight: Font.Weight
    public let tracking: CGFloat
}

${struct('MavioTypography', 'A type scale: standard for phones, tablets and the Mac, tv for Apple TV.', Object.keys(t.typography.standard), 'MavioTypeStyle', mapValues(t.typography, (s) => mapValues(s, typeStyle)))}

${constants('MavioSpacing', 'Spacing steps, in points.', t.spacing, 'CGFloat', num, (k) => `s${k}`)}

${constants('MavioRadius', 'Corner radii, in points.', t.radius, 'CGFloat')}

/// A shadow, in points.
public struct MavioShadow: Sendable {
    public let color: Color
    public let x: CGFloat
    public let y: CGFloat
    public let blur: CGFloat
    public let spread: CGFloat
}

${constants('MavioElevation', 'The shadows of raised surfaces.', t.elevation, 'MavioShadow', shadow)}

${constants('MavioDuration', 'Durations, in seconds.', t.duration, 'TimeInterval', (v) => num(v / 1000))}

/// An easing curve, as cubic Bézier control points.
public struct MavioEasing: Sendable {
    public let x1, y1, x2, y2: Double

    /// An animation along the curve.
    public func animation(duration: TimeInterval) -> Animation {
        .timingCurve(x1, y1, x2, y2, duration: duration)
    }
}

${constants('MavioEasings', 'The easing curves of motion.', t.easing, 'MavioEasing', ([x1, y1, x2, y2]) => `MavioEasing(x1: ${num(x1)}, y1: ${num(y1)}, x2: ${num(x2)}, y2: ${num(y2)})`)}

${struct('MavioLayout', 'The layout values of a form factor, in points (focusScale is a factor).', Object.keys(t.layout.phone), 'CGFloat', mapValues(t.layout, (l) => mapValues(l, num)))}
`;
}

function svg(paths) {
  const body = paths.map((d) => `<path d="${d}"/>`).join('');
  return `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 ${VIEWBOX} ${VIEWBOX}" width="24" height="24">${body}</svg>\n`;
}

function emitIcons(icons) {
  const root = 'gen/swift/Icons.xcassets';
  write(`${root}/Contents.json`, JSON.stringify({ info: { author: 'xcode', version: 1 } }, null, 2) + '\n');
  for (const [name, weights] of Object.entries(icons)) {
    for (const [weight, paths] of Object.entries(weights)) {
      const asset = weight === 'regular' ? name : `${name}${pascal(weight)}`;
      write(`${root}/${asset}.imageset/${asset}.svg`, svg(paths));
      const contents = {
        images: [{ filename: `${asset}.svg`, idiom: 'universal' }],
        info: { author: 'xcode', version: 1 },
        properties: { 'preserves-vector-representation': true, 'template-rendering-intent': 'template' },
      };
      write(`${root}/${asset}.imageset/Contents.json`, JSON.stringify(contents, null, 2) + '\n');
    }
  }
}

// The text of a message's parts in Apple format specifiers: arguments are
// %@ and the count %lld, numbered (%N$@) when there are several, the
// count first.
function format(ps, args) {
  const at = (arg) => (args.length === 1 ? '%' : `%${args.indexOf(arg) + 1}$`);
  return ps
    .map((p) => {
      if (typeof p === 'string') return p.replace(/%/g, '%%');
      if (p.count) return `${at(args[0])}lld`;
      return `${at(p.arg)}@`;
    })
    .join('');
}

const unit = (value) => ({ stringUnit: { state: 'translated', value } });

function xcstrings(s) {
  const strings = {};
  for (const { key, description } of entriesFor(s, 'apple')) {
    const localizations = {};
    for (const locale of s.locales) {
      const msg = s.messages[locale][key];
      const args = argsOf(msg);
      if (msg.plural) {
        const plural = Object.fromEntries(Object.entries(formParts(msg)).map(([f, ps]) => [f, unit(format(ps, args))]));
        localizations[locale] = { variations: { plural } };
      } else {
        localizations[locale] = unit(format(msg.parts, args));
      }
    }
    strings[key] = { comment: description, extractionState: 'manual', localizations };
  }
  return JSON.stringify({ sourceLanguage: SOURCE_LOCALE, strings, version: '1.0' }, null, 2) + '\n';
}

const camel = (key) => key.split(/[._]/).map((w, i) => (i ? w[0].toUpperCase() + w.slice(1) : w)).join('');

// An accessor per string: a property for a constant one, a function taking
// its arguments for one with arguments, the count as an Int.
function stringsSwift(s) {
  const lines = [
    HEADER('//'),
    'import Foundation',
    '',
    '/// The strings of the design system in the user\'s language.',
    'public enum MavioStrings {',
    '    /// The bundle of the string catalog.',
    '    public static let bundle = Bundle.module',
  ];
  for (const { key, description } of entriesFor(s, 'apple')) {
    const msg = s.messages[SOURCE_LOCALE][key];
    const args = argsOf(msg);
    const lookup = `NSLocalizedString(${JSON.stringify(key)}, bundle: .module, comment: "")`;
    lines.push('', `    /// ${description}`);
    if (args.length === 0) {
      lines.push(`    public static var ${camel(key)}: String { ${lookup} }`);
      continue;
    }
    const params = args.map((a) => `${camel(a)}: ${msg.plural === a ? 'Int' : 'String'}`).join(', ');
    lines.push(
      `    public static func ${camel(key)}(${params}) -> String {`,
      `        String.localizedStringWithFormat(${lookup}, ${args.map(camel).join(', ')})`,
      '    }',
    );
  }
  lines.push('}', '');
  return lines.join('\n');
}

export function emitSwift(tokens, icons, strings) {
  write('gen/swift/MavioTokens.swift', tokensSwift(tokens));
  emitIcons(icons);
  write('gen/swift/Localizable.xcstrings', xcstrings(strings));
  write('gen/swift/MavioStrings.swift', stringsSwift(strings));
}
