#if os(macOS)
    import AppKit
    import MavioMac

    /// The pages of a server's settings this area has.
    @MainActor
    public enum ServerPages {
        /// The page of section of server, nil when it is another area's.
        public static func page(for section: ServerSection, server: ServerRecord, context: MacContext)
            -> NSViewController?
        {
            switch section {
            case .overview: OverviewPage(context: context, server: server)
            case .general: GeneralPage(context: context, server: server)
            case .users: UsersPage(context: context, server: server)
            case .devices: DevicesPage(context: context, server: server)
            case .apiKeys: ApiKeysPage(context: context, server: server)
            case .libraries: LibrariesPage(context: context, server: server)
            case .storage: StoragePage(context: context, server: server)
            case .transcoding: TranscodingPage(context: context, server: server)
            case .activity: ActivityPage(context: context, server: server)
            case .issues: IssuesPage(context: context, server: server)
            case .tasks: TasksPage(context: context, server: server)
            case .logs: LogsPage(context: context, server: server)
            case .backups: BackupsPage(context: context, server: server)
            default: nil
            }
        }
    }
#endif
