// The settings' Plugins (docs/clients.md §6.3.3): the plugins of the server
// chosen, as cards in two tabs, Available, what its catalogs offer,
// installed or not, and Installed, filtered by category; a plugin's
// details, to install, update, enable, disable and uninstall it; and the
// catalogs, to add and remove.
import { timestampDate } from "@bufbuild/protobuf/wkt";
import {
  errorMessage,
  type ServerEntry,
  useCatalogPlugins,
  useContainerRuntime,
  usePluginActions,
  usePluginCatalogs,
  usePluginConfig,
  usePluginConfigOptions,
  usePluginContainers,
  usePluginLogs,
  usePlugins,
  useProxies,
} from "@mavio/client-ts";
import { ContainerMode, ContainerState, type ContainerStatus, GpuSupport } from "@mavio/proto/mavio/plugin/v1/container_pb";
import { Category, Runtime } from "@mavio/proto/mavio/plugin/v1/plugin_pb";
import {
  type CatalogPlugin,
  type ContainerCommands,
  type Plugin,
  type PluginContainer,
  PluginState,
} from "@mavio/proto/mavio/system/v1/plugin_pb";
import { Link, useNavigate, useSearch } from "@tanstack/react-router";
import { type FormEvent, type ReactNode, useContext, useState } from "react";

import { locale, t } from "../i18n.ts";
import { Icon, type IconName } from "../icon.tsx";
import { optionSources, type Schema, SchemaForm } from "../schema-form.tsx";
import { Button, Group, Row } from "../ui.tsx";
import { Switch } from "../controls/inputs.tsx";
import { Select } from "../controls/select.tsx";
import type { SettingsSearch } from "./settings.tsx";
import { proxyChoices, proxyDirect } from "./admin/proxies.tsx";
import { BackLink, SectionTrail, TitleAccessory } from "./admin/common.tsx";
import { levelNames, LogRow } from "./admin/backups.tsx";
import { Confirm } from "../controls/dialog.tsx";
import { Tip } from "../controls/tooltip.tsx";
import { useUnsavedGuard } from "../controls/unsaved.tsx";

const states: Record<PluginState, () => string> = {
  [PluginState.UNSPECIFIED]: () => "",
  [PluginState.READY]: () => t("plugins.stateReady"),
  [PluginState.UNCONFIGURED]: () => t("plugins.stateUnconfigured"),
  [PluginState.FAILED]: () => t("plugins.stateFailed"),
  [PluginState.DISABLED]: () => t("plugins.stateDisabled"),
};

const categories: Record<Category, { title: () => string; icon: IconName; color: string }> = {
  [Category.UNSPECIFIED]: { title: () => t("plugins.categoryOther"), icon: "plugins", color: "#8E8E93" },
  [Category.METADATA]: { title: () => t("plugins.categoryMetadata"), icon: "info", color: "#007AFF" },
  [Category.SUBTITLES]: { title: () => t("plugins.categorySubtitles"), icon: "subtitles", color: "#34C759" },
  [Category.DOWNLOADS]: { title: () => t("plugins.categoryDownloads"), icon: "downloads", color: "#FF9500" },
  [Category.AUTHENTICATION]: { title: () => t("plugins.categoryAuthentication"), icon: "key", color: "#FF2D55" },
  [Category.NOTIFICATIONS]: { title: () => t("plugins.categoryNotifications"), icon: "pending", color: "#FF3B30" },
  [Category.DEVICES]: { title: () => t("plugins.categoryDevices"), icon: "television", color: "#32ADE6" },
  [Category.AI]: { title: () => t("plugins.categoryAi"), icon: "plugins", color: "#AF52DE" },
  [Category.SPEECH]: { title: () => t("plugins.categorySpeech"), icon: "plugins", color: "#5856D6" },
};

/** A plugin as shown: what its catalog offers of it and what is installed, either or both. */
interface Entry {
  id: string;
  name: string;
  description: string;
  author: string;
  category: Category;
  offered?: CatalogPlugin;
  installed?: Plugin;
}

function entry(id: string, offered?: CatalogPlugin, installed?: Plugin): Entry {
  const m = installed?.manifest;
  return {
    id,
    name: m?.name || offered?.name || id,
    description: m?.description || offered?.description || "",
    author: m?.author || offered?.author || "",
    category: m?.category || offered?.category || Category.UNSPECIFIED,
    offered,
    installed,
  };
}

/** The newest version the catalog offers when newer than the one installed. */
function updateOf(e: Entry): string | undefined {
  const newest = e.offered?.versions[0]?.version;
  const current = e.installed?.manifest?.version;
  return e.installed && newest && current && newest !== current ? newest : undefined;
}

