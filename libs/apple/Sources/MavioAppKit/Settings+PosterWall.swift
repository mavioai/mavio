#if os(macOS)
    import AppKit

    /// The wall behind sign-in in a wide window, as the web UI's: posters
    /// drawn from gradients, tilted, blurred under a veil, their columns
    /// drifting slowly up and down.
    public final class PosterWallView: FlippedView {
        /// The colors of the posters, top to bottom: warm, cool and dusk, as
        /// films are.
        private static let tones: [(UInt32, UInt32)] = [
            (0xFF9F0A, 0x6B2400), (0x2A6F97, 0x0A1A2E), (0x8E5CF7, 0x24104D), (0xF0412F, 0x3A0A0A),
            (0x30B07A, 0x0A2E20), (0xFFD34E, 0x7A3E00), (0x5AC8FA, 0x10325A), (0xFF6482, 0x420D26),
            (0xC7A17A, 0x33241A), (0x64D2FF, 0x1A2A66), (0xE07A5F, 0x2E1A3A),
        ]
        private static let columnCount = 8
        private static let rowCount = 9

        private let wall = CALayer()
        private var columns: [CALayer] = []
        private var posters: [[CAGradientLayer]] = []
        private let veil = CAGradientLayer()
        private var laidOutWidth: CGFloat = 0

        /// A wall filling the view.
        public init() {
            super.init(frame: .zero)
            translatesAutoresizingMaskIntoConstraints = false
            wantsLayer = true
            layerUsesCoreImageFilters = true
            layer?.masksToBounds = true
            layer?.addSublayer(wall)
            for c in 0..<Self.columnCount {
                let column = CALayer()
                var shown: [CAGradientLayer] = []
                for r in 0..<Self.rowCount {
                    let tone = Self.tones[(c * 4 + r * 7) % Self.tones.count]
                    let poster = CAGradientLayer()
                    poster.colors = [NSColor(hex: tone.0).cgColor, NSColor(hex: tone.1).cgColor]
                    poster.startPoint = CGPoint(x: 0.37, y: 0)
                    poster.endPoint = CGPoint(x: 0.63, y: 1)
                    poster.cornerRadius = 8
                    poster.masksToBounds = true
                    let shine = CAGradientLayer()
                    shine.type = .radial
                    shine.colors = [
                        NSColor.white.withAlphaComponent(0.28).cgColor, NSColor.white.withAlphaComponent(0).cgColor,
                    ]
                    poster.addSublayer(shine)
                    column.addSublayer(poster)
                    shown.append(poster)
                }
                wall.addSublayer(column)
                columns.append(column)
                posters.append(shown)
            }
            veil.type = .radial
            veil.startPoint = CGPoint(x: 0.5, y: 0.5)
            veil.endPoint = CGPoint(x: 1, y: 1)
            layer?.addSublayer(veil)
            setAccessibilityElement(false)
        }

        /// Unavailable: the view is made in code.
        @available(*, unavailable)
        public required init?(coder: NSCoder) { fatalError() }

        override public var wantsUpdateLayer: Bool { true }

        override public func updateLayer() {
            var dark = false
            effectiveAppearance.performAsCurrentDrawingAppearance {
                dark = NSAppearance.currentDrawing().bestMatch(from: [.aqua, .darkAqua]) == .darkAqua
            }
            wall.opacity = dark ? 0.7 : 0.6
            let tint: NSColor = dark ? .black : .white
            veil.colors = [
                tint.withAlphaComponent(0.4).cgColor, tint.withAlphaComponent(dark ? 0.82 : 0.8).cgColor,
            ]
        }

        override public func layout() {
            super.layout()
            CATransaction.begin()
            CATransaction.setDisableActions(true)
            defer { CATransaction.commit() }
            veil.frame = bounds
            let w = bounds.width
            guard w > 0 else { return }
            // The wall is half again as wide as the window, so that its
            // tilted edges stay outside it.
            let width = w * 1.5
            let gap = width * 0.01
            let posterWidth = (width - gap * CGFloat(Self.columnCount - 1)) / CGFloat(Self.columnCount)
            let posterHeight = posterWidth * 3 / 2
            let height = posterHeight * CGFloat(Self.rowCount) + gap * CGFloat(Self.rowCount - 1)
            wall.transform = CATransform3DIdentity
            wall.bounds = CGRect(x: 0, y: 0, width: width, height: height)
            wall.position = CGPoint(x: w / 2, y: -w * 0.25 + height / 2)
            var transform = CATransform3DMakeRotation(-8 * .pi / 180, 0, 0, 1)
            transform = CATransform3DScale(transform, 1.15, 1.15, 1)
            wall.transform = transform
            wall.filters = [CIFilter(name: "CIGaussianBlur", parameters: [kCIInputRadiusKey: 2])].compactMap(\.self)
            for (c, column) in columns.enumerated() {
                column.frame = CGRect(
                    x: CGFloat(c) * (posterWidth + gap), y: c % 2 == 1 ? -width * 0.06 : 0, width: posterWidth,
                    height: height)
                for (r, poster) in posters[c].enumerated() {
                    poster.frame = CGRect(
                        x: 0, y: CGFloat(r) * (posterHeight + gap), width: posterWidth, height: posterHeight)
                    if let shine = poster.sublayers?.first as? CAGradientLayer {
                        shine.frame = poster.bounds
                        shine.startPoint = CGPoint(x: 0.25, y: 0.12)
                        shine.endPoint = CGPoint(x: 0.25 + 90 / posterWidth, y: 0.12 + 90 / posterHeight)
                    }
                }
            }
            if width != laidOutWidth {
                laidOutWidth = width
                drift(by: width * 0.12)
            }
        }

        /// Drifts the columns up and down, the even ones against the odd,
        /// over a minute and a half each way.
        private func drift(by distance: CGFloat) {
            for column in columns { column.removeAnimation(forKey: "drift") }
            guard !NSWorkspace.shared.accessibilityDisplayShouldReduceMotion else { return }
            for (c, column) in columns.enumerated() {
                let animation = CABasicAnimation(keyPath: "transform.translation.y")
                animation.fromValue = 0
                animation.toValue = (c % 2 == 0 ? -1 : 1) * distance
                animation.duration = 90
                animation.autoreverses = true
                animation.repeatCount = .infinity
                animation.timingFunction = CAMediaTimingFunction(name: .easeInEaseOut)
                column.add(animation, forKey: "drift")
            }
        }

        override public func hitTest(_ point: NSPoint) -> NSView? { nil }
    }
#endif
