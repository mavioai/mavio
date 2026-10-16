// The user signed in and the menu it opens (docs/clients.md §6.3):
// switching to another user of the server, the settings, the account at
// mavio.dev or the servers, and signing out.
import { errorMessage, type ServerEntry, serverKey, type SignIn, useServersStore, useSignIn } from "@mavio/client-ts";
import type { SwitchableUser } from "@mavio/proto/mavio/auth/v1/auth_pb";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useLocation } from "@tanstack/react-router";
import { useCallback, useEffect, useState } from "react";

import { Sheet } from "../controls/dialog.tsx";
import { Menu, MenuChoice, MenuExternal, MenuIconOption, MenuItem, MenuLabel, MenuLink, MenuSeparator } from "../controls/menu.tsx";
import { Tip } from "../controls/tooltip.tsx";
import { toggleFullScreen, useFullScreen } from "../fullscreen.ts";
import { t } from "../i18n.ts";
import { Icon, type IconName } from "../icon.tsx";
import { shortcutLabel, useShortcutsShown } from "../keys.ts";
import { type Appearance, setPrefs, usePrefs } from "../prefs.ts";
import { Avatar } from "../ui.tsx";

/** The sign-in as the user menu presents it. */
export function useAccount(): { shown?: SignIn; name: string; line: string } {
  const shown = useSignIn();
  if (!shown) return { name: "", line: "" };
  return {
    shown,
    name: shown.name,
    line:
      shown.kind === "mavioDev"
        ? t("menu.serverCount", { count: shown.servers.length })
        : (shown.servers[0]?.name ?? ""),
  };
}

/** The address of the Mavio account's page at mavio.dev. */
export function useMavioDevAccountUrl(): string | undefined {
  const servers = useServersStore();
  const site = useSignIn()?.kind === "mavioDev" ? servers.mavioDevSite() : undefined;
  return site ? `${site}/account` : undefined;
}

/**
 * The users the shown standalone server's session may switch to
 * (docs/accounts.md §4.4), and switching to one.
 */
export function useSwitchUsers(server?: ServerEntry) {
  const servers = useServersStore();
  const queryClient = useQueryClient();
  const users = useQuery({
    queryKey: serverKey(server?.id ?? "", "switchable", server?.userId),
    queryFn: () => servers.switchableUsers(server!),
    enabled: !!server && server.mode !== "mavioDev",
    retry: false,
  });
  const switchTo = useMutation({
    mutationFn: ({ user, pin }: { user: SwitchableUser; pin?: string }) => servers.switchUser(server!, user.id, pin),
    // Everything the server answered was the previous user's.
    onSuccess: () => queryClient.removeQueries({ queryKey: serverKey(server!.id) }),
  });
  return { users: users.data && users.data.length > 1 ? users.data : [], switchTo };
}

/**
 * The button with the user's avatar and name, which opens the user menu;
 * on the television, three dots beside the tabs instead.
 */
