// The website's copy: the strings of libs/design marked for the website, in
// the language the browser asks for, formatted as their ICU MessageFormat
// subset allows: {name} arguments and one top-level plural with #. The
// pages and the e-mails share them.
import { type Locale, locales, type MessageKey, messages, sourceLocale } from "@mavio/design/website-strings";

export type { Locale, MessageKey };

/** The first of languages the strings have, else English. */
export function pickLocale(languages: readonly string[]): Locale {
  for (const tag of languages) {
    const lang = tag.toLowerCase();
    if (lang.startsWith("zh")) return "zh-Hans";
    const found = locales.find((l) => l.toLowerCase() === lang || l.toLowerCase() === lang.split("-")[0]);
    if (found) return found;
  }
  return sourceLocale as Locale;
}

/** The locale an Accept-Language header asks for first, its weights in order. */
export function localeOf(acceptLanguage: string | null | undefined): Locale {
  const tags = (acceptLanguage ?? "")
    .split(",")
    .map((part, i) => {
      const [tag, ...params] = part.trim().split(";");
      const q = params.map((p) => /^\s*q=([\d.]+)/.exec(p)?.[1]).find(Boolean);
      return { tag: tag.trim(), q: q === undefined ? 1 : Number(q), i };
    })
    .filter((x) => x.tag && x.tag !== "*" && x.q > 0)
    .sort((a, b) => b.q - a.q || a.i - b.i);
  return pickLocale(tags.map((x) => x.tag));
}

/** The locale of l when the strings have it, else English. */
export function asLocale(l: string | undefined): Locale {
  return locales.find((x) => x === l) ?? (sourceLocale as Locale);
}

export type Args = Record<string, string | number>;

/** The end of the brace group starting at start, or -1. */
function closing(s: string, start: number): number {
  let depth = 0;
  for (let i = start; i < s.length; i++) {
    if (s[i] === "{") depth++;
    else if (s[i] === "}" && --depth === 0) return i;
  }
  return -1;
}

/** Formats a message with its arguments. */
export function format(message: string, args: Args = {}, loc: string = sourceLocale): string {
  const plural = /\{(\w+),\s*plural,/.exec(message);
  if (plural) {
    const end = closing(message, plural.index);
    const body = message.slice(plural.index + plural[0].length, end);
    const forms = new Map<string, string>();
    for (let i = 0; i < body.length; ) {
      const open = body.indexOf("{", i);
      if (open < 0) break;
      const close = closing(body, open);
      forms.set(body.slice(i, open).trim(), body.slice(open + 1, close));
      i = close + 1;
    }
    const n = Number(args[plural[1]] ?? 0);
    const form = forms.get(`=${n}`) ?? forms.get(new Intl.PluralRules(loc).select(n)) ?? forms.get("other") ?? "";
    message = message.slice(0, plural.index) + form.replaceAll("#", new Intl.NumberFormat(loc).format(n)) + message.slice(end + 1);
  }
  return message.replace(/\{(\w+)\}/g, (_, k: string) => String(args[k] ?? ""));
}

/** Translator returns the strings of locale. */
export function translator(locale: Locale) {
  return (key: MessageKey, args?: Args): string => format(messages[locale][key] ?? messages.en[key], args, locale);
}

/** T formats a string of the website in one locale. */
export type T = ReturnType<typeof translator>;
