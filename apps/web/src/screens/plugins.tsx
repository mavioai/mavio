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
  usePluginActions,
  usePluginCatalogs,
  usePluginConfig,
  usePlugins,
} from "@mavio/client-ts";
import { Category } from "@mavio/proto/mavio/plugin/v1/plugin_pb";
import { type CatalogPlugin, type Plugin, PluginState } from "@mavio/proto/mavio/system/v1/plugin_pb";
import { Link, useNavigate, useSearch } from "@tanstack/react-router";
import { type FormEvent, type ReactNode, useState } from "react";

import { locale, t } from "../i18n.ts";
import { Icon, type IconName } from "../icon.tsx";
import { up } from "../shell/up.ts";
import { type Schema, SchemaForm } from "../schema-form.tsx";
import { Button, Group, Row } from "../ui.tsx";
import { Switch } from "../controls/inputs.tsx";
import { Select } from "../controls/select.tsx";
import type { SettingsSearch } from "./settings.tsx";
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
      <header>
        <h2 className="type-title1">{t("settings.plugins")}</h2>
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
            className="inline-flex min-h-9 items-center rounded-md bg-fill px-3 type-callout font-semibold hover:bg-fill-selected"
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
      {mark(t("plugins.installed"), "text-text-secondary")}
      {mark(states[state](), tone)}
      {updateOf(e) && mark(t("plugins.updateAvailable"), "text-accent-text")}
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

/** A link back to the list of plugins, as it was left. */
function BackToList() {
  const cur = useSearch({ strict: false }) as SettingsSearch;
  return (
    <Link
      to="/settings/$section"
      params={{ section: "plugins" }}
      search={{ server: cur.server, tab: cur.tab, category: cur.category }}
      {...up}
      className="inline-flex items-center gap-1 self-start type-body text-accent-text"
    >
      <Icon name="back" size={18} />
      {t("settings.plugins")}
    </Link>
  );
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
  const update = updateOf(e);
  const enabled = e.installed?.state !== PluginState.DISABLED;
  const toggling = actions.setEnabled.isPending;
  const failure = [actions.install, actions.uninstall, actions.setEnabled].find((x) => x.isError)?.error;
  const installing = (version?: string) =>
    actions.install.isPending && actions.install.variables?.id === id && actions.install.variables.version === version;
  const info: [string, ReactNode][] = [];
  if (e.installed) {
    const state = states[e.installed.state]();
    info.push([t("plugins.state"), e.installed.state === PluginState.FAILED && e.installed.error ? `${state}: ${e.installed.error}` : state]);
    if (m?.version) info.push([t("plugins.version"), m.version]);
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
        <div className="min-w-0 flex-1">
          <h2 className="type-title1">{e.name}</h2>
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
              <Switch
                on={toggling ? !enabled : enabled}
                label={t("plugins.enabled")}
                onChange={(on) => !toggling && actions.setEnabled.mutate({ id, enabled: on })}
              />
              <Button variant="secondary" className="text-danger" onClick={() => setConfirming(true)}>
                {t("plugins.uninstall")}
              </Button>
            </>
          )}
        </div>
      </header>
      {confirming && (
        <Group>
          <Row label={t("plugins.uninstallTitle", { name: e.name })} detail={t("plugins.uninstallMessage")}>
            <Button variant="plain" onClick={() => setConfirming(false)}>
              {t("action.cancel")}
            </Button>
            <Button
              variant="secondary"
              className="text-danger"
              disabled={actions.uninstall.isPending}
              onClick={() => actions.uninstall.mutate(id, { onSettled: () => setConfirming(false) })}
            >
              {t("plugins.uninstall")}
            </Button>
          </Row>
        </Group>
      )}
      {failure && <p className="type-callout text-danger">{errorMessage(failure)}</p>}
      {e.description && <p className="type-body">{e.description}</p>}
      {e.installed && m && (m.configSchema || m.configPage) && (
        <Configuration server={server} id={id} schema={m.configSchema} page={m.configPage} />
      )}
      <Group title={t("plugins.information")}>
        {info.map(([label, value]) => (
          <div key={label} className="flex min-h-11 flex-wrap items-baseline gap-x-6 gap-y-1 px-4 py-2.5">
            <span className="w-40 shrink-0 type-body text-text-secondary">{label}</span>
            <span className="min-w-0 flex-1 type-body">{value}</span>
          </div>
        ))}
      </Group>
      {e.offered && e.offered.versions.length > 0 && (
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

/** The schema of a manifest, undefined when it has none the form can draw. */
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
        actions.configure.mutate({ id, config: text }, { onSuccess: () => setDone(true) });
      }}
    >
      {guard.dialog}
      <SchemaForm schema={schema} value={value} onChange={(v) => (setValue(v), setDone(false))} />
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
