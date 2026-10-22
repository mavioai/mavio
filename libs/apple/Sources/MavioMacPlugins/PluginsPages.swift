#if os(macOS)
    import AppKit
    import MavioMac

    /// The pages of a server's settings this area has.
    @MainActor
    public enum PluginsPages {
        /// The page of section of server, nil when it is another area's.
        public static func page(for section: ServerSection, server: ServerRecord, context: MacContext)
            -> NSViewController?
        {
            switch section {
            case .ai: AIPage(context: context, server: server)
            case .plugins: PluginsPage(context: context, server: server)
            default: nil
            }
        }

        /// The page of the plugin of server whose ID is id.
        public static func plugin(_ id: String, server: ServerRecord, context: MacContext) -> NSViewController {
            PluginPage(context: context, store: PluginsStore(server: server, model: context.model), id: id)
        }

        /// The Assistant settings of the user on server.
        public static func assistant(server: ServerRecord?, context: MacContext) -> NSViewController {
            AssistantPrefsPage(context: context, server: server)
        }
    }
#endif
