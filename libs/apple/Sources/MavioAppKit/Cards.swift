#if os(macOS)
    import AppKit
    import MavioCommon
    import MavioDesign
    import MavioKit

    /// A card: artwork of an aspect over its title and subtitle, opening
    /// what it shows when clicked or selected. In television mode a focused
    /// card scales by `layout.focusScale` and its artwork gets a 3 pt
    /// `color.accent` ring 1.5 pt apart and a shadow (docs/tv.md §4).
    public class CardView: FlippedView, Focusable {
        /// The artwork.
        public let artwork: ArtworkView
        /// The title, under the artwork.
        public let titleLabel: NSTextField
        /// The subtitle, under the title.
        public let subtitleLabel: NSTextField
        /// The overlays' layer over the artwork, for marks.
        public let overlay = FlippedView()
        let theme: Theme
        private let ring = Box()
        private let progress = Box()
        private let progressTrack = Box()
        private var progressWidth: NSLayoutConstraint?
        private let check: Box
        private let left: NSTextField
        private let leftBox: Box
        private var tracking: NSTrackingArea?
        private var hovering = false
        /// What a click or select does.
        public var onPress: (@MainActor () -> Void)?
        /// The menu a secondary click shows, if any.
        public var contextMenu: (@MainActor () -> NSMenu?)?

        /// A card of width whose artwork is aspect wide to its height
        /// (2/3 for a poster, 16/9 for a thumbnail, 1 for a circle), round
        /// for a person's photo.
        public init(width: CGFloat?, aspect: CGFloat, round: Bool = false, icon: String = "movies", theme: Theme) {
            self.theme = theme
            artwork = ArtworkView(icon: icon, theme: theme)
            artwork.radius = round ? 9999 : MavioRadius.md
            titleLabel = .label("", theme.fonts.headline, color: theme.colors.textPrimary)
            subtitleLabel = .label(" ", theme.fonts.footnote, color: theme.colors.textSecondary)
            check = Box(fill: theme.colors.scrim, radius: 999)
            left = .label("", theme.fonts.caption, color: .white)
            leftBox = Box(fill: theme.colors.scrim, radius: MavioRadius.sm)
            super.init(frame: .zero)
            translatesAutoresizingMaskIntoConstraints = false
            wantsLayer = true
            ring.border = theme.colors.accent
            ring.borderWidth = 3
            ring.radius = round ? 9999 : MavioRadius.md + 4.5
            ring.isHidden = true
            if round {
                titleLabel.alignment = .center
                subtitleLabel.alignment = .center
                titleLabel.font = theme.fonts.callout.font
                titleLabel.maximumNumberOfLines = 2
                subtitleLabel.maximumNumberOfLines = 2
                titleLabel.lineBreakMode = .byWordWrapping
                subtitleLabel.lineBreakMode = .byWordWrapping
            }
            for v in [artwork, overlay, ring, titleLabel, subtitleLabel] as [NSView] { addSubview(v) }
            overlay.translatesAutoresizingMaskIntoConstraints = false
            let side: CGFloat = round ? (theme.isTV ? 160 : 96) : 0
            NSLayoutConstraint.activate([
                artwork.topAnchor.constraint(equalTo: topAnchor),
                artwork.widthAnchor.constraint(equalTo: artwork.heightAnchor, multiplier: aspect),
                overlay.leadingAnchor.constraint(equalTo: artwork.leadingAnchor),
                overlay.trailingAnchor.constraint(equalTo: artwork.trailingAnchor),
                overlay.topAnchor.constraint(equalTo: artwork.topAnchor),
                overlay.bottomAnchor.constraint(equalTo: artwork.bottomAnchor),
                ring.leadingAnchor.constraint(equalTo: artwork.leadingAnchor, constant: -4.5),
                ring.trailingAnchor.constraint(equalTo: artwork.trailingAnchor, constant: 4.5),
                ring.topAnchor.constraint(equalTo: artwork.topAnchor, constant: -4.5),
                ring.bottomAnchor.constraint(equalTo: artwork.bottomAnchor, constant: 4.5),
                titleLabel.topAnchor.constraint(
                    equalTo: artwork.bottomAnchor, constant: round ? MavioSpacing.s8 : MavioSpacing.s4),
                titleLabel.leadingAnchor.constraint(equalTo: leadingAnchor),
                titleLabel.trailingAnchor.constraint(equalTo: trailingAnchor),
                subtitleLabel.topAnchor.constraint(equalTo: titleLabel.bottomAnchor),
                subtitleLabel.leadingAnchor.constraint(equalTo: leadingAnchor),
                subtitleLabel.trailingAnchor.constraint(equalTo: trailingAnchor),
                subtitleLabel.bottomAnchor.constraint(lessThanOrEqualTo: bottomAnchor),
            ])
            if round {
                artwork.size(width: side, height: side)
                artwork.centerXAnchor.constraint(equalTo: centerXAnchor).isActive = true
            } else {
                artwork.leadingAnchor.constraint(equalTo: leadingAnchor).isActive = true
                artwork.trailingAnchor.constraint(equalTo: trailingAnchor).isActive = true
            }
            if let width { self.size(width: width) }
            setupOverlays()
            setAccessibilityElement(true)
            setAccessibilityRole(.button)
        }

        /// Unavailable: the view is made in code.
        @available(*, unavailable)
        public required init?(coder: NSCoder) { fatalError() }

        private func setupOverlays() {
            let icon = IconView("played", fill: true, size: 18, color: .white)
            check.addFilling(icon, insets: .all(MavioSpacing.s2))
            check.isHidden = true
            leftBox.addFilling(left, insets: .symmetric(horizontal: MavioSpacing.s4))
            leftBox.isHidden = true
            progressTrack.fill = theme.colors.scrim
            progress.fill = theme.colors.progress
            progressTrack.isHidden = true
            progressTrack.addSubview(progress)
            progress.translatesAutoresizingMaskIntoConstraints = false
            let width = progress.widthAnchor.constraint(equalToConstant: 0)
            progressWidth = width
            NSLayoutConstraint.activate([
                progress.leadingAnchor.constraint(equalTo: progressTrack.leadingAnchor),
                progress.topAnchor.constraint(equalTo: progressTrack.topAnchor),
                progress.bottomAnchor.constraint(equalTo: progressTrack.bottomAnchor), width,
            ])
            for v in [check, leftBox, progressTrack] { overlay.addSubview(v) }
            NSLayoutConstraint.activate([
                check.topAnchor.constraint(equalTo: overlay.topAnchor, constant: MavioSpacing.s8),
                check.trailingAnchor.constraint(equalTo: overlay.trailingAnchor, constant: -MavioSpacing.s8),
                progressTrack.leadingAnchor.constraint(equalTo: overlay.leadingAnchor),
                progressTrack.trailingAnchor.constraint(equalTo: overlay.trailingAnchor),
                progressTrack.bottomAnchor.constraint(equalTo: overlay.bottomAnchor),
                progressTrack.heightAnchor.constraint(equalToConstant: 3),
                leftBox.leadingAnchor.constraint(equalTo: overlay.leadingAnchor, constant: MavioSpacing.s8),
                leftBox.bottomAnchor.constraint(equalTo: progressTrack.topAnchor, constant: -MavioSpacing.s4),
            ])
            overlay.wantsLayer = true
            overlay.layer?.cornerRadius = MavioRadius.md
            overlay.layer?.masksToBounds = true
        }

        /// Shows a title and subtitle under the artwork, and a title to read
        /// aloud.
        public func setText(_ title: String, _ subtitle: String?) {
            titleLabel.stringValue = title
            subtitleLabel.stringValue = subtitle ?? " "
            subtitleLabel.isHidden = subtitle == nil && titleLabel.maximumNumberOfLines != 1
            setAccessibilityLabel(title)
        }

        /// Shows a card's marks: the played check, the progress begun, and
        /// the minutes left when timeLeft.
        public func setMarks(played: Bool, progress value: Float?, minutesLeft: Int?) {
            check.isHidden = !played
            let begun = !played && (value ?? 0) > 0
            progressTrack.isHidden = !begun
            leftBox.isHidden = !begun || minutesLeft == nil
            if let minutesLeft { left.stringValue = MavioStrings.itemMinutesLeft(count: minutesLeft) }
            progressRatio = CGFloat(min(value ?? 0, 1))
            needsLayout = true
        }

        private var progressRatio: CGFloat = 0

        override public func layout() {
            super.layout()
            progressWidth?.constant = overlay.bounds.width * progressRatio
        }

        // MARK: Focus and pointer

        /// Whether television mode's focus is on the card.
        public var isFocused = false {
            didSet {
                guard oldValue != isFocused else { return }
                ring.isHidden = !isFocused
                artwork.elevation =
                    isFocused ? MavioNSShadow(color: theme.colors.tvFocusShadow, x: 0, y: 12, blur: 32, spread: 0) : nil
                scale(isFocused ? theme.layout.focusScale : 1)
            }
        }

        private func scale(_ factor: CGFloat) {
            guard let layer else { return }
            let b = layer.bounds
            var t = CATransform3DMakeTranslation(b.midX, b.midY, 0)
            t = CATransform3DScale(t, factor, factor, 1)
            t = CATransform3DTranslate(t, -b.midX, -b.midY, 0)
            CATransaction.begin()
            CATransaction.setAnimationDuration(MavioDuration.fast)
            CATransaction.setAnimationTimingFunction(MavioEasings.standard.timingFunction)
            layer.transform = t
            CATransaction.commit()
        }

        /// Opens what the card shows.
        public func activate() { onPress?() }

        override public func mouseUp(with event: NSEvent) {
            if bounds.contains(convert(event.locationInWindow, from: nil)) { activate() }
        }

        override public func mouseDown(with event: NSEvent) {}

        override public func menu(for event: NSEvent) -> NSMenu? {
            contextMenu?()
        }

        override public func updateTrackingAreas() {
            super.updateTrackingAreas()
            if let tracking { removeTrackingArea(tracking) }
            guard !theme.isTV else { return }
            let area = NSTrackingArea(
                rect: bounds, options: [.mouseEnteredAndExited, .activeInKeyWindow, .inVisibleRect], owner: self)
            addTrackingArea(area)
            tracking = area
        }

        override public func mouseEntered(with event: NSEvent) {
            hovering = true
            artwork.alphaValue = 0.85
        }

        override public func mouseExited(with event: NSEvent) {
            hovering = false
            artwork.alphaValue = 1
        }

        override public func accessibilityPerformPress() -> Bool {
            activate()
            return true
        }

        override public func viewWillMove(toWindow newWindow: NSWindow?) {
            super.viewWillMove(toWindow: newWindow)
            focusWillMove(to: newWindow)
        }
    }

    extension CardView {
        /// The card of a server's item: a poster, or a 16:9 thumbnail when
        /// wide, width wide (nil fills the space given, as in grids).
        public static func item(_ card: Card, wide: Bool? = nil, width: CGFloat? = nil, theme: Theme) -> CardView {
            let isWideCard = wide ?? isWide(card)
            let view = CardView(width: width, aspect: isWideCard ? 16 / 9 : 2 / 3, icon: card.kind.icon, theme: theme)
            view.show(card, wide: isWideCard)
            return view
        }

        /// Shows a server's item.
        public func show(_ card: Card, wide: Bool) {
            let url = wide ? (card.thumb?.url ?? card.poster?.url) : (card.poster?.url ?? card.thumb?.url)
            artwork.set(url, title: card.title, icon: card.kind.icon)
            setText(card.title, subtitle(card))
            setMarks(played: card.played, progress: card.progress, minutesLeft: wide ? minutesLeft(card) : nil)
        }

        /// The card of a person: the photo in a circle, the name and the
        /// role (libs/design/components.md, Person Card).
        public static func person(name: String, photo: String?, role: String?, theme: Theme) -> CardView {
            let view = CardView(
                width: theme.isTV ? 180 : 112, aspect: 1, round: true, icon: "profile", theme: theme)
            view.artwork.set(photo, title: name, icon: "profile")
            view.setText(name, role)
            return view
        }

        /// A card loading: its artwork's shape and a bar where its title
        /// will be, as tall as the card, so that nothing moves when it
        /// arrives.
        public static func placeholder(wide: Bool, width: CGFloat?, theme: Theme) -> CardView {
            let view = CardView(width: width, aspect: wide ? 16 / 9 : 2 / 3, theme: theme)
            view.artwork.setEmpty()
            view.setText(" ", " ")
            let bar = Box(fill: theme.colors.fill, radius: MavioRadius.sm)
            view.addSubview(bar)
            NSLayoutConstraint.activate([
                bar.leadingAnchor.constraint(equalTo: view.titleLabel.leadingAnchor),
                bar.centerYAnchor.constraint(equalTo: view.titleLabel.centerYAnchor),
                bar.heightAnchor.constraint(equalToConstant: 14),
                bar.widthAnchor.constraint(equalTo: view.widthAnchor, multiplier: 0.75),
            ])
            view.setAccessibilityElement(false)
            return view
        }
    }
#endif
