// The settings' General (docs/clients.md §6.3.8): the server's name, the
// language and country of its metadata, how long devices stay signed in,
// the plugin that resets forgotten passwords, and how it is reached: its
// base URL, HTTPS, discovery on the local network and, in mavio.dev mode,
// the relay.
import { create } from "@bufbuild/protobuf";
import { Capability } from "@mavio/proto/mavio/plugin/v1/plugin_pb";
import { MetadataSettingsSchema, NetworkSettingsSchema, type ServerSettings } from "@mavio/proto/mavio/system/v1/system_pb";
import {
  errorMessage,
  type ServerEntry,
  useLocalization,
  usePlugins,
  useServerSettings,
  useServersStore,
  useUpdateServerSettings,
} from "@mavio/client-ts";
import { useState } from "react";

import { t } from "../../i18n.ts";
import { Button, Group, Row, Select, Toggle } from "../../ui.tsx";
import { countryOptions, input, languageOptions, Page } from "./common.tsx";

/** The section of the server chosen in the settings. */
export function GeneralSection({ server }: { server: ServerEntry }) {
  const settings = useServerSettings(server);
  const s = settings.data?.settings;
  return (
    <Page title={t("settings.general")}>
      {settings.isError && <p className="type-callout text-danger">{errorMessage(settings.error)}</p>}
      {s && <GeneralForm server={server} settings={s} />}
    </Page>
  );
}

/** The settings as edited. */
interface Draft {
  name: string;
  language: string;
  country: string;
  idleDays: number;
  resetPlugin: string;
  baseUrl: string;
  httpsPort: number;
  certificate: string;
  key: string;
  discovery: boolean;
  relay: boolean;
}

function draftOf(s: ServerSettings): Draft {
  const n = s.network;
  return {
    name: n?.serverName ?? "",
    language: s.metadata?.preferredLanguage ?? "",
    country: s.metadata?.country ?? "",
    idleDays: s.sessionIdleDays,
    resetPlugin: s.passwordResetPlugin,
    baseUrl: n?.baseUrl ?? "",
    httpsPort: n?.httpsPort ?? 0,
    certificate: n?.certificatePath ?? "",
    key: n?.keyPath ?? "",
    discovery: n?.localDiscovery ?? false,
    relay: n?.relay ?? false,
  };
}

