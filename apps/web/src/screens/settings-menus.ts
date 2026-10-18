// The menus of a server's settings: a second level holding the sections of
// one concern, apart from the settings themselves, which load as they are
// first opened, for the television's Back to find a section's menu; and the
// advanced tools, which the app's sidebar lists once pinned.
import { t } from "../i18n.ts";
import type { IconName } from "../icon.tsx";

/** A menu of the server's settings. */
export type MenuId = "media" | "acquiring" | "extensions" | "maintenance";

/** The sections of each menu, in their order. */
export const menuSections: Record<MenuId, readonly string[]> = {
  media: ["libraries", "storage", "transcoding"],
  acquiring: ["acquisition", "wanted", "downloads", "subtitles"],
  extensions: ["plugins", "ai", "conversations"],
  maintenance: ["activity", "issues", "tasks", "logs", "backups"],
};

export function isMenu(s: string | undefined): s is MenuId {
  return s !== undefined && Object.hasOwn(menuSections, s);
}

/** The menu holding a section, if any. */
export function menuOf(section: string): MenuId | undefined {
  return (Object.keys(menuSections) as MenuId[]).find((m) => menuSections[m].includes(section));
}

/**
 * An advanced tool: a section of the settings' Advanced group, which can be
 * pinned to the sidebar's Features group to open in the app's own layout.
 */
export type ToolId = "indexer-search";

/** The advanced tools, in their order. */
export const tools: readonly ToolId[] = ["indexer-search"];

/** How each tool shows, in the settings and in the sidebar. */
export const toolLooks: Record<ToolId, { icon: IconName; color: string; title: () => string }> = {
  "indexer-search": { icon: "indexers", color: "#0A84FF", title: () => t("indexers.title") },
};

export function isTool(s: string): s is ToolId {
  return (tools as readonly string[]).includes(s);
}
