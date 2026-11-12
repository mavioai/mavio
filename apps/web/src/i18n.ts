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

/** Where this browser's settings are kept (prefs.ts), the language chosen among them. */
export const prefsKey = "mavio.prefs";

/**
 * The cookie app.mavio.dev keeps its language in, shared with mavio.dev's
 * pages (docs/mavio-dev.md §2), so that choosing it on either changes both.
 */
const localeCookie = "mavio-locale";

/** On app.mavio.dev, the site's domain the language cookie is shared on; elsewhere undefined. */
const sharedDomain = ((): string | undefined => {
  const api = import.meta.env.VITE_MAVIO_DEV_URL as string | undefined;
  if (!api || typeof location === "undefined") return undefined;
  const site = new URL(api).hostname;
  return location.hostname.endsWith(`.${site}`) ? site : undefined;
})();

/** On app.mavio.dev, the language the shared cookie holds, "system" without one; elsewhere undefined. */
export const sharedLanguage: Locale | "system" | undefined = (() => {
  if (!sharedDomain) return undefined;
  const v = /(?:^|;\s*)mavio-locale=([^;]*)/.exec(document.cookie)?.[1];
  return locales.find((l) => l === v) ?? "system";
})();

/** keepLanguage keeps the language chosen in the shared cookie on app.mavio.dev. */
export function keepLanguage(language: Locale | "system") {
  if (!sharedDomain) return;
  const attrs = `; path=/; domain=${sharedDomain}; samesite=lax; secure`;
  document.cookie = language === "system" ? `${localeCookie}=; max-age=0${attrs}` : `${localeCookie}=${language}; max-age=31536000${attrs}`;
}

/** The language chosen: on app.mavio.dev the shared cookie's, else the Appearance settings'; undefined to follow the system's. */
function chosenLocale(): Locale | undefined {
  if (sharedLanguage) return sharedLanguage === "system" ? undefined : sharedLanguage;
  try {
    const { language } = JSON.parse(localStorage.getItem(prefsKey) ?? "{}") as { language?: string };
    return locales.find((l) => l === language);
  } catch {
    return undefined;
  }
}

/** The locale of this browser: the language chosen, else the first of the system's the strings have. */
export const locale: Locale = chosenLocale() ?? pickLocale(typeof navigator === "undefined" ? [] : navigator.languages);

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
