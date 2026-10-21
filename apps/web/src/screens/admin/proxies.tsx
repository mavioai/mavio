// The settings' Proxies (docs/clients.md §6.3.30): the server's proxies,
// which the server and its plugins reach sites through, added, edited,
// tested and deleted here; the system proxy first, read only; and the
// proxies of the server's own requests.
import { create } from "@bufbuild/protobuf";
import { errorMessage, type ServerEntry, systemProxyId, useOutboundProxies, useProxies, useTestProxy } from "@mavio/client-ts";
import { OutboundProxiesSchema, type Proxy as ProxyMessage, ProxyKind, ProxySchema, type TestProxyResponse } from "@mavio/proto/mavio/system/v1/proxy_pb";
import { type ReactNode, useState } from "react";

import { locale, t } from "../../i18n.ts";
import { Button, Group, Row } from "../../ui.tsx";
import { NumberField, Switch } from "../../controls/inputs.tsx";
import { Select } from "../../controls/select.tsx";
import { input } from "../../controls/styles.ts";
import { Confirm } from "../../controls/dialog.tsx";
import { AddRow, ColorTile, Page } from "./common.tsx";

/** The kinds of proxies, by the scheme of their addresses. */
const kinds: { value: string; kind: ProxyKind; label: string }[] = [
  { value: "http", kind: ProxyKind.HTTP, label: "HTTP" },
  { value: "socks4", kind: ProxyKind.SOCKS4, label: "SOCKS4" },
  { value: "socks5", kind: ProxyKind.SOCKS5, label: "SOCKS5" },
];

/** A proxy's address, kind://host:port. */
function addressOf(p: ProxyMessage): string {
  const scheme = kinds.find((k) => k.kind === p.kind)?.value ?? "";
  const host = p.host.includes(":") ? `[${p.host}]` : p.host;
  return `${scheme}://${host}:${p.port}`;
}

/** The section of the server chosen in the settings. */
export function ProxiesSection({ server }: { server: ServerEntry }) {
  const { list } = useProxies(server);
  // The proxy edited, by its ID; "" for a new one.
  const [editing, setEditing] = useState<string>();
  const system = list.data?.find((p) => p.system);
  const own = list.data?.filter((p) => !p.system) ?? [];
  return (
    <Page
      title={t("settings.proxies")}
      message={t("proxies.message")}
    >
      {system && (
        <Group footer={t("proxies.systemFooter")}>
          <SystemRow server={server} proxy={system} />
        </Group>
      )}
      <Group title={t("proxies.list")}>
        {editing === "" && <ProxyForm server={server} onDone={() => setEditing(undefined)} />}
        {own.map((p) =>
          editing === p.id ? (
            <ProxyForm key={p.id} server={server} proxy={p} onDone={() => setEditing(undefined)} />
          ) : (
            <ProxyRow key={p.id} server={server} proxy={p} disabled={editing !== undefined} onEdit={() => setEditing(p.id)} />
          ),
        )}
        {list.isSuccess && own.length === 0 && editing !== "" && <p className="px-4 py-3 type-callout text-text-secondary">{t("proxies.empty")}</p>}
        {list.isError && <p className="px-4 py-3 type-callout break-words text-danger">{errorMessage(list.error)}</p>}
        {editing === undefined && list.isSuccess && <AddRow label={t("proxies.add")} onClick={() => setEditing("")} />}
      </Group>
      <ServerRequests server={server} />
    </Page>
  );
}

/** The choices of a proxy: none, the system proxy, then the server's by name; one chosen but gone by its ID. */
export function proxyChoices(list: ProxyMessage[] | undefined, current: string): { value: string; label: string }[] {
  const collator = new Intl.Collator(locale, { sensitivity: "base", numeric: true });
  const own = (list ?? []).filter((p) => !p.system).toSorted((a, b) => collator.compare(a.name, b.name));
  const out = [
    { value: "", label: t("proxies.none") },
    { value: systemProxyId, label: t("proxies.system") },
    ...own.map((p) => ({ value: p.id, label: p.name })),
  ];
  if (list && !out.some((o) => o.value === current)) out.push({ value: current, label: t("proxies.unknown") });
  return out;
}

