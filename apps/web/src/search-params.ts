// The search parameters of the routes, which the router parses as JSON: a
// value that reads as a number or a boolean, such as a remote named 115 in a
// typed address, comes as one.

/** A text search parameter, taken back as text when the router parsed it as a number or a boolean; undefined when empty. */
export function searchText(v: unknown): string | undefined {
  if (typeof v === "number" || typeof v === "boolean") return String(v);
  return typeof v === "string" && v !== "" ? v : undefined;
}

/** Where the settings are: the server chosen, and within Plugins, the plugin or the catalogs shown. */
export interface SettingsSearch {
  server?: string;
  /** Plugins: the tab of installed plugins rather than of those available. */
  tab?: "installed";
  /** Plugins: the category shown, a Category's number. */
  category?: number;
  /** Plugins: the plugin shown. */
  plugin?: string;
  /** Plugins: the catalogs shown. */
  catalogs?: true;
  /** Libraries: the library shown, "new" for one to add. */
  library?: string;
  /** Users: the user shown, "new" for one to add. */
  user?: string;
  /** Storage: the remote shown, "new" for one to add. */
  remote?: string;
  /** Acquisition: the page shown, and the profile or format on it. */
  acquisition?: string;
  /** Subtitles: the page shown, and the profile on it. */
  subtitles?: string;
  id?: string;
  /** A page's tab or kind shown, as Downloads' tab, when not its first. */
  view?: string;
  /** A page's filter, as Wanted's or Issues', when not its first. */
  filter?: string;
}

/** The settings' search parameters, from the address. */
export function settingsSearch(s: Record<string, unknown>): SettingsSearch {
  const str = searchText;
  return {
    server: str(s.server),
    tab: s.tab === "installed" ? "installed" : undefined,
    category: typeof s.category === "number" && Number.isInteger(s.category) ? s.category : undefined,
    plugin: str(s.plugin),
    catalogs: s.catalogs === true ? true : undefined,
    library: str(s.library),
    user: str(s.user),
    remote: str(s.remote),
    acquisition: str(s.acquisition),
    subtitles: str(s.subtitles),
    id: str(s.id),
    view: str(s.view),
    filter: str(s.filter),
  };
}
