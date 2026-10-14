// Password hashing for better-auth (docs/mavio-dev.md §2): scrypt from
// node:crypto, stored in PHC format so that the parameters travel with the
// hash and a sign-in can hash again when they change.
import { randomBytes, scrypt, timingSafeEqual } from "node:crypto";

/** The parameters new hashes use. */
export const params = { ln: 14, r: 16, p: 1, keyLength: 64 } as const;

type Params = { ln: number; r: number; p: number };

function derive(password: string, salt: Uint8Array, { ln, r, p }: Params, keyLength: number): Promise<Buffer> {
  const N = 2 ** ln;
  return new Promise((resolve, reject) =>
    scrypt(password.normalize("NFKC"), salt, keyLength, { N, r, p, maxmem: 256 * N * r * p }, (err, key) =>
      err ? reject(err) : resolve(key),
    ),
  );
}

const b64 = (b: Uint8Array) => Buffer.from(b).toString("base64").replace(/=+$/, "");

/** hash returns the PHC string of password with a new salt. */
export async function hash(password: string): Promise<string> {
  const salt = randomBytes(16);
  const key = await derive(password, salt, params, params.keyLength);
  return `$scrypt$ln=${params.ln},r=${params.r},p=${params.p}$${b64(salt)}$${b64(key)}`;
}

function parse(phc: string): { params: Params; salt: Buffer; key: Buffer } | undefined {
  const m = /^\$scrypt\$ln=(\d+),r=(\d+),p=(\d+)\$([A-Za-z0-9+/]+)\$([A-Za-z0-9+/]+)$/.exec(phc);
  if (!m) return undefined;
  const [ln, r, p] = [m[1], m[2], m[3]].map(Number);
  if (ln < 1 || ln > 20 || r < 1 || r > 32 || p < 1 || p > 4) return undefined;
  return { params: { ln, r, p }, salt: Buffer.from(m[4], "base64"), key: Buffer.from(m[5], "base64") };
}

/** verify reports whether password matches the PHC string. */
export async function verify({ hash, password }: { hash: string; password: string }): Promise<boolean> {
  const parsed = parse(hash);
  if (!parsed || parsed.key.length < 16) return false;
  const key = await derive(password, parsed.salt, parsed.params, parsed.key.length);
  return timingSafeEqual(key, parsed.key);
}

/** outdated reports whether a PHC string was made with other parameters than new hashes. */
export function outdated(phc: string): boolean {
  const parsed = parse(phc);
  return (
    !parsed ||
    parsed.params.ln !== params.ln ||
    parsed.params.r !== params.r ||
    parsed.params.p !== params.p ||
    parsed.key.length !== params.keyLength
  );
}