/**
 * Whether a proxy choice reaches sites directly: none, or the system proxy
 * while the system sets none. Programs the server starts go from one to
 * the other only by starting again; unknown before the list is read.
 */
export function proxyDirect(list: ProxyMessage[] | undefined, id: string): boolean | undefined {
  if (id === "") return true;
  if (id !== systemProxyId) return false;
  const system = list?.find((p) => p.system);
  return system && system.kind === ProxyKind.UNSPECIFIED;
}

/** What the server's own requests reach, as OutboundProxies names it. */
type RequestKey = "pluginCatalogs" | "images" | "mavioDev" | "acme";

/** What the server's own requests reach, each with its proxy; mavio.dev and its certificates only in mavio.dev mode. */
const requests: { key: RequestKey; label: () => string; detail: () => string; mavioDev?: boolean }[] = [
  { key: "pluginCatalogs", label: () => t("proxies.forCatalogs"), detail: () => t("proxies.forCatalogsDetail") },
  { key: "images", label: () => t("proxies.forImages"), detail: () => t("proxies.forImagesDetail") },
  { key: "mavioDev", label: () => t("proxies.forMavioDev"), detail: () => t("proxies.forMavioDevDetail"), mavioDev: true },
  { key: "acme", label: () => t("proxies.forAcme"), detail: () => t("proxies.forAcmeDetail"), mavioDev: true },
];

/** The proxies of the server's own requests, each saved as it is chosen. */
function ServerRequests({ server }: { server: ServerEntry }) {
  const { list } = useProxies(server);
  const { get: settings, update } = useOutboundProxies(server);
  // The choice being saved, shown until the settings come back.
  const [choosing, setChoosing] = useState<{ key: RequestKey; id: string }>();
  const current = settings.data;
  if (!current) return null;
  const choose = (key: RequestKey, id: string) => {
    setChoosing({ key, id });
    update.mutate(create(OutboundProxiesSchema, { ...current, [key]: id }), { onSettled: () => setChoosing(undefined) });
  };
  return (
    <Group title={t("proxies.serverRequests")} footer={t("proxies.serverRequestsFooter")}>
      {requests
        .filter((r) => !r.mavioDev || server.mode === "mavioDev")
        .map((r) => {
          const value = choosing?.key === r.key ? choosing.id : current[r.key];
          return (
            <Row key={r.key} label={r.label()} detail={r.detail()}>
              <Select
                value={value}
                label={r.label()}
                className="w-48"
                disabled={update.isPending}
                options={proxyChoices(list.data, value)}
                onChange={(id) => choose(r.key, id)}
              />
            </Row>
          );
        })}
      {update.isError && <p className="px-4 py-3 type-callout break-words text-danger">{errorMessage(update.error)}</p>}
    </Group>
  );
}

/** The system proxy: what the system sets, with the hosts it reaches directly, or that it sets none. */
function SystemRow({ server, proxy }: { server: ServerEntry; proxy: ProxyMessage }) {
  const set = proxy.kind !== ProxyKind.UNSPECIFIED;
  return (
    <ProxyLine
      server={server}
      target={{ id: proxy.id }}
      tile={<ColorTile icon="network" color={set ? "#007AFF" : "#8E8E93"} />}
      name={t("proxies.system")}
      detail={set ? [addressOf(proxy), proxy.bypass.length ? t("proxies.bypass", { hosts: proxy.bypass.join(", ") }) : undefined].filter(Boolean).join(" · ") : t("proxies.systemUnset")}
    />
  );
}

