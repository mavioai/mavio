#if os(macOS)
    import AppKit

    extension Page {
        /// A card opening an item's details, with its actions in a context
        /// menu.
        public func itemCard(_ card: Card, wide: Bool? = nil, width: CGFloat? = nil) -> CardView {
            let view = CardView.item(card, wide: wide, width: width, theme: theme)
            view.onPress = { [weak self] in self?.open(.item(serverId: card.serverId, itemId: card.itemId)) }
            view.contextMenu = { [weak self] in self?.cardMenu(card) }
            return view
        }

        /// The menu of a card: marking it played or not, and as a favorite.
        public func cardMenu(_ card: Card) -> NSMenu {
            let menu = NSMenu()
            let model = model
            menu.addItem(
                MenuAction(card.played ? MavioStrings.itemMarkUnplayed : MavioStrings.itemMarkPlayed) {
                    Task { try? await model.setPlayed(card, !card.played) }
                })
            let favorite = MenuAction(MavioStrings.itemFavorite) {
                Task { try? await model.setFavorite(card, !card.favorite) }
            }
            favorite.state = card.favorite ? .on : .off
            menu.addItem(favorite)
            return menu
        }

        /// A title discovery offers, opening the copy a server has, else
        /// the title as serverId discovers it.
        public func titleCard(_ title: TitleCard, serverId: String?) -> CardView {
            let view = CardView(width: theme.layout.poster, aspect: 2 / 3, theme: theme)
            view.artwork.set(title.posterUrl, title: title.title)
            view.setText(title.title, title.year.map(String.init) ?? " ")
            if let copy = title.available.first {
                view.onPress = { [weak self] in self?.open(.item(serverId: copy.serverId, itemId: copy.itemId)) }
            } else if let serverId, let tmdbId = title.externalIds["tmdb"].flatMap({ Int32($0) }) {
                view.onPress = { [weak self] in
                    self?.open(.title(serverId: serverId, kind: title.movie ? .movie : .series, tmdbId: tmdbId))
                }
            }
            return view
        }

        /// A row of Home or of a library's Recommended tab; several says
        /// the rows come from several servers, which the badges name.
        public func rowView(_ row: Row, several: Bool) -> NSView {
            switch row.content {
            case let .unavailable(message):
                let label = NSStackView.horizontal(
                    [
                        IconView("offline", size: 18, color: theme.colors.textTertiary),
                        NSTextField.label(
                            MavioStrings.homeServerUnreachable(server: row.serverName ?? ""), theme.fonts.callout,
                            color: theme.colors.textTertiary),
                    ], spacing: MavioSpacing.s8)
                label.toolTip = message
                let holder = NSView()
                holder.translatesAutoresizingMaskIntoConstraints = false
                holder.addFilling(
                    label, insets: .symmetric(horizontal: theme.layout.margin, vertical: MavioSpacing.s12))
                return holder
            case .loading:
                let shelf = Shelf(row.serverName ?? "", theme: theme)
                let theme = theme
                shelf.row.show(count: 8) { _ in
                    CardView.placeholder(wide: false, width: theme.layout.poster, theme: theme)
                }
                return shelf
            case let .items(cards):
                let wide = row.kind == .continueWatching || cards.allSatisfy(isWide)
                let shelf = Shelf(rowTitle(row), accessory: accessory(row, several: several), theme: theme)
                let width = wide ? theme.layout.thumbnail : theme.layout.poster
                shelf.row.show(count: cards.count) { [unowned self] in
                    self.itemCard(cards[$0], wide: wide, width: width)
                }
                return shelf
            case let .titles(titles):
                let shelf = Shelf(rowTitle(row), accessory: accessory(row, several: several), theme: theme)
                shelf.row.show(count: titles.count) { [unowned self] in
                    self.titleCard(titles[$0], serverId: row.serverId)
                }
                return shelf
            }
        }

        private func accessory(_ row: Row, several: Bool) -> NSView? {
            var views: [NSView] = []
            if several, let name = row.serverName, row.kind != .continueWatching {
                let badge = Box(fill: theme.colors.surfaceRaised, radius: MavioRadius.sm)
                badge.addFilling(
                    NSTextField.label(name, theme.fonts.caption, color: theme.colors.textSecondary),
                    insets: .symmetric(horizontal: 6, vertical: 2))
                views.append(badge)
            }
            if row.offline {
                views.append(
                    NSStackView.horizontal(
                        [
                            IconView("offline", size: 14, color: theme.colors.textTertiary),
                            NSTextField.label(
                                MavioStrings.homeOffline, theme.fonts.caption, color: theme.colors.textTertiary),
                        ], spacing: MavioSpacing.s4))
            }
            if row.more, let serverId = row.serverId, let hubId = row.hubId {
                views.append(Spacer())
                let title = rowTitle(row)
                views.append(
                    MavioButton(MavioStrings.homeMore, kind: .link, theme: theme) { [weak self] in
                        self?.open(.hub(serverId: serverId, hubId: hubId, title: title))
                    })
            }
            return views.isEmpty ? nil : NSStackView.horizontal(views, spacing: MavioSpacing.s8)
        }
    }

    /// A menu item running a closure.
    @MainActor
    public final class MenuAction: NSMenuItem {
        private let run: @MainActor () -> Void

        /// An item titled title, running run, with a key equivalent.
        public init(_ title: String, icon: String? = nil, key: String = "", run: @escaping @MainActor () -> Void) {
            self.run = run
            super.init(title: title, action: #selector(runAction), keyEquivalent: key)
            target = self
            if let icon { image = .icon(icon) }
        }

        /// Unavailable: the view is made in code.
        @available(*, unavailable)
        public required init(coder: NSCoder) { fatalError() }

        @objc private func runAction() { run() }
    }
#endif
