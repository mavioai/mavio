// The settings (docs/clients.md §6.3): those of this browser, its
// appearance, TV mode, playback and the assistant; the account, on its own page under the
// user's card, with its credentials and the servers in their order; and
// those of a server, chosen above them with a Mavio account, the
// devices signed in to it as the user and, for its administrators, its
// overview, general settings, users and API keys, and menus of the rest:
// media, acquisition and downloads, extensions, and monitoring and
// maintenance. Wide windows show them beside their own sidebar, the menus
// opening in place; phones and televisions list them first, a menu as a
// list of its own.
import { timestampDate } from "@bufbuild/protobuf/wkt";
import {
  errorMessage,
  type ServerEntry,
  serverKey,
  shareableUrl,
  useAdminServers,
  useServers,
  useRevokeSession,
  useServersStore,
  useSessions,
  useSignedIn,
  useSignIn,
} from "@mavio/client-ts";
import { locales } from "@mavio/design/strings";
import { SubtitleMode, UserService } from "@mavio/proto/mavio/user/v1/user_pb";
import { useMutation, useQuery } from "@tanstack/react-query";
import { Link, Navigate, useNavigate, useParams, useSearch } from "@tanstack/react-router";
import { type ReactNode, useState } from "react";

import { Tip } from "../controls/tooltip.tsx";
import { locale, t } from "../i18n.ts";
import { Icon, type IconName } from "../icon.tsx";
import { SidebarResizer, sidebarWidth } from "../shell/sidebar.tsx";
import { up } from "../shell/up.ts";
import { type Appearance, qualities, setPrefs, usePrefs, useWidthClass } from "../prefs.ts";
import type { SettingsSearch } from "../search-params.ts";
import { PinPad, useAccount, useMavioDevAccountUrl, useSwitchUsers } from "../shell/user-menu.tsx";
import { Avatar, Button, Field, Group, Row } from "../ui.tsx";
import { Pictures, Switch } from "../controls/inputs.tsx";
import { Select } from "../controls/select.tsx";
import { GeneralSection } from "./admin/general.tsx";
import { IssuesSection } from "./issues.tsx";
import { LibrariesSection } from "./admin/libraries.tsx";
import { ActivitySection } from "./admin/activity.tsx";
import { OverviewSection } from "./admin/overview.tsx";
import { TasksSection } from "./admin/tasks.tsx";
import { AcquisitionSection } from "./admin/acquisition.tsx";
import { StorageSection } from "./admin/storage.tsx";
import { SubtitlesSection } from "./admin/subtitles.tsx";
import { AiSection } from "./admin/ai.tsx";
import { AssistantPrefs } from "./assistant-settings.tsx";
import { ConversationsSection } from "./admin/conversations.tsx";
import { DownloadsSection } from "./admin/downloads.tsx";
import { WantedSection } from "./admin/wanted.tsx";
import { ApiKeysSection, BackupsSection, LogsSection } from "./admin/backups.tsx";
import { TranscodingSection } from "./admin/transcoding.tsx";
import { UsersSection } from "./admin/users.tsx";
import { PluginsSection } from "./plugins.tsx";
import { RemoteDialog } from "./remote.tsx";
import { Security, SwitchUser } from "./security.tsx";
import { useUnsavedGuard } from "../controls/unsaved.tsx";
import { isMenu, type MenuId, menuOf, menuSections } from "./settings-menus.ts";

/** Where the settings are, which the pages of the sections read. */
export type { SettingsSearch } from "../search-params.ts";

type SectionId = "appearance" | "playback" | "subtitles-audio" | "assistant" | "account" | "overview" | "activity" | "general" | "devices" | "issues" | "libraries" | "users" | "transcoding" | "storage" | "acquisition" | "wanted" | "downloads" | "subtitles" | "ai" | "conversations" | "plugins" | "tasks" | "backups" | "logs" | "api-keys";

