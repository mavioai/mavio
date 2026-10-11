// Writes gen/go, the Go package design, for the Windows and Linux app:
// tokens, icon path data and the strings with their formatter.
import { HEADER, num, pascal, write } from './util.js';
import { entriesFor, formParts, SOURCE_LOCALE } from './strings.js';

const PKG = 'package design';
const head = () => `${HEADER('//')}\n\n${PKG}`;

const exported = (k) => pascal(k);

function color({ r, g, b, a }) {
  return `Color{R: ${r}, G: ${g}, B: ${b}, A: ${Math.round(a * 255)}}`;
}

function fields(o, fn, indent = '\t') {
  return Object.entries(o)
    .map(([k, v]) => `${indent}${exported(k)}: ${fn(v)},`)
    .join('\n');
}

function structType(name, doc, keys, type) {
  return `// ${doc}\ntype ${name} struct {\n${keys.map((k) => `\t${exported(k)} ${typeof type === 'function' ? type(k) : type}`).join('\n')}\n}`;
}

function tokensGo(t) {
  const typeStyle = (s) => `TypeStyle{Size: ${s.size}, LineHeight: ${s.lineHeight}, Weight: ${s.weight}, Tracking: ${num(s.tracking)}}`;
  const shadow = (s) => `Shadow{Color: ${color(s.color)}, X: ${s.x}, Y: ${s.y}, Blur: ${s.blur}, Spread: ${s.spread}}`;
  const vars = (doc, prefix, o, fn) =>
    `// ${doc}\nvar (\n${Object.entries(o).map(([k, v]) => `\t${prefix}${exported(k)} = ${fn(v)}`).join('\n')}\n)`;
  const consts = (doc, prefix, o, fn) =>
    `// ${doc}\nconst (\n${Object.entries(o).map(([k, v]) => `\t${prefix}${exported(k)} = ${fn(v)}`).join('\n')}\n)`;
  return `${head()}

import "time"

// Color is an sRGB color with alpha.
type Color struct{ R, G, B, A uint8 }

${structType('Colors', 'Colors are the semantic colors of a theme.', Object.keys(t.themes.dark), 'Color')}

${vars('The themes.', '', Object.fromEntries(Object.entries(t.themes).map(([k, v]) => [`${k}Colors`, v])), (c) => `Colors{\n${fields(c, color, '\t\t')}\n\t}`)}

// TypeStyle is a style of the type scale, in points; the family is the
// platform's system font.
type TypeStyle struct {
	Size, LineHeight float32
	Weight           int
	Tracking         float32
}

${structType('Typography', 'Typography is a type scale.', Object.keys(t.typography.standard), 'TypeStyle')}

${vars('The type scales: standard for desktops, tv for television.', '', Object.fromEntries(Object.entries(t.typography).map(([k, v]) => [`${k}Typography`, v])), (s) => `Typography{\n${fields(s, typeStyle, '\t\t')}\n\t}`)}

${consts('Spacing steps, in points.', 'Spacing', t.spacing, num)}

${consts('Corner radii, in points.', 'Radius', t.radius, num)}

// Shadow is a shadow, in points.
type Shadow struct {
	Color                    Color
	X, Y, Blur, Spread float32
}

${vars('The shadows of raised surfaces.', 'Elevation', t.elevation, shadow)}

${consts('Durations.', 'Duration', t.duration, (v) => `${v} * time.Millisecond`)}

// Easing is a cubic Bézier curve's control points.
type Easing [4]float32

${vars('The easing curves of motion.', 'Easing', t.easing, (e) => `Easing{${e.map(num).join(', ')}}`)}

${structType('Layout', 'Layout holds the layout values of a form factor, in points (FocusScale is a factor).', Object.keys(t.layout.phone), 'float32')}

${vars('The layouts of the form factors.', '', Object.fromEntries(Object.entries(t.layout).map(([k, v]) => [`${k}Layout`, v])), (l) => `Layout{\n${fields(l, num, '\t\t')}\n\t}`)}
`.replace(/\n{3,}/g, '\n\n');
}

function iconsGo(icons) {
  const entries = Object.entries(icons)
    .map(([name, w]) => `\t${JSON.stringify(name)}: {\n\t\tRegular: ${goPaths(w.regular)},\n\t\tFill:    ${goPaths(w.fill)},\n\t},`)
    .join('\n');
  return `${head()}

// IconViewBox is the side of the square view box of every icon's paths.
const IconViewBox = 256

// Icon holds the SVG path data of an icon in its Regular and Fill weights.
type Icon struct{ Regular, Fill []string }

// Icons maps the icons' names to their path data.
var Icons = map[string]Icon{
${entries}
}
`;
}

