#if os(macOS)
    import AppKit
    import MavioMac

    /// A discovered title (docs/clients.md §6.3.2), as Seerr's page: its
    /// backdrop with the name, facts, genres and status over it, and the
    /// actions — Watch, the trailer, requesting it, approving or declining
    /// its requests, reporting an issue and managing it — then its overview
    /// beside its collection and facts, its seasons, its cast and the titles
    /// alike. Laid out as an item's details in television mode, focus
    /// starting on its first action (docs/tv.md §6.2, §6.3).
    final class TitlePage: Page, TitleBarReaching {
        /// The backdrop goes behind the title bar, as an item's details'.
        var reachesTitleBar: Bool { true }

        /// The server discovering the title.
        let serverId: String
        private let kind: MediaKind
        private let tmdbId: Int32
        private(set) var info: TitleInfo?
        private var ratings: [Rating] = []
        /// The user signed in, whose role the requests' actions follow.
        private(set) var user: Mavio_User_V1_User?
        private var percent: Int?
        private var observer: Observer?
        private var revision: Int?
        private var loading: Task<Void, Never>?
        private var side: [Task<Void, Never>] = []
        private let page = ScrollPage(
            [], spacing: 0, insets: NSEdgeInsets(top: 0, left: 0, bottom: MavioSpacing.s40, right: 0))
        private var hero: TitleHero?
        private var heroHeight: NSLayoutConstraint?
        private let factsHolder = NSView()
        private var allStudios = false
        /// The rows of titles alike, kept as the title is read again.
        private var shelves: [CatalogQuery: CatalogShelf] = [:]
        /// Why the last request failed.
        var requestFailure: String?
        /// Why approving or declining the last time failed.
        var decideFailure: String?
        /// Whether a request or a decision is on its way.
        var busy = false
        /// Told the title each time it is read, as by a dialog showing it.
        var onInfo: (@MainActor (TitleInfo) -> Void)?

        /// The title of kind and TMDB ID as serverId discovers it.
        init(context: MacContext, serverId: String, kind: MediaKind, tmdbId: Int32) {
            self.serverId = serverId
            self.kind = kind
            self.tmdbId = tmdbId
            super.init(context: context)
        }

        isolated deinit {
            observer?.cancel()
            loading?.cancel()
            side.forEach { $0.cancel() }
        }

        override func viewDidLoad() {
            super.viewDidLoad()
            factsHolder.translatesAutoresizingMaskIntoConstraints = false
            // Read again as the server's data changes.
            observer = Observer { [weak self] in
                guard let self else { return }
                let now = self.model.revision(self.serverId)
                guard now != self.revision else { return }
                self.revision = now
                self.load()
            }
            let model = model
            let serverId = serverId
            side.append(
                Task { [weak self] in
                    let user = await RequestsAPI.currentUser(model, serverId)
                    guard let self, !Task.isCancelled else { return }
                    self.user = user
                    self.renderActions()
                    self.loadBadge()
                })
        }

        override func viewDidLayout() {
            super.viewDidLayout()
            let target = heroTargetHeight
            if let heroHeight, abs(heroHeight.constant - target) > 0.5 { heroHeight.constant = target }
        }

        /// The header's height: as the window allows between 520 and 700 pt
        /// on the desktop, 86 % of the page's in television mode.
        private var heroTargetHeight: CGFloat {
            let h = view.bounds.height
            return theme.isTV ? max(480, h * 0.86) : min(max(520, h * 0.7), 700)
        }

        /// Reads the title again.
        func load() {
            let (model, serverId, kind, tmdbId) = (model, serverId, kind, tmdbId)
            loading?.cancel()
            loading = Task { [weak self] in
                do {
                    let info = try await model.title(serverId, kind, tmdbId)
                    guard let self, !Task.isCancelled else { return }
                    let first = self.info == nil
                    self.info = info
                    self.render()
                    if first { self.loadSide(info) }
                    self.loadBadge()
                } catch {
                    guard let self, !Task.isCancelled, !(error is CancellationError), self.info == nil else { return }
                    self.showError(error.reason) { [weak self] in
                        self?.show(LoadingView())
                        self?.load()
                    }
                }
            }
        }

        private func loadSide(_ info: TitleInfo) {
            let (model, serverId) = (model, serverId)
            let (kind, tmdbId) = (info.mediaKind, info.tmdbID)
            side.append(
                Task { [weak self] in
                    let ratings = (try? await model.ratings(serverId, kind, tmdbId)) ?? []
                    guard let self, !Task.isCancelled else { return }
                    self.ratings = ratings
                    self.renderFacts()
                })
        }

        /// How far the title's downloads have come, for administrators.
        private func loadBadge() {
            guard user?.admin == true, let itemId = info?.itemID, !itemId.isEmpty else { return }
            let (model, serverId) = (model, serverId)
            side.append(
                Task { [weak self] in
                    let percent = await RequestsAPI.downloading(model, serverId, itemId)
                    guard let self, !Task.isCancelled else { return }
                    self.percent = percent
                    self.renderBadge()
                })
        }

        // MARK: Rendering

        private func render() {
            guard let info else { return }
            title = info.name
            var views: [NSView] = []
            let hero = TitleHero(page: self, info: info)
            self.hero = hero
            let height = hero.heightAnchor.constraint(equalToConstant: heroTargetHeight)
            height.isActive = true
            heroHeight = height
            views.append(hero)
            views.append(overview(info))
            if info.mediaKind == .series, !info.seasons.isEmpty {
                views.append(SeasonsView(page: self, info: info))
            }
            if let cast = castShelf(info) { views.append(cast) }
            for c in context.discovery.related(info.mediaKind) {
                let query = CatalogQuery(
                    serverId: serverId, catalog: c.id, kind: info.mediaKind, tmdbId: info.tmdbID,
                    title: c.kind == .recommendations
                        ? MavioStrings.discoverRecommendations : MavioStrings.discoverSimilar)
                let shelf = shelves[query] ?? CatalogShelf(query: query, page: self)
                shelves[query] = shelf
                views.append(shelf)
            }
            page.setViews(views)
            hero.setActions(actions(info))
            hero.setBadge(percent)
            onInfo?(info)
            if page.superview == nil {
                show(page)
                if theme.isTV, let first = hero.firstAction, let engine = FocusEngine.active(for: view) {
                    engine.prefer(first)
                }
            }
            FocusEngine.active(for: view)?.schedule()
        }

        /// The actions, again, as the user or the title change.
        func renderActions() {
            guard let info, let hero else { return }
            hero.setActions(actions(info))
            FocusEngine.active(for: view)?.schedule()
        }

        private func renderFacts() {
            guard let info else { return }
            factsHolder.subviews.forEach { $0.removeFromSuperview() }
            factsHolder.addFilling(factsBox(info))
            FocusEngine.active(for: view)?.schedule()
        }

        private func renderBadge() {
            guard let hero else { return }
            hero.setBadge(percent)
        }

        // MARK: Overview

        /// The overview and facts in two columns, as Seerr's: the tagline,
        /// overview, leading crew and keywords beside its collection,
        /// ratings and facts, each genre, keyword, studio, network and
        /// language opening the popular titles alike.
        private func overview(_ info: TitleInfo) -> NSView {
            let leading = overviewLeading(info)
            renderFacts()
            var trailingViews: [NSView] = []
            if let collection = CollectionCard(page: self, info: info) { trailingViews.append(collection) }
            trailingViews.append(factsHolder)
            let trailing = NSStackView.vertical(trailingViews, spacing: MavioSpacing.s24)
            for v in trailingViews { v.widthAnchor.constraint(equalTo: trailing.widthAnchor).isActive = true }
            // The overview takes the width the facts leave; each column is
            // as tall as its views, from the top, and the row as the taller.
            let left = RegionView(leading)
            let right = RegionView(trailing)
            let row = NSView()
            row.translatesAutoresizingMaskIntoConstraints = false
            row.addSubview(left)
            row.addSubview(right)
            let snug = row.heightAnchor.constraint(equalToConstant: 0)
            snug.priority = .init(1)
            NSLayoutConstraint.activate([
                left.leadingAnchor.constraint(equalTo: row.leadingAnchor),
                left.topAnchor.constraint(equalTo: row.topAnchor),
                right.leadingAnchor.constraint(equalTo: left.trailingAnchor, constant: MavioSpacing.s32),
                right.trailingAnchor.constraint(equalTo: row.trailingAnchor),
                right.topAnchor.constraint(equalTo: row.topAnchor),
                right.widthAnchor.constraint(equalToConstant: theme.isTV ? 480 : 380),
                row.bottomAnchor.constraint(greaterThanOrEqualTo: left.bottomAnchor),
                row.bottomAnchor.constraint(greaterThanOrEqualTo: right.bottomAnchor),
                snug,
            ])
            return margined(row, top: MavioSpacing.s24, bottom: MavioSpacing.s12)
        }

        private func overviewLeading(_ info: TitleInfo) -> NSView {
            var views: [NSView] = []
            if !info.tagline.isEmpty {
                views.append(
                    label(
                        info.tagline, theme.fonts.title3, weight: .regular, color: theme.colors.textSecondary, lines: 0,
                        italic: true))
            }
            views.append(heading(MavioStrings.discoverOverview))
            views.append(
                NSTextField.label(
                    info.overview.isEmpty ? MavioStrings.discoverNoOverview : info.overview, theme.fonts.body,
                    color: theme.colors.textPrimary, lines: 0))
            let crew = leadingCrew(info)
            if !crew.isEmpty {
                var rows: [NSView] = []
                for pair in stride(from: 0, to: crew.count, by: 2) {
                    let cells = crew[pair..<min(pair + 2, crew.count)].map(crewCell)
                    let row = NSStackView.horizontal(
                        cells + (cells.count == 1 ? [NSView()] : []), spacing: MavioSpacing.s24, alignment: .top)
                    row.distribution = .fillEqually
                    rows.append(row)
                }
                let grid = NSStackView.vertical(rows, spacing: MavioSpacing.s16)
                for r in rows { r.widthAnchor.constraint(equalTo: grid.widthAnchor).isActive = true }
                views.append(inset(grid, NSEdgeInsets(top: MavioSpacing.s8, left: 0, bottom: 0, right: 0)))
                let (serverId, kind, tmdbId) = (serverId, info.mediaKind, info.tmdbID)
                let full = Pressable(
                    NSStackView.horizontal(
                        [
                            NSTextField.label(
                                MavioStrings.discoverFullCrew, theme.fonts.callout, color: theme.colors.textSecondary),
                            IconView("forward", size: 16, color: theme.colors.textSecondary),
                        ], spacing: MavioSpacing.s4), look: .highlight, radius: MavioRadius.sm, theme: theme
                ) { [weak self] in
                    self?.open(.titleCredits(serverId: serverId, kind: kind, tmdbId: tmdbId, crew: true))
                }
                full.setAccessibilityLabel(MavioStrings.discoverFullCrew)
                views.append(NSStackView.horizontal([Spacer(), full], spacing: 0))
            }
            let keywords = titleKeywords(info)
            if !keywords.isEmpty {
                let chips = keywords.map { k -> NSView in
                    if !k.id.isEmpty,
                        let query = context.discovery.alike(serverId, info.mediaKind, k.name, { $0.keywords = [k] })
                    {
                        return chip(k.name) { [weak self] in self?.open(.catalog(query)) }
                    }
                    return chip(k.name)
                }
                views.append(
                    inset(
                        WrapStack(chips, spacing: MavioSpacing.s8, lineSpacing: MavioSpacing.s8),
                        NSEdgeInsets(top: MavioSpacing.s8, left: 0, bottom: 0, right: 0)))
            }
            let stack = NSStackView.vertical(views, spacing: MavioSpacing.s16)
            for v in views { v.widthAnchor.constraint(equalTo: stack.widthAnchor).isActive = true }
            return stack
        }

        private func crewCell(_ c: TitleCredit) -> NSView {
            let job = NSTextField.label(c.jobLabel, theme.fonts.headline, color: theme.colors.textPrimary)
            let name: NSView
            if c.tmdbId > 0 {
                let serverId = serverId
                name = textLink(c.name, theme.fonts.callout, color: theme.colors.textSecondary) { [weak self] in
                    self?.open(.discoveredPerson(serverId: serverId, tmdbId: c.tmdbId))
                }
            } else {
                name = NSTextField.label(c.name, theme.fonts.callout, color: theme.colors.textSecondary)
            }
            return NSStackView.vertical([job, name], spacing: 0)
        }

        /// The facts in a bordered box, as Seerr's: the ratings, then a fact
        /// a row, the services streaming it and the links elsewhere.
        private func factsBox(_ info: TitleInfo) -> NSView {
            var rows: [NSView] = []
            let marks = ratingMarks(
                kind: info.mediaKind, tmdbId: info.tmdbID, ratings: ratings,
                score: info.voteCount > 0 ? info.communityRating : nil)
            if !marks.isEmpty {
                let views = marks.map(ratingView)
                let wrap = NSStackView.horizontal(views, spacing: MavioSpacing.s16)
                // Centered: the two spaces kept equal, which they would
                // otherwise share by chance.
                let (before, after) = (Spacer(), Spacer())
                rows.append(NSStackView.horizontal([before, wrap, after], spacing: 0))
                before.widthAnchor.constraint(equalTo: after.widthAnchor).isActive = true
            }
            for (name, values) in titleFactRows(
                info, serverId: serverId, discovery: context.discovery, allStudios: allStudios)
            {
                var shown: [NSView] = values.map { v in
                    if let query = v.query {
                        return textLink(v.text, theme.fonts.callout, color: theme.colors.textSecondary) { [weak self] in
                            self?.open(.catalog(query))
                        }
                    }
                    let text = NSTextField.label(
                        v.text, theme.fonts.callout, color: theme.colors.textSecondary, lines: 0)
                    text.alignment = .right
                    return text
                }
                if name == studiosLabel(info), hasMoreStudios(info) {
                    let icon = IconView("expand", size: 14, color: theme.colors.textSecondary)
                    icon.upsideDown = allStudios
                    let more = Pressable(
                        NSStackView.horizontal(
                            [
                                NSTextField.label(
                                    allStudios ? MavioStrings.actionShowLess : MavioStrings.discoverShowMore,
                                    theme.fonts.callout, color: theme.colors.textSecondary),
                                icon,
                            ], spacing: MavioSpacing.s4), look: .highlight, radius: MavioRadius.sm, theme: theme
                    ) { [weak self] in
                        guard let self else { return }
                        self.allStudios.toggle()
                        self.renderFacts()
                    }
                    shown.append(more)
                }
                let column = NSStackView.vertical(shown, spacing: 2, alignment: .trailing)
                let title = label(name, theme.fonts.callout, weight: .semibold, color: theme.colors.textPrimary)
                title.setContentCompressionResistancePriority(.required, for: .horizontal)
                rows.append(
                    NSStackView.horizontal([title, Spacer(), column], spacing: MavioSpacing.s16, alignment: .top))
            }
            if !info.watchProviders.isEmpty { rows.append(watchProviders(info.watchProviders)) }
            let item = info.itemID.isEmpty ? nil : Route.item(serverId: serverId, itemId: info.itemID)
            if let links = externalLinks(titleLinks(info, ratings), item: item) {
                // Across the box from its left, under the streaming services, as
                // the web UI's.
                rows.append(links)
            }
            let box = Box(radius: 14)
            box.border = theme.colors.border
            let stack = NSStackView.vertical([], spacing: 0)
            for (i, row) in rows.enumerated() {
                if i > 0 {
                    let line = Box(fill: theme.colors.border)
                    line.size(height: 1)
                    stack.addArrangedSubview(line)
                    line.widthAnchor.constraint(equalTo: stack.widthAnchor).isActive = true
                }
                let padded = inset(row, .symmetric(horizontal: MavioSpacing.s16, vertical: 10))
                stack.addArrangedSubview(padded)
                padded.widthAnchor.constraint(equalTo: stack.widthAnchor).isActive = true
            }
            box.addFilling(stack)
            return box
        }

        /// A rating: its site's logo and value, opening its page on the
        /// desktop.
        private func ratingView(_ m: RatingMark) -> NSView {
            let content = NSStackView.horizontal(
                [
                    serviceLogo(m.logo, height: m.height),
                    label(m.value, theme.fonts.callout, weight: .semibold, color: theme.colors.textPrimary),
                ], spacing: 6)
            content.setAccessibilityElement(true)
            content.setAccessibilityLabel("\(m.label) \(m.value)")
            guard let url = m.url, !theme.isTV else { return content }
            let link = Pressable(content, look: .highlight, radius: MavioRadius.sm, theme: theme) {
                NSWorkspace.shared.open(url)
            }
            link.setAccessibilityLabel("\(m.label) \(m.value)")
            return link
        }

        /// The services streaming the title, by their logos.
        private func watchProviders(_ providers: [Mavio_Plugin_V1_WatchProvider]) -> NSView {
            let logos = providers.map { p -> NSView in
                guard !p.logoURL.isEmpty else {
                    return NSTextField.label(p.name, theme.fonts.footnote, color: theme.colors.textPrimary)
                }
                let art = ArtworkView(p.logoURL, title: p.name, theme: theme)
                art.radius = 6
                art.size(width: 32, height: 32)
                art.alphaValue = 0.7
                art.toolTip = p.name
                art.setAccessibilityLabel(p.name)
                return art
            }
            let wrap = WrapStack(logos, spacing: MavioSpacing.s12, lineSpacing: MavioSpacing.s12)
            let stack = NSStackView.vertical(
                [
                    label(
                        MavioStrings.itemStreaming, theme.fonts.callout, weight: .semibold,
                        color: theme.colors.textPrimary), wrap,
                ],
                spacing: MavioSpacing.s8)
            wrap.widthAnchor.constraint(equalTo: stack.widthAnchor).isActive = true
            return stack
        }

        // MARK: Cast

        /// The first twenty of the cast, each opening their page, with See
        /// All opening the whole cast and crew: at the row's arrows on
        /// the desktop, as a card at its end in television mode.
        private func castShelf(_ info: TitleInfo) -> NSView? {
            let shown = Array(info.credits.filter(\.isCast).sorted { $0.order < $1.order }.prefix(20))
            guard !shown.isEmpty else { return nil }
            let route = Route.titleCredits(serverId: serverId, kind: info.mediaKind, tmdbId: info.tmdbID, crew: false)
            var accessory: NSView?
            if !theme.isTV {
                accessory = MavioButton(MavioStrings.creditsAll, kind: .link, theme: theme) { [weak self] in
                    self?.open(route)
                }
            }
            let shelf = Shelf(MavioStrings.itemCast, accessory: accessory, theme: theme)
            let serverId = serverId
            let tv = theme.isTV
            shelf.row.show(count: shown.count + (tv ? 1 : 0)) { [weak self] i in
                guard let self else { return NSView() }
                if i == shown.count {
                    return self.seeAllCard(height: 180, width: 180) { [weak self] in self?.open(route) }
                }
                return self.creditCard(shown[i], serverId: serverId)
            }
            return shelf
        }
    }

    /// The header of a discovered title: its backdrop, or its poster
    /// blurred, darkened at its start and foot, and over its lower left the
    /// poster, the name, the facts with the genres and status, and the
    /// actions.
    @MainActor
    final class TitleHero: FlippedView {
        private let actions = WrapStack([], spacing: MavioSpacing.s12, lineSpacing: MavioSpacing.s12)
        private let failures = NSStackView.vertical([], spacing: MavioSpacing.s4)
        private let badge: Box
        private let badgeLabel: NSTextField
        private let theme: Theme
        /// The first action, which focus starts on in television mode.
        var firstAction: NSView? { actions.views.first }

        /// The header of info on page.
        init(page: TitlePage, info: TitleInfo) {
            theme = page.theme
            badgeLabel = label("", theme.fonts.caption, weight: .semibold, color: .white)
            badge = Box(fill: theme.colors.scrim, radius: MavioRadius.sm)
            super.init(frame: .zero)
            translatesAutoresizingMaskIntoConstraints = false
            wantsLayer = true
            layer?.masksToBounds = true
            appearance = NSAppearance(named: .darkAqua)
            let series = info.mediaKind == .series
            addFilling(
                heroArtwork(
                    backdrop: info.backdropURL, poster: info.posterURL, icon: series ? "shows" : "movies", theme: theme)
            )
            addHeroGradients(to: self)
            badge.addFilling(badgeLabel, insets: .symmetric(horizontal: 6))
            badge.isHidden = true

            let tv = theme.isTV
            let name = label(info.name, theme.fonts.title1, weight: .bold, color: .white, lines: 2)
            name.font = .systemFont(ofSize: tv ? 72 : 52, weight: .bold)
            let views: [NSView] = [name, facts(page: page, info: info), actions, failures]
            let column = NSStackView.vertical(views, spacing: MavioSpacing.s12)
            column.setCustomSpacing(MavioSpacing.s16, after: views[1])
            for v in views.dropFirst() { v.widthAnchor.constraint(equalTo: column.widthAnchor).isActive = true }
            column.widthAnchor.constraint(lessThanOrEqualToConstant: tv ? 960 : 640).isActive = true
            var row: [NSView] = []
            if !tv, let poster = sized(info.posterURL, 500) {
                row.append(heroPoster(poster, title: info.name, theme: theme))
            }
            row.append(column)
            let content = NSStackView.horizontal(row, spacing: MavioSpacing.s40, alignment: .bottom)
            addSubview(content)
            let side = tv ? theme.layout.margin : MavioSpacing.s40
            NSLayoutConstraint.activate([
                content.leadingAnchor.constraint(equalTo: leadingAnchor, constant: side),
                content.trailingAnchor.constraint(lessThanOrEqualTo: trailingAnchor, constant: -side),
                content.bottomAnchor.constraint(equalTo: bottomAnchor, constant: -MavioSpacing.s40),
                content.topAnchor.constraint(greaterThanOrEqualTo: topAnchor, constant: MavioSpacing.s24),
            ])
            let fill = column.widthAnchor.constraint(equalToConstant: tv ? 960 : 640)
            fill.priority = .defaultLow
            fill.isActive = true
        }

        @available(*, unavailable)
        required init?(coder: NSCoder) { fatalError() }

        /// As Seerr's: the year, kind, rating and runtime, then three of the
        /// genres, each opening the popular titles alike, the status and an
        /// administrator's download badge.
        private func facts(page: TitlePage, info: TitleInfo) -> NSView {
            let color = NSColor.white.withAlphaComponent(0.8)
            let style = theme.fonts.callout
            let shown = Array(titleGenres(info).prefix(3))
            let facts = titleHeaderFacts(info)
            var views: [NSView] = []
            let text = facts.joined(separator: " · ") + (shown.isEmpty || facts.isEmpty ? "" : " · ")
            if !text.isEmpty { views.append(NSTextField.label(text, style, color: color)) }
            for (i, g) in shown.enumerated() {
                let name = g.name + (i < shown.count - 1 ? " /" : "")
                if !g.id.isEmpty,
                    let query = page.context.discovery.alike(
                        page.serverId, info.mediaKind, g.name, { $0.genres = [g.id] })
                {
                    views.append(
                        page.textLink(name, style, color: color) { [weak page] in page?.open(.catalog(query)) })
                } else {
                    views.append(NSTextField.label(name, style, color: color))
                }
            }
            if let status = statusLabel(info.status) {
                views.append(
                    inset(
                        pill(
                            status, theme.fonts.caption, fill: NSColor.white.withAlphaComponent(0.2), color: .white,
                            radius: MavioRadius.sm, insets: .symmetric(horizontal: 6)),
                        NSEdgeInsets(top: 0, left: MavioSpacing.s8, bottom: 0, right: 0)))
            }
            views.append(inset(badge, NSEdgeInsets(top: 0, left: MavioSpacing.s8, bottom: 0, right: 0)))
            return WrapStack(views, spacing: MavioSpacing.s4, lineSpacing: 6)
        }

        /// Replaces the actions, with why one failed below them.
        func setActions(_ made: (actions: [NSView], failures: [String])) {
            actions.setViews(made.actions)
            failures.setViews(
                made.failures.map { NSTextField.label($0, theme.fonts.callout, color: .white, lines: 0) })
        }

        /// Shows how far the downloads have come, nil hiding it.
        func setBadge(_ percent: Int?) {
            badgeLabel.stringValue = percent.map { MavioStrings.discoverDownloading(percent: String($0)) } ?? ""
            badge.isHidden = percent == nil
            badge.superview?.isHidden = percent == nil
            needsLayout = true
        }
    }

    /// A header's artwork: the backdrop, else the poster blurred over
    /// `color.surface`, else the surface alone.
    @MainActor
    func heroArtwork(backdrop: String, poster: String, icon: String, theme: Theme) -> NSView {
        if let url = sized(backdrop, 1280) {
            let art = ArtworkView(url, icon: icon, theme: theme)
            art.radius = 0
            return art
        }
        let ground = Box(fill: theme.colors.surface)
        if let url = sized(poster, 342) {
            let art = ArtworkView(url, theme: theme)
            art.radius = 0
            art.showsFill = false
            art.alphaValue = 0.7
            art.wantsLayer = true
            art.layerUsesCoreImageFilters = true
            if let blur = CIFilter(name: "CIGaussianBlur", parameters: [kCIInputRadiusKey: 48]) {
                art.contentFilters = [blur]
            }
            ground.addSubview(art)
            art.translatesAutoresizingMaskIntoConstraints = false
            // Scaled by a quarter, so that the blur's edges fall outside.
            NSLayoutConstraint.activate([
                art.centerXAnchor.constraint(equalTo: ground.centerXAnchor),
                art.centerYAnchor.constraint(equalTo: ground.centerYAnchor),
                art.widthAnchor.constraint(equalTo: ground.widthAnchor, multiplier: 1.25),
                art.heightAnchor.constraint(equalTo: ground.heightAnchor, multiplier: 1.25),
            ])
        }
        return ground
    }

    /// Darkens a header at its start and its foot, under its text.
    @MainActor
    func addHeroGradients(to view: NSView) {
        let black = NSColor.black
        view.addFilling(
            GradientView(
                colors: [black.withAlphaComponent(0.82), black.withAlphaComponent(0.45), .clear],
                locations: [0, 0.45, 0.75], from: CGPoint(x: 0, y: 0.5), to: CGPoint(x: 1, y: 0.5)))
        view.addFilling(
            GradientView(
                colors: [black.withAlphaComponent(0.75), .clear], locations: [0, 0.55], from: CGPoint(x: 0.5, y: 1),
                to: CGPoint(x: 0.5, y: 0)))
    }

    /// A header's poster: 200 pt wide, rounded, over a shadow.
    @MainActor
    func heroPoster(_ url: String, title: String, theme: Theme) -> NSView {
        let art = ArtworkView(url, title: title, theme: theme)
        art.radius = MavioRadius.lg
        art.size(width: 200, height: 300)
        let holder = Box(radius: MavioRadius.lg)
        holder.elevation = MavioNSShadow(
            color: NSColor.black.withAlphaComponent(0.36), x: 0, y: 12, blur: 32, spread: 0)
        holder.addFilling(art)
        return holder
    }

    /// A movie's collection as Seerr's card: its backdrop and name, opening
    /// its page.
    @MainActor
    final class CollectionCard: NSView {
        private let name: NSTextField
        private let art: ArtworkView
        private var task: Task<Void, Never>?

        /// The card of info's collection, nil when it has none.
        init?(page: TitlePage, info: TitleInfo) {
            let id = Int32(info.externalIds["tmdb_collection"] ?? "") ?? 0
            guard id > 0 else { return nil }
            let theme = page.theme
            name = label(
                info.collectionName.isEmpty ? MavioStrings.itemCollection : info.collectionName, theme.fonts.headline,
                weight: .semibold, color: .white)
            art = ArtworkView(nil, icon: "collections", theme: theme)
            art.radius = 0
            art.showsFill = false
            super.init(frame: .zero)
            translatesAutoresizingMaskIntoConstraints = false
            let ground = Box(fill: theme.colors.surfaceRaised, radius: 14)
            ground.wantsLayer = true
            ground.layer?.masksToBounds = true
            ground.addFilling(art)
            ground.addFilling(
                GradientView(
                    colors: [NSColor.black.withAlphaComponent(0.45), NSColor.black.withAlphaComponent(0.8)],
                    from: CGPoint(x: 0.5, y: 0), to: CGPoint(x: 0.5, y: 1)))
            let view = pill(
                MavioStrings.collectionView, theme.fonts.footnote, fill: NSColor.white.withAlphaComponent(0.2),
                color: .white, insets: .symmetric(horizontal: MavioSpacing.s12, vertical: MavioSpacing.s4))
            let row = NSStackView.horizontal([name, Spacer(), view], spacing: MavioSpacing.s12)
            ground.addFilling(row, insets: .all(MavioSpacing.s16))
            ground.heightAnchor.constraint(greaterThanOrEqualToConstant: 80).isActive = true
            let serverId = page.serverId
            let card = Pressable(ground, look: .lift, radius: 14, theme: theme) { [weak page] in
                page?.open(.discoveredCollection(serverId: serverId, tmdbId: id))
            }
            card.setAccessibilityLabel(name.stringValue)
            addFilling(card)
            let model = page.model
            task = Task { [weak self] in
                guard let c = try? await model.collection(serverId, id), let self, !Task.isCancelled else { return }
                if !c.name.isEmpty { self.name.stringValue = c.name }
                self.art.set(sized(c.backdropURL.isEmpty ? c.posterURL : c.backdropURL, 780))
            }
        }

        @available(*, unavailable)
        required init?(coder: NSCoder) { fatalError() }

        isolated deinit { task?.cancel() }
    }
#endif