export function UserButton({
  compact = false,
  dots = false,
  placement = "above",
}: {
  compact?: boolean;
  dots?: boolean;
  placement?: "above" | "below";
}) {
  const servers = useServersStore();
  const account = useAccount();
  const [open, setOpen] = useState(false);
  const { pathname } = useLocation();
  useEffect(() => setOpen(false), [pathname]);
  const { tv } = usePrefs();
  const full = useFullScreen();
  // Shortcuts are shown only where a keyboard can press them.
  const keys = useShortcutsShown();
  const shown = account.shown;
  const server = shown?.kind === "standalone" ? shown.servers[0] : undefined;
  const { users, switchTo } = useSwitchUsers(server);
  const [asking, setAsking] = useState<SwitchableUser>();
  const manage = useMavioDevAccountUrl();
  if (!shown) return null;
  const trigger = (
    <button
      type="button"
      data-tv-dots={dots || undefined}
      aria-label={t("nav.userMenu", { name: account.name })}
      className={
        dots
          ? "flex size-16 items-center justify-center rounded-full text-text-secondary"
          : `flex w-full items-center gap-2.5 rounded-[10px] text-left hover:bg-fill data-popup-open:bg-fill ${compact ? "justify-center p-1.5" : "p-2"}`
      }
    >
      {dots ? <Icon name="moreVertical" size={32} /> : <Avatar name={account.name} size={compact ? 36 : 32} />}
      {!compact && !dots && (
        <>
          <span className="min-w-0 flex-1">
            <span className="block truncate type-callout font-semibold">{account.name}</span>
            <span className="block truncate type-caption text-text-secondary">{account.line}</span>
          </span>
          <Icon name="selector" size={16} className="text-text-tertiary" />
        </>
      )}
    </button>
  );
  return (
    <>
      <Menu
        trigger={compact || dots ? <Tip label={account.name}>{trigger}</Tip> : trigger}
        side={placement === "above" ? "top" : "bottom"}
        align={placement === "above" ? "start" : "end"}
        width={tv ? "w-[400px]" : "w-[300px]"}
        label={t("nav.userMenu", { name: account.name })}
        open={open}
        onOpenChange={setOpen}
        start="[data-menu-start]"
      >
        <div className="flex items-center gap-3 px-2.5 py-2">
          <Avatar name={account.name} size={36} />
          <div className="min-w-0 flex-1">
            <div className="truncate type-headline">{account.name}</div>
            <div className="truncate type-footnote text-text-secondary">
              {shown.kind === "mavioDev" ? t("auth.mavioAccount") : t("auth.standaloneUser", { server: account.line })}
            </div>
          </div>
          <ThemeSwitch />
        </div>
        {users.length > 0 && (
          <>
            <MenuSeparator />
            <MenuLabel>{t("auth.switchUser")}</MenuLabel>
            {users.map((u) => (
              <MenuItem
                key={u.id}
                disabled={u.current || switchTo.isPending}
                closeOnClick={false}
                onClick={() => (u.hasPin ? (setOpen(false), setAsking(u)) : switchTo.mutate({ user: u }, { onSuccess: () => setOpen(false) }))}
                end={u.current ? <Icon name="check" size={16} className="text-accent-text" /> : u.hasPin && <Icon name="lock" size={14} className="text-text-tertiary" />}
              >
                <span className="flex items-center gap-2.5">
                  <Avatar name={u.name} size={24} />
                  <span className="min-w-0 flex-1 truncate">{u.name}</span>
                </span>
              </MenuItem>
            ))}
          </>
        )}
        <MenuSeparator />
        <MenuLink start icon="settings" to="/settings" shortcut={keys ? shortcutLabel("settings") : undefined}>
          {t("nav.settings")}
        </MenuLink>
        <MenuItem icon={tv ? "sidebar" : "television"} shortcut={keys ? shortcutLabel("tvMode") : undefined} onClick={() => setPrefs({ tv: !tv })}>
          {tv ? t("menu.standardMode") : t("menu.tvMode")}
        </MenuItem>
        {tv && (
          <MenuItem icon={full ? "exitFullScreen" : "fullScreen"} shortcut={keys ? shortcutLabel("fullScreen") : undefined} onClick={toggleFullScreen}>
            {full ? t("menu.exitFullScreen") : t("menu.fullScreen")}
          </MenuItem>
        )}
        {shown.kind === "mavioDev" ? (
          manage && (
            <MenuExternal icon="external" href={manage}>
              {t("menu.manageAccount")}
            </MenuExternal>
          )
        ) : (
          <MenuLink icon="server" to="/settings/$section" params={{ section: "servers" }}>
            {t("menu.manageServers")}
          </MenuLink>
        )}
        <MenuSeparator />
        <MenuItem icon="signOut" onClick={() => void servers.signOut()}>
          {t("auth.signOut")}
        </MenuItem>
      </Menu>
      {asking && (
        <PinPad
          name={asking.name}
          busy={switchTo.isPending}
          error={switchTo.error ? errorMessage(switchTo.error) : undefined}
          onCancel={() => (setAsking(undefined), switchTo.reset())}
          onPin={(pin) => switchTo.mutate({ user: asking, pin }, { onSuccess: () => setAsking(undefined) })}
        />
      )}
    </>
  );
}

