#if os(macOS)
    import AppKit
    import MavioMac

    /// A title Home's hero shows, and the line above it: the row it is from.
    struct HeroEntry {
        /// The item.
        let card: Card
        /// The row it is from, over its title.
        let eyebrow: String
        /// The server and item.
        var id: String { "\(card.serverId)/\(card.itemId)" }

        private static let kinds: Set<CardKind> = [.movie, .series, .video, .musicVideo]

        /// Up to five titles with backdrops from Home's rows, Continue
        /// Watching aside, as the web UI's.
        static func from(_ rows: [Row]) -> [HeroEntry] {
            var out: [HeroEntry] = []
            var seen: Set<String> = []
            for row in rows where row.kind != .continueWatching {
                guard case let .items(cards) = row.content else { continue }
                for card in cards where kinds.contains(card.kind) && card.backdrop != nil {
                    let entry = HeroEntry(card: card, eyebrow: rowTitle(row))
                    guard seen.insert(entry.id).inserted else { continue }
                    out.append(entry)
                    if out.count == 5 { return out }
                }
            }
            return out
        }
    }

    /// The hero at the top of Home (docs/clients.md §6.3): the latest films
    /// and shows over their backdrops, one after another every 9 seconds,
    /// each with its facts and the buttons to play it and open its details;
    /// swiped through sideways, or with the dots. On the desktop 470 pt high;
    /// in television mode 78 % of the page's height, reaching behind the
    /// bar, its title at 72 pt (docs/tv.md §6.1), and still while focus is
    /// on its buttons.
    final class HeroView: FlippedView {
        private let entries: [HeroEntry]
        private let theme: Theme
        private weak var page: Page?
        private var slides: [HeroSlide] = []
        private var dots: [Box] = []
        private var dotWidths: [NSLayoutConstraint] = []
        private var timer: Task<Void, Never>?
        private var swipe: CGFloat = 0
        /// The entry shown.
        private(set) var index = 0
        /// Called as another title shows, with its ID.
        var onShow: ((String) -> Void)?

        /// How long each title shows before the next.
        private static let interval: Duration = .seconds(9)

        /// A hero of entries on page, showing the one of ID shown.
        init(entries: [HeroEntry], shown: String?, page: Page) {
            self.entries = entries
            theme = page.theme
            self.page = page
            super.init(frame: .zero)
            translatesAutoresizingMaskIntoConstraints = false
            wantsLayer = true
            layer?.masksToBounds = true
            appearance = NSAppearance(named: .darkAqua)
            let surface = Box(fill: theme.colors.surface)
            addFilling(surface)
            for entry in entries {
                let slide = HeroSlide(entry: entry, page: page)
                slide.isHidden = true
                addFilling(slide)
                slides.append(slide)
            }
            index = entries.firstIndex { $0.id == shown } ?? 0
            slides[index].isHidden = false
            if entries.count > 1 { addDots() }
            setAccessibilityElement(false)
            schedule()
        }

        @available(*, unavailable)
        required init?(coder: NSCoder) { fatalError() }

        private func addDots() {
            let stack = NSStackView.horizontal([], spacing: 6)
            for i in entries.indices {
                let dot = Box(fill: .white, radius: 3)
                let width = dot.widthAnchor.constraint(equalToConstant: 6)
                dot.heightAnchor.constraint(equalToConstant: 6).isActive = true
                width.isActive = true
                dotWidths.append(width)
                dots.append(dot)
                // A taller target than the dot.
                let target = FlippedView()
                target.translatesAutoresizingMaskIntoConstraints = false
                target.addSubview(dot)
                NSLayoutConstraint.activate([
                    dot.leadingAnchor.constraint(equalTo: target.leadingAnchor),
                    dot.trailingAnchor.constraint(equalTo: target.trailingAnchor),
                    dot.centerYAnchor.constraint(equalTo: target.centerYAnchor),
                    target.heightAnchor.constraint(equalToConstant: 22),
                ])
                target.setAccessibilityElement(true)
                target.setAccessibilityRole(.button)
                target.setAccessibilityLabel(entries[i].card.title)
                let click = NSClickGestureRecognizer(target: self, action: #selector(dotClicked(_:)))
                target.addGestureRecognizer(click)
                target.identifier = NSUserInterfaceItemIdentifier(String(i))
                stack.addArrangedSubview(target)
            }
            addSubview(stack)
            NSLayoutConstraint.activate([
                stack.trailingAnchor.constraint(
                    equalTo: trailingAnchor, constant: theme.isTV ? -theme.layout.margin : -MavioSpacing.s32),
                stack.bottomAnchor.constraint(equalTo: bottomAnchor, constant: theme.isTV ? -40 : -MavioSpacing.s16),
            ])
            restyleDots()
        }

        @objc private func dotClicked(_ sender: NSClickGestureRecognizer) {
            guard let i = sender.view?.identifier.flatMap({ Int($0.rawValue) }) else { return }
            show(i)
        }

        private func restyleDots() {
            NSAnimationContext.runAnimationGroup { context in
                context.duration = MavioDuration.fast
                context.allowsImplicitAnimation = true
                for (i, dot) in dots.enumerated() {
                    dot.fill = i == index ? .white : .white.withAlphaComponent(0.45)
                    dotWidths[i].constant = i == index ? 20 : 6
                }
                layoutSubtreeIfNeeded()
            }
            for (i, dot) in dots.enumerated() {
                dot.superview?.setAccessibilitySelected(i == index)
            }
        }

        /// Shows the entry at i, fading from the one shown.
        func show(_ i: Int) {
            guard i != index, slides.indices.contains(i) else { return }
            let old = slides[index]
            let new = slides[i]
            let focusInside = isFocusInside
            index = i
            new.alphaValue = 0
            new.isHidden = false
            NSAnimationContext.runAnimationGroup { context in
                context.duration = MavioDuration.slow
                new.animator().alphaValue = 1
                old.animator().alphaValue = 0
            } completionHandler: { [weak old] in
                MainActor.assumeIsolated {
                    old?.isHidden = true
                    old?.alphaValue = 1
                }
            }
            restyleDots()
            onShow?(entries[i].id)
            if focusInside, let engine = FocusEngine.active(for: self) {
                engine.focus(new.firstAction)
            }
            schedule()
        }

        /// The first of the shown title's buttons, which television mode's
        /// focus lands on.
        var firstAction: NSView? { slides.indices.contains(index) ? slides[index].firstAction : nil }

        private var isFocusInside: Bool {
            guard let focused = FocusEngine.active(for: self)?.focused else { return false }
            return focused.isDescendant(of: self)
        }

        /// Shows the next title after a while, unless focus is on the hero's
        /// buttons in television mode.
        private func schedule() {
            timer?.cancel()
            guard entries.count > 1 else { return }
            timer = Task { [weak self] in
                while !Task.isCancelled {
                    try? await Task.sleep(for: Self.interval)
                    guard !Task.isCancelled, let self else { return }
                    if self.window == nil || self.isFocusInside { continue }
                    self.show((self.index + 1) % self.entries.count)
                    return
                }
            }
        }

        // MARK: Swiping

        override func scrollWheel(with event: NSEvent) {
            guard entries.count > 1, abs(event.scrollingDeltaX) > abs(event.scrollingDeltaY) || swipe != 0 else {
                nextResponder?.scrollWheel(with: event)
                return
            }
            if event.phase == .began { swipe = 0 }
            swipe += event.scrollingDeltaX
            if event.phase == .ended || event.phase == .cancelled || (event.phase == [] && event.momentumPhase == []) {
                if abs(swipe) > 40 {
                    let step = swipe < 0 ? 1 : -1
                    show((index + step + entries.count) % entries.count)
                }
                swipe = 0
            }
        }

        isolated deinit { timer?.cancel() }
    }

    /// One title of the hero: its backdrop under gradients, and over its
    /// lower left the row it is from, its logo or name, its facts, its
    /// overview and the buttons.
    private final class HeroSlide: FlippedView {
        private let entry: HeroEntry
        private(set) var firstAction: NSView?

        init(entry: HeroEntry, page: Page) {
            self.entry = entry
            super.init(frame: .zero)
            translatesAutoresizingMaskIntoConstraints = false
            let theme = page.theme
            let tv = theme.isTV
            let card = entry.card
            let art = ArtworkView(card.backdrop?.url, title: card.title, icon: card.kind.icon, theme: theme)
            art.setAccessibilityElement(false)
            addFilling(art)
            if tv {
                addFilling(
                    GradientView(
                        colors: [.black.withAlphaComponent(0.6), .clear], locations: [0, 0.6],
                        from: CGPoint(x: 0, y: 0.5), to: CGPoint(x: 1, y: 0.5)))
                addFilling(
                    GradientView(
                        colors: [theme.colors.tvBackground, theme.colors.tvBackground.withAlphaComponent(0.6), .clear],
                        locations: [0, 0.3, 0.7], from: CGPoint(x: 0.5, y: 1), to: CGPoint(x: 0.5, y: 0)))
            } else {
                addFilling(
                    GradientView(
                        colors: [.black.withAlphaComponent(0.78), .black.withAlphaComponent(0.35), .clear],
                        locations: [0, 0.45, 0.75], from: CGPoint(x: 0, y: 0.5), to: CGPoint(x: 1, y: 0.5)))
                addFilling(
                    GradientView(
                        colors: [.black.withAlphaComponent(0.55), .clear], locations: [0, 0.45],
                        from: CGPoint(x: 0.5, y: 1), to: CGPoint(x: 0.5, y: 0)))
            }
            let text = content(page: page)
            addSubview(text)
            let side = tv ? theme.layout.margin : MavioSpacing.s40
            NSLayoutConstraint.activate([
                text.leadingAnchor.constraint(equalTo: leadingAnchor, constant: side),
                text.bottomAnchor.constraint(
                    equalTo: bottomAnchor, constant: tv ? -MavioSpacing.s48 : -MavioSpacing.s40),
                text.widthAnchor.constraint(lessThanOrEqualToConstant: tv ? 960 : 560),
                text.trailingAnchor.constraint(lessThanOrEqualTo: trailingAnchor, constant: -side),
            ])
        }

        @available(*, unavailable)
        required init?(coder: NSCoder) { fatalError() }

        private func content(page: Page) -> NSView {
            let theme = page.theme
            let tv = theme.isTV
            let card = entry.card
            var views: [NSView] = []
            let eyebrow = label(
                entry.eyebrow.uppercased(), theme.fonts.footnote, weight: .semibold,
                color: .white.withAlphaComponent(0.75))
            eyebrow.attributedStringValue = NSAttributedString(
                string: entry.eyebrow.uppercased(),
                attributes: [
                    .kern: 0.6, .font: NSFont.systemFont(ofSize: theme.fonts.footnote.size, weight: .semibold),
                    .foregroundColor: NSColor.white.withAlphaComponent(0.75),
                ])
            views.append(eyebrow)
            let title = NSTextField.label(
                card.title, theme.fonts.title1, color: .white, lines: 2)
            title.font = .systemFont(ofSize: tv ? 72 : 52, weight: .bold)
            title.preferredMaxLayoutWidth = tv ? 960 : 560
            if let logo = card.logo {
                views.append(
                    LogoView(
                        logo.url, maxWidth: tv ? 640 : 280, maxHeight: tv ? 160 : 96, label: card.title, fallback: title
                    ))
            } else {
                views.append(title)
            }
            let facts =
                [card.year.map(String.init), card.runtime.map(minutesText)].compactMap(\.self)
                + card.genres.prefix(2)
            var factViews: [NSView] = []
            if !facts.isEmpty {
                factViews.append(
                    NSTextField.label(
                        facts.joined(separator: " · "), theme.fonts.callout, color: .white.withAlphaComponent(0.8)))
            }
            if let rating = card.officialRating {
                let badge = Box(radius: 4)
                badge.border = .white.withAlphaComponent(0.5)
                badge.addFilling(
                    NSTextField.label(rating, theme.fonts.caption, color: .white.withAlphaComponent(0.9)),
                    insets: .symmetric(horizontal: 6, vertical: 1))
                factViews.append(badge)
            }
            if !factViews.isEmpty { views.append(NSStackView.horizontal(factViews, spacing: MavioSpacing.s8)) }
            if let overview = card.overview {
                let text = NSTextField.label(
                    overview, theme.fonts.body, color: .white.withAlphaComponent(0.85), lines: tv ? 2 : 3)
                text.preferredMaxLayoutWidth = tv ? 960 : 560
                views.append(text)
            }
            var buttons: [NSView] = []
            if card.kind != .series {
                let play = MavioButton(
                    MavioStrings.playerPlay, icon: "play", kind: .artworkPrimary, large: true, theme: theme
                ) { [weak page] in
                    var start: TimeInterval?
                    if let progress = card.progress, let runtime = card.runtime {
                        start = TimeInterval(progress) * runtime
                    }
                    page?.context.play(
                        PlayRequest(serverId: card.serverId, itemId: card.itemId, title: card.title, start: start))
                }
                buttons.append(play)
            }
            let details = MavioButton(
                MavioStrings.itemDetails, icon: "info", kind: .artworkSecondary, large: true, theme: theme
            ) { [weak page] in
                page?.open(.item(serverId: card.serverId, itemId: card.itemId))
            }
            buttons.append(details)
            firstAction = buttons.first
            let row = NSStackView.horizontal(buttons, spacing: MavioSpacing.s12)
            let stack = NSStackView.vertical(views, spacing: MavioSpacing.s12)
            stack.addArrangedSubview(row)
            stack.setCustomSpacing(MavioSpacing.s20, after: views[views.count - 1])
            for v in views where v is NSTextField {
                v.widthAnchor.constraint(lessThanOrEqualTo: stack.widthAnchor).isActive = true
            }
            return stack
        }
    }
#endif
