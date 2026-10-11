// Reads the strings of strings/, checks them and parses them into messages
// the emitters convert into each platform's format.
//
// A message is ICU MessageFormat limited to what every platform's format can
// express: text, {name} arguments, and at most one top-level
// {count, plural, ...} using # for the count, a message with a plural having
// no other argument (String Catalogs vary a string on a single number).
import { readFileSync, readdirSync } from 'node:fs';
import { parse, TYPE } from '@formatjs/icu-messageformat-parser';

export const SOURCE_LOCALE = 'en';
export const PLATFORMS = ['web', 'apple', 'android', 'windows-linux'];
const PLURAL_FORMS = ['zero', 'one', 'two', 'few', 'many', 'other'];

// Parts are strings (text), { arg } and { count: true } for #.
function parts(elements, key, inPlural) {
  return elements.map((el) => {
    switch (el.type) {
      case TYPE.literal:
        return el.value;
      case TYPE.argument:
        return { arg: el.value };
      case TYPE.pound:
        if (!inPlural) throw new Error(`${key}: # outside a plural`);
        return { count: true };
      default:
        throw new Error(`${key}: unsupported ICU element ${JSON.stringify(el)}`);
    }
  });
}

// parseMessage turns an ICU message into { plural, before, forms, after },
// plural being the count's argument name or undefined, or { parts } for a
// message without a plural.
export function parseMessage(key, text) {
  const elements = parse(text, { requiresOtherClause: true });
  const plurals = elements.filter((el) => el.type === TYPE.plural);
  if (plurals.length === 0) return { parts: parts(elements, key, false) };
  if (plurals.length > 1) throw new Error(`${key}: more than one plural`);
  const at = elements.indexOf(plurals[0]);
  const pl = plurals[0];
  if (pl.pluralType !== 'cardinal' || pl.offset) throw new Error(`${key}: only cardinal plurals without offset`);
  const forms = {};
  for (const [form, opt] of Object.entries(pl.options)) {
    if (!PLURAL_FORMS.includes(form)) throw new Error(`${key}: plural form ${form}; use the CLDR categories`);
    forms[form] = parts(opt.value, key, true);
  }
  return {
    plural: pl.value,
    before: parts(elements.slice(0, at), key, false),
    forms,
    after: parts(elements.slice(at + 1), key, false),
  };
}

// The arguments of a message in order of first appearance, the count first.
export function argsOf(msg) {
  const names = msg.plural ? [msg.plural] : [];
  const visit = (ps) => {
    for (const p of ps) if (p.arg && !names.includes(p.arg)) names.push(p.arg);
  };
  if (msg.parts) visit(msg.parts);
  else {
    visit(msg.before);
    for (const f of Object.values(msg.forms)) visit(f);
    visit(msg.after);
  }
  return names;
}

// The form texts of a message, each with the parts around the plural.
export function formParts(msg) {
  if (msg.parts) return { other: msg.parts };
  return Object.fromEntries(Object.entries(msg.forms).map(([f, ps]) => [f, [...msg.before, ...ps, ...msg.after]]));
}

export function readStrings() {
  const source = JSON.parse(readFileSync(`strings/${SOURCE_LOCALE}.json`, 'utf8'));
  delete source.$comment;
  const locales = [SOURCE_LOCALE];
  const texts = { [SOURCE_LOCALE]: Object.fromEntries(Object.entries(source).map(([k, v]) => [k, v.message])) };
  for (const file of readdirSync('strings').sort()) {
    const locale = file.replace(/\.json$/, '');
    if (locale === SOURCE_LOCALE) continue;
    locales.push(locale);
    texts[locale] = JSON.parse(readFileSync(`strings/${file}`, 'utf8'));
  }

  const entries = Object.entries(source).map(([key, v]) => {
    if (!/^[a-z]+(\.[a-zA-Z]+)+$/.test(key)) throw new Error(`${key}: keys are dotted camelCase`);
    if (!v.description) throw new Error(`${key}: no description`);
    for (const p of v.platforms ?? []) if (!PLATFORMS.includes(p)) throw new Error(`${key}: unknown platform ${p}`);
    return { key, description: v.description, platforms: v.platforms ?? PLATFORMS };
  });

  const messages = {};
  const failures = [];
  for (const locale of locales) {
    const categories = new Intl.PluralRules(locale).resolvedOptions().pluralCategories;
    messages[locale] = {};
    for (const key of Object.keys(texts[locale])) {
      if (!(key in source)) failures.push(`${locale}: ${key} is not in ${SOURCE_LOCALE}.json`);
    }
    for (const { key } of entries) {
      const text = texts[locale][key];
      if (text === undefined) {
        failures.push(`${locale}: ${key} missing`);
        continue;
      }
      try {
        const msg = parseMessage(key, text);
        const want = argsOf(parseMessage(key, texts[SOURCE_LOCALE][key])).sort().join();
        if (argsOf(msg).sort().join() !== want) failures.push(`${locale}: ${key} has arguments ${argsOf(msg)}, want ${want}`);
        if (msg.plural) {
          if (argsOf(msg).length > 1) failures.push(`${locale}: ${key} has arguments besides the plural's count`);
          for (const c of categories) if (!(c in msg.forms)) failures.push(`${locale}: ${key} lacks plural form ${c}`);
          for (const f of Object.keys(msg.forms)) {
            if (!categories.includes(f)) failures.push(`${locale}: ${key} has plural form ${f}, which ${locale} does not use`);
          }
        }
        messages[locale][key] = { text, ...msg };
      } catch (err) {
        failures.push(`${locale}: ${err.message}`);
      }
    }
  }
  if (failures.length) throw new Error(`strings:\n${failures.join('\n')}`);
  return { locales, entries, messages };
}

// The entries a platform uses.
export function entriesFor(strings, platform) {
  return strings.entries.filter((e) => e.platforms.includes(platform));
}
