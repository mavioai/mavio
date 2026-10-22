#if os(macOS)
    import AppKit
    import MavioDesign

    /// A scroll view that scrolls sideways only, passing vertical scrolling
    /// on to the page around it.
    public final class HorizontalScrollView: NSScrollView {
        // No scroller, as the web UI's shelves: the collection view in it
        // turns one on for its horizontal layout, so it stays off.
        override public var hasHorizontalScroller: Bool {
            get { false }
            set { super.hasHorizontalScroller = false }
        }

        override public func scrollWheel(with event: NSEvent) {
            if abs(event.scrollingDeltaY) > abs(event.scrollingDeltaX) {
                nextResponder?.scrollWheel(with: event)
            } else {
                super.scrollWheel(with: event)
            }
        }
    }

    /// An item of a collection hosting a view made for its index.
    final class HostItem: NSCollectionViewItem {
        static let identifier = NSUserInterfaceItemIdentifier("HostItem")

        override func loadView() {
            view = FlippedView()
        }

        /// Shows content, in place of what the item showed.
        func host(_ content: NSView) {
            view.subviews.forEach { $0.removeFromSuperview() }
            view.addSubview(content)
            content.translatesAutoresizingMaskIntoConstraints = false
            NSLayoutConstraint.activate([
                content.leadingAnchor.constraint(equalTo: view.leadingAnchor),
                content.topAnchor.constraint(equalTo: view.topAnchor),
                content.trailingAnchor.constraint(equalTo: view.trailingAnchor),
            ])
        }
    }

    /// A row of cards scrolling sideways, made as they come into view: the
    /// web UI's shelves, with arrows on the desktop and none in television
    /// mode, where focus scrolls it (docs/tv.md §4).
    public final class CardRow: NSView, NSCollectionViewDataSource {
        private let collection = NSCollectionView()
        private let scroll = HorizontalScrollView()
        private let flow = NSCollectionViewFlowLayout()
        private let theme: Theme
        private var count = 0
        private var make: (@MainActor (Int) -> NSView)?
        private var made: [Int: NSView] = [:]
        private var heightConstraint: NSLayoutConstraint!
        /// Room above and below the cards for one to grow with focus.
        private var room: CGFloat { theme.isTV ? 24 : 0 }

        /// A row in theme, its cards inset by the layout's margin.
        public init(theme: Theme) {
            self.theme = theme
            super.init(frame: .zero)
            translatesAutoresizingMaskIntoConstraints = false
            flow.scrollDirection = .horizontal
            flow.minimumLineSpacing = theme.layout.gutter
            flow.minimumInteritemSpacing = theme.layout.gutter
            flow.sectionInset = NSEdgeInsets(
                top: room, left: theme.layout.margin, bottom: room, right: theme.layout.margin)
            collection.collectionViewLayout = flow
            collection.dataSource = self
            collection.backgroundColors = [.clear]
            collection.isSelectable = false
            collection.register(HostItem.self, forItemWithIdentifier: HostItem.identifier)
            scroll.documentView = collection
            scroll.drawsBackground = false
            scroll.hasHorizontalScroller = false
            scroll.hasVerticalScroller = false
            scroll.verticalScrollElasticity = .none
            scroll.horizontalScrollElasticity = .allowed
            scroll.automaticallyAdjustsContentInsets = false
            addFilling(scroll)
            heightConstraint = heightAnchor.constraint(equalToConstant: 0)
            heightConstraint.isActive = true
        }

        /// Unavailable: the view is made in code.
        @available(*, unavailable)
        public required init?(coder: NSCoder) { fatalError() }

        /// Shows count cards, each made by make, all as large as the first.
        public func show(count: Int, make: @escaping @MainActor (Int) -> NSView) {
            self.count = count
            self.make = make
            made = [:]
            if count > 0, let sample = card(0) {
                let size = sample.fittingSize
                flow.itemSize = NSSize(width: ceil(size.width), height: ceil(size.height))
                heightConstraint.constant = ceil(size.height) + room * 2
            } else {
                heightConstraint.constant = 0
            }
            collection.reloadData()
        }

        /// Scrolls a page sideways, as the desktop's arrows do.
        public func page(forward: Bool) {
            let clip = scroll.contentView
            let width = clip.bounds.width - theme.layout.margin * 2
            let maxX = max(0, collection.frame.width - clip.bounds.width)
            let x = min(max(0, clip.bounds.origin.x + (forward ? width : -width)), maxX)
            NSAnimationContext.runAnimationGroup { context in
                context.duration = MavioDuration.normal
                clip.animator().setBoundsOrigin(NSPoint(x: x, y: 0))
            }
        }

        /// Returns the row to its start.
        public func scrollToStart() {
            scroll.contentView.setBoundsOrigin(.zero)
            scroll.reflectScrolledClipView(scroll.contentView)
        }

        /// The number of items in a section.
        public func collectionView(_ collectionView: NSCollectionView, numberOfItemsInSection section: Int) -> Int {
            count
        }

        /// The item at an index path.
        public func collectionView(
            _ collectionView: NSCollectionView, itemForRepresentedObjectAt indexPath: IndexPath
        ) -> NSCollectionViewItem {
            let item = collectionView.makeItem(withIdentifier: HostItem.identifier, for: indexPath)
            if let host = item as? HostItem, let view = card(indexPath.item) { host.host(view) }
            return item
        }

        /// The card at index, made once and kept while the row shows the
        /// same cards, so that scrolling back does not make it again.
        private func card(_ index: Int) -> NSView? {
            if let view = made[index] { return view }
            guard let make else { return nil }
            let view = make(index)
            made[index] = view
            return view
        }
    }

    /// A titled row of cards: the title, an accessory beside it and, on the
    /// desktop, arrows paging it at its end.
    public final class Shelf: NSView {
        /// The row of cards.
        public let row: CardRow

        /// A shelf titled title, with accessory after the title.
        public init(_ title: String, accessory: NSView? = nil, theme: Theme) {
            row = CardRow(theme: theme)
            super.init(frame: .zero)
            translatesAutoresizingMaskIntoConstraints = false
            let label = NSTextField.label(title, theme.fonts.title3, color: theme.colors.textPrimary)
            var header: [NSView] = [label]
            if let accessory { header.append(accessory) }
            header.append(Spacer())
            if !theme.isTV {
                let back = MavioButton(icon: "back", kind: .circle, theme: theme) { [weak self] in
                    self?.row.page(forward: false)
                }
                let forward = MavioButton(icon: "forward", kind: .circle, theme: theme) { [weak self] in
                    self?.row.page(forward: true)
                }
                header += [back, forward]
            }
            let top = NSStackView.horizontal(header, spacing: MavioSpacing.s8)
            addSubview(top)
            addSubview(row)
            NSLayoutConstraint.activate([
                top.topAnchor.constraint(equalTo: topAnchor, constant: MavioSpacing.s8),
                top.leadingAnchor.constraint(equalTo: leadingAnchor, constant: theme.layout.margin),
                top.trailingAnchor.constraint(equalTo: trailingAnchor, constant: -theme.layout.margin),
                row.topAnchor.constraint(equalTo: top.bottomAnchor, constant: MavioSpacing.s8),
                row.leadingAnchor.constraint(equalTo: leadingAnchor),
                row.trailingAnchor.constraint(equalTo: trailingAnchor),
                row.bottomAnchor.constraint(equalTo: bottomAnchor, constant: -MavioSpacing.s8),
            ])
        }

        /// Unavailable: the view is made in code.
        @available(*, unavailable)
        public required init?(coder: NSCoder) { fatalError() }
    }

    /// Views in columns at least minimum wide, gutter apart, filling the
    /// width: a grid that lays out every view it holds, for the sets a
    /// page shows whole, as search results or a person's titles.
    public final class FlowGrid: FlippedView {
        private let minimum: CGFloat
        private let gutter: CGFloat
        private let rowSpacing: CGFloat
        private var items: [NSView] = []
        private var height: CGFloat = 0

        /// A grid of columns at least minimum wide.
        public init(minimum: CGFloat, gutter: CGFloat, rowSpacing: CGFloat) {
            self.minimum = minimum
            self.gutter = gutter
            self.rowSpacing = rowSpacing
            super.init(frame: .zero)
            translatesAutoresizingMaskIntoConstraints = false
        }

        /// Unavailable: the view is made in code.
        @available(*, unavailable)
        public required init?(coder: NSCoder) { fatalError() }

        /// Replaces the grid's views.
        public func setItems(_ views: [NSView]) {
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
            guard width > 0, !items.isEmpty else {
                setHeight(0)
                return
            }
            let columns = max(1, Int((width + gutter) / (minimum + gutter)))
            let itemWidth = floor((width - gutter * CGFloat(columns - 1)) / CGFloat(columns))
            var y: CGFloat = 0
            for start in stride(from: 0, to: items.count, by: columns) {
                let row = items[start..<min(start + columns, items.count)]
                var rowHeight: CGFloat = 0
                for (i, v) in row.enumerated() {
                    let h = v.fittingSize(width: itemWidth)
                    v.frame = NSRect(x: CGFloat(i) * (itemWidth + gutter), y: y, width: itemWidth, height: h)
                    rowHeight = max(rowHeight, h)
                }
                y += rowHeight + rowSpacing
            }
            setHeight(y - rowSpacing)
        }

        private func setHeight(_ h: CGFloat) {
            guard h != height else { return }
            height = h
            invalidateIntrinsicContentSize()
        }
    }

    /// Rows one under another that the page around them scrolls, made only
    /// as they come near the page's view and dropped once far from it, as
    /// the web UI's virtual lists: for lists too long to make whole, as a
    /// season's episodes. A row not yet made counts as tall as the rows
    /// made so far are on average; one made keeps the height it measured,
    /// by its key, and the page keeps still as rows above its view change
    /// height.
    public final class LazyRows: FlippedView {
        private let estimate: CGFloat
        private let spacing: CGFloat
        private var keys: [String] = []
        private var make: (@MainActor (Int) -> NSView)?
        private var heights: [String: CGFloat] = [:]
        private var made: [String: NSView] = [:]
        private var width: CGFloat = 0
        private var height: CGFloat = 0
        private var pending = false
        private weak var clip: NSClipView?
        /// How far beyond the page's view rows are made.
        private static let overscan: CGFloat = 600

        /// Rows estimate tall until made, spacing apart.
        public init(estimate: CGFloat, spacing: CGFloat = 0) {
            self.estimate = estimate
            self.spacing = spacing
            super.init(frame: .zero)
            translatesAutoresizingMaskIntoConstraints = false
        }

        /// Unavailable: the view is made in code.
        @available(*, unavailable)
        public required init?(coder: NSCoder) { fatalError() }

        isolated deinit { NotificationCenter.default.removeObserver(self) }

        /// Shows a row for each key, made by make from its index; rows of
        /// keys shown before are made again, keeping their heights.
        public func show(keys: [String], make: @escaping @MainActor (Int) -> NSView) {
            made.values.forEach { $0.removeFromSuperview() }
            made = [:]
            self.keys = keys
            self.make = make
            let kept = Set(keys)
            heights = heights.filter { kept.contains($0.key) }
            update()
        }

        /// Forgets the height of the row of key, measured again as it is
        /// made.
        public func invalidate(_ key: String) { heights[key] = nil }

        override public var intrinsicContentSize: NSSize {
            NSSize(width: NSView.noIntrinsicMetric, height: height)
        }

        override public func viewDidMoveToWindow() {
            super.viewDidMoveToWindow()
            NotificationCenter.default.removeObserver(self, name: NSView.boundsDidChangeNotification, object: nil)
            clip = enclosingScrollView?.contentView
            if let clip {
                clip.postsBoundsChangedNotifications = true
                NotificationCenter.default.addObserver(
                    self, selector: #selector(scrolled), name: NSView.boundsDidChangeNotification, object: clip)
            }
            schedule()
        }

        override public func layout() {
            super.layout()
            if bounds.width != width {
                // Rows wrap anew at another width.
                width = bounds.width
                heights = [:]
                made.values.forEach { $0.removeFromSuperview() }
                made = [:]
            }
            schedule()
        }

        @objc private func scrolled() { update() }

        /// Updates once the layout under way ends.
        private func schedule() {
            guard !pending else { return }
            pending = true
            DispatchQueue.main.async { [weak self] in
                self?.pending = false
                self?.update()
            }
        }

        private func update() {
            guard let make, width > 0 else { return setHeight(totalHeight()) }
            let view = visibleRect
            let near =
                view.isEmpty ? NSRect.zero : view.insetBy(dx: 0, dy: -Self.overscan)
            var y: CGFloat = 0
            var shift: CGFloat = 0
            var keep = Set<String>()
            for (i, key) in keys.enumerated() {
                var h = heights[key] ?? average
                if !near.isEmpty, y + h >= near.minY, y <= near.maxY {
                    keep.insert(key)
                    let row: NSView
                    if let existing = made[key] {
                        row = existing
                    } else {
                        row = make(i)
                        row.translatesAutoresizingMaskIntoConstraints = true
                        addSubview(row)
                        made[key] = row
                    }
                    if heights[key] == nil {
                        let measured = row.fittingSize(width: width)
                        // A row above the view changing height moves what
                        // the view shows: the page scrolls by as much.
                        if y + h <= view.minY { shift += measured - h }
                        heights[key] = measured
                        h = measured
                    }
                    row.frame = NSRect(x: 0, y: y, width: width, height: h)
                }
                y += h + spacing
            }
            for (key, row) in made where !keep.contains(key) {
                row.removeFromSuperview()
                made[key] = nil
            }
            setHeight(keys.isEmpty ? 0 : y - spacing)
            if shift != 0, let clip, let scroll = enclosingScrollView {
                superview?.layoutSubtreeIfNeeded()
                var origin = clip.bounds.origin
                origin.y += clip.isFlipped ? shift : -shift
                clip.setBoundsOrigin(origin)
                scroll.reflectScrolledClipView(clip)
            }
        }

        /// The height of a row not yet measured.
        private var average: CGFloat {
            heights.isEmpty ? estimate : heights.values.reduce(0, +) / CGFloat(heights.count)
        }

        private func totalHeight() -> CGFloat {
            guard !keys.isEmpty else { return 0 }
            let average = average
            return keys.reduce(0) { $0 + (heights[$1] ?? average) } + spacing * CGFloat(keys.count - 1)
        }

        private func setHeight(_ h: CGFloat) {
            guard h != height else { return }
            height = h
            invalidateIntrinsicContentSize()
        }
    }

    extension NSView {
        /// The height the view takes at a width.
        func fittingSize(width: CGFloat) -> CGFloat {
            let constraint = widthAnchor.constraint(equalToConstant: width)
            constraint.priority = .required - 1
            constraint.isActive = true
            let height = fittingSize.height
            constraint.isActive = false
            return ceil(height)
        }
    }

    /// A grid of many cards that is its page: it scrolls, makes its cards as
    /// they come into view, shows a header above them, and asks for more as
    /// the end comes into view, as a library's items.
    public final class CardGrid: NSView, NSCollectionViewDataSource, NSCollectionViewDelegateFlowLayout {
        /// The scroll view, the page's.
        public let scroll = NSScrollView()
        private let collection = NSCollectionView()
        private let flow = NSCollectionViewFlowLayout()
        private let theme: Theme
        private var count = 0
        private var make: (@MainActor (Int) -> NSView)?
        private var itemHeight: (@MainActor (CGFloat) -> CGFloat)?
        private var header: NSView?
        /// Called as the last cards come into view.
        public var onReachEnd: (@MainActor () -> Void)?
        private let wide: Bool

        /// A grid of posters, or of thumbnails when wide.
        public init(wide: Bool = false, theme: Theme) {
            self.theme = theme
            self.wide = wide
            super.init(frame: .zero)
            translatesAutoresizingMaskIntoConstraints = false
            flow.minimumInteritemSpacing = theme.layout.gutter
            flow.minimumLineSpacing = theme.isTV ? MavioSpacing.s48 : MavioSpacing.s24
            flow.sectionInset = NSEdgeInsets(
                top: MavioSpacing.s16, left: theme.layout.margin, bottom: MavioSpacing.s40,
                right: theme.layout.margin)
            collection.collectionViewLayout = flow
            collection.dataSource = self
            collection.delegate = self
            collection.backgroundColors = [.clear]
            collection.isSelectable = false
            collection.register(HostItem.self, forItemWithIdentifier: HostItem.identifier)
            collection.register(
                HeaderView.self, forSupplementaryViewOfKind: NSCollectionView.elementKindSectionHeader,
                withIdentifier: HeaderView.identifier)
            scroll.documentView = collection
            scroll.drawsBackground = false
            scroll.hasVerticalScroller = true
            scroll.autohidesScrollers = true
            addFilling(scroll)
            NotificationCenter.default.addObserver(
                self, selector: #selector(scrolled), name: NSView.boundsDidChangeNotification,
                object: scroll.contentView)
            scroll.contentView.postsBoundsChangedNotifications = true
        }

        /// Unavailable: the view is made in code.
        @available(*, unavailable)
        public required init?(coder: NSCoder) { fatalError() }

        /// Shows a header above the cards.
        public func setHeader(_ view: NSView?) {
            header = view
            collection.collectionViewLayout?.invalidateLayout()
            collection.reloadData()
        }

        /// Shows count cards, each made by make, height tall at a width.
        public func show(
            count: Int, height: @escaping @MainActor (CGFloat) -> CGFloat, make: @escaping @MainActor (Int) -> NSView
        ) {
            self.count = count
            self.make = make
            itemHeight = height
            collection.reloadData()
        }

        /// Returns the grid to its top.
        public func scrollToTop() {
            scroll.contentView.setBoundsOrigin(.zero)
            scroll.reflectScrolledClipView(scroll.contentView)
        }

        @objc private func scrolled() {
            let clip = scroll.contentView.bounds
            if clip.maxY > collection.frame.height - clip.height { onReachEnd?() }
        }

        private var itemWidth: CGFloat {
            let minimum = wide ? theme.layout.thumbnail : theme.layout.poster
            let width = max(0, scroll.contentView.bounds.width - theme.layout.margin * 2)
            let gutter = theme.layout.gutter
            let columns = max(1, Int((width + gutter) / (minimum + gutter)))
            return floor((width - gutter * CGFloat(columns - 1)) / CGFloat(columns)) - 0.5
        }

        override public func layout() {
            super.layout()
            flow.invalidateLayout()
        }

        /// The number of items in a section.
        public func collectionView(_ collectionView: NSCollectionView, numberOfItemsInSection section: Int) -> Int {
            count
        }

        /// The item at an index path.
        public func collectionView(
            _ collectionView: NSCollectionView, itemForRepresentedObjectAt indexPath: IndexPath
        ) -> NSCollectionViewItem {
            let item = collectionView.makeItem(withIdentifier: HostItem.identifier, for: indexPath)
            if let host = item as? HostItem, let make { host.host(make(indexPath.item)) }
            if indexPath.item >= count - 1 { DispatchQueue.main.async { [weak self] in self?.onReachEnd?() } }
            return item
        }

        /// The header of a section.
        public func collectionView(
            _ collectionView: NSCollectionView,
            viewForSupplementaryElementOfKind kind: NSCollectionView.SupplementaryElementKind,
            at indexPath: IndexPath
        ) -> NSView {
            let view = collectionView.makeSupplementaryView(
                ofKind: kind, withIdentifier: HeaderView.identifier, for: indexPath)
            (view as? HeaderView)?.host(header)
            return view
        }

        /// An item's size.
        public func collectionView(
            _ collectionView: NSCollectionView, layout collectionViewLayout: NSCollectionViewLayout,
            sizeForItemAt indexPath: IndexPath
        ) -> NSSize {
            let w = itemWidth
            return NSSize(width: w, height: itemHeight?(w) ?? w * 1.5)
        }

        /// A section header's size.
        public func collectionView(
            _ collectionView: NSCollectionView, layout collectionViewLayout: NSCollectionViewLayout,
            referenceSizeForHeaderInSection section: Int
        ) -> NSSize {
            guard let header else { return .zero }
            let width = scroll.contentView.bounds.width
            return NSSize(width: width, height: header.fittingSize(width: width))
        }
    }

    /// A header of a collection hosting a view.
    final class HeaderView: NSView, NSCollectionViewElement {
        static let identifier = NSUserInterfaceItemIdentifier("HeaderView")

        func host(_ content: NSView?) {
            subviews.forEach { $0.removeFromSuperview() }
            if let content { addFilling(content) }
        }
    }
#endif
