#if os(macOS)
    import AppKit
    import MavioMac

    /// A discovered person's page (docs/clients.md §6.3.2), as Seerr's,
    /// opened from the cast and crew of a discovered title: their photo,
    /// dates, other names, links and biography, then the movies and series
    /// they play in and work on, the department they are known for first.
    final class DiscoveredPersonPage: Page {
        private let serverId: String
        private let tmdbId: Int32
        private var person: Mavio_Acquisition_V1_Person?
        private var kinds = 0
        private var more = false
        private var task: Task<Void, Never>?
        private let page = ScrollPage(
            [], spacing: 0, insets: NSEdgeInsets(top: 0, left: 0, bottom: MavioSpacing.s40, right: 0))
        private let sections = NSStackView.vertical([], spacing: 0)

        /// The person of TMDB ID as serverId discovers them.
        init(context: MacContext, serverId: String, tmdbId: Int32) {
            self.serverId = serverId
            self.tmdbId = tmdbId
            super.init(context: context)
        }

        isolated deinit { task?.cancel() }

        override func viewDidLoad() {
            super.viewDidLoad()
            load()
        }

        private func load() {
            let (model, serverId, tmdbId) = (model, serverId, tmdbId)
            task?.cancel()
            task = Task { [weak self] in
                do {
                    let person = try await model.discoveredPerson(serverId, tmdbId)
                    guard let self, !Task.isCancelled else { return }
                    self.person = person
                    self.render(person)
                } catch {
                    guard let self, !Task.isCancelled, !(error is CancellationError), self.person == nil else { return }
                    self.showError(error.reason) { [weak self] in
                        self?.show(LoadingView())
                        self?.load()
                    }
                }
            }
        }

        private func render(_ p: Mavio_Acquisition_V1_Person) {
            title = p.name
            page.setViews([header(p), sections])
            renderSections()
            show(page)
            FocusEngine.active(for: view)?.schedule()
        }

        private func header(_ p: Mavio_Acquisition_V1_Person) -> NSView {
            let side: CGFloat = theme.isTV ? 240 : 176
            let photo = ArtworkView(sized(p.imageURL, 342), title: p.name, icon: "profile", theme: theme)
            photo.radius = side / 2
            photo.size(width: side, height: side)
            var views: [NSView] = [
                NSTextField.label(p.name, theme.fonts.title1, color: theme.colors.textPrimary, lines: 0)
            ]
            let facts = personFacts(p)
            if !facts.isEmpty {
                views.append(
                    NSTextField.label(
                        facts.joined(separator: " | "), theme.fonts.callout, color: theme.colors.textSecondary, lines: 0
                    ))
            }
            if !p.alsoKnownAs.isEmpty {
                views.append(
                    NSTextField.label(
                        MavioStrings.personAlsoKnownAs(names: p.alsoKnownAs.joined(separator: ", ")),
                        theme.fonts.callout,
                        color: theme.colors.textSecondary, lines: 0))
            }
            if let links = externalLinks(personLinks(p), item: nil) { views.append(links) }
            if !p.biography.isEmpty { views.append(biography(p.biography)) }
            let tabs = PillTabs(
                [MavioStrings.personAllKinds, MavioStrings.personMovies, MavioStrings.personSeries], selected: kinds,
                theme: theme)
            tabs.setAccessibilityLabel(MavioStrings.personKinds)
            tabs.onChange = { [weak self] i in
                guard let self else { return }
                self.kinds = i
                self.renderSections()
            }
            views.append(NSStackView.horizontal([tabs, Spacer()], spacing: 0))
            let column = NSStackView.vertical(views, spacing: MavioSpacing.s12)
            for v in views { v.widthAnchor.constraint(equalTo: column.widthAnchor).isActive = true }
            column.widthAnchor.constraint(lessThanOrEqualToConstant: 768).isActive = true
            let fill = column.widthAnchor.constraint(equalToConstant: 768)
            fill.priority = .defaultLow
            fill.isActive = true
            let row = NSStackView.horizontal([photo, column, Spacer()], spacing: MavioSpacing.s24, alignment: .top)
            return margined(RegionView(row), top: MavioSpacing.s24)
        }

        /// The biography, six lines until More shows it whole.
        private func biography(_ text: String) -> NSView {
            let body = NSTextField.label(text, theme.fonts.body, color: theme.colors.textPrimary, lines: more ? 0 : 6)
            let toggle = textLink(
                more ? MavioStrings.personLess : MavioStrings.personMore, theme.fonts.callout, weight: .semibold,
                color: theme.colors.accentText
            ) { [weak self] in
                guard let self, let person = self.person else { return }
                self.more.toggle()
                self.page.setViews([self.header(person), self.sections])
                FocusEngine.active(for: self.view)?.schedule()
            }
            let stack = NSStackView.vertical(
                [body, NSStackView.horizontal([toggle, Spacer()], spacing: 0)], spacing: MavioSpacing.s4)
            for v in stack.arrangedSubviews { v.widthAnchor.constraint(equalTo: stack.widthAnchor).isActive = true }
            return stack
        }

        private func renderSections() {
            guard let p = person else { return }
            let cast = section(MavioStrings.personAppearances, p.cast)
            let crew = section(MavioStrings.personCrew, p.crew)
            let acting = p.knownForDepartment == "Acting" || p.knownForDepartment.isEmpty
            let views = (acting ? [cast, crew] : [crew, cast]).compactMap(\.self)
            sections.setViews([])
            for v in views {
                sections.addArrangedSubview(v)
                v.widthAnchor.constraint(equalTo: sections.widthAnchor).isActive = true
            }
            FocusEngine.active(for: view)?.schedule()
        }

        private func section(_ title: String, _ all: [Mavio_Acquisition_V1_PersonTitle]) -> NSView? {
            let shown = all.filter { kinds == 0 || ($0.title.mediaKind == .series) == (kinds == 2) }
            guard !shown.isEmpty else { return nil }
            let head = NSStackView.horizontal(
                [
                    heading(title),
                    NSTextField.label(String(shown.count), theme.fonts.callout, color: theme.colors.textSecondary),
                ], spacing: MavioSpacing.s8)
            head.alignment = .firstBaseline
            let grid = FlowGrid(minimum: theme.layout.poster, gutter: theme.layout.gutter, rowSpacing: MavioSpacing.s24)
            grid.setItems(shown.map { discoveredCard($0.title, serverId: serverId, role: $0.role, width: nil) })
            let stack = NSStackView.vertical([head, grid], spacing: MavioSpacing.s12)
            grid.widthAnchor.constraint(equalTo: stack.widthAnchor).isActive = true
            return margined(RegionView(stack), top: MavioSpacing.s32)
        }
    }
#endif
