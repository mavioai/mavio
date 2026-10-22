#if os(macOS)
    import AppKit
    import MavioMac

    /// What the primary action plays: the item itself, or a series' or
    /// season's episode to watch next.
    struct NextUp {
        let itemId: String
        let name: String
        let position: TimeInterval?
        let progress: Float?
        let runtime: TimeInterval?
        let played: Bool
        /// Whether it is the item shown.
        let own: Bool
        /// The episode's numbers, as "S1 E2", when it is not the item shown.
        let episode: String?

        /// Where it resumes, when it is partly watched.
        var resumable: Bool { (position ?? 0) > 0 && !played }

        /// How far it was watched, from 0 to 1.
        var fraction: Float? {
            if let progress, progress > 0 { return progress }
            guard let position, let runtime, runtime > 0 else { return nil }
            return Float(min(position / runtime, 1))
        }

        /// The time left.
        var left: TimeInterval? {
            guard let runtime, let position else { return nil }
            return max(60, runtime - position)
        }

        /// When it would end if played now.
        var endsAt: Date? {
            guard let runtime else { return nil }
            let left = runtime - (resumable ? position ?? 0 : 0)
            return left > 0 ? .now.addingTimeInterval(left) : nil
        }
    }

    /// An item's details, as the web UI's (libs/design/components.md,
    /// Details Header and Details Screen): the header with what to watch and
    /// how, a series' episodes by season, an episode's season, the cast, the
    /// extras, the collection, items like it, and its details and media. In
    /// television mode the backdrop fills 86 % of the page's height and
    /// focus starts on the primary action (docs/tv.md §6.2).
    final class DetailsPage: Page, TitleBarReaching {
        /// The backdrop goes behind the title bar.
        var reachesTitleBar: Bool { true }

        let serverId: String
        let itemId: String
        private(set) var details: Details?
        private var observer: Observer?
        private var loadTask: Task<Void, Never>?
        private var reportTask: Task<Void, Never>?
        private var revision: Int?
        /// The version of this server chosen to play.
        var sourceIndex = 0
        /// Whether the user may report problems on the server.
        private var mayReport = false
        /// Whether the user administers the server, who may manage the
        /// item's metadata.
        private var admin = false
        /// Counts the issues reported here, each listing them again.
        var reports = 0
        private let page = ScrollPage([], spacing: 0, insets: NSEdgeInsets(top: 0, left: 0, bottom: 40, right: 0))
        private var header: NSView?
        private var headerHeight: NSLayoutConstraint?
        private var actionsHolder: NSStackView?
        private var downloadHolder: NSView?
        private var downloadState: String?

        override var assistantScreen: AssistantScreen? {
            AssistantScreen("item", serverId: serverId, itemId: itemId)
        }

        /// The details of an item of a server.
        init(serverId: String, itemId: String, context: MacContext) {
            self.serverId = serverId
            self.itemId = itemId
            super.init(context: context)
        }

        override func viewDidLoad() {
            super.viewDidLoad()
            show(LoadingView())
            observer = Observer { [weak self] in self?.observe() }
        }

        private func observe() {
            let now = model.revision(serverId)
            let downloads = model.downloads.entries
            if now != revision {
                revision = now
                load()
            } else {
                _ = downloads
                refreshDownload()
            }
        }

        /// Reads the item again.
        func load() {
            loadTask?.cancel()
            loadTask = Task { [weak self] in
                guard let self else { return }
                do {
                    let loaded = try await self.model.client.details(serverId: self.serverId, itemId: self.itemId)
                    guard !Task.isCancelled else { return }
                    let first = self.details == nil
                    self.details = loaded
                    self.build(scrollToTop: false)
                    if first { self.loadMayReport() }
                } catch is CancellationError {
                } catch {
                    guard !Task.isCancelled, self.details == nil else { return }
                    self.showError(error.reason) { [weak self] in self?.load() }
                }
            }
        }

        private func loadMayReport() {
            guard let details, !details.offline else { return }
            reportTask = Task { [weak self] in
                guard let self else { return }
                let (admin, may) = await self.model.userRights(self.serverId)
                guard !Task.isCancelled else { return }
                if may != self.mayReport || admin != self.admin {
                    (self.admin, self.mayReport) = (admin, may)
                    self.rebuildActions()
                }
            }
        }

        // MARK: Derived

        var card: Card? { details?.card }
        /// The files of this server.
        var own: [Source] { details.map { d in d.sources.filter { $0.serverId == d.card.serverId } } ?? [] }
        /// The same title on the user's other servers.
        var copies: [Source] { details.map { d in d.sources.filter { $0.serverId != d.card.serverId } } ?? [] }
        var source: Source? { own.indices.contains(sourceIndex) ? own[sourceIndex] : own.first }
        private var inSeries: Bool { card?.kind == .episode || card?.kind == .season }

        var next: NextUp? {
            guard let details else { return nil }
            let card = details.card
            if card.kind == .series || card.kind == .season {
                return details.next.map { e in
                    NextUp(
                        itemId: e.card.itemId, name: e.card.title, position: e.position, progress: e.card.progress,
                        runtime: e.card.runtime, played: e.card.played, own: false, episode: short(e))
                }
            }
            guard source != nil else { return nil }
            return NextUp(
                itemId: card.itemId, name: card.title, position: details.position, progress: card.progress,
                runtime: card.runtime, played: card.played, own: true, episode: nil)
        }

        private var download: DownloadEntry? {
            model.downloads.entry(serverId: serverId, itemId: itemId)
        }

        // MARK: Building

        private func build(scrollToTop: Bool) {
            guard let details else { return }
            title = details.card.title
            if page.superview == nil { show(page) }
            let header = makeHeader(details)
            self.header = header
            var views: [NSView] = [header]
            let sections = DetailsSections(page: self, details: details)
            let body = NSStackView.vertical(sections.views, spacing: MavioSpacing.s8)
            for v in body.arrangedSubviews { v.widthAnchor.constraint(equalTo: body.widthAnchor).isActive = true }
            views.append(inset(body, NSEdgeInsets(top: MavioSpacing.s24, left: 0, bottom: 0, right: 0)))
            page.setViews(views)
            if scrollToTop { page.scrollToTop() }
            if let engine = FocusEngine.active(for: view) {
                if let first = actionsHolder?.arrangedSubviews.first?.subviews.first as? Focusable {
                    engine.prefer(first)
                }
                engine.schedule()
            }
        }

        /// The header's height: 70 % of the page's, from 520 to 700 pt, on
        /// the desktop; 86 % in television mode.
        private var headerTarget: CGFloat {
            let h = view.bounds.height
            return theme.isTV ? max(480, h * 0.86) : min(max(520, h * 0.7), 700)
        }

        override func viewDidLayout() {
            super.viewDidLayout()
            if let headerHeight, headerHeight.constant != headerTarget { headerHeight.constant = headerTarget }
        }

        /// The artwork, and over its lower left the series an episode or
        /// season belongs to, the logo or title, the facts and badges, the
        /// tagline, overview and actions.
        private func makeHeader(_ details: Details) -> NSView {
            let tv = theme.isTV
            let card = details.card
            let header = FlippedView()
            header.translatesAutoresizingMaskIntoConstraints = false
            header.wantsLayer = true
            header.layer?.masksToBounds = true
            header.appearance = NSAppearance(named: .darkAqua)
            let height = header.heightAnchor.constraint(equalToConstant: headerTarget)
            height.isActive = true
            headerHeight = height
            let image = details.backdrop ?? (card.kind == .episode ? card.thumb : nil)
            if let image {
                let art = ArtworkView(image.url, title: card.title, icon: card.kind.icon, theme: theme)
                art.setAccessibilityElement(false)
                header.addFilling(art)
            } else {
                header.addFilling(Box(fill: theme.colors.surface))
                if let poster = card.poster {
                    // Without a backdrop the poster, blurred, colors the header.
                    let art = ArtworkView(poster.url, theme: theme)
                    art.showsFill = false
                    art.alphaValue = 0.7
                    art.setAccessibilityElement(false)
                    art.layerUsesCoreImageFilters = true
                    if let blur = CIFilter(name: "CIGaussianBlur", parameters: [kCIInputRadiusKey: 48]) {
                        art.contentFilters = [blur]
                    }
                    header.addSubview(art)
                    NSLayoutConstraint.activate([
                        art.centerXAnchor.constraint(equalTo: header.centerXAnchor),
                        art.centerYAnchor.constraint(equalTo: header.centerYAnchor),
                        art.widthAnchor.constraint(equalTo: header.widthAnchor, multiplier: 1.25),
                        art.heightAnchor.constraint(equalTo: header.heightAnchor, multiplier: 1.25),
                    ])
                }
            }
            if tv {
                header.addFilling(
                    GradientView(
                        colors: [.black.withAlphaComponent(0.65), .clear], locations: [0, 0.65],
                        from: CGPoint(x: 0, y: 0.5), to: CGPoint(x: 1, y: 0.5)))
                header.addFilling(
                    GradientView(
                        colors: [theme.colors.tvBackground, theme.colors.tvBackground.withAlphaComponent(0.7), .clear],
                        locations: [0, 0.3, 0.75], from: CGPoint(x: 0.5, y: 1), to: CGPoint(x: 0.5, y: 0)))
            } else {
                header.addFilling(
                    GradientView(
                        colors: [.black.withAlphaComponent(0.82), .black.withAlphaComponent(0.45), .clear],
                        locations: [0, 0.45, 0.75], from: CGPoint(x: 0, y: 0.5), to: CGPoint(x: 1, y: 0.5)))
                header.addFilling(
                    GradientView(
                        colors: [.black.withAlphaComponent(0.75), .clear], locations: [0, 0.55],
                        from: CGPoint(x: 0.5, y: 1), to: CGPoint(x: 0.5, y: 0)))
            }
            var row: [NSView] = []
            if !tv, image == nil, let poster = card.poster {
                let art = ArtworkView(poster.url, title: card.title, icon: card.kind.icon, theme: theme)
                art.radius = MavioRadius.lg
                art.size(width: 200).aspect(2 / 3)
                let holder = Box(radius: MavioRadius.lg)
                holder.elevation = MavioNSShadow(
                    color: .black.withAlphaComponent(0.36), x: 0, y: 12, blur: 32, spread: 0)
                holder.addFilling(art)
                row.append(holder)
            }
            let text = headerText(details)
            row.append(text)
            let stack = NSStackView.horizontal(row, spacing: MavioSpacing.s40, alignment: .bottom)
            header.addSubview(stack)
            let side = tv ? theme.layout.margin : MavioSpacing.s40
            NSLayoutConstraint.activate([
                stack.leadingAnchor.constraint(equalTo: header.leadingAnchor, constant: side),
                stack.trailingAnchor.constraint(lessThanOrEqualTo: header.trailingAnchor, constant: -side),
                stack.bottomAnchor.constraint(equalTo: header.bottomAnchor, constant: tv ? -MavioSpacing.s48 : -40),
                stack.topAnchor.constraint(greaterThanOrEqualTo: header.topAnchor, constant: MavioSpacing.s24),
                text.widthAnchor.constraint(lessThanOrEqualToConstant: tv ? 960 : 640),
            ])
            return header
        }

        private func headerText(_ details: Details) -> NSView {
            let tv = theme.isTV
            let card = details.card
            let width: CGFloat = tv ? 960 : 640
            var views: [NSView] = []
            let faint = NSColor.white.withAlphaComponent(0.85)
            if let series = details.series {
                let name = NSTextField.label(series.name, theme.fonts.headline, color: faint)
                let content: NSView =
                    series.logo.map {
                        LogoView($0.url, maxWidth: 240, maxHeight: 48, label: series.name, fallback: name)
                    } ?? name
                views.append(
                    Pressable(content, look: .highlight, radius: MavioRadius.sm, theme: theme) { [weak self] in
                        self?.open(.item(serverId: card.serverId, itemId: series.itemId))
                    })
            }
            if let season = details.seasonNumber, let episode = details.episodeNumber {
                views.append(
                    NSTextField.label(
                        MavioStrings.itemEpisodeNumber(season: String(season), episode: String(episode)),
                        theme.fonts.headline, color: faint))
            }
            let title = NSTextField.label(card.title, theme.fonts.title1, color: .white, lines: 0)
            title.font = .systemFont(ofSize: tv ? 72 : 52, weight: .bold)
            title.preferredMaxLayoutWidth = width
            if !inSeries, let logo = details.logo {
                views.append(
                    LogoView(
                        logo.url, maxWidth: tv ? 640 : 320, maxHeight: tv ? 160 : 112, label: card.title,
                        fallback: title))
            } else {
                views.append(title)
            }
            let facts = facts(details)
            if !facts.isEmpty {
                // Each separator keeps to the fact before it, so that no line starts with one.
                let text = facts.map { $0.replacingOccurrences(of: " ", with: "\u{00a0}") }.joined(
                    separator: "\u{00a0}· ")
                let label = NSTextField.label(
                    text, theme.fonts.callout, color: .white.withAlphaComponent(0.8), lines: 0)
                label.preferredMaxLayoutWidth = width
                views.append(label)
            }
            let badges = badges(details)
            if !badges.isEmpty || model.signedIn.count > 1 {
                var marks: [NSView] = badges.map { badge in
                    let box = Box(radius: 4)
                    box.border = .white.withAlphaComponent(0.5)
                    box.addFilling(
                        label(badge, theme.fonts.caption, weight: .semibold, color: .white.withAlphaComponent(0.9)),
                        insets: .symmetric(horizontal: 6, vertical: 1))
                    return box
                }
                if model.signedIn.count > 1 {
                    let box = Box(fill: .white.withAlphaComponent(0.2), radius: 4)
                    box.addFilling(
                        NSTextField.label(
                            details.serverName, theme.fonts.caption, color: .white.withAlphaComponent(0.8)),
                        insets: .symmetric(horizontal: 6, vertical: 1))
                    marks.append(box)
                }
                views.append(WrapStack(marks, spacing: MavioSpacing.s12, lineSpacing: 6))
            }
            if !tv, let tagline = details.tagline {
                let label = NSTextField.label(
                    tagline, theme.fonts.headline, color: .white.withAlphaComponent(0.9), lines: 0)
                label.preferredMaxLayoutWidth = width
                views.append(label)
            }
            if let overview = details.overview {
                let label = NSTextField.label(
                    overview, theme.fonts.body, color: .white.withAlphaComponent(0.85), lines: tv ? 2 : 3)
                label.preferredMaxLayoutWidth = width
                if tv {
                    views.append(label)
                } else {
                    let button = Pressable(label, look: .highlight, radius: MavioRadius.sm, theme: theme) {
                        [weak self] in self?.openOverview()
                    }
                    button.setAccessibilityLabel(overview)
                    views.append(button)
                }
            }
            let actions = NSStackView.vertical([], spacing: MavioSpacing.s12)
            actionsHolder = actions
            fillActions(actions)
            let stack = NSStackView.vertical(views, spacing: MavioSpacing.s12)
            stack.addArrangedSubview(actions)
            stack.setCustomSpacing(MavioSpacing.s16, after: views[views.count - 1])
            for v in views where v is WrapStack {
                v.widthAnchor.constraint(equalTo: stack.widthAnchor).isActive = true
            }
            actions.widthAnchor.constraint(equalTo: stack.widthAnchor).isActive = true
            let w = stack.widthAnchor.constraint(equalToConstant: width)
            w.priority = .defaultLow
            w.isActive = true
            return stack
        }

        /// The year, or a series' years and seasons, a season's episodes, an
        /// episode's air date, the runtime and up to three genres.
        private func facts(_ details: Details) -> [String] {
            let card = details.card
            var facts: [String] = []
            switch card.kind {
            case .series:
                if let years = years(details) { facts.append(years) }
                if !details.children.isEmpty {
                    facts.append(MavioStrings.itemSeasonCount(count: details.children.count))
                }
            case .season:
                if let year = card.year { facts.append(String(year)) }
                if !details.episodes.isEmpty {
                    facts.append(MavioStrings.itemEpisodeCount(count: details.episodes.count))
                }
            case .episode:
                if let premiere = details.premiere {
                    facts.append(premiere.formatted(date: .abbreviated, time: .omitted))
                }
            default:
                if let year = card.year { facts.append(String(year)) }
            }
            if card.kind != .series, let runtime = card.runtime { facts.append(minutesText(runtime)) }
            if card.kind != .episode, !details.genres.isEmpty {
                facts.append(details.genres.prefix(3).joined(separator: " / "))
            }
            return facts
        }

        /// The years a series ran, as "2023–" while it continues.
        private func years(_ details: Details) -> String? {
            guard let from = details.card.year else { return nil }
            let to = details.endDate.map { Calendar.current.component(.year, from: $0) }
            if let to, to != from { return "\(from)–\(to)" }
            return details.seriesStatus == .continuing ? "\(from)–" : String(from)
        }

        /// The official rating and the chosen version's resolution, range,
        /// surround and subtitles.
        private func badges(_ details: Details) -> [String] {
            [
                details.officialRating, source?.resolution, source?.range, source?.surround,
                source?.subtitles.isEmpty == false ? "CC" : nil,
            ].compactMap(\.self)
        }

        // MARK: Actions

        private func rebuildActions() {
            guard let actionsHolder else { return }
            fillActions(actionsHolder)
            FocusEngine.active(for: view)?.schedule()
        }

        /// Play, or what plays next, then as circles Play from Beginning, the
        /// trailer, favorite, played, the version menu, the download and
        /// More; under them the progress, the episode's name, the time left
        /// and when it would end.
        private func fillActions(_ holder: NSStackView) {
            guard let details else { return }
            let card = details.card
            var buttons: [NSView] = []
            let next = next
            if let next {
                let title =
                    if let episode = next.episode {
                        next.resumable
                            ? MavioStrings.itemResumeEpisode(episode: episode)
                            : MavioStrings.itemPlayEpisode(episode: episode)
                    } else {
                        next.resumable ? MavioStrings.itemResume : MavioStrings.itemPlay
                    }
                buttons.append(
                    MavioButton(title, icon: "play", kind: .artworkPrimary, large: true, theme: theme) {
                        [weak self] in self?.play(next, start: next.resumable ? next.position : nil)
                    })
                if next.resumable {
                    let restart = MavioButton(icon: "previous", kind: .artworkCircle, large: true, theme: theme) {
                        [weak self] in self?.play(next, start: 0)
                    }
                    restart.setAccessibilityLabel(MavioStrings.itemFromStart)
                    restart.toolTip = theme.isTV ? nil : MavioStrings.itemFromStart
                    buttons.append(restart)
                }
            }
            if let trailer = details.extras.first(where: { $0.kind == .trailer }) {
                buttons.append(
                    circle("movies", label: MavioStrings.itemTrailer) { [weak self] in
                        self?.context.play(
                            PlayRequest(
                                serverId: trailer.card.serverId, itemId: trailer.card.itemId, title: trailer.card.title)
                        )
                    })
            } else if !theme.isTV, let url = details.remoteTrailer.flatMap(URL.init(string:)) {
                buttons.append(circle("movies", label: MavioStrings.itemTrailer) { openWeb(url) })
            }
            let model = model
            let heart = IconView(
                "favorite", fill: card.favorite, size: 20,
                color: card.favorite ? NSColor(red: 1, green: 0.27, blue: 0.23, alpha: 1) : .white)
            buttons.append(
                circleButton(heart, overArtwork: true, theme: theme, label: MavioStrings.itemFavorite) {
                    Task { try? await model.setFavorite(card, !card.favorite) }
                })
            buttons.append(
                circleButton(
                    IconView("played", fill: card.played, size: 20, color: .white), overArtwork: true, theme: theme,
                    label: card.played ? MavioStrings.itemMarkUnplayed : MavioStrings.itemMarkPlayed
                ) {
                    Task { try? await model.setPlayed(card, !card.played) }
                })
            if own.count > 1 || !copies.isEmpty { buttons.append(versionPill()) }
            if source != nil, !details.offline {
                let holder = NSView()
                holder.translatesAutoresizingMaskIntoConstraints = false
                downloadHolder = holder
                fillDownload(holder)
                buttons.append(holder)
            } else {
                downloadHolder = nil
            }
            // The More menu: Watch Together, the metadata actions of
            // administrators, and reporting a problem with the item, in
            // groups apart, each named without an ellipsis as the web UI's.
            let groupable = !details.offline && playable(card.kind)
            let reportable = mayReport && [.movie, .series, .season, .episode].contains(card.kind)
            let actions = admin ? MetadataAction.of(itemKind(card.kind)) : []
            if groupable || reportable || !actions.isEmpty {
                let more = MavioButton(icon: "more", kind: .artworkCircle, large: true, theme: theme)
                more.setAccessibilityLabel(MavioStrings.listsMore)
                more.onPress = { [weak self, weak more] in
                    guard let self, let more else { return }
                    let menu = NSMenu()
                    if groupable {
                        menu.addItem(MenuAction(MavioStrings.syncplayWatchTogether) { [weak self] in self?.together() })
                    }
                    if !actions.isEmpty {
                        if menu.numberOfItems > 0 { menu.addItem(.separator()) }
                        for action in actions {
                            menu.addItem(MenuAction(action.title) { [weak self] in self?.metadata(action) })
                        }
                    }
                    if reportable {
                        if menu.numberOfItems > 0 { menu.addItem(.separator()) }
                        menu.addItem(MenuAction(menuName(MavioStrings.issuesReport)) { [weak self] in self?.report() })
                    }
                    popUp(menu, under: more)
                }
                buttons.append(more)
            }
            var rows: [NSView] = [WrapStack(buttons, spacing: MavioSpacing.s12, lineSpacing: MavioSpacing.s12)]
            if let next, let status = status(next) { rows.append(status) }
            holder.setViews(rows)
            if let wrap = rows.first { wrap.widthAnchor.constraint(equalTo: holder.widthAnchor).isActive = true }
        }

        private func circle(_ icon: String, label: String, action: @escaping @MainActor () -> Void) -> MavioButton {
            let button = MavioButton(icon: icon, kind: .artworkCircle, large: true, theme: theme, action: action)
            button.setAccessibilityLabel(label)
            if !theme.isTV { button.toolTip = label }
            return button
        }

        private func status(_ next: NextUp) -> NSView? {
            let parts = [
                next.episode == nil ? nil : next.name,
                next.resumable ? next.left.map { MavioStrings.itemTimeLeft(time: minutesText($0)) } : nil,
                next.endsAt.map { MavioStrings.itemEndsAt(time: $0.formatted(date: .omitted, time: .shortened)) },
            ].compactMap(\.self)
            var views: [NSView] = []
            if next.resumable, let progress = next.fraction {
                let track = Box(fill: .white.withAlphaComponent(0.35), radius: 2)
                track.size(width: 160, height: 4)
                let bar = Box(fill: .white, radius: 2)
                track.addSubview(bar)
                NSLayoutConstraint.activate([
                    bar.leadingAnchor.constraint(equalTo: track.leadingAnchor),
                    bar.topAnchor.constraint(equalTo: track.topAnchor),
                    bar.bottomAnchor.constraint(equalTo: track.bottomAnchor),
                    bar.widthAnchor.constraint(equalToConstant: 160 * CGFloat(progress)),
                ])
                views.append(track)
            }
            if !parts.isEmpty {
                views.append(
                    NSTextField.label(
                        parts.joined(separator: " · "), theme.fonts.footnote, color: .white.withAlphaComponent(0.8)))
            }
            return views.isEmpty ? nil : NSStackView.horizontal(views, spacing: MavioSpacing.s12)
        }

        /// The version to play, then the same title on the user's other
        /// servers, in a menu under a pill naming the chosen one.
        private func versionPill() -> NSView {
            let name = source?.versionName ?? details?.serverName ?? ""
            let row = NSStackView.horizontal(
                [
                    label(
                        MavioStrings.itemVersion, theme.fonts.callout, weight: .semibold,
                        color: .white.withAlphaComponent(0.7)),
                    label(name, theme.fonts.callout, weight: .semibold, color: .white),
                    IconView("expand", size: 14, color: .white),
                ], spacing: MavioSpacing.s8)
            let pill = Pressable(row, look: .highlight, radius: 999, insets: .symmetric(horizontal: 16), theme: theme)
            pill.restingFill = .white.withAlphaComponent(0.16)
            pill.size(height: theme.isTV ? 72 : 44)
            pill.setAccessibilityLabel("\(MavioStrings.itemVersion) \(name)")
            pill.onPress = { [weak self, weak pill] in
                guard let self, let pill else { return }
                popUp(self.versionMenu(), under: pill)
            }
            return pill
        }

        private func versionMenu() -> NSMenu {
            let menu = NSMenu()
            let own = own
            if own.count > 1 {
                menu.addItem(.sectionHeader(title: MavioStrings.itemVersions))
                for (i, s) in own.enumerated() {
                    let item = MenuAction(s.versionName) { [weak self] in
                        guard let self else { return }
                        self.sourceIndex = i
                        self.build(scrollToTop: false)
                    }
                    item.state = i == sourceIndex ? .on : .off
                    item.subtitle = versionDetails(s)
                    menu.addItem(item)
                }
            }
            if !copies.isEmpty {
                if own.count > 1 { menu.addItem(.separator()) }
                for copy in copies {
                    let item = MenuAction(MavioStrings.itemOnServer(server: copy.serverName)) { [weak self] in
                        self?.open(.item(serverId: copy.serverId, itemId: copy.itemId))
                    }
                    item.image = NSImage(systemSymbolName: "server.rack", accessibilityDescription: nil)
                    menu.addItem(item)
                }
            }
            return menu
        }

        private func versionDetails(_ s: Source) -> String {
            [s.resolution, s.range, s.surround, s.format, s.sizeText].compactMap(\.self).joined(separator: " · ")
        }

        // MARK: Download

        private func refreshDownload() {
            guard let holder = downloadHolder else { return }
            let entry = download
            let state = entry.map { "\($0.state)/\($0.received)" } ?? "none"
            guard state != downloadState else { return }
            fillDownload(holder)
        }

        private func fillDownload(_ holder: NSView) {
            let entry = download
            downloadState = entry.map { "\($0.state)/\($0.received)" } ?? "none"
            let focused = FocusEngine.active(for: view)?.focused?.isDescendant(of: holder) == true
            holder.subviews.forEach { $0.removeFromSuperview() }
            let content: NSView
            switch entry?.state {
            case .running?, .queued?:
                let progress = NSProgressIndicator()
                progress.style = .spinning
                progress.isIndeterminate = entry?.state == .queued || (entry?.total ?? 0) == 0
                progress.minValue = 0
                progress.maxValue = 1
                if let entry, entry.total > 0 { progress.doubleValue = Double(entry.received) / Double(entry.total) }
                progress.controlSize = .small
                progress.appearance = NSAppearance(named: .darkAqua)
                if progress.isIndeterminate { progress.startAnimation(nil) }
                progress.translatesAutoresizingMaskIntoConstraints = false
                progress.size(width: 20, height: 20)
                content = progress
            case .done?:
                content = IconView("downloads", fill: true, size: 20, color: .white)
            default:
                content = IconView("downloads", size: 20, color: .white)
            }
            let button = circleButton(content, overArtwork: true, theme: theme, label: MavioStrings.itemDownload) {}
            button.onPress = { [weak self, weak button] in
                guard let self, let button else { return }
                popUp(self.downloadMenu(), under: button)
            }
            holder.addFilling(button)
            if focused { FocusEngine.active(for: view)?.focus(button) }
        }

        private func downloadMenu() -> NSMenu {
            let menu = NSMenu()
            let downloads = model.downloads
            if let entry = download {
                if entry.state == .paused || entry.state == .failed {
                    menu.addItem(
                        MenuAction(MavioStrings.downloadResume) { Task { try? await downloads.resume(entry) } })
                }
                menu.addItem(MenuAction(MavioStrings.itemRemoveDownload) { Task { await downloads.remove(entry) } })
            } else {
                let serverId = serverId
                let itemId = itemId
                menu.addItem(
                    MenuAction(MavioStrings.itemDownload) {
                        Task { try? await downloads.add(serverId: serverId, itemId: itemId) }
                    })
            }
            return menu
        }

        // MARK: Playing

        private func play(_ next: NextUp, start: TimeInterval?) {
            guard let card else { return }
            if next.own, let source {
                let here = source.itemId == card.itemId
                let file = here ? download.flatMap(model.downloads.file) : nil
                context.play(
                    PlayRequest(
                        serverId: source.serverId, itemId: source.itemId, title: card.title,
                        mediaSourceId: source.mediaSourceId, start: here ? start : nil, file: file))
            } else {
                context.play(PlayRequest(serverId: card.serverId, itemId: next.itemId, title: next.name, start: start))
            }
        }

        func short(_ e: Episode) -> String? {
            guard let number = e.number else { return nil }
            return MavioStrings.itemEpisodeShort(season: String(e.season ?? 0), episode: String(number))
        }

        // MARK: Sheets

        /// The tagline and the whole overview, in a sheet.
        private func openOverview() {
            guard let details else { return }
            presentAsSheet(OverviewSheet(details: details, theme: theme))
        }

        /// Reports a problem with the item, in a sheet.
        /// Opens Watch Together, where the item plays in the group from
        /// where it was left.
        private func together() {
            guard let details, let server = model.server(serverId) else { return }
            let position = next?.own == true && next?.resumable == true ? next?.position : nil
            SyncPlayDialog(context: context, server: server, item: (itemId, details.card.title, position))
                .present(over: self)
        }

        /// Opens the dialog of a metadata action on the item.
        private func metadata(_ action: MetadataAction) {
            guard let details else { return }
            openMetadata(action, serverId: serverId, itemId: itemId, name: details.card.title, over: self) {}
        }

        private func report() {
            guard let details, let server = model.server(serverId) else { return }
            presentAsSheet(
                ReportIssueSheet(
                    server: server, itemId: itemId, name: details.card.title, series: details.card.kind == .series,
                    context: context
                ) { [weak self] in
                    guard let self else { return }
                    self.reports += 1
                    self.build(scrollToTop: false)
                })
        }

        isolated deinit {
            observer?.cancel()
            loadTask?.cancel()
            reportTask?.cancel()
        }
    }

    /// The tagline and the whole overview of an item.
    private final class OverviewSheet: NSViewController {
        private let details: Details
        private let theme: Theme

        init(details: Details, theme: Theme) {
            self.details = details
            self.theme = theme
            super.init(nibName: nil, bundle: nil)
            title = details.card.title
        }

        @available(*, unavailable)
        required init?(coder: NSCoder) { fatalError() }

        override func loadView() {
            var views: [NSView] = [
                NSTextField.label(details.card.title, theme.fonts.title3, color: theme.colors.textPrimary, lines: 0)
            ]
            if let tagline = details.tagline {
                views.append(
                    NSTextField.label(tagline, theme.fonts.headline, color: theme.colors.textPrimary, lines: 0))
            }
            if let overview = details.overview {
                views.append(
                    NSTextField.label(
                        overview, theme.fonts.body, color: theme.colors.textSecondary, lines: 0, selectable: true))
            }
            for case let label as NSTextField in views { label.preferredMaxLayoutWidth = 472 }
            let stack = NSStackView.vertical(views, spacing: MavioSpacing.s12)
            let page = ScrollPage([stack], insets: .all(MavioSpacing.s24))
            let done = MavioButton(MavioStrings.actionDone, kind: .primary, theme: theme) { [weak self] in
                guard let self else { return }
                self.presentingViewController?.dismiss(self)
            }
            let root = NSView(frame: NSRect(x: 0, y: 0, width: 520, height: 420))
            root.addSubview(page)
            root.addSubview(done)
            NSLayoutConstraint.activate([
                page.topAnchor.constraint(equalTo: root.topAnchor),
                page.leadingAnchor.constraint(equalTo: root.leadingAnchor),
                page.trailingAnchor.constraint(equalTo: root.trailingAnchor),
                page.bottomAnchor.constraint(equalTo: done.topAnchor, constant: -MavioSpacing.s12),
                done.trailingAnchor.constraint(equalTo: root.trailingAnchor, constant: -MavioSpacing.s24),
                done.bottomAnchor.constraint(equalTo: root.bottomAnchor, constant: -MavioSpacing.s16),
                root.widthAnchor.constraint(equalToConstant: 520),
                root.heightAnchor.constraint(equalToConstant: 420),
            ])
            view = root
        }
    }
#endif
