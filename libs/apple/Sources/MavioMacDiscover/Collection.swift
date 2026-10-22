#if os(macOS)
    import AppKit
    import MavioMac

    /// A discovered collection's page (docs/clients.md §6.3.2), as Seerr's:
    /// its backdrop, name, number of movies and what the server has of
    /// them, requesting the movies chosen and administrators blocklisting
    /// all of them, then its overview and its movies in release order.
    final class DiscoveredCollectionPage: Page, TitleBarReaching {
        /// The backdrop goes behind the title bar, as an item's details'.
        var reachesTitleBar: Bool { true }

        private let serverId: String
        private let tmdbId: Int32
        private var collection: Mavio_Acquisition_V1_Collection?
        private var admin = false
        private var blocking = false
        private var failure: String?
        private var observer: Observer?
        private var revision: Int?
        private var task: Task<Void, Never>?
        private var userTask: Task<Void, Never>?
        private let page = ScrollPage(
            [], spacing: 0, insets: NSEdgeInsets(top: 0, left: 0, bottom: MavioSpacing.s40, right: 0))
        private let actions = WrapStack([], spacing: MavioSpacing.s12, lineSpacing: MavioSpacing.s12)
        private let failureLabel: NSTextField
        private var heroHeight: NSLayoutConstraint?

        /// The collection of TMDB ID as serverId discovers it.
        init(context: MacContext, serverId: String, tmdbId: Int32) {
            self.serverId = serverId
            self.tmdbId = tmdbId
            failureLabel = .label("", context.theme.fonts.callout, color: .white, lines: 0)
            super.init(context: context)
        }

        isolated deinit {
            observer?.cancel()
            task?.cancel()
            userTask?.cancel()
        }

        override func viewDidLoad() {
            super.viewDidLoad()
            observer = Observer { [weak self] in
                guard let self else { return }
                let now = self.model.revision(self.serverId)
                guard now != self.revision else { return }
                self.revision = now
                self.load()
            }
            let (model, serverId) = (model, serverId)
            userTask = Task { [weak self] in
                let admin = await RequestsAPI.currentUser(model, serverId)?.admin == true
                guard let self, !Task.isCancelled else { return }
                self.admin = admin
                self.renderActions()
            }
        }

        override func viewDidLayout() {
            super.viewDidLayout()
            if let heroHeight, abs(heroHeight.constant - heroTarget) > 0.5 { heroHeight.constant = heroTarget }
        }

        private var heroTarget: CGFloat {
            let h = view.bounds.height
            return theme.isTV ? max(480, h * 0.86) : min(max(520, h * 0.7), 700)
        }

        /// Reads the collection again.
        private func load() {
            let (model, serverId, tmdbId) = (model, serverId, tmdbId)
            task?.cancel()
            task = Task { [weak self] in
                do {
                    let c = try await model.collection(serverId, tmdbId)
                    guard let self, !Task.isCancelled else { return }
                    self.collection = c
                    self.render(c)
                } catch {
                    guard let self, !Task.isCancelled, !(error is CancellationError), self.collection == nil else {
                        return
                    }
                    self.showError(error.reason) { [weak self] in
                        self?.show(LoadingView())
                        self?.load()
                    }
                }
            }
        }

        private var blocked: Bool { collection.map { collectionStatus($0.parts) == .blocklisted } ?? false }

        private func render(_ c: Mavio_Acquisition_V1_Collection) {
            title = c.name
            var views: [NSView] = [hero(c)]
            if !c.overview.isEmpty {
                views.append(
                    margined(
                        NSTextField.label(c.overview, theme.fonts.body, color: theme.colors.textPrimary, lines: 0),
                        top: MavioSpacing.s24))
            }
            let grid = FlowGrid(minimum: theme.layout.poster, gutter: theme.layout.gutter, rowSpacing: MavioSpacing.s24)
            grid.setItems(c.parts.map { discoveredCard($0, serverId: serverId, width: nil) })
            views.append(margined(RegionView(grid), top: MavioSpacing.s24))
            page.setViews(views)
            renderActions()
            if page.superview == nil {
                show(page)
                if theme.isTV, let first = actions.views.first { FocusEngine.active(for: view)?.prefer(first) }
            }
            FocusEngine.active(for: view)?.schedule()
        }

        private func hero(_ c: Mavio_Acquisition_V1_Collection) -> NSView {
            let hero = FlippedView()
            hero.translatesAutoresizingMaskIntoConstraints = false
            hero.wantsLayer = true
            hero.layer?.masksToBounds = true
            hero.appearance = NSAppearance(named: .darkAqua)
            if let backdrop = sized(c.backdropURL, 1280) {
                let art = ArtworkView(backdrop, icon: "collections", theme: theme)
                art.radius = 0
                hero.addFilling(art)
            } else {
                hero.addFilling(Box(fill: theme.colors.surface))
            }
            hero.addFilling(
                GradientView(
                    colors: [.black, NSColor.black.withAlphaComponent(0.5), .clear], locations: [0, 0.5, 1],
                    from: CGPoint(x: 0.5, y: 1), to: CGPoint(x: 0.5, y: 0)))
            let tv = theme.isTV
            let name = label(c.name, theme.fonts.title1, weight: .bold, color: .white, lines: 2)
            name.font = .systemFont(ofSize: tv ? 72 : 52, weight: .bold)
            let facts = [
                MavioStrings.collectionMovieCount(count: c.parts.count), statusLabel(collectionStatus(c.parts)),
            ]
            .compactMap(\.self)
            let views: [NSView] = [
                name,
                NSTextField.label(
                    facts.joined(separator: " · "), theme.fonts.callout, color: NSColor.white.withAlphaComponent(0.8)),
                actions, failureLabel,
            ]
            let column = NSStackView.vertical(views, spacing: MavioSpacing.s12)
            for v in views.dropFirst() { v.widthAnchor.constraint(equalTo: column.widthAnchor).isActive = true }
            column.widthAnchor.constraint(lessThanOrEqualToConstant: tv ? 960 : 640).isActive = true
            let fill = column.widthAnchor.constraint(equalToConstant: tv ? 960 : 640)
            fill.priority = .defaultLow
            fill.isActive = true
            var row: [NSView] = []
            if !tv, let poster = sized(c.posterURL, 500) { row.append(heroPoster(poster, title: c.name, theme: theme)) }
            row.append(column)
            let content = NSStackView.horizontal(row, spacing: MavioSpacing.s40, alignment: .bottom)
            hero.addSubview(content)
            let side = tv ? theme.layout.margin : MavioSpacing.s40
            NSLayoutConstraint.activate([
                content.leadingAnchor.constraint(equalTo: hero.leadingAnchor, constant: side),
                content.trailingAnchor.constraint(lessThanOrEqualTo: hero.trailingAnchor, constant: -side),
                content.bottomAnchor.constraint(equalTo: hero.bottomAnchor, constant: -MavioSpacing.s40),
                content.topAnchor.constraint(greaterThanOrEqualTo: hero.topAnchor, constant: MavioSpacing.s24),
            ])
            let height = hero.heightAnchor.constraint(equalToConstant: heroTarget)
            height.isActive = true
            heroHeight = height
            return hero
        }

        /// Request, choosing the movies not requested yet, and for
        /// administrators Block or Unblock; below them why blocking failed.
        private func renderActions() {
            guard let c = collection else { return }
            var views: [NSView] = []
            if c.parts.contains(where: \.requestable) {
                views.append(
                    MavioButton(MavioStrings.collectionRequest, icon: "add", kind: .artworkPrimary, theme: theme) {
                        [weak self] in self?.askMovies()
                    })
            }
            if admin {
                let button = MavioButton(
                    blocked ? MavioStrings.collectionUnblock : MavioStrings.collectionBlock,
                    icon: blocked ? "check" : "close", kind: .artworkSecondary, theme: theme
                ) { [weak self] in self?.block() }
                button.isEnabled = !blocking
                views.append(button)
            }
            actions.setViews(views)
            actions.isHidden = views.isEmpty
            failureLabel.stringValue = failure ?? ""
            failureLabel.isHidden = failure == nil
            FocusEngine.active(for: view)?.schedule()
        }

        private func block() {
            let (model, serverId, tmdbId, on) = (model, serverId, tmdbId, !blocked)
            blocking = true
            renderActions()
            Task { [weak self] in
                var failure: String?
                do {
                    try await RequestsAPI.blockCollection(model, serverId, tmdbId, on)
                } catch {
                    failure = error.reason
                }
                guard let self else { return }
                self.blocking = false
                self.failure = failure
                self.renderActions()
                self.load()
            }
        }

        private func askMovies() {
            guard let c = collection else { return }
            CollectionRequestDialog(page: self, serverId: serverId, collection: c) { [weak self] in self?.load() }
                .present()
        }
    }

    /// The dialog choosing the movies of a collection to request, all those
    /// not requested yet by default; requesting each in turn, the first
    /// that fails stopping and saying why.
    @MainActor
    private final class CollectionRequestDialog {
        private let dialog: Dialog
        private weak var page: Page?
        private let serverId: String
        private let collection: Mavio_Acquisition_V1_Collection
        private let reload: @MainActor () -> Void
        private var chosen: Set<Int32>
        private var boxes: [Int32: CheckBox] = [:]
        private var allBox: CheckBox?
        private var busy = false
        private let confirmButton: MavioButton
        private let failureLabel: NSTextField
        private var retained: CollectionRequestDialog?

        /// The dialog requesting movies of collection on page; reload reads
        /// it again once requested.
        init(
            page: Page, serverId: String, collection: Mavio_Acquisition_V1_Collection,
            reload: @escaping @MainActor () -> Void
        ) {
            self.page = page
            self.serverId = serverId
            self.collection = collection
            self.reload = reload
            let theme = page.theme
            chosen = Set(collection.parts.filter(\.requestable).map(\.tmdbId))
            dialog = Dialog(title: MavioStrings.discoverRequestTitle(name: collection.name), width: 460, theme: theme)
            confirmButton = MavioButton("", kind: .primary, theme: theme)
            failureLabel = .label("", theme.fonts.callout, color: theme.colors.danger, lines: 0)
            failureLabel.isHidden = true
            confirmButton.onPress = { [weak self] in self?.submit() }
            var rows: [NSView] = []
            let (allRow, allBox) = checkRow(MavioStrings.collectionAllMovies, on: true, theme: theme) {
                [weak self] on in
                guard let self else { return }
                self.chosen = on ? Set(self.open) : []
                self.restyle()
            }
            self.allBox = allBox
            rows.append(allRow)
            for p in collection.parts {
                let details = [p.year > 0 ? String(p.year) : nil, statusLabel(p.status)].compactMap(\.self)
                let id = p.tmdbId
                let (row, box) = checkRow(
                    p.name, details: details, on: chosen.contains(id), enabled: p.requestable, theme: theme
                ) { [weak self] on in
                    guard let self else { return }
                    if on { self.chosen.insert(id) } else { self.chosen.remove(id) }
                    self.restyle()
                }
                boxes[id] = box
                rows.append(row)
            }
            dialog.setContent([group(nil, rows: rows, theme: theme), failureLabel])
            let cancel = MavioButton(MavioStrings.actionCancel, kind: .secondary, theme: theme) { [weak self] in
                self?.dialog.close()
            }
            dialog.setButtons(trailing: [cancel, confirmButton])
            restyle()
        }

        private var open: [Int32] { collection.parts.filter(\.requestable).map(\.tmdbId) }

        private func restyle() {
            allBox?.state = chosen.count == open.count ? .on : .off
            for (id, box) in boxes { box.state = chosen.contains(id) ? .on : .off }
            confirmButton.title = MavioStrings.collectionRequestMovies(count: chosen.count)
            confirmButton.isEnabled = !busy && !chosen.isEmpty
        }

        /// Shows the dialog over its page.
        func present() {
            guard let page else { return }
            retained = self
            dialog.onClose = { [weak self] in self?.retained = nil }
            dialog.present(over: page)
        }

        private func submit() {
            guard let page else { return }
            let ids = collection.parts.map(\.tmdbId).filter(chosen.contains)
            let (model, serverId) = (page.model, serverId)
            busy = true
            restyle()
            Task { [weak self] in
                var failure: String?
                do {
                    for id in ids { try await RequestsAPI.create(model, serverId, .movie, id, seasons: nil) }
                } catch {
                    failure = error.reason
                }
                guard let self else { return }
                self.busy = false
                self.reload()
                if let failure {
                    self.failureLabel.stringValue = MavioStrings.discoverRequestFailed(reason: failure)
                    self.failureLabel.isHidden = false
                    self.restyle()
                } else {
                    self.dialog.close()
                }
            }
        }
    }
#endif
