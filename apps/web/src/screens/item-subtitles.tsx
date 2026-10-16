// A movie's or episode's subtitles on its details, for administrators
// (docs/clients.md §6.3.21): its subtitle profile and the languages it
// lacks; the subtitles it has, each synchronized, modified, translated,
// extracted or deleted; the providers' subtitles searched and downloaded;
// and a subtitle file uploaded.
import type { MessageInitShape } from "@bufbuild/protobuf";
import {
  errorMessage,
  type ServerEntry,
  useAdminServers,
  useAiModels,
  useItemSubtitleActions,
  useItemSubtitles,
  useSubtitleSearch,
  useSubtitleTranslators,
} from "@mavio/client-ts";
import type { ItemSubtitle, ScoredSubtitle, SubtitleLanguageSpec, SubtitleLanguageSpecSchema } from "@mavio/proto/mavio/acquisition/v1/subtitle_pb";
import { type Item, ItemKind } from "@mavio/proto/mavio/library/v1/item_pb";
import { type ReactNode, useState } from "react";

import { discoverLanguage, locale, t } from "../i18n.ts";
import { Button, Group } from "../ui.tsx";
import { FileField, Switch } from "../controls/inputs.tsx";
import { Select } from "../controls/select.tsx";
import { Confirm, Sheet } from "../controls/dialog.tsx";
import { mods } from "./admin/subtitle-settings.tsx";
import { useSubtitleLanguages } from "./admin/subtitles.tsx";

type Spec = MessageInitShape<typeof SubtitleLanguageSpecSchema>;

const languageNames = new Intl.DisplayNames([locale], { type: "language", fallback: "code" });

/** A subtitle language with its kind, as "English (Forced)". */
function specLabel(s?: Spec | SubtitleLanguageSpec): string {
  if (!s?.language) return t("itemsubs.unknown");
  const name = languageNames.of(s.language) ?? s.language;
  if (s.forced) return t("itemsubs.forcedLanguage", { language: name });
  if (s.hearingImpaired) return t("itemsubs.hiLanguage", { language: name });
  return name;
}

const fileName = (path: string) => path.slice(path.lastIndexOf("/") + 1);

/** The item's subtitles, when the user administers its server and it is a movie or an episode. */
export function ItemSubtitles({ server, item }: { server: ServerEntry; item: Item }) {
  const admin = useAdminServers().some((s) => s.id === server.id);
  if (!admin || (item.kind !== ItemKind.MOVIE && item.kind !== ItemKind.EPISODE)) return null;
  return <SubtitlesPanel server={server} item={item} />;
}

function SubtitlesPanel({ server, item }: { server: ServerEntry; item: Item }) {
  const subs = useItemSubtitles(server, item.id);
  const [searching, setSearching] = useState(false);
  const [uploading, setUploading] = useState(false);
  const data = subs.data;
  const external = data?.subtitles.filter((s) => s.path) ?? [];
  return (
    <section className="flex flex-col gap-4 px-margin pt-8">
      <h2 className="type-title3">{t("itemsubs.title")}</h2>
      <div className="flex max-w-3xl flex-col gap-6">
        {subs.isError && <p className="type-callout break-words text-danger">{errorMessage(subs.error)}</p>}
        {data && (
          <Group footer={t("itemsubs.profileFooter")}>
            <div className="flex min-h-12 flex-wrap items-center justify-between gap-x-3 gap-y-1 px-4 py-2">
              <span className="type-body">{t("itemsubs.profile")}</span>
              <span className="type-callout text-text-secondary">{data.profile?.name || t("itemsubs.noProfile")}</span>
            </div>
            {data.missing.length > 0 && (
              <div className="flex min-h-12 flex-wrap items-center justify-between gap-x-3 gap-y-1 px-4 py-2">
                <span className="type-body">{t("itemsubs.missing")}</span>
                <span className="type-callout text-danger">{data.missing.map(specLabel).join(", ")}</span>
              </div>
            )}
            <div className="flex flex-wrap gap-2 px-4 py-3">
              <Button variant="secondary" onClick={() => setSearching(true)}>
                {t("itemsubs.search")}
              </Button>
              <Button variant="secondary" onClick={() => setUploading(!uploading)}>
                {t("itemsubs.upload")}
              </Button>
            </div>
            {uploading && <UploadPanel server={server} itemId={item.id} onDone={() => setUploading(false)} />}
          </Group>
        )}
        {data && (
          <Group title={t("itemsubs.present")}>
            {data.subtitles.map((s) => (
              <SubtitleRow key={s.path || `stream-${s.streamIndex}`} server={server} itemId={item.id} subtitle={s} external={external} />
            ))}
            {data.subtitles.length === 0 && <p className="px-4 py-3 type-callout text-text-secondary">{t("itemsubs.none")}</p>}
          </Group>
        )}
      </div>
      {searching && data && <SearchDialog server={server} item={item} missing={data.missing} onClose={() => setSearching(false)} />}
    </section>
  );
}

