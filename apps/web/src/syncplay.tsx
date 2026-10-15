// SyncPlay in the web UI (docs/clients.md §6.3.24): the group this browser
// is in, kept from its servers' events, with the clock kept in step with
// the group's server; a group asking its members to play brings the player
// to its entry, where useGroupPlayer has the video follow the group.
import { durationFromMs, durationMs, timestampFromMs, timestampMs } from "@bufbuild/protobuf/wkt";
import { groupPositionAt, type ServerEntry, syncClock, useServersStore } from "@mavio/client-ts";
import {
  type SyncPlayCommand,
  SyncPlayCommandKind,
  type SyncPlayGroup,
  SyncPlayService,
  SyncPlayState,
} from "@mavio/proto/mavio/session/v1/syncplay_pb";
import { useRouter } from "@tanstack/react-router";
import { createContext, type ReactNode, type RefObject, useCallback, useContext, useEffect, useMemo, useRef, useState } from "react";

import { useServerEvents } from "./events.tsx";

/** The group this browser is in, on its server. */
export interface Joined {
  server: ServerEntry;
  group: SyncPlayGroup;
}

interface SyncPlay {
  joined?: Joined;
  /** Takes the group a request to join or create answered with. */
  enter(server: ServerEntry, group: SyncPlayGroup): void;
  /** Leaves the group. */
  leave(): Promise<void>;
  /** The time by the group's server's clock, in milliseconds. */
  serverNow(): number;
}

const Context = createContext<SyncPlay | undefined>(undefined);

/** The SyncPlay of the browser; undefined outside SyncPlayProvider. */
export function useSyncPlay(): SyncPlay | undefined {
  return useContext(Context);
}

/** Whether a player shows an item of a server. */
const playerOf = (pathname: string, server: ServerEntry, itemId: string) => pathname === `/s/${server.id}/play/${itemId}`;

/** Keeps the group of the browser, its clock, and brings the player to the group's entry when the group waits for it. */
export function SyncPlayProvider({ children }: { children: ReactNode }) {
  const servers = useServersStore();
  const router = useRouter();
  const [joined, setJoined] = useState<Joined>();
  const offset = useRef(0);

  useServerEvents((server, e) => {
    if (e.kind.case !== "syncPlay") return;
    const group = e.kind.value.group;
    if (!group) {
      setJoined((j) => (j?.server.id === server.id ? undefined : j));
      return;
    }
    setJoined({ server, group });
    const entry = group.queue[group.currentIndex];
    const me = group.members.find((m) => m.sessionId === server.sessionId);
    if (group.state === SyncPlayState.IDLE || !entry || !me?.buffering) return;
    const { pathname } = router.state.location;
    if (playerOf(pathname, server, entry.itemId)) return;
    const start = Math.round(group.position ? durationMs(group.position) : 0);
    void router.navigate({
      to: "/s/$serverId/play/$itemId",
      params: { serverId: server.id, itemId: entry.itemId },
      search: { start: start || undefined },
      // A player moves on to the group's next entry in place.
      replace: pathname.includes("/play/"),
    });
  });

  // The clock, measured as the group starts and every half minute, and the round trip told to the group.
  const server = joined?.server;
  useEffect(() => {
    if (!server) return;
    let stopped = false;
    const measure = async () => {
      try {
        const s = await syncClock(servers, server);
        if (stopped) return;
        offset.current = s.offset;
        await servers.client(SyncPlayService, server).reportPing({ ping: durationFromMs(Math.min(60_000, Math.round(s.rtt))) });
      } catch {
        // Measured again at the next turn.
      }
    };
    void measure();
    const timer = setInterval(measure, 30_000);
    return () => {
      stopped = true;
      clearInterval(timer);
    };
  }, [servers, server]);

  const value = useMemo<SyncPlay>(
    () => ({
      joined,
      enter: (server, group) => setJoined({ server, group }),
      leave: async () => {
        if (!joined) return;
        await servers.client(SyncPlayService, joined.server).leaveGroup({});
        setJoined(undefined);
      },
      serverNow: () => Date.now() + offset.current,
    }),
    [servers, joined],
  );
  return <Context.Provider value={value}>{children}</Context.Provider>;
}

/** What the player does instead of acting on its own while it follows a group. */
export interface GroupPlayer {
  group: SyncPlayGroup;
  unpause(): void;
  pause(): void;
  /** Seeks the group to a position in milliseconds. */
  seek(ms: number): void;
  /** Plays the group's next entry, else items after it. */
  next(itemIds: string[]): void;
  /** The video ended: the group's first member moves it on. */
  ended(): void;
  /** The player restarted its stream, as when the user picks other tracks. */
  restarted(): void;
}

const HAVE_FUTURE_DATA = 3;

/**
 * Has a player's video follow the group while the group plays its item:
 * it waits paused at the group's position until the group plays, reports
 * when it is ready or buffering, carries out the group's commands at their
 * time, and keeps up with the group's position. Undefined while the player
 * plays on its own.
 */