const sections: Record<SectionId, { icon: IconName; color: string; title: () => string; admin?: boolean }> = {
  appearance: { icon: "appearance", color: "#5E5CE6", title: () => t("settings.appearance") },
  playback: { icon: "play", color: "#FF3B30", title: () => t("settings.playback") },
  "subtitles-audio": { icon: "subtitles", color: "#34C759", title: () => t("settings.subtitlesAudio") },
  assistant: { icon: "microphone", color: "#FF2D55", title: () => t("assistant.title") },
  account: { icon: "profile", color: "#FF9500", title: () => t("settings.account") },
  overview: { icon: "server", color: "#8E8E93", title: () => t("settings.overview"), admin: true },
  activity: { icon: "activity", color: "#FF2D55", title: () => t("settings.activity"), admin: true },
  general: { icon: "settings", color: "#8E8E93", title: () => t("settings.general"), admin: true },
  devices: { icon: "devices", color: "#32ADE6", title: () => t("settings.devices") },
  issues: { icon: "error", color: "#FF9500", title: () => t("settings.issues") },
  libraries: { icon: "folder", color: "#007AFF", title: () => t("settings.libraries"), admin: true },
  users: { icon: "users", color: "#FF9500", title: () => t("settings.users"), admin: true },
  transcoding: { icon: "quality", color: "#FF2D55", title: () => t("settings.transcoding"), admin: true },
  storage: { icon: "server", color: "#30B0C7", title: () => t("settings.storage"), admin: true },
  acquisition: { icon: "downloads", color: "#34C759", title: () => t("settings.acquisition"), admin: true },
  wanted: { icon: "pending", color: "#FF3B30", title: () => t("settings.wanted"), admin: true },
  downloads: { icon: "downloads", color: "#34C759", title: () => t("settings.downloads"), admin: true },
  subtitles: { icon: "subtitles", color: "#FF9500", title: () => t("settings.subtitles"), admin: true },
  ai: { icon: "key", color: "#BF5AF2", title: () => t("settings.ai"), admin: true },
  conversations: { icon: "info", color: "#5E5CE6", title: () => t("settings.conversations"), admin: true },
  backups: { icon: "folder", color: "#5856D6", title: () => t("settings.backups"), admin: true },
  logs: { icon: "info", color: "#8E8E93", title: () => t("settings.logs"), admin: true },
  "api-keys": { icon: "lock", color: "#636366", title: () => t("settings.apiKeys"), admin: true },
  plugins: { icon: "plugins", color: "#AF52DE", title: () => t("settings.plugins"), admin: true },
  tasks: { icon: "pending", color: "#5856D6", title: () => t("settings.tasks"), admin: true },
};

const menus: Record<MenuId, { icon: IconName; color: string; title: () => string }> = {
  media: { icon: "movies", color: "#FF9F0A", title: () => t("settings.mediaMenu") },
  acquiring: { icon: "cloud", color: "#30D158", title: () => t("settings.acquisitionMenu") },
  extensions: { icon: "plugins", color: "#5E5CE6", title: () => t("settings.extensionsMenu") },
  maintenance: { icon: "admin", color: "#8E8E93", title: () => t("settings.maintenanceMenu") },
};

/** An entry of a group: a section, or a menu of sections. */
type Entry = SectionId | MenuId;

function lookOf(e: Entry) {
  return isMenu(e) ? menus[e] : sections[e];
}

/**
 * The groups: this device's sections, and the server's, in the order a
 * server is set up and kept: the server and who reaches it, then the menus
 * of its media, of acquiring media, of what extends it, and of watching
 * over and maintaining it.
 */
const allGroups: { title: () => string; entries: Entry[]; server?: boolean }[] = [
  { title: () => t("settings.thisDevice"), entries: ["appearance", "playback", "subtitles-audio", "assistant"] },
  { title: () => t("settings.serverGroup"), entries: ["overview", "general", "users", "devices", "api-keys", "media", "acquiring", "extensions", "maintenance"], server: true },
];

/** The server whose settings are shown, whether the user administers it, and whether that is known yet. */
export interface Chosen {
  server: ServerEntry;
  admin: boolean;
  known: boolean;
}

