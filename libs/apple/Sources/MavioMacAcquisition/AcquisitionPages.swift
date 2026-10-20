#if os(macOS)
    import AppKit
    import MavioMac

    /// The pages of a server's settings this area has.
    @MainActor
    public enum AcquisitionPages {
        /// The page of section of server, nil when it is another area's.
        public static func page(for section: ServerSection, server: ServerRecord, context: MacContext)
            -> NSViewController?
        {
            switch section {
            case .acquisition: AcquisitionPage(context: context, server: server)
            case .wanted: WantedPage(context: context, server: server)
            case .downloads: DownloadsPage(context: context, server: server)
            case .subtitles: SubtitlesPage(context: context, server: server)
            case .indexerSearch: IndexerSearchPage(context: context, server: server)
            case .proxies: ProxiesPage(context: context, server: server)
            default: nil
            }
        }
    }
#endif
