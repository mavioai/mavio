// The web UI's client of the Mavio API (docs/clients.md §4.1): Connect
// clients over connect-es, one per server the user is signed in to, and
// the TanStack Query hooks the screens use.
export { backoff, follow, refreshes, useEvents } from "./events.ts";
export {
  type CatalogRow,
  rowsOf,
  titlesOf,
  useCatalog,
  useDiscovery,
  useRequestTitle,
  useTitle,
} from "./discover.ts";
export {
  type Content,
  type Entry,
  type Hubs,
  merge,
  type Section,
  type ServerHubs,
  type Trending,
  type Version,
} from "./home.ts";
export {
  type Copy,
  errorMessage,
  isTotpRequired,
  isUnauthenticated,
  type Filter,
  libraryItemsRequest,
  type SearchResults,
  type ServerLibraries,
  ServersProvider,
  type Sort,
  serverKey,
  seriesNextUp,
  useChildren,
  useEpisodes,
  useHome,
  useHubItems,
  useItem,
  useLibraries,
  useLibrary,
  useLibraryItems,
  useNextEpisode,
  useOtherCopies,
  useRecommended,
  useSearch,
  useServer,
  useServers,
  useServersStore,
  useSignedIn,
  useSignIn,
  useSimilar,
  useUserData,
} from "./hooks.ts";
export {
  useAdminServers,
  useCatalogPlugins,
  usePluginActions,
  usePluginCatalogs,
  usePluginConfig,
  usePlugins,
} from "./plugins.ts";
export { useDirectory, useLibraryActions, useLocalization, useServerLibraries } from "./admin.ts";
export { type Artwork, artwork, imageOf, imageUrl, libraryImageUrl, thumbOf } from "./images.ts";
export {
  browserDecoders,
  capabilities,
  type Decoders,
  nativeHls,
  Playback,
  type PlayOptions,
} from "./playback.ts";
export {
  assertionMessage,
  browserDeviceKey,
  type DeviceKey,
  memoryDeviceKey,
  refreshMessage,
  renewalMessage,
} from "./device-key.ts";
export { type Server, type Session, transport, urlOf } from "./server.ts";
export {
  type Account,
  type LinkCode,
  normalizeUrl,
  type Options,
  type QuickConnect,
  type ServerEntry,
  type ServerInfo,
  Servers,
  type SignIn,
  type Storage,
} from "./servers.ts";