/** A proxy of the server's own, edited when pressed. */
function ProxyRow({ server, proxy, disabled, onEdit }: { server: ServerEntry; proxy: ProxyMessage; disabled: boolean; onEdit: () => void }) {
  return (
    <ProxyLine
      server={server}
      target={{ id: proxy.id }}
      tile={<ColorTile icon="network" color="#30B0C7" />}
      name={proxy.name}
      detail={[addressOf(proxy), proxy.username].filter(Boolean).join(" · ")}
      disabled={disabled}
      onEdit={onEdit}
    />
  );
}

/** A proxy's line: its name and address, editing it when it may be, and testing it, with the result under it. */
function ProxyLine({
  server,
  target,
  tile,
  name,
  detail,
  disabled,
  onEdit,
}: {
  server: ServerEntry;
  target: { id: string };
  tile: ReactNode;
  name: string;
  detail: string;
  disabled?: boolean;
  onEdit?: () => void;
}) {
  const test = useTestProxy(server);
  const body = (
    <>
      {tile}
      <div className="min-w-0 flex-1">
        <div className="truncate type-body">{name}</div>
        <div className="type-footnote break-words text-text-secondary">{detail}</div>
      </div>
    </>
  );
  return (
    // The whole entry, its test and result with it, shows it is pressable.
    <div className={`flex flex-col ${onEdit && !disabled ? "hover:tint" : ""}`}>
      <div className="flex min-h-13 items-center gap-3 px-4 py-2.5">
        {onEdit ? (
          <button type="button" disabled={disabled} onClick={onEdit} className="-my-2.5 -ml-4 flex min-w-0 flex-1 items-center gap-3 py-2.5 pl-4 text-left">
            {body}
          </button>
        ) : (
          body
        )}
        <Button variant="secondary" disabled={test.isPending} onClick={() => test.mutate(target)}>
          {test.isPending ? "…" : t("proxies.test")}
        </Button>
      </div>
      <TestResult result={test.data} error={test.isError ? errorMessage(test.error) : undefined} />
    </div>
  );
}

const regions = (() => {
  try {
    return new Intl.DisplayNames([locale], { type: "region" });
  } catch {
    return undefined;
  }
})();

/** A test's result: how long the site took through the proxy, the exit address and its country, or why it failed. */
function TestResult({ result, error }: { result?: TestProxyResponse; error?: string }) {
  if (error) return <p className="px-4 pb-2.5 type-footnote break-words text-danger">{t("proxies.testFailed", { error })}</p>;
  if (!result) return null;
  if (!result.ok) return <p className="px-4 pb-2.5 type-footnote break-words text-danger">{t("proxies.testFailed", { error: result.error })}</p>;
  const ms = result.latency ? Math.round(Number(result.latency.seconds) * 1000 + result.latency.nanos / 1e6) : 0;
  let country = result.country;
  if (/^[A-Z]{2}$/.test(country)) country = regions?.of(country) ?? country;
  return (
    <p className="px-4 pb-2.5 type-footnote text-success">
      {[t("proxies.testOk", { ms }), result.exitIp && t("proxies.exit", { address: result.exitIp }), country].filter(Boolean).join(" · ")}
    </p>
  );
}

/** What a proxy's form holds. */
interface Draft {
  name: string;
  kind: string;
  host: string;
  port: number;
  username: string;
  password: string;
  clearPassword: boolean;
}