/** The section of the server chosen in the settings. */
export function PluginsSection({ server }: { server: ServerEntry }) {
  const search = useSearch({ strict: false }) as SettingsSearch;
  if (search.catalogs) return <CatalogsPage server={server} />;
  if (search.plugin) return <PluginPage server={server} id={search.plugin} />;
  return <PluginList server={server} search={search} />;
}

/** The plugins of a server, those its catalogs offer and those installed, by ID. */
function useEntries(server: ServerEntry) {
  const installed = usePlugins(server);
  const catalog = useCatalogPlugins(server);
  const offered = new Map((catalog.data ?? []).map((p) => [p.id, p]));
  const byId = new Map((installed.data ?? []).map((p) => [p.manifest?.id ?? "", p]));
  return {
    installed,
    catalog,
    available: (catalog.data ?? []).map((p) => entry(p.id, p, byId.get(p.id))),
    installedEntries: (installed.data ?? []).map((p) => entry(p.manifest?.id ?? "", offered.get(p.manifest?.id ?? ""), p)),
    get: (id: string) => (offered.has(id) || byId.has(id) ? entry(id, offered.get(id), byId.get(id)) : undefined),
  };
}

const segment =
  "rounded-[7px] px-4 py-1 type-callout font-semibold text-text-secondary hover:text-on-background data-[active=true]:bg-surface-overlay data-[active=true]:text-on-background data-[active=true]:shadow-level1";

function PluginList({ server, search }: { server: ServerEntry; search: SettingsSearch }) {
  const cur = search;
  const accessory = useContext(TitleAccessory);
  const trail = useContext(SectionTrail);
  const { installed, catalog, available, installedEntries } = useEntries(server);
  const tabInstalled = search.tab === "installed";
  const shown = tabInstalled ? installedEntries : available;
  const present = [...new Set(shown.map((e) => e.category))].sort((a, b) => a - b);
  if (search.category !== undefined && !present.includes(search.category)) present.push(search.category);
  const list = shown
    .filter((e) => search.category === undefined || e.category === search.category)
    .sort((a, b) => a.name.localeCompare(b.name, locale));
  const loaded = tabInstalled ? installed.isSuccess : catalog.isSuccess;
  return (
    <div className="flex flex-col gap-7">
      {trail}
      <header>
        <div className="flex items-center gap-4">
          <h2 className="min-w-0 flex-1 type-title1">{t("settings.plugins")}</h2>
          {accessory}
        </div>
        <p className="mt-1 type-body text-text-secondary">{t("settings.pluginsMessage")}</p>
      </header>
      <div className="flex flex-wrap items-center gap-3">
        <div role="tablist" className="flex rounded-[9px] bg-fill p-0.5">
          <Link
            role="tab"
            to="/settings/$section"
            params={{ section: "plugins" }}
            search={{ server: cur.server }}
            aria-selected={!tabInstalled}
            data-active={!tabInstalled}
            className={segment}
          >
            {t("plugins.available")}
          </Link>
          <Link
            role="tab"
            to="/settings/$section"
            params={{ section: "plugins" }}
            search={{ server: cur.server, tab: "installed" as const }}
            aria-selected={tabInstalled}
            data-active={tabInstalled}
            className={segment}
          >
            {t("plugins.installed")}
          </Link>
        </div>
        <div className="ml-auto flex flex-wrap items-center gap-3">
          <CategoryChoice present={present} value={search.category} />
          <Link
            to="/settings/$section"
            params={{ section: "plugins" }}
            search={{ server: cur.server, catalogs: true as const }}
            className="inline-flex min-h-9 cursor-default items-center rounded-md bg-fill px-3 type-callout font-semibold desk:min-h-7 desk:px-2.5 hover:bg-fill-selected"
          >
            {t("plugins.manageCatalogs")}
          </Link>
        </div>
      </div>
      {!tabInstalled && catalog.isError && (
        <p className="type-callout text-danger">{t("plugins.catalogsUnreachable", { reason: errorMessage(catalog.error) })}</p>
      )}
      {loaded && list.length === 0 && (
        <p className="py-10 text-center type-body text-text-secondary">
          {search.category !== undefined
            ? t("plugins.noneInCategory")
            : tabInstalled
              ? t("plugins.noneInstalled")
              : t("plugins.noneAvailable")}
        </p>
      )}
      <div className="grid gap-4 sm:grid-cols-2 xl:grid-cols-3">
        {list.map((e) => (
          <PluginCard key={e.id} entry={e} />
        ))}
      </div>
    </div>
  );
}

