// The settings (docs/clients.md §6.3): those of this browser, its
// appearance, TV mode and playback, and those of the account and its
// servers, the user's languages, the sign-ins kept, the servers in their
// order, the account's credentials and the devices signed in as the user.
// Wide windows show them beside their own sidebar; phones list them first,
// as the profile.
import { timestampDate } from "@bufbuild/protobuf/wkt";
import {
  errorMessage,
  type ServerEntry,
  serverKey,
  useServers,
  useServersStore,
  useSignedIn,
  useSignIns,
} from "@mavio/client-ts";
import { AuthService } from "@mavio/proto/mavio/auth/v1/auth_pb";
import { SubtitleMode, UserService } from "@mavio/proto/mavio/user/v1/user_pb";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Link, Navigate, useNavigate, useParams } from "@tanstack/react-router";
import { type ReactNode, useState } from "react";

import { locale, t } from "../i18n.ts";
import { Icon, type IconName } from "../icon.tsx";
import { type Appearance, type Quality, setPrefs, usePrefs, useWidthClass } from "../prefs.ts";
import { PinPad, useAccount, useMavioDevAccountUrl, useSwitchUsers } from "../shell/user-menu.tsx";
import { Avatar, Button, Field, Group, Row, Select, Toggle } from "../ui.tsx";
import { Security, SwitchUser } from "./security.tsx";

type SectionId = "appearance" | "playback" | "subtitles-audio" | "sign-ins" | "servers" | "account" | "devices";

const sections: Record<SectionId, { icon: IconName; color: string; title: () => string }> = {
  appearance: { icon: "appearance", color: "#5E5CE6", title: () => t("settings.appearance") },
  playback: { icon: "play", color: "#FF3B30", title: () => t("settings.playback") },
  "subtitles-audio": { icon: "subtitles", color: "#34C759", title: () => t("settings.subtitlesAudio") },
  "sign-ins": { icon: "key", color: "#007AFF", title: () => t("auth.signIns") },
  servers: { icon: "server", color: "#8E8E93", title: () => t("auth.servers") },
  account: { icon: "profile", color: "#FF9500", title: () => t("settings.account") },
  devices: { icon: "devices", color: "#32ADE6", title: () => t("settings.devices") },
};

const groups: { title: () => string; ids: SectionId[] }[] = [
  { title: () => t("settings.thisDevice"), ids: ["appearance", "playback", "subtitles-audio"] },
  { title: () => t("settings.accountAndServers"), ids: ["sign-ins", "servers", "account", "devices"] },
];

function isSection(s: string | undefined): s is SectionId {
  return s !== undefined && s in sections;
}

export function SettingsScreen() {
  const { section } = useParams({ strict: false }) as { section?: string };
  const width = useWidthClass();
  const prefs = usePrefs();
  if (section !== undefined && !isSection(section)) return <Navigate to="/settings" />;
  if (width === "compact" && !prefs.tv) {
    return section ? <CompactSection id={section} /> : <ProfileList />;
  }
  const id = section ?? "appearance";
  return (
    <div className="flex min-h-screen">
      <aside className="sticky top-0 flex h-screen w-[300px] shrink-0 flex-col gap-4 overflow-y-auto border-r border-border bg-sidebar px-4 py-5">
        <Link to="/" className="inline-flex items-center gap-1 self-start type-body text-accent-text">
          <Icon name="back" size={18} />
          {t("action.back")}
        </Link>
        <h1 className="px-1 type-title1">{t("nav.settings")}</h1>
        <AccountCard />
        {groups.map((g) => (
          <nav key={g.title()} aria-label={g.title()} className="flex flex-col gap-0.5">
            <div className="px-2 pb-1 type-caption font-semibold text-text-tertiary">{g.title()}</div>
            {g.ids.map((s) => (
              <Link
                key={s}
                to="/settings/$section"
                params={{ section: s }}
                data-status={s === id ? "active" : undefined}
                className="flex min-h-10 items-center gap-3 rounded-[10px] px-2 type-body hover:bg-fill data-[status=active]:bg-fill-selected"
              >
                <Tile id={s} />
                {sections[s].title()}
              </Link>
            ))}
          </nav>
        ))}
      </aside>
      <main className="min-w-0 flex-1 px-14 py-10">
        <div className="max-w-[900px]">
          <SectionView id={id} />
        </div>
      </main>
    </div>
  );
}

