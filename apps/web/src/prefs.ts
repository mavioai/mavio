// The settings of this browser (docs/clients.md §6.1, §6.3): appearance, TV
// mode, the sidebar, playback and the assistant. They are kept in localStorage, which may
// be unavailable; the defaults apply then.
import { breakpoints } from "@mavio/design/tokens";
import { useSyncExternalStore } from "react";

import type { Locale } from "@mavio/design/strings";

import { prefsKey, t } from "./i18n.ts";

/** The color theme: the system's, or one chosen. */
export type Appearance = "system" | "light" | "dark";

/** The highest bitrate streamed to this browser, by name. */
export type Quality = "original" | "auto" | "1080p" | "720p" | "480p";

/** The bitrate each quality allows, in bits per second. */
export const qualityBitrates: Record<Quality, number> = {
  original: 200_000_000,
  auto: 40_000_000,
  "1080p": 10_000_000,
  "720p": 4_000_000,
  "480p": 1_500_000,
};

/** The qualities in the order they are offered, with their names. */
export const qualities: { value: Quality; label: () => string }[] = [
  { value: "original", label: () => t("player.qualityOriginal") },
  { value: "auto", label: () => t("player.qualityAuto") },
  { value: "1080p", label: () => "1080p" },
  { value: "720p", label: () => "720p" },
  { value: "480p", label: () => "480p" },
];

export interface Prefs {
  appearance: Appearance;
  /** The UI's language, or the system's; read by i18n.ts as the page loads. */
  language: Locale | "system";
  /** The television layout, for a computer connected to a TV. */
  tv: boolean;
  /** Whether the desktop layout's sidebar is expanded, where the window is wide enough. */
  sidebar: boolean;
  /** The width of the expanded sidebars, of the app and the settings, in pixels; undefined for the layout's. */
  sidebarWidth?: number;
  quality: Quality;
  autoplayNext: boolean;
  /** Whether the assistant speaks its answers to spoken requests. */
  assistantSpeak: boolean;
  /** The voice speaking them: a server voice's ID, or "" for the browser's. */
  assistantVoice: string;
  /** Which recognizer the assistant tries first: the server's or the browser's. */
  assistantRecognition: "server" | "device";
}

const key = prefsKey;
const defaults: Prefs = { appearance: "system", language: "system", tv: false, sidebar: true, quality: "auto", autoplayNext: true, assistantSpeak: true, assistantVoice: "", assistantRecognition: "server" };

function read(): Prefs {
  try {
    return { ...defaults, ...(JSON.parse(localStorage.getItem(key) ?? "{}") as Partial<Prefs>) };
  } catch {
    return defaults;
  }
}

let current = read();
const listeners = new Set<() => void>();

function subscribe(l: () => void) {
  listeners.add(l);
  return () => listeners.delete(l);
}

/** This browser's settings. */
export function usePrefs(): Prefs {
  return useSyncExternalStore(subscribe, () => current);
}

/** The settings now, outside React. */
export function prefs(): Prefs {
  return current;
}

/** Changes some of this browser's settings. */
export function setPrefs(change: Partial<Prefs>) {
  current = { ...current, ...change };
  try {
    localStorage.setItem(key, JSON.stringify(current));
  } catch {
    // Kept for this page only.
  }
  applyTheme();
  for (const l of listeners) l();
}

/** The narrowest and widest the sidebars are dragged to, in pixels. */
export const sidebarWidths = { min: 200, max: 400 };

/**
 * Sets the root element's data-theme and data-tv, which tokens.css and the
 * styles read: television starts dark unless a theme is chosen; and the
 * width of the sidebars chosen, --mavio-sidebar-width.
 */
export function applyTheme() {
  const root = document.documentElement;
  const theme = current.appearance === "system" ? (current.tv ? "dark" : undefined) : current.appearance;
  if (theme) root.dataset.theme = theme;
  else delete root.dataset.theme;
  if (current.tv) root.dataset.tv = "";
  else delete root.dataset.tv;
  if (current.sidebarWidth) root.style.setProperty("--mavio-sidebar-width", `${current.sidebarWidth}px`);
  else root.style.removeProperty("--mavio-sidebar-width");
}

/** The layout a window's width chooses (docs/clients.md §6.3). */
export type WidthClass = "compact" | "medium" | "expanded";

const queries =
  typeof matchMedia === "undefined"
    ? undefined
    : {
        medium: matchMedia(`(min-width: ${breakpoints.tablet}px)`),
        expanded: matchMedia(`(min-width: ${breakpoints.expanded}px)`),
      };

function widthClass(): WidthClass {
  if (!queries) return "expanded";
  return queries.expanded.matches ? "expanded" : queries.medium.matches ? "medium" : "compact";
}

function subscribeWidth(l: () => void) {
  queries?.medium.addEventListener("change", l);
  queries?.expanded.addEventListener("change", l);
  return () => {
    queries?.medium.removeEventListener("change", l);
    queries?.expanded.removeEventListener("change", l);
  };
}

/** The width class of the window, which follows its resizing. */
export function useWidthClass(): WidthClass {
  return useSyncExternalStore(subscribeWidth, widthClass);
}
