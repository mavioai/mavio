// Home across servers (docs/clients.md §5.4), as the shared core builds it
// for the apps (libs/client-rust/core/src/home.rs): one Continue Watching
// across servers by last activity; a title on several servers once, from
// the server the user played it on last, else the first in their order;
// the other hubs server by server; Trending once.
import { timestampDate } from "@bufbuild/protobuf/wkt";
import type { DiscoveredTitle } from "@mavio/proto/mavio/acquisition/v1/discover_pb";
import { MediaKind } from "@mavio/proto/mavio/acquisition/v1/profile_pb";
import type { Hub } from "@mavio/proto/mavio/home/v1/home_pb";
import { type Item, ItemKind } from "@mavio/proto/mavio/library/v1/item_pb";
import { HubKind } from "@mavio/proto/mavio/library/v1/library_pb";

/** The providers whose IDs tell that two servers have the same title. */
const matchingProviders = ["tmdb", "tvdb", "imdb"];

/** What is known of one server's hubs. */
export type Hubs =
  /** The server has not answered yet, and nothing is kept of it. */
  | { state: "loading" }
  /** The server's hubs as it answered. */
  | { state: "fresh"; hubs: Hub[] }
  /** Hubs kept from an earlier answer: the server is unreachable. */
  | { state: "cached"; hubs: Hub[] }
  /** The server is unreachable and nothing is kept of it. */
  | { state: "unavailable"; reason: string };

/** One server's part of Home, in the user's order of servers. */
export interface ServerHubs {
  serverId: string;
  serverName: string;
  hubs: Hubs;
}

/** Another server's copy of a title. */
export interface Version {
  serverId: string;
  itemId: string;
}

/** An item on Home, with the other servers' copies of the same title. */
export interface Entry {
  serverId: string;
  item: Item;
  versions: Version[];
}

/** A trending title, with the servers that have it. */
export interface Trending {
  title: DiscoveredTitle;
  available: Version[];
}

/** What a section shows. */
export type Content =
  | { type: "items"; entries: Entry[] }
  | { type: "titles"; titles: Trending[] }
  | { type: "loading" }
  | { type: "unavailable"; reason: string };

/** A row of Home. */
export interface Section {
  /** Stable across updates: "continue-watching", "trending", or the server's ID and its hub's. */
  id: string;
  /** Unspecified for a server's placeholder. */
  kind: HubKind;
  /** Unset for Continue Watching. */
  serverId?: string;
  serverName?: string;
  /** The hub's ID on its server, which pages its items. */
  hubId?: string;
  libraryName?: string;
  seedName?: string;
  more: boolean;
  /** Whether the section comes from kept hubs. */
  offline: boolean;
  content: Content;
}

function hubsOf(h: Hubs): Hub[] {
  return h.state === "fresh" || h.state === "cached" ? h.hubs : [];
}

function keys(kind: string, ids: { [provider: string]: string }): string[] {
  return Object.entries(ids)
    .filter(([p, v]) => matchingProviders.includes(p) && v !== "")
    .map(([p, v]) => `${kind}/${p}/${v}`)
    .sort();
}

function itemClass(item: Item): string {
  switch (item.kind) {
    case ItemKind.MOVIE:
      return "movie";
    case ItemKind.SERIES:
      return "series";
    case ItemKind.EPISODE:
      return "episode";
    case ItemKind.SEASON:
      return "season";
    case ItemKind.MUSIC_ALBUM:
      return "album";
    default:
      return "other";
  }
}

const itemKeys = (item: Item) => keys(itemClass(item), item.externalIds);

function titleKeys(title: DiscoveredTitle): string[] {
  const kind =
    title.mediaKind === MediaKind.MOVIE ? "movie" : title.mediaKind === MediaKind.SERIES ? "series" : "other";
  return keys(kind, title.externalIds);
}

/** When the user last played an item, in milliseconds. */
function lastPlayed(item: Item): number | undefined {
  const t = item.userData?.lastPlayedTime;
  return t ? timestampDate(t).getTime() : undefined;
}

/** Compares activities, the latest first and none last. */
function byActivity(a: number | undefined, b: number | undefined): number {
  return (b ?? -Infinity) - (a ?? -Infinity);
}

interface Copy {
  server: number;
  itemId: string;
  played?: number;
}

/**
 * Where each title is: its copies on the servers, grouped by matching keys,
 * with union-find merging groups an item links by keys of both.
 */
class Titles {
  private groups = new Map<string, number>();
  private parents: number[] = [];
  private copies: Copy[][] = [];

  private root(g: number): number {
    while (this.parents[g] !== g) {
      this.parents[g] = this.parents[this.parents[g]];
      g = this.parents[g];
    }
    return g;
  }

  private group(ks: string[]): number | undefined {
    if (ks.length === 0) return undefined;
    let found: number | undefined;
    for (const k of ks) {
      const g0 = this.groups.get(k);
      if (g0 === undefined) continue;
      const g = this.root(g0);
      if (found !== undefined && found !== g) {
        this.copies[found].push(...this.copies[g]);
        this.copies[g] = [];
        this.parents[g] = found;
      } else {
        found = g;
      }
    }
    if (found === undefined) {
      found = this.parents.length;
      this.parents.push(found);
      this.copies.push([]);
    }
    for (const k of ks) this.groups.set(k, found);
    return found;
  }

  add(server: number, item: Item) {
    const g = this.group(itemKeys(item));
    if (g === undefined) return;
    const copies = this.copies[g];
    if (!copies.some((c) => c.server === server && c.itemId === item.id)) {
      copies.push({ server, itemId: item.id, played: lastPlayed(item) });
    }
  }

