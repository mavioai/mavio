// The web UI's copy (docs/clients.md §6.6): the strings of libs/design in
// the browser's language, formatted as their ICU MessageFormat subset
// allows: {name} arguments and one top-level plural with #.
import { type Locale, locales, type MessageKey, messages, sourceLocale } from "@mavio/design/strings";

/** The first of the browser's languages the strings have, else English. */
export function pickLocale(languages: readonly string[]): Locale {
  for (const tag of languages) {
    const lang = tag.toLowerCase();
    if (lang.startsWith("zh")) return "zh-Hans";
    const found = locales.find((l) => l.toLowerCase() === lang || l.toLowerCase() === lang.split("-")[0]);
    if (found) return found;
  }
  return sourceLocale as Locale;
}

/** The locale of this browser. */
export const locale: Locale = pickLocale(typeof navigator === "undefined" ? [] : navigator.languages);

/** The language of discovered names and overviews: the locale's, as ISO 639-1. */
export const discoverLanguage = locale.split("-")[0];

type Args = Record<string, string | number>;

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
export function format(message: string, args: Args = {}, loc: string = locale): string {
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
    const form =
      forms.get(`=${n}`) ?? forms.get(new Intl.PluralRules(loc).select(n)) ?? forms.get("other") ?? "";
    message =
      message.slice(0, plural.index) +
      form.replaceAll("#", new Intl.NumberFormat(loc).format(n)) +
      message.slice(end + 1);
  }
  return message.replace(/\{(\w+)\}/g, (_, k: string) => String(args[k] ?? ""));
}

/** A string of the web UI in the browser's language. */
export function t(key: MessageKey, args?: Args): string {
  return format(messages[locale][key] ?? messages.en[key], args);
}
