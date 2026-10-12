import MavioDesign
import MavioKit
import SwiftUI

/// A titled horizontal row of cards.
public struct Shelf<Accessory: View, Content: View>: View {
    private let title: String
    private let accessory: Accessory
    private let content: Content
    @Environment(\.theme) private var theme

    /// A row titled title, with an accessory after the title (badges, the
    /// more action) and the cards of content.
    public init(_ title: String, @ViewBuilder accessory: () -> Accessory, @ViewBuilder content: () -> Content) {
        self.title = title
        self.accessory = accessory()
        self.content = content()
    }

    /// The title above the scrolling cards.
    public var body: some View {
        VStack(alignment: .leading, spacing: MavioSpacing.s8) {
            HStack(spacing: MavioSpacing.s8) {
                Text(title).type(theme.type.title3).lineLimit(1)
                    .foregroundStyle(theme.colors.textPrimary)
                accessory
            }
            .padding(.horizontal, theme.layout.margin)
            ScrollView(.horizontal) {
                LazyHStack(alignment: .top, spacing: theme.layout.gutter) {
                    content
                }
                .scrollTargetLayout()
            }
            .contentMargins(.horizontal, theme.layout.margin, for: .scrollContent)
            .scrollTargetBehavior(.viewAligned)
            .scrollIndicators(.hidden)
        }
        .padding(.vertical, MavioSpacing.s8)
    }
}

extension Shelf where Accessory == EmptyView {
    /// A row titled title with the cards of content.
    public init(_ title: String, @ViewBuilder content: () -> Content) {
        self.init(title, accessory: { EmptyView() }, content: content)
    }
}

/// The empty, error and offline states of a screen.
public struct StatusView: View {
    private let icon: String?
    private let title: String
    private let message: String?
    private let action: (label: String, run: () -> Void)?
    @Environment(\.theme) private var theme

    /// A state with an icon, a title, a message and an action.
    public init(
        icon: String? = nil, title: String, message: String? = nil, action: (label: String, run: () -> Void)? = nil
    ) {
        self.icon = icon
        self.title = title
        self.message = message
        self.action = action
    }

    /// The state, centered.
    public var body: some View {
        VStack(spacing: MavioSpacing.s12) {
            if let icon { Icon(icon, size: 48).foregroundStyle(theme.colors.textTertiary) }
            Text(title).type(theme.type.title3).foregroundStyle(theme.colors.textPrimary)
            if let message {
                Text(message).type(theme.type.body).foregroundStyle(theme.colors.textSecondary)
                    .multilineTextAlignment(.center)
            }
            if let action {
                Button(action.label, action: action.run).buttonStyle(.bordered)
            }
        }
        .padding(theme.layout.margin)
        .frame(maxWidth: .infinity, maxHeight: .infinity)
    }
}

/// The title of a row of Home or of a library's Recommended tab, named by
/// its kind, library and seed.
public func rowTitle(_ row: Row) -> String {
    let library = row.libraryName ?? ""
    let seed = row.seedName ?? ""
    switch row.kind {
    case .continueWatching: return MavioStrings.homeContinueWatching
    case .availableRequests: return MavioStrings.homeAvailableRequests
    case .recentlyAdded: return MavioStrings.homeLatest(library: library)
    case .recentlyReleased: return MavioStrings.homeRecentlyReleased(library: library)
    case .recentlyAired: return MavioStrings.homeRecentlyAired(library: library)
    case .topRated: return MavioStrings.homeTopRated(library: library)
    case .topUnplayed: return MavioStrings.homeTopUnplayed(library: library)
    case .startWatching: return MavioStrings.homeStartWatching(library: library)
    case .rediscover: return MavioStrings.homeRediscover(library: library)
    case .becauseYouWatched: return MavioStrings.homeBecauseYouWatched(title: seed)
    case .becauseYouLike: return MavioStrings.homeBecauseYouLike(title: seed)
    case .moreInGenre: return MavioStrings.homeMoreInGenre(genre: seed)
    case .starring: return MavioStrings.homeStarring(name: seed)
    case .directedBy: return MavioStrings.homeDirectedBy(name: seed)
    case .trending: return MavioStrings.homeTrending
    case .server: return row.serverName ?? ""
    }
}