/** The choice of the category shown, among those of the tab's plugins. */
function CategoryChoice({ present, value }: { present: Category[]; value: Category | undefined }) {
  const navigate = useNavigate();
  const cur = useSearch({ strict: false }) as SettingsSearch;
  const options = [
    { value: "", label: t("plugins.allCategories") },
    ...present.map((c) => ({ value: String(c), label: categories[c].title() })),
  ];
  return (
    <Select
      value={value === undefined ? "" : String(value)}
      label={t("plugins.category")}
      options={options}
      onChange={(v) =>
        void navigate({
          to: "/settings/$section",
          params: { section: "plugins" },
          search: { ...cur, category: v === "" ? undefined : Number(v) },
          replace: true,
        })
      }
    />
  );
}

function CategoryTile({ category, size = 40 }: { category: Category; size?: number }) {
  const c = categories[category];
  return (
    <span
      className="flex shrink-0 items-center justify-center rounded-[22%] text-white"
      style={{ background: c.color, width: size, height: size }}
    >
      <Icon name={c.icon} fill size={Math.round(size * 0.55)} />
    </span>
  );
}

/** A plugin's state as a short mark. */
function Badge({ entry: e }: { entry: Entry }) {
  const mark = (text: string, tone: string) => <span className={`type-caption font-semibold ${tone}`}>{text}</span>;
  if (!e.installed) {
    const v = e.offered?.versions[0]?.version;
    return v ? mark(v, "text-text-secondary") : mark(t("plugins.noVersion"), "text-text-tertiary");
  }
  const state = e.installed.state;
  const tone = state === PluginState.FAILED ? "text-danger" : state === PluginState.READY ? "text-[#34C759]" : "text-text-secondary";
  return (
    <span className="flex flex-wrap items-center gap-x-2">
      {e.installed.devFolder ? mark(t("plugins.development"), "text-accent-text") : mark(t("plugins.installed"), "text-text-secondary")}
      {mark(states[state](), tone)}
      {updateOf(e) && mark(t("plugins.updateAvailable"), "text-on-background")}
    </span>
  );
}

function PluginCard({ entry: e }: { entry: Entry }) {
  const cur = useSearch({ strict: false }) as SettingsSearch;
  const version = e.installed?.manifest?.version;
  return (
    <Link
      to="/settings/$section"
      params={{ section: "plugins" }}
      search={{ ...cur, plugin: e.id }}
      className="flex min-h-40 flex-col gap-3 rounded-[14px] bg-surface p-4 transition hover:tint"
    >
      <div className="flex items-center gap-3">
        <CategoryTile category={e.category} />
        <div className="min-w-0">
          <div className="truncate type-headline">{e.name}</div>
          <div className="truncate type-footnote text-text-secondary">
            {[categories[e.category].title(), version].filter(Boolean).join(" · ")}
          </div>
        </div>
      </div>
      <p className="line-clamp-3 flex-1 type-footnote text-text-secondary">{e.description}</p>
      <Badge entry={e} />
    </Link>
  );
}

/** The trail back to the list of plugins, as it was left. */
function BackToList() {
  const cur = useSearch({ strict: false }) as SettingsSearch;
  return <BackLink section="plugins" label={t("settings.plugins")} search={{ tab: cur.tab, category: cur.category }} />;
}

const day = new Intl.DateTimeFormat(locale, { dateStyle: "medium" });

