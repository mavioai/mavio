#if os(macOS)
    import AppKit
    import MavioMac

    /// A dialog over a page: its title, its content scrolling under it, and
    /// its buttons at its foot, the leading ones apart from the trailing.
    /// On the desktop it is a sheet of the window, Escape closing it; in
    /// television mode a panel over the page dimmed behind it, keeping focus
    /// within it, back closing it and focus returning to what opened it
    /// (docs/tv.md §4).
    @MainActor
    final class Dialog {
        /// The theme it draws with.
        let theme: Theme
        /// What closing it does, after it closed.
        var onClose: (@MainActor () -> Void)?
        private let titleLabel: NSTextField
        private let content = NSStackView.vertical([], spacing: MavioSpacing.s16)
        private let footer = NSStackView.horizontal([], spacing: MavioSpacing.s8)
        private let panel: Box
        private let scroll = NSScrollView()
        private let width: CGFloat
        private let fixedHeight: CGFloat?
        private var maxHeight: NSLayoutConstraint?
        private var sheet: DialogController?
        private var overlay: DialogOverlay?
        private weak var opener: NSView?
        private weak var page: NSViewController?
        /// Itself while shown, as nothing else need keep it.
        private var shown: Dialog?

        /// A dialog titled title, width wide; height tall when given, else
        /// as tall as its content up to most of the window's.
        init(title: String, width: CGFloat = 520, height: CGFloat? = nil, theme: Theme) {
            self.theme = theme
            self.width = theme.isTV ? max(width * 1.4, 760) : width
            fixedHeight = height.map { theme.isTV ? $0 * 1.3 : $0 }
            titleLabel = .label(
                title, theme.isTV ? theme.fonts.title2 : theme.fonts.title3, color: theme.colors.textPrimary)
            titleLabel.maximumNumberOfLines = 2
            panel = Box(
                fill: theme.isTV ? theme.colors.surfaceRaised : theme.colors.background, radius: theme.isTV ? 16 : 0)
            build()
        }

        /// The dialog's title.
        var title: String {
            get { titleLabel.stringValue }
            set { titleLabel.stringValue = newValue }
        }

        private func build() {
            let pad: CGFloat = theme.isTV ? MavioSpacing.s40 : MavioSpacing.s24
            let document = FlippedView()
            document.translatesAutoresizingMaskIntoConstraints = false
            document.addSubview(content)
            content.pin(
                to: document,
                insets: NSEdgeInsets(top: MavioSpacing.s4, left: pad, bottom: MavioSpacing.s16, right: pad))
            scroll.documentView = document
            scroll.drawsBackground = false
            scroll.hasVerticalScroller = true
            scroll.autohidesScrollers = true
            scroll.automaticallyAdjustsContentInsets = false
            scroll.translatesAutoresizingMaskIntoConstraints = false
            NSLayoutConstraint.activate([
                document.leadingAnchor.constraint(equalTo: scroll.contentView.leadingAnchor),
                document.trailingAnchor.constraint(equalTo: scroll.contentView.trailingAnchor),
                document.topAnchor.constraint(equalTo: scroll.contentView.topAnchor),
            ])
            for v in [titleLabel, scroll, footer] as [NSView] { panel.addSubview(v) }
            panel.translatesAutoresizingMaskIntoConstraints = false
            NSLayoutConstraint.activate([
                panel.widthAnchor.constraint(equalToConstant: width),
                titleLabel.topAnchor.constraint(equalTo: panel.topAnchor, constant: pad),
                titleLabel.leadingAnchor.constraint(equalTo: panel.leadingAnchor, constant: pad),
                titleLabel.trailingAnchor.constraint(lessThanOrEqualTo: panel.trailingAnchor, constant: -pad),
                scroll.topAnchor.constraint(equalTo: titleLabel.bottomAnchor, constant: MavioSpacing.s16),
                scroll.leadingAnchor.constraint(equalTo: panel.leadingAnchor),
                scroll.trailingAnchor.constraint(equalTo: panel.trailingAnchor),
                footer.topAnchor.constraint(equalTo: scroll.bottomAnchor, constant: MavioSpacing.s12),
                footer.leadingAnchor.constraint(equalTo: panel.leadingAnchor, constant: pad),
                footer.trailingAnchor.constraint(equalTo: panel.trailingAnchor, constant: -pad),
                footer.bottomAnchor.constraint(equalTo: panel.bottomAnchor, constant: -pad),
            ])
            if let fixedHeight {
                panel.heightAnchor.constraint(equalToConstant: fixedHeight).isActive = true
            } else {
                // As tall as the content, up to the most the window gives.
                let fit = scroll.heightAnchor.constraint(equalTo: document.heightAnchor)
                fit.priority = .init(740)
                fit.isActive = true
                let most = scroll.heightAnchor.constraint(lessThanOrEqualToConstant: theme.isTV ? 640 : 560)
                most.isActive = true
                maxHeight = most
            }
        }

        /// Replaces the content, each view as wide as the dialog's column.
        func setContent(_ views: [NSView]) {
            content.setViews([])
            for v in views {
                content.addArrangedSubview(v)
                v.widthAnchor.constraint(equalTo: content.widthAnchor).isActive = true
            }
            FocusEngine.active(for: panel)?.schedule()
        }

        /// Replaces the buttons at the foot: leading at its start, trailing
        /// at its end, the confirming one last.
        func setButtons(leading: [NSView] = [], trailing: [NSView]) {
            footer.setViews(leading + [Spacer()] + trailing)
            FocusEngine.active(for: panel)?.schedule()
        }

        /// Shows the dialog over page.
        func present(over page: NSViewController) {
            self.page = page
            shown = self
            if theme.isTV, let root = page.view.window?.contentView {
                let engine = FocusEngine.active(for: page.view)
                opener = engine?.focused
                let overlay = DialogOverlay(panel: panel) { [weak self] in self?.close() }
                root.addFilling(overlay)
                if let maxHeight { maxHeight.constant = max(320, root.bounds.height * 0.8 - 200) }
                self.overlay = overlay
                engine?.schedule()
            } else {
                let sheet = DialogController(panel: panel) { [weak self] in self?.close() }
                self.sheet = sheet
                page.presentAsSheet(sheet)
            }
        }

        /// Closes the dialog.
        func close() {
            if let sheet {
                self.sheet = nil
                sheet.presentingViewController?.dismiss(sheet)
            }
            if let overlay {
                self.overlay = nil
                let engine = page.flatMap { FocusEngine.active(for: $0.view) }
                overlay.removeFromSuperview()
                if let opener, opener.window != nil { engine?.focus(opener) } else { engine?.schedule() }
            }
            onClose?()
            shown = nil
        }

        /// What a dialog opened from this one presents over: its sheet on
        /// the desktop, its page in television mode.
        var presenter: NSViewController? { sheet ?? page }

        /// Scrolls the content to its top.
        func scrollToTop() {
            scroll.contentView.setBoundsOrigin(.zero)
            scroll.reflectScrolledClipView(scroll.contentView)
        }
    }

    /// The sheet holding a dialog's panel on the desktop.
    @MainActor
    private final class DialogController: NSViewController {
        private let panel: NSView
        private let cancel: @MainActor () -> Void

        /// A sheet of panel, whose Escape does cancel.
        init(panel: NSView, cancel: @escaping @MainActor () -> Void) {
            self.panel = panel
            self.cancel = cancel
            super.init(nibName: nil, bundle: nil)
        }

        @available(*, unavailable)
        required init?(coder: NSCoder) { fatalError() }

        override func loadView() {
            let root = NSView()
            root.addFilling(panel)
            root.layoutSubtreeIfNeeded()
            root.setFrameSize(root.fittingSize)
            view = root
        }

        override func cancelOperation(_ sender: Any?) { cancel() }
    }

    /// The panel of a dialog over a page dimmed behind it, in television
    /// mode: a scope of focus whose back closes it.
    @MainActor
    private final class DialogOverlay: FlippedView, FocusScope, KeyHandling {
        private let cancel: @MainActor () -> Void

        /// The panel over a scrim, whose back does cancel.
        init(panel: Box, cancel: @escaping @MainActor () -> Void) {
            self.cancel = cancel
            super.init(frame: .zero)
            translatesAutoresizingMaskIntoConstraints = false
            let scrim = Box(fill: NSColor.black.withAlphaComponent(0.6))
            addFilling(scrim)
            panel.elevation = MavioNSElevation.level3
            addSubview(panel)
            panel.center(in: self)
            panel.heightAnchor.constraint(lessThanOrEqualTo: heightAnchor, constant: -MavioSpacing.s80).isActive = true
        }

        @available(*, unavailable)
        required init?(coder: NSCoder) { fatalError() }

        /// Closes the dialog on back.
        func handleKey(_ event: NSEvent) -> Bool {
            guard event.keyCode == 53 || event.keyCode == 51 else { return false }
            cancel()
            return true
        }

        // The page behind takes no clicks.
        override func mouseDown(with event: NSEvent) {}
        override func scrollWheel(with event: NSEvent) {}
    }
#endif
