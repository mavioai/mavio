#if os(macOS)
    import AppKit
    import QuartzCore

    /// The assistant button (docs/clients.md §6.3.29): shown while the
    /// server's assistant is on, it starts a voice session, held to talk
    /// while held or clicked to start a conversation; filled while the
    /// session listens.
    public final class AssistantButton: Box, Focusable {
        private let assistant: Assistant
        private let theme: Theme
        private let icon: IconView
        private let restingFill: NSColor?
        private var observer: Observer?
        private var tracking: NSTrackingArea?
        private var hovering = false
        private var pressing = false

        /// The button of assistant, side points square, its icon size
        /// points large in color, filled with fill at rest.
        public init(
            assistant: Assistant, theme: Theme, side: CGFloat = 28, size: CGFloat = 18, color: NSColor? = nil,
            fill: NSColor? = nil
        ) {
            self.assistant = assistant
            self.theme = theme
            restingFill = fill
            icon = IconView("microphone", size: size, color: color ?? theme.colors.textSecondary)
            super.init(fill: fill, radius: side / 2)
            addSubview(icon)
            icon.center(in: self)
            self.size(width: side, height: side)
            toolTip = MavioStrings.assistantTitle
            setAccessibilityElement(true)
            setAccessibilityRole(.button)
            setAccessibilityLabel(MavioStrings.assistantTitle)
            observer = Observer { [weak self] in
                guard let self else { return }
                self.isHidden = self.assistant.server == nil
                self.icon.fillIcon = self.assistant.listening
            }
        }

        /// Unavailable: the view is made in code.
        @available(*, unavailable)
        public required init?(coder: NSCoder) { fatalError() }

        isolated deinit { observer?.cancel() }

        /// Whether television mode's focus is on it.
        public var isFocused = false { didSet { restyle() } }

        /// Tells a page that shows its own tips that the pointer came over
        /// the button, or left it.
        public var onHover: (@MainActor (Bool) -> Void)?

        /// Starts or ends a conversation, as select does.
        public func activate() { assistant.tap() }

        override public func accessibilityPerformPress() -> Bool {
            assistant.tap()
            return true
        }

        override public func acceptsFirstMouse(for event: NSEvent?) -> Bool { true }

        override public func mouseDown(with event: NSEvent) {
            pressing = true
            assistant.press()
        }

        override public func mouseUp(with event: NSEvent) {
            guard pressing else { return }
            pressing = false
            assistant.release()
        }

        override public func updateTrackingAreas() {
            super.updateTrackingAreas()
            if let tracking { removeTrackingArea(tracking) }
            let area = NSTrackingArea(
                rect: bounds, options: [.mouseEnteredAndExited, .activeInActiveApp, .inVisibleRect], owner: self)
            addTrackingArea(area)
            tracking = area
        }

        override public func mouseEntered(with event: NSEvent) {
            hovering = true
            restyle()
            onHover?(true)
        }

        override public func mouseExited(with event: NSEvent) {
            hovering = false
            restyle()
            onHover?(false)
        }

        private func restyle() {
            if isFocused && theme.isTV {
                fill = .white
                icon.contentTintColor = .black
            } else {
                fill = hovering ? theme.colors.fill : restingFill
                icon.contentTintColor = theme.isTV ? theme.colors.textSecondary : icon.contentTintColor
            }
        }
    }

    /// The assistant's orb and cards over a window's content
    /// (libs/design/components.md, Assistant): the orb at the top right with
    /// its status pill, the cards below it. Clicks pass through to the page
    /// but on the orb and the cards; a click elsewhere, or Escape, dismisses
    /// it.
    public final class AssistantOverlay: FlippedView {
        private let assistant: Assistant
        private let orb: OrbView
        private let orbHolder = FlippedView()
        private let pill = Box()
        private let pillLabel: NSTextField
        private let cards = NSStackView.vertical([], spacing: MavioSpacing.s8, alignment: .trailing)
        private var spoken: NSTextField?
        private var segments: [(text: String, at: Double)] = []
        private var lit = -1
        private var played = 0.0
        /// When the orb began to leave.
        private var leftAt: CFTimeInterval?
        private var observer: Observer?
        private var monitor: Any?
        private var shownLeaving = false
        private var cardsKey = ""

        /// The overlay of assistant.
        public init(assistant: Assistant) {
            self.assistant = assistant
            orb = OrbView(assistant: assistant)
            pillLabel = .label("", MavioFonts.desktop.footnote)
            super.init(frame: .zero)
            translatesAutoresizingMaskIntoConstraints = false
            setAccessibilityRole(.group)
            setAccessibilityLabel(MavioStrings.assistantTitle)

            orbHolder.translatesAutoresizingMaskIntoConstraints = false
            orbHolder.size(width: OrbRenderer.size, height: OrbRenderer.size)
            orbHolder.addSubview(orb)
            orb.center(in: orbHolder)
            orb.size(width: OrbRenderer.canvasSize, height: OrbRenderer.canvasSize)
            orb.onFrame = { [weak self] in self?.frame() }

            pill.fill = Self.cardColor
            pill.border = .assistantBorder
            pill.radius = MavioRadius.full
            pill.addFilling(pillLabel, insets: NSEdgeInsets(top: 4, left: 12, bottom: 4, right: 12))

            for v in [orbHolder, pill, cards] as [NSView] { addSubview(v) }
            NSLayoutConstraint.activate([
                // At the window's top, over its transparent title bar, as the web
                // UI's at the page's.
                orbHolder.topAnchor.constraint(equalTo: topAnchor, constant: MavioSpacing.s12),
                orbHolder.trailingAnchor.constraint(equalTo: trailingAnchor, constant: -MavioSpacing.s12),
                pill.trailingAnchor.constraint(equalTo: orbHolder.leadingAnchor, constant: -MavioSpacing.s8),
                pill.centerYAnchor.constraint(equalTo: orbHolder.centerYAnchor),
                // Just under the orb's light, which fills only the middle of its box.
                cards.topAnchor.constraint(equalTo: orbHolder.bottomAnchor, constant: -12),
                cards.trailingAnchor.constraint(equalTo: trailingAnchor, constant: -MavioSpacing.s16),
                cards.widthAnchor.constraint(equalToConstant: Self.cardWidth),
            ])
            observer = Observer { [weak self] in self?.update() }
        }

        /// Unavailable: the view is made in code.
        @available(*, unavailable)
        public required init?(coder: NSCoder) { fatalError() }

        isolated deinit {
            observer?.cancel()
            if let monitor { NSEvent.removeMonitor(monitor) }
        }

        /// The cards' width.
        static let cardWidth: CGFloat = 380

        /// The cards' material: `color.surfaceOverlay` at 72% in the dark
        /// theme, white at 74% in the light.
        static let cardColor = NSColor(name: nil) { appearance in
            appearance.bestMatch(from: [.darkAqua, .aqua]) == .darkAqua
                ? MavioNSColors.dark.surfaceOverlay.withAlphaComponent(0.72)
                : NSColor.white.withAlphaComponent(0.74)
        }

        override public func viewDidMoveToWindow() {
            super.viewDidMoveToWindow()
            if let monitor {
                NSEvent.removeMonitor(monitor)
                self.monitor = nil
            }
            orb.running = window != nil
            guard window != nil else { return }
            monitor = NSEvent.addLocalMonitorForEvents(matching: [.leftMouseDown, .rightMouseDown, .keyDown]) {
                [weak self] event in
                guard let self else { return event }
                nonisolated(unsafe) let e = event
                let swallowed = MainActor.assumeIsolated { self.swallows(e) }
                return swallowed ? nil : event
            }
        }

        /// Whether a point of the window is on the orb or a card.
        private func owns(_ point: NSPoint) -> Bool {
            let p = convert(point, from: nil)
            if orbHolder.frame.insetBy(dx: -8, dy: -8).contains(p) { return true }
            if !pill.isHidden && pill.frame.contains(p) { return true }
            let inCards = cards.convert(p, from: self)
            return cards.arrangedSubviews.contains { !$0.isHidden && $0.frame.contains(inCards) }
        }

        /// Whether event dismisses the assistant, and goes no further.
        private func swallows(_ event: NSEvent) -> Bool {
            guard event.window === window, !assistant.leaving else { return false }
            switch event.type {
            case .keyDown:
                let typing = window?.firstResponder is NSText
                let tv = UserDefaults.standard.bool(forKey: DeviceSetting.tvMode)
                if event.keyCode == 53 || (event.keyCode == 51 && tv && !typing) {
                    assistant.close()
                    return true
                }
                return false
            default:
                if owns(event.locationInWindow) { return false }
                // The assistant button starts and ends the session itself.
                if let hit = window?.contentView?.hitTest(
                    window!.contentView!.convert(event.locationInWindow, from: nil)),
                    hit is AssistantButton || hit.superview is AssistantButton
                {
                    return false
                }
                // The press only dismisses: it does not reach the page.
                assistant.close()
                return true
            }
        }

        override public func hitTest(_ point: NSPoint) -> NSView? {
            guard let superview, owns(superview.convert(point, to: nil)) else { return nil }
            return super.hitTest(point)
        }

        // MARK: Updating

        private func update() {
            let a = assistant
            let status = a.status
            pill.isHidden = status == nil
            pillLabel.stringValue = status ?? ""
            if a.leaving != shownLeaving {
                shownLeaving = a.leaving
                if a.leaving {
                    NSAnimationContext.runAnimationGroup { ctx in
                        ctx.duration = MavioDuration.slow
                        animator().alphaValue = 0
                    }
                } else {
                    // Shown again at once, not faded back in once gone.
                    alphaValue = 1
                }
            }
            buildCards()
        }

        private func buildCards() {
            let a = assistant
            // The cards are built anew only as what they show changes.
            let key = [
                a.heard, a.heardAt == nil ? "" : "at", a.userSpeaking ? "s" : "", a.answer,
                a.activity.map { "\($0.id)\($0.summary)\($0.done)\($0.failed)" }.joined(),
                a.items.map(\.id).joined(), a.choose ? "c" : "", a.confirmation?.id ?? "", a.error ?? "",
                a.off ? "off" : "",
            ].joined(separator: "\u{1}")
            guard key != cardsKey else { return }
            cardsKey = key
            var views: [NSView] = []
            let colors = MavioNSColors.system
            let fonts = MavioFonts.desktop
            if a.off {
                views.append(
                    Card(
                        NSTextField.label(MavioStrings.assistantOff, fonts.body, color: colors.textSecondary, lines: 0))
                )
            }
            spoken = nil
            if !a.heard.isEmpty || !a.answer.isEmpty {
                var rows: [NSView] = []
                if !a.heard.isEmpty {
                    let heard = NSTextField.label("", fonts.body, color: colors.textSecondary, lines: 0)
                    let text = NSMutableAttributedString(
                        string: a.heard, attributes: [.foregroundColor: colors.textSecondary, .font: fonts.body.font])
                    if !a.userSpeaking && a.heardAt != nil {
                        text.append(
                            NSAttributedString(
                                string: " ›",
                                attributes: [.foregroundColor: colors.textTertiary, .font: fonts.body.font]))
                    }
                    heard.attributedStringValue = text
                    rows.append(heard)
                }
                if !a.answer.isEmpty {
                    let answer = NSTextField.label("", fonts.body, lines: 0)
                    segments = SpokenText.segments(a.answer)
                    lit = -1
                    spoken = answer
                    rows.append(answer)
                    light()
                }
                views.append(Card(NSStackView.vertical(rows, spacing: 10, alignment: .leading).filled()))
            }
            if !a.activity.isEmpty {
                let rows = a.activity.map { t -> NSView in
                    let tile = Box(fill: colors.accent.withAlphaComponent(0.16), radius: MavioRadius.sm)
                    tile.size(width: 26, height: 26)
                    let glyph = IconView("search", size: 14, color: colors.accent)
                    tile.addSubview(glyph)
                    glyph.center(in: tile)
                    let label = NSTextField.label(t.summary, fonts.footnote, color: colors.textSecondary, lines: 0)
                    label.setContentHuggingPriority(.init(1), for: .horizontal)
                    // The card's width less its insets, the tile, the end and the gaps.
                    label.preferredMaxLayoutWidth = Self.cardWidth - 28 - 26 - 16 - 20
                    let end: NSView
                    if t.failed {
                        end = IconView("close", size: 16, color: colors.danger)
                    } else if t.done {
                        end = IconView("check", size: 16, color: colors.success)
                    } else {
                        let spin = NSProgressIndicator()
                        spin.style = .spinning
                        spin.controlSize = .small
                        spin.startAnimation(nil)
                        spin.translatesAutoresizingMaskIntoConstraints = false
                        spin.size(width: 16, height: 16)
                        end = spin
                    }
                    return NSStackView.horizontal([tile, label, end], spacing: 10)
                }
                views.append(
                    Card(
                        NSStackView.vertical(rows, spacing: MavioSpacing.s8, alignment: .leading).filled(),
                        insets: NSEdgeInsets(top: 12, left: 14, bottom: 12, right: 14)))
            }
            if !a.items.isEmpty {
                let rows = a.items.enumerated().map { i, item -> NSView in
                    var parts: [NSView] = []
                    if a.choose {
                        let n = NSTextField.label("\(i + 1)", fonts.footnote, color: colors.textTertiary)
                        n.font = .systemFont(ofSize: fonts.footnote.size, weight: .semibold)
                        n.alignment = .center
                        n.size(width: 16)
                        parts.append(n)
                    }
                    let art = ArtworkView(a.poster(item, maxWidth: 80), title: item.name, icon: "movies")
                    art.radius = 5
                    art.size(width: 34, height: 51)
                    parts.append(art)
                    let title = NSTextField.label(item.name, fonts.callout)
                    title.font = .systemFont(ofSize: fonts.callout.size, weight: .medium)
                    var texts: [NSView] = [title]
                    if let s = Assistant.subtitle(item) {
                        texts.append(NSTextField.label(s, fonts.caption, color: colors.textSecondary))
                    }
                    let column = NSStackView.vertical(texts, spacing: 1)
                    column.setContentHuggingPriority(.init(1), for: .horizontal)
                    parts.append(column)
                    let row = NSStackView.horizontal(parts, spacing: 10)
                    return Pressable(
                        row, radius: MavioRadius.md, insets: .all(6), theme: .desktop
                    ) { [weak self] in self?.assistant.choose(item) }
                }
                views.append(
                    Card(
                        NSStackView.vertical(rows, spacing: 2, alignment: .leading).filled(),
                        insets: .all(10)))
            }
            if let c = a.confirmation {
                let text = NSTextField.label(c.text, fonts.body, lines: 0)
                let cancel = MavioButton(MavioStrings.actionCancel, kind: .secondary, large: true) { [weak self] in
                    self?.assistant.confirm(false)
                }
                let confirm = MavioButton(MavioStrings.assistantConfirm, kind: .primary, large: true) { [weak self] in
                    self?.assistant.confirm(true)
                }
                let buttons = NSStackView.horizontal([cancel, confirm], spacing: MavioSpacing.s8)
                buttons.distribution = .fillEqually
                views.append(
                    Card(
                        NSStackView.vertical([text, buttons], spacing: MavioSpacing.s12, alignment: .leading).filled()))
            }
            if let e = a.error {
                views.append(Card(NSTextField.label(e, fonts.body, color: colors.danger, lines: 0, selectable: true)))
            }
            cards.setViews(views)
        }

        /// Lights the answer's words spoken so far.
        private func light() {
            guard let spoken else { return }
            let live = assistant.answerLive
            if live { played = max(played, assistant.speech()?.played ?? 0) } else { played = .infinity }
            let count = segments.prefix { $0.at <= played + 120 }.count
            guard count != lit else { return }
            lit = count
            let colors = MavioNSColors.system
            let font = MavioFonts.desktop.body.font
            let text = NSMutableAttributedString()
            for (i, s) in segments.enumerated() {
                text.append(
                    NSAttributedString(
                        string: s.text,
                        attributes: [
                            .font: font, .foregroundColor: i < count ? colors.textPrimary : colors.textTertiary,
                        ]))
            }
            spoken.attributedStringValue = text
        }

        /// Each frame of the orb.
        private func frame() {
            if !assistant.answerLive && !assistant.answer.isEmpty && played.isFinite { played = .infinity }
            if assistant.answer.isEmpty { played = 0 }
            light()
            // Leaving, it shrinks about its center over the fade, as the web
            // UI's.
            let now = CACurrentMediaTime()
            if assistant.leaving {
                if leftAt == nil { leftAt = now }
            } else if assistant.open {
                leftAt = nil
            }
            let t = leftAt.map { min(1, (now - $0) / MavioDuration.slow) } ?? 0
            let scale = 1 - 0.6 * (1 - pow(1 - t, 3))
            let c = CGPoint(x: orbHolder.bounds.midX, y: orbHolder.bounds.midY)
            orbHolder.layer?.setAffineTransform(
                CGAffineTransform(translationX: c.x + orb.offset, y: c.y).scaledBy(x: scale, y: scale)
                    .translatedBy(x: -c.x, y: -c.y))
        }
    }

    extension NSStackView {
        /// The stack, its views as wide as it.
        fileprivate func filled() -> NSStackView {
            for view in arrangedSubviews {
                view.widthAnchor.constraint(equalTo: widthAnchor).isActive = true
            }
            return self
        }
    }

    /// A card of the assistant's: its content on the cards' material.
    private final class Card: Box {
        init(_ content: NSView, insets: NSEdgeInsets = NSEdgeInsets(top: 14, left: 16, bottom: 14, right: 16)) {
            super.init(radius: MavioRadius.xl)
            elevation = MavioNSElevation.level3
            let blur = NSVisualEffectView()
            blur.material = .popover
            blur.blendingMode = .withinWindow
            blur.state = .active
            blur.wantsLayer = true
            blur.layer?.cornerRadius = MavioRadius.xl
            blur.layer?.cornerCurve = .continuous
            blur.layer?.masksToBounds = true
            addFilling(blur)
            let tint = Box(fill: AssistantOverlay.cardColor, radius: MavioRadius.xl)
            tint.border = .assistantBorder
            addFilling(tint)
            addFilling(content, insets: insets)
            widthAnchor.constraint(equalToConstant: AssistantOverlay.cardWidth).isActive = true
            // Wrapping labels wrap at the card's width.
            Self.wrap(content, width: AssistantOverlay.cardWidth - insets.left - insets.right)
        }

        private static func wrap(_ view: NSView, width: CGFloat) {
            if let label = view as? NSTextField, label.maximumNumberOfLines != 1, label.preferredMaxLayoutWidth == 0 {
                label.preferredMaxLayoutWidth = width
            }
            for sub in view.subviews { wrap(sub, width: width) }
        }

        @available(*, unavailable)
        required init?(coder: NSCoder) { fatalError() }

        override func layout() {
            super.layout()
            layer?.shadowPath = CGPath(
                roundedRect: bounds, cornerWidth: MavioRadius.xl, cornerHeight: MavioRadius.xl, transform: nil)
        }
    }

    /// The orb, drawn each frame by `OrbRenderer` on a canvas half again as
    /// large as its box; pressed, it starts or ends the session as the
    /// assistant button does.
    final class OrbView: NSView {
        private let assistant: Assistant
        private let renderer = OrbRenderer()
        private var link: CADisplayLink?
        private var pressing = false
        /// Called each frame, after the orb moves on.
        var onFrame: (@MainActor () -> Void)?

        init(assistant: Assistant) {
            self.assistant = assistant
            super.init(frame: .zero)
            translatesAutoresizingMaskIntoConstraints = false
            wantsLayer = true
            layerContentsRedrawPolicy = .onSetNeedsDisplay
            setAccessibilityElement(true)
            setAccessibilityRole(.button)
            setAccessibilityLabel(MavioStrings.assistantTalk)
        }

        @available(*, unavailable)
        required init?(coder: NSCoder) { fatalError() }

        /// How far the orb is shaken sideways now.
        var offset: CGFloat { renderer.offset }

        /// Whether it draws each frame.
        var running = false {
            didSet {
                guard running != oldValue else { return }
                link?.invalidate()
                link = nil
                if running {
                    let link = displayLink(target: self, selector: #selector(tick))
                    link.add(to: .main, forMode: .common)
                    self.link = link
                }
            }
        }

        @objc private func tick(_ link: CADisplayLink) {
            let reduce = NSWorkspace.shared.accessibilityDisplayShouldReduceMotion
            renderer.step(
                now: CACurrentMediaTime(), mode: assistant.mode, level: assistant.level(),
                interrupts: assistant.interrupts, reduceMotion: reduce)
            needsDisplay = true
            onFrame?()
        }

        override func draw(_ dirtyRect: NSRect) {
            guard let g = NSGraphicsContext.current?.cgContext else { return }
            let light = effectiveAppearance.bestMatch(from: [.darkAqua, .aqua]) == .aqua
            let reduce = NSWorkspace.shared.accessibilityDisplayShouldReduceMotion
            let side = bounds.width
            if light {
                // A tight dark shadow and a wider one of the blue, under it.
                g.setShadow(
                    offset: CGSize(width: 0, height: -2), blur: 3,
                    color: CGColor(srgbRed: 20 / 255, green: 12 / 255, blue: 80 / 255, alpha: 0.22))
                g.beginTransparencyLayer(auxiliaryInfo: nil)
                g.setShadow(
                    offset: CGSize(width: 0, height: -10), blur: 18,
                    color: CGColor(srgbRed: 90 / 255, green: 70 / 255, blue: 245 / 255, alpha: 0.42))
                g.beginTransparencyLayer(auxiliaryInfo: nil)
            }
            renderer.draw(in: g, side: side, now: CACurrentMediaTime(), light: light, reduceMotion: reduce)
            if light {
                g.endTransparencyLayer()
                g.endTransparencyLayer()
            }
        }

        override func acceptsFirstMouse(for event: NSEvent?) -> Bool { true }

        override func mouseDown(with event: NSEvent) {
            pressing = true
            assistant.press()
        }

        override func mouseUp(with event: NSEvent) {
            guard pressing else { return }
            pressing = false
            assistant.release()
        }

        override func accessibilityPerformPress() -> Bool {
            assistant.tap()
            return true
        }
    }

    extension NSColor {
        /// The cards' 1 pt border.
        fileprivate static var assistantBorder: NSColor { MavioNSColors.system.border }
    }
#endif