/** A section's icon on its colored tile. */
function Tile({ id }: { id: SectionId }) {
  return (
    <span className="flex h-7 w-7 shrink-0 items-center justify-center rounded-[7px] text-white" style={{ background: sections[id].color }}>
      <Icon name={sections[id].icon} fill size={17} />
    </span>
  );
}

/** The user of the shown sign-in. */
function AccountCard() {
  const account = useAccount();
  return (
    <Link to="/settings/$section" params={{ section: "account" }} className="flex items-center gap-3 rounded-[14px] bg-surface p-3 hover:bg-surface-raised">
      <Avatar name={account.name} size={44} />
      <div className="min-w-0">
        <div className="truncate type-headline">{account.name}</div>
        <div className="truncate type-footnote text-text-secondary">
          {account.shown?.kind === "mavioDev" ? t("auth.mavioAccount") : account.line}
        </div>
      </div>
    </Link>
  );
}

/** The phone's profile: the account, the users to switch to, the sections and signing out. */
function ProfileList() {
  const servers = useServersStore();
  const account = useAccount();
  const shown = account.shown;
  return (
    <div className="mx-auto flex min-h-screen max-w-xl flex-col gap-6 px-margin pt-[calc(env(safe-area-inset-top)+12px)] pb-12">
      <Link to="/" className="inline-flex items-center gap-1 self-start type-body text-accent-text">
        <Icon name="back" size={18} />
        {t("nav.home")}
      </Link>
      <h1 className="type-display">{t("nav.profile")}</h1>
      <AccountCard />
      {shown?.kind === "standalone" && <Members server={shown.servers[0]} />}
      {groups.map((g) => (
        <Group key={g.title()} title={g.title()}>
          {g.ids.map((s) => (
            <Link key={s} to="/settings/$section" params={{ section: s }} className="hover:bg-surface-raised">
              <Row label={sections[s].title()} leading={<Tile id={s} />}>
                <Icon name="forward" size={16} className="text-text-tertiary" />
              </Row>
            </Link>
          ))}
        </Group>
      ))}
      {shown && (
        <Button variant="secondary" className="text-danger" onClick={() => void servers.signOut(shown.key)}>
          {t("auth.signOut")}
        </Button>
      )}
    </div>
  );
}

/** The server's users as avatars, to switch to with their PIN. */
function Members({ server }: { server: ServerEntry }) {
  const { users, switchTo } = useSwitchUsers(server);
  const [asking, setAsking] = useState<(typeof users)[number]>();
  if (users.length === 0) return null;
  return (
    <Group title={t("auth.switchUser")}>
      <div className="scroll-row flex gap-5 overflow-x-auto p-4">
        {users.map((u) => (
          <button
            key={u.id}
            type="button"
            disabled={u.current || switchTo.isPending}
            onClick={() => (u.hasPin ? setAsking(u) : switchTo.mutate({ user: u }))}
            className="flex w-16 shrink-0 flex-col items-center gap-1.5"
          >
            <span className={`relative rounded-full ${u.current ? "ring-2 ring-accent ring-offset-2 ring-offset-surface" : ""}`}>
              <Avatar name={u.name} size={56} />
              {u.hasPin && (
                <span className="absolute -right-0.5 -bottom-0.5 rounded-full bg-surface-overlay p-1">
                  <Icon name="lock" fill size={12} />
                </span>
              )}
            </span>
            <span className="w-full truncate text-center type-footnote">{u.name}</span>
          </button>
        ))}
      </div>
      {asking && (
        <PinPad
          name={asking.name}
          busy={switchTo.isPending}
          error={switchTo.error ? errorMessage(switchTo.error) : undefined}
          onCancel={() => (setAsking(undefined), switchTo.reset())}
          onPin={(pin) => switchTo.mutate({ user: asking, pin }, { onSuccess: () => setAsking(undefined) })}
        />
      )}
    </Group>
  );
}

/** A section on a phone, below a link back to the list. */
function CompactSection({ id }: { id: SectionId }) {
  return (
    <div className="mx-auto flex max-w-xl flex-col gap-4 px-margin pt-[calc(env(safe-area-inset-top)+12px)] pb-12">
      <Link to="/settings" className="inline-flex items-center gap-1 self-start type-body text-accent-text">
        <Icon name="back" size={18} />
        {t("nav.profile")}
      </Link>
      <SectionView id={id} />
    </div>
  );
}

