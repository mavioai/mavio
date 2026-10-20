#if os(macOS)
    import AppKit
    import MavioMac

    /// The pages of a server's media: Home, the libraries, an item's
    /// details, a library, a hub, credits, people, search and downloads.
    @MainActor
    public enum MediaPages {
        /// Home.
        public static func home(context: MacContext) -> NSViewController { HomePage(context: context) }

        /// Every server's libraries, as television mode's Libraries tab
        /// shows them.
        public static func libraries(context: MacContext) -> NSViewController { LibrariesPage(context: context) }

        /// The page of route, nil when it is another area's.
        public static func page(for route: Route, context: MacContext) -> NSViewController? {
            switch route {
            case let .item(serverId, itemId):
                DetailsPage(serverId: serverId, itemId: itemId, context: context)
            case let .library(library):
                LibraryPage(library: library, context: context)
            case let .hub(serverId, hubId, title):
                HubPage(serverId: serverId, hubId: hubId, title: title, context: context)
            case let .credits(serverId, itemId, crew):
                CreditsPage(serverId: serverId, itemId: itemId, crew: crew, context: context)
            case let .person(serverId, personId):
                PersonPage(serverId: serverId, personId: personId, context: context)
            case let .filteredLibrary(serverId, libraryId, filter):
                LibraryPage(serverId: serverId, libraryId: libraryId, filter: filter, context: context)
            case .search:
                SearchPage(context: context)
            case .downloads:
                DownloadsPage(context: context)
            default:
                nil
            }
        }
    }
#endif
