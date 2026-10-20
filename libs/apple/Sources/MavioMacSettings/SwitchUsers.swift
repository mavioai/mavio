#if os(macOS)
    import AppKit
    import MavioMac

    /// The users of a standalone server as avatars, to switch to with their
    /// PIN, as the web UI's account settings show them: a section hidden
    /// until there is someone else to switch to.
    @MainActor
    final class SwitchUsers {
        /// The section, which a page adds.
        let section: FormSection
        private weak var page: Page?
        private var server: ServerRecord
        private var users: [SwitchableUser] = []
        private var busy = false
        private var failure: String?
        private var pad: PinPadController?
        private let theme: Theme

        /// Switching among the users of server, for page.
        init(page: Page, server: ServerRecord) {
            self.page = page
            self.server = server
            theme = page.theme
            section = FormSection(MavioStrings.authSwitchUser, theme: page.theme)
            section.isHidden = true
        }

        /// Shows users, of server.
        func set(_ users: [SwitchableUser], server: ServerRecord) {
            self.users = users
            self.server = server
            render()
        }

        private func render() {
            // Switching needs someone else to switch to.
            section.isHidden = users.count <= 1
            let members = users.map { user in
                MemberView(user, theme: theme) { [weak self] in self?.choose(user) } as NSView
            }
            for case let member as MemberView in members {
                member.isEnabled = !member.current && !busy
            }
            let row = NSStackView.horizontal(members, spacing: MavioSpacing.s20, alignment: .top)
            let scroll = NSScrollView()
            scroll.drawsBackground = false
            scroll.hasHorizontalScroller = false
            scroll.verticalScrollElasticity = .none
            scroll.translatesAutoresizingMaskIntoConstraints = false
            let document = FlippedView()
            document.translatesAutoresizingMaskIntoConstraints = false
            document.addFilling(row, insets: .symmetric(horizontal: MavioSpacing.s16, vertical: MavioSpacing.s12))
            scroll.documentView = document
            NSLayoutConstraint.activate([
                document.topAnchor.constraint(equalTo: scroll.contentView.topAnchor),
                document.leadingAnchor.constraint(equalTo: scroll.contentView.leadingAnchor),
                scroll.heightAnchor.constraint(equalTo: document.heightAnchor),
            ])
            var rows: [NSView] = [scroll]
            if let failure, pad == nil {
                let label = NSTextField.label(failure, theme.fonts.callout, color: theme.colors.danger, lines: 0)
                let holder = NSView()
                holder.translatesAutoresizingMaskIntoConstraints = false
                holder.addFilling(label, insets: .symmetric(horizontal: MavioSpacing.s16, vertical: MavioSpacing.s12))
                rows.append(holder)
            }
            section.setRows(rows)
            if let view = page?.view { FocusEngine.active(for: view)?.schedule() }
        }

        private func choose(_ user: SwitchableUser) {
            guard !user.current, !busy else { return }
            if user.hasPin {
                ask(user)
            } else {
                switchTo(user, pin: nil)
            }
        }

        /// Asks user's PIN on the pad, as a sheet.
        private func ask(_ user: SwitchableUser) {
            guard let page else { return }
            failure = nil
            let pad = PinPadController(theme: theme, name: user.name) { [weak self] pin in
                self?.switchTo(user, pin: pin)
            } cancel: { [weak self] in
                self?.closePad()
            }
            self.pad = pad
            page.presentAsSheet(pad)
        }

        private func closePad() {
            if let pad { pad.presentingViewController?.dismiss(pad) }
            pad = nil
            failure = nil
            render()
        }

        private func switchTo(_ user: SwitchableUser, pin: String?) {
            guard let model = page?.model else { return }
            busy = true
            failure = nil
            pad?.busy = true
            render()
            let serverId = server.id
            Task { [weak self] in
                do {
                    try await model.switchUser(serverId, to: user.id, pin: pin)
                    guard let self else { return }
                    self.busy = false
                    self.closePad()
                } catch {
                    guard let self else { return }
                    self.busy = false
                    self.pad?.busy = false
                    if let pad = self.pad {
                        pad.error = error.reason
                    } else {
                        self.failure = error.reason
                    }
                    self.render()
                }
            }
        }
    }

    /// A user to switch to: their avatar, ringed when current and marked
    /// with a lock when they have a PIN, over their name.
    private final class MemberView: Box, Focusable {
        let current: Bool
        private let theme: Theme
        private let action: @MainActor () -> Void
        private let name: NSTextField
        private let ring: Box

        var isEnabled = true { didSet { restyle() } }

        init(_ user: SwitchableUser, theme: Theme, action: @escaping @MainActor () -> Void) {
            current = user.current
            self.theme = theme
            self.action = action
            let side: CGFloat = theme.isTV ? 80 : 56
            ring = Box(radius: (side + 6) / 2)
            ring.size(width: side + 6, height: side + 6)
            let avatar = AvatarView(name: user.name, size: side, theme: theme)
            ring.addSubview(avatar)
            avatar.center(in: ring)
            name = .label(user.name, theme.fonts.footnote, color: theme.colors.textPrimary)
            name.alignment = .center
            super.init(radius: MavioRadius.lg)
            if user.hasPin {
                let badge = Box(fill: theme.colors.surfaceRaised, radius: 10)
                badge.size(width: 20, height: 20)
                let lock = IconView("lock", fill: true, size: 12, color: theme.colors.textPrimary)
                badge.addSubview(lock)
                lock.center(in: badge)
                ring.addSubview(badge)
                NSLayoutConstraint.activate([
                    badge.trailingAnchor.constraint(equalTo: ring.trailingAnchor),
                    badge.bottomAnchor.constraint(equalTo: ring.bottomAnchor),
                ])
            }
            let stack = NSStackView.vertical([ring, name], spacing: 6, alignment: .centerX)
            addFilling(stack, insets: .all(theme.isTV ? MavioSpacing.s8 : 0))
            name.widthAnchor.constraint(lessThanOrEqualToConstant: theme.isTV ? 120 : 68).isActive = true
            widthAnchor.constraint(equalToConstant: theme.isTV ? 136 : 68).isActive = true
            setAccessibilityElement(true)
            setAccessibilityRole(.button)
            setAccessibilityLabel(user.name)
            if user.current { setAccessibilityValue(MavioStrings.authCurrent) }
            restyle()
        }

        @available(*, unavailable)
        required init?(coder: NSCoder) { fatalError() }

        var isFocused = false { didSet { restyle() } }

        func activate() {
            guard isEnabled else { return }
            action()
        }

        private func restyle() {
            let focused = isFocused && theme.isTV
            fill = focused ? .white : nil
            name.textColor = focused ? .black : theme.colors.textPrimary
            ring.border = current ? theme.colors.accent : nil
            ring.borderWidth = 2
            alphaValue = isEnabled || current ? 1 : 0.5
        }

        override func mouseDown(with event: NSEvent) {}

        override func mouseUp(with event: NSEvent) {
            if bounds.contains(convert(event.locationInWindow, from: nil)) { activate() }
        }

        override func viewWillMove(toWindow newWindow: NSWindow?) {
            super.viewWillMove(toWindow: newWindow)
            focusWillMove(to: newWindow)
        }

        override func accessibilityPerformPress() -> Bool {
            activate()
            return true
        }
    }
#endif