function SectionView({ id }: { id: SectionId }) {
  switch (id) {
    case "appearance":
      return <AppearanceSection />;
    case "playback":
      return <PlaybackSection />;
    case "subtitles-audio":
      return <LanguagesSection />;
    case "sign-ins":
      return <SignInsSection />;
    case "servers":
      return <ServersSection />;
    case "account":
      return <AccountSection />;
    case "devices":
      return <DevicesSection />;
  }
}

/** A section's title, the line under it and its groups. */
function Section({ title, message, children }: { title: string; message?: ReactNode; children: ReactNode }) {
  return (
    <div className="flex flex-col gap-7">
      <header>
        <h2 className="type-title1">{title}</h2>
        {message && <p className="mt-1 type-body text-text-secondary">{message}</p>}
      </header>
      {children}
    </div>
  );
}

const themes: { value: Appearance; label: () => string }[] = [
  { value: "system", label: () => t("settings.system") },
  { value: "light", label: () => t("settings.light") },
  { value: "dark", label: () => t("settings.dark") },
];

function AppearanceSection() {
  const prefs = usePrefs();
  return (
    <Section title={t("settings.appearance")} message={t("settings.appearanceMessage")}>
      <div role="radiogroup" aria-label={t("settings.appearance")} className="grid grid-cols-3 gap-4">
        {themes.map((th) => {
          const on = prefs.appearance === th.value;
          return (
            <button
              key={th.value}
              type="button"
              role="radio"
              aria-checked={on}
              onClick={() => setPrefs({ appearance: th.value })}
              className="flex flex-col items-center gap-2"
            >
              <ThemePreview theme={th.value} on={on} />
              <span className={`type-callout ${on ? "font-semibold" : "text-text-secondary"}`}>{th.label()}</span>
            </button>
          );
        })}
      </div>
      <Group footer={t("settings.tvModeNote")}>
        <Row label={t("settings.tvMode")} detail={t("settings.tvModeMessage")}>
          <Toggle on={prefs.tv} label={t("settings.tvMode")} onChange={(tv) => setPrefs({ tv })} />
        </Row>
      </Group>
    </Section>
  );
}

/** A small window in a theme; System shows both halves. */
function ThemePreview({ theme, on }: { theme: Appearance; on: boolean }) {
  const pane = (dark: boolean) => (
    <div className={`flex h-full flex-1 gap-1.5 p-2 ${dark ? "bg-[#1C1C1E]" : "bg-[#F2F2F7]"}`}>
      <div className={`w-1/4 rounded-[4px] ${dark ? "bg-[#2C2C2E]" : "bg-white"}`} />
      <div className="flex flex-1 flex-col gap-1.5">
        <div className="h-1/2 rounded-[4px] bg-gradient-to-br from-[#FFD34E] via-[#FF9A2E] to-[#F0412F]" />
        <div className={`h-1.5 w-2/3 rounded-full ${dark ? "bg-[#3A3A3C]" : "bg-[#D1D1D6]"}`} />
      </div>
    </div>
  );
  return (
    <div
      className={`flex aspect-[16/10] w-full overflow-hidden rounded-[12px] border ${on ? "border-accent ring-2 ring-accent" : "border-border"}`}
    >
      {theme === "system" ? (
        <>
          {pane(false)}
          {pane(true)}
        </>
      ) : (
        pane(theme === "dark")
      )}
    </div>
  );
}

const qualities: { value: Quality; label: () => string }[] = [
  { value: "original", label: () => t("player.qualityOriginal") },
  { value: "auto", label: () => t("player.qualityAuto") },
  { value: "1080p", label: () => "1080p" },
  { value: "720p", label: () => "720p" },
  { value: "480p", label: () => "480p" },
];

function PlaybackSection() {
  const prefs = usePrefs();
  return (
    <Section title={t("settings.playback")} message={t("settings.appearanceMessage")}>
      <Group>
        <Row label={t("settings.maxQuality")}>
          <Select
            value={prefs.quality}
            label={t("settings.maxQuality")}
            options={qualities.map((q) => ({ value: q.value, label: q.label() }))}
            onChange={(quality) => setPrefs({ quality })}
          />
        </Row>
        <Row label={t("settings.autoplayNext")}>
          <Toggle on={prefs.autoplayNext} label={t("settings.autoplayNext")} onChange={(autoplayNext) => setPrefs({ autoplayNext })} />
        </Row>
      </Group>
    </Section>
  );
}

