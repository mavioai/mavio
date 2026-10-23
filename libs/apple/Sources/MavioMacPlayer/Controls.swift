#if os(macOS)
    import AppKit
    import MavioMac

    /// A view whose own area lets clicks through to what lies under it; its
    /// subviews still take them.
    @MainActor
    class PassThroughView: FlippedView {
        override func hitTest(_ point: NSPoint) -> NSView? {
            let hit = super.hitTest(point)
            return hit === self ? nil : hit
        }
    }

    /// A gradient of black from top to bottom, at stops given from the top.
    @MainActor
    final class ShadeView: PassThroughView {
        private let gradient: NSGradient?

        /// A gradient through stops of a location from the top and an
        /// opacity of black.
        init(_ stops: [(location: CGFloat, opacity: CGFloat)]) {
            var locations = stops.map(\.location)
            gradient = NSGradient(
                colors: stops.map { NSColor.black.withAlphaComponent($0.opacity) }, atLocations: &locations,
                colorSpace: .sRGB)
            super.init(frame: .zero)
            translatesAutoresizingMaskIntoConstraints = false
        }

        @available(*, unavailable)
        required init?(coder: NSCoder) { fatalError() }

        override func draw(_ dirtyRect: NSRect) {
            // The view is flipped: its top is at y 0.
            gradient?.draw(from: .zero, to: NSPoint(x: 0, y: bounds.height), options: [])
        }

        override func hitTest(_ point: NSPoint) -> NSView? { nil }
    }

    /// An icon button of the player's controller: white, 36 pt square, with
    /// a tooltip naming it.
    @MainActor
    final class IconButton: NSButton {
        /// What a click does.
        var onPress: @MainActor () -> Void
        private let iconSize: CGFloat

        /// A button of icon, filled when fill, size points large, labelled
        /// label, running run.
        init(
            _ icon: String, _ label: String, size: CGFloat = 22, fill: Bool = true, run: @escaping @MainActor () -> Void
        ) {
            onPress = run
            iconSize = size
            super.init(frame: .zero)
            isBordered = false
            bezelStyle = .regularSquare
            imagePosition = .imageOnly
            imageScaling = .scaleProportionallyDown
            contentTintColor = .white
            refusesFirstResponder = true
            target = self
            action = #selector(pressed)
            self.size(width: 36, height: 36)
            set(icon, label, fill: fill)
        }

        @available(*, unavailable)
        required init?(coder: NSCoder) { fatalError() }

        /// Shows another icon and label.
        func set(_ icon: String, _ label: String, fill: Bool = true) {
            let image = NSImage.icon(icon, fill: fill).copy() as? NSImage ?? NSImage()
            image.isTemplate = true
            image.size = NSSize(width: iconSize, height: iconSize)
            self.image = image
            toolTip = label
            setAccessibilityLabel(label)
        }

        @objc private func pressed() { onPress() }
    }

    /// The timeline of television mode's controller: the bar, with the
    /// position and the time left under it.
    @MainActor
    final class TimelineBar: NSView {
        private let track = Box(fill: .white.withAlphaComponent(0.3), radius: 999)
        private let filled = Box(fill: .white, radius: 999)
        private let elapsed: NSTextField
        private let remaining: NSTextField
        private var fraction: CGFloat = 0
        private var width: NSLayoutConstraint?

        /// A timeline in theme.
        init(theme: Theme) {
            let font = NSFont.monospacedDigitSystemFont(ofSize: theme.fonts.footnote.size, weight: .regular)
            elapsed = .label("0:00", theme.fonts.footnote, color: .white.withAlphaComponent(0.8))
            remaining = .label("-0:00", theme.fonts.footnote, color: .white.withAlphaComponent(0.8))
            elapsed.font = font
            remaining.font = font
            super.init(frame: .zero)
            translatesAutoresizingMaskIntoConstraints = false
            setAccessibilityElement(true)
            setAccessibilityRole(.progressIndicator)
            setAccessibilityLabel(MavioStrings.playerPosition)
            addSubview(track)
            track.addSubview(filled)
            let times = NSStackView.horizontal([elapsed, Spacer(), remaining])
            addSubview(times)
            NSLayoutConstraint.activate([
                track.topAnchor.constraint(equalTo: topAnchor),
                track.leadingAnchor.constraint(equalTo: leadingAnchor),
                track.trailingAnchor.constraint(equalTo: trailingAnchor),
                track.heightAnchor.constraint(equalToConstant: 10),
                filled.leadingAnchor.constraint(equalTo: track.leadingAnchor),
                filled.topAnchor.constraint(equalTo: track.topAnchor),
                filled.bottomAnchor.constraint(equalTo: track.bottomAnchor),
                times.topAnchor.constraint(equalTo: track.bottomAnchor, constant: MavioSpacing.s8),
                times.leadingAnchor.constraint(equalTo: leadingAnchor),
                times.trailingAnchor.constraint(equalTo: trailingAnchor),
                times.bottomAnchor.constraint(equalTo: bottomAnchor),
            ])
            set(position: 0, duration: 0)
        }

        @available(*, unavailable)
        required init?(coder: NSCoder) { fatalError() }

        /// Shows position of duration.
        func set(position: TimeInterval, duration: TimeInterval) {
            elapsed.stringValue = clock(position)
            remaining.stringValue = "-" + clock(max(duration - position, 0))
            fraction = duration > 0 ? CGFloat(min(position / duration, 1)) : 0
            setAccessibilityValue(elapsed.stringValue)
            width?.isActive = false
            width = filled.widthAnchor.constraint(equalTo: track.widthAnchor, multiplier: max(fraction, 0.0001))
            width?.isActive = true
        }
    }

    /// A choice of television mode's side panel: its label, a detail under
    /// it and a check when it is the current one; white with black content
    /// while focused.
    @MainActor
    final class ChoiceRow: Box, Focusable {
        private let theme: Theme
        private let titleLabel: NSTextField
        private let detailLabel: NSTextField?
        private let check: IconView
        /// What select does.
        var onPress: (@MainActor () -> Void)?

        /// A choice of label and detail, checked when current.
        init(_ label: String, detail: String?, checked: Bool, theme: Theme) {
            self.theme = theme
            titleLabel = .label(label, theme.fonts.body, color: .white)
            detailLabel = detail.flatMap { $0.isEmpty ? nil : .label($0, theme.fonts.footnote, color: .white) }
            detailLabel?.alphaValue = 0.7
            check = IconView("check", size: 28, color: .white)
            check.isHidden = !checked
            super.init(radius: 14)
            let text = NSStackView.vertical([titleLabel, detailLabel].compactMap(\.self), spacing: 2)
            let row = NSStackView.horizontal([text, Spacer(), check], spacing: MavioSpacing.s12)
            addFilling(
                row,
                insets: NSEdgeInsets(
                    top: MavioSpacing.s16, left: MavioSpacing.s24, bottom: MavioSpacing.s16, right: MavioSpacing.s24))
            setAccessibilityElement(true)
            setAccessibilityRole(.button)
            setAccessibilityLabel(label)
            setAccessibilitySelected(checked)
        }

        @available(*, unavailable)
        required init?(coder: NSCoder) { fatalError() }

        /// Whether focus is on the row.
        var isFocused = false {
            didSet {
                fill = isFocused ? .white : nil
                elevation =
                    isFocused ? MavioNSShadow(color: theme.colors.tvFocusShadow, x: 0, y: 12, blur: 32, spread: 0) : nil
                let color: NSColor = isFocused ? .black : .white
                titleLabel.textColor = color
                detailLabel?.textColor = color
                check.contentTintColor = color
            }
        }

        /// Picks the choice.
        func activate() { onPress?() }
    }

    /// A brief notice at the player's top leading corner of what a key, the
    /// wheel or a control changed — the volume, the speed, a seek — with a
    /// bar of the level where there is one: shown at once, faded out a
    /// second after the last change.
    @MainActor
    final class PlayerNotice: Box {
        private let icon = IconView("volume", fill: true, size: 18, color: .white)
        private let label: NSTextField
        private let track = Box(fill: .white.withAlphaComponent(0.3), radius: 999)
        private let filled = Box(fill: .white, radius: 999)
        private var level: NSLayoutConstraint?
        private var hideTask: Task<Void, Never>?

        /// A notice in theme.
        init(theme: Theme) {
            label = .label("", theme.fonts.headline, color: .white)
            label.font = NSFont.monospacedDigitSystemFont(ofSize: theme.fonts.headline.size, weight: .semibold)
            super.init(fill: NSColor.black.withAlphaComponent(0.6), radius: 10)
            track.addSubview(filled)
            let row = NSStackView.horizontal([icon, label, track], spacing: MavioSpacing.s12)
            addFilling(row, insets: NSEdgeInsets(top: 8, left: 14, bottom: 8, right: 16))
            NSLayoutConstraint.activate([
                track.widthAnchor.constraint(equalToConstant: 100),
                track.heightAnchor.constraint(equalToConstant: 6),
                filled.leadingAnchor.constraint(equalTo: track.leadingAnchor),
                filled.topAnchor.constraint(equalTo: track.topAnchor),
                filled.bottomAnchor.constraint(equalTo: track.bottomAnchor),
            ])
            alphaValue = 0
            setAccessibilityElement(true)
            setAccessibilityRole(.staticText)
        }

        @available(*, unavailable)
        required init?(coder: NSCoder) { fatalError() }

        override func hitTest(_ point: NSPoint) -> NSView? { nil }

        /// Shows icon and text, with a bar at level, from 0 to 1, when given.
        func show(_ icon: String, _ text: String, level: Double? = nil) {
            self.icon.name = icon
            label.stringValue = text
            setAccessibilityLabel(text)
            track.isHidden = level == nil
            self.level?.isActive = false
            self.level = filled.widthAnchor.constraint(
                equalTo: track.widthAnchor, multiplier: max(CGFloat(min(level ?? 0, 1)), 0.0001))
            self.level?.isActive = true
            hideTask?.cancel()
            NSAnimationContext.runAnimationGroup(
                { context in
                    context.duration = 0
                    animator().alphaValue = 1
                }, completionHandler: nil)
            hideTask = Task { @MainActor [weak self] in
                try? await Task.sleep(for: .seconds(1))
                guard !Task.isCancelled, let self else { return }
                NSAnimationContext.runAnimationGroup(
                    { context in
                        context.duration = 0.5
                        self.animator().alphaValue = 0
                    }, completionHandler: nil)
            }
        }
    }

    /// A time as the player shows it: minutes and seconds, with hours
    /// before them from an hour on.
    func clock(_ t: TimeInterval) -> String {
        let s = Int(t.isFinite ? max(t, 0) : 0)
        return s >= 3600
            ? String(format: "%d:%02d:%02d", s / 3600, s / 60 % 60, s % 60) : String(format: "%d:%02d", s / 60, s % 60)
    }
#endif
