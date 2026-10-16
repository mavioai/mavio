// Acquisition's rules (docs/clients.md §6.3.11): delay profiles, which
// hold releases back for better ones, in order; release profiles, which
// require or ignore terms in releases' titles; and remote path mappings,
// which map the folders download clients on other hosts report.
import { create } from "@bufbuild/protobuf";
import { type Duration, durationFromMs, durationMs } from "@bufbuild/protobuf/wkt";
import { errorMessage, type ServerEntry, useDelayProfiles, useIndexers, useReleaseProfiles, useRemotePathMappings } from "@mavio/client-ts";
import {
  type DelayProfile,
  DelayProfileSchema,
  type ReleaseProfile,
  ReleaseProfileSchema,
  RemotePathMappingSchema,
} from "@mavio/proto/mavio/acquisition/v1/profile_pb";
import { DownloadProtocol, IndexerRefSchema } from "@mavio/proto/mavio/acquisition/v1/release_pb";
import { type ReactNode, useState } from "react";

import { t } from "../../i18n.ts";
import { Icon } from "../../icon.tsx";
import { Button, Group, Row } from "../../ui.tsx";
import { Switch } from "../../controls/inputs.tsx";
import { Select } from "../../controls/select.tsx";
import { BackLink, Page, patched } from "./common.tsx";
import { input } from "../../controls/styles.ts";
import { NumberInput } from "./quality.tsx";

const back = () => <BackLink section="acquisition" label={t("settings.acquisition")} />;

const splitList = (s: string, by: RegExp) =>
  s
    .split(by)
    .map((x) => x.trim())
    .filter(Boolean);

/** A row of a list that opens into its form. */
function EntryRow({ label, detail, disabled, onClick, trailing }: { label: ReactNode; detail?: ReactNode; disabled?: boolean; onClick: () => void; trailing?: ReactNode }) {
  return (
    <div className="flex min-h-13 items-center gap-1 pr-2">
      <button type="button" disabled={disabled} onClick={onClick} className="min-w-0 flex-1 px-4 py-2.5 text-left enabled:hover:bg-surface-raised">
        <div className="truncate type-body">{label}</div>
        {detail && <div className="type-footnote break-words text-text-secondary">{detail}</div>}
      </button>
      {trailing}
    </div>
  );
}

/** The buttons under an entry's form. */
function FormButtons({
  valid,
  busy,
  error,
  onSave,
  onCancel,
  onRemove,
}: {
  valid: boolean;
  busy: boolean;
  error?: unknown;
  onSave: () => void;
  onCancel: () => void;
  onRemove?: () => void;
}) {
  const [removing, setRemoving] = useState(false);
  return (
    <div className="flex flex-wrap items-center gap-3 px-4 py-2.5">
      <Button disabled={!valid || busy} onClick={onSave}>
        {busy ? "…" : t("action.save")}
      </Button>
      <Button variant="plain" onClick={onCancel}>
        {t("action.cancel")}
      </Button>
      {onRemove &&
        (removing ? (
          <Button variant="secondary" className="text-danger" disabled={busy} onClick={onRemove}>
            {t("rules.removeConfirm")}
          </Button>
        ) : (
          <Button variant="plain" className="text-danger" onClick={() => setRemoving(true)}>
            {t("rules.remove")}
          </Button>
        ))}
      {error != null && <span className="type-callout break-words text-danger">{errorMessage(error)}</span>}
    </div>
  );
}

const minutes = (d?: Duration) => (d ? Math.round(durationMs(d) / 60_000) : 0);