export function useGroupPlayer(
  server: ServerEntry,
  itemId: string,
  video: RefObject<HTMLVideoElement | null>,
  onStop: () => void,
): GroupPlayer | undefined {
  const servers = useServersStore();
  const sp = useSyncPlay();
  const group = sp?.joined?.server.id === server.id ? sp.joined.group : undefined;
  const entry = group?.queue[group.currentIndex];
  const active = !!group && group.state !== SyncPlayState.IDLE && entry?.itemId === itemId;
  const me = group?.members.find((m) => m.sessionId === server.sessionId);

  const latest = useRef({ group, entry, active, sp });
  useEffect(() => {
    latest.current = { group, entry, active, sp };
  });
  const rpc = useCallback(() => servers.client(SyncPlayService, server), [servers, server]);

  // Tells the group where the video is, and whether it can play on.
  const report = useCallback(
    (ready: boolean) => {
      const { entry, sp } = latest.current;
      const v = video.current;
      if (!v || !entry || !sp) return;
      void rpc()
        .reportState({
          entryId: entry.id,
          position: durationFromMs(Math.round(v.currentTime * 1000)),
          when: timestampFromMs(sp.serverNow()),
          playing: !v.paused,
          ready,
        })
        .catch(() => {});
    },
    [rpc, video],
  );

  // Pauses at a position and reports ready once the video can play there.
  const readyAt = useCallback(
    (ms: number) => {
      const v = video.current;
      if (!v) return;
      v.pause();
      v.playbackRate = 1;
      if (v.readyState > 0 && Math.abs(v.currentTime - ms / 1000) > 0.25) v.currentTime = ms / 1000;
      const check = () => {
        if (v.readyState < HAVE_FUTURE_DATA || v.seeking) return;
        v.removeEventListener("canplay", check);
        v.removeEventListener("seeked", check);
        report(true);
      };
      v.addEventListener("canplay", check);
      v.addEventListener("seeked", check);
      check();
    },
    [report, video],
  );

  // A member the group waits for gets ready at the group's position.
  const waiting = active && group.state === SyncPlayState.WAITING && !!me?.buffering;
  const positionMs = group?.position ? durationMs(group.position) : 0;
  useEffect(() => {
    if (waiting) readyAt(positionMs);
  }, [waiting, positionMs, readyAt]);

  // The group's commands, at their time.
  const timer = useRef<ReturnType<typeof setTimeout>>(undefined);
  useEffect(() => () => clearTimeout(timer.current), []);
  useServerEvents((s, e) => {
    if (s.id !== server.id || e.kind.case !== "syncPlay") return;
    const g = e.kind.value.group;
    const c: SyncPlayCommand | undefined = e.kind.value.command;
    const v = video.current;
    const sync = latest.current.sp;
    if (!c || !v || !sync || !g || g.queue[g.currentIndex]?.itemId !== itemId) return;
    clearTimeout(timer.current);
    const at = c.position ? durationMs(c.position) : 0;
    switch (c.kind) {
      case SyncPlayCommandKind.PAUSE:
        v.pause();
        if (Math.abs(v.currentTime - at / 1000) > 0.5) v.currentTime = at / 1000;
        break;
      case SyncPlayCommandKind.SEEK:
        readyAt(at);
        break;
      case SyncPlayCommandKind.STOP:
        onStop();
        break;
      case SyncPlayCommandKind.UNPAUSE: {
        // when is in the server's time; the video starts then, or catches up when it passed.
        const delay = (c.when ? timestampMs(c.when) : sync.serverNow()) - sync.serverNow();
        v.playbackRate = 1;
        if (delay > 0) {
          v.currentTime = at / 1000;
          timer.current = setTimeout(() => void v.play().catch(() => {}), delay);
        } else {
          v.currentTime = (at - delay) / 1000;
          void v.play().catch(() => {});
        }
      }
    }
  });

  // Buffering while the group plays makes the group wait.
  const playing = active && group.state === SyncPlayState.PLAYING;
  useEffect(() => {
    const v = video.current;
    if (!playing || !v) return;
    const stall = () => report(false);
    v.addEventListener("waiting", stall);
    return () => v.removeEventListener("waiting", stall);
  }, [playing, report, video]);

  // Keeps up with the group: far behind or ahead it skips, near it speeds up or slows down a little.
  useEffect(() => {
    const v = video.current;
    if (!playing || !v) return;
    const tick = setInterval(() => {
      const { group, sp } = latest.current;
      if (!group || !sp || v.paused || v.seeking) return;
      const off = v.currentTime - groupPositionAt(group, sp.serverNow()) / 1000;
      if (Math.abs(off) > 2) {
        v.playbackRate = 1;
        v.currentTime -= off;
      } else v.playbackRate = Math.abs(off) < 0.05 ? 1 : 1 - Math.max(-0.5, Math.min(0.5, off)) / 5;
    }, 1000);
    return () => {
      clearInterval(tick);
      v.playbackRate = 1;
    };
  }, [playing, video]);

  return useMemo<GroupPlayer | undefined>(() => {
    if (!active || !group) return undefined;
    const fail = () => {};
    return {
      group,
      unpause: () => void rpc().unpause({}).catch(fail),
      pause: () => void rpc().pause({}).catch(fail),
      seek: (ms) => void rpc().seek({ position: durationFromMs(Math.max(0, Math.round(ms))) }).catch(fail),
      next: (itemIds) =>
        void (group.currentIndex < group.queue.length - 1 ? rpc().nextItem({}) : rpc().setQueue({ itemIds })).catch(fail),
      ended: () => {
        if (group.members[0]?.sessionId !== server.sessionId) return;
        void (group.currentIndex < group.queue.length - 1 ? rpc().nextItem({}) : rpc().stop({})).catch(fail);
      },
      restarted: () => {
        if (latest.current.group?.state === SyncPlayState.PLAYING) report(false);
      },
    };
  }, [active, group, rpc, server.sessionId, report]);
}
