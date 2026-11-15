// The lyrics of a track (docs/clients.md §6.3.27): read from the lyric file
// beside it, with the line and the words sung at a time when synced; and,
// for administrators, the lyrics providers' searched, previewed and
// downloaded beside it.
import { durationMs } from "@bufbuild/protobuf/wkt";
import { Code, ConnectError } from "@connectrpc/connect";
import { ItemService, type LyricLine } from "@mavio/proto/mavio/library/v1/item_pb";
import { MetadataService } from "@mavio/proto/mavio/library/v1/metadata_pb";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";

import { serverKey, useServersStore } from "./hooks.ts";
import type { ServerEntry } from "./servers.ts";

/** A track's lyrics, null when it has none; not fetched when disabled. */
export function useLyrics(server: ServerEntry, itemId: string, enabled = true) {
  const servers = useServersStore();
  return useQuery({
    queryKey: serverKey(server.id, "lyrics", itemId),
    queryFn: async () => {
      try {
        return await servers.client(ItemService, server).getLyrics({ itemId });
      } catch (e) {
        if (e instanceof ConnectError && e.code === Code.NotFound) return null;
        throw e;
      }
    },
    enabled,
    staleTime: 5 * 60_000,
  });
}

/** The lyrics the providers have of a track, each provider's best first; not searched when disabled. */
export function useRemoteLyrics(server: ServerEntry, itemId: string, enabled = true) {
  const servers = useServersStore();
  return useQuery({
    queryKey: serverKey(server.id, "remote-lyrics", itemId),
    queryFn: async () => (await servers.client(MetadataService, server).searchRemoteLyrics({ itemId })).lyrics,
    enabled,
    staleTime: 5 * 60_000,
    retry: false,
  });
}

/** Lyrics a provider has of a track, as they would be saved; not fetched when disabled. */
export function useRemoteLyricsPreview(server: ServerEntry, itemId: string, provider: string, id: string, enabled = true) {
  const servers = useServersStore();
  return useQuery({
    queryKey: serverKey(server.id, "remote-lyrics", itemId, provider, id),
    queryFn: async () => (await servers.client(MetadataService, server).getRemoteLyrics({ itemId, provider, id })).lyrics ?? null,
    enabled,
    staleTime: Infinity,
    retry: false,
  });
}

/** Saves a provider's lyrics beside a track, refreshing the lyrics shown. */
export function useDownloadLyrics(server: ServerEntry, itemId: string) {
  const servers = useServersStore();
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (v: { provider: string; id: string }) => servers.client(MetadataService, server).downloadRemoteLyrics({ itemId, ...v }),
    onSuccess: () => qc.invalidateQueries({ queryKey: serverKey(server.id, "lyrics", itemId) }),
  });
}

/** The index of the line sung at a time in milliseconds, -1 before the first. */
export function lyricLineAt(lines: LyricLine[], ms: number): number {
  let at = -1;
  for (const [i, l] of lines.entries()) {
    if (!l.start) continue;
    if (durationMs(l.start) > ms) break;
    at = i;
  }
  return at;
}

/** The part of a line sung by a time in milliseconds, in UTF-16 code units, by its words' times; all of it without them. */
export function lyricSungLength(line: LyricLine, ms: number): number {
  if (line.cues.length === 0) return line.text.length;
  let sung = 0;
  for (const c of line.cues) {
    if (!c.start || durationMs(c.start) > ms) break;
    sung = c.endPosition;
  }
  return sung;
}
