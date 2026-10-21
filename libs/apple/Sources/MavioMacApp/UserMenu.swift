#if os(macOS)
    import AppKit
    import MavioMac
    import MavioMacSettings

    /// The user's menu over the window, as the web UI's popup: the user with
    /// the appearance beside them, the users of a standalone server to
    /// switch to, the settings, television mode and, in it, full screen,
    /// the account at mavio.dev or the servers, and signing out. On the
    /// desktop it opens above its button; in television mode below the menu
    /// button, 400 pt wide in frosted glass (docs/tv.md §5). A click
    /// elsewhere, Escape or back closes it.
    @MainActor
    final class UserMenuOverlay: FlippedView, FocusScope {
        private unowned let root: RootController
        private weak var anchor: NSView?
        private let panel: Box
        private let items = NSStackView.vertical([], spacing: 0)
        private var context: MacContext { root.context }
        private var theme: Theme { root.context.theme }
        private var users: [SwitchableUser] = []
        private var busy = false
        private var loading: Task<Void, Never>?

        init(root: RootController, anchor: NSView) {
            self.root = root
            self.anchor = anchor
            let theme = root.context.theme
            panel = Box(radius: 14)
            super.init(frame: .zero)
            translatesAutoresizingMaskIntoConstraints = false
            let glass = NSVisualEffectView()
            glass.material = theme.isTV ? .fullScreenUI : .popover
            glass.blendingMode = .withinWindow
            glass.state = .active
            glass.wantsLayer = true
            glass.layer?.cornerRadius = 14
            glass.layer?.masksToBounds = true
            panel.addFilling(glass)
            let tint = Box(fill: theme.colors.surfaceOverlay.withAlphaComponent(theme.isTV ? 0.45 : 0.9), radius: 14)
            tint.border = theme.colors.border
            panel.addFilling(tint)
            panel.elevation = MavioNSShadow(
                color: NSColor.black.withAlphaComponent(0.18), x: 0, y: 8, blur: 32, spread: 0)
            panel.addFilling(items, insets: .all(6))
            addSubview(panel)
            panel.size(width: theme.isTV ? 400 : 300)
            (anchor as? UserButton)?.open = true
            (anchor as? TVMenuButton)?.open = true
            build()
            if let server = standalone {
                let model = context.model
                loading = Task { [weak self] in
                    let users = await model.switchableUsers(server)
                    guard let self, !Task.isCancelled else { return }
                    self.users = users
                    self.build()
                }
            }
        }

        @available(*, unavailable)
        required init?(coder: NSCoder) { fatalError() }

        isolated deinit { loading?.cancel() }

        /// Places the menu by its button and shows it.
        func appear() {
            guard let anchor, let superview else { return }
            superview.layoutSubtreeIfNeeded()
            let frame = convert(anchor.bounds, from: anchor)
            if theme.isTV {
                NSLayoutConstraint.activate([
                    panel.topAnchor.constraint(equalTo: topAnchor, constant: frame.maxY + MavioSpacing.s12),
                    panel.trailingAnchor.constraint(
                        equalTo: leadingAnchor, constant: frame.maxX),
                ])
            } else {
                NSLayoutConstraint.activate([
                    panel.bottomAnchor.constraint(equalTo: topAnchor, constant: frame.minY - MavioSpacing.s8),
                    panel.leadingAnchor.constraint(
                        equalTo: leadingAnchor, constant: max(MavioSpacing.s8, frame.minX)),
                ])
            }
            panel.alphaValue = 0
            NSAnimationContext.runAnimationGroup { context in
                context.duration = MavioDuration.instant
                panel.animator().alphaValue = 1
            }
            window?.makeFirstResponder(self)
            if theme.isTV, let engine = FocusEngine.active(for: self) {
                engine.focus(firstItem)
            }
        }

        override func removeFromSuperview() {
            (anchor as? UserButton)?.open = false
            (anchor as? TVMenuButton)?.open = false
            super.removeFromSuperview()
        }

        override var acceptsFirstResponder: Bool { true }

        override func cancelOperation(_ sender: Any?) { root.closeUserMenu() }

        override func mouseDown(with event: NSEvent) {
            let point = convert(event.locationInWindow, from: nil)
            if !panel.frame.contains(point) { root.closeUserMenu() }
        }

        override func hitTest(_ point: NSPoint) -> NSView? {
            super.hitTest(point) ?? self
        }

        /// The server of a standalone sign-in, whose users the menu
        /// switches among.
        private var standalone: ServerRecord? {
            context.model.account == nil ? context.model.signedInServers.first : nil
        }

        private weak var settingsItem: NSView?
        private var firstItem: NSView? { settingsItem }

        private func build() {
            let model = context.model
            let (name, line) = signedIn(model)
            var views: [NSView] = []
            let title = NSTextField.label(name, theme.fonts.headline, color: theme.colors.textPrimary)
            let subtitle = NSTextField.label(
                model.account != nil ? MavioStrings.authMavioAccount : MavioStrings.authStandaloneUser(server: line),
                theme.fonts.footnote, color: theme.colors.textSecondary)
            let names = NSStackView.vertical([title, subtitle], spacing: 0)
            names.setContentHuggingPriority(.init(1), for: .horizontal)
            names.setContentCompressionResistancePriority(.init(1), for: .horizontal)
            let head = NSStackView.horizontal(
                [
                    AvatarView(name: name, size: theme.isTV ? 48 : 36, theme: theme), names,
                    AppearanceSwitch(theme: theme),
                ],
                spacing: MavioSpacing.s12)
            // The name takes the room between the avatar and the switch,
            // which ends the row.
            head.distribution = .fill
            views.append(padded(head, horizontal: 10, vertical: MavioSpacing.s8))
            if users.count > 1 {
                views.append(separator())
                views.append(
                    padded(
                        NSTextField.label(
                            MavioStrings.authSwitchUser, theme.fonts.caption, color: theme.colors.textSecondary),
                        horizontal: 10, vertical: MavioSpacing.s4))
                for user in users {
                    let end: NSView? =
                        user.current
                        ? IconView("check", size: 16, color: theme.colors.accentText)
                        : user.hasPin ? IconView("lock", size: 14, color: theme.colors.textTertiary) : nil
                    let row = MenuRow(
                        lead: AvatarView(name: user.name, size: 24, theme: theme), title: user.name, end: end,
                        theme: theme
                    ) { [weak self] in self?.choose(user) }
                    row.enabled = !user.current && !busy
                    views.append(row)
                }
            }
            views.append(separator())
            let settings = item("settings", MavioStrings.navSettings, shortcut: "⌘,") { [weak self] in
                self?.root.showSettings(account: false)
            }
            settingsItem = settings
            views.append(settings)
            let tv = theme.isTV
            views.append(
                item(
                    tv ? "sidebar" : "television", tv ? MavioStrings.menuStandardMode : MavioStrings.menuTvMode,
                    shortcut: "⌃⌘T"
                ) {
                    [weak self] in
                    self?.root.closeUserMenu()
                    self?.root.app.toggleTVMode()
                })
            if tv {
                let full = root.app.isFullScreen
                views.append(
                    item(
                        full ? "exitFullScreen" : "fullScreen",
                        full ? MavioStrings.menuExitFullScreen : MavioStrings.menuFullScreen, shortcut: "⌃⌘F"
                    ) { [weak self] in
                        self?.root.closeUserMenu()
                        self?.root.app.toggleFullScreen()
                    })
            }
            if let account = model.account {
                if !tv {
                    views.append(
                        item("external", MavioStrings.menuManageAccount) { [weak self] in
                            self?.root.closeUserMenu()
                            if let url = URL(string: Self.site(account.api) + "/account") {
                                NSWorkspace.shared.open(url)
                            }
                        })
                }
            } else {
                views.append(
                    item("server", MavioStrings.menuManageServers) { [weak self] in
                        self?.root.showSettings(account: true)
                    })
            }
            views.append(separator())
            views.append(
                item("signOut", MavioStrings.authSignOut) { [weak self] in
                    self?.root.closeUserMenu()
                    Task { try? await model.signOut() }
                })
            items.setViews(views)
            for view in views { view.widthAnchor.constraint(equalTo: items.widthAnchor).isActive = true }
        }

        private func padded(_ view: NSView, horizontal: CGFloat, vertical: CGFloat) -> NSView {
            let holder = NSView()
            holder.translatesAutoresizingMaskIntoConstraints = false
            holder.addFilling(view, insets: .symmetric(horizontal: horizontal, vertical: vertical))
            return holder
        }

        private func separator() -> NSView {
            let line = Box(fill: theme.colors.border)
            line.size(height: 1)
            return padded(line, horizontal: 10, vertical: MavioSpacing.s4)
        }

        private func item(
            _ icon: String, _ title: String, shortcut: String? = nil, action: @escaping @MainActor () -> Void
        )
            -> MenuRow
        {
            let end = shortcut.flatMap {
                theme.isTV ? nil : NSTextField.label($0, theme.fonts.footnote, color: theme.colors.textTertiary)
            }
            return MenuRow(
                lead: IconView(icon, size: theme.isTV ? 26 : 18, color: theme.colors.textSecondary), title: title,
                end: end,
                theme: theme, action: action)
        }

        /// The site of mavio.dev whose API is api.
        private static func site(_ api: String) -> String {
            var site = api
            while site.hasSuffix("/") { site.removeLast() }
            if site.hasSuffix("/api") { site.removeLast(4) }
            return site
        }

        // MARK: Switching users

        private func choose(_ user: SwitchableUser) {
            if user.hasPin {
                let root = root
                root.closeUserMenu()
                PinSheet.ask(root: root, user: user, server: standalone)
            } else {
                Task { await switchTo(user) }
            }
        }

        private func switchTo(_ user: SwitchableUser) async {
            guard let server = standalone else { return }
            busy = true
            build()
            defer {
                busy = false
                build()
            }
            do {
                try await context.model.switchUser(server.id, to: user.id, pin: nil)
                root.closeUserMenu()
            } catch {
                let alert = NSAlert()
                alert.messageText = MavioStrings.stateError
                alert.informativeText = (error as? CoreError)?.reason ?? error.localizedDescription
                if let window { alert.beginSheetModal(for: window, completionHandler: nil) }
            }
        }
    }

    /// The PIN pad of switching to a user with a PIN, as a sheet.
    @MainActor
    enum PinSheet {
        static func ask(root: RootController, user: SwitchableUser, server: ServerRecord?) {
            guard let server else { return }
            let model = root.context.model
            var pad: PinPadController?
            pad = SettingsPages.pinPad(
                context: root.context, name: user.name,
                submit: { pin in
                    guard let shown = pad else { return }
                    shown.busy = true
                    shown.error = nil
                    Task {
                        do {
                            try await model.switchUser(server.id, to: user.id, pin: pin)
                            shown.busy = false
                            root.dismiss(shown)
                            pad = nil
                        } catch {
                            shown.busy = false
                            shown.error = (error as? CoreError)?.reason ?? error.localizedDescription
                        }
                    }
                },
                cancel: {
                    guard let shown = pad else { return }
                    root.dismiss(shown)
                    pad = nil
                })
            if let pad { root.presentAsSheet(pad) }
        }
    }

    /// A row of the user's menu: highlighted under the pointer and, in
    /// television mode, with focus, as the web UI's.
    final class MenuRow: Box, Focusable {
        private let theme: Theme
        private let action: @MainActor () -> Void
        private var hovering = false
        private var tracking: NSTrackingArea?
        var enabled = true { didSet { alphaValue = enabled ? 1 : 0.5 } }

        init(lead: NSView, title: String, end: NSView?, theme: Theme, action: @escaping @MainActor () -> Void) {
            self.theme = theme
            self.action = action
            super.init(radius: theme.isTV ? 10 : 6)
            let label = NSTextField.label(title, theme.fonts.body, color: theme.colors.textPrimary)
            label.setContentHuggingPriority(.init(1), for: .horizontal)
            let stack = NSStackView.horizontal([lead, label] + (end.map { [$0] } ?? []), spacing: MavioSpacing.s8)
            addFilling(
                stack,
                insets: .symmetric(
                    horizontal: theme.isTV ? MavioSpacing.s12 : MavioSpacing.s8,
                    vertical: theme.isTV ? MavioSpacing.s8 : MavioSpacing.s4))
            heightAnchor.constraint(greaterThanOrEqualToConstant: theme.isTV ? 52 : 28).isActive = true
            setAccessibilityElement(true)
            setAccessibilityRole(.menuItem)
            setAccessibilityLabel(title)
        }

        @available(*, unavailable)
        required init?(coder: NSCoder) { fatalError() }

        var isFocused = false { didSet { restyle() } }

        func activate() { if enabled { action() } }

        private func restyle() {
            fill = (isFocused || hovering) && enabled ? theme.colors.fill : nil
        }

        override func mouseDown(with event: NSEvent) {}

        override func mouseUp(with event: NSEvent) {
            if bounds.contains(convert(event.locationInWindow, from: nil)) { activate() }
        }

        override func accessibilityPerformPress() -> Bool {
            activate()
            return true
        }

        override func updateTrackingAreas() {
            super.updateTrackingAreas()
            if let tracking { removeTrackingArea(tracking) }
            guard !theme.isTV else { return }
            let area = NSTrackingArea(
                rect: bounds, options: [.mouseEnteredAndExited, .activeInKeyWindow, .inVisibleRect], owner: self)
            addTrackingArea(area)
            tracking = area
        }

        override func mouseEntered(with event: NSEvent) {
            hovering = true
            restyle()
        }

        override func mouseExited(with event: NSEvent) {
            hovering = false
            restyle()
        }

        override func viewWillMove(toWindow newWindow: NSWindow?) {
            super.viewWillMove(toWindow: newWindow)
            focusWillMove(to: newWindow)
        }
    }

    /// The appearance, chosen beside the user's name: the system's, light
    /// or dark, in a capsule.
    final class AppearanceSwitch: Box {
        private var buttons: [AppearanceButton] = []

        init(theme: Theme) {
            super.init(fill: theme.colors.fill, radius: 999)
            let stack = NSStackView.horizontal([], spacing: 2)
            // The capsule is as wide as its buttons, the user's name
            // taking the rest of the row.
            stack.setHuggingPriority(.required, for: .horizontal)
            addFilling(stack, insets: .all(2))
            for choice in Appearance.allCases {
                let button = AppearanceButton(choice, theme: theme) { [weak self] in self?.refresh() }
                buttons.append(button)
                stack.addArrangedSubview(button)
            }
            refresh()
        }

        @available(*, unavailable)
        required init?(coder: NSCoder) { fatalError() }

        private func refresh() {
            let raw = UserDefaults.standard.string(forKey: DeviceSetting.appearance) ?? ""
            let current = Appearance(rawValue: raw) ?? .system
            for button in buttons { button.chosen = button.choice == current }
        }
    }

    /// A choice of appearance.
    final class AppearanceButton: Box, Focusable {
        let choice: Appearance
        private let theme: Theme
        private let icon: IconView
        private let changed: @MainActor () -> Void

        init(_ choice: Appearance, theme: Theme, changed: @escaping @MainActor () -> Void) {
            self.choice = choice
            self.theme = theme
            self.changed = changed
            let name =
                switch choice {
                case .system: "appearance"
                case .light: "lightMode"
                case .dark: "darkMode"
                }
            let side: CGFloat = theme.isTV ? 40 : 28
            icon = IconView(name, size: theme.isTV ? 22 : 16, color: theme.colors.textSecondary)
            super.init(radius: side / 2)
            size(width: side, height: side)
            addSubview(icon)
            icon.center(in: self)
            toolTip = choice.label
            setAccessibilityElement(true)
            setAccessibilityRole(.radioButton)
            setAccessibilityLabel(choice.label)
        }

        @available(*, unavailable)
        required init?(coder: NSCoder) { fatalError() }

        var chosen = false { didSet { restyle() } }
        var isFocused = false { didSet { restyle() } }

        func activate() {
            UserDefaults.standard.set(choice.rawValue, forKey: DeviceSetting.appearance)
            changed()
        }

        private func restyle() {
            let focused = isFocused && theme.isTV
            fill = focused ? .white : chosen ? theme.colors.surfaceRaised : nil
            elevation =
                chosen && !focused
                ? MavioNSShadow(color: NSColor.black.withAlphaComponent(0.12), x: 0, y: 1, blur: 2, spread: 0) : nil
            icon.contentTintColor = focused ? .black : chosen ? theme.colors.textPrimary : theme.colors.textSecondary
            setAccessibilityValue(chosen)
        }

        override func mouseDown(with event: NSEvent) {}

        override func mouseUp(with event: NSEvent) {
            if bounds.contains(convert(event.locationInWindow, from: nil)) { activate() }
        }

        override func accessibilityPerformPress() -> Bool {
            activate()
            return true
        }

        override func viewWillMove(toWindow newWindow: NSWindow?) {
            super.viewWillMove(toWindow: newWindow)
            focusWillMove(to: newWindow)
        }
    }
#endif
