#if os(macOS)
    import AppKit
    @_exported import MavioAppKit
    @_exported import MavioCommon
    @_exported import MavioKit
    @_exported import MavioDesign

    /// What every page of the Mac app is given: the model, the discovery of
    /// titles, the theme it draws with, and how it opens a route, plays an
    /// item or opens the settings — what the SwiftUI screens took from the
    /// environment.
    @MainActor
    public final class MacContext {
        /// The servers, the user and Home.
        public let model: AppModel
        /// The server that discovers titles, and its catalogs.
        public let discovery: Discovery
        /// The theme: the desktop's, or television mode's.
        public let theme: Theme
        /// Plays an item, in a player window or full screen.
        public let play: @MainActor (PlayRequest) -> Void
        /// The assistant over the app's windows.
        public var assistant: Assistant?
        /// Opens the sheet adding a server.
        public var addServer: @MainActor () -> Void = {}
        /// Opens the settings, on the account's page when account.
        public var openSettings: @MainActor (_ account: Bool) -> Void = { _ in }
        /// Shows the remote control of a session of a server.
        public var openRemote: @MainActor (_ server: ServerRecord, _ sessionId: String) -> Void = { _, _ in }
        /// The page of a route, which the shell sets.
        public var page: @MainActor (Route, MacContext) -> NSViewController = { route, context in
            PendingPage(context: context, name: "\(route)")
        }
        /// The page of a section of a server's settings, which the shell
        /// sets.
        public var sectionPage: @MainActor (ServerSection, ServerRecord, MacContext) -> NSViewController = {
            section, _, context in PendingPage(context: context, name: section.title)
        }

        /// Whether the sidebar shows a search field, which the Search page
        /// then does not; posts `sidebarSearchesDidChange` as it changes.
        public var sidebarSearches = false {
            didSet {
                if oldValue != sidebarSearches {
                    NotificationCenter.default.post(name: Self.sidebarSearchesDidChange, object: self)
                }
            }
        }
        /// Posted as whether the sidebar shows a search field changes.
        public static let sidebarSearchesDidChange = Notification.Name("MavioSidebarSearchesDidChange")

        /// A context of model and discovery, in theme.
        public init(
            model: AppModel, discovery: Discovery, theme: Theme, play: @escaping @MainActor (PlayRequest) -> Void
        ) {
            self.model = model
            self.discovery = discovery
            self.theme = theme
            self.play = play
        }

        /// A context alike, in another theme.
        public func with(theme: Theme) -> MacContext {
            let context = MacContext(model: model, discovery: discovery, theme: theme, play: play)
            context.addServer = addServer
            context.assistant = assistant
            context.openSettings = openSettings
            context.openRemote = openRemote
            context.page = page
            context.sectionPage = sectionPage
            return context
        }

        /// Opens route over the page from: pushed on its stack.
        public func open(_ route: Route, from page: NSViewController) {
            page.navigation?.push(self.page(route, self))
        }
    }

    /// A page that searches a query it is given: Search, given the query
    /// confirmed in the sidebar's field.
    @MainActor
    public protocol QuerySearching: AnyObject {
        /// Searches query, again when it is the one searched.
        func search(query: String)
    }

    /// A page of the Mac app: a screen controller with its context.
    open class Page: ScreenController {
        /// What the page shows, as the assistant's requests tell it; nil
        /// for the place's.
        open var assistantScreen: AssistantScreen? { nil }

        /// The context.
        public let context: MacContext
        /// The model.
        public var model: AppModel { context.model }

        /// A page in context.
        public init(context: MacContext) {
            self.context = context
            super.init(theme: context.theme)
        }

        /// Opens route over this page.
        public func open(_ route: Route) {
            context.open(route, from: self)
        }

        /// Opens a page over this page.
        public func push(_ page: NSViewController) {
            navigation?.push(page)
        }
    }

    /// A page not yet in AppKit, which says what it is.
    public final class PendingPage: Page {
        private let name: String

        /// The page of name.
        public init(context: MacContext, name: String) {
            self.name = name
            super.init(context: context)
        }

        override public func viewDidLoad() {
            super.viewDidLoad()
            show(StatusView(icon: "info", title: name, theme: theme))
        }
    }
#endif
