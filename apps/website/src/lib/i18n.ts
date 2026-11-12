// The website's copy: the strings of libs/design marked for the website, in
// the language the browser asks for, formatted as their ICU MessageFormat
// subset allows: {name} arguments and one top-level plural with #. The
// pages and the e-mails share them.
import { type Locale, locales, type MessageKey, messages, sourceLocale } from "@mavio/design/website-strings";

export type { Locale, MessageKey };

/** The first of languages the strings have, if any. */
export function matchLocale(languages: readonly string[]): Locale | undefined {
  for (const tag of languages) {
    const lang = tag.toLowerCase();
    if (lang.startsWith("zh")) return "zh-Hans";
    const found = locales.find((l) => l.toLowerCase() === lang || l.toLowerCase() === lang.split("-")[0]);
    if (found) return found;
  }
  return undefined;
}

/** The first of languages the strings have, else English. */
export function pickLocale(languages: readonly string[]): Locale {
  return matchLocale(languages) ?? (sourceLocale as Locale);
}

/**
 * The cookie holding the language chosen on mavio.dev or app.mavio.dev,
 * shared by the site and its subdomains; without it the pages follow
 * Accept-Language.
 */
export const localeCookie = "mavio-locale";

/** How long the language chosen is kept: a year. */
export const localeCookieAge = 365 * 24 * 60 * 60;

/** The value of the cookie named name in a Cookie header. */
export function cookieValue(cookies: string | null | undefined, name: string): string | undefined {
  for (const part of (cookies ?? "").split(";")) {
    const i = part.indexOf("=");
    if (i > 0 && part.slice(0, i).trim() === name) return decodeURIComponent(part.slice(i + 1).trim());
  }
  return undefined;
}

/** The locale a request's language cookie holds, if it is one of the strings'. */
export function chosenLocale(cookies: string | null | undefined): Locale | undefined {
  const v = cookieValue(cookies, localeCookie);
  return locales.find((l) => l === v);
}

/**
 * The locale a request is answered in: the OpenID Connect ui_locales its
 * address carries, a client's language passed to /authorize; else the one
 * chosen in the cookie; else its Accept-Language.
 */
export function requestLocale(req: { uiLocales?: string | null; cookies?: string | null; acceptLanguage?: string | null }): Locale {
  return matchLocale((req.uiLocales ?? "").split(" ").filter(Boolean)) ?? chosenLocale(req.cookies) ?? localeOf(req.acceptLanguage);
}

/** The domain the language cookie is shared on: the site's and its subdomains', or the host alone for an address or localhost. */
export function cookieDomain(host: string): string | undefined {
  const h = host.replace(/:\d+$/, "");
  return /^[\d.]+$|^\[|^localhost$|\.localhost$/.test(h) || !h.includes(".") ? undefined : h.replace(/^(www|app)\./, "");
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
