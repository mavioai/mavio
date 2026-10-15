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
export {
  invitationLink,
  useActivities,
  useAllSessions,
  useDirectory,
  useInvitationActions,
  useInvitations,
  useJobs,
  useLibraryActions,
  useLocalization,
  useRatings,
  useRunTask,
  useServerLibraries,
  useServerSettings,
  useSystemInfo,
  useTasks,
  useUpdateServerSettings,
  useUserActions,
  useUsers,
} from "./admin.ts";
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
export {
  useOfflineClients,
  useRemoteActions,
  useRemoteFolders,
  useRemotes,
  useSetOfflineClients,
  useStorageProviders,
  useStorageStatus,
} from "./storage.ts";
export {
  useCustomFormats,
  useDelayProfiles,
  useDownloadClients,
  useIndexers,
  useQualityDefinitions,
  useQualityProfiles,
  useReleaseProfiles,
  useRemotePathMappings,
} from "./profiles.ts";
export {
  useItemSubtitleActions,
  useItemSubtitles,
  useSubtitleAssignments,
  useSubtitleProfiles,
  useSubtitleSearch,
  useSubtitleProviders,
  useSubtitleSettings,
  useSubtitleTranslators,
} from "./subtitles.ts";
export { isNoAiGateway, useAiActions, useAiLogin, useAiModels, useAiProviders } from "./ai.ts";
export { useApiKeys, useBackupDownload, useBackups, useLogs } from "./backups.ts";
export { useQuota, useRequestActions, useRequests } from "./requests.ts";
export { usePerson, usePersonItems } from "./people.ts";
export { useSendCommand, useSessions } from "./remote.ts";
export { useExternalIdKinds, useMetadataActions, useRemoteImages, useRemoteSearch } from "./metadata.ts";
export { useCollectionActions, useCollections, usePlaylistActions, usePlaylistEntries, usePlaylists } from "./lists.ts";
export {
  useBlocklist,
  useDownloadActions,
  useGrab,
  useHistory,
  useManualImport,
  useQueue,
  useReleases,
  useWanted,
  useWantedActions,
  useWantedList,
} from "./acquisition.ts";
export { type ClockSample, clockSample, groupPositionAt, syncClock, useSyncPlayGroups } from "./syncplay.ts";
export { useCurrentUser, useIssue, useIssueActions, useIssues } from "./issues.ts";
