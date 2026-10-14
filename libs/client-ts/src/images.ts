// Artwork of items and libraries: plain HTTP from each item's server, at
// the URLs the server signed for the session (docs/accounts.md §5.4).
import { type Image, ImageKind, type Item } from "@mavio/proto/mavio/library/v1/item_pb";

import { type Server, urlOf } from "./server.ts";

/** An image of an item, ready to draw. */
export interface Artwork {
  url: string;
  /** Width over height; 0 when unknown. */
  aspect: number;
  blurhash?: string;
}

/** The URL of an image, its signed path, scaled to at most maxWidth CSS pixels on this display. */
export function imageUrl(server: Server, path: string, maxWidth?: number): string {
  const url = urlOf(server, path);
  if (!maxWidth) return url;
  const dpr = typeof devicePixelRatio === "number" ? devicePixelRatio : 1;
  return `${url}?maxWidth=${Math.round(maxWidth * Math.min(dpr, 3))}`;
}

/** An item's first image of a kind that has a URL. */
export function imageOf(item: { images: Image[] }, kind: ImageKind): Image | undefined {
  let best: Image | undefined;
  for (const i of item.images) {
    if (i.kind === kind && i.url && (!best || i.index < best.index)) best = i;
  }
  return best;
}

/** An item's artwork of a kind, scaled to maxWidth. */
export function artwork(server: Server, item: { images: Image[] }, kind: ImageKind, maxWidth?: number) {
  const image = imageOf(item, kind);
  if (!image) return undefined;
  return {
    url: imageUrl(server, image.url, maxWidth),
    aspect: image.height > 0 ? image.width / image.height : 0,
    blurhash: image.blurhash || undefined,
  } satisfies Artwork;
}

/** The landscape artwork of an item: its thumbnail, else its primary image or backdrop. */
export function thumbOf(server: Server, item: Item, maxWidth?: number): Artwork | undefined {
  return (
    artwork(server, item, ImageKind.THUMB, maxWidth) ??
    artwork(server, item, ImageKind.PRIMARY, maxWidth) ??
    artwork(server, item, ImageKind.BACKDROP, maxWidth)
  );
}

/** A library's collage of its items; undefined when the server signed none. */
export function libraryImageUrl(server: Server, library: { imageUrl: string }, maxWidth?: number): string | undefined {
  if (!library.imageUrl) return undefined;
  const url = urlOf(server, library.imageUrl);
  return maxWidth ? `${url}?maxWidth=${maxWidth}` : url;
}
