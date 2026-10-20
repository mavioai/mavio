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
            case .conversations: ConversationsPage(context: context, server: server)
            default: nil
            }
        }
    }
#endif
