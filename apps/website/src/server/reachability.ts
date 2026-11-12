// Checking servers' published URLs from the internet (docs/mavio-dev.md
// §4.3): mavio.dev opens a TCP connection to the host and port of each URL
// at a public address, so that clients try the URLs that accepted one
// before those that did not. Only URLs a server published are checked, at
// most every minute per server, and never those of loopback, private or
// link-local addresses, which only the server's own network reaches.
import type { ConnectionCheckRow } from "./db/schema.ts";

/** Dial opens a TCP connection to host and port within timeout milliseconds, and tells whether it opened. */
export type Dial = (host: string, port: number, timeout: number) => Promise<boolean>;

/** Checks is how the checks run: dialing, and work carried on after the response. */
export interface Checks {
  dial: Dial;
  /** Runs a check after the request that started it has been answered. */
  background: (work: Promise<unknown>) => void;
}

/** How long a connection may take to open. */
export const dialTimeout = 3000;

/** The fewest milliseconds between a server's checks. */
export const minInterval = 60_000;

/** How old a server's checks may grow before listing its servers checks them again. */
export const maxAge = 6 * 60 * 60_000;

/** The most URLs of a server checked. */
const maxUrls = 8;

/**
 * checkable returns the host and port to check of a published URL, or
 * undefined for one that is not checked: not http or https, or at a
 * loopback, private, link-local or otherwise non-public address, written
 * as an IP address or as the s.mavio.fyi name of one.
 */
export function checkable(url: string, zone = "s.mavio.fyi"): { host: string; port: number } | undefined {
  let u: URL;
  try {
    u = new URL(url);
  } catch {
    return undefined;
  }
  if (u.protocol !== "http:" && u.protocol !== "https:") return undefined;
  const host = u.hostname.replace(/^\[|\]$/g, "").toLowerCase();
  const port = u.port ? Number(u.port) : u.protocol === "https:" ? 443 : 80;
  if (host === "localhost" || host.endsWith(".localhost") || host.endsWith(".local") || !host.includes(".") && !host.includes(":")) {
    return undefined;
  }
  // <address>.<ID>.<zone>, the address written with dashes.
  const labels = host.split(".");
  const named = host.endsWith(`.${zone}`) && labels.length === zone.split(".").length + 2 ? addressOfLabel(labels[0]) : undefined;
  const ip = named ?? (isIPv4(host) || host.includes(":") ? host : undefined);
  if (ip !== undefined && !isPublic(ip)) return undefined;
  return { host, port };
}

/** The IP address a s.mavio.fyi name's first label writes with dashes, or undefined for another label, such as a relay's. */
function addressOfLabel(label: string): string | undefined {
  if (label.startsWith("relay-")) return undefined;
  const v4 = label.replaceAll("-", ".");
  if (isIPv4(v4)) return v4;
  return label.includes("--") || label.split("-").length === 8 ? label.replaceAll("-", ":") : undefined;
}

function isIPv4(s: string): boolean {
  const parts = s.split(".");
  return parts.length === 4 && parts.every((p) => /^\d{1,3}$/.test(p) && Number(p) <= 255);
}

/** Whether an IP address is reachable from the internet: not loopback, private, link-local, shared, documentation or multicast. */
export function isPublic(ip: string): boolean {
  if (isIPv4(ip)) {
    const [a, b] = ip.split(".").map(Number);
    return !(
      a === 0 ||
      a === 10 ||
      a === 127 ||
      (a === 100 && b >= 64 && b < 128) ||
      (a === 169 && b === 254) ||
      (a === 172 && b >= 16 && b < 32) ||
      (a === 192 && b === 168) ||
      (a === 192 && b === 0) ||
      (a === 198 && (b === 18 || b === 19)) ||
      (a === 198 && b === 51 && ip.split(".")[2] === "100") ||
      (a === 203 && b === 0 && ip.split(".")[2] === "113") ||
      a >= 224
    );
  }
  const v6 = ip.toLowerCase();
  if (v6 === "::" || v6 === "::1") return false;
  const first = Number.parseInt(v6.split(":")[0] || "0", 16);
  // Global unicast only (2000::/3), less documentation (2001:db8::/32).
  return first >= 0x2000 && first < 0x4000 && !v6.startsWith("2001:db8:");
}

/** checkConnections checks a server's published URLs at, returning what it found of those it checks. */
export async function checkConnections(urls: string[], dial: Dial, at: Date): Promise<ConnectionCheckRow[]> {
  const targets = urls.slice(0, maxUrls).flatMap((url) => {
    const t = checkable(url);
    return t ? [{ url, ...t }] : [];
  });
  return Promise.all(
    targets.map(async (t) => ({
      url: t.url,
      reachable: await dial(t.host, t.port, dialTimeout).catch(() => false),
      checkedAt: at.toISOString(),
    })),
  );
}

/**
 * due tells whether a server's URLs are checked now: when one it publishes
 * at a public address has not been checked or the last check is older
 * than maxAge, and never within minInterval of the last.
 */
export function due(s: { connections: string[]; connectionChecks: ConnectionCheckRow[]; connectionsCheckedAt: Date | null }, now: Date): boolean {
  const unchecked = s.connections.slice(0, maxUrls).some((u) => checkable(u) && !s.connectionChecks.some((c) => c.url === u));
  if (!s.connectionsCheckedAt) return unchecked;
  const age = now.getTime() - s.connectionsCheckedAt.getTime();
  if (age < minInterval) return false;
  return unchecked || (age > maxAge && s.connectionChecks.length > 0);
}

/** The checks of the URLs a server still publishes, as clients are told them. */
export function current(s: { connections: string[]; connectionChecks: ConnectionCheckRow[] }): ConnectionCheckRow[] {
  return s.connectionChecks.filter((c) => s.connections.includes(c.url));
}
