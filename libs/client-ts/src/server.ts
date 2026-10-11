import { createConnectTransport } from "@connectrpc/connect-web";
import { Code, ConnectError, type Interceptor, type Transport } from "@connectrpc/connect";

/** A server the user is signed in to. */
export interface Server {
  /** The server's base URL, such as `https://media.example.com`. */
  url: string;
  /** The bearer token of the user's session on it, once signed in. */
  token?: string;
}

/**
 * The Connect transport of a server, which sends the user's token on every
 * call. Servers answer cross-origin requests, so the web UI calls each of
 * them directly. onUnauthenticated runs when the server rejects the token.
 */
export function transport(server: Server, onUnauthenticated?: () => void): Transport {
  const auth: Interceptor = (next) => async (req) => {
    if (server.token) {
      req.header.set("Authorization", `Bearer ${server.token}`);
    }
    try {
      return await next(req);
    } catch (err) {
      if (server.token && ConnectError.from(err).code === Code.Unauthenticated) onUnauthenticated?.();
      throw err;
    }
  };
  return createConnectTransport({ baseUrl: server.url, interceptors: [auth] });
}

/** The absolute URL of a path the server returned relative to its base URL. */
export function urlOf(server: Server, path: string): string {
  if (path.includes("://")) return path;
  return `${server.url.replace(/\/+$/, "")}/${path.replace(/^\/+/, "")}`;
}