const subtitleModes: { value: SubtitleMode; label: () => string }[] = [
  { value: SubtitleMode.UNSPECIFIED, label: () => t("settings.subtitleModeDefault") },
  { value: SubtitleMode.ALWAYS, label: () => t("settings.subtitleModeAlways") },
  { value: SubtitleMode.FOREIGN, label: () => t("settings.subtitleModeForeign") },
  { value: SubtitleMode.SMART, label: () => t("settings.subtitleModeSmart") },
  { value: SubtitleMode.FORCED, label: () => t("settings.subtitleModeForced") },
  { value: SubtitleMode.NONE, label: () => t("settings.subtitleModeNone") },
];

/** Three-letter codes from a field: lowercase, separated by commas or spaces. */
function languages(s: string): string[] | undefined {
  const list = s
    .toLowerCase()
    .split(/[\s,，、]+/)
    .filter(Boolean);
  return list.every((l) => /^[a-z]{3}$/.test(l)) ? list : undefined;
}

/**
 * The user's languages and when subtitles show, read from the shown
 * sign-in's first server and saved to each of them.
 */
function LanguagesSection() {
  const servers = useServersStore();
  const signedIn = useSignedIn();
  const first = signedIn[0];
  const me = useQuery({
    queryKey: serverKey(first?.id ?? "", "currentUser", first?.userId),
    queryFn: async () => (await servers.client(UserService, first!).getCurrentUser({})).user,
    enabled: !!first,
  });
  const prefs = me.data?.preferences;
  return (
    <Section title={t("settings.subtitlesAudio")} message={t("settings.subtitlesAudioMessage")}>
      {me.data && (
        <LanguagesForm
          key={me.dataUpdatedAt}
          audio={prefs?.audioLanguages.join(", ") ?? ""}
          subtitles={prefs?.subtitleLanguages.join(", ") ?? ""}
          mode={prefs?.subtitleMode ?? SubtitleMode.UNSPECIFIED}
          save={async (p) => {
            await Promise.all(
              signedIn.map((s) =>
                servers.client(UserService, s).updatePreferences({
                  preferences: { ...p, playDefaultAudioTrack: prefs?.playDefaultAudioTrack ?? true },
                }),
              ),
            );
            await me.refetch();
          }}
        />
      )}
    </Section>
  );
}

function LanguagesForm({
  audio: audio0,
  subtitles: subtitles0,
  mode: mode0,
  save,
}: {
  audio: string;
  subtitles: string;
  mode: SubtitleMode;
  save: (p: { audioLanguages: string[]; subtitleLanguages: string[]; subtitleMode: SubtitleMode }) => Promise<void>;
}) {
  const [audio, setAudio] = useState(audio0);
  const [subtitles, setSubtitles] = useState(subtitles0);
  const [mode, setMode] = useState(mode0);
  const saving = useMutation({ mutationFn: save });
  const a = languages(audio);
  const s = languages(subtitles);
  const changed = audio !== audio0 || subtitles !== subtitles0 || mode !== mode0;
  return (
    <form
      className="flex flex-col gap-7"
      onSubmit={(e) => {
        e.preventDefault();
        if (a && s) saving.mutate({ audioLanguages: a, subtitleLanguages: s, subtitleMode: mode });
      }}
    >
      <Group footer={t("settings.languagesHint")}>
        <div className="flex flex-col gap-4 p-4">
          <Field label={t("settings.audioLanguages")} value={audio} onChange={(e) => setAudio(e.target.value)} placeholder="chi, eng" />
          <Field
            label={t("settings.subtitleLanguages")}
            value={subtitles}
            onChange={(e) => setSubtitles(e.target.value)}
            placeholder="chi, eng"
          />
        </div>
      </Group>
      <Group>
        <Row label={t("settings.subtitleMode")}>
          <Select
            value={String(mode)}
            label={t("settings.subtitleMode")}
            options={subtitleModes.map((m) => ({ value: String(m.value), label: m.label() }))}
            onChange={(v) => setMode(Number(v) as SubtitleMode)}
          />
        </Row>
      </Group>
      <div className="flex items-center gap-3">
        <Button type="submit" disabled={!changed || !a || !s || saving.isPending}>
          {t("action.save")}
        </Button>
        {saving.error && <p className="type-callout text-danger">{errorMessage(saving.error)}</p>}
      </div>
    </form>
  );
}

