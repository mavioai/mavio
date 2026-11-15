// A track's lyrics in the player (docs/clients.md §6.3.27): synced lyrics
// follow the song, the line sung kept in the middle and, with its words'
// times, filled word by word; a line seeks to it. Other lyrics scroll as
// text. Administrators search the lyrics providers, preview what they found
// and download it beside the track.
import { durationMs } from "@bufbuild/protobuf/wkt";
import {
  errorMessage,
  lyricLineAt,
  lyricSungLength,
  type ServerEntry,
  useDownloadLyrics,
  useRemoteLyrics,
  useRemoteLyricsPreview,
} from "@mavio/client-ts";
import type { GetLyricsResponse, Item } from "@mavio/proto/mavio/library/v1/item_pb";
import type { RemoteLyrics } from "@mavio/proto/mavio/library/v1/metadata_pb";
import { type RefObject, useEffect, useRef, useState } from "react";

import { Sheet } from "../controls/dialog.tsx";
import { t } from "../i18n.ts";
import { Button } from "../ui.tsx";
import { clock } from "./seekbar.tsx";

const fade = "[mask-image:linear-gradient(to_bottom,transparent,black_12%,black_88%,transparent)]";

/** The lyrics over the player at a time in seconds; clicking elsewhere does what clicking the video does. */
export function LyricsView({
  lyrics,
  time,
  onSeek,
  onBackground,
}: {
  lyrics: GetLyricsResponse;
  time: number;
  onSeek: (s: number) => void;
  onBackground: () => void;
}) {
  const ms = time * 1000;
  const at = lyrics.synced ? lyricLineAt(lyrics.lines, ms) : -1;
  const list = useRef<HTMLOListElement>(null);
  useEffect(() => {
    if (at < 0) return;
    list.current?.children[at]?.scrollIntoView({ block: "center", behavior: "smooth" });
  }, [at]);

  if (!lyrics.synced)
    return (
      <div className={`absolute inset-x-0 top-20 bottom-32 overflow-y-auto px-6 ${fade}`} onClick={onBackground}>
        <div aria-label={t("player.lyrics")} className="mx-auto max-w-2xl py-12 type-title3 leading-relaxed whitespace-pre-wrap text-white/85">
          {lyrics.lines.map((l) => l.text).join("\n")}
        </div>
      </div>
    );
  return (
    <div className={`absolute inset-x-0 top-20 bottom-32 overflow-y-auto px-6 ${fade}`} onClick={onBackground}>
      <ol ref={list} aria-label={t("player.lyrics")} className="mx-auto flex max-w-2xl flex-col gap-5 py-[30vh]">
        {lyrics.lines.map((l, i) => {
          const sung = i === at ? lyricSungLength(l, ms) : 0;
          return (
            <li key={i}>
              <button
                type="button"
                aria-current={i === at || undefined}
                disabled={!l.start}
                onClick={(e) => {
                  e.stopPropagation();
                  if (l.start) onSeek(durationMs(l.start) / 1000);
                }}
                className={`text-left type-title1 font-bold transition-colors duration-300 ${i === at ? "" : "text-white/35 hover:text-white/60"}`}
              >
                {i === at ? (
                  <>
                    <span className="text-white">{l.text.slice(0, sung)}</span>
                    <span className="text-white/50">{l.text.slice(sung)}</span>
                  </>
                ) : (
                  l.text || "♪"
                )}
              </button>
            </li>
          );
        })}
      </ol>
    </div>
  );
}

/**
 * The lyrics the providers have of a track, each with its name, artists,
 * album and length, whether it is synced, its provider and match, its text
 * previewed, and downloaded in place, replacing the track's lyric files.
 */
