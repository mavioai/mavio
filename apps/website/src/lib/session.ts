import { createIsomorphicFn } from "@tanstack/react-start";
import { getRequestHeader } from "@tanstack/react-start/server";
import { useNavigate, useRouteContext } from "@tanstack/react-router";
import { useEffect } from "react";

import { authClient } from "./auth-client.ts";

/** useAccount returns the signed-in account, sending others to sign in. */
export function useAccount() {
  const { data, isPending } = authClient.useSession();
  const navigate = useNavigate();
  useEffect(() => {
    if (!isPending && !data) {
      void navigate({ to: "/sign-in", search: { next: window.location.pathname + window.location.search } });
    }
  }, [data, isPending, navigate]);
  return data?.user;
}

/**
 * hasSession returns whether the page is rendered for a session: on the
 * server, whether the request carries its cookie; in the browser, what the
 * server rendered, which <html data-session> carries, so that hydrating
 * renders the same page.
 */
export const hasSession = createIsomorphicFn()
  .server((): boolean => /(?:^|;\s*)(?:__Secure-)?better-auth\.session_token=/.test(getRequestHeader("cookie") ?? ""))
  .client((): boolean => document.documentElement.dataset.session === "1");

/** useSignedIn returns whether a session is signed in, as rendered until better-auth knows. */
export function useSignedIn(): boolean {
  const { data, isPending } = authClient.useSession();
  const rendered = useRouteContext({ from: "__root__", select: (c) => c.session });
  return isPending ? rendered : !!data;
}