function SignInsSection() {
  const servers = useServersStore();
  const { all, shown } = useSignIns();
  return (
    <Section title={t("auth.signIns")} message={t("settings.signInsMessage")}>
      <Group>
        {all.map((s) => (
          <Row
            key={s.key}
            label={s.name}
            leading={<Avatar name={s.name} size={36} />}
            detail={
              s.kind === "mavioDev"
                ? `${t("auth.mavioAccount")} · ${s.servers.map((x) => x.name).join(", ")}`
                : t("auth.standaloneUser", { server: s.servers[0].name })
            }
          >
            {s.key === shown?.key ? (
              <span className="type-callout text-text-secondary">{t("auth.shown")}</span>
            ) : (
              <Button variant="secondary" onClick={() => servers.show(s.key)}>
                {t("auth.show")}
              </Button>
            )}
            <Button variant="plain" className="text-danger" onClick={() => void servers.signOut(s.key)}>
              {t("auth.signOut")}
            </Button>
          </Row>
        ))}
        <Link to="/sign-in" search={{ add: true }} className="flex min-h-13 items-center gap-3 px-4 type-body text-accent-text hover:bg-surface-raised">
          <Icon name="add" size={20} />
          {t("settings.addSignIn")}
        </Link>
      </Group>
    </Section>
  );
}

function ServersSection() {
  const servers = useServersStore();
  const list = useServers();
  const account = useAccount();
  const navigate = useNavigate();
  const [copied, setCopied] = useState<string>();
  const move = (i: number, by: number) => {
    const ids = list.map((s) => s.id);
    [ids[i], ids[i + by]] = [ids[i + by], ids[i]];
    servers.order(ids);
  };
  return (
    <Section
      title={t("auth.servers")}
      message={account.shown?.kind === "mavioDev" ? t("settings.serversMavioMessage") : t("settings.serversMessage")}
    >
      <Group>
        {list.map((s, i) => (
          <Row
            key={s.id}
            label={s.name}
            leading={
              <span className="flex h-9 w-9 shrink-0 items-center justify-center rounded-[9px] bg-fill type-headline">
                {[...s.name][0]?.toUpperCase()}
              </span>
            }
            detail={`${s.token ? t("auth.signedInAs", { name: s.userName }) : t("auth.signedOut")} · ${s.url}`}
          >
            <div className="flex items-center gap-0.5">
              <IconButton icon="expand" className="rotate-180" label={t("auth.moveUp")} disabled={i === 0} onClick={() => move(i, -1)} />
              <IconButton icon="expand" label={t("auth.moveDown")} disabled={i === list.length - 1} onClick={() => move(i, 1)} />
              <IconButton
                icon={copied === s.id ? "check" : "external"}
                label={t("settings.copyAddress")}
                onClick={() => void navigator.clipboard?.writeText(s.url).then(() => setCopied(s.id))}
              />
              {!s.token && (
                <Button variant="secondary" onClick={() => void navigate({ to: "/sign-in", search: { server: s.url, add: true } })}>
                  {t("auth.signIn")}
                </Button>
              )}
              <Button variant="plain" className="text-danger" onClick={() => void servers.remove(s.id)}>
                {t("auth.signOut")}
              </Button>
            </div>
          </Row>
        ))}
        <Link to="/sign-in" search={{ add: true }} className="flex min-h-13 items-center gap-3 px-4 type-body text-accent-text hover:bg-surface-raised">
          <Icon name="add" size={20} />
          {t("auth.addServer")}
        </Link>
      </Group>
    </Section>
  );
}

function IconButton({
  icon,
  label,
  className,
  ...props
}: React.ButtonHTMLAttributes<HTMLButtonElement> & { icon: IconName; label: string }) {
  return (
    <button
      type="button"
      aria-label={label}
      title={label}
      {...props}
      className="flex h-9 w-9 items-center justify-center rounded-lg text-text-secondary hover:bg-fill hover:text-on-background disabled:opacity-30 disabled:hover:bg-transparent"
    >
      <Icon name={icon} size={18} className={className} />
    </button>
  );
}