const themes: { value: Appearance; icon: IconName; label: () => string }[] = [
  { value: "system", icon: "appearance", label: () => t("settings.system") },
  { value: "light", icon: "lightMode", label: () => t("settings.light") },
  { value: "dark", icon: "darkMode", label: () => t("settings.dark") },
];

/** The appearance, chosen beside the account's name: the system's, light or dark. */
function ThemeSwitch() {
  const { appearance } = usePrefs();
  return (
    <MenuChoice
      value={appearance}
      onChange={(v) => setPrefs({ appearance: v })}
      label={t("settings.appearance")}
      className="flex shrink-0 rounded-full bg-fill p-0.5"
    >
      {themes.map((th) => (
        <MenuIconOption key={th.value} value={th.value} icon={th.icon} label={th.label()} />
      ))}
    </MenuChoice>
  );
}

/** The dialog asking for a user's four-digit PIN, with a keypad and the keyboard. */
export function PinPad({
  name,
  busy,
  error,
  onPin,
  onCancel,
}: {
  name: string;
  busy: boolean;
  error?: string;
  onPin: (pin: string) => void;
  onCancel: () => void;
}) {
  const [pin, setPin] = useState("");
  const press = useCallback(
    (k: string) => {
      if (busy) return;
      const next = k === "⌫" ? pin.slice(0, -1) : (pin + k).slice(0, 4);
      setPin(next);
      if (next.length === 4 && pin.length < 4) onPin(next);
    },
    [busy, pin, onPin],
  );
  // A wrong PIN clears the dots for another try.
  useEffect(() => {
    if (error) setPin("");
  }, [error]);
  useEffect(() => {
    const key = (e: KeyboardEvent) => {
      if (/^\d$/.test(e.key)) press(e.key);
      else if (e.key === "Backspace") press("⌫");
      else return;
      e.preventDefault();
      e.stopImmediatePropagation();
    };
    document.addEventListener("keydown", key, true);
    return () => document.removeEventListener("keydown", key, true);
  }, [press]);
  return (
    <Sheet label={t("auth.enterPin", { name })} width="pin" className="p-6 text-center" onClose={onCancel}>
      <Avatar name={name} size={56} className="mx-auto" />
      <h2 className="mt-3 type-title3">{t("auth.enterPin", { name })}</h2>
      <div className="my-5 flex justify-center gap-4" aria-live="polite">
        {[0, 1, 2, 3].map((i) => (
          <span key={i} className={`h-3 w-3 rounded-full border-2 border-on-background ${i < pin.length ? "bg-on-background" : ""}`} />
        ))}
      </div>
      {error && <p className="-mt-2 mb-3 type-footnote text-danger">{error}</p>}
      <div className="grid grid-cols-3 gap-3">
        {["1", "2", "3", "4", "5", "6", "7", "8", "9", "", "0", "⌫"].map((k) =>
          k === "" ? (
            <span key="blank" />
          ) : (
            <button
              key={k}
              type="button"
              aria-label={k === "⌫" ? t("action.delete") : k}
              onClick={() => press(k)}
              className="mx-auto flex h-16 w-16 items-center justify-center rounded-full bg-fill type-title2 hover:bg-fill-selected active:opacity-70"
            >
              {k === "⌫" ? <Icon name="backspace" /> : k}
            </button>
          ),
        )}
      </div>
      <button type="button" onClick={onCancel} className="mt-4 type-body text-accent-text">
        {t("action.cancel")}
      </button>
    </Sheet>
  );
}
