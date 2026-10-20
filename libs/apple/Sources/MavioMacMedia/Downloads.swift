#if os(macOS)
    import AppKit
    import MavioMac

    /// The items downloaded for offline playback, with the downloads going
    /// on: each plays its file, and its menu removes it or resumes it.
    final class DownloadsPage: Page {
        private let page = ScrollPage([], spacing: 0, insets: .symmetric(vertical: MavioSpacing.s16))
        private var observer: Observer?
        private var shown: [DownloadEntry]?

        override func viewDidLoad() {
            super.viewDidLoad()
            title = MavioStrings.navDownloads
            observer = Observer { [weak self] in self?.render() }
        }

        override func viewWillAppear() {
            super.viewWillAppear()
            model.downloads.reload()
        }

        private func render() {
            let downloads = model.downloads
            let entries = downloads.entries
            guard entries != shown else { return }
            shown = entries
            guard !entries.isEmpty else {
                show(
                    StatusView(
                        icon: "downloads", title: MavioStrings.stateEmpty, message: MavioStrings.downloadEmptyMessage,
                        theme: theme))
                return
            }
            // Where downloads are kept, so they are not taken for a cache.
            var views: [NSView] = [
                margined(
                    NSTextField.label(
                        MavioStrings.downloadMessage, theme.fonts.footnote, color: theme.colors.textSecondary, lines: 0),
                    bottom: MavioSpacing.s8)
            ]
            views += entries.map(row)
            if page.superview == nil { show(page) }
            let focused = FocusEngine.active(for: view)?.focused
            let index = focused.flatMap { f in page.stack.arrangedSubviews.firstIndex { f.isDescendant(of: $0) } }
            page.setViews(views)
            if let engine = FocusEngine.active(for: view) {
                if let index, let target = views[min(index, views.count - 1)].subviews.first as? Focusable {
                    engine.focus(target)
                }
                engine.schedule()
            }
        }

        private func row(_ entry: DownloadEntry) -> NSView {
            let poster = ArtworkView(
                entry.card.poster?.url ?? entry.card.thumb?.url, title: entry.card.title, icon: entry.card.kind.icon,
                theme: theme)
            poster.radius = MavioRadius.sm
            poster.size(width: 60, height: 90)
            var text: [NSView] = [
                NSTextField.label(entry.card.title, theme.fonts.headline, color: theme.colors.textPrimary, lines: 2)
            ]
            if let subtitle = entry.card.subtitle {
                text.append(NSTextField.label(subtitle, theme.fonts.footnote, color: theme.colors.textSecondary))
            }
            text.append(NSTextField.label(entry.serverName, theme.fonts.caption, color: theme.colors.textTertiary))
            text.append(state(entry))
            let column = NSStackView.vertical(text, spacing: MavioSpacing.s4)
            let content = NSStackView.horizontal([poster, column, Spacer()], spacing: MavioSpacing.s12)
            let downloads = model.downloads
            let context = context
            let row = Pressable(
                content, look: .highlight, radius: MavioRadius.md,
                insets: .symmetric(horizontal: MavioSpacing.s8, vertical: 6),
                theme: theme
            ) {
                guard let file = downloads.file(entry) else { return }
                context.play(
                    PlayRequest(
                        serverId: entry.card.serverId, itemId: entry.card.itemId, title: entry.card.title,
                        start: entry.card.progress.flatMap { p in entry.card.runtime.map { $0 * Double(p) } },
                        file: file))
            }
            row.contextMenu = {
                let menu = NSMenu()
                if entry.state == .paused || entry.state == .failed {
                    menu.addItem(
                        MenuAction(MavioStrings.downloadResume) { Task { try? await downloads.resume(entry) } })
                }
                menu.addItem(MenuAction(MavioStrings.itemRemoveDownload) { Task { await downloads.remove(entry) } })
                return menu
            }
            row.setAccessibilityLabel(entry.card.title)
            return inset(row, .symmetric(horizontal: theme.layout.margin - MavioSpacing.s8))
        }

        private func state(_ entry: DownloadEntry) -> NSView {
            let size = { (n: UInt64) in ByteCountFormatter.string(fromByteCount: Int64(n), countStyle: .file) }
            let caption = { (text: String, color: NSColor) in
                NSTextField.label(text, self.theme.fonts.caption, color: color)
            }
            switch entry.state {
            case .queued:
                return caption(MavioStrings.downloadWaiting, theme.colors.textSecondary)
            case .running:
                let bar = NSProgressIndicator()
                bar.style = .bar
                bar.isIndeterminate = false
                bar.minValue = 0
                bar.maxValue = 1
                bar.doubleValue = entry.total > 0 ? Double(entry.received) / Double(entry.total) : 0
                bar.controlSize = .small
                bar.translatesAutoresizingMaskIntoConstraints = false
                bar.size(width: 240)
                return NSStackView.vertical(
                    [
                        bar,
                        caption(
                            MavioStrings.downloadProgress(received: size(entry.received), total: size(entry.total)),
                            theme.colors.textSecondary),
                    ], spacing: 2)
            case .paused:
                return caption(MavioStrings.downloadPaused, theme.colors.textSecondary)
            case .failed:
                return caption(MavioStrings.downloadFailed, theme.colors.danger)
            case .done:
                return caption(size(entry.total), theme.colors.textSecondary)
            }
        }

        isolated deinit { observer?.cancel() }
    }
#endif
