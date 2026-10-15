// Relay tickets (docs/mavio-dev.md §5): what admits a client's connection
// to a server through the relay of its region, checked by apps/relay with
// the secret both share.
import { base32 } from "./ids.ts";

/** Relays are mavio.dev's relays and the secret of their tickets. */
export interface Relays {
  /** The relays by region, such as eu, with the base URLs servers connect to. */
  urls: Map<string, string>;
  /** The secret shared with the relays. */
  secret: Uint8Array;
  /** The domain of servers' names, such as mavio.site. */
  zone: string;
}

/** ticketLifetime is how long a ticket admits new connections, in seconds. */
export const ticketLifetime = 300;

/** parseRelays reads the RELAYS binding, the base URLs by region. */
export function parseRelays(byRegion: Record<string, string>, secret: string, zone = "mavio.site"): Relays {
  const urls = new Map(Object.entries(byRegion));
  for (const region of urls.keys()) {
    if (!/^[a-z0-9]{1,16}$/.test(region)) throw new Error(`relay region ${region} must be a label without dashes`);
  }
  return { urls, secret: new TextEncoder().encode(secret), zone };
}

/**
 * relayTicket returns the ticket admitting connections to the server with
 * an ID through the relay of a region until exp, in Unix seconds: exp as
 * four big-endian bytes and the first 16 bytes of an HMAC-SHA256 over the
 * ID, the region and those bytes, in lowercase base32.
 */
export async function relayTicket(secret: Uint8Array, id: string, region: string, exp: number): Promise<string> {
  const expiry = new Uint8Array(4);
  new DataView(expiry.buffer).setUint32(0, exp);
  const key = await crypto.subtle.importKey("raw", secret.slice(), { name: "HMAC", hash: "SHA-256" }, false, ["sign"]);
  const label = new TextEncoder().encode(`mavio relay ticket\0${id}\0${region}\0`);
  const mac = new Uint8Array(await crypto.subtle.sign("HMAC", key, concat(label, expiry)));
  return base32(concat(expiry, mac.subarray(0, 16)));
}

/** relayHost is the name a client reaches a server at through a relay. */
export function relayHost(region: string, ticket: string, id: string, zone: string): string {
  return `relay-${region}-${ticket}.${id}.${zone}`;
}

function concat(a: Uint8Array, b: Uint8Array) {
  const out = new Uint8Array(a.length + b.length);
  out.set(a);
  out.set(b, a.length);
  return out;
}
