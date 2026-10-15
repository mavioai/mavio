// The servers the web UI is signed in to (docs/clients.md §4.1, §5.4): the
// one serving it and those the user added, each with its own user and
// token, in the user's order, grouped into the sign-ins of
// docs/accounts.md §7, of which one is shown.
import type { DescService } from "@bufbuild/protobuf";
import { timestampDate } from "@bufbuild/protobuf/wkt";
import type { Timestamp } from "@bufbuild/protobuf/wkt";
import { type Client, Code, ConnectError, createClient, type Transport } from "@connectrpc/connect";
import { AccountMode, AuthService, type GetAuthInfoResponse, type SwitchableUser } from "@mavio/proto/mavio/auth/v1/auth_pb";
import { DisplayPreferencesService } from "@mavio/proto/mavio/user/v1/display_preferences_pb";

import { assertionMessage, browserDeviceKey, type DeviceKey, refreshMessage } from "./device-key.ts";
import { transport } from "./server.ts";

/** A server the user added. */
export interface ServerEntry {
  /** The server's own ID, the same at every address. */
  id: string;
  name: string;
  version: string;
  /** Its base URL, such as `https://media.example.com/mavio`. */
  url: string;
  userId: string;
  userName: string;
  /** How the server signs people in, as it told at sign-in. */
  mode?: "standalone" | "mavioDev";
  /** In mavio.dev mode, the Mavio account signed in. */
  account?: { id: string; name: string };
  /**
   * The key of the sign-in a standalone server was added to; unset when
   * it is a sign-in of its own.
   */
  signIn?: string;
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

/**
 * A sign-in the browser keeps (docs/accounts.md §7): a Mavio account with
 * the servers it is signed in to, or a user of one standalone server.
 */
export interface SignIn {
  /** `account:<ID>` or `server:<ID>`. */
  key: string;
  kind: "mavioDev" | "standalone";
  /** The account's or the user's name. */
  name: string;
  /** Its signed-in servers, in the user's order. */
  servers: ServerEntry[];
}

/** The key of the sign-in a server's session belongs to. */
export function signInKey(s: ServerEntry): string {
  return s.mode === "mavioDev" && s.account ? `account:${s.account.id}` : (s.signIn ?? `server:${s.id}`);
}

/** A mavio.dev sign-in on its way to mavio.dev and back. */
interface PendingSignIn {
  state: string;
  verifier: string;
  url: string;
  redirectUri: string;
}

/** A Quick Connect request: the user enters the code on a signed-in device. */
export interface QuickConnect {
  code: string;
  expires: Date;
  server: ServerInfo;
  secret: string;
}

/** Where the servers are kept in the browser. */
export interface Storage {
  getItem(key: string): string | null;
  setItem(key: string, value: string): void;
}

const serversKey = "mavio.servers";
const deviceKey = "mavio.device";
const shownKey = "mavio.signIn";
const pendingKey = "mavio.pendingSignIn";

/** The client and view of the display preferences holding the user's servers. */
const prefsClient = "mavio-web";
const prefsView = "servers";

/** How long before its expiry an access token is refreshed. */
const refreshMargin = 5 * 60_000;

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

/** Normalizes an address the user typed into a base URL. */
export function normalizeUrl(input: string, defaultPort = 8686): string {
  let s = input.trim().replace(/\/+$/, "");
  if (s === "") throw new Error("empty address");
  const typedScheme = s.includes("://");
  if (!typedScheme) s = `http://${s}`;
  const u = new URL(s);
  if (u.protocol !== "http:" && u.protocol !== "https:") throw new Error(`${input} is not http or https`);
  if (!typedScheme && u.port === "") u.port = String(defaultPort);
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

/**
 * The servers, kept in the browser with their sessions, and their list
 * without sessions in the display preferences of the serving server, so
 * that it follows the user across browsers. Each session belongs to the
 * browser's device key, which signs its refreshes. Observed with
 * useSyncExternalStore.
 */
export class Servers {
  private list: ServerEntry[];
  private shown: string | null;
  private listeners = new Set<() => void>();
  private transports = new Map<string, { sessionId?: string; url: string; transport: Transport }>();
  private refreshing = new Map<string, Promise<string | undefined>>();

  constructor(
    private storage: Storage,
    /** The base URL of the server serving the web UI. */
    readonly home: string,
    private key: DeviceKey = browserDeviceKey(),
  ) {
    try {
      this.list = JSON.parse(storage.getItem(serversKey) ?? "[]") as ServerEntry[];
    } catch {
      this.list = [];
    }
    this.shown = storage.getItem(shownKey);
  }

  subscribe = (listener: () => void) => {
    this.listeners.add(listener);
    return () => this.listeners.delete(listener);
  };

  /** The servers in the user's order; a stable array until they change. */
  snapshot = () => this.list;

  /** One server. */
  get(id: string): ServerEntry | undefined {
    return this.list.find((s) => s.id === id);
  }

  /** The servers the user is signed in to, in their order, in every sign-in. */
  signedIn(): ServerEntry[] {
    return this.list.filter((s) => s.token);
  }

  /** The sign-ins kept, in the order of their first servers. */
  signIns(): SignIn[] {
    const out = new Map<string, SignIn>();
    for (const s of this.signedIn()) {
      const key = signInKey(s);
      const known = out.get(key);
      if (known) known.servers.push(s);
      else out.set(key, { key, kind: key.startsWith("account:") ? "mavioDev" : "standalone", name: "", servers: [s] });
    }
    // Named by the account, or by the user of the server whose sign-in it is.
    for (const x of out.values()) {
      const own = x.kind === "mavioDev" ? x.servers.find((s) => s.account) : x.servers.find((s) => x.key === `server:${s.id}`);
      x.name = (x.kind === "mavioDev" ? own?.account?.name : own?.userName) ?? x.servers[0].userName;
    }
    return [...out.values()];
  }

  /** The key the user chose to show; read with shownSignIn. */
  shownSnapshot = () => this.shown;

  /**
   * The sign-in shown: the one the user chose, else the one of the serving
   * server, else the first.
   */
  shownSignIn(): SignIn | undefined {
    const all = this.signIns();
    const home = this.homeServer();
    return (
      all.find((x) => x.key === this.shown) ??
      (home?.token ? all.find((x) => x.key === signInKey(home)) : undefined) ??
      all[0]
    );
  }

  /** Shows another kept sign-in. */
  show(key: string) {
    this.shown = key;
    this.storage.setItem(shownKey, key);
    for (const l of this.listeners) l();
  }

  /**
   * The Connect client of a service on a server, whose access token is
   * refreshed as it expires; a session the server ended signs the user out
   * of it.
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
   * refreshes signs the user out of it.
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
    if (!s?.sessionId) {
      this.forgetToken(id);
      return undefined;
    }
    const sessionId = s.sessionId;
    const auth = createClient(AuthService, transport({ url: s.url }));
    try {
      const { nonce } = await auth.startRefresh({ sessionId });
      const signature = await this.key.sign(refreshMessage(sessionId, nonce));
      const r = await auth.refreshSession({ sessionId, nonce, signature });
      const at = this.list.findIndex((x) => x.id === id);
      if (at < 0 || this.list[at].sessionId !== sessionId) return undefined;
      this.set(this.list.with(at, { ...this.list[at], token: r.accessToken, expires: expiry(r) }), false);
      return r.accessToken;
    } catch (err) {
      if (ConnectError.from(err).code === Code.Unauthenticated) {
        this.forgetToken(id);
        return undefined;
      }
      throw err;
    }
  }

  /** The server serving the web UI, once added. */
  homeServer(): ServerEntry | undefined {
    return this.list.find((s) => s.url === this.home);
  }

  private set(list: ServerEntry[], sync = true) {
    this.list = list;
    this.storage.setItem(serversKey, JSON.stringify(list));
    for (const l of this.listeners) l();
    if (sync) void this.save();
  }

  /** Writes the list, without sessions, to the serving server's preferences. */
  private async save() {
    const home = this.homeServer();
    if (!home?.token) return;
    const list = this.list.map(({ sessionId: _, token: __, expires: ___, ...s }) => s);
    try {
      await this.client(DisplayPreferencesService, home).setDisplayPreferences({
        client: prefsClient,
        view: prefsView,
        values: { list: JSON.stringify(list) },
      });
    } catch {
      // Kept in the browser; written with the next change.
    }
  }

  /** Adds the servers the user added in other browsers, signed out here. */
  async load() {
    const home = this.homeServer();
    if (!home?.token) return;
    const prefs = await this.client(DisplayPreferencesService, home).getDisplayPreferences({
      client: prefsClient,
      view: prefsView,
    });
    const raw = prefs.preferences?.values.list;
    if (!raw) return;
    let kept: ServerEntry[];
    try {
      kept = JSON.parse(raw) as ServerEntry[];
    } catch {
      return;
    }
    const merged = kept.map((k) => {
      const here = this.get(k.id);
      return { ...k, ...here, sessionId: here?.sessionId, token: here?.token, expires: here?.expires };
    });
    const rest = this.list.filter((s) => !kept.some((k) => k.id === s.id));
    this.set([...merged, ...rest], false);
  }

  /** Asks the server at an address who it is. */
  async probe(address: string): Promise<ServerInfo> {
    const url = normalizeUrl(address);
    return info(url, await createClient(AuthService, transport({ url })).getAuthInfo({}));
  }

  private added(i: ServerInfo, r: SignedIn, account?: { id: string; name: string }): ServerEntry {
    const entry: ServerEntry = {
      id: i.id,
      name: i.name,
      version: i.version,
      url: i.url,
      mode: i.mode,
      account,
      // Signing in again keeps the sign-in it was added to.
      signIn: this.get(i.id)?.signIn,
      userId: r.user?.id ?? "",
      userName: r.user?.name ?? "",
      sessionId: r.session?.id,
      token: r.accessToken,
      expires: expiry(r),
    };
    const at = this.list.findIndex((s) => s.id === i.id);
    this.set(at < 0 ? [...this.list, entry] : this.list.with(at, entry));
    // A new sign-in is shown.
    this.show(signInKey(entry));
    return entry;
  }

  /**
   * Signs in to the server at an address with a name and password, and the
   * current code of the user's TOTP when it has one; without the code
   * such a user's sign-in fails as isTotpRequired tells.
   */
  async signIn(address: string, name: string, password: string, totpCode?: string): Promise<ServerEntry> {
    const i = await this.probe(address);
    const r = await createClient(AuthService, transport({ url: i.url })).login({
      name,
      password,
      totpCode,
      device: await device(this.storage, this.key),
    });
    return this.added(i, r);
  }

  /** Signs in to the server at an address with a passkey of the browser's choosing. */
  async signInWithPasskey(address: string): Promise<ServerEntry> {
    const i = await this.probe(address);
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
    this.set(this.list.with(at, entry), false);
    return entry;
  }

  /**
   * Sets up a new server in mavio.dev mode with a claim code from
   * mavio.dev/claim, which makes its account the administrator; the
   * account then signs in with startMavioDevSignIn.
   */
  async claimServer(address: string, claimCode: string): Promise<ServerInfo> {
    const i = await this.probe(address);
    await createClient(AuthService, transport({ url: i.url })).claimServer({ claimCode: claimCode.trim() });
    return this.probe(i.url);
  }

  /**
   * Starts signing a Mavio account in to the server at an address
   * (docs/accounts.md §6): returns the mavio.dev page to go to, which comes
   * back to redirectUri, where finishMavioDevSignIn completes it.
   */
  async startMavioDevSignIn(address: string, redirectUri: string): Promise<string> {
    const i = await this.probe(address);
    if (i.mode !== "mavioDev" || !i.mavioDevUrl) throw new Error(`${i.name} does not sign in with Mavio accounts`);
    const verifier = randomString();
    const state = randomString();
    const challenge = base64url(new Uint8Array(await crypto.subtle.digest("SHA-256", new TextEncoder().encode(verifier))));
    const pending: PendingSignIn = { state, verifier, url: i.url, redirectUri };
    this.storage.setItem(pendingKey, JSON.stringify(pending));
    const u = new URL(`${i.mavioDevUrl.replace(/\/api\/?$/, "")}/authorize`);
    u.search = new URLSearchParams({
      client_id: i.id,
      redirect_uri: redirectUri,
      state,
      code_challenge: challenge,
      code_challenge_method: "S256",
      device_key: base64url(await this.key.publicKey()),
    }).toString();
    return u.toString();
  }

  /**
   * Completes a mavio.dev sign-in with the parameters mavio.dev came back
   * with: exchanges the code for a sign-in assertion and signs in to the
   * server with it.
   */
  async finishMavioDevSignIn(params: URLSearchParams): Promise<ServerEntry> {
    let pending: PendingSignIn | undefined;
    try {
      pending = JSON.parse(this.storage.getItem(pendingKey) ?? "null") ?? undefined;
    } catch {
      pending = undefined;
    }
    this.storage.setItem(pendingKey, "null");
    if (!pending || params.get("state") !== pending.state) throw new Error("this sign-in was not started here");
    if (params.get("error")) throw new Error(params.get("error_description") ?? params.get("error") ?? "");
    const i = await this.probe(pending.url);
    if (!i.mavioDevUrl) throw new Error(`${i.name} does not sign in with Mavio accounts`);
    const res = await fetch(`${i.mavioDevUrl.replace(/\/$/, "")}/oauth/token`, {
      method: "POST",
      body: new URLSearchParams({
        grant_type: "authorization_code",
        code: params.get("code") ?? "",
        client_id: i.id,
        redirect_uri: pending.redirectUri,
        code_verifier: pending.verifier,
      }),
    });
    const token = (await res.json()) as {
      assertion?: string;
      account?: { id: string; userName?: string; displayName?: string };
      error_description?: string;
    };
    if (!res.ok || !token.assertion || !token.account) throw new Error(token.error_description ?? res.statusText);
    const r = await createClient(AuthService, transport({ url: i.url })).signInWithAssertion({
      assertion: token.assertion,
      device: await device(this.storage, this.key),
      signature: await this.key.sign(assertionMessage(token.assertion)),
    });
    const name = token.account.displayName || token.account.userName || "";
    return this.added(i, r, { id: token.account.id, name });
  }

  /** Creates the first user of a new server and signs in as them. */
  async createFirstUser(address: string, name: string, password: string): Promise<ServerEntry> {
    const i = await this.probe(address);
    const r = await createClient(AuthService, transport({ url: i.url })).createFirstUser({
      name,
      password,
      device: await device(this.storage, this.key),
    });
    return this.added(i, r);
  }

  /** Starts signing in with Quick Connect. */
  async startQuickConnect(address: string): Promise<QuickConnect> {
    const i = await this.probe(address);
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

  /**
   * Adds a standalone server signed in to a kept sign-in, such as a Mavio
   * account's, whose servers it then joins; the sign-in is shown.
   */
  addTo(id: string, key: string) {
    const at = this.list.findIndex((s) => s.id === id);
    if (at < 0 || this.list[at].mode === "mavioDev") return;
    const own = key === `server:${id}`;
    const { signIn: _, ...rest } = this.list[at];
    this.set(this.list.with(at, own ? rest : { ...rest, signIn: key }));
    this.show(key);
  }

  /** Forgets the session of a server that ended it. */
  forgetToken(id: string) {
    const at = this.list.findIndex((s) => s.id === id);
    if (at >= 0 && this.list[at].token) {
      const { sessionId: _, token: __, expires: ___, ...rest } = this.list[at];
      this.set(this.list.with(at, rest), false);
    }
  }

  /** Removes a kept sign-in: signs out of its servers, which stay listed. */
  async signOut(key: string) {
    const sessions = this.signedIn().filter((s) => signInKey(s) === key);
    await Promise.all(
      sessions.map(async (s) => {
        try {
          await this.client(AuthService, s).logout({});
        } catch {
          // Its session ends once idle.
        }
        this.forgetToken(s.id);
      }),
    );
  }

  /** Signs out of a server and forgets it. */
  async remove(id: string) {
    const s = this.get(id);
    if (s?.token) {
      try {
        await this.client(AuthService, s).logout({});
      } catch {
        // Its session ends once idle.
      }
    }
    this.set(this.list.filter((x) => x.id !== id));
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
