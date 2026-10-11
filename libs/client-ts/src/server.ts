import { createConnectTransport } from "@connectrpc/connect-web";
import type { Interceptor, Transport } from "@connectrpc/connect";

/** A server the user is signed in to. */
export interface Server {
  /** The server's origin, such as `https://media.example.com`. */
  url: string;
  /** The bearer token of the user's session on it, once signed in. */
  token?: string;
}

/**
 * The Connect transport of a server, which sends the user's token on every
 * call. Servers answer cross-origin requests, so the web UI calls each of
 * them directly.
 */
export function transport(server: Server): Transport {
  const auth: Interceptor = (next) => (req) => {
    if (server.token) {
      req.header.set("Authorization", `Bearer ${server.token}`);
    }
    return next(req);
  };
  return createConnectTransport({ baseUrl: server.url, interceptors: [auth] });
}
