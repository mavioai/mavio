// The settings' Transcoding (docs/clients.md §6.3.9): how the server
// re-encodes what devices cannot play — the hardware acceleration its
// ffmpeg supports, the encoder's preset and quality, tone mapping of HDR,
// deinterlacing, the downmix of audio, cropping black borders, and the
// folder of running transcodes.
import { create } from "@bufbuild/protobuf";
import { errorMessage, type ServerEntry, useServerSettings, useUpdateServerSettings } from "@mavio/client-ts";
import {
  HardwareAcceleration,
  type ServerSettings,
  type TranscodingSettings,
  TranscodingSettingsSchema,
} from "@mavio/proto/mavio/system/v1/system_pb";
import { useState } from "react";

import { t } from "../../i18n.ts";
import { Button, Group, Row } from "../../ui.tsx";
import { NumberField, Switch } from "../../controls/inputs.tsx";
import { Select } from "../../controls/select.tsx";
import { Page, patched } from "./common.tsx";
import { input } from "../../controls/styles.ts";

const accelerations: Partial<Record<HardwareAcceleration, () => string>> = {
  [HardwareAcceleration.AUTO]: () => t("transcoding.accelerationAuto"),
  [HardwareAcceleration.NONE]: () => t("transcoding.accelerationNone"),
  [HardwareAcceleration.VIDEOTOOLBOX]: () => "VideoToolbox",
};

const presets = ["ultrafast", "superfast", "veryfast", "faster", "fast", "medium", "slow", "slower", "veryslow"];
const tonemaps = ["bt2390", "hable", "mobius", "reinhard", "clip", "linear", "gamma"];
const ranges = ["auto", "tv", "pc"];
const deinterlacers = ["yadif", "bwdif"];

/** The section of the server chosen in the settings. */
export function TranscodingSection({ server }: { server: ServerEntry }) {
  const settings = useServerSettings(server);
  const d = settings.data;
  return (
    <Page title={t("settings.transcoding")} message={t("transcoding.message")}>
      {settings.isError && <p className="type-callout text-danger">{errorMessage(settings.error)}</p>}
      {d?.settings && <TranscodingForm server={server} settings={d.settings} available={d.availableHardwareAccelerations} />}
    </Page>
  );
}

const draftOf = (s: ServerSettings): TranscodingSettings => create(TranscodingSettingsSchema, s.transcoding);