type Panel = "sync" | "mods" | "translate" | "delete";

function SubtitleRow({ server, itemId, subtitle: s, external }: { server: ServerEntry; itemId: string; subtitle: ItemSubtitle; external: ItemSubtitle[] }) {
  const actions = useItemSubtitleActions(server, itemId);
  const [panel, setPanel] = useState<Panel>();
  const toggle = (p: Panel) => setPanel(panel === p ? undefined : p);
  const detail = [
    s.path ? fileName(s.path) : t("itemsubs.embedded", { index: s.streamIndex }),
    s.codec.toUpperCase(),
    !s.counted && t("itemsubs.notCounted"),
  ].filter(Boolean);
  if (panel === "delete")
    return (
      <Confirm
        title={t("itemsubs.deleteTitle", { name: fileName(s.path) })}
        message={actions.remove.isError ? errorMessage(actions.remove.error) : undefined}
        action={t("itemsubs.delete")}
        busy={actions.remove.isPending}
        onCancel={() => setPanel(undefined)}
        onConfirm={() => actions.remove.mutate(s.path, { onSuccess: () => setPanel(undefined) })}
      />
    );
  return (
    <div className="flex flex-col gap-2 px-4 py-3">
      <div className="flex flex-wrap items-center gap-x-3 gap-y-2">
        <div className="min-w-0 flex-1 basis-56">
          <div className="type-body">{specLabel(s.language)}</div>
          <div className="type-footnote break-all text-text-secondary">{detail.join(" · ")}</div>
        </div>
        <div className="flex flex-wrap gap-1">
          {s.path ? (
            <>
              <Button variant="plain" onClick={() => toggle("sync")}>
                {t("itemsubs.sync")}
              </Button>
              <Button variant="plain" onClick={() => toggle("mods")}>
                {t("itemsubs.modify")}
              </Button>
            </>
          ) : (
            <Button variant="plain" disabled={actions.extract.isPending} onClick={() => actions.extract.mutate({ streamIndex: s.streamIndex, language: s.language })}>
              {actions.extract.isPending ? "…" : t("itemsubs.extract")}
            </Button>
          )}
          <Button variant="plain" onClick={() => toggle("translate")}>
            {t("itemsubs.translate")}
          </Button>
          {s.path && (
            <Button variant="plain" className="text-danger" onClick={() => setPanel("delete")}>
              {t("itemsubs.delete")}
            </Button>
          )}
        </div>
      </div>
      {actions.extract.isError && <p className="type-footnote break-words text-danger">{errorMessage(actions.extract.error)}</p>}
      {panel === "sync" && <SyncPanel server={server} itemId={itemId} subtitle={s} others={external.filter((x) => x.path !== s.path)} />}
      {panel === "mods" && <ModsPanel server={server} itemId={itemId} path={s.path} onDone={() => setPanel(undefined)} />}
      {panel === "translate" && <TranslatePanel server={server} itemId={itemId} subtitle={s} />}
    </div>
  );
}

/** A row of controls under a subtitle, on the inset fill. */
function Inset({ children }: { children: ReactNode }) {
  return <div className="flex flex-col gap-3 rounded-[10px] bg-fill p-3">{children}</div>;
}

function SyncPanel({ server, itemId, subtitle: s, others }: { server: ServerEntry; itemId: string; subtitle: ItemSubtitle; others: ItemSubtitle[] }) {
  const { sync } = useItemSubtitleActions(server, itemId);
  const [reference, setReference] = useState("");
  return (
    <Inset>
      <div className="flex flex-wrap items-center gap-2">
        <Select
          value={reference}
          label={t("itemsubs.reference")}
          options={[{ value: "", label: t("itemsubs.audio") }, ...others.map((o) => ({ value: o.path, label: `${specLabel(o.language)} · ${fileName(o.path)}` }))]}
          onChange={setReference}
        />
        <Button disabled={sync.isPending} onClick={() => sync.mutate({ path: s.path, referencePath: reference || undefined })}>
          {sync.isPending ? t("itemsubs.syncing") : t("itemsubs.sync")}
        </Button>
      </div>
      {sync.isSuccess && (
        <p className="type-footnote text-text-secondary">{t("itemsubs.synced", { offset: Number(sync.data.offsetMs), ratio: sync.data.ratio.toFixed(4) })}</p>
      )}
      {sync.isError && <p className="type-footnote break-words text-danger">{errorMessage(sync.error)}</p>}
    </Inset>
  );
}