/** A plugin's details and what can be done to it. */
function PluginPage({ server, id }: { server: ServerEntry; id: string }) {
  const { installed, catalog, get } = useEntries(server);
  const actions = usePluginActions(server);
  const [confirming, setConfirming] = useState(false);
  const e = get(id);
  if (!e) {
    return (
      <div className="flex flex-col gap-7">
        <BackToList />
        {installed.isSuccess && !catalog.isPending && <p className="type-body text-text-secondary">{t("plugins.notFound")}</p>}
      </div>
    );
  }
  const m = e.installed?.manifest;
  const newest = e.offered?.versions[0]?.version;
  const dev = e.installed?.devFolder ?? "";
  // A plugin under development takes no catalog version.
  const update = dev ? undefined : updateOf(e);
  const enabled = e.installed?.state !== PluginState.DISABLED;
  const toggling = actions.setEnabled.isPending;
  const failure = [actions.install, actions.uninstall, actions.setEnabled, actions.restart].find((x) => x.isError)?.error;
  const installing = (version?: string) =>
    actions.install.isPending && actions.install.variables?.id === id && actions.install.variables.version === version;
  const info: [string, ReactNode][] = [];
  if (e.installed) {
    const state = states[e.installed.state]();
    info.push([t("plugins.state"), e.installed.state === PluginState.FAILED && e.installed.error ? `${state}: ${e.installed.error}` : state]);
    if (m?.version) info.push([t("plugins.version"), m.version]);
    if (dev) info.push([t("plugins.devFolder"), <span key="dev" className="font-mono break-all">{dev}</span>]);
  }
  if (newest) info.push([t("plugins.latestVersion"), newest]);
  info.push([t("plugins.category"), categories[e.category].title()]);
  if (e.author) info.push([t("plugins.author"), e.author]);
  if (m?.license) info.push([t("plugins.license"), m.license]);
  const homepage = m?.homepage || e.offered?.homepage;
  if (homepage) {
    info.push([
      t("plugins.website"),
      <a key="homepage" href={homepage} target="_blank" rel="noreferrer" className="inline-flex items-center gap-1 break-all text-accent-text">
        {homepage}
        <Icon name="external" size={14} />
      </a>,
    ]);
  }
  if (e.offered) info.push([t("plugins.catalog"), <span key="catalog" className="break-all">{e.offered.catalogUrl}</span>]);
  const hosts = m?.permissions?.httpHosts ?? [];
  if (hosts.length > 0) info.push([t("plugins.networkAccess"), hosts.join(", ")]);
  return (
    <div className="flex flex-col gap-7">
      <BackToList />
      <header className="flex flex-wrap items-center gap-4">
        <CategoryTile category={e.category} size={64} />
        {/* A plugin whose manifest cannot be read is named by its long ID: the buttons wrap below rather than squeeze it. */}
        <div className="min-w-48 flex-1">
          <h2 className="type-title1 wrap-anywhere">{e.name}</h2>
          {e.author && <p className="type-body text-text-secondary">{t("plugins.by", { author: e.author })}</p>}
        </div>
        <div className="flex flex-wrap items-center gap-3">
          {!e.installed && newest && (
            <Button disabled={actions.install.isPending} onClick={() => actions.install.mutate({ id })}>
              {installing(undefined) ? "…" : t("plugins.install")}
            </Button>
          )}
          {update && (
            <Button disabled={actions.install.isPending} onClick={() => actions.install.mutate({ id, version: update })}>
              {installing(update) ? "…" : t("plugins.update", { version: update })}
            </Button>
          )}
          {e.installed && (
            <>
              {/* Starting or stopping a process plugin takes a while; the switch waits for it. */}
              <Tip label={t(enabled ? "plugins.disabling" : "plugins.enabling")} disabled={!toggling}>
                <span className="inline-flex">
                  <Switch
                    on={toggling ? !enabled : enabled}
                    label={t("plugins.enabled")}
                    disabled={toggling}
                    onChange={(on) => actions.setEnabled.mutate({ id, enabled: on })}
                  />
                </span>
              </Tip>
              {enabled && (
                <Button variant="secondary" disabled={actions.restart.isPending} onClick={() => actions.restart.mutate(id)}>
                  {actions.restart.isPending ? "…" : t("plugins.restart")}
                </Button>
              )}
              {!dev && (
                <Button variant="secondary" className="text-danger" onClick={() => setConfirming(true)}>
                  {t("plugins.uninstall")}
                </Button>
              )}
            </>
          )}
        </div>
      </header>
      {confirming && (
        <Confirm
          title={t("plugins.uninstallTitle", { name: e.name })}
          message={t("plugins.uninstallMessage")}
          action={t("plugins.uninstall")}
          busy={actions.uninstall.isPending}
          onCancel={() => setConfirming(false)}
          onConfirm={() => actions.uninstall.mutate(id, { onSettled: () => setConfirming(false) })}
        />
      )}
      {failure && <p className="type-callout text-danger">{errorMessage(failure)}</p>}
      {e.description && <p className="type-body">{e.description}</p>}
      {e.installed && m && (m.configSchema || m.configPage) && (
        <Configuration server={server} id={id} schema={m.configSchema} page={m.configPage} />
      )}
      {e.installed && m && m.containers.length > 0 && <Containers server={server} id={id} />}
      {e.installed && m && ((m.runtime === Runtime.WASM && hosts.length > 0) || m.runtime === Runtime.PROCESS) && (
        <PluginProxy server={server} id={id} name={m.name} hosts={m.runtime === Runtime.WASM ? hosts : undefined} proxy={e.installed.proxy} />
      )}
      <Group title={t("plugins.information")}>
        {info.map(([label, value]) => (
          <div key={label} className="flex min-h-11 flex-wrap items-baseline gap-x-6 gap-y-1 px-4 py-2.5">
            <span className="w-40 shrink-0 type-body text-text-secondary">{label}</span>
            <span className="min-w-0 flex-1 type-body">{value}</span>
          </div>
        ))}
      </Group>
      {e.installed && <PluginLogs server={server} id={id} dev={dev} />}
      {!dev && e.offered && e.offered.versions.length > 0 && (
        <Group title={t("plugins.versions")}>
          {e.offered.versions.map((v) => (
            <Row
              key={v.version}
              label={v.version}
              detail={[v.releaseTime && day.format(timestampDate(v.releaseTime)), v.changelog].filter(Boolean).join(" · ")}
            >
              {v.version === m?.version ? (
                <span className="type-callout text-text-secondary">{t("plugins.installed")}</span>
              ) : (
                e.installed && (
                  <Button variant="secondary" disabled={actions.install.isPending} onClick={() => actions.install.mutate({ id, version: v.version })}>
                    {installing(v.version) ? "…" : t("plugins.installThis")}
                  </Button>
                )
              )}
            </Row>
          ))}
        </Group>
      )}
    </div>
  );
}

