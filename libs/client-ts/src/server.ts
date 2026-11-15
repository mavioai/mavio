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
 * With a language, calls ask for it in their Accept-Language header.
 */
export function transport(server: Server, session?: Session, language?: string): Transport {
  const auth: Interceptor = (next) => async (req) => {
    if (language) req.header.set("Accept-Language", language);
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

/**
 * Whether a server's address is on the local network, as the core's
 * `is_local_address` decides (docs/clients.md §6.5): a loopback, private
 * or link-local IP address, a `localhost` or `.local` name, or a
 * s.mavio.fyi name of such an address (`192-168-1-10.<ID>.s.mavio.fyi`).
 * Clients send more over it than over the internet, where an upload or a
 * relay sets the pace.
 */
export function isLocalAddress(address: string): boolean {
  const input = address.trim().replace(/\/+$/, "");
  if (!input) return false;
  let host: string;
  try {
    const u = new URL(input.includes("://") ? input : `http://${input}`);
    if (u.protocol !== "http:" && u.protocol !== "https:") return false;
    host = u.hostname.toLowerCase();
  } catch {
    return false;
  }
  if (host.startsWith("[")) return localIPv6(host.slice(1, -1));
  if (host === "localhost" || host.endsWith(".localhost") || host.endsWith(".local")) return true;
  const v4 = ipv4(host);
  if (v4) return localIPv4(v4);
  // `<address>.<ID>.s.mavio.fyi`, the address written with dashes.
  const labels = host.split(".");
  if (labels.length === 5 && labels.slice(2).join(".") === "s.mavio.fyi") {
    const dotted = ipv4(labels[0].replaceAll("-", "."));
    if (dotted) return localIPv4(dotted);
    const v6 = labels[0].replaceAll("-", ":");
    return /^[0-9a-f:]+$/.test(v6) && validIPv6(v6) && localIPv6(v6);
  }
  return false;
}

/** The octets of a dotted IPv4 address, undefined for anything else. */
function ipv4(s: string): number[] | undefined {
  const parts = s.split(".");
  if (parts.length !== 4 || !parts.every((p) => /^(0|[1-9]\d{0,2})$/.test(p) && Number(p) <= 255)) return undefined;
  return parts.map(Number);
}

function localIPv4([a, b]: number[]): boolean {
  return a === 127 || a === 10 || (a === 172 && b >= 16 && b <= 31) || (a === 192 && b === 168) || (a === 169 && b === 254);
}

function validIPv6(s: string): boolean {
  try {
    new URL(`http://[${s}]`);
    return true;
  } catch {
    return false;
  }
}

/** The eight groups of an IPv6 address, as the URL parser normalizes it. */
function groups(s: string): number[] {
  const [head, tail] = s.includes("::") ? s.split("::") : [s, undefined];
  const parse = (part: string | undefined) => (part ? part.split(":").map((g) => parseInt(g, 16)) : []);
  const h = parse(head);
  const t = parse(tail);
  return tail === undefined ? h : [...h, ...Array<number>(8 - h.length - t.length).fill(0), ...t];
}

function localIPv6(s: string): boolean {
  let normalized: string;
  try {
    normalized = new URL(`http://[${s}]`).hostname.slice(1, -1);
  } catch {
    return false;
  }
  const g = groups(normalized);
  // An IPv4-mapped address, ::ffff:a.b.c.d.
  if (g.slice(0, 5).every((x) => x === 0) && g[5] === 0xffff) return localIPv4([g[6] >> 8, g[6] & 0xff]);
  const loopback = g.slice(0, 7).every((x) => x === 0) && g[7] === 1;
  return loopback || (g[0] & 0xfe00) === 0xfc00 || (g[0] & 0xffc0) === 0xfe80;
}