/** Acquisition's page of the delay profiles. */
export function DelayProfilesPage({ server }: { server: ServerEntry }) {
  const delays = useDelayProfiles(server);
  // The profile edited, by ID; "new" for one to add.
  const [editing, setEditing] = useState<string>();
  const list = delays.list.data ?? [];
  const ordered = list.filter((p) => !p.default);
  const move = (i: number, by: number) => {
    const ids = ordered.map((p) => p.id);
    const [id] = ids.splice(i, 1);
    ids.splice(i + by, 0, id);
    delays.reorder.mutate(ids);
  };
  const summary = (p: DelayProfile) => {
    const protocols = [p.enableUsenet && t("acquisition.usenet"), p.enableTorrent && t("acquisition.torrent")].filter(Boolean).join(" + ");
    return [
      protocols,
      t("rules.prefers", { protocol: p.preferredProtocol === DownloadProtocol.TORRENT ? t("acquisition.torrent") : t("acquisition.usenet") }),
      p.enableUsenet && t("rules.usenetDelayOf", { minutes: minutes(p.usenetDelay) }),
      p.enableTorrent && t("rules.torrentDelayOf", { minutes: minutes(p.torrentDelay) }),
    ]
      .filter(Boolean)
      .join(" · ");
  };
  const close = () => setEditing(undefined);
  const reset = () => (delays.create.reset(), delays.update.reset(), delays.remove.reset());
  return (
    <Page title={t("acquisition.delays")} message={t("rules.delaysMessage")} back={back()}>
      <Group
        title={
          <span className="flex items-center justify-between gap-3">
            {t("rules.delaysInOrder")}
            {editing === undefined && (
              <Button variant="plain" onClick={() => (reset(), setEditing("new"))}>
                {t("rules.add")}
              </Button>
            )}
          </span>
        }
        footer={t("rules.delaysFooter")}
      >
        {list.map((p) =>
          editing === p.id ? (
            <DelayForm
              key={p.id}
              profile={p}
              busy={delays.update.isPending || delays.remove.isPending}
              error={delays.update.error ?? delays.remove.error}
              onCancel={close}
              onSave={(x) => delays.update.mutate(x, { onSuccess: close })}
              onRemove={p.default ? undefined : () => delays.remove.mutate(p.id, { onSuccess: close })}
            />
          ) : (
            <EntryRow
              key={p.id}
              label={p.default ? t("rules.defaultDelay") : p.tags.join(", ")}
              detail={summary(p)}
              disabled={editing !== undefined}
              onClick={() => (reset(), setEditing(p.id))}
              trailing={
                !p.default && (
                  <>
                    <button type="button" aria-label={t("acquisition.moveUp")} disabled={p.order === ordered[0]?.order || delays.reorder.isPending} onClick={() => move(ordered.indexOf(p), -1)} className="rounded-md p-1.5 text-text-secondary hover:bg-fill disabled:opacity-30">
                      <Icon name="back" size={16} className="rotate-90" />
                    </button>
                    <button type="button" aria-label={t("acquisition.moveDown")} disabled={p === ordered.at(-1) || delays.reorder.isPending} onClick={() => move(ordered.indexOf(p), 1)} className="rounded-md p-1.5 text-text-secondary hover:bg-fill disabled:opacity-30">
                      <Icon name="back" size={16} className="-rotate-90" />
                    </button>
                  </>
                )
              }
            />
          ),
        )}
        {editing === "new" && (
          <DelayForm
            profile={create(DelayProfileSchema, { enableUsenet: true, enableTorrent: true, preferredProtocol: DownloadProtocol.USENET })}
            busy={delays.create.isPending}
            error={delays.create.error}
            onCancel={close}
            onSave={(x) => delays.create.mutate(x, { onSuccess: close })}
          />
        )}
        {delays.list.isError && <p className="px-4 py-3 type-callout text-danger">{errorMessage(delays.list.error)}</p>}
        {delays.reorder.isError && <p className="px-4 py-3 type-callout text-danger">{errorMessage(delays.reorder.error)}</p>}
      </Group>
    </Page>
  );
}