/** A plugin's recent log records, newest first; under development, with where it runs from. */
function PluginLogs({ server, id, dev }: { server: ServerEntry; id: string; dev: string }) {
  const logs = usePluginLogs(server, id);
  const names = levelNames();
  return (
    <Group title={t("plugins.logs")} footer={dev ? `${t("plugins.logsFooter")} ${t("plugins.devFolderDetail")}` : t("plugins.logsFooter")}>
      <div className="max-h-96 overflow-y-auto">
        {(logs.data ?? []).map((r, i) => (
          <LogRow key={i} record={r} level={names[r.level] ?? ""} />
        ))}
        {logs.isSuccess && logs.data.length === 0 && <p className="px-4 py-3 type-callout text-text-secondary">{t("logs.none")}</p>}
        {logs.isError && <p className="px-4 py-3 type-callout break-words text-danger">{errorMessage(logs.error)}</p>}
      </div>
    </Group>
  );
}

/** The schema of a manifest, undefined when it has none the form can draw. */
/**
 * The proxy a plugin reaches sites through, saved as it is chosen: a WASM
 * plugin's requests to hosts, which the server performs, or a process
 * plugin's, which go to the server's proxy gateway; a process plugin going
 * from direct to proxied or back restarts, once confirmed.
 */
function PluginProxy({ server, id, name, hosts, proxy }: { server: ServerEntry; id: string; name: string; hosts?: string[]; proxy: string }) {
  const { list } = useProxies(server);
  const { setProxy } = usePluginActions(server);
  const [restarting, setRestarting] = useState<string>();
  const value = setProxy.isPending && setProxy.variables ? setProxy.variables.proxy : proxy;
  const save = (p: string) => setProxy.mutate({ id, proxy: p }, { onSettled: () => setRestarting(undefined) });
  return (
    <Group title={t("plugins.network")}>
      <Row label={t("plugins.proxy")} detail={hosts ? t("plugins.proxyDetail", { hosts: hosts.join(", ") }) : t("plugins.proxyProcessDetail")} wrap={!hosts}>
        <Select
          value={value}
          label={t("plugins.proxy")}
          className="w-48"
          disabled={setProxy.isPending}
          options={proxyChoices(list.data, value)}
          onChange={(p) => {
            const was = proxyDirect(list.data, proxy);
            const next = proxyDirect(list.data, p);
            if (!hosts && was !== undefined && next !== undefined && was !== next) setRestarting(p);
            else save(p);
          }}
        />
      </Row>
      {restarting !== undefined && (
        <Confirm
          title={t("plugins.proxyRestartTitle", { name })}
          message={t("plugins.proxyRestartMessage")}
          action={t("plugins.proxyRestart")}
          busy={setProxy.isPending}
          onCancel={() => setRestarting(undefined)}
          onConfirm={() => save(restarting)}
        />
      )}
      {setProxy.isError && <p className="px-4 py-3 type-callout break-words text-danger">{errorMessage(setProxy.error)}</p>}
    </Group>
  );
}

function parseSchema(s: string): Schema | undefined {
  try {
    const schema = JSON.parse(s) as Schema;
    return schema.properties && Object.keys(schema.properties).length > 0 ? schema : undefined;
  } catch {
    return undefined;
  }
}

/** An installed plugin's configuration: the form of its schema, or a link to its own page. */
function Configuration({ server, id, schema, page }: { server: ServerEntry; id: string; schema: string; page: string }) {
  const parsed = parseSchema(schema);
  const config = usePluginConfig(server, id, !!parsed);
  const own = page && `${server.url.replace(/\/$/, "")}/plugins/${encodeURIComponent(id)}/${page}`;
  return (
    <section className="flex flex-col gap-2">
      <div className="flex items-center gap-3 px-1">
        <h2 className="flex-1 type-headline">{t("plugins.configuration")}</h2>
        {own && (
          <a href={own} target="_blank" rel="noreferrer" className="inline-flex items-center gap-1 type-callout text-accent-text">
            {t("plugins.openSettingsPage")}
            <Icon name="external" size={14} />
          </a>
        )}
      </div>
      {parsed && config.isSuccess && <ConfigForm server={server} id={id} schema={parsed} saved={config.data} />}
      {config.isError && <p className="type-callout text-danger">{errorMessage(config.error)}</p>}
    </section>
  );
}