function goPaths(ps) {
  return `[]string{${ps.map((d) => JSON.stringify(d)).join(', ')}}`;
}

function goParts(ps) {
  return `[]Part{${ps
    .map((p) => (typeof p === 'string' ? `{Text: ${JSON.stringify(p)}}` : p.count ? '{Count: true}' : `{Arg: ${JSON.stringify(p.arg)}}`))
    .join(', ')}}`;
}

const FORMS = { zero: 'Zero', one: 'One', two: 'Two', few: 'Few', many: 'Many', other: 'Other' };

function stringsGo(s) {
  const entries = entriesFor(s, 'windows-linux');
  const locales = s.locales
    .map((locale) => {
      const msgs = entries
        .map(({ key }) => {
          const msg = s.messages[locale][key];
          const forms = Object.entries(formParts(msg))
            .map(([f, ps]) => `plural.${FORMS[f]}: ${goParts(ps)}`)
            .join(', ');
          return `\t\t${JSON.stringify(key)}: {Plural: ${JSON.stringify(msg.plural ?? '')}, Forms: map[plural.Form][]Part{${forms}}},`;
        })
        .join('\n');
      return `\t${JSON.stringify(locale)}: {\n${msgs}\n\t},`;
    })
    .join('\n');
  return `${head()}

import "golang.org/x/text/feature/plural"

// SourceLocale is the locale the strings are written in, and the fallback of
// the others.
const SourceLocale = ${JSON.stringify(SOURCE_LOCALE)}

// Locales are the locales of the strings, the source first.
var Locales = []string{${s.locales.map((l) => JSON.stringify(l)).join(', ')}}

var catalog = map[string]map[string]Message{
${locales}
}
`;
}

const FORMAT_GO = `${head()}

import (
	"fmt"
	"strings"

	"golang.org/x/text/feature/plural"
	"golang.org/x/text/language"
)

// Part is a piece of a message: text, an argument, or the count of its plural.
type Part struct {
	Text  string
	Arg   string
	Count bool
}

// Message is a string of a locale. A message with a plural names its count
// argument in Plural and has a form for each plural category of its locale;
// one without has only the Other form.
type Message struct {
	Plural string
	Forms  map[plural.Form][]Part
}

// Match returns the locale of the strings that best matches the user's
// preferred locales, BCP 47 tags in order of preference.
func Match(preferred ...string) string {
	tags := make([]language.Tag, len(Locales))
	for i, l := range Locales {
		tags[i] = language.Make(l)
	}
	want := make([]language.Tag, 0, len(preferred))
	for _, p := range preferred {
		want = append(want, language.Make(p))
	}
	_, i, _ := language.NewMatcher(tags).Match(want...)
	return Locales[i]
}

// Format returns the string key of locale, one of Locales, with args, falling
// back to SourceLocale; the count of a plural is an integer. It returns key
// when no locale has it.
func Format(locale, key string, args map[string]any) string {
	msg, ok := catalog[locale][key]
	if !ok {
		locale = SourceLocale
		if msg, ok = catalog[locale][key]; !ok {
			return key
		}
	}
	form := plural.Other
	var count int
	if msg.Plural != "" {
		count = toInt(args[msg.Plural])
		form = plural.Cardinal.MatchPlural(language.Make(locale), count, 0, 0, 0, 0)
	}
	parts, ok := msg.Forms[form]
	if !ok {
		parts = msg.Forms[plural.Other]
	}
	var b strings.Builder
	for _, p := range parts {
		switch {
		case p.Count:
			fmt.Fprint(&b, count)
		case p.Arg != "":
			fmt.Fprint(&b, args[p.Arg])
		default:
			b.WriteString(p.Text)
		}
	}
	return b.String()
}

func toInt(v any) int {
	switch n := v.(type) {
	case int:
		return n
	case int64:
		return int(n)
	case int32:
		return int(n)
	case uint:
		return int(n)
	case float64:
		return int(n)
	}
	return 0
}
`;

const DOC_GO = `${HEADER('//')}

// Package design holds the design system of the Mavio clients for the Windows
// and Linux app: the tokens (colors, type scales, spacing, radii, shadows,
// motion, layouts), the icons' path data and the localized strings, generated
// from libs/design by pnpm nx run design:generate.
${PKG}
`;

export function emitGo(tokens, icons, strings) {
  write('gen/go/doc.go', DOC_GO);
  write('gen/go/tokens.go', tokensGo(tokens));
  write('gen/go/icons.go', iconsGo(icons));
  write('gen/go/strings.go', stringsGo(strings));
  write('gen/go/format.go', FORMAT_GO);
}
