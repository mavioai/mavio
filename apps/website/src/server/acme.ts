// Servers' ACME challenges and account bindings (docs/mavio-dev.md §4.2):
// the TXT records of DNS-01 challenges in mavio.dev's Cloudflare zone, and
// the external account bindings of Google Trust Services, minted through
// Google's Public CA API.

/** Acme is what mavio.dev answers servers' certificate requests with. */
export interface Acme {
  /** The domain under which challenges' TXT records are named, such as acme.mavio.dev. */
  domain: string;
  dns: AcmeDns;
  /** Mints external account bindings; undefined offers none. */
  bindings?: AcmeBindings;
}

/** AcmeDns writes and removes TXT records. */
export interface AcmeDns {
  /** set replaces the TXT records of name with values. */
  set(name: string, values: string[]): Promise<void>;
  /** clear removes the TXT records of name. */
  clear(name: string): Promise<void>;
}

/** AcmeBindings mints external account bindings (RFC 8555 §7.3.4). */
export interface AcmeBindings {
  create(): Promise<{ keyId: string; hmacKey: Uint8Array }>;
}

type Fetch = typeof fetch;

const cloudflareAPI = "https://api.cloudflare.com/client/v4";

/** cloudflareDns writes TXT records in a Cloudflare zone with an API token allowed to edit its DNS. */
export function cloudflareDns(token: string, zoneId: string, fetcher: Fetch = fetch): AcmeDns {
  const call = async (path: string, init: RequestInit = {}) => {
    const res = await fetcher(`${cloudflareAPI}/zones/${zoneId}/dns_records${path}`, {
      ...init,
      headers: { authorization: `Bearer ${token}`, "content-type": "application/json" },
    });
    const body = (await res.json()) as { success: boolean; errors?: { message: string }[]; result: unknown };
    if (!res.ok || !body.success) {
      throw new Error(`Cloudflare DNS: ${res.status} ${body.errors?.map((e) => e.message).join("; ") ?? ""}`);
    }
    return body.result;
  };
  const clear = async (name: string) => {
    const records = (await call(`?type=TXT&name.exact=${encodeURIComponent(name)}`)) as { id: string }[];
    await Promise.all(records.map((r) => call(`/${r.id}`, { method: "DELETE" })));
  };
  return {
    clear,
    async set(name, values) {
      await clear(name);
      for (const value of values) {
        await call("", { method: "POST", body: JSON.stringify({ type: "TXT", name, content: `"${value}"`, ttl: 60 }) });
      }
    },
  };
}

/** The fields of a Google service account's key file that minting uses. */
interface ServiceAccount {
  client_email: string;
  private_key: string;
  token_uri?: string;
}

/**
 * googleBindings mints Google Trust Services' external account keys in a
 * Google Cloud project, as a service account allowed to create them,
 * whose JSON key file is credentials.
 */
export function googleBindings(credentials: string, project: string, fetcher: Fetch = fetch, now = () => new Date()): AcmeBindings {
  const account = JSON.parse(credentials) as ServiceAccount;
  const tokenURI = account.token_uri ?? "https://oauth2.googleapis.com/token";
  return {
    async create() {
      const accessToken = await googleToken(account, tokenURI, fetcher, now());
      const res = await fetcher(`https://publicca.googleapis.com/v1/projects/${project}/locations/global/externalAccountKeys`, {
        method: "POST",
        headers: { authorization: `Bearer ${accessToken}`, "content-type": "application/json" },
        body: "{}",
      });
      if (!res.ok) throw new Error(`Google Public CA: ${res.status} ${await res.text()}`);
      const key = (await res.json()) as { keyId: string; b64MacKey: string };
      return { keyId: key.keyId, hmacKey: new Uint8Array(Buffer.from(key.b64MacKey, "base64")) };
    },
  };
}

/** googleToken exchanges a JWT signed by the service account for an access token (RFC 7523). */
async function googleToken(account: ServiceAccount, tokenURI: string, fetcher: Fetch, at: Date): Promise<string> {
  const der = Buffer.from(account.private_key.replace(/-----[A-Z ]+-----|\s/g, ""), "base64");
  const key = await crypto.subtle.importKey("pkcs8", der, { name: "RSASSA-PKCS1-v1_5", hash: "SHA-256" }, false, ["sign"]);
  const iat = Math.floor(at.getTime() / 1000);
  const enc = (v: unknown) => Buffer.from(JSON.stringify(v)).toString("base64url");
  const unsigned = `${enc({ alg: "RS256", typ: "JWT" })}.${enc({
    iss: account.client_email,
    scope: "https://www.googleapis.com/auth/cloud-platform",
    aud: tokenURI,
    iat,
    exp: iat + 600,
  })}`;
  const signature = await crypto.subtle.sign("RSASSA-PKCS1-v1_5", key, new TextEncoder().encode(unsigned));
  const res = await fetcher(tokenURI, {
    method: "POST",
    headers: { "content-type": "application/x-www-form-urlencoded" },
    body: new URLSearchParams({
      grant_type: "urn:ietf:params:oauth:grant-type:jwt-bearer",
      assertion: `${unsigned}.${Buffer.from(signature).toString("base64url")}`,
    }),
  });
  if (!res.ok) throw new Error(`Google OAuth: ${res.status} ${await res.text()}`);
  return ((await res.json()) as { access_token: string }).access_token;
}
