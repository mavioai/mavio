import { Code } from "@connectrpc/connect";
import { timestampDate } from "@bufbuild/protobuf/wkt";
import { AccountService } from "@mavio/proto/mavio/cloud/v1/account_pb";
import { describe, expect, test } from "vitest";

import { schema } from "../src/server/db/index.ts";
import { relayTicket } from "../src/server/relay.ts";
import { claimed, code, relaySecret } from "./harness.ts";

describe("relay", () => {
  test("tickets match apps/relay's", async () => {
    // apps/relay/internal/relay/ticket_test.go's vector.
    expect(await relayTicket(relaySecret, "abcdefghijklmnopqrstuvwxyz234567", "eu", 1_800_000_000)).toBe(
      "nne5eaf3rf7i7gajowc76q6yypqckda5",
    );
  });

  test("servers list relays and record their region", async () => {
    const { site, server, api } = await claimed();
    const { relays } = await api.listRelays({});
    expect(relays.map((r) => [r.region, r.url])).toEqual([["eu", "https://relay-eu.test"]]);
    await api.setRelayRegion({ region: "eu" });
    const row = await site.db.query.servers.findFirst({ where: (s, { eq }) => eq(s.id, server.id) });
    expect(row?.relayRegion).toBe("eu");
    expect(await code(api.setRelayRegion({ region: "us" }))).toBe(Code.InvalidArgument);
    expect(await code(api.setRelayRegion({ region: "e-u" }))).toBe(Code.InvalidArgument);
    await api.setRelayRegion({ region: "" });
    const cleared = await site.db.query.servers.findFirst({ where: (s, { eq }) => eq(s.id, server.id) });
    expect(cleared?.relayRegion).toBe("");
  });

  test("tickets need access, a relay connection and a Mavio Pass", async () => {
    const { site, alice, asAlice, server, api } = await claimed();
    const ticket = () => asAlice.createRelayTicket({ serverId: server.id });
    expect(await code(ticket())).toBe(Code.FailedPrecondition);
    await api.setRelayRegion({ region: "eu" });
    expect(await code(ticket())).toBe(Code.FailedPrecondition);
    await site.db.insert(schema.passes).values({ accountId: alice.id, expiresAt: new Date(Date.now() + 86_400_000) });
    const before = Date.now();
    const resp = await ticket();
    const m = /^relay-eu-([a-z2-7]{32})\.([a-z2-7]{32})\.mavio\.site$/.exec(resp.host);
    expect(m?.[2]).toBe(server.id);
    const exp = timestampDate(resp.expireTime!).getTime();
    expect(exp).toBeGreaterThan(before + 290_000);
    expect(m?.[1]).toBe(await relayTicket(relaySecret, server.id, "eu", exp / 1000));

    // Bob has no access, then reaches the server with alice's Pass.
    const bob = await site.signUp("bob");
    const asBob = site.client(AccountService, bob.headers);
    expect(await code(asBob.createRelayTicket({ serverId: server.id }))).toBe(Code.PermissionDenied);
    await asAlice.grantAccess({ serverId: server.id, userName: "bob", access: { allLibraries: true } });
    await asBob.acceptInvitation({ serverId: server.id });
    expect((await asBob.createRelayTicket({ serverId: server.id })).host).toMatch(/^relay-eu-/);
  });
});
