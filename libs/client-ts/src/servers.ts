// The servers the web UI is signed in to (docs/clients.md §4.1, §5.4,
// docs/accounts.md §6, §7): one sign-in at a time. A Mavio account holds a
// session of the browser's device key at mavio.dev, with which it signs in
// to each of the account's servers; a standalone server's user is signed
// in to the server serving the web UI alone. app.mavio.dev, served by no
// server, signs Mavio accounts in only.
import type { DescService } from "@bufbuild/protobuf";
import { timestampDate } from "@bufbuild/protobuf/wkt";
import type { Timestamp } from "@bufbuild/protobuf/wkt";
import { type Client, Code, ConnectError, createClient, type Transport } from "@connectrpc/connect";
import { createConnectTransport } from "@connectrpc/connect-web";
import { AccountMode, AuthService, type GetAuthInfoResponse, type SwitchableUser } from "@mavio/proto/mavio/auth/v1/auth_pb";
import { AccountService } from "@mavio/proto/mavio/cloud/v1/account_pb";

import { assertionMessage, browserDeviceKey, type DeviceKey, refreshMessage, renewalMessage } from "./device-key.ts";
import { transport } from "./server.ts";

/** A server signed in to. */
export interface ServerEntry {
  /** The server's own ID, the same at every address. */
  id: string;
  name: string;
  version: string;
  /** The base URL it is reached at, such as `https://media.example.com/mavio`; empty while unreachable. */
  url: string;
  /** The URLs a Mavio account's server published, best first. */
  urls?: string[];
  userId: string;
  userName: string;
  /** How the server signs people in, as it told at sign-in. */
  mode?: "standalone" | "mavioDev";
  /** This browser's session; unset when signed out. */
  sessionId?: string;
  /** The session's access token; unset when signed out. */
  token?: string;
  /** When the access token expires, in milliseconds since the epoch. */
  expires?: number;
}

/** What a server tells before anyone signs in. */
export interface ServerInfo {
  id: string;
  name: string;
  version: string;
  url: string;
  setupRequired: boolean;
  /** How the server signs people in; unset until its setup chooses. */
  mode?: "standalone" | "mavioDev";
  /** The base URL of mavio.dev's API, where it is claimed and signs in; unset on standalone servers. */
  mavioDevUrl?: string;
}

/** A Mavio account signed in. */
export interface Account {
  id: string;
  name: string;
}

/** The sign-in of the web UI: a Mavio account's, or a standalone server user's. */
export interface SignIn {
  kind: "mavioDev" | "standalone";
  /** The account's or the user's name. */
  name: string;
  /** The Mavio account. */
  account?: Account;
  /** The servers signed in to, in the user's order. */
  servers: ServerEntry[];
}

/** The device's session at mavio.dev. */
interface AccountSession {
  /** The base URL of mavio.dev's API. */
  api: string;
  clientId: string;
  sessionId: string;
  token: string;
  /** When the access token expires, in milliseconds since the epoch. */
  expires: number;
  account: Account;
}

/** A sign-in on its way to mavio.dev and back. */
interface Pending {
  api: string;
  clientId: string;
  state: string;
  verifier: string;
  redirectUri: string;
  /** Where the web UI goes once signed in. */
  returnTo: string;
}

/** A Quick Connect request: the user enters the code on a signed-in device. */
export interface QuickConnect {
  code: string;
  expires: Date;
  server: ServerInfo;
  secret: string;
}

/** A link code a television shows, which an account approves at mavio.dev/link. */
export interface LinkCode {
  userCode: string;
  /** Where to enter it. */
  verificationUri: string;
  /** Where to go to approve it without typing it. */
  verificationUriComplete: string;
  deviceCode: string;
  /** How often to poll, in milliseconds. */
  interval: number;
  expires: Date;
}

/** Where the sign-in is kept in the browser. */
export interface Storage {
  getItem(key: string): string | null;
  setItem(key: string, value: string): void;
}

