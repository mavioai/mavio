import { MediaKind, Protocol } from "@mavio/proto/mavio/playback/v1/playback_pb";
import { expect, test } from "vitest";

import { capabilities, type Decoders } from "./playback.ts";

function decoders(mse: string[], native: string[] = []): Decoders {
  return {
    mse: (t) => mse.some((c) => t.includes(c)),
    native: (t) => native.some((c) => t.includes(c)),
  };
}

test("declares what Media Source Extensions decode", () => {
  const caps = capabilities(decoders(["avc1", "av01", "mp4a.40.2", "opus"], ["webm"]), 8_000_000);
  expect(caps.maxStreamingBitrate).toBe(8_000_000n);
  const mp4 = caps.directPlay.find((p) => p.container.startsWith("mp4"))!;
  expect([mp4.videoCodec, mp4.audioCodec]).toEqual(["h264,av1", "aac,opus"]);
  expect(caps.directPlay.some((p) => p.container === "webm")).toBe(true);
  const hls = caps.transcoding.find((p) => p.kind === MediaKind.VIDEO)!;
  expect([hls.protocol, hls.container, hls.videoCodec]).toEqual([Protocol.HLS, "mp4", "h264,av1"]);
});

test("native HLS declares what the media element plays", () => {
  const caps = capabilities(decoders([], ["mpegurl", "avc1", "hvc1", "mp4a.40.2", "ec-3"]));
  expect(caps.name).toContain("native HLS");
  const hls = caps.transcoding.find((p) => p.kind === MediaKind.VIDEO)!;
  expect([hls.videoCodec, hls.audioCodec]).toEqual(["h264,hevc", "aac,eac3"]);
  expect(caps.directPlay.some((p) => p.container === "webm")).toBe(false);
});

test("H.264 and AAC are always declared", () => {
  const mp4 = capabilities(decoders([])).directPlay[0];
  expect([mp4.videoCodec, mp4.audioCodec]).toEqual(["h264", "aac"]);
});
