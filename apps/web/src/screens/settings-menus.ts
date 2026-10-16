// The menus of a server's settings: a second level holding the sections of
// one concern, apart from the settings themselves, which load as they are
// first opened, for the television's Back to find a section's menu.

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