function DelayForm({
  profile,
  busy,
  error,
  onCancel,
  onSave,
  onRemove,
}: {
  profile: DelayProfile;
  busy: boolean;
  error?: unknown;
  onCancel: () => void;
  onSave: (p: DelayProfile) => void;
  onRemove?: () => void;
}) {
  const [d, setD] = useState(() => ({
    usenet: profile.enableUsenet,
    torrent: profile.enableTorrent,
    preferred: (profile.preferredProtocol || DownloadProtocol.USENET) as DownloadProtocol,
    usenetDelay: minutes(profile.usenetDelay),
    torrentDelay: minutes(profile.torrentDelay),
    bypassQuality: profile.bypassIfHighestQuality,
    bypassScore: profile.bypassIfAboveCustomFormatScore,
    minScore: profile.minimumCustomFormatScore,
    tags: profile.tags.join(", "),
  }));
  const set = (patch: Partial<typeof d>) => setD((x) => ({ ...x, ...patch }));
  const tags = splitList(d.tags, /,/);
  const valid = (d.usenet || d.torrent) && (profile.default || tags.length > 0);
  const toggle = (key: "usenet" | "torrent" | "bypassQuality" | "bypassScore", label: string, detail?: string) => (
    <Row label={label} detail={detail}>
      <Switch on={d[key]} label={label} onChange={(on) => set({ [key]: on })} />
    </Row>
  );
  return (
    <div className="flex flex-col divide-y divide-border bg-background/40">
      {!profile.default && (
        <Row label={t("rules.tags")} detail={t("rules.delayTagsDetail")}>
          <input className={`${input} sm:w-72`} aria-label={t("rules.tags")} value={d.tags} onChange={(e) => set({ tags: e.target.value })} />
        </Row>
      )}
      {toggle("usenet", t("acquisition.usenet"))}
      {toggle("torrent", t("acquisition.torrent"))}
      {d.usenet && d.torrent && (
        <Row label={t("rules.preferred")}>
          <Select
            value={String(d.preferred)}
            label={t("rules.preferred")}
            options={[
              { value: String(DownloadProtocol.USENET), label: t("acquisition.usenet") },
              { value: String(DownloadProtocol.TORRENT), label: t("acquisition.torrent") },
            ]}
            onChange={(v) => set({ preferred: Number(v) as DownloadProtocol })}
          />
        </Row>
      )}
      {d.usenet && (
        <Row label={t("rules.usenetDelay")} detail={t("rules.minutes")}>
          <NumberInput value={d.usenetDelay} max={525600} label={t("rules.usenetDelay")} onChange={(v) => set({ usenetDelay: Math.trunc(v) })} />
        </Row>
      )}
      {d.torrent && (
        <Row label={t("rules.torrentDelay")} detail={t("rules.minutes")}>
          <NumberInput value={d.torrentDelay} max={525600} label={t("rules.torrentDelay")} onChange={(v) => set({ torrentDelay: Math.trunc(v) })} />
        </Row>
      )}
      {toggle("bypassQuality", t("rules.bypassQuality"), t("rules.bypassQualityDetail"))}
      {toggle("bypassScore", t("rules.bypassScore"))}
      {d.bypassScore && (
        <Row label={t("rules.minScore")}>
          <NumberInput value={d.minScore} min={-100000} max={100000} label={t("rules.minScore")} onChange={(v) => set({ minScore: Math.trunc(v) })} />
        </Row>
      )}
      <FormButtons
        valid={valid}
        busy={busy}
        error={error}
        onCancel={onCancel}
        onRemove={onRemove}
        onSave={() =>
          onSave(
            patched(DelayProfileSchema, profile, {
              enableUsenet: d.usenet,
              enableTorrent: d.torrent,
              preferredProtocol: d.usenet && d.torrent ? d.preferred : d.usenet ? DownloadProtocol.USENET : DownloadProtocol.TORRENT,
              usenetDelay: durationFromMs(d.usenetDelay * 60_000),
              torrentDelay: durationFromMs(d.torrentDelay * 60_000),
              bypassIfHighestQuality: d.bypassQuality,
              bypassIfAboveCustomFormatScore: d.bypassScore,
              minimumCustomFormatScore: d.minScore,
              tags: profile.default ? [] : tags,
            }),
          )
        }
      />
    </div>
  );
}