const serversKey = "mavio.servers";
const accountKey = "mavio.account";
const deviceKey = "mavio.device";
const pendingKey = "mavio.pendingSignIn";

/** How long before its expiry an access token is refreshed. */
const refreshMargin = 5 * 60_000;
/** How long before its expiry mavio.dev's access token is renewed. */
const renewMargin = 60_000;
/** How long a server's address is tried. */
const reachTimeout = 4000;

/** The grant type that renews a device's session at mavio.dev. */
const renewGrant = "urn:mavio:params:oauth:grant-type:device-signature";
const deviceCodeGrant = "urn:ietf:params:oauth:grant-type:device_code";

/** The client ID of app.mavio.dev. */
const appClient = "app";

/** A sign-in's answer. */
interface SignedIn {
  accessToken: string;
  accessTokenExpireTime?: Timestamp;
  session?: { id: string };
  user?: { id: string; name: string };
}

/** The device as servers list it among the user's devices. */
async function device(storage: Storage, key: DeviceKey) {
  let id = storage.getItem(deviceKey);
  if (!id) {
    id = crypto.randomUUID();
    storage.setItem(deviceKey, id);
  }
  const ua = typeof navigator === "undefined" ? "" : navigator.userAgent;
  const browser = /Firefox\//.test(ua)
    ? "Firefox"
    : /Edg\//.test(ua)
      ? "Edge"
      : /Chrome\//.test(ua)
        ? "Chrome"
        : /Safari\//.test(ua)
          ? "Safari"
          : "Browser";
  return { id, name: browser, client: "Mavio Web", clientVersion: "0.0.0", publicKey: await key.publicKey() };
}

/** Normalizes a base URL: no trailing slashes. */
export function normalizeUrl(input: string): string {
  const u = new URL(input.trim());
  if (u.protocol !== "http:" && u.protocol !== "https:") throw new Error(`${input} is not http or https`);
  return `${u.protocol}//${u.host}${u.pathname.replace(/\/+$/, "")}`;
}

function expiry(r: { accessTokenExpireTime?: Timestamp }): number | undefined {
  return r.accessTokenExpireTime ? timestampDate(r.accessTokenExpireTime).getTime() : undefined;
}

function info(url: string, r: GetAuthInfoResponse): ServerInfo {
  if (!r.serverId) throw new Error(`${url} is not a Mavio server`);
  const mode = r.mode === AccountMode.STANDALONE ? "standalone" : r.mode === AccountMode.MAVIO_DEV ? "mavioDev" : undefined;
  return {
    id: r.serverId,
    name: r.serverName,
    version: r.version,
    url,
    setupRequired: r.setupRequired,
    mode,
    mavioDevUrl: r.mavioDevUrl || undefined,
  };
}

