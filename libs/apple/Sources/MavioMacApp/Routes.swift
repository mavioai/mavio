#if os(macOS)
    import AppKit
    import MavioMac
    import MavioMacAcquisition
    import MavioMacDiscover
    import MavioMacMedia
    import MavioMacPlugins
    import MavioMacServer
    import MavioMacSettings

    /// The page of every route and of every section of a server's settings,
    /// from the area that has it.
    @MainActor
    enum Routes {
        static func page(_ route: Route, _ context: MacContext) -> NSViewController {
            MediaPages.page(for: route, context: context)
                ?? DiscoverPages.page(for: route, context: context)
                ?? PendingPage(context: context, name: "\(route)")
        }

        static func section(_ section: ServerSection, _ server: ServerRecord, _ context: MacContext)
            -> NSViewController
        {
            ServerPages.page(for: section, server: server, context: context)
                ?? AcquisitionPages.page(for: section, server: server, context: context)
                ?? PluginsPages.page(for: section, server: server, context: context)
                ?? PendingPage(context: context, name: section.title)
        }

        static func remote(_ context: MacContext, _ server: ServerRecord, _ sessionId: String) -> NSViewController {
            SettingsPages.remote(context: context, server: server, sessionId: sessionId)
        }

        static func signIn(_ context: MacContext, adding: Bool, done: @escaping @MainActor () -> Void)
            -> NSViewController
        {
            SettingsPages.signIn(context: context, adding: adding, done: done)
        }

        static func settings(_ context: MacContext, account: Bool, close: @escaping @MainActor () -> Void)
            -> NSViewController
        {
            SettingsPages.settings(context: context, account: account, close: close)
        }

        static func home(_ context: MacContext) -> NSViewController { MediaPages.home(context: context) }

        static func libraries(_ context: MacContext) -> NSViewController { MediaPages.libraries(context: context) }

        static func discover(_ context: MacContext) -> NSViewController { DiscoverPages.discover(context: context) }

        /// Gives context the pages.
        static func wire(_ context: MacContext) {
            context.page = page
            context.sectionPage = section
        }
    }
#endif
