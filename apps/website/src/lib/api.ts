// The website's client of AccountService, with the session's cookie.
import { createClient } from "@connectrpc/connect";
import { createConnectTransport } from "@connectrpc/connect-web";
import { AccountService } from "@mavio/proto/mavio/cloud/v1/account_pb";

export const accounts = createClient(
  AccountService,
  createConnectTransport({ baseUrl: "/api", fetch: (input, init) => fetch(input, { ...init, credentials: "same-origin" }) }),
);

/** message returns an error's message for the page. */
export function message(err: unknown): string {
  if (err && typeof err === "object" && "rawMessage" in err && typeof err.rawMessage === "string") return err.rawMessage;
  if (err instanceof Error) return err.message;
  return String(err);
}
