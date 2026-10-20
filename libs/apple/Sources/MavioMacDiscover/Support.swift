#if os(macOS)
    import AppKit
    import MavioMac

    // MARK: Views

    /// A view inset in a holder.
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
        italic: Bool = false
    ) -> NSTextField {
        let field = NSTextField.label(text, style, color: color, lines: lines)
        var font = NSFont.systemFont(ofSize: style.size, weight: weight ?? style.weight)
        if italic { font = NSFontManager.shared.convert(font, toHaveTrait: .italicFontMask) }
        field.font = font
        return field
    }

    /// The height a view takes at a width.
    @MainActor
    func height(of view: NSView, width: CGFloat) -> CGFloat {
        let constraint = view.widthAnchor.constraint(equalToConstant: width)
        constraint.priority = .required - 1
        constraint.isActive = true
        let height = view.fittingSize.height
        constraint.isActive = false
        return ceil(height)
    }

    /// The logo of a site, as `ServiceLogos` names it, height tall, or
    /// width wide when given; tinted when tint is given.
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

    /// A line of a group's rows, inset at its start.
    @MainActor
    func divider(_ theme: Theme, leading: CGFloat = 0) -> NSView {
        let line = Box(fill: theme.colors.border)
        line.size(height: 1)
        return inset(line, NSEdgeInsets(top: 0, left: leading, bottom: 0, right: 0))
    }

    /// When something happened, relative to now, as "5 minutes ago".
    func fromNow(_ date: Date) -> String {
        date.formatted(.relative(presentation: .named))
    }

    /// A box of fill holding text as a pill.
    @MainActor
    func pill(
        _ text: String, _ style: MavioFontStyle, fill: NSColor, color: NSColor, radius: CGFloat = 999,
        insets: NSEdgeInsets = .symmetric(horizontal: MavioSpacing.s8, vertical: 2)
    ) -> Box {
        let box = Box(fill: fill, radius: radius)
        box.addFilling(label(text, style, weight: .semibold, color: color), insets: insets)
        return box
    }

    /// A vertical stack of rows on a card of `color.surface`, lines between
    /// them, its heading above and its footer below, as the web UI's Group.
    @MainActor
    func group(_ title: String?, footer: String? = nil, rows: [NSView], theme: Theme) -> NSView {
        let card = Box(fill: theme.isTV ? theme.colors.fill : theme.colors.surface, radius: 14)
        let stack = NSStackView.vertical([], spacing: 0)
        for (i, row) in rows.enumerated() {
            if i > 0 {
                let line = divider(theme)
                stack.addArrangedSubview(line)
                line.widthAnchor.constraint(equalTo: stack.widthAnchor).isActive = true
            }
            stack.addArrangedSubview(row)
            row.widthAnchor.constraint(equalTo: stack.widthAnchor).isActive = true
        }
        card.addFilling(stack)
        var views: [NSView] = []
        if let title {
            views.append(
                inset(
                    NSTextField.label(title, theme.fonts.headline, color: theme.colors.textPrimary),
                    .symmetric(horizontal: MavioSpacing.s4)))
        }
        views.append(card)
        if let footer {
            views.append(
                inset(
                    NSTextField.label(footer, theme.fonts.footnote, color: theme.colors.textSecondary, lines: 0),
                    .symmetric(horizontal: MavioSpacing.s4)))
        }
        let outer = NSStackView.vertical(views, spacing: MavioSpacing.s8)
        for v in views { v.widthAnchor.constraint(equalTo: outer.widthAnchor).isActive = true }
        return outer
    }

    /// A check box doing change as it is checked or not.
    @MainActor
    final class CheckBox: NSButton {
        private let change: @MainActor (Bool) -> Void

        /// A check box, on or not.
        init(on: Bool, change: @escaping @MainActor (Bool) -> Void) {
            self.change = change
            super.init(frame: .zero)
            translatesAutoresizingMaskIntoConstraints = false
            setButtonType(.switch)
            title = ""
            state = on ? .on : .off
            target = self
            action = #selector(changed)
        }

        @available(*, unavailable)
        required init?(coder: NSCoder) { fatalError() }

        @objc private func changed() { change(state == .on) }

        /// Checks it or not, as a click does.
        func toggle() {
            guard isEnabled else { return }
            state = state == .on ? .off : .on
            change(state == .on)
        }
    }

    /// A row of a dialog's list: a check box, the title and details at its
    /// end; the whole row toggles it, and is focusable in television mode.
    @MainActor
    func checkRow(
        _ title: String, details: [String] = [], on: Bool, enabled: Bool = true, theme: Theme,
        change: @escaping @MainActor (Bool) -> Void
    ) -> (FormRow, CheckBox) {
        let box = CheckBox(on: on, change: change)
        box.isEnabled = enabled
        let end = NSStackView.horizontal(
            details.map { NSTextField.label($0, theme.fonts.callout, color: theme.colors.textSecondary) } + [box],
            spacing: MavioSpacing.s12)
        let row = FormRow(title, accessory: end, theme: theme)
        if enabled { row.onPress = { [weak box] in box?.toggle() } }
        row.alphaValue = enabled ? 1 : 0.5
        return (row, box)
    }

    // MARK: Page helpers

    extension Page {
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

        /// A link of text in a style: a genre, a crew member's name, a fact;
        /// focusable in television mode.
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

        /// A capsule of text in fill, a link when it does action: a keyword,
        /// a filter's value.
        func chip(
            _ text: String, image: String? = nil, fill: NSColor? = nil, color: NSColor? = nil, removable: Bool = false,
            action: (@MainActor () -> Void)? = nil
        ) -> NSView {
            let color = color ?? theme.colors.textPrimary
            var views: [NSView] = []
            if let image {
                let art = ArtworkView(image, title: text, theme: theme)
                art.radius = MavioRadius.sm
                art.size(width: 20, height: 20)
                views.append(art)
            }
            views.append(label(text, theme.fonts.footnote, weight: .semibold, color: color))
            if removable { views.append(IconView("close", size: 14, color: color)) }
            let row = NSStackView.horizontal(views, spacing: 6)
            let insets = NSEdgeInsets.symmetric(horizontal: MavioSpacing.s12, vertical: 6)
            guard let action else {
                let box = Box(fill: fill ?? theme.colors.fill, radius: 999)
                box.addFilling(row, insets: insets)
                return box
            }
            let box = Pressable(row, look: .highlight, radius: 999, insets: insets, theme: theme, action: action)
            box.restingFill = fill ?? theme.colors.fill
            box.setAccessibilityLabel(text)
            return box
        }

        /// The card of a discovered title, which opens it; under its year and
        /// kind, a person's role in it.
        func discoveredCard(_ title: DiscoveredTitle, serverId: String, role: String? = nil, width: CGFloat?)
            -> CardView
        {
            let series = title.mediaKind == .series
            let icon = series ? "shows" : "movies"
            let card = CardView(width: width, aspect: 2 / 3, icon: icon, theme: theme)
            card.artwork.set(sized(title.posterURL, 342), title: title.name, icon: icon)
            card.setText(title.name, titleCardSubtitle(title))
            if let role, !role.isEmpty {
                let line = NSTextField.label(role, theme.fonts.footnote, color: theme.colors.textSecondary)
                card.addSubview(line)
                NSLayoutConstraint.activate([
                    line.topAnchor.constraint(equalTo: card.subtitleLabel.bottomAnchor),
                    line.leadingAnchor.constraint(equalTo: card.leadingAnchor),
                    line.trailingAnchor.constraint(equalTo: card.trailingAnchor),
                    line.bottomAnchor.constraint(lessThanOrEqualTo: card.bottomAnchor),
                ])
            }
            addStatusMark(title.status, to: card)
            card.setAccessibilityLabel(title.name)
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

        /// The card at the end of a television row opening all of it
        /// (docs/tv.md §6.2, §6.3), as tall as the row's artwork.
        func seeAllCard(height: CGFloat, width: CGFloat? = nil, action: @escaping @MainActor () -> Void) -> NSView {
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
            let outer = FlippedView()
            outer.translatesAutoresizingMaskIntoConstraints = false
            outer.addSubview(card)
            NSLayoutConstraint.activate([
                card.topAnchor.constraint(equalTo: outer.topAnchor),
                card.leadingAnchor.constraint(equalTo: outer.leadingAnchor),
                card.trailingAnchor.constraint(equalTo: outer.trailingAnchor),
                card.bottomAnchor.constraint(lessThanOrEqualTo: outer.bottomAnchor),
            ])
            return outer
        }

        /// A person credited on a discovered title, opening their page when
        /// TMDB knows them.
        func creditCard(_ credit: TitleCredit, serverId: String) -> CardView {
            let card = CardView.person(
                name: credit.name, photo: sized(credit.imageURL, 185), role: credit.shownRole, theme: theme)
            if credit.tmdbId > 0 {
                card.onPress = { [weak self] in
                    self?.open(.discoveredPerson(serverId: serverId, tmdbId: credit.tmdbId))
                }
            }
            return card
        }

        /// The logos linking a title or person elsewhere, as Seerr's, faint;
        /// first Mavio's, opening its item when the server has it. Television
        /// mode, which opens no web pages, shows Mavio's alone.
        func externalLinks(_ links: [TitleLink], item: Route?) -> NSView? {
            var views: [NSView] = []
            if let item {
                let mark = WordmarkView(height: 14)
                let link = Pressable(
                    mark, look: .highlight, radius: MavioRadius.sm, insets: .all(2), theme: theme
                ) { [weak self] in self?.open(item) }
                link.setAccessibilityLabel(MavioStrings.discoverOpenItem)
                views.append(link)
            }
            if !theme.isTV {
                for l in links {
                    let logo = serviceLogo(l.logo, height: 24, width: l.width, tint: theme.colors.textPrimary)
                    let url = l.url
                    let link = Pressable(logo, look: .highlight, radius: MavioRadius.sm, theme: theme) {
                        NSWorkspace.shared.open(url)
                    }
                    link.toolTip = l.name
                    link.setAccessibilityLabel(l.name)
                    views.append(link)
                }
            }
            guard !views.isEmpty else { return nil }
            let wrap = WrapStack(views, spacing: MavioSpacing.s20, lineSpacing: MavioSpacing.s12)
            wrap.alphaValue = 0.5
            return wrap
        }
    }

    /// A view that becomes a focus region, as the overview beside its facts.
    @MainActor
    final class RegionView: NSView, FocusRegion {
        /// A region of content.
        init(_ content: NSView) {
            super.init(frame: .zero)
            translatesAutoresizingMaskIntoConstraints = false
            addFilling(content)
        }

        @available(*, unavailable)
        required init?(coder: NSCoder) { fatalError() }
    }
#endif
