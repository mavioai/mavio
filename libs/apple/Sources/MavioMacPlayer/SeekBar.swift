#if os(macOS)
    import AppKit
    import MavioMac

    /// The desktop controller's seek bar: the track with what has played
    /// filled, gaps where chapters start, and the knob while the pointer is
    /// over it or drags it. The pointer's time over it is told as it moves,
    /// for the picture and the chapter's name shown above it.
    @MainActor
    final class SeekBar: NSView {
        /// The pointer's time over the bar and where it is across the bar,
        /// nil once it leaves.
        var onHover: (@MainActor (_ time: TimeInterval?, _ x: CGFloat) -> Void)?
        /// A drag began.
        var onScrubStart: (@MainActor () -> Void)?
        /// A drag moved to a time.
        var onScrub: (@MainActor (TimeInterval) -> Void)?
        /// A drag ended at a time.
        var onScrubEnd: (@MainActor (TimeInterval) -> Void)?

        private(set) var duration: TimeInterval = 0
        private var position: TimeInterval = 0
        private var chapters: [TimeInterval] = []
        private var hovering = false
        private var dragging = false
        private var tracking: NSTrackingArea?

        private static let trackHeight: CGFloat = 4
        private static let knob: CGFloat = 12
        /// The gap cut where a chapter starts.
        private static let gap: CGFloat = 2

        override init(frame: NSRect) {
            super.init(frame: frame)
            translatesAutoresizingMaskIntoConstraints = false
            heightAnchor.constraint(equalToConstant: 18).isActive = true
            setContentHuggingPriority(.init(1), for: .horizontal)
            setContentCompressionResistancePriority(.init(1), for: .horizontal)
            setAccessibilityElement(true)
            setAccessibilityRole(.slider)
            setAccessibilityLabel(MavioStrings.playerPosition)
        }

        @available(*, unavailable)
        required init?(coder: NSCoder) { fatalError() }

        override var isFlipped: Bool { true }
        override var mouseDownCanMoveWindow: Bool { false }
        override func acceptsFirstMouse(for event: NSEvent?) -> Bool { true }

        /// Shows position of duration.
        func set(position: TimeInterval, duration: TimeInterval) {
            guard position != self.position || duration != self.duration else { return }
            (self.position, self.duration) = (position, duration)
            setAccessibilityValue(clock(position))
            needsDisplay = true
        }

        /// Cuts gaps where the chapters start.
        func set(chapters: [TimeInterval]) {
            self.chapters = chapters.filter { $0 > 0 }
            needsDisplay = true
        }

        /// The time at a point across the bar.
        func time(at x: CGFloat) -> TimeInterval {
            let usable = max(bounds.width - Self.knob, 1)
            return duration * Double(min(max((x - Self.knob / 2) / usable, 0), 1))
        }

        /// The point across the bar of a time.
        private func x(of time: TimeInterval) -> CGFloat {
            let fraction = duration > 0 ? CGFloat(min(max(time / duration, 0), 1)) : 0
            return Self.knob / 2 + fraction * max(bounds.width - Self.knob, 1)
        }

        override func draw(_ dirtyRect: NSRect) {
            let height = hovering || dragging ? Self.trackHeight + 2 : Self.trackHeight
            let track = NSRect(
                x: Self.knob / 2, y: (bounds.height - height) / 2, width: max(bounds.width - Self.knob, 0),
                height: height)
            NSGraphicsContext.saveGraphicsState()
            // The chapters' gaps are left out of the track and its fill.
            let clip = NSBezierPath(rect: bounds)
            clip.windingRule = .evenOdd
            for chapter in chapters where duration > 0 && chapter < duration {
                clip.appendRect(NSRect(x: x(of: chapter) - Self.gap / 2, y: 0, width: Self.gap, height: bounds.height))
            }
            clip.addClip()
            NSColor.white.withAlphaComponent(0.3).setFill()
            NSBezierPath(roundedRect: track, xRadius: height / 2, yRadius: height / 2).fill()
            var played = track
            played.size.width = x(of: position) - track.minX
            NSColor.white.setFill()
            NSBezierPath(roundedRect: played, xRadius: height / 2, yRadius: height / 2).fill()
            NSGraphicsContext.restoreGraphicsState()
            guard hovering || dragging, duration > 0 else { return }
            let knob = NSRect(
                x: x(of: position) - Self.knob / 2, y: (bounds.height - Self.knob) / 2, width: Self.knob,
                height: Self.knob)
            NSColor.white.setFill()
            NSBezierPath(ovalIn: knob).fill()
        }

        override func updateTrackingAreas() {
            super.updateTrackingAreas()
            if let tracking { removeTrackingArea(tracking) }
            let area = NSTrackingArea(
                rect: bounds, options: [.mouseMoved, .mouseEnteredAndExited, .activeInKeyWindow, .inVisibleRect],
                owner: self)
            addTrackingArea(area)
            tracking = area
        }

        override func mouseEntered(with event: NSEvent) { hover(event) }
        override func mouseMoved(with event: NSEvent) { hover(event) }

        override func mouseExited(with event: NSEvent) {
            hovering = false
            needsDisplay = true
            if !dragging { onHover?(nil, 0) }
        }

        private func hover(_ event: NSEvent) {
            hovering = true
            needsDisplay = true
            guard duration > 0 else { return }
            let x = convert(event.locationInWindow, from: nil).x
            onHover?(time(at: x), x)
        }

        override func mouseDown(with event: NSEvent) {
            guard duration > 0 else { return }
            dragging = true
            onScrubStart?()
            drag(event)
        }

        override func mouseDragged(with event: NSEvent) {
            guard dragging else { return }
            drag(event)
        }

        override func mouseUp(with event: NSEvent) {
            guard dragging else { return }
            dragging = false
            let x = convert(event.locationInWindow, from: nil).x
            let target = time(at: x)
            position = target
            needsDisplay = true
            onScrubEnd?(target)
            if bounds.contains(convert(event.locationInWindow, from: nil)) {
                onHover?(target, x)
            } else {
                hovering = false
                onHover?(nil, 0)
            }
        }

        private func drag(_ event: NSEvent) {
            let x = min(max(convert(event.locationInWindow, from: nil).x, 0), bounds.width)
            let target = time(at: x)
            position = target
            needsDisplay = true
            onScrub?(target)
            onHover?(target, x)
        }

        override func accessibilityPerformIncrement() -> Bool {
            onScrubEnd?(min(position + 10, duration))
            return true
        }

        override func accessibilityPerformDecrement() -> Bool {
            onScrubEnd?(max(position - 10, 0))
            return true
        }
    }

    /// The picture of a time over the seek bar, the chapter's name and the
    /// time under it.
    @MainActor
    final class ScrubPreview: Box {
        private let image = NSImageView()
        private let label: NSTextField
        private var imageHeight: NSLayoutConstraint!
        /// The picture's width.
        static let width: CGFloat = 176

        init(theme: Theme) {
            label = .label("", theme.fonts.footnote, color: .white)
            label.font = NSFont.monospacedDigitSystemFont(ofSize: theme.fonts.footnote.size, weight: .medium)
            label.alignment = .center
            label.lineBreakMode = .byTruncatingTail
            super.init(fill: NSColor.black.withAlphaComponent(0.75), radius: 8)
            image.imageScaling = .scaleProportionallyUpOrDown
            image.wantsLayer = true
            image.layer?.cornerRadius = 6
            image.layer?.masksToBounds = true
            image.translatesAutoresizingMaskIntoConstraints = false
            let stack = NSStackView.vertical([image, label], spacing: MavioSpacing.s4)
            stack.alignment = .centerX
            addFilling(stack, insets: NSEdgeInsets(top: 4, left: 4, bottom: 6, right: 4))
            imageHeight = image.heightAnchor.constraint(equalToConstant: 0)
            NSLayoutConstraint.activate([
                image.widthAnchor.constraint(equalToConstant: Self.width),
                label.widthAnchor.constraint(equalToConstant: Self.width),
                imageHeight,
            ])
            isHidden = true
        }

        @available(*, unavailable)
        required init?(coder: NSCoder) { fatalError() }

        /// Shows the time with the chapter's name.
        func set(time: TimeInterval, chapter: String?) {
            label.stringValue = [chapter, clock(time)].compactMap(\.self).joined(separator: " · ")
        }

        /// Shows a picture, or none.
        func set(picture: CGImage?) {
            image.image = picture.map { NSImage(cgImage: $0, size: .zero) }
            let aspect = picture.map { CGFloat($0.height) / CGFloat(max($0.width, 1)) } ?? 0
            imageHeight.constant = (Self.width * aspect).rounded()
            image.isHidden = picture == nil
        }
    }
#endif