/** Acquisition's page of the release profiles. */
export function ReleaseProfilesPage({ server }: { server: ServerEntry }) {
  const releases = useReleaseProfiles(server);
  const indexers = useIndexers(server);
  const [editing, setEditing] = useState<string>();
  const list = releases.list.data ?? [];
  const close = () => setEditing(undefined);
  const reset = () => (releases.create.reset(), releases.update.reset(), releases.remove.reset());
  const indexerName = (p: ReleaseProfile) =>
    p.indexer && (indexers.data?.indexers.find((x) => x.pluginId === p.indexer?.pluginId && x.id === p.indexer?.indexerId)?.name ?? p.indexer.indexerId);
  const form = (p: ReleaseProfile, isNew: boolean) => (
    <ReleaseForm
      key={p.id || "new"}
      profile={p}
      indexers={indexers.data?.indexers ?? []}
      busy={releases.create.isPending || releases.update.isPending || releases.remove.isPending}
      error={releases.create.error ?? releases.update.error ?? releases.remove.error}
      onCancel={close}
      onSave={(x) => (isNew ? releases.create : releases.update).mutate(x, { onSuccess: close })}
      onRemove={isNew ? undefined : () => releases.remove.mutate(p.id, { onSuccess: close })}
    />
  );
  return (
    <Page title={t("acquisition.releases")} message={t("rules.releasesMessage")} back={back()}>
      <Group
        title={
          <span className="flex items-center justify-between gap-3">
            {t("acquisition.releases")}
            {editing === undefined && (
              <Button variant="plain" onClick={() => (reset(), setEditing("new"))}>
                {t("rules.add")}
              </Button>
            )}
          </span>
        }
        footer={t("rules.releasesFooter")}
      >
        {list.map((p) =>
          editing === p.id ? (
            form(p, false)
          ) : (
            <EntryRow
              key={p.id}
              label={p.name || t("rules.unnamed")}
              detail={[
                !p.enabled && t("acquisition.off"),
                p.required.length > 0 && t("rules.requiredCount", { count: p.required.length }),
                p.ignored.length > 0 && t("rules.ignoredCount", { count: p.ignored.length }),
                indexerName(p),
                p.tags.join(", "),
              ]
                .filter(Boolean)
                .join(" · ")}
              disabled={editing !== undefined}
              onClick={() => (reset(), setEditing(p.id))}
            />
          ),
        )}
        {editing === "new" && form(create(ReleaseProfileSchema, { enabled: true }), true)}
        {releases.list.isSuccess && list.length === 0 && editing === undefined && <p className="px-4 py-3 type-callout text-text-secondary">{t("rules.noReleases")}</p>}
        {releases.list.isError && <p className="px-4 py-3 type-callout text-danger">{errorMessage(releases.list.error)}</p>}
      </Group>
    </Page>
  );
}

function ReleaseForm({
  profile,
  indexers,
  busy,
  error,
  onCancel,
  onSave,
  onRemove,
}: {
  profile: ReleaseProfile;
  indexers: { pluginId: string; id: string; name: string }[];
  busy: boolean;
  error?: unknown;
  onCancel: () => void;
  onSave: (p: ReleaseProfile) => void;
  onRemove?: () => void;
}) {
  const [d, setD] = useState(() => ({
    name: profile.name,
    enabled: profile.enabled,
    required: profile.required.join("\n"),
    ignored: profile.ignored.join("\n"),
    indexer: profile.indexer ? `${profile.indexer.pluginId}/${profile.indexer.indexerId}` : "",
    tags: profile.tags.join(", "),
  }));
  const set = (patch: Partial<typeof d>) => setD((x) => ({ ...x, ...patch }));
  const required = splitList(d.required, /\n/);
  const ignored = splitList(d.ignored, /\n/);
  const terms = (key: "required" | "ignored", label: string, detail: string) => (
    <div className="flex flex-col gap-1.5 px-4 py-2.5">
      <div className="type-body">{label}</div>
      <div className="type-footnote text-text-secondary">{detail}</div>
      <textarea rows={3} className={`${input} py-2 font-mono`} aria-label={label} value={d[key]} onChange={(e) => set({ [key]: e.target.value })} />
    </div>
  );
  const [plugin, id] = d.indexer.split("/");
  return (
    <div className="flex flex-col divide-y divide-border bg-background/40">
      <Row label={t("rules.name")}>
        <input className={`${input} sm:w-72`} aria-label={t("rules.name")} value={d.name} onChange={(e) => set({ name: e.target.value })} />
      </Row>
      <Row label={t("rules.enabled")}>
        <Switch on={d.enabled} label={t("rules.enabled")} onChange={(enabled) => set({ enabled })} />
      </Row>
      {terms("required", t("rules.required"), t("rules.requiredDetail"))}
      {terms("ignored", t("rules.ignored"), t("rules.ignoredDetail"))}
      <Row label={t("rules.indexer")}>
        <Select
          value={d.indexer}
          label={t("rules.indexer")}
          className="w-48"
          options={[
            { value: "", label: t("rules.allIndexers") },
            ...indexers.map((x) => ({ value: `${x.pluginId}/${x.id}`, label: x.name })),
            ...(d.indexer && !indexers.some((x) => `${x.pluginId}/${x.id}` === d.indexer) ? [{ value: d.indexer, label: id ?? d.indexer }] : []),
          ]}
          onChange={(indexer) => set({ indexer })}
        />
      </Row>
      <Row label={t("rules.tags")} detail={t("rules.releaseTagsDetail")}>
        <input className={`${input} sm:w-72`} aria-label={t("rules.tags")} value={d.tags} onChange={(e) => set({ tags: e.target.value })} />
      </Row>
      <FormButtons
        valid={required.length + ignored.length > 0}
        busy={busy}
        error={error}
        onCancel={onCancel}
        onRemove={onRemove}
        onSave={() =>
          onSave(
            create(ReleaseProfileSchema, {
              id: profile.id,
              name: d.name.trim(),
              enabled: d.enabled,
              required,
              ignored,
              indexer: d.indexer ? create(IndexerRefSchema, { pluginId: plugin, indexerId: id }) : undefined,
              tags: splitList(d.tags, /,/),
            }),
          )
        }
      />
    </div>
  );
}

