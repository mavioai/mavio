// Styled ASS and SSA subtitles (docs/clients.md §4.1, §6.5): JASSUB,
// libass in WebAssembly, draws them over the video in a canvas of its own
// with their authored styles, positions and karaoke, as libass does in the
// Apple and Android apps. The fonts are those attached to the item; what
// they lack libass draws with the default font, JASSUB's Liberation Sans,
// or Noto Sans SC for a script with Chinese or Japanese text. The player
// imports this module only as such a track plays, so that JASSUB, its
// worker and its WebAssembly, and the fonts, load then; Vite emits them
// with the build, which serves them itself, with no CDN.
import notoSansSC from "@openfonts/noto-sans-sc_chinese-simplified/files/noto-sans-sc-chinese-simplified-400.woff2?url";
import JASSUB from "jassub";
import liberationSans from "jassub/dist/default.woff2?url";

import type { Attachment } from "@mavio/proto/mavio/playback/v1/playback_pb";

/** Text of the Chinese and Japanese scripts, and their punctuation. */
const cjk = /[\p{Script=Han}\p{Script=Hiragana}\p{Script=Katakana}\u3000-\u303f\uff00-\uffef]/u;

// The family the regular weight of Noto Sans SC names itself in this
// file, as Google Fonts' instance of its variable font was named.
const noto = "noto sans sc thin";

/** The default font of a script: Noto Sans SC for one with Chinese or Japanese text, else Liberation Sans. */
export function defaultFont(script: string): string {
  return cjk.test(script) ? noto : "liberation sans";
}

/** Whether an attachment is a font, by its type or its file's extension. */
function isFont(a: Attachment): boolean {
  return /font|truetype|opentype/i.test(a.mimeType) || /\.(ttf|ttc|otf|otc|woff2?)$/i.test(a.fileName);
}

/** Styled subtitles drawn over a video. */
export interface StyledSubtitles {
  /** The canvas they are drawn in, beside the video, which moves with it. */
  canvas: HTMLCanvasElement;
  /** Shows them later by seconds, or earlier when negative. */
  setOffset(seconds: number): void;
  /** Stops drawing them and removes the canvas. */
  destroy(): void;
}

/**
 * Draws the ASS or SSA script at url over video, shown later by offset
 * seconds, with the fonts among the item's attachments, each with its
 * absolute URL. It resolves once the script and libass are loaded, and
 * rejects when JASSUB cannot run.
 */
export async function styledSubtitles(
  video: HTMLVideoElement,
  url: string,
  attachments: { attachment: Attachment; url: string }[],
  offset: number,
  signal: AbortSignal,
): Promise<StyledSubtitles> {
  const res = await fetch(url, { signal });
  if (!res.ok) throw new Error(`subtitles: ${res.status}`);
  const script = await res.text();
  const fonts = attachments.filter((a) => isFont(a.attachment)).map((a) => a.url);
  // Absolute, as the worker fetches them.
  const at = (path: string) => new URL(path, location.href).href;
  const canvas = document.createElement("canvas");
  canvas.className = "pointer-events-none absolute";
  video.after(canvas);
  let renderer: JASSUB;
  try {
    renderer = new JASSUB({
      video,
      canvas,
      subContent: script,
      fonts,
      // Loaded as libass first asks for them.
      availableFonts: { "liberation sans": at(liberationSans), [noto]: at(notoSansSC) },
      defaultFont: defaultFont(script),
      // No fonts of the computer's or Google's, which need permission or the internet.
      queryFonts: false,
      timeOffset: -offset,
    });
  } catch (err) {
    canvas.remove();
    throw err;
  }
  const destroy = () => void renderer.destroy().catch(() => undefined);
  try {
    await renderer.ready;
  } catch (err) {
    destroy();
    throw err;
  }
  if (signal.aborted) {
    destroy();
    throw signal.reason;
  }
  return {
    canvas,
    setOffset(seconds) {
      renderer.timeOffset = -seconds;
      // A paused video draws no frames: the subtitles are drawn again at once.
      void renderer.resize(true).catch(() => undefined);
    },
    destroy,
  };
}