/** The form adding a proxy, or editing one and deleting it. */
function ProxyForm({ server, proxy, onDone }: { server: ServerEntry; proxy?: ProxyMessage; onDone: () => void }) {
  const { create: add, update, remove } = useProxies(server);
  const test = useTestProxy(server);
  const [d, setD] = useState<Draft>({
    name: proxy?.name ?? "",
    kind: kinds.find((k) => k.kind === proxy?.kind)?.value ?? "http",
    host: proxy?.host ?? "",
    port: proxy?.port ?? 8080,
    username: proxy?.username ?? "",
    password: "",
    clearPassword: false,
  });
  const [removing, setRemoving] = useState(false);
  const set = (patch: Partial<Draft>) => setD((x) => ({ ...x, ...patch }));
  const message = () =>
    create(ProxySchema, {
      id: proxy?.id ?? "",
      name: d.name.trim(),
      kind: kinds.find((k) => k.value === d.kind)?.kind ?? ProxyKind.HTTP,
      host: d.host.trim(),
      port: d.port,
      username: d.username.trim(),
      password: d.password,
    });
  const save = proxy ? update : add;
  const busy = save.isPending || remove.isPending;
  const valid = d.name.trim() !== "" && d.host.trim() !== "" && d.port >= 1 && d.port <= 65535;
  const submit = () => {
    const done = { onSuccess: onDone };
    if (proxy) update.mutate({ proxy: message(), clearPassword: d.clearPassword && d.password === "" }, done);
    else add.mutate(message(), done);
  };
  const failure = save.isError ? errorMessage(save.error) : undefined;
  const text = (key: "name" | "host" | "username", label: string, placeholder?: string) => (
    <Row label={label}>
      <input className={`${input} sm:w-72`} aria-label={label} placeholder={placeholder} value={d[key]} onChange={(e) => set({ [key]: e.target.value })} />
    </Row>
  );
  if (removing && proxy)
    return (
      <Confirm
        title={t("proxies.removeTitle", { name: proxy.name })}
        message={remove.isError ? errorMessage(remove.error) : t("proxies.removeMessage")}
        action={t("proxies.remove")}
        busy={remove.isPending}
        onCancel={() => setRemoving(false)}
        onConfirm={() => remove.mutate(proxy.id, { onSuccess: onDone })}
      />
    );
  return (
    <div className="flex flex-col divide-y divide-border bg-background/40">
      {text("name", t("proxies.name"))}
      <Row label={t("proxies.kind")}>
        <Select value={d.kind} label={t("proxies.kind")} options={kinds.map((k) => ({ value: k.value, label: k.label }))} onChange={(kind) => set({ kind })} />
      </Row>
      {text("host", t("proxies.host"), "proxy.lan")}
      <Row label={t("proxies.port")}>
        <NumberField min={1} max={65535} className="w-32" label={t("proxies.port")} value={d.port} onChange={(v) => set({ port: v ?? 0 })} />
      </Row>
      {text("username", t("proxies.username"))}
      <Row label={t("proxies.password")}>
        <input
          type="password"
          autoComplete="new-password"
          className={`${input} sm:w-72`}
          aria-label={t("proxies.password")}
          placeholder={proxy?.hasPassword && !d.clearPassword ? t("proxies.passwordKept") : undefined}
          value={d.password}
          onChange={(e) => set({ password: e.target.value })}
        />
      </Row>
      {proxy?.hasPassword && d.password === "" && (
        <Row label={t("proxies.clearPassword")}>
          <Switch on={d.clearPassword} label={t("proxies.clearPassword")} onChange={(clearPassword) => set({ clearPassword })} />
        </Row>
      )}
      <div className="flex flex-wrap items-center gap-3 px-4 py-2.5">
        <Button disabled={!valid || busy} onClick={submit}>
          {save.isPending ? "…" : t("action.save")}
        </Button>
        <Button variant="secondary" disabled={!valid || test.isPending} onClick={() => test.mutate({ proxy: message() })}>
          {test.isPending ? "…" : t("proxies.test")}
        </Button>
        <Button variant="plain" onClick={onDone}>
          {t("action.cancel")}
        </Button>
        {proxy && (
          <Button variant="plain" className="text-danger" onClick={() => (remove.reset(), setRemoving(true))}>
            {t("proxies.remove")}
          </Button>
        )}
        {failure && <span className="type-callout break-words text-danger">{failure}</span>}
      </div>
      <TestResult result={test.data} error={test.isError ? errorMessage(test.error) : undefined} />
    </div>
  );
}
