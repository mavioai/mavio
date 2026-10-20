#if os(macOS)
    import AppKit
    import MavioMac

    /// A discovered title's whole cast and crew, or its crew alone, as an
    /// item's (libs/design/components.md, Credits Screen): the people of
    /// each job in a grid under its name.
    final class TitleCreditsPage: Page {
        private let serverId: String
        private let kind: MediaKind
        private let tmdbId: Int32
        private let crew: Bool
        private var shown = false
        private var task: Task<Void, Never>?

        /// The cast and crew, or the crew alone, of the title of kind and
        /// TMDB ID as serverId discovers it.
        init(context: MacContext, serverId: String, kind: MediaKind, tmdbId: Int32, crew: Bool) {
            self.serverId = serverId
            self.kind = kind
            self.tmdbId = tmdbId
            self.crew = crew
            super.init(context: context)
        }

        isolated deinit { task?.cancel() }

        override func viewDidLoad() {
            super.viewDidLoad()
            title = crew ? MavioStrings.creditsFullCrew : MavioStrings.itemCast
            load()
        }

        private func load() {
            let (model, serverId, kind, tmdbId, crew) = (model, serverId, kind, tmdbId, crew)
            task?.cancel()
            task = Task { [weak self] in
                do {
                    let all = try await model.title(serverId, kind, tmdbId).credits
                    guard let self, !Task.isCancelled else { return }
                    // The cast comes in billing order.
                    self.render(crew ? all.filter { !$0.isCast } : all)
                } catch {
                    guard let self, !Task.isCancelled, !(error is CancellationError), !self.shown else { return }
                    self.showError(error.reason) { [weak self] in
                        self?.show(LoadingView())
                        self?.load()
                    }
                }
            }
        }

        private func render(_ credits: [TitleCredit]) {
            shown = true
            var views: [NSView] = []
            if theme.isTV {
                views.append(
                    margined(NSTextField.label(title ?? "", theme.fonts.title1, color: theme.colors.textPrimary)))
            }
            if credits.isEmpty {
                views.append(
                    margined(
                        NSTextField.label(MavioStrings.creditsNone, theme.fonts.body, color: theme.colors.textSecondary)
                    ))
            }
            for job in [CreditJob.actor, .guestStar] + CreditJob.crewOrder {
                let people = credits.filter { $0.kind.job == job }
                guard !people.isEmpty else { continue }
                let grid = FlowGrid(
                    minimum: theme.isTV ? 160 : 104, gutter: MavioSpacing.s16, rowSpacing: MavioSpacing.s24)
                grid.setItems(people.map { creditCard($0, serverId: serverId) })
                let section = NSStackView.vertical([heading(job.label), grid], spacing: MavioSpacing.s12)
                grid.widthAnchor.constraint(equalTo: section.widthAnchor).isActive = true
                views.append(margined(RegionView(section)))
            }
            let page = ScrollPage(
                [], spacing: MavioSpacing.s24,
                insets: NSEdgeInsets(top: MavioSpacing.s16, left: 0, bottom: MavioSpacing.s16, right: 0))
            page.setViews(views)
            show(page)
            FocusEngine.active(for: view)?.schedule()
        }
    }
#endif