function ConfigForm({ server, id, schema, saved }: { server: ServerEntry; id: string; schema: Schema; saved: string }) {
  const actions = usePluginActions(server);
  const options = usePluginConfigOptions(server, id, optionSources(schema));
  const containers = usePluginContainers(server, id);
  const unset = (name: string) => {
    const c = containers.data?.find((c) => c.dependency?.name === name);
    return !!c && (c.status?.mode ?? ContainerMode.UNSPECIFIED) === ContainerMode.UNSPECIFIED;
  };
  const [value, setValue] = useState<unknown>(() => (saved ? JSON.parse(saved) : {}));
  const [done, setDone] = useState(false);
  const text = JSON.stringify(value);
  // Saving refreshes saved, which the form then matches.
  const changed = text !== JSON.stringify(saved ? JSON.parse(saved) : {});
  const guard = useUnsavedGuard(changed);
  return (
    <form
      className="flex flex-col gap-3"
      onSubmit={(ev) => {
        ev.preventDefault();
        setDone(false);
        // The configuration stored holds the IDs the server generated.
        actions.configure.mutate({ id, config: text }, { onSuccess: (r) => (setValue(JSON.parse(r.configJson)), setDone(true)) });
      }}
    >
      {guard.dialog}
      <SchemaForm schema={schema} value={value} options={options} unset={unset} onChange={(v) => (setValue(v), setDone(false))} />
      <div className="flex flex-wrap items-center gap-3">
        <Button type="submit" disabled={!changed || actions.configure.isPending}>
          {actions.configure.isPending ? "…" : t("action.save")}
        </Button>
        {done && !changed && <span className="type-callout text-text-secondary">{t("plugins.configurationSaved")}</span>}
        {actions.configure.isError && <span className="type-callout text-danger">{errorMessage(actions.configure.error)}</span>}
      </div>
    </form>
  );
}

/** A container dependency's state, with how much of its image is downloaded. */
export function containerState(st: ContainerStatus): string {
  if (st.state === ContainerState.PULLING && st.queued) return t("plugins.containerQueued");
  if (st.state === ContainerState.PULLING && st.progress > 0) return `${containerStates[st.state]()} ${Math.floor(st.progress * 100)}%`;
  return containerStates[st.state]();
}

const containerStates: Record<ContainerState, () => string> = {
  [ContainerState.UNSPECIFIED]: () => t("plugins.containerUnavailable"),
  [ContainerState.PULLING]: () => t("plugins.containerPulling"),
  [ContainerState.STARTING]: () => t("plugins.containerStarting"),
  [ContainerState.READY]: () => t("plugins.containerReady"),
  [ContainerState.UNHEALTHY]: () => t("plugins.containerUnhealthy"),
  [ContainerState.UNAVAILABLE]: () => t("plugins.containerUnavailable"),
};

/** The docker run command a container runs as, shown when asked for. */
export function ContainerCommand({ command }: { command: string }) {
  return <ContainerDisclosure label={t("plugins.command")} text={command} />;
}

/** Text of a container's, such as its command or error, shown when asked for. */
export function ContainerDisclosure({ label, text, danger }: { label: string; text: string; danger?: boolean }) {
  const [shown, setShown] = useState(false);
  return (
    <div className="flex flex-col items-start gap-1.5 px-4 pb-2.5">
      <button
        type="button"
        className="flex items-center gap-1 type-callout text-text-secondary hover:text-on-background"
        aria-expanded={shown}
        onClick={() => setShown(!shown)}
      >
        <Icon name="expand" size={14} className={`shrink-0 transition-transform ${shown ? "" : "-rotate-90"}`} />
        {label}
      </button>
      {shown && (
        <pre
          className={`w-full overflow-x-auto rounded-md bg-fill p-2.5 type-footnote whitespace-pre-wrap break-all select-all ${danger ? "text-danger" : ""}`}
        >
          {text}
        </pre>
      )}
    </div>
  );
}

