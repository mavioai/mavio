#if os(macOS)
    import AppKit
    import MavioMac

    /// The pages of Discover and of requests.
    @MainActor
    public enum DiscoverPages {
        /// Discover, the place of the sidebar.
        public static func discover(context: MacContext) -> NSViewController {
            DiscoverPage(context: context)
        }

        /// The page of route, nil when it is another area's.
        public static func page(for route: Route, context: MacContext) -> NSViewController? {
            switch route {
            case let .catalog(query):
                CatalogPage(context: context, query: query)
            case let .title(serverId, kind, tmdbId):
                TitlePage(context: context, serverId: serverId, kind: kind, tmdbId: tmdbId)
            case let .titleCredits(serverId, kind, tmdbId, crew):
                TitleCreditsPage(context: context, serverId: serverId, kind: kind, tmdbId: tmdbId, crew: crew)
            case let .discoveredPerson(serverId, tmdbId):
                DiscoveredPersonPage(context: context, serverId: serverId, tmdbId: tmdbId)
            case let .discoveredCollection(serverId, tmdbId):
                DiscoveredCollectionPage(context: context, serverId: serverId, tmdbId: tmdbId)
            default: nil
            }
        }
    }
#endif