function GeneralForm({ server, settings }: { server: ServerEntry; settings: ServerSettings }) {
  const servers = useServersStore();
  const update = useUpdateServerSettings(server);
  const { languages, countries } = useLocalization(server);
  const plugins = usePlugins(server);
  const [draft, setDraft] = useState(() => draftOf(settings));
  const [notice, setNotice] = useState<string>();
  const set = (patch: Partial<Draft>) => (setDraft((d) => ({ ...d, ...patch })), setNotice(undefined));
  const changed = JSON.stringify(draft) !== JSON.stringify(draftOf(settings));
  const mavioDev = server.mode === "mavioDev";
  const resetters = (plugins.data ?? []).filter((p) => p.manifest?.capabilities.includes(Capability.PASSWORD_RESET));
  const save = () =>
    update.mutate(
      (s) => {
        const n = (s.network ??= create(NetworkSettingsSchema));
        n.serverName = draft.name.trim();
        n.baseUrl = draft.baseUrl.trim();
        n.httpsPort = draft.httpsPort;
        n.certificatePath = draft.certificate.trim();
        n.keyPath = draft.key.trim();
        n.localDiscovery = draft.discovery;
        n.relay = draft.relay;
        s.metadata = create(MetadataSettingsSchema, { preferredLanguage: draft.language, country: draft.country });
        s.sessionIdleDays = draft.idleDays;
        s.passwordResetPlugin = draft.resetPlugin;
      },
      {
        onSuccess: async () => {
          setNotice(t("general.saved"));
          // An empty name is the host's, which only the server knows.
          const info = await servers.probe(server.url).catch(() => undefined);
          if (info?.name) servers.rename(server.id, info.name);
        },
      },
    );
  const text = (key: "name" | "baseUrl" | "certificate" | "key", label: string, detail?: string, placeholder?: string) => (
    <Row label={label} detail={detail}>
      <input
        className={`${input} sm:w-72`}
        aria-label={label}
        placeholder={placeholder}
        value={draft[key]}
        onChange={(e) => set({ [key]: e.target.value })}
      />
    </Row>
  );
  return (
    <>
      <Group>
        {text("name", t("general.name"), t("general.nameDetail"))}
        <Row label={t("general.language")} detail={t("general.languageDetail")}>
          <Select
            value={draft.language}
            label={t("general.language")}
            className="w-48"
            options={[{ value: "", label: t("general.languageDefault") }, ...languageOptions(languages)]}
            onChange={(language) => set({ language })}
          />
        </Row>
        <Row label={t("general.country")} detail={t("general.countryDetail")}>
          <Select
            value={draft.country}
            label={t("general.country")}
            className="w-48"
            options={[{ value: "", label: t("general.countryDefault") }, ...countryOptions(countries)]}
            onChange={(country) => set({ country })}
          />
        </Row>
      </Group>
      <Group title={t("general.signIn")}>
        <Row label={t("general.idleDays")} detail={t("general.idleDaysDetail")}>
          <input
            type="number"
            min={1}
            max={3650}
            className={`${input} w-24`}
            aria-label={t("general.idleDays")}
            value={draft.idleDays}
            onChange={(e) => set({ idleDays: Math.max(1, Math.min(3650, Math.trunc(Number(e.target.value) || 1))) })}
          />
        </Row>
        {!mavioDev && (
          <Row label={t("general.resetPlugin")} detail={resetters.length ? t("general.resetPluginDetail") : t("general.resetPluginNone")}>
            <Select
              value={draft.resetPlugin}
              label={t("general.resetPlugin")}
              options={[
                { value: "", label: t("general.resetOff") },
                ...resetters.map((p) => ({ value: p.manifest!.id, label: p.manifest!.name || p.manifest!.id })),
                ...(draft.resetPlugin && !resetters.some((p) => p.manifest?.id === draft.resetPlugin)
                  ? [{ value: draft.resetPlugin, label: draft.resetPlugin }]
                  : []),
              ]}
              onChange={(resetPlugin) => set({ resetPlugin })}
            />
          </Row>
        )}
      </Group>
      <Group title={t("general.network")} footer={t("general.networkFooter")}>
        {text("baseUrl", t("general.baseUrl"), t("general.baseUrlDetail"), "/mavio")}
        <Row label={t("general.httpsPort")} detail={mavioDev ? t("general.httpsPortMavioDev") : t("general.httpsPortDetail")}>
          <input
            type="number"
            min={0}
            max={65535}
            className={`${input} w-28`}
            aria-label={t("general.httpsPort")}
            value={draft.httpsPort}
            onChange={(e) => set({ httpsPort: Math.max(0, Math.min(65535, Math.trunc(Number(e.target.value) || 0))) })}
          />
        </Row>
        {draft.httpsPort > 0 && text("certificate", t("general.certificate"), t("general.certificateDetail"), "/etc/mavio/cert.pem")}
        {draft.httpsPort > 0 && text("key", t("general.key"), undefined, "/etc/mavio/key.pem")}
        <Row label={t("general.discovery")} detail={t("general.discoveryDetail")}>
          <Toggle on={draft.discovery} label={t("general.discovery")} onChange={(discovery) => set({ discovery })} />
        </Row>
        {mavioDev && (
          <Row label={t("general.relay")} detail={t("general.relayDetail")}>
            <Toggle on={draft.relay} label={t("general.relay")} onChange={(relay) => set({ relay })} />
          </Row>
        )}
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