/** An installed plugin's container dependencies: how each runs, its state, and the commands running it elsewhere. */
function Containers({ server, id }: { server: ServerEntry; id: string }) {
  const list = usePluginContainers(server, id);
  return (
    <section className="flex flex-col gap-2">
      <h2 className="px-1 type-headline">{t("plugins.containers")}</h2>
      {list.data?.map((c) => <ContainerDependency key={c.dependency?.name} server={server} id={id} c={c} />)}
      {list.isError && <p className="type-callout text-danger">{errorMessage(list.error)}</p>}
    </section>
  );
}

function ContainerDependency({ server, id, c }: { server: ServerEntry; id: string; c: PluginContainer }) {
  const actions = usePluginActions(server);
  const proxies = useProxies(server);
  // A GPU is offered only where the engine gives containers one.
  const gpu = useContainerRuntime(server).data?.gpu ?? false;
  const [recreating, setRecreating] = useState<string>();
  const d = c.dependency!;
  const st = c.status;
  const mode = st?.mode ?? ContainerMode.UNSPECIFIED;
  const [address, setAddress] = useState(mode === ContainerMode.EXTERNAL ? (st?.address ?? "") : "");
  const [chosen, setChosen] = useState(mode);
  const set = (m: ContainerMode, extra: { address?: string; useGpu?: boolean; proxy?: string } = {}, onSettled?: () => void) =>
    actions.setContainer.mutate({ pluginId: id, name: d.name, mode: m, useGpu: gpu && c.useGpu, ...extra }, { onSettled });
  const proxy =
    actions.setContainer.isPending && actions.setContainer.variables?.proxy !== undefined ? actions.setContainer.variables.proxy : c.proxy;
  const setProxy = (p: string) => set(ContainerMode.MANAGED, { proxy: p }, () => setRecreating(undefined));
  const state = st && containerState(st);
  const commands = (title: string, cmds: ContainerCommands | undefined) =>
    cmds && (
      <div className="flex flex-col gap-1.5 px-4 py-2.5">
        <span className="type-callout text-text-secondary">{title}</span>
        {/* A server in a container is joined on its network, which the administrator names. */}
        <span className="type-footnote text-text-secondary">
          {cmds.dockerRun.includes("<network>") ? t("plugins.containerDockerRunNetwork", { network: "<network>" }) : t("plugins.containerDockerRun")}
        </span>
        <pre className="overflow-x-auto rounded-md bg-fill p-2.5 type-footnote whitespace-pre-wrap break-all select-all">{cmds.dockerRun}</pre>
        <span className="type-footnote text-text-secondary">
          {t(cmds.dockerRun.includes("<network>") ? "plugins.containerComposeServer" : "plugins.containerCompose")}
        </span>
        <pre className="overflow-x-auto rounded-md bg-fill p-2.5 type-footnote select-all">{cmds.compose}</pre>
      </div>
    );
  return (
    <Group>
      <Row
        label={
          <span className="flex items-center gap-2">
            {d.name}
            <span className="rounded-full bg-fill px-2 py-0.5 type-footnote text-text-secondary">
              {t(d.required ? "plugins.containerRequired" : "plugins.containerOptional")}
            </span>
          </span>
        }
        detail={[d.description, state].filter(Boolean).join(" · ")}
        wrap
      >
        <Select
          value={String(chosen)}
          label={t("plugins.containerMode")}
          disabled={actions.setContainer.isPending}
          options={[
            { value: String(ContainerMode.UNSPECIFIED), label: t("plugins.containerUnset") },
            { value: String(ContainerMode.MANAGED), label: t("plugins.containerManaged") },
            { value: String(ContainerMode.EXTERNAL), label: t("plugins.containerExternal") },
          ]}
          onChange={(v) => {
            const m = Number(v) as ContainerMode;
            setChosen(m);
            if (m !== ContainerMode.EXTERNAL) set(m, { useGpu: m === ContainerMode.MANAGED && (d.gpu === GpuSupport.REQUIRED || (gpu && c.useGpu)) });
          }}
        />
      </Row>
      {chosen === ContainerMode.MANAGED && d.gpu !== GpuSupport.UNSPECIFIED && gpu && (
        <Row label={t("plugins.containerUseGpu")}>
          <Switch
            on={c.useGpu}
            label={t("plugins.containerUseGpu")}
            disabled={actions.setContainer.isPending || d.gpu === GpuSupport.REQUIRED}
            onChange={(on) => set(ContainerMode.MANAGED, { useGpu: on })}
          />
        </Row>
      )}
      {chosen === ContainerMode.MANAGED && mode === ContainerMode.MANAGED && (
        <>
          {/* A container given the proxy of each request reaches sites directly. */}
          {!d.direct && (
            <Row label={t("plugins.proxy")} detail={t("plugins.containerProxyDetail")} wrap>
              <Select
                value={proxy}
                label={t("plugins.proxy")}
                className="w-48"
                disabled={actions.setContainer.isPending}
                options={proxyChoices(proxies.list.data, proxy)}
                onChange={(p) => {
                  const was = proxyDirect(proxies.list.data, c.proxy);
                  const next = proxyDirect(proxies.list.data, p);
                  if (was !== undefined && next !== undefined && was !== next) setRecreating(p);
                  else setProxy(p);
                }}
              />
            </Row>
          )}
          {/* Apart from the divider above it, as it is not below a row of its own. */}
          {c.command && (
            <div className="pt-2.5">
              <ContainerCommand command={c.command} />
            </div>
          )}
        </>
      )}
      {st?.error && (
        <div className="pt-2.5">
          <ContainerDisclosure label={t("plugins.containerError")} text={st.error} danger />
        </div>
      )}
      {recreating !== undefined && (
        <Confirm
          title={t("plugins.containerProxyTitle", { name: d.name })}
          message={t("plugins.containerProxyMessage")}
          action={t("plugins.containerRecreate")}
          busy={actions.setContainer.isPending}
          onCancel={() => setRecreating(undefined)}
          onConfirm={() => setProxy(recreating)}
        />
      )}
      {chosen === ContainerMode.EXTERNAL && (
        <>
          <form
            className="flex min-h-13 items-center gap-3 px-4 py-2.5"
            onSubmit={(ev) => {
              ev.preventDefault();
              set(ContainerMode.EXTERNAL, { address: address.trim(), useGpu: false });
            }}
          >
            <input
              type="url"
              value={address}
              onChange={(ev) => setAddress(ev.target.value)}
              placeholder={c.commands?.address ?? c.gpuCommands?.address ?? t("plugins.containerAddress")}
              aria-label={t("plugins.containerAddress")}
              className="min-w-0 flex-1 bg-transparent type-body outline-none placeholder:text-text-tertiary"
            />
            <Button
              type="submit"
              variant="secondary"
              disabled={!address.trim() || (mode === ContainerMode.EXTERNAL && address.trim().replace(/\/$/, "") === st?.address) || actions.setContainer.isPending}
            >
              {t("action.save")}
            </Button>
          </form>
          <p className="px-4 py-2.5 type-footnote text-text-secondary">
            {t("plugins.containerCommandsHint", { address: c.commands?.address ?? c.gpuCommands?.address ?? "" })}
          </p>
          {commands(t("plugins.containerCommands"), c.commands)}
          {commands(t("plugins.containerGpuCommands"), c.gpuCommands)}
        </>
      )}
      {actions.setContainer.isError && <p className="px-4 py-2.5 type-callout text-danger">{errorMessage(actions.setContainer.error)}</p>}
    </Group>
  );
}

