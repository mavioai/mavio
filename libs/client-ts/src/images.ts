// Artwork of items and libraries: plain HTTP from each item's server, the
// image IDs being the credentials (docs/clients.md §3).
import { type Image, ImageKind, type Item } from "@mavio/proto/mavio/library/v1/item_pb";

import { type Server, urlOf } from "./server.ts";

/** An image of an item, ready to draw. */
export interface Artwork {
  url: string;
  /** Width over height; 0 when unknown. */
  aspect: number;
  blurhash?: string;
}

/** The URL of an image scaled to at most maxWidth CSS pixels on this display. */
export function imageUrl(server: Server, id: string, maxWidth?: number): string {
  const url = urlOf(server, `images/${id}`);
  if (!maxWidth) return url;
  const dpr = typeof devicePixelRatio === "number" ? devicePixelRatio : 1;
  return `${url}?maxWidth=${Math.round(maxWidth * Math.min(dpr, 3))}`;
}

/** An item's first image of a kind. */
export function imageOf(item: { images: Image[] }, kind: ImageKind): Image | undefined {
  let best: Image | undefined;
  for (const i of item.images) {
    if (i.kind === kind && i.id && (!best || i.index < best.index)) best = i;
  }
  return best;
}

/** An item's artwork of a kind, scaled to maxWidth. */
export function artwork(server: Server, item: { images: Image[] }, kind: ImageKind, maxWidth?: number) {
  const image = imageOf(item, kind);
  if (!image) return undefined;
  return {
    url: imageUrl(server, image.id, maxWidth),
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

/** A library's collage of its items. */
export function libraryImageUrl(server: Server, libraryId: string, maxWidth?: number): string {
  const url = urlOf(server, `images/collages/${libraryId}`);
  return maxWidth ? `${url}?maxWidth=${maxWidth}` : url;
}
