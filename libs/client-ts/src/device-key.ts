// The key of this browser's sessions (docs/accounts.md §5.3): an Ed25519
// key pair whose private half WebCrypto keeps non-extractable in IndexedDB,
// so that the web UI signs with it but no script can read it.

/** A device's key: its public half and signatures by its private half. */
export interface DeviceKey {
  /** The 32-byte Ed25519 public key. */
  publicKey(): Promise<Uint8Array>;
  /** The 64-byte Ed25519 signature of a message. */
  sign(message: Uint8Array): Promise<Uint8Array>;
}

/** What a device signs to sign in with a sign-in assertion, as the server checks it. */
export function assertionMessage(assertion: string): Uint8Array {
  return new TextEncoder().encode(`mavio-sign-in-assertion-v1\0${assertion}`);
}

/** What a device signs to refresh its session, as the server checks it. */
export function refreshMessage(sessionId: string, nonce: Uint8Array): Uint8Array {
  const context = new TextEncoder().encode("mavio-session-refresh-v1\0");
  const hex = sessionId.replaceAll("-", "");
  if (!/^[0-9a-f]{32}$/i.test(hex)) throw new Error(`${sessionId} is not a session ID`);
  const id = Uint8Array.from({ length: 16 }, (_, i) => Number.parseInt(hex.slice(2 * i, 2 * i + 2), 16));
  const out = new Uint8Array(context.length + id.length + nonce.length);
  out.set(context);
  out.set(id, context.length);
  out.set(nonce, context.length + id.length);
  return out;
}

function generate(): Promise<CryptoKeyPair> {
  return crypto.subtle.generateKey({ name: "Ed25519" }, false, ["sign", "verify"]) as Promise<CryptoKeyPair>;
}

function deviceKeyOf(pair: Promise<CryptoKeyPair>): DeviceKey {
  return {
    async publicKey() {
      return new Uint8Array(await crypto.subtle.exportKey("raw", (await pair).publicKey));
    },
    async sign(message) {
      return new Uint8Array(await crypto.subtle.sign("Ed25519", (await pair).privateKey, message as BufferSource));
    },
  };
}

/** A key kept in memory only, for tests and browsers without IndexedDB. */
export function memoryDeviceKey(): DeviceKey {
  return deviceKeyOf(generate());
}

const dbName = "mavio";
const storeName = "keys";
const keyName = "device";

function request<T>(r: IDBRequest<T>): Promise<T> {
  return new Promise((resolve, reject) => {
    r.onsuccess = () => resolve(r.result);
    r.onerror = () => reject(r.error);
  });
}

/** The pair kept in IndexedDB, made the first time. */
async function storedPair(): Promise<CryptoKeyPair> {
  const open = indexedDB.open(dbName, 1);
  open.onupgradeneeded = () => open.result.createObjectStore(storeName);
  const db = await request(open);
  try {
    const kept = (await request(db.transaction(storeName).objectStore(storeName).get(keyName))) as
      | CryptoKeyPair
      | undefined;
    if (kept) return kept;
    const pair = await generate();
    // Another tab may have made one meanwhile; add keeps the first.
    const tx = db.transaction(storeName, "readwrite");
    try {
      await request(tx.objectStore(storeName).add(pair, keyName));
      return pair;
    } catch {
      return (await request(db.transaction(storeName).objectStore(storeName).get(keyName))) as CryptoKeyPair;
    }
  } finally {
    db.close();
  }
}

/** This browser's key, kept in IndexedDB where there is one. */
export function browserDeviceKey(): DeviceKey {
  if (typeof indexedDB === "undefined") return memoryDeviceKey();
  let pair: Promise<CryptoKeyPair> | undefined;
  const lazy = () => (pair ??= storedPair());
  return {
    publicKey: () => deviceKeyOf(lazy()).publicKey(),
    sign: (message) => deviceKeyOf(lazy()).sign(message),
  };
}