/** Acquisition's page of the remote path mappings. */
export function RemotePathMappingsPage({ server }: { server: ServerEntry }) {
  const mappings = useRemotePathMappings(server);
  const [adding, setAdding] = useState(false);
  const [d, setD] = useState({ host: "", remote: "", local: "" });
  const list = mappings.list.data ?? [];
  const valid = d.host.trim() && d.remote.trim() && /^(\/|[A-Za-z]:[\\/])/.test(d.local.trim());
  const text = (key: keyof typeof d, label: string, detail: string, placeholder: string) => (
    <Row label={label} detail={detail}>
      <input className={`${input} sm:w-72`} aria-label={label} placeholder={placeholder} value={d[key]} onChange={(e) => setD((x) => ({ ...x, [key]: e.target.value }))} />
    </Row>
  );
  return (
    <Page title={t("acquisition.mappings")} message={t("rules.mappingsMessage")} back={back()}>
      <Group
        title={
          <span className="flex items-center justify-between gap-3">
            {t("acquisition.mappings")}
            {!adding && (
              <Button variant="plain" onClick={() => (mappings.create.reset(), setAdding(true))}>
                {t("rules.add")}
              </Button>
            )}
          </span>
        }
      >
        {list.map((m) => (
          <Row key={m.id} label={m.host} detail={`${m.remotePath} → ${m.localPath}`}>
            <Button
              variant="plain"
              className="text-danger"
              disabled={mappings.remove.isPending && mappings.remove.variables === m.id}
              onClick={() => mappings.remove.mutate(m.id)}
            >
              {t("rules.remove")}
            </Button>
          </Row>
        ))}
        {adding && (
          <div className="flex flex-col divide-y divide-border bg-background/40">
            {text("host", t("rules.host"), t("rules.hostDetail"), "192.168.1.10")}
            {text("remote", t("rules.remotePath"), t("rules.remotePathDetail"), "/downloads/")}
            {text("local", t("rules.localPath"), t("rules.localPathDetail"), "/mnt/downloads/")}
            <FormButtons
              valid={Boolean(valid)}
              busy={mappings.create.isPending}
              error={mappings.create.error}
              onCancel={() => setAdding(false)}
              onSave={() =>
                mappings.create.mutate(create(RemotePathMappingSchema, { host: d.host.trim(), remotePath: d.remote.trim(), localPath: d.local.trim() }), {
                  onSuccess: () => (setAdding(false), setD({ host: "", remote: "", local: "" })),
                })
              }
            />
          </div>
        )}
        {mappings.list.isSuccess && list.length === 0 && !adding && <p className="px-4 py-3 type-callout text-text-secondary">{t("rules.noMappings")}</p>}
        {(mappings.list.isError || mappings.remove.isError) && (
          <p className="px-4 py-3 type-callout text-danger">{errorMessage(mappings.list.error ?? mappings.remove.error)}</p>
        )}
      </Group>
    </Page>
  );
}
