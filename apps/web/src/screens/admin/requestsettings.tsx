// The Acquisition settings' Requests (docs/clients.md §6.3.11): where
// approved requests go — the library and quality profile of movies, of
// series and of anime — what new users may request and their quotas, and
// the keywords whose titles are blocklisted, as Seerr's Radarr and Sonarr
// servers, user defaults and blocklisted tags.
import {
  errorMessage,
  type NamedValue,
  type ServerEntry,
  useCatalogs,
  useFilterValues,
  useQualityProfiles,
  useRequestSettings,
  useServerLibraries,
} from "@mavio/client-ts";
import { MediaKind } from "@mavio/proto/mavio/acquisition/v1/profile_pb";
import { type RequestSettings, RequestSettingsSchema } from "@mavio/proto/mavio/acquisition/v1/request_pb";
import { MinimumAvailability, NewItemMonitor } from "@mavio/proto/mavio/acquisition/v1/wanted_pb";
import { LibraryKind } from "@mavio/proto/mavio/library/v1/library_pb";
import { DiscoverFilter } from "@mavio/proto/mavio/plugin/v1/discovery_pb";
import { useEffect, useState } from "react";

import { discoverLanguage, t } from "../../i18n.ts";
import { Icon } from "../../icon.tsx";
import { Button, Group, Row } from "../../ui.tsx";
import { NumberField, Switch } from "../../controls/inputs.tsx";
import { Select } from "../../controls/select.tsx";
import { input } from "../../controls/styles.ts";
import { useUnsavedGuard } from "../../controls/unsaved.tsx";
import { BackLink, Page, patched } from "./common.tsx";

/** The page of the request settings. */
export function RequestSettingsPage({ server }: { server: ServerEntry }) {
  const settings = useRequestSettings(server).get;
  return (
    <Page title={t("requestsettings.title")} message={t("requestsettings.message")} back={<BackLink section="acquisition" label={t("settings.acquisition")} />}>
      {settings.isError && <p className="type-callout text-danger">{errorMessage(settings.error)}</p>}
      {settings.data && <RequestSettingsForm server={server} settings={settings.data} />}
    </Page>
  );
}

type Flag = "defaultRequestMovies" | "defaultRequestSeries" | "defaultAutoApproveMovies" | "defaultAutoApproveSeries" | "defaultReportIssues" | "seasonFolder" | "specials";
type Choice = "movieLibraryId" | "movieQualityProfileId" | "seriesLibraryId" | "seriesQualityProfileId" | "animeLibraryId" | "animeQualityProfileId";

