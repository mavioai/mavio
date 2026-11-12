#if os(macOS)
    import AppKit
    import MavioCommon

    /// A QR code, drawn dark on light whatever the theme, for a phone to
    /// scan, with a quiet zone around it as scanners need, fitted in a
    /// square `side` wide.
    public final class QRView: NSView {
        private let dark: [[Bool]]
        private let quiet: CGFloat

        /// A QR code's modules in a white frame `quiet` wide, rounded by
        /// `radius`, read out as `label`.
        public init(
            _ modules: QRModules, side: CGFloat = 224, quiet: CGFloat = 12, radius: CGFloat = 8, label: String = "QR"
        ) {
            self.dark = modules.dark
            self.quiet = quiet
            super.init(frame: .zero)
            translatesAutoresizingMaskIntoConstraints = false
            let width = CGFloat(dark.map(\.count).max() ?? 0)
            let height = CGFloat(max(dark.count, 1))
            let inner = side - quiet * 2
            let module = min(inner / max(width, 1), inner / height)
            size(width: width * module + quiet * 2, height: height * module + quiet * 2)
            wantsLayer = true
            layer?.cornerRadius = radius
            layer?.masksToBounds = true
            setAccessibilityElement(true)
            setAccessibilityRole(.image)
            setAccessibilityLabel(label)
        }

        @available(*, unavailable)
        required init?(coder: NSCoder) { fatalError() }

        override public var isFlipped: Bool { true }

        override public func draw(_ dirtyRect: NSRect) {
            NSColor.white.setFill()
            bounds.fill()
            let width = CGFloat(dark.map(\.count).max() ?? 0)
            let inner = bounds.insetBy(dx: quiet, dy: quiet)
            let module = min(inner.width / max(width, 1), inner.height / CGFloat(max(dark.count, 1)))
            NSColor.black.setFill()
            for (y, row) in dark.enumerated() {
                for (x, on) in row.enumerated() where on {
                    CGRect(
                        x: inner.minX + CGFloat(x) * module, y: inner.minY + CGFloat(y) * module, width: module,
                        height: module
                    ).fill()
                }
            }
        }
    }
#endif