/** The server chosen, else the one serving the web UI, else the first signed in to. */
function useChosen(): Chosen | undefined {
  const servers = useServersStore();
  const signedIn = useSignedIn();
  const admins = useAdminServers();
  const { server: id } = useSearch({ strict: false }) as SettingsSearch;
  const home = servers.homeServer();
  const server = signedIn.find((s) => s.id === id) ?? signedIn.find((s) => s.id === home?.id) ?? signedIn[0];
  // The user as useAdminServers asks for it, shared through the cache.
  const me = useQuery({
    queryKey: serverKey(server?.id ?? "", "currentUser", server?.userId),
    queryFn: async () => (await servers.client(UserService, server!).getCurrentUser({})).user ?? null,
    enabled: !!server,
    staleTime: 5 * 60_000,
    retry: false,
  });
  return server && { server, admin: admins.some((a) => a.id === server.id), known: me.isFetched };
}

/**
 * The groups, a server's administration only for its administrators; a
 * menu left with one section is that section, and one left with none is
 * not shown.
 */
function useGroups(chosen: Chosen | undefined) {
  const shown = (id: SectionId) => !sections[id].admin || !!chosen?.admin;
  return allGroups
    .filter((g) => !g.server || chosen)
    .map((g) => ({
      ...g,
      entries: g.entries.flatMap((e): Entry[] => {
        if (!isMenu(e)) return shown(e) ? [e] : [];
        const ids = menuItems(e, chosen);
        return ids.length > 1 ? [e] : ids;
      }),
    }));
}

/** A menu's sections shown to the user. */
function menuItems(menu: MenuId, chosen: Chosen | undefined): SectionId[] {
  return (menuSections[menu] as SectionId[]).filter((id) => !sections[id].admin || !!chosen?.admin);
}

type Groups = ReturnType<typeof useGroups>;

/** The menu a section is shown in, if it is in one. */
function shownMenuOf(groups: Groups, id: SectionId): MenuId | undefined {
  const menu = menuOf(id);
  return menu && groups.some((g) => g.entries.includes(menu)) ? menu : undefined;
}

function isSection(s: string | undefined): s is SectionId {
  return s !== undefined && Object.hasOwn(sections, s);
}

export function SettingsScreen() {
  const { section } = useParams({ strict: false }) as { section?: string };
  const search = useSearch({ strict: false }) as SettingsSearch;
  const width = useWidthClass();
  const prefs = usePrefs();
  const chosen = useChosen();
  const admins = useAdminServers();
  const groups = useGroups(chosen);
  // The servers moved to the account's page.
  if (section === "servers") return <Navigate to="/settings/$section" params={{ section: "account" }} replace />;
  if (section !== undefined && !isSection(section) && !isMenu(section)) return <Navigate to="/settings" replace />;
  // A server the user does not administer has no administration; wait
  // until the servers administered are known.
  if (isSection(section) && sections[section].admin && chosen && !chosen.admin && admins.length > 0) {
    return <Navigate to="/settings/$section" params={{ section: "devices" }} search={{ server: search.server }} replace />;
  }
  const compact = width === "compact" || prefs.tv;
  if (isMenu(section)) {
    // A menu is a list of its own on a phone and a television; a wide
    // window, opening menus in its sidebar, shows the menu's first section,
    // as does a menu left with one.
    if (!chosen?.known) return null;
    const ids = menuItems(section, chosen);
    if (compact && ids.length > 1) return <MenuList menu={section} ids={ids} />;
    return ids[0] ? (
      <Navigate to="/settings/$section" params={{ section: ids[0] }} search={{ server: search.server }} replace />
    ) : (
      <Navigate to="/settings" search={{ server: search.server }} replace />
    );
  }
  // A phone and a television show the list of sections, then a section in
  // its place; the television without a sidebar (docs/tv.md §6.6).
  if (compact) {
    return section ? <CompactSection id={section} menu={shownMenuOf(groups, section)} chosen={chosen} /> : <ProfileList groups={groups} />;
  }
  const id = section ?? "appearance";
  return (
    <div className="flex min-h-screen">
      <aside className="sticky top-0 h-screen shrink-0" style={{ width: sidebarWidth }}>
        <div className="flex h-full flex-col gap-4 overflow-y-auto border-r border-border bg-sidebar px-4 py-5">
          {/* Home, as on a phone: the settings' own pages lie between. */}
          <Link to="/" className="inline-flex items-center gap-1 self-start type-body text-accent-text">
            <Icon name="back" size={18} />
            {t("nav.home")}
          </Link>
          {/* The section's own title names the page, once it shows. */}
          <h1 data-title-later className="px-1 type-title1">{t("nav.settings")}</h1>
          <AccountCard active={id === "account"} />
          {groups.map((g) => (
            <nav key={g.title()} aria-label={g.title()} className="flex flex-col gap-0.5">
              <div className="px-2 pb-1 type-caption font-semibold text-text-tertiary">{g.title()}</div>
              {g.server && chosen && <ServerChoice chosen={chosen} section={id} />}
              {g.entries.map((e) =>
                isMenu(e) ? (
                  <SidebarMenu key={e} menu={e} ids={menuItems(e, chosen)} current={id} />
                ) : (
                  <SidebarLink key={e} id={e} current={id} />
                ),
              )}
            </nav>
          ))}
        </div>
        <SidebarResizer />
      </aside>
      <main className="min-w-0 flex-1 px-14 py-10">
        <div className="max-w-[900px]">
          <SectionView id={id} chosen={chosen} />
        </div>
      </main>
    </div>
  );
}

