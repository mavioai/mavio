// The user signed in and the menu it opens (docs/clients.md §6.3):
// switching to another user of the server, the settings, the account at
// mavio.dev or the servers, and signing out.
import { errorMessage, type ServerEntry, serverKey, type SignIn, useServersStore, useSignIn } from "@mavio/client-ts";
import type { SwitchableUser } from "@mavio/proto/mavio/auth/v1/auth_pb";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Link, useLocation } from "@tanstack/react-router";
import { type ReactNode, useCallback, useEffect, useRef, useState } from "react";
import { createPortal } from "react-dom";

import { toggleFullScreen, useFullScreen } from "../fullscreen.ts";
import { t } from "../i18n.ts";
import { Icon, type IconName } from "../icon.tsx";
import { shortcutLabel, useShortcutsShown } from "../keys.ts";
import { type Appearance, setPrefs, usePrefs } from "../prefs.ts";
import { Avatar, useDismiss } from "../ui.tsx";

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
  const account = useAccount();
  const [open, setOpen] = useState(false);
  const ref = useRef<HTMLDivElement>(null);
  const close = useCallback(() => setOpen(false), []);
  useDismiss(ref, open, close);
  const { pathname } = useLocation();
  useEffect(close, [pathname, close]);
  if (!account.shown) return null;
  return (
    <div ref={ref} data-focus-scope className="relative">
      <button
        type="button"
        aria-haspopup="menu"
        aria-expanded={open}
        data-tv-dots={dots || undefined}
        aria-label={t("nav.userMenu", { name: account.name })}
        title={compact || dots ? account.name : undefined}
        onClick={() => setOpen(!open)}
        className={
          dots
            ? "flex size-16 items-center justify-center rounded-full text-text-secondary"
            : `flex w-full items-center gap-2.5 rounded-[10px] text-left hover:bg-fill ${compact ? "justify-center p-1.5" : "p-2"} ${open ? "bg-fill" : ""}`
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
      {open && <UserMenu placement={placement} onClose={close} />}
    </div>
  );
}

/** The glass menu of the user. */
function UserMenu({ placement, onClose }: { placement: "above" | "below"; onClose: () => void }) {
  const servers = useServersStore();
  const account = useAccount();
  const shown = account.shown!;
  const { tv } = usePrefs();
  const full = useFullScreen();
  // Shortcuts are shown only where a keyboard can press them.
  const keys = useShortcutsShown();
  const server = shown.kind === "standalone" ? shown.servers[0] : undefined;
  const { users, switchTo } = useSwitchUsers(server);
  const [asking, setAsking] = useState<SwitchableUser>();
  const manage = useMavioDevAccountUrl();
  // On the television the menu opens with focus on the settings.
  const ref = useRef<HTMLDivElement>(null);
  useEffect(() => {
    if (tv) ref.current?.querySelector<HTMLElement>("[data-menu-start]")?.focus();
  }, [tv]);

  return (
    <div
      ref={ref}
      role="menu"
      className={`absolute z-50 rounded-[14px] border border-border p-1.5 shadow-level3 ${
        // On the television, frosted glass over the artwork behind it.
        tv ? "w-[400px] bg-surface-overlay/45 backdrop-blur-3xl backdrop-saturate-150" : "w-[300px] bg-surface-overlay/85 backdrop-blur-xl"
      } ${
        placement === "above" ? "bottom-full left-0 mb-2" : "top-full right-0 mt-2"
      }`}
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
          <Divider />
          <div className="px-2.5 pt-1.5 pb-1 type-caption text-text-secondary">{t("auth.switchUser")}</div>
          {users.map((u) => (
            <button
              key={u.id}
              type="button"
              role="menuitem"
              disabled={u.current || switchTo.isPending}
              onClick={() => (u.hasPin ? setAsking(u) : switchTo.mutate({ user: u }, { onSuccess: onClose }))}
              className="flex min-h-10 w-full items-center gap-2.5 rounded-lg px-2.5 text-left type-body hover:bg-fill disabled:hover:bg-transparent"
            >
              <Avatar name={u.name} size={24} />
              <span className="min-w-0 flex-1 truncate">{u.name}</span>
              {u.current ? (
                <Icon name="check" size={16} className="text-accent-text" />
              ) : (
                u.hasPin && <Icon name="lock" size={14} className="text-text-tertiary" />
              )}
            </button>
          ))}
        </>
      )}
      <Divider />
      <MenuLink start icon="settings" to="/settings" onClose={onClose} shortcut={keys ? shortcutLabel("settings") : undefined}>
        {t("nav.settings")}
      </MenuLink>
      <button
        type="button"
        role="menuitem"
        onClick={() => {
          onClose();
          setPrefs({ tv: !tv });
        }}
        className={itemClass}
      >
        <Icon name={tv ? "sidebar" : "television"} size={18} className="text-text-secondary" />
        <span className="flex-1">{tv ? t("menu.standardMode") : t("menu.tvMode")}</span>
        {keys && <Shortcut>{shortcutLabel("tvMode")}</Shortcut>}
      </button>
      {tv && (
        <button type="button" role="menuitem" onClick={() => (onClose(), toggleFullScreen())} className={itemClass}>
          <Icon name={full ? "exitFullScreen" : "fullScreen"} size={18} className="text-text-secondary" />
          <span className="flex-1">{full ? t("menu.exitFullScreen") : t("menu.fullScreen")}</span>
          {keys && <Shortcut>{shortcutLabel("fullScreen")}</Shortcut>}
        </button>
      )}
      {shown.kind === "mavioDev" ? (
        manage && (
          <a role="menuitem" href={manage} target="_blank" rel="noreferrer" onClick={onClose} className={itemClass}>
            <Icon name="external" size={18} className="text-text-secondary" />
            <span className="flex-1">{t("menu.manageAccount")}</span>
          </a>
        )
      ) : (
        <MenuLink icon="server" to="/settings/$section" section="servers" onClose={onClose}>
          {t("menu.manageServers")}
        </MenuLink>
      )}
      <Divider />
      <button
        type="button"
        role="menuitem"
        onClick={() => {
          onClose();
          void servers.signOut();
        }}
        className={itemClass}
      >
        <Icon name="signOut" size={18} className="text-text-secondary" />
        <span className="flex-1">{t("auth.signOut")}</span>
      </button>
      {asking && (
        <PinPad
          name={asking.name}
          busy={switchTo.isPending}
          error={switchTo.error ? errorMessage(switchTo.error) : undefined}
          onCancel={() => (setAsking(undefined), switchTo.reset())}
          onPin={(pin) => switchTo.mutate({ user: asking, pin }, { onSuccess: () => (setAsking(undefined), onClose()) })}
        />
      )}
    </div>
  );
}