function TranscodingForm({ server, settings, available }: { server: ServerEntry; settings: ServerSettings; available: HardwareAcceleration[] }) {
  const update = useUpdateServerSettings(server);
  const [draft, setDraft] = useState(() => draftOf(settings));
  const [notice, setNotice] = useState<string>();
  const set = (patch: Partial<TranscodingSettings>) => (setDraft((d) => patched(TranscodingSettingsSchema, d, patch)), setNotice(undefined));
  const changed = JSON.stringify(draft) !== JSON.stringify(draftOf(settings));
  const save = () =>
    update.mutate(
      (s) => {
        s.transcoding = patched(TranscodingSettingsSchema, draft, { transcodeDir: draft.transcodeDir.trim() });
      },
      { onSuccess: () => setNotice(t("transcoding.saved")) },
    );
  const choices = (list: string[]) => list.map((v) => ({ value: v, label: v }));
  const number = (key: "h264Crf" | "h265Crf" | "threads" | "tonemapDesat" | "tonemapPeak" | "downmixBoost", label: string, detail: string | undefined, min: number, max: number, step = 1) => (
    <Row label={label} detail={detail}>
      <NumberField
        min={min}
        max={max}
        step={step}
        integer={step === 1}
        className="w-32"
        label={label}
        value={draft[key]}
        onChange={(v) => v !== undefined && set({ [key]: v })}
      />
    </Row>
  );
  const toggle = (key: "hardwareEncoding" | "deinterlaceDoubleRate" | "cropBlackBorders", label: string, detail?: string) => (
    <Row label={label} detail={detail}>
      <Switch on={draft[key]} label={label} onChange={(on) => set({ [key]: on })} />
    </Row>
  );
  const offered = available.length ? available : [HardwareAcceleration.AUTO, HardwareAcceleration.NONE];
  return (
    <>
      <Group title={t("transcoding.hardware")}>
        <Row label={t("transcoding.acceleration")} detail={t("transcoding.accelerationDetail")}>
          <Select
            value={String(draft.hardwareAcceleration)}
            label={t("transcoding.acceleration")}
            options={[...new Set([...offered, draft.hardwareAcceleration])]
              .filter((a) => a !== HardwareAcceleration.UNSPECIFIED)
              .map((a) => ({ value: String(a), label: accelerations[a]?.() ?? String(a) }))}
            onChange={(v) => set({ hardwareAcceleration: Number(v) as HardwareAcceleration })}
          />
        </Row>
        {draft.hardwareAcceleration !== HardwareAcceleration.NONE &&
          toggle("hardwareEncoding", t("transcoding.hardwareEncoding"), t("transcoding.hardwareEncodingDetail"))}
      </Group>
      <Group title={t("transcoding.encoding")}>
        <Row label={t("transcoding.preset")} detail={t("transcoding.presetDetail")}>
          <Select
            value={draft.encoderPreset}
            label={t("transcoding.preset")}
            options={[{ value: "", label: t("transcoding.presetAuto") }, ...choices(presets)]}
            onChange={(encoderPreset) => set({ encoderPreset })}
          />
        </Row>
        {number("h264Crf", t("transcoding.avcCrf"), t("transcoding.crfDetail"), 0, 51)}
        {number("h265Crf", t("transcoding.hevcCrf"), undefined, 0, 51)}
        {number("threads", t("transcoding.threads"), t("transcoding.threadsDetail"), 0, 256)}
      </Group>
      <Group title={t("transcoding.video")}>
        <Row label={t("transcoding.tonemap")} detail={t("transcoding.tonemapDetail")}>
          <Select value={draft.tonemapAlgorithm} label={t("transcoding.tonemap")} options={choices(tonemaps)} onChange={(tonemapAlgorithm) => set({ tonemapAlgorithm })} />
        </Row>
        <Row label={t("transcoding.tonemapRange")}>
          <Select value={draft.tonemapRange} label={t("transcoding.tonemapRange")} options={choices(ranges)} onChange={(tonemapRange) => set({ tonemapRange })} />
        </Row>
        {number("tonemapDesat", t("transcoding.tonemapDesat"), undefined, 0, 10, 0.1)}
        {number("tonemapPeak", t("transcoding.tonemapPeak"), t("transcoding.tonemapPeakDetail"), 0, 10000)}
        <Row label={t("transcoding.deinterlace")}>
          <Select value={draft.deinterlaceMethod} label={t("transcoding.deinterlace")} options={choices(deinterlacers)} onChange={(deinterlaceMethod) => set({ deinterlaceMethod })} />
        </Row>
        {toggle("deinterlaceDoubleRate", t("transcoding.doubleRate"), t("transcoding.doubleRateDetail"))}
        {toggle("cropBlackBorders", t("transcoding.crop"), t("transcoding.cropDetail"))}
      </Group>
      <Group title={t("transcoding.audio")}>{number("downmixBoost", t("transcoding.downmixBoost"), t("transcoding.downmixBoostDetail"), 0.5, 3, 0.1)}</Group>
      <Group title={t("transcoding.folder")} footer={t("transcoding.folderFooter")}>
        <Row label={t("transcoding.folder")}>
          <input
            className={`${input} sm:w-72`}
            aria-label={t("transcoding.folder")}
            placeholder={t("transcoding.folderDefault")}
            value={draft.transcodeDir}
            onChange={(e) => set({ transcodeDir: e.target.value })}
          />
        </Row>
      </Group>
      <div className="flex flex-wrap items-center gap-3">
        <Button disabled={!changed || update.isPending} onClick={save}>
          {update.isPending ? "…" : t("action.save")}
        </Button>
        {notice && !changed && <span className="type-callout text-text-secondary">{notice}</span>}
        {update.isError && <span className="type-callout text-danger">{errorMessage(update.error)}</span>}
      </div>
    </>
  );
}