export function LyricsSearch({
  server,
  item,
  container,
  onClose,
  onDownloaded,
}: {
  server: ServerEntry;
  item: Item;
  container?: RefObject<HTMLElement | null>;
  onClose: () => void;
  onDownloaded: () => void;
}) {
  const search = useRemoteLyrics(server, item.id);
  const list = search.data ?? [];
  const title = t("lyrics.searchTitle", { name: item.name });
  return (
    <Sheet label={title} width="wide" container={container} onClose={onClose}>
      <div className="flex max-h-[85vh] min-h-0 flex-col gap-3 p-6">
        <div className="flex flex-wrap items-center gap-3">
          <div className="min-w-0 flex-1">
            <h2 className="type-title2">{title}</h2>
            <p className="type-footnote text-text-secondary">{t("lyrics.searchFooter")}</p>
          </div>
          <Button variant="plain" onClick={onClose}>
            {t("keys.done")}
          </Button>
        </div>
        <div className="-mx-6 min-h-0 flex-1 divide-y divide-border overflow-y-auto border-t border-border">
          {search.isFetching && list.length === 0 && <p className="px-6 py-4 type-callout text-text-secondary">{t("lyrics.searching")}</p>}
          {search.isSuccess && !search.isFetching && list.length === 0 && <p className="px-6 py-4 type-callout text-text-secondary">{t("lyrics.noResults")}</p>}
          {search.isError && <p className="px-6 py-4 type-callout break-words text-danger">{errorMessage(search.error)}</p>}
          {list.map((l) => (
            <LyricsResult key={`${l.provider}/${l.id}`} server={server} itemId={item.id} lyrics={l} onDownloaded={onDownloaded} />
          ))}
        </div>
      </div>
    </Sheet>
  );
}

function LyricsResult({ server, itemId, lyrics: l, onDownloaded }: { server: ServerEntry; itemId: string; lyrics: RemoteLyrics; onDownloaded: () => void }) {
  const [previewing, setPreviewing] = useState(false);
  const preview = useRemoteLyricsPreview(server, itemId, l.provider, l.id, previewing);
  const download = useDownloadLyrics(server, itemId);
  const about = [l.artists.join(", "), l.album, l.duration && clock(Number(l.duration.seconds))].filter(Boolean);
  const detail = [l.synced ? t("lyrics.synced") : t("lyrics.plain"), l.provider, l.score > 0 && t("lyrics.score", { percent: Math.round(l.score * 100) })].filter(Boolean);
  return (
    <div className="flex flex-col gap-2 px-6 py-3">
      <div className="flex flex-wrap items-center gap-x-4 gap-y-1">
        <div className="flex min-w-0 flex-1 basis-64 flex-col gap-0.5">
          <p className="type-callout break-words">{l.name || l.id}</p>
          {about.length > 0 && <p className="type-footnote break-words text-text-secondary">{about.join(" · ")}</p>}
          <p className="type-footnote text-text-secondary">{detail.join(" · ")}</p>
          {download.isError && <p className="type-footnote break-words text-danger">{errorMessage(download.error)}</p>}
        </div>
        <div className="flex gap-1">
          <Button variant="plain" onClick={() => setPreviewing(!previewing)}>
            {previewing ? t("lyrics.hidePreview") : t("lyrics.preview")}
          </Button>
          <Button
            disabled={download.isPending || download.isSuccess}
            onClick={() => download.mutate({ provider: l.provider, id: l.id }, { onSuccess: onDownloaded })}
          >
            {download.isSuccess ? t("lyrics.downloaded") : download.isPending ? "…" : t("lyrics.download")}
          </Button>
        </div>
      </div>
      {previewing && (
        <div className="max-h-64 overflow-y-auto rounded-[10px] bg-fill p-3">
          {preview.isPending && <p className="type-footnote text-text-secondary">…</p>}
          {preview.isError && <p className="type-footnote break-words text-danger">{errorMessage(preview.error)}</p>}
          {preview.data && (
            <ol className="flex flex-col gap-0.5 type-callout">
              {preview.data.lines.map((line, i) => (
                <li key={i} className="flex gap-3">
                  {preview.data!.synced && (
                    <span className="w-12 shrink-0 text-text-secondary tabular-nums">{line.start ? clock(durationMs(line.start) / 1000) : ""}</span>
                  )}
                  <span className="min-w-0 break-words">{line.text || "♪"}</span>
                </li>
              ))}
            </ol>
          )}
        </div>
      )}
    </div>
  );
}