/**
 * The choice of the server whose settings are shown, with a Mavio account
 * alone: a standalone server's user is signed in to one server. The server
 * serving the web UI is marked. In a group, on a phone, it is padded as
 * the group's rows.
 */
function ServerChoice({ chosen, section, inGroup = false }: { chosen: Chosen; section?: SectionId; inGroup?: boolean }) {
  const servers = useServersStore();
  const signIn = useSignIn();
  const signedIn = useSignedIn();
  const navigate = useNavigate();
  if (signIn?.kind !== "mavioDev") return null;
  const home = servers.homeServer();
  return (
    <div className={`flex flex-col [&>select]:min-h-10 [&>select]:w-full ${inGroup ? "px-4 py-2.5" : "px-2 pb-1.5"}`}>
      <Select
        value={chosen.server.id}
        label={t("settings.serverChoice")}
        options={signedIn.map((s) => ({ value: s.id, label: s.id === home?.id ? t("settings.thisServer", { name: s.name }) : s.name }))}
        // Another server's settings take the place of these, as a choice of the page.
        onChange={(server) =>
          void navigate(
            section
              ? { to: "/settings/$section", params: { section }, search: { server }, replace: true }
              : { to: "/settings", search: { server }, replace: true },
          )
        }
      />
    </div>
  );
}

/** A section's link in the sidebar. */
function SidebarLink({ id, current }: { id: SectionId; current: SectionId }) {
  const search = useSearch({ strict: false }) as SettingsSearch;
  return (
    <Link
      to="/settings/$section"
      params={{ section: id }}
      search={{ server: search.server }}
      data-status={id === current ? "active" : undefined}
      aria-current={id === current ? "page" : undefined}
      className="flex min-h-10 items-center gap-3 rounded-[10px] px-2 type-body hover:bg-fill data-[status=active]:bg-fill-selected"
    >
      <Tile id={id} />
      {sections[id].title()}
    </Link>
  );
}

/**
 * A menu in the sidebar, opening in place to its sections: open while it
 * holds the section shown, until closed, and marked while closed over it.
 */