function ModsPanel({ server, itemId, path, onDone }: { server: ServerEntry; itemId: string; path: string; onDone: () => void }) {
  const { modify } = useItemSubtitleActions(server, itemId);
  const [chosen, setChosen] = useState<string[]>([]);
  return (
    <Inset>
      {mods.map(([m, label]) => (
        <div key={m} className="flex items-center justify-between gap-3">
          <span className="type-callout">{label()}</span>
          <Switch on={chosen.includes(m)} onChange={(on) => setChosen(on ? [...chosen, m] : chosen.filter((x) => x !== m))} label={label()} />
        </div>
      ))}
      <div className="flex items-center gap-2">
        <Button disabled={chosen.length === 0 || modify.isPending} onClick={() => modify.mutate({ path, mods: chosen }, { onSuccess: onDone })}>
          {modify.isPending ? "…" : t("itemsubs.apply")}
        </Button>
        {modify.isError && <p className="type-footnote break-words text-danger">{errorMessage(modify.error)}</p>}
      </div>
    </Inset>
  );
}

function TranslatePanel({ server, itemId, subtitle: s }: { server: ServerEntry; itemId: string; subtitle: ItemSubtitle }) {
  const { translate } = useItemSubtitleActions(server, itemId);
  const translators = useSubtitleTranslators(server);
  const models = useAiModels(server, true);
  const [language, setLanguage] = useState("");
  const [translator, setTranslator] = useState("");
  const languages = useSubtitleLanguages(server, language);
  const options = [
    { value: "", label: t("itemsubs.settingsTranslator") },
    ...(translators.data?.translators ?? []).map((x) => ({ value: x.id, label: x.displayName || x.name })),
    ...(models.data ?? []).map((m) => ({ value: `ai/${m.provider}/${m.id}`, label: `${m.name || m.id} (${m.provider})` })),
  ];
  const source = s.path ? ({ case: "path", value: s.path } as const) : ({ case: "streamIndex", value: s.streamIndex } as const);
  return (
    <Inset>
      <div className="flex flex-wrap items-center gap-2">
        <Select value={language} label={t("itemsubs.into")} options={[{ value: "", label: t("itemsubs.into") }, ...languages]} onChange={setLanguage} />
        <Select value={translator} label={t("subtitles.translator")} options={options} onChange={setTranslator} />
        <Button
          disabled={!language || translate.isPending || translate.isSuccess}
          onClick={() => translate.mutate({ source, language: { language }, translator: translator || undefined })}
        >
          {translate.isSuccess ? t("itemsubs.translationQueued") : translate.isPending ? "…" : t("itemsubs.translate")}
        </Button>
      </div>
      {translate.isSuccess && <p className="type-footnote text-text-secondary">{t("itemsubs.translationFooter")}</p>}
      {translate.isError && <p className="type-footnote break-words text-danger">{errorMessage(translate.error)}</p>}
    </Inset>
  );
}

const formats = ["srt", "ass", "ssa", "vtt", "sub"];

function UploadPanel({ server, itemId, onDone }: { server: ServerEntry; itemId: string; onDone: () => void }) {
  const { upload } = useItemSubtitleActions(server, itemId);
  const [file, setFile] = useState<File>();
  const [language, setLanguage] = useState("");
  const [forced, setForced] = useState(false);
  const [hi, setHi] = useState(false);
  const languages = useSubtitleLanguages(server, language);
  const format = file?.name.split(".").pop()?.toLowerCase() ?? "";
  const send = async () => {
    if (!file) return;
    const data = new Uint8Array(await file.arrayBuffer());
    upload.mutate({ data, format, language: { language, forced, hearingImpaired: hi } }, { onSuccess: onDone });
  };
  return (
    <div className="flex flex-col gap-3 px-4 py-3">
      <FileField label={t("itemsubs.chooseFile")} accept={formats.map((f) => `.${f}`).join(",")} file={file} onChange={setFile} />
      {file && !formats.includes(format) && <p className="type-footnote text-danger">{t("itemsubs.badFormat")}</p>}
      <Select value={language} label={t("itemsubs.language")} options={[{ value: "", label: t("itemsubs.language") }, ...languages]} onChange={setLanguage} />
      <div className="flex items-center justify-between gap-3">
        <span className="type-callout">{t("subtitles.forced")}</span>
        <Switch on={forced} onChange={(on) => (setForced(on), on && setHi(false))} label={t("subtitles.forced")} />
      </div>
      <div className="flex items-center justify-between gap-3">
        <span className="type-callout">{t("subtitles.hearingImpaired")}</span>
        <Switch on={hi} onChange={(on) => (setHi(on), on && setForced(false))} label={t("subtitles.hearingImpaired")} />
      </div>
      <div className="flex items-center gap-2">
        <Button disabled={!file || !language || !formats.includes(format) || upload.isPending} onClick={() => void send()}>
          {upload.isPending ? "…" : t("itemsubs.uploadAction")}
        </Button>
        {upload.isError && <p className="type-footnote break-words text-danger">{errorMessage(upload.error)}</p>}
      </div>
    </div>
  );
}