  of(ks: string[]): number | undefined {
    for (const k of ks) {
      const g = this.groups.get(k);
      if (g !== undefined) return this.root(g);
    }
    return undefined;
  }

  /** The server whose copy Home shows: the one played last, else the first in order. */
  preferred(g: number): number {
    const copies = [...this.copies[this.root(g)]];
    copies.sort((a, b) => byActivity(a.played, b.played) || a.server - b.server);
    return copies[0]?.server ?? -1;
  }

  versions(g: number, servers: ServerHubs[], except: number): Version[] {
    const out: Version[] = [];
    for (const c of this.copies[this.root(g)]) {
      const serverId = servers[c.server].serverId;
      if (c.server !== except && !out.some((v) => v.serverId === serverId)) {
        out.push({ serverId, itemId: c.itemId });
      }
    }
    return out;
  }

  /** The entry of an item of server s, or undefined when another server shows its title. */
  entry(servers: ServerHubs[], s: number, item: Item): Entry | undefined {
    const g = this.of(itemKeys(item));
    if (g !== undefined && this.preferred(g) !== s) return undefined;
    return {
      serverId: servers[s].serverId,
      item,
      versions: g === undefined ? [] : this.versions(g, servers, s),
    };
  }
}

function section(id: string, kind: HubKind, content: Content): Section {
  return { id, kind, more: false, offline: false, content };
}

/** Merges the servers' hubs into the sections of Home, the servers in the user's order. */
export function merge(servers: ServerHubs[]): Section[] {
  const titles = new Titles();
  servers.forEach((server, s) => {
    for (const hub of hubsOf(server.hubs)) for (const item of hub.items) titles.add(s, item);
  });
  const out: Section[] = [];
  const cw = continueWatching(servers, titles);
  if (cw) out.push(cw);
  servers.forEach((_, s) => out.push(...serverSections(servers, s, titles)));
  const tr = trending(servers, titles);
  if (tr) out.push(tr);
  return out;
}

/**
 * One Continue Watching across servers, by last activity: an item without
 * its own, such as a next episode, takes that of the item before it on its
 * server.
 */
function continueWatching(servers: ServerHubs[], titles: Titles): Section | undefined {
  const entries: { activity?: number; entry: Entry }[] = [];
  let offline = false;
  servers.forEach((server, s) => {
    const hub = hubsOf(server.hubs).find((h) => h.kind === HubKind.CONTINUE_WATCHING);
    if (!hub) return;
    offline ||= server.hubs.state === "cached";
    let activity: number | undefined;
    for (const item of hub.items) {
      activity = lastPlayed(item) ?? activity;
      const entry = titles.entry(servers, s, item);
      if (entry) entries.push({ activity, entry });
    }
  });
  if (entries.length === 0) return undefined;
  // Array.prototype.sort is stable: equal activity keeps the servers' order.
  entries.sort((a, b) => byActivity(a.activity, b.activity));
  const out = section("continue-watching", HubKind.CONTINUE_WATCHING, {
    type: "items",
    entries: entries.map((e) => e.entry),
  });
  out.offline = offline;
  return out;
}

/**
 * A server's hubs other than Continue Watching and Trending, without the
 * titles Home shows from other servers; a placeholder while it loads or
 * when it is unreachable with nothing kept.
 */
function serverSections(servers: ServerHubs[], s: number, titles: Titles): Section[] {
  const server = servers[s];
  const named = (sec: Section): Section => ({ ...sec, serverId: server.serverId, serverName: server.serverName });
  if (server.hubs.state === "loading") {
    return [named(section(`${server.serverId}/`, HubKind.UNSPECIFIED, { type: "loading" }))];
  }
  if (server.hubs.state === "unavailable") {
    return [
      named(
        section(`${server.serverId}/`, HubKind.UNSPECIFIED, { type: "unavailable", reason: server.hubs.reason }),
      ),
    ];
  }
  const offline = server.hubs.state === "cached";
  const out: Section[] = [];
  for (const hub of server.hubs.hubs) {
    // A kind newer than the client has no name to show.
    if (!(hub.kind in HubKind) || hub.kind === HubKind.UNSPECIFIED) continue;
    if (hub.kind === HubKind.CONTINUE_WATCHING || hub.kind === HubKind.TRENDING) continue;
    const entries = hub.items.flatMap((item) => titles.entry(servers, s, item) ?? []);
    if (entries.length === 0) continue;
    out.push({
      ...named(section(`${server.serverId}/${hub.id}`, hub.kind, { type: "items", entries })),
      hubId: hub.id,
      libraryName: hub.libraryName || undefined,
      seedName: hub.seedName || undefined,
      more: hub.more,
      offline,
    });
  }
  return out;
}

/** Trending once, from the first server that discovers, its titles marked available where any server has them. */
function trending(servers: ServerHubs[], titles: Titles): Section | undefined {
  for (const server of servers) {
    const hub = hubsOf(server.hubs).find((h) => h.kind === HubKind.TRENDING && h.titles.length > 0);
    if (!hub) continue;
    const list = hub.titles.map((title) => {
      const g = titles.of(titleKeys(title));
      return { title, available: g === undefined ? [] : titles.versions(g, servers, -1) };
    });
    return {
      ...section("trending", HubKind.TRENDING, { type: "titles", titles: list }),
      serverId: server.serverId,
      serverName: server.serverName,
      hubId: hub.id,
      more: hub.more,
      offline: server.hubs.state === "cached",
    };
  }
  return undefined;
}
