// The web UI's client of the Mavio API (docs/clients.md §4.1): Connect
// clients over connect-es, one per server the user is signed in to, and
// the TanStack Query hooks the screens use.
export { backoff, follow, refreshes, useEvents } from "./events.ts";
export {
  type CatalogFilters,
  type CatalogRow,
  discoverRequestOf,
  filterCount,
  type NamedValue,
  rowsOf,
  titlesOf,
  useCatalog,
  useCatalogs,
  useCollection,
  useDiscoveredPerson,
  useDiscovery,
  useFilterValues,
  useRequestTitle,
  useSearchTitles,
  useSeason,
  useTitle,
  useTitleRatings,
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
  isNotFound,
  isTotpRequired,
  isUnauthenticated,
  type Filter,
  type ItemValue,
  libraryItemsRequest,
  type SearchResults,
  type ServerLibraries,
  ServersProvider,
  type Sort,
  serverKey,
  seriesNextUp,
  useChildren,
  useEpisodes,
  useExtras,
  useHome,
  useHubItems,
  useItem,
  useLibraries,
  useLibrary,
  useLibraryItems,
  useNextEpisode,
  useOtherCopies,
  usePrefetchItem,
  useRecommended,
  useSearch,
  useSearchAgain,
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
  useContainerRuntime,
  useContainers,
  usePluginActions,
  usePluginCatalogs,
  usePluginConfig,
  usePluginConfigOptions,
  usePluginContainers,
  usePluginLogs,
  usePlugins,
} from "./plugins.ts";
export {
  invitationLink,
  useAcquisitionSettings,
  useRequestSettings,
  useActivities,
  useAllSessions,
  useDirectory,
  useGeneralSettings,
  useInvitationActions,
  useInvitations,
  useJobs,
  useLibraryActions,
  useLocalization,
  useOutboundProxies,
  useRatings,
  useRunTask,
  useServerLibraries,
  useSystemInfo,
  useSystemLoad,
  useTasks,
  useTranscodingSettings,
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
  shareableUrl,
  type SignIn,
  type Storage,
} from "./servers.ts";
export {
  useDisks,
  useOfflineDownloads,
  useRemoteActions,
  useRemoteFolders,
  useRemoteUsage,
  useRemotes,
  useStorageProviders,
  useStorageStatus,
  useUpdateOfflineDownloads,
} from "./storage.ts";
export {
  useCustomFormats,
  useDelayProfiles,
  useDownloadClientPaths,
  useDownloadClients,
  useIndexers,
  useQualityDefinitions,
  useQualityProfiles,
  useReleaseProfiles,
  useUpdateDownloadClientPaths,
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
export { lyricLineAt, lyricSungLength, useLyrics } from "./lyrics.ts";
export {
  type AiExchange,
  isNoAiGateway,
  useAiActions,
  useAiConversation,
  useAiConversationActions,
  useAiConversations,
  useAiLogin,
  useAiModels,
  useAiProviders,
  useAiSend,
} from "./ai.ts";
export { useApiKeys, useBackupDownload, useBackups, useLogs } from "./backups.ts";
export { systemProxyId, useProxies, useTestProxy } from "./proxies.ts";
export { useQuota, useRequestActions, useRequests } from "./requests.ts";
export { usePerson, usePersonItems } from "./people.ts";
export { useRevokeSession, useSendCommand, useSessions } from "./remote.ts";
export { useExternalIdKinds, useMetadataActions, useRemoteImages, useRemoteSearch } from "./metadata.ts";
export { useCollectionActions, useCollections, usePlaylistActions, usePlaylistEntries, usePlaylists } from "./lists.ts";
export {
  useBlocklist,
  useDownloadActions,
  useGrab,
  useHistory,
  useManualImport,
  useQueue,
  type ReleaseSearch,
  useReleaseSearch,
  useReleases,
  useSendRelease,
  useWanted,
  useWantedActions,
  useWantedList,
} from "./acquisition.ts";
export { type ClockSample, clockSample, groupPositionAt, syncClock, useSyncPlayGroups } from "./syncplay.ts";
export { useCurrentUser, useIssue, useIssueActions, useIssues } from "./issues.ts";
export {
  ask,
  keptSpeech,
  speechFormats,
  startVoiceSession,
  useAssistant,
  useAssistantActions,
  useAssistantConversation,
  useAssistantConversations,
  useAssistantMemories,
  useAssistantSettings,
  useUpdateAssistantSettings,
} from "./assistant.ts";
export { Grammar, slotKeyword, slotLanguage, slotNumber, slotOf } from "./intent.ts";
