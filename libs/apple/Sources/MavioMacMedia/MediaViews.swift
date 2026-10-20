#if os(macOS)
    import AppKit
    import MavioMac
    import Nuke

    /// An image shown whole at its own proportions, as a title's logo: at most
    /// maxWidth by maxHeight, at the bottom left of that space, which it takes
    /// while it loads; fallback in its place when it cannot.
    final class LogoView: FlippedView {
        private let image = NSImageView()
        private let fallback: NSView
        private var task: Task<Void, Never>?
        private var width: NSLayoutConstraint!
        private var height: NSLayoutConstraint!
        private let maxWidth: CGFloat
        private let maxHeight: CGFloat

        /// The image at url, fitting maxWidth by maxHeight, else fallback.
        init(_ url: String, maxWidth: CGFloat, maxHeight: CGFloat, label: String, fallback: NSView) {
            self.fallback = fallback
            self.maxWidth = maxWidth
            self.maxHeight = maxHeight
            super.init(frame: .zero)
            translatesAutoresizingMaskIntoConstraints = false
            image.imageScaling = .scaleProportionallyUpOrDown
            image.imageAlignment = .alignBottomLeft
            image.translatesAutoresizingMaskIntoConstraints = false
            addFilling(image)
            fallback.isHidden = true
            addSubview(fallback)
            NSLayoutConstraint.activate([
                fallback.leadingAnchor.constraint(equalTo: leadingAnchor),
                fallback.bottomAnchor.constraint(equalTo: bottomAnchor),
                fallback.trailingAnchor.constraint(lessThanOrEqualTo: trailingAnchor),
            ])
            width = widthAnchor.constraint(equalToConstant: maxWidth)
            height = heightAnchor.constraint(equalToConstant: maxHeight)
            width.priority = .defaultHigh
            NSLayoutConstraint.activate([width, height])
            setAccessibilityElement(true)
            setAccessibilityRole(.image)
            setAccessibilityLabel(label)
            load(url)
        }

        @available(*, unavailable)
        required init?(coder: NSCoder) { fatalError() }

        private func load(_ url: String) {
            guard let parsed = URL(string: url) else { return fail() }
            task = Task { [weak self] in
                do {
                    let loaded = try await ImagePipeline.shared.image(for: parsed)
                    guard !Task.isCancelled else { return }
                    self?.show(loaded)
                } catch {
                    guard !Task.isCancelled else { return }
                    self?.fail()
                }
            }
        }

        private func show(_ loaded: NSImage) {
            let size = loaded.size
            guard size.width > 0, size.height > 0 else { return fail() }
            let aspect = size.width / size.height
            let w = min(maxWidth, maxHeight * aspect)
            width.constant = w
            height.constant = w / aspect
            image.image = loaded
        }

        private func fail() {
            image.isHidden = true
            fallback.isHidden = false
            width.isActive = false
            height.isActive = false
            NSLayoutConstraint.activate([
                fallback.topAnchor.constraint(equalTo: topAnchor),
                fallback.trailingAnchor.constraint(equalTo: trailingAnchor),
            ])
        }

        isolated deinit { task?.cancel() }
    }

    /// The logo of a site, as `ServiceLogos` names it, height tall; tinted
    /// when it is drawn in one color, as the links' are.
    @MainActor
    func serviceLogo(_ name: String, height: CGFloat, width: CGFloat? = nil, tint: NSColor? = nil) -> NSImageView {
        let image = ServiceLogos.bundle.image(forResource: name) ?? NSImage()
        let view = NSImageView(image: image)
        view.imageScaling = .scaleProportionallyUpOrDown
        view.translatesAutoresizingMaskIntoConstraints = false
        if let tint {
            image.isTemplate = true
            view.contentTintColor = tint
        }
        let aspect = image.size.height > 0 ? image.size.width / image.size.height : 1
        view.size(width: width ?? ceil(height * aspect), height: height)
        view.setAccessibilityElement(false)
        return view
    }

    /// A view inset in a holder, as a page's sections are by its margin.
    @MainActor
    func inset(_ view: NSView, _ insets: NSEdgeInsets) -> NSView {
        let holder = NSView()
        holder.translatesAutoresizingMaskIntoConstraints = false
        holder.addFilling(view, insets: insets)
        return holder
    }

    /// A label with a weight other than its style's.
    @MainActor
    func label(
        _ text: String, _ style: MavioFontStyle, weight: NSFont.Weight? = nil, color: NSColor, lines: Int = 1,
        selectable: Bool = false
    ) -> NSTextField {
        let field = NSTextField.label(text, style, color: color, lines: lines, selectable: selectable)
        if let weight { field.font = .systemFont(ofSize: style.size, weight: weight) }
        return field
    }

    /// Opens a web page, as links do outside television mode.
    @MainActor
    func openWeb(_ url: URL) {
        NSWorkspace.shared.open(url)
    }

    /// Shows a menu under a view, as a button's.
    @MainActor
    func popUp(_ menu: NSMenu, under view: NSView) {
        let y = view.isFlipped ? view.bounds.height + 4 : -4
        menu.popUp(positioning: nil, at: NSPoint(x: 0, y: y), in: view)
    }

    extension Page {
        /// A chip: text, and an icon after it, in a capsule of fill, or with a
        /// border; focusable, doing action.
        func chip(
            _ text: String, icon: String? = nil, leadingIcon: String? = nil, style: MavioFontStyle? = nil,
            weight: NSFont.Weight = .semibold, fill: NSColor? = nil, color: NSColor? = nil, border: NSColor? = nil,
            height: CGFloat? = nil, insets: NSEdgeInsets? = nil, action: (@MainActor () -> Void)? = nil
        ) -> Pressable {
            let style = style ?? theme.fonts.footnote
            let color = color ?? theme.colors.textPrimary
            var views: [NSView] = []
            if let leadingIcon { views.append(IconView(leadingIcon, size: 16, color: color)) }
            views.append(label(text, style, weight: weight, color: color))
            if let icon { views.append(IconView(icon, size: 14, color: theme.colors.textSecondary)) }
            let row = NSStackView.horizontal(views, spacing: 6)
            let box = Pressable(
                row, look: .highlight, radius: 999,
                insets: insets ?? .symmetric(horizontal: 14, vertical: 6), theme: theme, action: action)
            box.restingFill = fill
            box.border = border
            if let height { box.size(height: height) }
            box.setAccessibilityLabel(text)
            return box
        }

        /// A link of text in a style: the crew's names, a series' name.
        func textLink(
            _ text: String, _ style: MavioFontStyle, weight: NSFont.Weight? = nil, color: NSColor,
            action: @escaping @MainActor () -> Void
        ) -> Pressable {
            let link = Pressable(
                label(text, style, weight: weight, color: color), look: .highlight, radius: MavioRadius.sm,
                theme: theme, action: action)
            link.setAccessibilityLabel(text)
            return link
        }

        /// A section's heading in `title3`.
        func heading(_ text: String) -> NSTextField {
            .label(text, theme.fonts.title3, color: theme.colors.textPrimary)
        }

        /// A view inset by the page's margin at its sides.
        func margined(_ view: NSView, top: CGFloat = 0, bottom: CGFloat = 0) -> NSView {
            inset(
                view,
                NSEdgeInsets(top: top, left: theme.layout.margin, bottom: bottom, right: theme.layout.margin))
        }

        /// A line saying a server cannot be reached, its reason on hover.
        func unreachable(_ name: String, reason: String?) -> NSView {
            let row = NSStackView.horizontal(
                [
                    IconView("offline", size: 18, color: theme.colors.textTertiary),
                    NSTextField.label(
                        MavioStrings.homeServerUnreachable(server: name), theme.fonts.callout,
                        color: theme.colors.textTertiary),
                ], spacing: MavioSpacing.s8)
            row.toolTip = reason
            return margined(row, top: MavioSpacing.s12, bottom: MavioSpacing.s12)
        }

        /// A credited person's card, opening their page when the server
        /// names them.
        func personCard(_ credit: CreditEntry, serverId: String, role: String?) -> CardView {
            let card = CardView.person(name: credit.name, photo: credit.photo?.url, role: role, theme: theme)
            if !credit.personId.isEmpty {
                card.onPress = { [weak self] in
                    self?.open(.person(serverId: serverId, personId: credit.personId))
                }
            }
            return card
        }

        /// The card at the end of a television row opening all of it
        /// (docs/tv.md §6.2, §6.3), as wide and tall as the row's cards.
        func seeAllCard(width: CGFloat? = nil, height: CGFloat, action: @escaping @MainActor () -> Void) -> NSView {
            let content = NSStackView.vertical(
                [
                    IconView("forward", size: 40, color: theme.colors.textPrimary),
                    NSTextField.label(MavioStrings.creditsAll, theme.fonts.headline, color: theme.colors.textPrimary),
                ], spacing: MavioSpacing.s12, alignment: .centerX)
            let holder = NSView()
            holder.translatesAutoresizingMaskIntoConstraints = false
            holder.addSubview(content)
            content.center(in: holder)
            let card = Pressable(holder, look: .lift, radius: MavioRadius.md, theme: theme, action: action)
            card.restingFill = theme.colors.fill
            card.size(width: width ?? theme.layout.poster, height: height)
            card.setAccessibilityLabel(MavioStrings.creditsAll)
            return card
        }

        /// The card of a discovered title, which opens it; under its year and
        /// kind.
        func discoveredCard(_ title: DiscoveredTitle, serverId: String, width: CGFloat?) -> CardView {
            let series = title.mediaKind == .series
            let card = CardView(width: width, aspect: 2 / 3, icon: series ? "shows" : "movies", theme: theme)
            card.artwork.set(sized(title.posterURL, 342), title: title.name, icon: series ? "shows" : "movies")
            card.setText(
                title.name,
                [
                    title.year > 0 ? String(title.year) : nil,
                    series ? MavioStrings.discoverSeries : MavioStrings.discoverMovie,
                ]
                .compactMap(\.self).joined(separator: " · "))
            addStatusMark(title.status, to: card)
            card.onPress = { [weak self] in
                self?.open(.title(serverId: serverId, kind: title.mediaKind, tmdbId: title.tmdbId))
            }
            return card
        }

        /// The mark of a title's status on its card: a check once available,
        /// a clock while requested or processing.
        private func addStatusMark(_ status: MediaStatus, to card: CardView) {
            let available = status == .available || status == .partiallyAvailable
            guard available || status == .requested || status == .wanted else { return }
            let mark = Box(fill: available ? theme.colors.success : theme.colors.scrim, radius: 999)
            mark.addFilling(IconView(available ? "check" : "pending", size: 16, color: .white), insets: .all(2))
            mark.alphaValue = status == .partiallyAvailable ? 0.7 : 1
            mark.setAccessibilityElement(true)
            mark.setAccessibilityLabel(statusLabel(status) ?? "")
            card.overlay.addSubview(mark)
            NSLayoutConstraint.activate([
                mark.topAnchor.constraint(equalTo: card.overlay.topAnchor, constant: 6),
                mark.trailingAnchor.constraint(equalTo: card.overlay.trailingAnchor, constant: -6),
            ])
        }
    }

    /// A round button of an icon or any view, glass over artwork or filled
    /// on a page: what MavioButton's circles are, for content of its own,
    /// such as a favorite's red heart or a download's progress.
    @MainActor
    func circleButton(
        _ content: NSView, overArtwork: Bool, theme: Theme, label: String, action: @escaping @MainActor () -> Void
    ) -> Pressable {
        let side: CGFloat = theme.isTV ? 72 : 44
        let holder = FlippedView()
        holder.translatesAutoresizingMaskIntoConstraints = false
        if overArtwork {
            let glass = NSVisualEffectView()
            glass.material = .hudWindow
            glass.blendingMode = .withinWindow
            glass.state = .active
            glass.wantsLayer = true
            glass.layer?.cornerRadius = side / 2
            glass.layer?.masksToBounds = true
            holder.addFilling(glass)
        }
        holder.addSubview(content)
        content.center(in: holder)
        let button = Pressable(holder, look: .highlight, radius: 999, theme: theme, action: action)
        button.restingFill = overArtwork ? .white.withAlphaComponent(0.16) : theme.colors.fill
        button.size(width: side, height: side)
        button.setAccessibilityLabel(label)
        return button
    }

    extension CardRow {
        /// Scrolls the row so that the card at index, of width, is first in
        /// view.
        func scroll(toIndex index: Int, itemWidth: CGFloat, theme: Theme) {
            guard index > 0, let scroll = subviews.compactMap({ $0 as? NSScrollView }).first else { return }
            layoutSubtreeIfNeeded()
            let clip = scroll.contentView
            let maxX = max(0, (scroll.documentView?.frame.width ?? 0) - clip.bounds.width)
            let x = min(maxX, CGFloat(index) * (itemWidth + theme.layout.gutter))
            clip.setBoundsOrigin(NSPoint(x: x, y: 0))
            scroll.reflectScrolledClipView(clip)
        }
    }
#endif