function RequestSettingsForm({ server, settings }: { server: ServerEntry; settings: RequestSettings }) {
  const update = useRequestSettings(server).update;
  const libraries = useServerLibraries(server).data ?? [];
  const profiles = useQualityProfiles(server).list.data ?? [];
  const [s, setS] = useState(settings);
  const [notice, setNotice] = useState<string>();
  const set = (patch: Partial<RequestSettings>) => (setS((x) => patched(RequestSettingsSchema, x, patch)), setNotice(undefined));
  const changed = JSON.stringify(s) !== JSON.stringify(settings);
  const guard = useUnsavedGuard(changed);
  const save = () => update.mutate(s, { onSuccess: (r) => (r && setS(r), setNotice(t("acquisition.saved"))) });

  const librariesOf = (kind: LibraryKind) => libraries.filter((l) => l.kind === kind).map((l) => ({ value: l.id, label: l.name }));
  const profilesOf = (kind: MediaKind) => profiles.filter((p) => p.kind === kind).map((p) => ({ value: p.id, label: p.name }));
  const choose = (key: Choice, label: string, options: { value: string; label: string }[], none: string, detail?: string) => {
    // A choice no longer offered, such as a deleted library, stays shown until changed.
    const all = s[key] !== "" && !options.some((o) => o.value === s[key]) ? [...options, { value: s[key], label: s[key] }] : options;
    return (
      <Row label={label} detail={detail}>
        <Select value={s[key]} label={label} className="w-60" options={[{ value: "", label: none }, ...all]} onChange={(v) => set({ [key]: v })} />
      </Row>
    );
  };
  const toggle = (key: Flag, label: string, detail?: string) => (
    <Row label={label} detail={detail}>
      <Switch on={s[key]} label={label} onChange={(on) => set({ [key]: on })} />
    </Row>
  );
  const quota = (limit: "movieQuotaLimit" | "seriesQuotaLimit", days: "movieQuotaDays" | "seriesQuotaDays", label: string) => (
    <Row label={label} detail={t("requestsettings.quotaDetail")}>
      <span className="flex items-center gap-2 type-callout text-text-secondary">
        <NumberField min={0} className="w-28" label={t("users.quotaLimit")} value={s[limit]} onChange={(v) => set({ [limit]: v ?? 0 })} />
        {t("users.quotaEvery")}
        <NumberField min={0} className="w-28" label={t("users.quotaDays")} value={s[days]} onChange={(v) => set({ [days]: v ?? 0 })} />
        {t("users.quotaDaysUnit")}
      </span>
    </Row>
  );
  const availabilities: [MinimumAvailability, string][] = [
    [MinimumAvailability.ANNOUNCED, t("requestsettings.announced")],
    [MinimumAvailability.IN_CINEMAS, t("requestsettings.inCinemas")],
    [MinimumAvailability.RELEASED, t("requestsettings.released")],
  ];
  const monitors: [NewItemMonitor, string][] = [
    [NewItemMonitor.ALL, t("requestsettings.monitorAll")],
    [NewItemMonitor.NONE, t("requestsettings.monitorNone")],
  ];

  return (
    <>
      {guard.dialog}
      <Group title={t("requestsettings.movies")} footer={t("requestsettings.moviesFooter")}>
        {choose("movieLibraryId", t("requestsettings.library"), librariesOf(LibraryKind.MOVIES), t("requestsettings.none"))}
        {choose("movieQualityProfileId", t("requestsettings.profile"), profilesOf(MediaKind.MOVIE), t("requestsettings.none"))}
        <Row label={t("requestsettings.minimumAvailability")} detail={t("requestsettings.minimumAvailabilityDetail")}>
          <Select
            value={String(s.minimumAvailability || MinimumAvailability.RELEASED)}
            label={t("requestsettings.minimumAvailability")}
            className="w-60"
            options={availabilities.map(([v, label]) => ({ value: String(v), label }))}
            onChange={(v) => set({ minimumAvailability: Number(v) as MinimumAvailability })}
          />
        </Row>
      </Group>
      <Group title={t("requestsettings.series")} footer={t("requestsettings.seriesFooter")}>
        {choose("seriesLibraryId", t("requestsettings.library"), librariesOf(LibraryKind.SHOWS), t("requestsettings.none"))}
        {choose("seriesQualityProfileId", t("requestsettings.profile"), profilesOf(MediaKind.SERIES), t("requestsettings.none"))}
        {toggle("seasonFolder", t("requestsettings.seasonFolder"), t("requestsettings.seasonFolderDetail"))}
        <Row label={t("requestsettings.monitorNew")} detail={t("requestsettings.monitorNewDetail")}>
          <Select
            value={String(s.monitorNewItems || NewItemMonitor.ALL)}
            label={t("requestsettings.monitorNew")}
            className="w-60"
            options={monitors.map(([v, label]) => ({ value: String(v), label }))}
            onChange={(v) => set({ monitorNewItems: Number(v) as NewItemMonitor })}
          />
        </Row>
        {toggle("specials", t("requestsettings.specials"), t("requestsettings.specialsDetail"))}
      </Group>
      <Group title={t("requestsettings.anime")} footer={t("requestsettings.animeFooter")}>
        {choose("animeLibraryId", t("requestsettings.library"), librariesOf(LibraryKind.SHOWS), t("requestsettings.sameAsSeries"))}
        {choose("animeQualityProfileId", t("requestsettings.profile"), profilesOf(MediaKind.SERIES), t("requestsettings.sameAsSeries"))}
      </Group>
      <Group title={t("requestsettings.defaults")} footer={t("requestsettings.defaultsFooter")}>
        {toggle("defaultRequestMovies", t("users.requestMovies"))}
        {toggle("defaultAutoApproveMovies", t("users.autoApproveMovies"))}
        {toggle("defaultRequestSeries", t("users.requestSeries"))}
        {toggle("defaultAutoApproveSeries", t("users.autoApproveSeries"))}
        {toggle("defaultReportIssues", t("users.reportIssues"))}
        {quota("movieQuotaLimit", "movieQuotaDays", t("users.movieQuota"))}
        {quota("seriesQuotaLimit", "seriesQuotaDays", t("users.seriesQuota"))}
      </Group>
      <BlocklistedTags server={server} tags={s.blocklistedTags} onChange={(blocklistedTags) => set({ blocklistedTags })}>
        <Row label={t("requestsettings.tagsLimit")} detail={t("requestsettings.tagsLimitDetail")}>
          <NumberField
            min={1}
            max={250}
            className="w-28"
            label={t("requestsettings.tagsLimit")}
            value={s.blocklistedTagsLimit || 50}
            onChange={(v) => set({ blocklistedTagsLimit: v ?? 0 })}
          />
        </Row>
      </BlocklistedTags>
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

/**
 * The blocklisted tags: keywords of a discovery provider, as
 * "<plugin>/<keyword>", found by name in the first catalog filtering by
 * keyword, and shown by their names.
 */
function BlocklistedTags({
  server,
  tags,
  onChange,
  children,
}: {
  server: ServerEntry;
  tags: string[];
  onChange: (tags: string[]) => void;
  children: React.ReactNode;
}) {
  const catalogs = useCatalogs(server, discoverLanguage).data ?? [];
  const catalog = catalogs.find((c) => c.filters.includes(DiscoverFilter.KEYWORD));
  const kind = catalog?.mediaKinds[0] ?? MediaKind.MOVIE;
  const [text, setText] = useState("");
  const [query, setQuery] = useState("");
  useEffect(() => {
    const timer = setTimeout(() => setQuery(text.trim()), 300);
    return () => clearTimeout(timer);
  }, [text]);
  const v = { catalog: catalog?.id ?? "", filter: DiscoverFilter.KEYWORD, mediaKind: kind, language: discoverLanguage };
  const found = useFilterValues(server, { ...v, query }, !!catalog && query !== "");
  // The names of the keywords chosen, those of the catalog's provider.
  const prefix = `${catalog?.pluginId}/`;
  const ids = tags.filter((tag) => tag.startsWith(prefix)).map((tag) => tag.slice(prefix.length));
  const named = useFilterValues(server, { ...v, ids }, !!catalog && ids.length > 0);
  const [added, setAdded] = useState<NamedValue[]>([]);
  const nameOf = (tag: string) => {
    const id = tag.startsWith(prefix) ? tag.slice(prefix.length) : "";
    return named.data?.find((x) => x.id === id)?.name ?? added.find((x) => x.id === id)?.name ?? tag;
  };
  return (
    <Group title={t("requestsettings.tags")} footer={t("requestsettings.tagsFooter")}>
      {tags.length > 0 && (
        <div className="flex flex-wrap gap-2 px-4 py-3">
          {tags.map((tag) => (
            <button
              key={tag}
              type="button"
              aria-label={t("rules.remove")}
              onClick={() => onChange(tags.filter((x) => x !== tag))}
              className="inline-flex min-h-8 items-center gap-1 rounded-full bg-fill px-3 type-footnote font-semibold"
            >
              {nameOf(tag)}
              <Icon name="close" size={14} />
            </button>
          ))}
        </div>
      )}
      {catalog ? (
        <div className="flex flex-col gap-2 px-4 py-3">
          <input
            type="search"
            className={input}
            aria-label={t("requestsettings.searchKeywords")}
            placeholder={t("requestsettings.searchKeywords")}
            value={text}
            onChange={(e) => setText(e.target.value)}
          />
          {query !== "" && (
            <div className="flex flex-wrap gap-2">
              {(found.data ?? [])
                .filter((x) => !tags.includes(prefix + x.id))
                .slice(0, 20)
                .map((x) => (
                  <button
                    key={x.id}
                    type="button"
                    onClick={() => (setAdded((a) => [...a, { id: x.id, name: x.name }]), onChange([...tags, prefix + x.id]), setText(""))}
                    className="inline-flex min-h-8 items-center gap-1 rounded-full border border-separator px-3 type-footnote"
                  >
                    <Icon name="add" size={14} />
                    {x.name}
                  </button>
                ))}
              {found.isSuccess && found.data.length === 0 && <span className="type-callout text-text-secondary">{t("requestsettings.noKeywords")}</span>}
            </div>
          )}
        </div>
      ) : (
        <p className="px-4 py-3 type-callout text-text-secondary">{t("requestsettings.noProvider")}</p>
      )}
      {children}
    </Group>
  );
}