function base64url(bytes: Uint8Array): string {
  return btoa(String.fromCharCode(...bytes))
    .replace(/\+/g, "-")
    .replace(/\//g, "_")
    .replace(/=+$/, "");
}

function randomString(): string {
  return base64url(crypto.getRandomValues(new Uint8Array(32)));
}

async function s256(verifier: string): Promise<string> {
  return base64url(new Uint8Array(await crypto.subtle.digest("SHA-256", new TextEncoder().encode(verifier))));
}

/** The site of mavio.dev's API. */
function siteOf(api: string): string {
  return api.replace(/\/+$/, "").replace(/\/api$/, "");
}

function read<T>(storage: Storage, key: string): T | undefined {
  try {
    return JSON.parse(storage.getItem(key) ?? "null") ?? undefined;
  } catch {
    return undefined;
  }
}

/** An OAuth endpoint's refusal. */
class OAuthError extends Error {
  constructor(
    readonly code: string,
    message: string,
  ) {
    super(message);
  }
}

/** Where the web UI runs. */
export interface Options {
  /**
   * The base URL of mavio.dev's API on app.mavio.dev, which no server
   * serves; unset when a server serves the web UI.
   */
  mavioDevUrl?: string;
  key?: DeviceKey;
}

/**
 * The sign-in, kept in the browser with its sessions, each of the
 * browser's device key, which signs their refreshes. Observed with
 * useSyncExternalStore.
 */
export class Servers {
  private list: ServerEntry[];
  private session: AccountSession | undefined;
  private signIn_: SignIn | undefined;
  private listeners = new Set<() => void>();
  private transports = new Map<string, { sessionId?: string; url: string; transport: Transport }>();
  private refreshing = new Map<string, Promise<string | undefined>>();
  private renewing: Promise<string | undefined> | undefined;
  private syncing: Promise<void> | undefined;
  private finishing: { key: string; done: Promise<string> } | undefined;
  private key: DeviceKey;
  /** The base URL of the server serving the web UI; unset on app.mavio.dev. */
  readonly home: string | undefined;
  /** The base URL of mavio.dev's API on app.mavio.dev. */
  readonly appApi: string | undefined;

  constructor(
    private storage: Storage,
    /** The origin and base path of the web UI, such as `https://media.example.com/mavio`. */
    readonly base: string,
    options: Options = {},
  ) {
    this.key = options.key ?? browserDeviceKey();
    this.appApi = options.mavioDevUrl;
    this.home = options.mavioDevUrl ? undefined : base;
    this.list = read<ServerEntry[]>(storage, serversKey) ?? [];
    this.session = read<AccountSession>(storage, accountKey);
    this.signIn_ = this.compute();
  }

  subscribe = (listener: () => void) => {
    this.listeners.add(listener);
    return () => this.listeners.delete(listener);
  };

  /** The servers in the user's order; a stable array until they change. */
  snapshot = () => this.list;

  /** The sign-in; a stable object until it changes. */
  signInSnapshot = () => this.signIn_;

  /** Whether this is app.mavio.dev, which signs Mavio accounts in only. */
  get app(): boolean {
    return !this.home;
  }

  /** The Mavio account signed in. */
  account(): Account | undefined {
    return this.session?.account;
  }

  /** One server. */
  get(id: string): ServerEntry | undefined {
    return this.list.find((s) => s.id === id);
  }

  /** The servers signed in to, in the user's order. */
  signedIn(): ServerEntry[] {
    return this.list.filter((s) => s.token);
  }

  /** The server serving the web UI, once signed in to. */
  homeServer(): ServerEntry | undefined {
    return this.home ? this.list.find((s) => s.url === this.home) : undefined;
  }

  private compute(): SignIn | undefined {
    const servers = this.signedIn();
    if (this.session) {
      const a = this.session.account;
      return { kind: "mavioDev", name: a.name, account: a, servers };
    }
    if (servers.length === 0) return undefined;
    return { kind: "standalone", name: servers[0].userName, servers };
  }

  private emit() {
    this.signIn_ = this.compute();
    for (const l of this.listeners) l();
  }

  private set(list: ServerEntry[]) {
    this.list = list;
    this.storage.setItem(serversKey, JSON.stringify(list));
    this.emit();
  }

  private setSession(s: AccountSession | undefined) {
    this.session = s;
    this.storage.setItem(accountKey, JSON.stringify(s ?? null));
    this.emit();
  }

  /**
   * The Connect client of a service on a server, whose access token is
   * refreshed as it expires; a session the server ended signs a Mavio
   * account in again, and the user of a standalone server out.
   */
  client<T extends DescService>(service: T, server: ServerEntry): Client<T> {
    let t = this.transports.get(server.id);
    if (!t || t.sessionId !== server.sessionId || t.url !== server.url) {
      const id = server.id;
      t = {
        sessionId: server.sessionId,
        url: server.url,
        transport: transport(server, { token: () => this.token(id), refresh: () => this.refresh(id) }),
      };
      this.transports.set(id, t);
    }
    return createClient(service, t.transport);
  }

  /** A server's access token, refreshed first when it is about to expire. */
  async token(id: string): Promise<string | undefined> {
    const s = this.get(id);
    if (!s?.token) return undefined;
    if (s.expires !== undefined && s.expires - Date.now() < refreshMargin) return this.refresh(id);
    return s.token;
  }

  /**
   * Refreshes a server's access token by signing its nonce with the device
   * key; one refresh runs at a time. A session the server no longer
   * refreshes is replaced with a new one of the Mavio account, or signs the
   * user out of it.
   */
  refresh(id: string): Promise<string | undefined> {
    let p = this.refreshing.get(id);
    if (!p) {
      p = this.doRefresh(id).finally(() => this.refreshing.delete(id));
      this.refreshing.set(id, p);
    }
    return p;
  }

  private async doRefresh(id: string): Promise<string | undefined> {
    const s = this.get(id);
    if (!s?.sessionId) return this.ended(id);
    const sessionId = s.sessionId;
    const auth = createClient(AuthService, transport({ url: s.url }));
    try {
      const { nonce } = await auth.startRefresh({ sessionId });
      const signature = await this.key.sign(refreshMessage(sessionId, nonce));
      const r = await auth.refreshSession({ sessionId, nonce, signature });
      const at = this.list.findIndex((x) => x.id === id);
      if (at < 0 || this.list[at].sessionId !== sessionId) return undefined;
      this.set(this.list.with(at, { ...this.list[at], token: r.accessToken, expires: expiry(r) }));
      return r.accessToken;
    } catch (err) {
      if (ConnectError.from(err).code === Code.Unauthenticated) return this.ended(id);
      throw err;
    }
  }

  // ended handles a server's session that ended: a Mavio account signs in again.
  private async ended(id: string): Promise<string | undefined> {
    this.forgetToken(id);
    const s = this.get(id);
    if (!this.session || !s?.url) return undefined;
    try {
      return (await this.assert(s)).token;
    } catch {
      return undefined;
    }
  }

  /** Forgets the session of a server that ended it. */
  forgetToken(id: string) {
    const at = this.list.findIndex((s) => s.id === id);
    if (at >= 0 && this.list[at].token) {
      const { sessionId: _, token: __, expires: ___, ...rest } = this.list[at];
      this.set(this.list.with(at, rest));
    }
  }

  /** Asks the server at a URL who it is. */
  async probe(url: string, timeoutMs?: number): Promise<ServerInfo> {
    const u = normalizeUrl(url);
    return info(u, await createClient(AuthService, transport({ url: u })).getAuthInfo({}, { timeoutMs }));
  }

  // Standalone servers: the serving server's users.

  private homeUrl(): string {
    if (!this.home) throw new Error("no server serves this page");
    return this.home;
  }

  private added(i: ServerInfo, r: SignedIn): ServerEntry {
    const entry: ServerEntry = {
      id: i.id,
      name: i.name,
      version: i.version,
      url: i.url,
      mode: i.mode,
      userId: r.user?.id ?? "",
      userName: r.user?.name ?? "",
      sessionId: r.session?.id,
      token: r.accessToken,
      expires: expiry(r),
    };
    const at = this.list.findIndex((s) => s.id === i.id);
    this.set(at < 0 ? [...this.list, entry] : this.list.with(at, entry));
    return entry;
  }

  /**
   * Signs in to the serving server with a name and password, and the
   * current code of the user's TOTP when it has one; without the code such
   * a user's sign-in fails as isTotpRequired tells.
   */
  async signInWithPassword(name: string, password: string, totpCode?: string): Promise<ServerEntry> {
    const i = await this.probe(this.homeUrl());
    const r = await createClient(AuthService, transport({ url: i.url })).login({
      name,
      password,
      totpCode,
      device: await device(this.storage, this.key),
    });
    return this.added(i, r);
  }

  /** Signs in to the serving server with a passkey of the browser's choosing. */
  async signInWithPasskey(): Promise<ServerEntry> {
    const i = await this.probe(this.homeUrl());
    const auth = createClient(AuthService, transport({ url: i.url }));
    const start = await auth.startPasskeySignIn({});
    const publicKey = PublicKeyCredential.parseRequestOptionsFromJSON(JSON.parse(start.optionsJson));
    const credential = (await navigator.credentials.get({ publicKey })) as PublicKeyCredential | null;
    if (!credential) throw new Error("no passkey was chosen");
    const r = await auth.finishPasskeySignIn({
      ceremonyId: start.ceremonyId,
      credentialJson: JSON.stringify(credential.toJSON()),
      device: await device(this.storage, this.key),
    });
    return this.added(i, r);
  }

  /** Sets the serving server up in standalone mode: creates its first user and signs in as them. */
  async createFirstUser(name: string, password: string): Promise<ServerEntry> {
    const i = await this.probe(this.homeUrl());
    const r = await createClient(AuthService, transport({ url: i.url })).createFirstUser({
      name,
      password,
      device: await device(this.storage, this.key),
    });
    return this.added(i, r);
  }

  /** Starts signing in to the serving server with Quick Connect. */
  async startQuickConnect(): Promise<QuickConnect> {
    const i = await this.probe(this.homeUrl());
    const r = await createClient(AuthService, transport({ url: i.url })).startQuickConnect({
      device: await device(this.storage, this.key),
    });
    return {
      code: r.code,
      secret: r.secret,
      expires: r.expireTime ? timestampDate(r.expireTime) : new Date(Date.now() + 5 * 60_000),
      server: i,
    };
  }

  /** Signs in once the Quick Connect request was authorized; undefined until then. */
  async pollQuickConnect(qc: QuickConnect): Promise<ServerEntry | undefined> {
    const auth = createClient(AuthService, transport({ url: qc.server.url }));
    const state = await auth.getQuickConnectState({ secret: qc.secret });
    if (!state.authorized) return undefined;
    const r = await auth.loginWithQuickConnect({ secret: qc.secret });
    return this.added(qc.server, r);
  }

  /** Signs in the device showing a Quick Connect code as the user of a server. */
  async authorizeQuickConnect(server: ServerEntry, code: string) {
    await this.client(AuthService, server).authorizeQuickConnect({ code: code.trim() });
  }

  /** The users a server's session may switch to (docs/accounts.md §4.4). */
  async switchableUsers(server: ServerEntry): Promise<SwitchableUser[]> {
    return (await this.client(AuthService, server).listSwitchableUsers({})).users;
  }

  /**
   * Switches a server's session to another user, with its PIN when it has
   * one; the session's token acts as that user from then on.
   */
  async switchUser(server: ServerEntry, userId: string, pin?: string): Promise<ServerEntry> {
    const r = await this.client(AuthService, server).switchUser({ userId, pin });
    const at = this.list.findIndex((s) => s.id === server.id);
    if (at < 0) throw new Error(`${server.name} was removed`);
    const entry = { ...this.list[at], userId: r.user?.id ?? userId, userName: r.user?.name ?? "" };
    this.set(this.list.with(at, entry));
    return entry;
  }

  // Mavio accounts: a session of the device key at mavio.dev.

  /** The base URL of mavio.dev's API and the client ID this web UI signs in as. */
  private async mavioClient(): Promise<{ api: string; clientId: string }> {
    if (this.appApi) return { api: this.appApi, clientId: appClient };
    const i = await this.probe(this.homeUrl());
    if (!i.mavioDevUrl) throw new Error(`${i.name} does not sign in with Mavio accounts`);
    return { api: i.mavioDevUrl, clientId: i.id };
  }

  /** The address of app.mavio.dev, where the accounts of a server other than its administrator watch. */
  async appUrl(): Promise<string> {
    const { api } = await this.mavioClient();
    const res = await fetch(`${siteOf(api)}/api/config`);
    if (!res.ok) throw new Error(`mavio.dev's configuration: ${res.statusText}`);
    return ((await res.json()) as { appURL: string }).appURL;
  }

  /** Where mavio.dev sends the browser back to. */
  callbackUri(): string {
    return `${this.base.replace(/\/+$/, "")}/auth/callback`;
  }

  private async pending(api: string, clientId: string, returnTo: string) {
    const p: Pending = { api, clientId, state: randomString(), verifier: randomString(), redirectUri: this.callbackUri(), returnTo };
    this.storage.setItem(pendingKey, JSON.stringify(p));
    return { ...p, challenge: await s256(p.verifier) };
  }

  /**
   * Starts signing a Mavio account in (docs/accounts.md §6): returns the
   * mavio.dev page to go to, which comes back to callbackUri, where
   * finishCallback completes it and returns returnTo. The serving server
   * pushes its web UI's request to mavio.dev, and signing in to a server
   * not claimed yet claims it; app.mavio.dev sends its own.
   */
  async startSignIn(returnTo = "/"): Promise<string> {
    const { api, clientId } = await this.mavioClient();
    const p = await this.pending(api, clientId, returnTo);
    if (clientId !== appClient) {
      const r = await createClient(AuthService, transport({ url: this.homeUrl() })).startSignIn({
        redirectUri: p.redirectUri,
        codeChallenge: p.challenge,
        deviceKey: await this.key.publicKey(),
        state: p.state,
      });
      return r.url;
    }
    const u = new URL(`${siteOf(api)}/authorize`);
    u.search = new URLSearchParams({
      client_id: clientId,
      redirect_uri: p.redirectUri,
      state: p.state,
      code_challenge: p.challenge,
      code_challenge_method: "S256",
      device_key: base64url(await this.key.publicKey()),
    }).toString();
    return u.toString();
  }

  /**
   * Completes a sign-in with the parameters mavio.dev came back
   * with: exchanges the code for a session of the device key and signs in
   * to the account's servers. Returns where the web UI goes next.
   */
  finishCallback(params: URLSearchParams): Promise<string> {
    // A page that completes it twice, as React's StrictMode does in
    // development, waits for the first time: the code is spent once.
    const key = params.toString();
    if (this.finishing?.key !== key) this.finishing = { key, done: this.doFinishCallback(params) };
    return this.finishing.done;
  }

  private async doFinishCallback(params: URLSearchParams): Promise<string> {
    const p = read<Pending>(this.storage, pendingKey);
    this.storage.setItem(pendingKey, "null");
    if (!p || params.get("state") !== p.state) throw new Error("this sign-in was not started here");
    const error = params.get("error");
    if (error) throw new OAuthError(error, params.get("error_description") ?? error);
    await this.startSession(
      p.api,
      p.clientId,
      await this.oauth(p.api, "token", {
        grant_type: "authorization_code",
        code: params.get("code") ?? "",
        client_id: p.clientId,
        redirect_uri: p.redirectUri,
        code_verifier: p.verifier,
      }),
    );
    await this.sync();
    return p.returnTo;
  }

  /** Starts signing in a television with a code an account approves at mavio.dev/link. */
  async startLink(): Promise<LinkCode> {
    const { api, clientId } = await this.mavioClient();
    const r = (await this.oauth(api, "device", { client_id: clientId, device_key: base64url(await this.key.publicKey()) })) as {
      device_code: string;
      user_code: string;
      verification_uri: string;
      verification_uri_complete: string;
      expires_in: number;
      interval: number;
    };
    return {
      userCode: r.user_code,
      verificationUri: r.verification_uri,
      verificationUriComplete: r.verification_uri_complete,
      deviceCode: r.device_code,
      interval: r.interval * 1000,
      expires: new Date(Date.now() + r.expires_in * 1000),
    };
  }

  /** Signs in once the link code was approved; false until then. */
  async pollLink(link: LinkCode): Promise<boolean> {
    const { api, clientId } = await this.mavioClient();
    let token: unknown;
    try {
      token = await this.oauth(api, "token", { grant_type: deviceCodeGrant, device_code: link.deviceCode, client_id: clientId });
    } catch (err) {
      if (err instanceof OAuthError && (err.code === "authorization_pending" || err.code === "slow_down")) return false;
      throw err;
    }
    await this.startSession(api, clientId, token);
    await this.sync();
    return true;
  }

  private async oauth(api: string, endpoint: string, fields: Record<string, string>): Promise<unknown> {
    const res = await fetch(`${api.replace(/\/+$/, "")}/oauth/${endpoint}`, { method: "POST", body: new URLSearchParams(fields) });
    const body = (await res.json().catch(() => ({}))) as { error?: string; error_description?: string };
    if (!res.ok) throw new OAuthError(body.error ?? "server_error", body.error_description ?? res.statusText);
    return body;
  }

  private async startSession(api: string, clientId: string, token: unknown) {
    const t = token as {
      access_token: string;
      expires_in: number;
      session_id: string;
      account: { id: string; userName?: string; displayName?: string };
    };
    // Another account's servers go with its sign-in.
    if (this.session && this.session.account.id !== t.account.id) await this.signOut();
    this.setSession({
      api,
      clientId,
      sessionId: t.session_id,
      token: t.access_token,
      expires: Date.now() + t.expires_in * 1000,
      account: { id: t.account.id, name: t.account.displayName || t.account.userName || "" },
    });
  }

  /** mavio.dev's access token, renewed with a signature of the device key when it is about to expire. */
  private accountToken(): Promise<string | undefined> {
    const s = this.session;
    if (!s) return Promise.resolve(undefined);
    if (s.expires - Date.now() > renewMargin) return Promise.resolve(s.token);
    this.renewing ??= this.renew(s).finally(() => (this.renewing = undefined));
    return this.renewing;
  }

  private async renew(s: AccountSession): Promise<string | undefined> {
    const timestamp = Date.now();
    const signature = base64url(await this.key.sign(renewalMessage(s.sessionId, timestamp)));
    let r: { access_token: string; expires_in: number };
    try {
      r = (await this.oauth(s.api, "token", {
        grant_type: renewGrant,
        session_id: s.sessionId,
        timestamp: String(timestamp),
        signature,
      })) as typeof r;
    } catch (err) {
      // A session mavio.dev ended signs the account out everywhere.
      if (err instanceof OAuthError && err.code === "invalid_grant") {
        await this.signOut();
        return undefined;
      }
      throw err;
    }
    if (this.session?.sessionId !== s.sessionId) return undefined;
    this.setSession({ ...s, token: r.access_token, expires: Date.now() + r.expires_in * 1000 });
    return r.access_token;
  }

  /** A Connect client of a service of mavio.dev, called with the account's session. */
  mavioDev<T extends DescService>(service: T): Client<T> {
    const s = this.session;
    if (!s) throw new Error("no Mavio account is signed in");
    return createClient(
      service,
      createConnectTransport({
        baseUrl: s.api,
        interceptors: [
          (next) => async (req) => {
            const token = await this.accountToken();
            if (!token) throw new ConnectError("signed out of mavio.dev", Code.Unauthenticated);
            req.header.set("Authorization", `Bearer ${token}`);
            return next(req);
          },
        ],
      }),
    );
  }

  /** The site of mavio.dev, where the account is managed. */
  mavioDevSite(): string | undefined {
    return this.session && siteOf(this.session.api);
  }

  /**
   * Follows the account's servers: signs in to those it newly reaches, at
   * the first of their URLs that answers, and forgets those it no longer
   * reaches. One sync runs at a time.
   */
  sync(): Promise<void> {
    this.syncing ??= this.doSync().finally(() => (this.syncing = undefined));
    return this.syncing;
  }

  private async doSync() {
    if (!this.session) return;
    const { servers } = await this.mavioDev(AccountService).listServers({});
    const homeId = this.home ? (await this.probe(this.home).catch(() => undefined))?.id : undefined;
    const known = new Map(this.list.map((s) => [s.id, s]));
    const next: ServerEntry[] = [];
    for (const s of servers) {
      const was = known.get(s.id);
      next.push({
        id: s.id,
        name: s.name,
        version: s.version,
        url: was?.url ?? "",
        urls: s.connections,
        userId: was?.userId ?? "",
        userName: was?.userName ?? "",
        mode: "mavioDev",
        sessionId: was?.sessionId,
        token: was?.token,
        expires: was?.expires,
        ...(s.id === homeId && this.home ? { url: this.home } : {}),
      });
    }
    // The user's order first, then the servers newly reached.
    const rank = (s: ServerEntry) => {
      const i = this.list.findIndex((x) => x.id === s.id);
      return i < 0 ? this.list.length : i;
    };
    const gone = this.list.filter((s) => !next.some((n) => n.id === s.id));
    this.set(next.toSorted((a, b) => rank(a) - rank(b)));
    await Promise.all(gone.map((s) => this.logout(s)));
    await Promise.all(
      this.list
        .filter((s) => !s.token)
        .map(async (s) => {
          try {
            await this.assert(s);
          } catch {
            // Unreachable for now; tried again with the next sync.
          }
        }),
    );
  }

  /** The first of a server's URLs that answers as the server, HTTPS only from an HTTPS page. */
  private async reach(s: ServerEntry): Promise<string | undefined> {
    const secure = typeof location !== "undefined" && location.protocol === "https:";
    const urls = (s.urls ?? []).filter((u) => !secure || u.startsWith("https:"));
    for (const u of urls) {
      try {
        if ((await this.probe(u, reachTimeout)).id === s.id) return normalizeUrl(u);
      } catch {
        // The next one.
      }
    }
    return undefined;
  }

  /** Signs the account in to a server with an assertion of mavio.dev. */
  private async assert(s: ServerEntry): Promise<ServerEntry> {
    const url = s.url || (await this.reach(s));
    if (!url) throw new Error(`${s.name} cannot be reached`);
    const { assertion } = await this.mavioDev(AccountService).createAssertion({ serverId: s.id, deviceKey: await this.key.publicKey() });
    const r = await createClient(AuthService, transport({ url })).signInWithAssertion({
      assertion,
      device: await device(this.storage, this.key),
      signature: await this.key.sign(assertionMessage(assertion)),
    });
    const at = this.list.findIndex((x) => x.id === s.id);
    if (at < 0) throw new Error(`${s.name} was removed`);
    const entry: ServerEntry = {
      ...this.list[at],
      url,
      userId: r.user?.id ?? "",
      userName: r.user?.name ?? "",
      sessionId: r.session?.id,
      token: r.accessToken,
      expires: expiry(r),
    };
    this.set(this.list.with(at, entry));
    return entry;
  }

  private async logout(s: ServerEntry) {
    if (!s.token) return;
    try {
      await createClient(AuthService, transport({ url: s.url, token: s.token })).logout({});
    } catch {
      // Its session ends once idle.
    }
  }

  /** Signs out: ends every server's session and the Mavio account's session at mavio.dev. */
  async signOut() {
    const list = this.list;
    const s = this.session;
    this.set([]);
    this.setSession(undefined);
    this.transports.clear();
    await Promise.all(list.map((x) => this.logout(x)));
    if (s) {
      try {
        await this.oauth(s.api, "revoke", { token: s.token });
      } catch {
        // It expires unused.
      }
    }
  }

  /** Orders the servers as the user arranged them. */
  order(ids: string[]) {
    const rank = (s: ServerEntry) => {
      const i = ids.indexOf(s.id);
      return i < 0 ? ids.length : i;
    };
    this.set(this.list.toSorted((a, b) => rank(a) - rank(b)));
  }
}