function SidebarMenu({ menu, ids, current }: { menu: MenuId; ids: SectionId[]; current: SectionId }) {
  const within = ids.includes(current);
  const [open, setOpen] = useState(within);
  // A section of the menu opened from elsewhere, as from a link, opens it.
  const [shown, setShown] = useState(current);
  if (shown !== current) {
    setShown(current);
    if (within && !open) setOpen(true);
  }
  return (
    <div className="flex flex-col gap-0.5">
      <button
        type="button"
        aria-expanded={open}
        data-status={within && !open ? "active" : undefined}
        onClick={() => setOpen(!open)}
        className="flex min-h-10 items-center gap-3 rounded-[10px] px-2 text-left type-body hover:bg-fill data-[status=active]:bg-fill-selected"
      >
        <Tile id={menu} />
        <span className="flex-1">{menus[menu].title()}</span>
        <Icon name="expand" size={16} className={`text-text-tertiary transition-transform ${open ? "" : "-rotate-90"}`} />
      </button>
      {open && (
        <div role="group" aria-label={menus[menu].title()} className="flex flex-col gap-0.5 pl-6">
          {ids.map((s) => (
            <SidebarLink key={s} id={s} current={current} />
          ))}
        </div>
      )}
    </div>
  );
}

/** An entry's icon on its colored tile. */
function Tile({ id }: { id: Entry }) {
  const look = lookOf(id);
  return (
    <span className="flex h-7 w-7 shrink-0 items-center justify-center rounded-[7px] text-white" style={{ background: look.color }}>
      <Icon name={look.icon} fill size={17} />
    </span>
  );
}

/** An entry's row in a list, opening it. */
function EntryRow({ id }: { id: Entry }) {
  const search = useSearch({ strict: false }) as SettingsSearch;
  return (
    <Link to="/settings/$section" params={{ section: id }} search={{ server: search.server }} className="hover:tint">
      <Row label={lookOf(id).title()} leading={<Tile id={id} />}>
        <Icon name="forward" size={16} className="text-text-tertiary" />
      </Row>
    </Link>
  );
}

