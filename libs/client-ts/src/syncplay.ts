// SyncPlay (docs/clients.md §6.3.24): the groups of devices watching
// together, the client's clock kept in step with the server's as in NTP,
// and where a group is at a moment.
import { durationMs, timestampMs } from "@bufbuild/protobuf/wkt";
import { type SyncPlayGroup, SyncPlayService, SyncPlayState } from "@mavio/proto/mavio/session/v1/syncplay_pb";
import { useQuery } from "@tanstack/react-query";

import { serverKey, useServersStore } from "./hooks.ts";
import type { ServerEntry, Servers } from "./servers.ts";

/** The groups on a server the user may join; refreshed as they open. */
export function useSyncPlayGroups(server: ServerEntry, enabled = true) {
  const servers = useServersStore();
  return useQuery({
    queryKey: serverKey(server.id, "syncplay-groups"),
    queryFn: async () => (await servers.client(SyncPlayService, server).listGroups({})).groups,
    enabled,
  });
}

/** A clock measurement: how far the server's clock is ahead of the client's, and the round trip, in milliseconds. */
export interface ClockSample {
  offset: number;
  rtt: number;
}

/**
 * The offset and round trip of an exchange sent at t0 and answered at t3 by
 * the client's clock, which the server received at t1 and answered at t2 by
 * its own.
 */
export function clockSample(t0: number, t1: number, t2: number, t3: number): ClockSample {
  return { offset: (t1 - t0 + (t2 - t3)) / 2, rtt: Math.max(0, t3 - t0 - (t2 - t1)) };
}

/** Measures the server's clock a few times, keeping the measurement of the shortest round trip. */
export async function syncClock(servers: Servers, server: ServerEntry, samples = 4, now = Date.now): Promise<ClockSample> {
  let best: ClockSample | undefined;
  for (let i = 0; i < samples; i++) {
    const t0 = now();
    const r = await servers.client(SyncPlayService, server).getTime({});
    const t3 = now();
    if (!r.receiveTime || !r.sendTime) continue;
    const s = clockSample(t0, timestampMs(r.receiveTime), timestampMs(r.sendTime), t3);
    if (!best || s.rtt < best.rtt) best = s;
  }
  if (!best) throw new Error("the server did not tell its time");
  return best;
}

/** Where a group is at a time of the server's clock, in milliseconds: advancing from its position while it plays. */
export function groupPositionAt(group: SyncPlayGroup, serverMs: number): number {
  const position = group.position ? durationMs(group.position) : 0;
  const at = group.positionTime ? timestampMs(group.positionTime) : serverMs;
  return group.state === SyncPlayState.PLAYING && serverMs > at ? position + (serverMs - at) : position;
}