/** The providers' subtitles of the item in a language, or those its profile wants. */
function SearchDialog({ server, item, missing, onClose }: { server: ServerEntry; item: Item; missing: SubtitleLanguageSpec[]; onClose: () => void }) {
  const [language, setLanguage] = useState(missing.length > 0 ? "" : discoverLanguage);
  const languages = useSubtitleLanguages(server, language);
  const specs: Spec[] = language
    ? [{ language }]
    : missing.map((m) => ({ language: m.language, forced: m.forced, hearingImpaired: m.hearingImpaired }));
  const search = useSubtitleSearch(server, item.id, specs, true);
  const list = search.data?.subtitles ?? [];
  return (
    <Sheet label={t("itemsubs.searchTitle", { name: item.name })} width="full" onClose={onClose}>
      <div className="flex max-h-[85vh] min-h-0 flex-col gap-3 p-6">
        <div className="flex flex-wrap items-center gap-3">
          <h2 className="min-w-0 flex-1 type-title2">
            {t("itemsubs.searchTitle", { name: item.name })}
          </h2>
          <Select
            value={language}
            label={t("itemsubs.language")}
            options={[...(missing.length > 0 ? [{ value: "", label: t("itemsubs.wantedLanguages") }] : []), ...languages]}
            onChange={setLanguage}
          />
          <Button variant="plain" onClick={onClose}>
            {t("keys.done")}
          </Button>
        </div>
        {(search.data?.failedProviders ?? []).map((p) => (
          <p key={p} className="type-footnote text-danger">
            {t("itemsubs.providerFailed", { provider: p })}
          </p>
        ))}
        <div className="-mx-6 min-h-0 flex-1 divide-y divide-border overflow-y-auto border-t border-border">
          {search.isFetching && list.length === 0 && <p className="px-6 py-4 type-callout text-text-secondary">{t("itemsubs.searching")}</p>}
          {search.isSuccess && !search.isFetching && list.length === 0 && <p className="px-6 py-4 type-callout text-text-secondary">{t("itemsubs.noResults")}</p>}
          {search.isError && <p className="px-6 py-4 type-callout break-words text-danger">{errorMessage(search.error)}</p>}
          {list.map((r) => (
            <ResultRow key={`${r.provider}/${r.subtitleId}`} server={server} itemId={item.id} result={r} />
          ))}
        </div>
      </div>
    </Sheet>
  );
}

function ResultRow({ server, itemId, result: r }: { server: ServerEntry; itemId: string; result: ScoredSubtitle }) {
  const { download } = useItemSubtitleActions(server, itemId);
  const detail = [specLabel(r.language), t("itemsubs.score", { percent: r.percent }), r.provider, r.uploader, r.format.toUpperCase()].filter(Boolean);
  return (
    <div className="flex flex-wrap items-center gap-x-4 gap-y-1 px-6 py-3">
      <div className="flex min-w-0 flex-1 basis-72 flex-col gap-0.5">
        <p className="type-callout break-all">{r.releaseNames[0] || r.subtitleId}</p>
        {r.releaseNames.length > 1 && <p className="type-footnote break-all text-text-secondary">{r.releaseNames.slice(1).join(", ")}</p>}
        <p className="type-footnote text-text-secondary">{detail.join(" · ")}</p>
        {r.notMatched.length > 0 && <p className="type-footnote text-text-tertiary">{t("itemsubs.notMatched", { fields: r.notMatched.join(", ") })}</p>}
        {r.blocklisted && <p className="type-footnote text-danger">{t("itemsubs.blocklisted")}</p>}
        {download.isError && <p className="type-footnote break-words text-danger">{errorMessage(download.error)}</p>}
      </div>
      <Button
        variant={r.blocklisted ? "secondary" : "primary"}
        disabled={download.isPending || download.isSuccess || !r.language}
        onClick={() => r.language && download.mutate({ provider: r.provider, subtitleId: r.subtitleId, language: r.language })}
      >
        {download.isSuccess ? t("itemsubs.downloaded") : download.isPending ? "…" : t("itemsubs.download")}
      </Button>
    </div>
  );
}
