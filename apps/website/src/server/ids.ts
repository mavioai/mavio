// Server IDs (docs/accounts.md §5.1): the first 20 bytes of the SHA-256 of
// the identity public key in lowercase base32 without padding.

const alphabet = "abcdefghijklmnopqrstuvwxyz234567";

/** base32 encodes bytes in lowercase RFC 4648 base32 without padding. */
export function base32(bytes: Uint8Array): string {
  let out = "";
  let bits = 0;
  let value = 0;
  for (const b of bytes) {
    value = (value << 8) | b;
    bits += 8;
    while (bits >= 5) {
      out += alphabet[(value >>> (bits - 5)) & 31];
      bits -= 5;
    }
  }
  if (bits > 0) out += alphabet[(value << (5 - bits)) & 31];
  return out;
}

/** serverID returns the ID of the server whose identity public key is publicKey. */
export async function serverID(publicKey: Uint8Array): Promise<string> {
  const digest = new Uint8Array(await crypto.subtle.digest("SHA-256", publicKey.slice()));
  return base32(digest.subarray(0, 20));
}

/** base64url encodes bytes in base64url without padding. */
export function base64url(bytes: Uint8Array): string {
  return Buffer.from(bytes).toString("base64url");
}

/** fromBase64url decodes base64url with or without padding; undefined when malformed. */
export function fromBase64url(s: string): Uint8Array<ArrayBuffer> | undefined {
  if (!/^[A-Za-z0-9_-]*={0,2}$/.test(s)) return undefined;
  return new Uint8Array(Buffer.from(s, "base64url"));
}
