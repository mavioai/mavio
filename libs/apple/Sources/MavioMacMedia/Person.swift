#if os(macOS)
    import AppKit
    import MavioMac

    /// A person's page (docs/clients.md §6.3.19), as the web UI's, opened
    /// from the cast and crew of an item's details: their photo, dates,
    /// biography and links, and the movies, series and episodes of the
    /// server crediting them. Television mode opens no links to the web.
    final class PersonPage: Page {
        private let serverId: String
        private let personId: String
        private var person: PersonDetails?
        private var more = false
        private var observer: Observer?
        private var loadTask: Task<Void, Never>?
        private var revision: Int?
        private let page = ScrollPage(
            [], spacing: 0, insets: NSEdgeInsets(top: 0, left: 0, bottom: MavioSpacing.s40, right: 0))

        /// The person of a server.
        init(serverId: String, personId: String, context: MacContext) {
            self.serverId = serverId
            self.personId = personId
            super.init(context: context)
        }

        override func viewDidLoad() {
            super.viewDidLoad()
            show(LoadingView())
            observer = Observer { [weak self] in
                guard let self else { return }
                let now = self.model.revision(self.serverId)
                guard now != self.revision else { return }
                self.revision = now
                self.load()
            }
        }

        private func load() {
            loadTask?.cancel()
            loadTask = Task { [weak self] in
                guard let self else { return }
                do {
                    let person = try await self.model.client.person(serverId: self.serverId, personId: self.personId)
                    guard !Task.isCancelled else { return }
                    self.person = person
                    self.title = person.name
                    self.render()
                } catch is CancellationError {
                } catch {
                    guard !Task.isCancelled, self.person == nil else { return }
                    self.showError(error.reason) { [weak self] in self?.load() }
                }
            }
        }

        private func render() {
            guard let person else { return }
            var views: [NSView] = [header(person)]
            if let movies = titles(MavioStrings.personMovies, person.items.filter { $0.kind == .movie }) {
                views.append(movies)
            }
            if let series = titles(MavioStrings.personSeries, person.items.filter { $0.kind == .series }) {
                views.append(series)
            }
            if let episodes = titles(
                MavioStrings.personEpisodes, person.items.filter { $0.kind == .episode }, wide: true)
            {
                views.append(episodes)
            }
            if person.items.isEmpty {
                views.append(
                    margined(
                        NSTextField.label(MavioStrings.personNone, theme.fonts.body, color: theme.colors.textSecondary),
                        top: MavioSpacing.s32))
            }
            if page.superview == nil { show(page) }
            page.setViews(views)
            FocusEngine.active(for: view)?.schedule()
        }

        /// The photo beside the name, dates, biography and links.
        private func header(_ person: PersonDetails) -> NSView {
            let photo = ArtworkView(person.photo?.url, title: person.name, icon: "profile", theme: theme)
            photo.radius = 14
            photo.size(width: theme.isTV ? 280 : 208).aspect(2 / 3)
            var text: [NSView] = [
                NSTextField.label(person.name, theme.fonts.title1, color: theme.colors.textPrimary, lines: 0)
            ]
            for fact in facts(person) {
                text.append(NSTextField.label(fact, theme.fonts.callout, color: theme.colors.textSecondary))
            }
            if let overview = person.overview {
                let body = NSTextField.label(
                    overview, theme.fonts.body, color: theme.colors.textPrimary, lines: more ? 0 : 5,
                    selectable: !theme.isTV)
                body.preferredMaxLayoutWidth = 768
                let toggle = textLink(
                    more ? MavioStrings.personLess : MavioStrings.personMore, theme.fonts.callout, weight: .semibold,
                    color: theme.colors.accentText
                ) { [weak self] in
                    guard let self else { return }
                    self.more.toggle()
                    self.render()
                }
                let stack = NSStackView.vertical([body, toggle], spacing: MavioSpacing.s4)
                body.widthAnchor.constraint(equalTo: stack.widthAnchor).isActive = true
                text.append(stack)
            }
            if !theme.isTV, !person.links.isEmpty {
                let chips = person.links.compactMap { link -> NSView? in
                    guard let url = URL(string: link.url) else { return nil }
                    return chip(link.name, icon: "external", fill: theme.colors.fill) { openWeb(url) }
                }
                text.append(
                    inset(
                        WrapStack(chips, spacing: MavioSpacing.s8, lineSpacing: MavioSpacing.s8),
                        NSEdgeInsets(top: MavioSpacing.s4, left: 0, bottom: 0, right: 0)))
            }
            let column = NSStackView.vertical(text, spacing: MavioSpacing.s12)
            for v in text.dropFirst() where !(v is NSTextField) {
                v.widthAnchor.constraint(equalTo: column.widthAnchor).isActive = true
            }
            let width = column.widthAnchor.constraint(lessThanOrEqualToConstant: 768)
            width.isActive = true
            let row = NSStackView.horizontal([photo, column], spacing: MavioSpacing.s24, alignment: .top)
            return margined(row, top: MavioSpacing.s24)
        }

        private func facts(_ person: PersonDetails) -> [String] {
            let day = { (d: Date) in d.formatted(date: .long, time: .omitted) }
            return [
                person.born.map {
                    MavioStrings.personBorn(date: day($0), age: String(years(from: $0, to: person.died ?? .now)))
                },
                person.birthPlace,
                person.died.map { MavioStrings.personDied(date: day($0)) },
            ].compactMap(\.self)
        }

        /// Whole years between two dates.
        private func years(from: Date, to: Date) -> Int {
            Calendar.current.dateComponents([.year], from: from, to: to).year ?? 0
        }

        /// A heading with its count over the cards of one kind.
        private func titles(_ title: String, _ cards: [Card], wide: Bool = false) -> NSView? {
            guard !cards.isEmpty else { return nil }
            let heading = NSStackView.horizontal(
                [
                    heading(title),
                    NSTextField.label(String(cards.count), theme.fonts.callout, color: theme.colors.textSecondary),
                ], spacing: MavioSpacing.s8)
            heading.alignment = .firstBaseline
            let grid = FlowGrid(
                minimum: wide ? theme.layout.thumbnail : theme.layout.poster, gutter: theme.layout.gutter,
                rowSpacing: theme.isTV ? MavioSpacing.s48 : MavioSpacing.s24)
            grid.setItems(cards.map { itemCard($0, wide: wide) })
            let stack = NSStackView.vertical([heading, grid], spacing: MavioSpacing.s12)
            grid.widthAnchor.constraint(equalTo: stack.widthAnchor).isActive = true
            return margined(stack, top: MavioSpacing.s32)
        }

        isolated deinit {
            observer?.cancel()
            loadTask?.cancel()
        }
    }
#endif