const themes: { value: Appearance; icon: IconName; label: () => string }[] = [
  { value: "system", icon: "appearance", label: () => t("settings.system") },
  { value: "light", icon: "lightMode", label: () => t("settings.light") },
  { value: "dark", icon: "darkMode", label: () => t("settings.dark") },
];

/** The appearance, chosen beside the account's name: the system's, light or dark. */
function ThemeSwitch() {
  const { appearance, tv } = usePrefs();
  return (
    <div role="radiogroup" aria-label={t("settings.appearance")} className="flex shrink-0 rounded-full bg-fill p-0.5">
      {themes.map((th) => {
        const on = appearance === th.value;
        return (
          <button
            key={th.value}
            type="button"
            role="radio"
            aria-checked={on}
            aria-label={th.label()}
            title={th.label()}
            onClick={() => setPrefs({ appearance: th.value })}
            className={`flex ${tv ? "size-9" : "size-7"} items-center justify-center rounded-full ${
              on ? "bg-surface-raised text-on-background shadow-level1" : "text-text-secondary hover:text-on-background"
            }`}
          >
            <Icon name={th.icon} fill={on} size={tv ? 20 : 16} />
          </button>
        );
      })}
    </div>
  );
}

/** The keys of a menu item's shortcut, at its end. */
function Shortcut({ children }: { children: string }) {
  return <kbd className="font-sans type-footnote text-text-tertiary">{children}</kbd>;
}

const itemClass = "flex min-h-10 w-full items-center gap-2.5 rounded-lg px-2.5 text-left type-body hover:bg-fill";

function Divider() {
  return <div className="mx-2.5 my-1 h-px bg-border" />;
}

function MenuLink({
  start = false,
  icon,
  to,
  section,
  shortcut,
  onClose,
  children,
}: {
  start?: boolean;
  icon: IconName;
  to: "/settings" | "/settings/$section";
  section?: string;
  shortcut?: string;
  onClose: () => void;
  children: ReactNode;
}) {
  return (
    <Link role="menuitem" data-menu-start={start || undefined} to={to} params={section ? { section } : undefined} onClick={onClose} className={itemClass}>
      <Icon name={icon} size={18} className="text-text-secondary" />
      <span className="flex-1">{children}</span>
      {shortcut && <Shortcut>{shortcut}</Shortcut>}
    </Link>
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
      else if (e.key === "Escape") onCancel();
      else return;
      e.preventDefault();
      e.stopImmediatePropagation();
    };
    document.addEventListener("keydown", key, true);
    return () => document.removeEventListener("keydown", key, true);
  }, [press, onCancel]);
  // In the body, outside the menu: its backdrop filter would hold a fixed
  // element, and a press on the keypad must not close the menu.
  return createPortal(
    <div
      className="fixed inset-0 z-[60] flex items-center justify-center bg-scrim backdrop-blur-sm"
      onPointerDown={(e) => {
        e.stopPropagation();
        if (e.target === e.currentTarget) onCancel();
      }}
    >
      <div role="dialog" aria-modal aria-label={t("auth.enterPin", { name })} className="w-[300px] rounded-[22px] bg-surface p-6 text-center shadow-level3">
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
      </div>
    </div>,
    document.body,
  );
}
