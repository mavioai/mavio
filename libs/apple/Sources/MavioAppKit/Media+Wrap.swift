#if os(macOS)
    import AppKit
    import MavioDesign

    /// Views side by side at their own sizes, wrapping onto the next line as
    /// the width ends, each line's views centered on it: the facts and
    /// badges of a title, its tags, a library's filters.
    public final class WrapStack: FlippedView {
        private let spacing: CGFloat
        private let lineSpacing: CGFloat
        private var items: [NSView] = []
        private var height: CGFloat = 0

        /// Views spacing apart, lines lineSpacing apart.
        public init(_ views: [NSView] = [], spacing: CGFloat, lineSpacing: CGFloat) {
            self.spacing = spacing
            self.lineSpacing = lineSpacing
            super.init(frame: .zero)
            translatesAutoresizingMaskIntoConstraints = false
            setContentHuggingPriority(.defaultHigh, for: .vertical)
            setViews(views)
        }

        /// Unavailable: the view is made in code.
        @available(*, unavailable)
        public required init?(coder: NSCoder) { fatalError() }

        /// The views, in order.
        public var views: [NSView] { items }

        /// Replaces the views.
        public func setViews(_ views: [NSView]) {
            items.forEach { $0.removeFromSuperview() }
            items = views
            for v in views {
                v.translatesAutoresizingMaskIntoConstraints = true
                addSubview(v)
            }
            needsLayout = true
            invalidateIntrinsicContentSize()
        }

        override public var intrinsicContentSize: NSSize {
            NSSize(width: NSView.noIntrinsicMetric, height: height)
        }

        override public func layout() {
            super.layout()
            let width = bounds.width
            let shown = items.filter { !$0.isHidden }
            guard width > 0, !shown.isEmpty else {
                setHeight(0)
                return
            }
            var lines: [[(NSView, NSSize)]] = [[]]
            var x: CGFloat = 0
            for v in shown {
                var size = v.fittingSize
                size.width = min(ceil(size.width), width)
                size.height = ceil(size.height)
                if x > 0, x + size.width > width {
                    lines.append([])
                    x = 0
                }
                lines[lines.count - 1].append((v, size))
                x += size.width + spacing
            }
            var y: CGFloat = 0
            for line in lines where !line.isEmpty {
                let lineHeight = line.map(\.1.height).max() ?? 0
                var x: CGFloat = 0
                for (v, size) in line {
                    v.frame = NSRect(
                        x: x, y: y + (lineHeight - size.height) / 2, width: size.width, height: size.height)
                    x += size.width + spacing
                }
                y += lineHeight + lineSpacing
            }
            setHeight(y - lineSpacing)
        }

        private func setHeight(_ h: CGFloat) {
            guard h != height else { return }
            height = h
            invalidateIntrinsicContentSize()
            // Grids that lay out by frames, as FlowGrid, measure again.
            var ancestor = superview
            while let view = ancestor {
                view.needsLayout = true
                ancestor = view.superview
            }
        }
    }
#endif