/** The catalogs' URLs, to add to and remove from. */
function CatalogsPage({ server }: { server: ServerEntry }) {
  const catalogs = usePluginCatalogs(server);
  const actions = usePluginActions(server);
  const [url, setUrl] = useState("");
  const list = catalogs.data ?? [];
  const add = (e: FormEvent) => {
    e.preventDefault();
    const u = url.trim();
    if (!u || list.includes(u)) return;
    actions.setCatalogs.mutate([...list, u], { onSuccess: () => setUrl("") });
  };
  return (
    <div className="flex flex-col gap-7">
      <BackToList />
      <header>
        <h2 className="type-title1">{t("plugins.catalogs")}</h2>
        <p className="mt-1 type-body text-text-secondary">{t("plugins.catalogsMessage")}</p>
      </header>
      <Group>
        {list.map((c) => (
          <Row key={c} label={<span className="break-all">{c}</span>}>
            <Button
              variant="plain"
              className="text-danger"
              disabled={actions.setCatalogs.isPending}
              onClick={() => actions.setCatalogs.mutate(list.filter((o) => o !== c))}
            >
              {t("plugins.removeCatalog")}
            </Button>
          </Row>
        ))}
        <form onSubmit={add} className="flex min-h-13 items-center gap-3 px-4 py-2.5">
          <input
            type="url"
            value={url}
            onChange={(e) => setUrl(e.target.value)}
            placeholder={t("plugins.catalogAddress")}
            aria-label={t("plugins.catalogAddress")}
            className="min-w-0 flex-1 bg-transparent type-body outline-none placeholder:text-text-tertiary"
          />
          <Button type="submit" variant="secondary" disabled={!url.trim() || actions.setCatalogs.isPending}>
            {t("plugins.addCatalog")}
          </Button>
        </form>
      </Group>
      {actions.setCatalogs.isError && <p className="type-callout text-danger">{errorMessage(actions.setCatalogs.error)}</p>}
    </div>
  );
}