function AccountSection() {
  const account = useAccount();
  const shown = account.shown;
  const manage = useMavioDevAccountUrl(shown);
  if (!shown) return null;
  if (shown.kind === "mavioDev") {
    return (
      <Section title={t("settings.account")} message={t("settings.mavioAccountMessage")}>
        <Group>
          <Row label={account.name} detail={account.line} leading={<Avatar name={account.name} size={44} />}>
            {manage && (
              <a href={manage} target="_blank" rel="noreferrer" className="inline-flex items-center gap-1.5 type-body text-accent-text">
                {t("settings.manageAtMavio")}
                <Icon name="external" size={16} />
              </a>
            )}
          </Row>
        </Group>
      </Section>
    );
  }
  const server = shown.servers[0];
  return (
    <Section title={t("settings.account")} message={t("settings.standaloneAccountMessage", { name: server.userName, server: server.name })}>
      <SwitchUser server={server} />
      <Security server={server} />
      <AuthorizeDevice server={server} />
    </Section>
  );
}

/** Signs in another device showing a Quick Connect code, as the user of a standalone server. */
function AuthorizeDevice({ server }: { server: ServerEntry }) {
  const servers = useServersStore();
  const [code, setCode] = useState("");
  const [state, setState] = useState<{ ok?: boolean; error?: string }>({});
  return (
    <Group title={t("auth.authorizeDevice")}>
      <form
        className="flex flex-wrap items-end gap-3 p-4"
        onSubmit={(e) => {
          e.preventDefault();
          setState({});
          servers
            .authorizeQuickConnect(server, code)
            .then(() => (setState({ ok: true }), setCode("")))
            .catch((err: unknown) => setState({ error: errorMessage(err) }));
        }}
      >
        <div className="w-40">
          <Field
            label={t("auth.quickConnect")}
            value={code}
            onChange={(e) => setCode(e.target.value.replace(/\D/g, "").slice(0, 6))}
            inputMode="numeric"
            placeholder="000000"
            className="font-mono"
          />
        </div>
        <Button type="submit" disabled={code.length < 6}>
          {t("auth.authorize")}
        </Button>
        {state.ok && <p className="w-full type-callout text-text-secondary">{t("auth.authorized")}</p>}
        {state.error && <p className="w-full type-callout text-danger">{state.error}</p>}
      </form>
    </Group>
  );
}

const when = new Intl.DateTimeFormat(locale, { dateStyle: "medium", timeStyle: "short" });

/** The sessions of the user on a server of the shown sign-in, to sign one out. */
function DevicesSection() {
  const servers = useServersStore();
  const signedIn = useSignedIn();
  const queryClient = useQueryClient();
  const [serverId, setServerId] = useState<string>();
  const server = signedIn.find((s) => s.id === serverId) ?? signedIn[0];
  const key = serverKey(server?.id ?? "", "sessions", server?.userId);
  const sessions = useQuery({
    queryKey: key,
    queryFn: async () => (await servers.client(AuthService, server!).listSessions({})).sessions,
    enabled: !!server,
  });
  const revoke = useMutation({
    mutationFn: (id: string) => servers.client(AuthService, server!).revokeSession({ id }),
    onSettled: () => queryClient.invalidateQueries({ queryKey: key }),
  });
  if (!server) return null;
  return (
    <Section title={t("settings.devices")} message={t("settings.devicesMessage", { server: server.name })}>
      {signedIn.length > 1 && (
        <Select
          value={server.id}
          label={t("auth.server")}
          options={signedIn.map((s) => ({ value: s.id, label: s.name }))}
          onChange={setServerId}
        />
      )}
      <Group>
        {(sessions.data ?? []).map((s) => (
          <Row
            key={s.id}
            label={s.device?.name || s.device?.client || "—"}
            leading={
              <span className="flex h-9 w-9 shrink-0 items-center justify-center rounded-[9px] bg-fill">
                <Icon name="devices" size={20} />
              </span>
            }
            detail={[s.device?.client, s.lastSeenTime && when.format(timestampDate(s.lastSeenTime))].filter(Boolean).join(" · ")}
          >
            {s.current ? (
              <span className="type-callout text-text-secondary">{t("settings.thisBrowser")}</span>
            ) : (
              <Button variant="plain" className="text-danger" disabled={revoke.isPending} onClick={() => revoke.mutate(s.id)}>
                {t("auth.signOut")}
              </Button>
            )}
          </Row>
        ))}
      </Group>
      {(sessions.error || revoke.error) && <p className="type-callout text-danger">{errorMessage(sessions.error ?? revoke.error)}</p>}
    </Section>
  );
}
