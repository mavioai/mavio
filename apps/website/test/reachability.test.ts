import { describe, expect, test } from "vitest";

import { checkable, due, isPublic, maxAge, minInterval } from "../src/server/reachability.ts";
import { claimed } from "./harness.ts";

const id = "abcdefghijklmnopqrstuvwxyz234567";

describe("published URLs", () => {
  test("are checked only at public addresses", () => {
    const cases: [string, { host: string; port: number } | undefined][] = [
      [`https://203-0-113-5.${id}.s.mavio.fyi:8920`, undefined],
      [`https://8-8-8-8.${id}.s.mavio.fyi:8920`, { host: `8-8-8-8.${id}.s.mavio.fyi`, port: 8920 }],
      [`https://192-168-1-20.${id}.s.mavio.fyi:8920`, undefined],
      [`https://2606-4700--1111.${id}.s.mavio.fyi`, { host: `2606-4700--1111.${id}.s.mavio.fyi`, port: 443 }],
      [`https://fd12-3456--1.${id}.s.mavio.fyi`, undefined],
      [`https://relay-hil-abc.${id}.s.mavio.fyi`, { host: `relay-hil-abc.${id}.s.mavio.fyi`, port: 443 }],
      ["https://media.example.com", { host: "media.example.com", port: 443 }],
      ["http://media.example.com:8686", { host: "media.example.com", port: 8686 }],
      ["http://8.8.4.4:8686", { host: "8.8.4.4", port: 8686 }],
      ["http://10.0.0.2:8686", undefined],
      ["http://100.100.1.1:8686", undefined],
      ["http://[2001:db8::1]:8686", undefined],
      ["http://[::1]:8686", undefined],
      ["http://localhost:8686", undefined],
      ["http://nas.local:8686", undefined],
      ["http://nas:8686", undefined],
      ["ftp://media.example.com", undefined],
      ["not a url", undefined],
    ];
    for (const [url, want] of cases) expect(checkable(url), url).toEqual(want);
    expect(isPublic("1.1.1.1")).toBe(true);
    expect(isPublic("172.20.0.1")).toBe(false);
    expect(isPublic("2606:4700::1111")).toBe(true);
    expect(isPublic("fe80::1")).toBe(false);
  });

  test("are checked when unchecked or old, never twice within a minute", () => {
    const at = new Date("2026-11-12T00:00:00Z");
    const url = "https://media.example.com";
    const checked = [{ url, reachable: true, checkedAt: at.toISOString() }];
    const ago = (ms: number) => new Date(at.getTime() - ms);
    const cases = [
      { name: "never checked", s: { connections: [url], connectionChecks: [], connectionsCheckedAt: null }, want: true },
      { name: "only private URLs", s: { connections: ["http://10.0.0.2:8686"], connectionChecks: [], connectionsCheckedAt: null }, want: false },
      { name: "a new URL", s: { connections: [url, "https://b.example.com"], connectionChecks: checked, connectionsCheckedAt: ago(2 * minInterval) }, want: true },
      { name: "a new URL within a minute", s: { connections: ["https://b.example.com"], connectionChecks: checked, connectionsCheckedAt: ago(1000) }, want: false },
      { name: "fresh", s: { connections: [url], connectionChecks: checked, connectionsCheckedAt: ago(2 * minInterval) }, want: false },
      { name: "old", s: { connections: [url], connectionChecks: checked, connectionsCheckedAt: ago(maxAge + 1) }, want: true },
    ];
    for (const c of cases) expect(due(c.s, at), c.name).toBe(c.want);
  });

  test("are told to accounts with what the check found", async () => {
    const { site, api, asAlice } = await claimed();
    site.listening.add("media.example.com:443");
    const urls = ["http://192.168.1.20:8686", "https://media.example.com", "https://8-8-8-8.x.s.mavio.fyi:8920"];
    await api.publishConnections({ urls });
    await site.settle();
    expect(site.dialed.toSorted()).toEqual(["8-8-8-8.x.s.mavio.fyi:8920", "media.example.com:443"]);
    const { servers } = await asAlice.listServers({});
    expect(servers[0].connections).toEqual(urls);
    expect(servers[0].connectionChecks.map((c) => [c.url, c.reachable])).toEqual([
      ["https://media.example.com", true],
      ["https://8-8-8-8.x.s.mavio.fyi:8920", false],
    ]);

    // Publishing again within a minute checks nothing; a URL no longer published is not told.
    await api.publishConnections({ urls: ["https://media.example.com", "https://other.example.com"] });
    await asAlice.listServers({});
    await site.settle();
    expect(site.dialed).toHaveLength(2);
    const after = (await asAlice.listServers({})).servers[0].connectionChecks;
    expect(after.map((c) => c.url)).toEqual(["https://media.example.com"]);
  });
});
