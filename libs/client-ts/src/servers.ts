// The servers the web UI is signed in to (docs/clients.md §4.1, §5.4): the
// one serving it and those the user added, each with its own user and
// token, in the user's order.
import type { DescService } from "@bufbuild/protobuf";
import { timestampDate } from "@bufbuild/protobuf/wkt";
import { type Client, createClient, type Transport } from "@connectrpc/connect";
import { AuthService, type GetAuthInfoResponse } from "@mavio/proto/mavio/auth/v1/auth_pb";
import { DisplayPreferencesService } from "@mavio/proto/mavio/user/v1/display_preferences_pb";

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
  /** The token of this browser's session; unset when signed out. */
  token?: string;
}

/** What a server tells before anyone signs in. */
export interface ServerInfo {
  id: string;
  name: string;
  version: string;
  url: string;
  setupRequired: boolean;
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

/** The client and view of the display preferences holding the user's servers. */
const prefsClient = "mavio-web";
const prefsView = "servers";

/** The device as servers list it among the user's devices. */
function device(storage: Storage) {
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
  return { id, name: browser, client: "Mavio Web", clientVersion: "0.0.0" };
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

function info(url: string, r: GetAuthInfoResponse): ServerInfo {
  if (!r.serverId) throw new Error(`${url} is not a Mavio server`);
  return { id: r.serverId, name: r.serverName, version: r.version, url, setupRequired: r.setupRequired };
}

/**
 * The servers, kept in the browser with their tokens, and their list
 * without tokens in the display preferences of the serving server, so that
 * it follows the user across browsers. Observed with useSyncExternalStore.
 */
export class Servers {
  private list: ServerEntry[];
  private listeners = new Set<() => void>();
  private transports = new Map<string, { token?: string; url: string; transport: Transport }>();

  constructor(
    private storage: Storage,
    /** The base URL of the server serving the web UI. */
    readonly home: string,
  ) {
    try {
      this.list = JSON.parse(storage.getItem(serversKey) ?? "[]") as ServerEntry[];
    } catch {
      this.list = [];
    }
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

  /** The servers the user is signed in to, in their order. */
  signedIn(): ServerEntry[] {
    return this.list.filter((s) => s.token);
  }

  /** The Connect client of a service on a server; a rejected token signs the user out of it. */
  client<T extends DescService>(service: T, server: ServerEntry): Client<T> {
    let t = this.transports.get(server.id);
    if (!t || t.token !== server.token || t.url !== server.url) {
      t = {
        token: server.token,
        url: server.url,
        transport: transport(server, () => this.forgetToken(server.id)),
      };
      this.transports.set(server.id, t);
    }
    return createClient(service, t.transport);
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

  /** Writes the list, without tokens, to the serving server's preferences. */
  private async save() {
    const home = this.homeServer();
    if (!home?.token) return;
    const list = this.list.map(({ token: _, ...s }) => s);
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
    const merged = kept.map((k) => ({ ...k, ...this.get(k.id), token: this.get(k.id)?.token }));
    const rest = this.list.filter((s) => !kept.some((k) => k.id === s.id));
    this.set([...merged, ...rest], false);
  }

  /** Asks the server at an address who it is. */
  async probe(address: string): Promise<ServerInfo> {
    const url = normalizeUrl(address);
    return info(url, await createClient(AuthService, transport({ url })).getAuthInfo({}));
  }

  private added(i: ServerInfo, token: string, user?: { id: string; name: string }): ServerEntry {
    const entry: ServerEntry = {
      id: i.id,
      name: i.name,
      version: i.version,
      url: i.url,
      userId: user?.id ?? "",
      userName: user?.name ?? "",
      token,
    };
    const at = this.list.findIndex((s) => s.id === i.id);
    this.set(at < 0 ? [...this.list, entry] : this.list.with(at, entry));
    return entry;
  }

  /** Signs in to the server at an address with a name and password. */
  async signIn(address: string, name: string, password: string): Promise<ServerEntry> {
    const i = await this.probe(address);
    const r = await createClient(AuthService, transport({ url: i.url })).login({
      name,
      password,
      device: device(this.storage),
    });
    return this.added(i, r.accessToken, r.user);
  }

  /** Creates the first user of a new server and signs in as them. */
  async createFirstUser(address: string, name: string, password: string): Promise<ServerEntry> {
    const i = await this.probe(address);
    const r = await createClient(AuthService, transport({ url: i.url })).createFirstUser({
      name,
      password,
      device: device(this.storage),
    });
    return this.added(i, r.accessToken, r.user);
  }

  /** Starts signing in with Quick Connect. */
  async startQuickConnect(address: string): Promise<QuickConnect> {
    const i = await this.probe(address);
    const r = await createClient(AuthService, transport({ url: i.url })).startQuickConnect({
      device: device(this.storage),
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
    return this.added(qc.server, r.accessToken, r.user);
  }

  /** Signs in the device showing a Quick Connect code as the user of a server. */
  async authorizeQuickConnect(server: ServerEntry, code: string) {
    await this.client(AuthService, server).authorizeQuickConnect({ code: code.trim() });
  }

  /** Forgets the token of a server the token of which it rejected. */
  forgetToken(id: string) {
    const at = this.list.findIndex((s) => s.id === id);
    if (at >= 0 && this.list[at].token) this.set(this.list.with(at, { ...this.list[at], token: undefined }), false);
  }

  /** Signs out of a server and forgets it. */
  async remove(id: string) {
    const s = this.get(id);
    if (s?.token) {
      try {
        await this.client(AuthService, s).logout({});
      } catch {
        // Its session ends with its token unused.
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
