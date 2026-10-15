// The website's clients of mavio.dev's services, with the session's cookie.
import { createClient } from "@connectrpc/connect";
import { createConnectTransport } from "@connectrpc/connect-web";
import { AccountService } from "@mavio/proto/mavio/cloud/v1/account_pb";
import { FriendService, HomeService } from "@mavio/proto/mavio/cloud/v1/home_pb";

const transport = createConnectTransport({
  baseUrl: "/api",
  fetch: (input, init) => fetch(input, { ...init, credentials: "same-origin" }),
});

export const accounts = createClient(AccountService, transport);
export const friends = createClient(FriendService, transport);
export const homes = createClient(HomeService, transport);

/** switchTo switches the session to another member of the Home, throwing the refusal's message. */
export async function switchTo(body: { accountId: string; pin?: string; password?: string }): Promise<void> {
  const res = await fetch("/api/auth/home/switch", {
    method: "POST",
    credentials: "same-origin",
    headers: { "content-type": "application/json" },
    body: JSON.stringify(body),
  });
  if (!res.ok) {
    const err = (await res.json().catch(() => ({}))) as { message?: string };
    throw new Error(err.message || res.statusText);
  }
}

/** Config is the site's configuration for its pages. */
export interface Config {
  /** app.mavio.dev's origin. */
  appURL: string;
}

/** config fetches the site's configuration. */
export async function config(): Promise<Config> {
  const res = await fetch("/api/config");
  if (!res.ok) throw new Error(res.statusText);
  return res.json();
}

/** message returns an error's message for the page. */
export function message(err: unknown): string {
  if (err && typeof err === "object" && "rawMessage" in err && typeof err.rawMessage === "string") return err.rawMessage;
  if (err instanceof Error) return err.message;
  return String(err);
}