/** The user signed in. */
/** The user's card, opening the account's page. */
function AccountCard({ active = false }: { active?: boolean }) {
  const account = useAccount();
  return (
    <Link
      to="/settings/$section"
      params={{ section: "account" }}
      aria-current={active ? "page" : undefined}
      className={`flex items-center gap-3 rounded-[14px] p-3 hover:tint ${active ? "bg-fill-selected" : "bg-surface"}`}
    >
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

/** The phone's settings: the account, the users to switch to, the sections and signing out. */
function ProfileList({ groups }: { groups: Groups }) {
  const servers = useServersStore();
  const account = useAccount();
  const shown = account.shown;
  const chosen = useChosen();
  const tv = usePrefs().tv;
  return (
    <div className={`mx-auto flex min-h-screen flex-col gap-6 px-margin pb-12 ${tv ? "max-w-3xl pt-16" : "max-w-xl pt-[calc(env(safe-area-inset-top)+12px)]"}`}>
      {/* The remote's Back leaves the settings on the television. */}
      {!tv && (
        <Link to="/" className="inline-flex items-center gap-1 self-start type-body text-accent-text">
          <Icon name="back" size={18} />
          {t("nav.home")}
        </Link>
      )}
      <h1 className="type-display">{t("nav.settings")}</h1>
      <AccountCard />
      {shown?.kind === "standalone" && <Members server={shown.servers[0]} />}
      {groups.map((g) => (
        <Group key={g.title()} title={g.title()}>
          {g.server && chosen && <ServerChoice chosen={chosen} inGroup />}
          {g.entries.map((e) => (
            <EntryRow key={e} id={e} />
          ))}
        </Group>
      ))}
      {shown && (
        <Button variant="secondary" className="text-danger" onClick={() => void servers.signOut()}>
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

/**
 * A menu on a phone and a television: its sections, below a link back to
 * the list on a phone; the television's Back returns to the list.
 */
function MenuList({ menu, ids }: { menu: MenuId; ids: SectionId[] }) {
  const tv = usePrefs().tv;
  return (
    <div className={`mx-auto flex min-h-screen flex-col gap-6 px-margin pb-12 ${tv ? "max-w-3xl pt-16" : "max-w-xl pt-[calc(env(safe-area-inset-top)+12px)]"}`}>
      {!tv && <UpLink />}
      <h1 className="type-display">{menus[menu].title()}</h1>
      <Group>
        {ids.map((s) => (
          <EntryRow key={s} id={s} />
        ))}
      </Group>
    </div>
  );
}

/** A phone's link up from a section to its menu, else from a section or a menu to the list. */
function UpLink({ menu }: { menu?: MenuId }) {
  const search = useSearch({ strict: false }) as SettingsSearch;
  const className = "inline-flex items-center gap-1 self-start type-body text-accent-text";
  return menu ? (
    <Link to="/settings/$section" params={{ section: menu }} search={{ server: search.server }} {...up} className={className}>
      <Icon name="back" size={18} />
      {menus[menu].title()}
    </Link>
  ) : (
    <Link to="/settings" search={{ server: search.server }} {...up} className={className}>
      <Icon name="back" size={18} />
      {t("nav.settings")}
    </Link>
  );
}

/**
 * A section on a phone, below a link back to its menu or the list; on a
 * television, whose Back returns there, alone.
 */
function CompactSection({ id, menu, chosen }: { id: SectionId; menu: MenuId | undefined; chosen: Chosen | undefined }) {
  const tv = usePrefs().tv;
  return (
    <div className={`mx-auto flex flex-col gap-4 px-margin pb-12 ${tv ? "max-w-3xl pt-16" : "max-w-xl pt-[calc(env(safe-area-inset-top)+12px)]"}`}>
      {!tv && <UpLink menu={menu} />}
      <SectionView id={id} chosen={chosen} />
    </div>
  );
}

function SectionView({ id, chosen }: { id: SectionId; chosen: Chosen | undefined }) {
  switch (id) {
    case "appearance":
      return <AppearanceSection />;
    case "playback":
      return <PlaybackSection />;
    case "subtitles-audio":
      return <LanguagesSection />;
    case "assistant":
      return (
        <Section title={t("assistant.title")} message={t("assistant.settingsMessage")}>
          <AssistantPrefs key={chosen?.server.id} server={chosen?.server} />
        </Section>
      );
    case "account":
      return <AccountSection />;
    case "overview":
      return chosen?.admin && <OverviewSection key={chosen.server.id} server={chosen.server} />;
    case "activity":
      return chosen?.admin && <ActivitySection key={chosen.server.id} server={chosen.server} />;
    case "general":
      return chosen?.admin && <GeneralSection key={chosen.server.id} server={chosen.server} />;
    case "acquisition":
      return chosen?.admin && <AcquisitionSection key={chosen.server.id} server={chosen.server} />;
    case "wanted":
      return chosen?.admin && <WantedSection key={chosen.server.id} server={chosen.server} />;
    case "downloads":
      return chosen?.admin && <DownloadsSection key={chosen.server.id} server={chosen.server} />;
    case "subtitles":
      return chosen?.admin && <SubtitlesSection key={chosen.server.id} server={chosen.server} />;
    case "ai":
      return chosen?.admin && <AiSection key={chosen.server.id} server={chosen.server} />;
    case "conversations":
      return chosen?.admin && <ConversationsSection key={chosen.server.id} server={chosen.server} />;
    case "backups":
      return chosen?.admin && <BackupsSection key={chosen.server.id} server={chosen.server} />;
    case "logs":
      return chosen?.admin && <LogsSection key={chosen.server.id} server={chosen.server} />;
    case "api-keys":
      return chosen?.admin && <ApiKeysSection key={chosen.server.id} server={chosen.server} />;
    case "storage":
      return chosen?.admin && <StorageSection key={chosen.server.id} server={chosen.server} />;
    case "transcoding":
      return chosen?.admin && <TranscodingSection key={chosen.server.id} server={chosen.server} />;
    case "tasks":
      return chosen?.admin && <TasksSection key={chosen.server.id} server={chosen.server} />;
    case "devices":
      return chosen && <DevicesSection key={chosen.server.id} server={chosen.server} />;
    case "issues":
      return chosen && <IssuesSection key={chosen.server.id} server={chosen.server} />;
    case "libraries":
      return chosen?.admin && <LibrariesSection key={chosen.server.id} server={chosen.server} />;
    case "users":
      return chosen?.admin && <UsersSection key={chosen.server.id} server={chosen.server} />;
    case "plugins":
      return chosen?.admin && <PluginsSection key={chosen.server.id} server={chosen.server} />;
  }
}

/** A section's title, the line under it and its groups. */
function Section({ title, message, children }: { title: string; message?: ReactNode; children: ReactNode }) {
  return (
    <div className="flex flex-col gap-7">
      <header>
        <h2 data-title className="type-title1">{title}</h2>
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
      <Pictures
        value={prefs.appearance}
        label={t("settings.appearance")}
        options={themes.map((th) => th.value)}
        onChange={(appearance) => setPrefs({ appearance })}
        className="grid grid-cols-3 gap-4"
        draw={(theme, on) => (
          <>
            <ThemePreview theme={theme} on={on} />
            <span className={`type-callout ${on ? "font-semibold" : "text-text-secondary"}`}>{themes.find((th) => th.value === theme)?.label()}</span>
          </>
        )}
      />
      <Group footer={t("settings.languageNote")}>
        <Row label={t("settings.language")}>
          <Select
            value={prefs.language}
            label={t("settings.language")}
            options={[
              { value: "system" as const, label: t("settings.systemLanguage") },
              ...locales.map((l) => ({ value: l, label: new Intl.DisplayNames([l], { type: "language" }).of(l) ?? l })),
            ]}
            onChange={(language) => {
              // The strings and formats are chosen as the page loads.
              setPrefs({ language });
              location.reload();
            }}
          />
        </Row>
      </Group>
      <Group footer={t("settings.tvModeNote")}>
        <Row label={t("settings.tvMode")} detail={t("settings.tvModeMessage")}>
          <Switch on={prefs.tv} label={t("settings.tvMode")} onChange={(tv) => setPrefs({ tv })} />
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
          <Switch on={prefs.autoplayNext} label={t("settings.autoplayNext")} onChange={(autoplayNext) => setPrefs({ autoplayNext })} />
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
 * The user's languages and when subtitles show, read from the first
 * server signed in to and saved to each of them.
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
  const prefs = me.data?.playbackPreferences;
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
                servers.client(UserService, s).updatePlaybackPreferences({
                  playbackPreferences: { ...p, playDefaultAudioTrack: prefs?.playDefaultAudioTrack ?? true },
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
  const guard = useUnsavedGuard(changed);
  return (
    <form
      className="flex flex-col gap-7"
      onSubmit={(e) => {
        e.preventDefault();
        if (a && s) saving.mutate({ audioLanguages: a, subtitleLanguages: s, subtitleMode: mode });
      }}
    >
      {guard.dialog}
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

/** The servers in the user's order, each with its address, moved up and down. */
function ServersGroup() {
  const servers = useServersStore();
  const list = useServers();
  const account = useAccount();
  const [copied, setCopied] = useState<string>();
  const move = (i: number, by: number) => {
    const ids = list.map((s) => s.id);
    [ids[i], ids[i + by]] = [ids[i + by], ids[i]];
    servers.order(ids);
  };
  return (
    <Group title={t("auth.servers")} footer={account.shown?.kind === "mavioDev" ? t("settings.serversMavioMessage") : t("settings.serversMessage")}>
      {list.map((s, i) => {
        const shared = shareableUrl(s);
        return (
          <Row
            key={s.id}
            label={s.name}
            leading={
              <span className="flex h-9 w-9 shrink-0 items-center justify-center rounded-[9px] bg-fill type-headline">
                {[...s.name][0]?.toUpperCase()}
              </span>
            }
            wrap
            detail={
              s.token ? (
                <>
                  {t("auth.signedInAs", { name: s.userName })} · <span className="break-all">{shared}</span>
                  {s.url !== shared && <span className="block break-all">{t("settings.reachedAt", { url: s.url })}</span>}
                </>
              ) : (
                t("auth.unreachable")
              )
            }
          >
            <div className="flex items-center gap-0.5">
              {list.length > 1 && (
                <>
                  <IconButton icon="expand" className="rotate-180" label={t("auth.moveUp")} disabled={i === 0} onClick={() => move(i, -1)} />
                  <IconButton icon="expand" label={t("auth.moveDown")} disabled={i === list.length - 1} onClick={() => move(i, 1)} />
                </>
              )}
              {shared && (
                <IconButton
                  icon={copied === s.id ? "check" : "copy"}
                  label={t("settings.copyAddress")}
                  onClick={() => void navigator.clipboard?.writeText(shared).then(() => setCopied(s.id))}
                />
              )}
            </div>
          </Row>
        );
      })}
    </Group>
  );
}

function IconButton({
  icon,
  label,
  className,
  ...props
}: React.ButtonHTMLAttributes<HTMLButtonElement> & { icon: IconName; label: string }) {
  return (
    <Tip label={label}>
      <button
        type="button"
        aria-label={label}
        {...props}
        className="flex h-9 w-9 items-center justify-center rounded-lg text-text-secondary hover:bg-fill hover:text-on-background disabled:opacity-30 disabled:hover:bg-transparent"
      >
        <Icon name={icon} size={18} className={className} />
      </button>
    </Tip>
  );
}

function AccountSection() {
  const account = useAccount();
  const shown = account.shown;
  const manage = useMavioDevAccountUrl();
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
        <ServersGroup />
      </Section>
    );
  }
  const server = shown.servers[0];
  return (
    <Section title={t("settings.account")} message={t("settings.standaloneAccountMessage", { name: server.userName, server: server.name })}>
      <SwitchUser server={server} />
      <Security server={server} />
      <AuthorizeDevice server={server} />
      <ServersGroup />
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

/** The sessions of the user on the server chosen, to sign one out or control one online (§6.3.23). */
function DevicesSection({ server }: { server: ServerEntry }) {
  const sessions = useSessions(server);
  const revoke = useRevokeSession(server);
  const [controlling, setControlling] = useState<string>();
  const detailOf = (s: NonNullable<typeof sessions.data>[number]) => {
    const playing = s.nowPlaying?.item;
    return [
      s.client,
      playing ? t("remote.playing", { name: playing.name }) : s.online ? t("remote.online") : s.lastSeenTime && when.format(timestampDate(s.lastSeenTime)),
    ]
      .filter(Boolean)
      .join(" · ");
  };
  return (
    <Section title={t("settings.devices")} message={t("settings.devicesMessage")}>
      <Group>
        {(sessions.data ?? []).map((s) => (
          <Row
            key={s.id}
            label={s.deviceName || s.client || "—"}
            leading={
              <span className="flex h-9 w-9 shrink-0 items-center justify-center rounded-[9px] bg-fill">
                <Icon name="devices" size={20} />
              </span>
            }
            detail={detailOf(s)}
          >
            {s.current ? (
              <span className="type-callout text-text-secondary">{t("settings.thisBrowser")}</span>
            ) : (
              <>
                {s.online && (
                  <Button variant="plain" onClick={() => setControlling(s.id)}>
                    {t("remote.control")}
                  </Button>
                )}
                <Button variant="plain" className="text-danger" disabled={revoke.isPending} onClick={() => revoke.mutate(s.id)}>
                  {t("auth.signOut")}
                </Button>
              </>
            )}
          </Row>
        ))}
      </Group>
      {controlling && <RemoteDialog server={server} sessionId={controlling} onClose={() => setControlling(undefined)} />}
      {(sessions.error || revoke.error) && <p className="type-callout text-danger">{errorMessage(sessions.error ?? revoke.error)}</p>}
    </Section>
  );
}
