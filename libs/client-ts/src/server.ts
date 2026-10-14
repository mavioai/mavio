import { createConnectTransport } from "@connectrpc/connect-web";
import { Code, ConnectError, type Interceptor, type Transport } from "@connectrpc/connect";

/** A server the user is signed in to. */
export interface Server {
  /** The server's base URL, such as `https://media.example.com`. */
  url: string;
  /** The access token of the user's session on it, once signed in. */
  token?: string;
}

/** The session a transport calls a server with. */
export interface Session {
  /** The access token, refreshed first when it is about to expire. */
  token(): Promise<string | undefined>;
  /** A new access token after the server rejected one; undefined once signed out. */
  refresh(): Promise<string | undefined>;
}

/**
 * The Connect transport of a server, which sends the user's access token on
 * every call. Servers answer cross-origin requests, so the web UI calls each
 * of them directly. With a session, a rejected token is refreshed and a
 * unary call made again; without one, the server's token is sent as it is.
 */
export function transport(server: Server, session?: Session): Transport {
  const auth: Interceptor = (next) => async (req) => {
    const token = session ? await session.token() : server.token;
    if (token) req.header.set("Authorization", `Bearer ${token}`);
    try {
      return await next(req);
    } catch (err) {
      if (!token || !session || ConnectError.from(err).code !== Code.Unauthenticated) throw err;
      const fresh = await session.refresh().catch(() => undefined);
      // A stream's caller starts it again.
      if (!fresh || req.stream) throw err;
      req.header.set("Authorization", `Bearer ${fresh}`);
      return await next(req);
    }
  };
  return createConnectTransport({ baseUrl: server.url, interceptors: [auth] });
}

/** The absolute URL of a path the server returned relative to its base URL. */
export function urlOf(server: Server, path: string): string {
  if (path.includes("://")) return path;
  return `${server.url.replace(/\/+$/, "")}/${path.replace(/^\/+/, "")}`;
}
