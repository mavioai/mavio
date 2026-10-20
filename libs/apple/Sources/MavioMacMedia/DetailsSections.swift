#if os(macOS)
    import AppKit
    import MavioMac

    /// The sections under an item's header: a series' episodes by season, a
    /// season's episodes or an episode's season; an album's tracks or a
    /// folder's items; the cast; the extras; the collection; items like it;
    /// what discovery recommends; its details; and its open issues.
    @MainActor
    struct DetailsSections {
        let page: DetailsPage
        let details: Details

        private var theme: Theme { page.theme }
        private var card: Card { details.card }

        /// The sections, in order, each as wide as the page.
        var views: [NSView] {
            var views: [NSView] = []
            if let episodes = episodes() { views.append(episodes) }
            if let children = children() { views.append(children) }
            if let cast = cast() { views.append(cast) }
            if let extras = extras() { views.append(extras) }
            if let collection = details.collection {
                views.append(
                    page.margined(collectionBanner(collection), top: MavioSpacing.s12, bottom: MavioSpacing.s12))
            }
            if !details.similar.isEmpty {
                let shelf = Shelf(MavioStrings.itemSimilar, theme: theme)
                let similar = details.similar
                let width = theme.layout.poster
                shelf.row.show(count: similar.count) { [unowned page] in page.itemCard(similar[$0], width: width) }
                views.append(shelf)
            }
            if let recommendations = recommendations() { views.append(recommendations) }
            views.append(AboutSection(page: page, details: details, source: page.source))
            if let server = page.model.server(card.serverId), !details.offline {
                views.append(ItemIssuesView(page: page, server: server, itemId: card.itemId))
            }
            return views
        }

        // MARK: Episodes

        /// A series' seasons and their episodes, a season's episodes, or an
        /// episode's season.
        private func episodes() -> NSView? {
            switch card.kind {
            case .series:
                guard !details.children.isEmpty else { return nil }
                return SeasonsSection(
                    page: page, seasons: details.children, episodes: details.episodes,
                    current: details.next?.card.itemId)
            case .season:
                guard !details.episodes.isEmpty else { return nil }
                return episodeList(details.episodes, current: details.next?.card.itemId)
            case .episode where details.episodes.count > 1:
                let title = page.heading(
                    MavioStrings.itemMoreFrom(season: details.seasonName ?? String(details.seasonNumber ?? 0)))
                let row = episodeCards(details.episodes, current: card.itemId)
                let stack = NSStackView.vertical(
                    [page.margined(title), row], spacing: theme.isTV ? 0 : MavioSpacing.s12)
                for v in stack.arrangedSubviews { v.widthAnchor.constraint(equalTo: stack.widthAnchor).isActive = true }
                return inset(stack, .symmetric(vertical: MavioSpacing.s8))
            default:
                return nil
            }
        }

        /// Episodes side by side, starting at the current one.
        func episodeCards(_ episodes: [Episode], current: String?) -> CardRow {
            let row = CardRow(theme: theme)
            let page = page
            row.show(count: episodes.count) { [unowned page] in
                episodeCard(episodes[$0], current: episodes[$0].card.itemId == current, page: page)
            }
            if let index = episodes.firstIndex(where: { $0.card.itemId == current }) {
                let width = theme.layout.thumbnail
                let theme = theme
                DispatchQueue.main.async { [weak row] in row?.scroll(toIndex: index, itemWidth: width, theme: theme) }
            }
            return row
        }

        /// A season's episodes, one under another (libs/design/components.md,
        /// Episode Row).
        private func episodeList(_ episodes: [Episode], current: String?) -> NSView {
            var rows: [NSView] = [page.margined(page.heading(MavioStrings.itemEpisodes), bottom: MavioSpacing.s8)]
            for e in episodes { rows.append(episodeRow(e, current: e.card.itemId == current)) }
            let stack = NSStackView.vertical(rows, spacing: 0)
            for v in rows { v.widthAnchor.constraint(equalTo: stack.widthAnchor).isActive = true }
            return inset(stack, .symmetric(vertical: MavioSpacing.s8))
        }

        private func episodeRow(_ e: Episode, current: Bool) -> NSView {
            let still = episodeStill(e.card, theme: theme)
            var text: [NSView] = []
            if let number = e.short {
                text.append(
                    label(
                        number, theme.fonts.footnote, weight: .semibold,
                        color: current ? theme.colors.accentText : theme.colors.textSecondary))
            }
            text.append(
                NSTextField.label(e.card.title, theme.fonts.headline, color: theme.colors.textPrimary, lines: 0))
            text.append(NSTextField.label(e.facts, theme.fonts.footnote, color: theme.colors.textTertiary))
            if let overview = e.overview {
                let field = NSTextField.label(
                    overview, theme.fonts.callout, color: theme.colors.textSecondary, lines: 3)
                text.append(field)
            }
            let column = NSStackView.vertical(text, spacing: MavioSpacing.s2)
            if e.overview != nil, text.count > 1 { column.setCustomSpacing(6, after: text[text.count - 2]) }
            let row = NSStackView.horizontal([still, column], spacing: MavioSpacing.s20, alignment: .top)
            let pressable = Pressable(
                row, look: .highlight, radius: MavioRadius.md,
                insets: .symmetric(horizontal: MavioSpacing.s12, vertical: MavioSpacing.s12), theme: theme
            ) { [weak page] in
                page?.open(.item(serverId: e.card.serverId, itemId: e.card.itemId))
            }
            pressable.setAccessibilityLabel([e.short, e.card.title].compactMap(\.self).joined(separator: " "))
            // The still is a third of the row, from 140 pt to a thumbnail's width.
            let width = still.widthAnchor.constraint(equalTo: pressable.widthAnchor, multiplier: 0.32)
            width.priority = .defaultHigh
            NSLayoutConstraint.activate([
                width,
                still.widthAnchor.constraint(greaterThanOrEqualToConstant: 140),
                still.widthAnchor.constraint(lessThanOrEqualToConstant: theme.layout.thumbnail),
            ])
            for case let field as NSTextField in text {
                field.setContentCompressionResistancePriority(.defaultLow, for: .horizontal)
            }
            return inset(
                pressable,
                .symmetric(horizontal: theme.layout.margin - MavioSpacing.s12))
        }

        // MARK: Children

        /// An album's tracks, a playlist's entries or a folder's items.
        private func children() -> NSView? {
            let kids = details.children
            guard !kids.isEmpty else { return nil }
            switch card.kind {
            case .series, .season, .episode:
                return nil
            case .album, .playlist, .audiobook:
                var rows: [NSView] = [page.heading(MavioStrings.itemTracks)]
                rows += kids.map(trackRow)
                let stack = NSStackView.vertical(rows, spacing: 0)
                stack.setCustomSpacing(MavioSpacing.s12, after: rows[0])
                for v in rows.dropFirst() { v.widthAnchor.constraint(equalTo: stack.widthAnchor).isActive = true }
                return page.margined(stack)
            default:
                let grid = FlowGrid(
                    minimum: theme.layout.poster, gutter: theme.layout.gutter, rowSpacing: MavioSpacing.s24)
                grid.setItems(kids.map { page.itemCard($0) })
                return page.margined(grid, top: MavioSpacing.s16, bottom: MavioSpacing.s16)
            }
        }

        /// A track of an album or an entry of a playlist.
        private func trackRow(_ card: Card) -> NSView {
            var views: [NSView] = [
                NSTextField.label(card.title, theme.fonts.body, color: theme.colors.textPrimary), Spacer(),
            ]
            if let runtime = card.runtime {
                views.append(
                    NSTextField.label(minutesText(runtime), theme.fonts.footnote, color: theme.colors.textSecondary))
            }
            let row = NSStackView.horizontal(views, spacing: MavioSpacing.s12)
            let pressable = Pressable(
                row, look: .highlight, radius: MavioRadius.sm,
                insets: .symmetric(horizontal: 6, vertical: MavioSpacing.s8),
                theme: theme
            ) { [weak page] in
                page?.open(.item(serverId: card.serverId, itemId: card.itemId))
            }
            pressable.setAccessibilityLabel(card.title)
            return pressable
        }

        // MARK: Cast

        /// The cast in billing order, then the creators, directors and
        /// writers; See All at its title on the desktop, at its end in
        /// television mode (docs/tv.md §6.2).
        private func cast() -> NSView? {
            let shown = details.credits.filter { $0.job.isCast || [.creator, .director, .writer].contains($0.job) }
            guard !shown.isEmpty else { return nil }
            let route = Route.credits(serverId: card.serverId, itemId: card.itemId, crew: false)
            let accessory: NSView? =
                theme.isTV
                ? nil
                : NSStackView.horizontal(
                    [
                        Spacer(),
                        MavioButton(MavioStrings.creditsAll, kind: .link, theme: theme) { [weak page] in
                            page?.open(route)
                        },
                    ], spacing: 0)
            let shelf = Shelf(MavioStrings.itemCast, accessory: accessory, theme: theme)
            let serverId = card.serverId
            let page = page
            let tv = theme.isTV
            let size =
                tv ? page.personCard(shown[0], serverId: serverId, role: shown[0].shownRole).fittingSize : .zero
            shelf.row.show(count: shown.count + (tv ? 1 : 0)) { [unowned page] i in
                if i < shown.count {
                    return page.personCard(shown[i], serverId: serverId, role: shown[i].shownRole)
                }
                return page.seeAllCard(width: ceil(size.width), height: ceil(size.height)) { [weak page] in
                    page?.open(route)
                }
            }
            return shelf
        }

        // MARK: Extras

        /// The trailers, featurettes and other extras, each played by itself.
        private func extras() -> NSView? {
            guard !details.extras.isEmpty else { return nil }
            let shelf = Shelf(MavioStrings.itemExtras, theme: theme)
            let extras = details.extras
            let backdrop = details.backdrop?.url
            let theme = theme
            let context = page.context
            shelf.row.show(count: extras.count) { i in
                let extra = extras[i]
                let view = CardView(width: theme.layout.thumbnail, aspect: 16 / 9, icon: "shows", theme: theme)
                view.artwork.set(extra.card.thumb?.url ?? backdrop, title: extra.card.title, icon: "shows")
                view.setText(
                    extra.card.title,
                    [extra.kind.label, extra.card.runtime.map(minutesText)].compactMap(\.self).joined(separator: " · "))
                let mark = Box(fill: theme.colors.scrim, radius: 999)
                mark.addFilling(IconView("play", fill: true, size: 14, color: .white), insets: .all(6))
                view.overlay.addSubview(mark)
                NSLayoutConstraint.activate([
                    mark.leadingAnchor.constraint(equalTo: view.overlay.leadingAnchor, constant: MavioSpacing.s8),
                    mark.bottomAnchor.constraint(equalTo: view.overlay.bottomAnchor, constant: -MavioSpacing.s8),
                ])
                view.onPress = {
                    context.play(
                        PlayRequest(serverId: extra.card.serverId, itemId: extra.card.itemId, title: extra.card.title))
                }
                return view
            }
            return shelf
        }

        // MARK: Collection

        /// The collection a movie belongs to, over its backdrop, else the
        /// movie's.
        private func collectionBanner(_ collection: CollectionLink) -> NSView {
            let holder = FlippedView()
            holder.translatesAutoresizingMaskIntoConstraints = false
            holder.appearance = NSAppearance(named: .darkAqua)
            let art = ArtworkView((collection.backdrop ?? details.backdrop)?.url, icon: "collections", theme: theme)
            art.setAccessibilityElement(false)
            holder.addFilling(art)
            holder.addFilling(
                GradientView(
                    colors: [
                        .black.withAlphaComponent(0.8), .black.withAlphaComponent(0.55), .black.withAlphaComponent(0.3),
                    ],
                    locations: [0, 0.5, 1], from: CGPoint(x: 0, y: 0.5), to: CGPoint(x: 1, y: 0.5)))
            let caption = label(
                MavioStrings.itemCollection, theme.fonts.footnote, weight: .semibold,
                color: .white.withAlphaComponent(0.7))
            let name = NSTextField.label(collection.name, theme.fonts.title3, color: .white)
            name.setContentCompressionResistancePriority(.defaultLow, for: .horizontal)
            let text = NSStackView.vertical([caption, name], spacing: MavioSpacing.s2)
            let pill = Box(fill: .white.withAlphaComponent(0.16), radius: 18)
            pill.addFilling(
                NSStackView.horizontal(
                    [
                        label(MavioStrings.itemViewCollection, theme.fonts.callout, weight: .semibold, color: .white),
                        IconView("forward", size: 14, color: .white),
                    ], spacing: MavioSpacing.s4),
                insets: .symmetric(horizontal: MavioSpacing.s16))
            pill.size(height: 36)
            let row = NSStackView.horizontal([text, Spacer(), pill], spacing: MavioSpacing.s16)
            holder.addFilling(row, insets: .symmetric(horizontal: MavioSpacing.s24, vertical: MavioSpacing.s20))
            holder.heightAnchor.constraint(greaterThanOrEqualToConstant: 112).isActive = true
            let serverId = card.serverId
            let banner = Pressable(holder, look: .lift, radius: 14, theme: theme) { [weak page] in
                page?.open(.item(serverId: serverId, itemId: collection.itemId))
            }
            banner.wantsLayer = true
            banner.layer?.masksToBounds = !theme.isTV
            holder.wantsLayer = true
            holder.layer?.cornerRadius = 14
            holder.layer?.masksToBounds = true
            banner.setAccessibilityLabel("\(MavioStrings.itemCollection) \(collection.name)")
            return banner
        }

        // MARK: Recommendations

        /// The titles the discovery providers recommend to those who liked
        /// a movie or series.
        private func recommendations() -> NSView? {
            let kind: MediaKind? = card.kind == .movie ? .movie : card.kind == .series ? .series : nil
            let tmdbId = Int32(details.externalIds["tmdb"] ?? "") ?? 0
            let discovery = page.context.discovery
            guard let kind, tmdbId > 0, let server = discovery.server,
                let catalog = discovery.related(kind).first(where: { $0.kind == .recommendations })
            else { return nil }
            return CatalogShelf(
                page: page,
                query: CatalogQuery(
                    serverId: server.id, catalog: catalog.id, kind: kind, tmdbId: tmdbId,
                    title: MavioStrings.discoverRecommendations))
        }
    }

    // MARK: - Episodes

    /// An episode in a row (libs/design/components.md, Episode Card): its
    /// still, number, length and air date, name and overview, all of a
    /// height so that the row's cards align.
    @MainActor
    private func episodeCard(_ e: Episode, current: Bool, page: Page) -> NSView {
        let theme = page.theme
        let width = theme.layout.thumbnail
        let still = episodeStill(e.card, theme: theme)
        still.size(width: width, height: width * 9 / 16)
        var line: [NSView] = []
        if let number = e.short {
            line.append(
                label(
                    number, theme.fonts.footnote, weight: .semibold,
                    color: current ? theme.colors.accentText : theme.colors.textSecondary))
        }
        let facts = NSTextField.label(e.facts, theme.fonts.footnote, color: theme.colors.textTertiary)
        facts.setContentCompressionResistancePriority(.defaultLow, for: .horizontal)
        line.append(facts)
        let title = NSTextField.label(e.card.title, theme.fonts.headline, color: theme.colors.textPrimary)
        title.setContentCompressionResistancePriority(.defaultLow, for: .horizontal)
        var views: [NSView] = [still, NSStackView.horizontal(line, spacing: MavioSpacing.s8), title]
        if let overview = e.overview {
            let field = NSTextField.label(overview, theme.fonts.footnote, color: theme.colors.textSecondary, lines: 3)
            field.preferredMaxLayoutWidth = width
            field.setContentCompressionResistancePriority(.defaultLow, for: .horizontal)
            views.append(field)
        }
        let stack = NSStackView.vertical(views, spacing: MavioSpacing.s2)
        stack.setCustomSpacing(MavioSpacing.s4 + MavioSpacing.s2, after: still)
        for v in views.dropFirst() { v.widthAnchor.constraint(lessThanOrEqualTo: stack.widthAnchor).isActive = true }
        let holder = FlippedView()
        holder.translatesAutoresizingMaskIntoConstraints = false
        holder.addSubview(stack)
        NSLayoutConstraint.activate([
            stack.leadingAnchor.constraint(equalTo: holder.leadingAnchor),
            stack.trailingAnchor.constraint(equalTo: holder.trailingAnchor),
            stack.topAnchor.constraint(equalTo: holder.topAnchor),
            stack.bottomAnchor.constraint(lessThanOrEqualTo: holder.bottomAnchor),
        ])
        // Every card as tall as one with three lines of overview.
        let height =
            width * 9 / 16 + MavioSpacing.s4 + MavioSpacing.s2 * 3 + theme.fonts.footnote.lineHeight * 4
            + theme.fonts.headline.lineHeight
        holder.size(width: width, height: ceil(height))
        let card = Pressable(holder, look: .lift, radius: MavioRadius.md, theme: theme) { [weak page] in
            page?.open(.item(serverId: e.card.serverId, itemId: e.card.itemId))
        }
        card.setAccessibilityLabel([e.short, e.card.title].compactMap(\.self).joined(separator: " "))
        return card
    }

    /// An episode's still with its progress and the played check.
    @MainActor
    private func episodeStill(_ card: Card, theme: Theme) -> NSView {
        let art = ArtworkView(card.thumb?.url ?? card.poster?.url, title: card.title, icon: "shows", theme: theme)
        art.radius = MavioRadius.md
        art.aspect(16 / 9)
        if let progress = card.progress, progress > 0, progress < 1, !card.played {
            let track = Box(fill: .white.withAlphaComponent(0.35), radius: 2)
            let bar = Box(fill: .white, radius: 2)
            track.addSubview(bar)
            art.addSubview(track)
            NSLayoutConstraint.activate([
                track.leadingAnchor.constraint(equalTo: art.leadingAnchor, constant: 10),
                track.trailingAnchor.constraint(equalTo: art.trailingAnchor, constant: -10),
                track.bottomAnchor.constraint(equalTo: art.bottomAnchor, constant: -10),
                track.heightAnchor.constraint(equalToConstant: 4),
                bar.leadingAnchor.constraint(equalTo: track.leadingAnchor),
                bar.topAnchor.constraint(equalTo: track.topAnchor),
                bar.bottomAnchor.constraint(equalTo: track.bottomAnchor),
                bar.widthAnchor.constraint(equalTo: track.widthAnchor, multiplier: CGFloat(progress)),
            ])
        }
        if card.played {
            let mark = Box(fill: theme.colors.scrim, radius: 999)
            mark.addFilling(IconView("played", fill: true, size: 18, color: .white), insets: .all(MavioSpacing.s2))
            art.addSubview(mark)
            NSLayoutConstraint.activate([
                mark.topAnchor.constraint(equalTo: art.topAnchor, constant: 6),
                mark.trailingAnchor.constraint(equalTo: art.trailingAnchor, constant: -6),
            ])
        }
        return art
    }

    extension Episode {
        /// The numbers, as "S1 E2".
        fileprivate var short: String? {
            guard let number else { return nil }
            return MavioStrings.itemEpisodeShort(season: String(season ?? 0), episode: String(number))
        }

        /// The runtime and air date.
        fileprivate var facts: String {
            [card.runtime.map(minutesText), premiere?.formatted(date: .abbreviated, time: .omitted)]
                .compactMap(\.self).joined(separator: " · ")
        }
    }

    /// A series' episodes by season: the seasons as pills, starting with the
    /// season of the episode to watch next, and the chosen one's episodes in
    /// a row, with More opening the season. In television mode focus on a
    /// pill chooses it.
    private final class SeasonsSection: NSView {
        private weak var page: DetailsPage?
        private let seasons: [Card]
        private let episodes: [Episode]
        private let current: String?
        private let rowHolder = NSView()
        private var chosen: Int

        init(page: DetailsPage, seasons: [Card], episodes: [Episode], current: String?) {
            self.page = page
            self.seasons = seasons
            self.episodes = episodes
            self.current = current
            let currentSeason = episodes.first { $0.card.itemId == current }?.seasonId
            chosen = seasons.firstIndex { $0.itemId == currentSeason } ?? 0
            super.init(frame: .zero)
            translatesAutoresizingMaskIntoConstraints = false
            build(page)
        }

        @available(*, unavailable)
        required init?(coder: NSCoder) { fatalError() }

        private func build(_ page: DetailsPage) {
            let theme = page.theme
            var header: [NSView] = [page.heading(MavioStrings.itemEpisodes)]
            if seasons.count > 1 {
                let pills = PillTabs(seasons.map(\.title), selected: chosen, theme: theme)
                pills.choosesOnFocus = theme.isTV
                pills.onChange = { [weak self] in self?.choose($0) }
                header.append(pills)
            } else {
                header.append(
                    NSTextField.label(seasons[0].title, theme.fonts.callout, color: theme.colors.textSecondary))
            }
            header.append(Spacer())
            header.append(
                MavioButton(MavioStrings.homeMore, kind: .link, theme: theme) { [weak self] in
                    guard let self, let page = self.page else { return }
                    let season = self.seasons[self.chosen]
                    page.open(.item(serverId: season.serverId, itemId: season.itemId))
                })
            let top = NSStackView.horizontal(header, spacing: MavioSpacing.s16)
            for v in header.dropFirst() where v is PillTabs {
                v.setContentCompressionResistancePriority(.defaultLow, for: .horizontal)
            }
            rowHolder.translatesAutoresizingMaskIntoConstraints = false
            let stack = NSStackView.vertical(
                [page.margined(top), rowHolder], spacing: theme.isTV ? 0 : MavioSpacing.s12)
            for v in stack.arrangedSubviews { v.widthAnchor.constraint(equalTo: stack.widthAnchor).isActive = true }
            addFilling(stack, insets: .symmetric(vertical: MavioSpacing.s8))
            showRow()
        }

        private func choose(_ index: Int) {
            guard index != chosen, seasons.indices.contains(index) else { return }
            chosen = index
            showRow()
            if let page { FocusEngine.active(for: page.view)?.schedule() }
        }

        private func showRow() {
            guard let page else { return }
            rowHolder.subviews.forEach { $0.removeFromSuperview() }
            let season = seasons[chosen]
            let shown = episodes.filter { $0.seasonId == season.itemId }
            let row = DetailsSections(page: page, details: page.details!).episodeCards(shown, current: current)
            rowHolder.addFilling(row)
        }
    }

    // MARK: - Recommendations

    /// A catalog of discovery in a row: placeholders while it loads, nothing
    /// when it is empty, and More opening all of it when there is more.
    final class CatalogShelf: NSView {
        private weak var page: Page?
        private let query: CatalogQuery
        private var task: Task<Void, Never>?

        /// The first page of query.
        init(page: Page, query: CatalogQuery) {
            self.page = page
            self.query = query
            super.init(frame: .zero)
            translatesAutoresizingMaskIntoConstraints = false
            let theme = page.theme
            let shelf = Shelf(query.title, theme: theme)
            shelf.row.show(count: 8) { _ in CardView.placeholder(wide: false, width: theme.layout.poster, theme: theme)
            }
            addFilling(shelf)
            load()
        }

        @available(*, unavailable)
        required init?(coder: NSCoder) { fatalError() }

        private func load() {
            task = Task { [weak self] in
                guard let self, let model = self.page?.model else { return }
                let response = try? await model.discover(self.query, page: 1)
                guard !Task.isCancelled else { return }
                self.show(response)
            }
        }

        private func show(_ response: Mavio_Acquisition_V1_DiscoverResponse?) {
            guard let page else { return }
            subviews.forEach { $0.removeFromSuperview() }
            guard let response, !response.titles.isEmpty else {
                isHidden = true
                return
            }
            let theme = page.theme
            let query = query
            let accessory: NSView? =
                response.page < response.totalPages
                ? NSStackView.horizontal(
                    [
                        Spacer(),
                        MavioButton(MavioStrings.homeMore, kind: .link, theme: theme) { [weak page] in
                            page?.open(.catalog(query))
                        },
                    ], spacing: 0)
                : nil
            let shelf = Shelf(query.title, accessory: accessory, theme: theme)
            let titles = response.titles
            shelf.row.show(count: titles.count) { [unowned page] in
                page.discoveredCard(titles[$0], serverId: query.serverId, width: theme.layout.poster)
            }
            addFilling(shelf)
            FocusEngine.active(for: self)?.schedule()
        }

        isolated deinit { task?.cancel() }
    }
#endif
