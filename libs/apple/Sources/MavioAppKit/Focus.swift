#if os(macOS)
    import AppKit
    import MavioDesign
    import ObjectiveC

    /// A view that television mode's focus lands on: it shows focus, and
    /// select activates it (docs/tv.md §4).
    @MainActor
    public protocol Focusable: NSView {
        /// Whether focus is on the view, which it shows.
        var isFocused: Bool { get set }
        /// Does what select does: what a click does.
        func activate()
        /// Whether focus lands on the view itself; when not, it lands on
        /// the focusable views within it, as a row's buttons or field.
        var canFocus: Bool { get }
    }

    extension Focusable {
        /// Whether focus may rest on it; true unless a view says otherwise.
        public var canFocus: Bool { true }
    }

    /// A container whose elements are a choice, as the tabs or a library's
    /// filters: focus coming into it from outside lands on its current one.
    @MainActor
    public protocol FocusChoice: NSView {
        /// The element chosen.
        var currentChoice: NSView? { get }
    }

    /// A region with rows of its own, as Search's keyboard beside its
    /// results: left and right cross between regions only at their edges.
    @MainActor
    public protocol FocusRegion: NSView {}

    /// A view over the page, as a menu or a dialog: while one shows, focus
    /// moves only within the topmost.
    @MainActor
    public protocol FocusScope: NSView {}

    /// A view that handles keys itself while focus is within it, as a
    /// keyboard typing or the PIN pad deleting a digit on back.
    @MainActor
    public protocol KeyHandling: NSView {
        /// Handles a key, returning whether it did.
        func handleKey(_ event: NSEvent) -> Bool
    }

    /// A direction of the D-pad.
    public enum FocusDirection: Sendable {
        case up, down, left, right
    }

    /// Television mode's focus in a window: the arrow keys move it to the
    /// nearest element by rows, Return activates it, Escape and Backspace
    /// go back; every page has one element with focus, and the page scrolls
    /// to keep it in view (docs/tv.md §4).
    @MainActor
    public final class FocusEngine {
        private weak var window: NSWindow?
        private var monitor: Any?
        private var pending = false
        /// The element with focus.
        public private(set) weak var focused: NSView?
        /// Where the element with focus was, to find the nearest when it goes.
        private var lastFrame: CGRect = .zero
        /// What back does when nothing focused handles it: returns whether
        /// it went back.
        public var onBack: (@MainActor () -> Bool)?
        /// What focus does when it moves up from the page's first row, as
        /// going to the tabs.
        public var onEdge: (@MainActor (FocusDirection) -> Bool)?
        /// Elements focus prefers when a page opens, as a details page's
        /// primary action.
        private var preferred: [ObjectIdentifier: WeakView] = [:]

        private struct WeakView {
            weak var view: NSView?
        }

        /// The engine of window, made the first time.
        public static func of(_ window: NSWindow) -> FocusEngine {
            if let engine = objc_getAssociatedObject(window, &engineKey) as? FocusEngine { return engine }
            let engine = FocusEngine(window: window)
            objc_setAssociatedObject(window, &engineKey, engine, .OBJC_ASSOCIATION_RETAIN_NONATOMIC)
            return engine
        }

        /// The engine of the window view is in, if television mode runs it.
        public static func active(for view: NSView) -> FocusEngine? {
            guard let window = view.window,
                let engine = objc_getAssociatedObject(window, &engineKey) as? FocusEngine, engine.monitor != nil
            else { return nil }
            return engine
        }

        private init(window: NSWindow) {
            self.window = window
        }

        /// Starts handling the keys of the window.
        public func start() {
            guard monitor == nil else { return }
            monitor = NSEvent.addLocalMonitorForEvents(matching: .keyDown) { [weak self] event in
                guard let self, let window = self.window, event.window === window else { return event }
                return self.handle(event) ? nil : event
            }
            ensureFocus()
        }

        /// Stops, taking focus away.
        public func stop() {
            if let monitor { NSEvent.removeMonitor(monitor) }
            monitor = nil
            setFocus(nil)
        }

        /// Whether the engine handles the window's keys.
        public var isRunning: Bool { monitor != nil }

        // MARK: Keys

        private func handle(_ event: NSEvent) -> Bool {
            // Shortcuts with ⌘ or ⌃ are the menu bar's.
            if !event.modifierFlags.intersection([.command, .control, .option]).isEmpty { return false }
            ensureNow()
            // A field being typed in keeps its keys, but Escape.
            if let editor = window?.firstResponder as? NSTextView, editor.isFieldEditor || editor.isEditable {
                if event.keyCode == 53 {
                    window?.makeFirstResponder(nil)
                    return true
                }
                return false
            }
            var view = focused
            while let v = view {
                if let handler = v as? KeyHandling, handler.handleKey(event) { return true }
                view = v.superview
            }
            switch event.keyCode {
            case 123: move(.left)
            case 124: move(.right)
            case 125: move(.down)
            case 126: move(.up)
            case 36, 76, 49:
                if let field = focused as? NSTextField {
                    TVKeyboard.present(for: field)
                } else {
                    (focused as? Focusable)?.activate()
                }
            case 53, 51:
                if onBack?() != true { NSSound.beep() }
            default:
                // A computer's keyboard types into a focused field directly.
                if let field = focused as? NSTextField, let chars = event.characters, !chars.isEmpty {
                    window?.makeFirstResponder(field)
                }
                return false
            }
            return true
        }

        // MARK: Focus

        /// Puts focus on view.
        public func focus(_ view: NSView?) {
            setFocus(view)
            if let view { scrollIntoView(view) }
        }

        /// Makes view the element focus goes to when its page opens, or when
        /// focus has nowhere else to be.
        public func prefer(_ view: NSView) {
            preferred[ObjectIdentifier(view)] = WeakView(view: view)
            schedule()
        }

        /// Checks soon that an element has focus, after views come and go.
        public func schedule() {
            guard !pending else { return }
            pending = true
            DispatchQueue.main.async { [weak self] in
                self?.ensureNow()
            }
        }

        /// Puts focus on the preferred element, else the first one, when no
        /// element has it.
        public func ensureFocus() {
            schedule()
        }

        private func ensureNow() {
            pending = false
            guard isRunning else { return }
            let all = candidates()
            if let focused, all.contains(where: { $0 === focused }) {
                lastFrame = frame(focused)
                return
            }
            let preferredShown = preferred.values.compactMap(\.view).filter { v in all.contains { $0 === v } }
            preferred = preferred.filter { $0.value.view != nil }
            if let first = preferredShown.first {
                focus(first)
            } else if focused != nil || lastFrame != .zero, let nearest = nearest(to: lastFrame, in: all) {
                focus(nearest)
            } else {
                focus(firstElement(all))
            }
        }

        private func setFocus(_ view: NSView?) {
            if let old = focused as? Focusable, old !== view { old.isFocused = false }
            if let old = focused as? NSTextField, old !== view { Self.ring(old, false) }
            focused = view
            if let view {
                (view as? Focusable)?.isFocused = true
                if let field = view as? NSTextField { Self.ring(field, true) }
                lastFrame = frame(view)
            }
        }

        /// A text field with focus gets a white ring of 3 pt (§4).
        private static func ring(_ field: NSTextField, _ on: Bool) {
            field.wantsLayer = true
            field.layer?.cornerRadius = 8
            field.layer?.borderWidth = on ? 3 : 0
            field.layer?.borderColor = NSColor.white.cgColor
        }

        private func firstElement(_ all: [NSView]) -> NSView? {
            all.min { a, b in
                let fa = frame(a)
                let fb = frame(b)
                return abs(fa.minY - fb.minY) > 4 ? fa.minY < fb.minY : fa.minX < fb.minX
            }
        }

        private func nearest(to rect: CGRect, in all: [NSView]) -> NSView? {
            all.min { distance(frame($0), rect) < distance(frame($1), rect) }
        }

        private func distance(_ a: CGRect, _ b: CGRect) -> CGFloat {
            hypot(a.midX - b.midX, a.midY - b.midY)
        }

        /// The elements focus may move to: the shown, enabled focusable
        /// views of the topmost scope, else of the window.
        private func candidates() -> [NSView] {
            guard let root = window?.contentView else { return [] }
            var scopes: [NSView] = []
            collectScopes(root, into: &scopes)
            let base = scopes.last ?? root
            var found: [NSView] = []
            collect(base, into: &found)
            return found
        }

        private func collectScopes(_ view: NSView, into scopes: inout [NSView]) {
            guard !view.isHidden, view.alphaValue > 0.01 else { return }
            if view is FocusScope { scopes.append(view) }
            for sub in view.subviews { collectScopes(sub, into: &scopes) }
        }

        private func collect(_ view: NSView, into found: inout [NSView]) {
            guard !view.isHidden, view.alphaValue > 0.01 else { return }
            let field = (view as? NSTextField).map { $0.isEditable }
            if (view as? Focusable)?.canFocus ?? (field == true), (view as? NSControl)?.isEnabled ?? true,
                !view.bounds.isEmpty
            {
                found.append(view)
                return
            }
            for sub in view.subviews { collect(sub, into: &found) }
        }

        /// The view's frame in the window, flipped so that y grows down.
        private func frame(_ view: NSView) -> CGRect {
            guard let content = window?.contentView else { return .zero }
            let r = view.convert(view.bounds, to: content)
            return content.isFlipped
                ? r : CGRect(x: r.minX, y: content.bounds.height - r.maxY, width: r.width, height: r.height)
        }

        /// Whether the view is within the window's visible part of the
        /// scroll views it lies in.
        private func inView(_ view: NSView) -> Bool {
            var rect = view.bounds
            var v: NSView = view
            while let scroll = v.enclosingScrollView {
                let inClip = view.convert(rect, to: scroll.contentView)
                if !scroll.contentView.bounds.intersects(inClip) { return false }
                rect = view.convert(view.bounds, to: scroll)
                v = scroll
            }
            return true
        }

        private func region(of view: NSView) -> NSView? {
            var v = view.superview
            while let s = v {
                if s is FocusRegion { return s }
                v = s.superview
            }
            return nil
        }

        private func choice(of view: NSView) -> FocusChoice? {
            var v = view.superview
            while let s = v {
                if let c = s as? FocusChoice { return c }
                v = s.superview
            }
            return nil
        }

        /// Moves focus in a direction by rows (§4), shaking the focused
        /// element where it cannot move.
        public func move(_ direction: FocusDirection) {
            let all = candidates()
            guard let current = focused, all.contains(where: { $0 === current }) else {
                ensureNow()
                return
            }
            let target = next(from: current, direction, all)
            guard var target else {
                if onEdge?(direction) == true { return }
                shake(current)
                return
            }
            // A choice is entered at its current element.
            if let c = choice(of: target), choice(of: current) !== c, let chosen = c.currentChoice,
                all.contains(where: { $0 === chosen })
            {
                target = chosen
            }
            focus(target)
        }

        private func next(from current: NSView, _ direction: FocusDirection, _ all: [NSView]) -> NSView? {
            let here = region(of: current)
            let mine = all.filter { region(of: $0) === here }
            let rows = Self.rows(mine.map { ($0, frame($0)) })
            guard let rowIndex = rows.firstIndex(where: { $0.contains { $0.0 === current } }) else { return nil }
            let row = rows[rowIndex]
            let f = frame(current)
            switch direction {
            case .left, .right:
                let index = row.firstIndex { $0.0 === current }!
                let step = direction == .right ? 1 : -1
                if row.indices.contains(index + step) { return row[index + step].0 }
                // Another region beside this one, at its edge.
                if let across = acrossRegions(from: f, direction, all.filter { region(of: $0) !== here }) {
                    return across
                }
                // Past a row's end, the next row's first in view; past its
                // start, the previous row's last in view.
                let other = rowIndex + step
                guard rows.indices.contains(other) else { return nil }
                let shown = rows[other].filter { inView($0.0) }
                let pool = shown.isEmpty ? rows[other] : shown
                return direction == .right ? pool.first?.0 : pool.last?.0
            case .up, .down:
                let other = rowIndex + (direction == .down ? 1 : -1)
                guard rows.indices.contains(other) else {
                    // Leaving the region upward or downward.
                    let outside = all.filter { region(of: $0) !== here }
                    return vertical(from: f, direction, outside)
                }
                let shown = rows[other].filter { inView($0.0) }
                let pool = shown.isEmpty ? rows[other] : shown
                return pool.min { abs($0.1.midX - f.midX) < abs($1.1.midX - f.midX) }?.0
            }
        }

        private func acrossRegions(from f: CGRect, _ direction: FocusDirection, _ others: [NSView]) -> NSView? {
            let side = others.map { ($0, frame($0)) }.filter { _, r in
                direction == .right ? r.minX >= f.maxX - 1 : r.maxX <= f.minX + 1
            }
            return side.min { a, b in
                let da = abs(a.1.midY - f.midY) + abs(a.1.midX - f.midX) * 0.5
                let db = abs(b.1.midY - f.midY) + abs(b.1.midX - f.midX) * 0.5
                return da < db
            }?.0
        }

        private func vertical(from f: CGRect, _ direction: FocusDirection, _ others: [NSView]) -> NSView? {
            let side = others.map { ($0, frame($0)) }.filter { _, r in
                direction == .down ? r.minY >= f.maxY - 1 : r.maxY <= f.minY + 1
            }
            guard !side.isEmpty else { return nil }
            let rows = Self.rows(side)
            let row = direction == .down ? rows.first! : rows.last!
            return row.min { abs($0.1.midX - f.midX) < abs($1.1.midX - f.midX) }?.0
        }

        /// The elements by rows, top to bottom, each left to right: two are
        /// in one row when they overlap vertically by half the smaller one's
        /// height.
        static func rows(_ items: [(NSView, CGRect)]) -> [[(NSView, CGRect)]] {
            var rows: [[(NSView, CGRect)]] = []
            for item in items.sorted(by: { $0.1.minY < $1.1.minY }) {
                if let i = rows.firstIndex(where: { row in
                    row.contains { other in
                        let overlap = min(other.1.maxY, item.1.maxY) - max(other.1.minY, item.1.minY)
                        return overlap >= min(other.1.height, item.1.height) / 2
                    }
                }) {
                    rows[i].append(item)
                } else {
                    rows.append([item])
                }
            }
            return rows.map { $0.sorted { $0.1.minX < $1.1.minX } }.sorted { a, b in
                (a.map(\.1.minY).min() ?? 0) < (b.map(\.1.minY).min() ?? 0)
            }
        }

        private func shake(_ view: NSView) {
            guard let layer = view.layer else { return }
            let animation = CAKeyframeAnimation(keyPath: "transform.translation.x")
            animation.values = [0, -8, 8, -6, 6, -3, 3, 0]
            animation.duration = 0.32
            layer.add(animation, forKey: "shake")
        }

        // MARK: Scrolling

        /// Scrolls the scroll views view lies in to keep it in view: 200 pt
        /// below the top and 120 above the bottom vertically, the top shown
        /// for an element within the first screen; the margin sideways.
        public func scrollIntoView(_ view: NSView) {
            var inner: NSView = view
            while let scroll = inner.enclosingScrollView {
                guard let document = scroll.documentView else { break }
                let clip = scroll.contentView
                let rect = view.convert(view.bounds, to: document)
                var origin = clip.bounds.origin
                let visible = clip.bounds.size
                let docSize = document.frame.size
                let flipped = document.isFlipped
                if scroll.hasHorizontalScroller || docSize.width > visible.width + 1 {
                    let margin: CGFloat = 80
                    if rect.minX - margin < origin.x {
                        origin.x = max(0, rect.minX - margin)
                    } else if rect.maxX + margin > origin.x + visible.width {
                        origin.x = min(docSize.width - visible.width, rect.maxX + margin - visible.width)
                    }
                }
                if docSize.height > visible.height + 1 {
                    // In a flipped document y grows down, as the window's.
                    let top = flipped ? rect.minY : docSize.height - rect.maxY
                    let bottom = flipped ? rect.maxY : docSize.height - rect.minY
                    var y = flipped ? origin.y : docSize.height - origin.y - visible.height
                    if bottom < visible.height - 120 {
                        y = 0
                    } else if top - 200 < y {
                        y = max(0, top - 200)
                    } else if bottom + 120 > y + visible.height {
                        y = min(docSize.height - visible.height, bottom + 120 - visible.height)
                    }
                    origin.y = flipped ? y : docSize.height - y - visible.height
                }
                if origin != clip.bounds.origin {
                    NSAnimationContext.runAnimationGroup { context in
                        context.duration = MavioDuration.normal
                        context.timingFunction = MavioEasings.standard.timingFunction
                        clip.animator().setBoundsOrigin(origin)
                    }
                    scroll.reflectScrolledClipView(clip)
                }
                inner = scroll
            }
        }
    }

    nonisolated(unsafe) private var engineKey: UInt8 = 0

    extension NSView {
        /// Tells television mode's focus that a focusable view comes or
        /// goes: called as it moves to or from a window.
        public func focusWillMove(to newWindow: NSWindow?) {
            focusMayHaveChanged()
            if let newWindow, let engine = objc_getAssociatedObject(newWindow, &engineKey) as? FocusEngine {
                engine.schedule()
            }
        }

        /// Tells television mode's focus that a focusable view came or went.
        public func focusMayHaveChanged() {
            if let window, let engine = objc_getAssociatedObject(window, &engineKey) as? FocusEngine {
                engine.schedule()
            }
        }
    }
#endif
