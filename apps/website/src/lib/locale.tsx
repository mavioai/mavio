// The pages' language: on the server, the request's (requestLocale), a
// ui_locales in its address being kept in the language cookie; in the
// browser, the one the server rendered the page in, which <html lang>
// carries, so that hydrating renders the same text, until the language is
// switched.
import { Code, ConnectError } from "@connectrpc/connect";
import { locales, messages } from "@mavio/design/website-strings";
import { createIsomorphicFn } from "@tanstack/react-start";
import { getRequestHeader, getRequestUrl, setCookie } from "@tanstack/react-start/server";
import { useRouteContext, useRouter } from "@tanstack/react-router";
import { Fragment, type ReactNode, useCallback } from "react";

import {
  type Args,
  asLocale,
  chosenLocale,
  cookieDomain,
  format,
  type Locale,
  localeCookie,
  localeCookieAge,
  type MessageKey,
  pickLocale,
  requestLocale,
  translator,
  type T,
} from "./i18n.ts";

/** PageLocale is the page's language and the one chosen, undefined when it follows the browser's. */
export interface PageLocale {
  locale: Locale;
  chosen?: Locale;
}

/** currentLocale returns the locale of the page being rendered. */
export const currentLocale = createIsomorphicFn()
  .server((): PageLocale => {
    const url = getRequestUrl();
    const cookies = getRequestHeader("cookie");
    const locale = requestLocale({ uiLocales: url.searchParams.get("ui_locales"), cookies, acceptLanguage: getRequestHeader("accept-language") });
    let chosen = chosenLocale(cookies);
    // A client's language passed to /authorize stays for the pages after.
    if (url.searchParams.has("ui_locales") && locale !== chosen) {
      setCookie(localeCookie, locale, { path: "/", domain: cookieDomain(url.host), maxAge: localeCookieAge, sameSite: "lax", secure: url.protocol === "https:" });
      chosen = locale;
    }
    return { locale, chosen };
  })
  .client((): PageLocale => ({ locale: asLocale(document.documentElement.lang), chosen: chosenLocale(document.cookie) }));

/** useLocale returns the page's locale. */
export function useLocale(): Locale {
  return useRouteContext({ from: "__root__", select: (c) => c.locale });
}

/** useChosenLocale returns the language chosen, undefined when the pages follow the browser's. */
export function useChosenLocale(): Locale | undefined {
  return useRouteContext({ from: "__root__", select: (c) => c.chosen });
}

/**
 * useSetLocale returns the function switching the pages' language, or back
 * to the browser's with undefined: it keeps the choice in the language
 * cookie, which app.mavio.dev shares, and renders the page again in it.
 */
export function useSetLocale() {
  const router = useRouter();
  return useCallback(
    async (l: Locale | undefined) => {
      const domain = cookieDomain(location.host);
      const attrs = `; path=/${domain ? `; domain=${domain}` : ""}; samesite=lax${location.protocol === "https:" ? "; secure" : ""}`;
      document.cookie = l ? `${localeCookie}=${l}; max-age=${localeCookieAge}${attrs}` : `${localeCookie}=; max-age=0${attrs}`;
      document.documentElement.lang = l ?? pickLocale(navigator.languages);
      await router.invalidate();
    },
    [router],
  );
}

/** The languages the pages can be switched to, each named in itself. */
export const localeNames: readonly { locale: Locale; name: string }[] = locales.map((l) => ({
  locale: l,
  name: new Intl.DisplayNames([l], { type: "language" }).of(l) ?? l,
}));

/** useT returns the page's strings. */
export function useT(): T {
  return translator(useLocale());
}

/**
 * useRich returns a string with some of its arguments as elements, such as
 * a server's name set in its own type.
 */
export function useRich() {
  const locale = useLocale();
  return (key: MessageKey, args: Record<string, ReactNode>): ReactNode => {
    const plain: Args = {};
    for (const k of Object.keys(args)) plain[k] = `\u0000${k}\u0000`;
    const text = format(messages[locale][key] ?? messages.en[key], plain, locale);
    return text.split("\u0000").map((part, i) => <Fragment key={i}>{i % 2 ? args[part] : part}</Fragment>);
  };
}

/** The codes of better-auth's errors the pages word themselves. */
const authErrors: Record<string, MessageKey> = {
  INVALID_EMAIL_OR_PASSWORD: "site.error.credentials",
  INVALID_USERNAME_OR_PASSWORD: "site.error.credentials",
  EMAIL_NOT_VERIFIED: "site.signIn.unverified",
  USER_ALREADY_EXISTS: "site.error.exists",
  USER_ALREADY_EXISTS_USE_ANOTHER_EMAIL: "site.error.exists",
  USERNAME_IS_ALREADY_TAKEN: "site.error.exists",
  PASSWORD_TOO_SHORT: "site.error.passwordShort",
  INVALID_PASSWORD: "site.error.wrongPassword",
  INVALID_TOKEN: "site.reset.invalid",
  INVALID_CODE: "site.error.code",
  TOO_MANY_REQUESTS: "site.error.tooMany",
  BANNED_USER: "site.error.suspended",
};

/**
 * describe words an error for the page: better-auth's and the rate
 * limits' in the page's language, and the services' as they gave it.
 */
export function describe(t: T, err: unknown): string | undefined {
  if (!err) return undefined;
  if (err instanceof ConnectError) return err.code === Code.ResourceExhausted ? t("site.error.tooMany") : err.rawMessage;
  if (typeof err === "object") {
    const e = err as { code?: string; status?: number; message?: string };
    const key = (e.code && authErrors[e.code]) || (e.status === 429 ? "site.error.tooMany" : undefined);
    if (key) return t(key);
    if (e.message) return e.message;
  }
  return String(err);
}
