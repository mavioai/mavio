#if os(macOS)
    import AppKit
    import MavioMac

    /// The player's window content.
    @MainActor
    public enum PlayerPages {
        /// The player of request; close closes its window. It draws dark,
        /// as the picture does, and stops the playback as it leaves the
        /// window.
        public static func player(
            context: MacContext, request: PlayRequest, close: @escaping @MainActor () -> Void
        ) -> NSViewController {
            var theme = context.theme
            theme.colors = .dark
            return PlayerPage(context: context.with(theme: theme), request: request, close: close)
        }

        /// What a player's window content plays, nil for other content.
        public static func playing(_ content: NSViewController?) -> PlayRequest? {
            (content as? PlayerPage)?.playing
        }
    }
#endif
