// A track's lyrics in the player (docs/clients.md §6.3.27): synced lyrics
// follow the song, the line sung kept in the middle and, with its words'
// times, filled word by word; a line seeks to it. Other lyrics scroll as
// text.
import { durationMs } from "@bufbuild/protobuf/wkt";
import { lyricLineAt, lyricSungLength } from "@mavio/client-ts";
import type { GetLyricsResponse } from "@mavio/proto/mavio/library/v1/item_pb";
import { useEffect, useRef } from "react";

import { t } from "../i18n.ts";

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
