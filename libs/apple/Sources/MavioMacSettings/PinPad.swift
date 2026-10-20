#if os(macOS)
    import AppKit
    import MavioMac

    /// Asks for a user's four-digit PIN on a phone's keypad, as the web
    /// UI's: the fourth digit sends it, and a wrong one clears the dots. The
    /// keyboard's digits type it and Delete takes one back; in television
    /// mode back deletes a digit and closes the pad once there is none
    /// (docs/tv.md §4).
    public final class PinPadController: NSViewController {
        private let theme: Theme
        private let name: String
        private let submit: @MainActor (String) -> Void
        private let cancel: @MainActor () -> Void
        private var pin = "" { didSet { showDots() } }
        private var dots: [Box] = []
        private var keys: [PinKey] = []
        private let errorLabel: NSTextField

        /// Whether the PIN entered is being checked, which disables the keys.
        public var busy = false {
            didSet { for key in keys where key.digit != nil || key.isDelete { key.isEnabled = !busy } }
        }

        /// The error of the PIN last entered, which clears the dots.
        public var error: String? {
            didSet {
                errorLabel.stringValue = error ?? " "
                errorLabel.alphaValue = error == nil ? 0 : 1
                if error != nil {
                    pin = ""
                    NSHapticFeedbackManager.defaultPerformer.perform(.generic, performanceTime: .now)
                    shake()
                }
            }
        }

        /// The pad asking name's PIN, giving it to submit; cancel closes it.
        public init(
            theme: Theme, name: String, submit: @escaping @MainActor (String) -> Void,
            cancel: @escaping @MainActor () -> Void
        ) {
            self.theme = theme
            self.name = name
            self.submit = submit
            self.cancel = cancel
            errorLabel = .label(" ", theme.fonts.footnote, color: theme.colors.danger)
            errorLabel.alphaValue = 0
            super.init(nibName: nil, bundle: nil)
        }

        /// Unavailable: the view is made in code.
        @available(*, unavailable)
        public required init?(coder: NSCoder) { fatalError() }

        override public func loadView() {
            let root = PinPadView(pad: self, fill: theme.isTV ? theme.colors.tvBackground : theme.colors.background)
            let title = NSTextField.label(
                MavioStrings.authEnterPin(name: name), theme.fonts.title3, color: theme.colors.textPrimary, lines: 0)
            title.alignment = .center
            dots = (0..<4).map { _ in
                let dot = Box(radius: 6)
                dot.border = theme.colors.textPrimary
                dot.borderWidth = 2
                dot.size(width: 12, height: 12)
                return dot
            }
            let dotRow = NSStackView.horizontal(dots, spacing: MavioSpacing.s16)
            dotRow.setAccessibilityElement(true)
            dotRow.setAccessibilityLabel(MavioStrings.authPin)
            let side: CGFloat = theme.isTV ? 96 : 72
            var rows: [NSView] = []
            for row in [["1", "2", "3"], ["4", "5", "6"], ["7", "8", "9"], ["", "0", "⌫"]] {
                let views = row.map { k -> NSView in
                    let key = PinKey(k, side: side, theme: theme) { [weak self] in self?.press(k) }
                    keys.append(key)
                    return key
                }
                rows.append(NSStackView.horizontal(views, spacing: MavioSpacing.s20))
            }
            let grid = NSStackView.vertical(rows, spacing: MavioSpacing.s12, alignment: .centerX)
            let stack = NSStackView.vertical(
                [AvatarView(name: name, size: theme.isTV ? 80 : 56, theme: theme), title, dotRow, errorLabel, grid],
                spacing: MavioSpacing.s20, alignment: .centerX)
            root.addSubview(stack)
            stack.center(in: root)
            NSLayoutConstraint.activate([
                stack.leadingAnchor.constraint(greaterThanOrEqualTo: root.leadingAnchor, constant: MavioSpacing.s24),
                stack.topAnchor.constraint(greaterThanOrEqualTo: root.topAnchor, constant: MavioSpacing.s24),
                title.widthAnchor.constraint(lessThanOrEqualToConstant: 320),
            ])
            root.frame = NSRect(x: 0, y: 0, width: theme.isTV ? 520 : 360, height: theme.isTV ? 760 : 560)
            view = root
            showDots()
        }

        override public func viewDidAppear() {
            super.viewDidAppear()
            view.window?.makeFirstResponder(view)
            if let five = keys.first(where: { $0.digit == "5" }) { FocusEngine.active(for: view)?.prefer(five) }
        }

        fileprivate func press(_ k: String) {
            switch k {
            // Cancel takes the keypad's empty corner, as on a phone's.
            case "": cancel()
            case "⌫": pin = String(pin.dropLast())
            default:
                guard !busy, pin.count < 4 else { return }
                if error != nil { error = nil }
                pin += k
                if pin.count == 4 { submit(pin) }
            }
        }

        /// Handles a key typed or pressed on the remote: digits, Delete,
        /// and back in television mode, which deletes a digit or closes.
        fileprivate func handle(_ event: NSEvent, back: Bool) -> Bool {
            if let c = event.charactersIgnoringModifiers, c.count == 1, let d = c.first, d.isASCII, d.isNumber {
                press(String(d))
                return true
            }
            switch event.keyCode {
            case 51, 117:
                if pin.isEmpty, back { cancel() } else { press("⌫") }
                return true
            case 53:
                if pin.isEmpty || !back { cancel() } else { press("⌫") }
                return true
            default:
                return false
            }
        }

        private func showDots() {
            for (i, dot) in dots.enumerated() { dot.fill = i < pin.count ? theme.colors.textPrimary : nil }
            dots.first?.superview?.setAccessibilityValue("\(pin.count)/4")
        }

        private func shake() {
            guard let row = dots.first?.superview else { return }
            row.wantsLayer = true
            guard let layer = row.layer else { return }
            let animation = CAKeyframeAnimation(keyPath: "transform.translation.x")
            animation.values = [0, -8, 8, -6, 6, -3, 3, 0]
            animation.duration = 0.32
            layer.add(animation, forKey: "shake")
        }
    }

    /// The pad's view, which takes the keys typed and, as a scope, keeps
    /// television mode's focus within it.
    private final class PinPadView: Box, FocusScope, KeyHandling {
        weak var pad: PinPadController?

        init(pad: PinPadController, fill: NSColor) {
            self.pad = pad
            super.init(fill: fill)
            translatesAutoresizingMaskIntoConstraints = true
        }

        @available(*, unavailable)
        required init?(coder: NSCoder) { fatalError() }

        override var acceptsFirstResponder: Bool { true }

        override func keyDown(with event: NSEvent) {
            if pad?.handle(event, back: false) != true { super.keyDown(with: event) }
        }

        func handleKey(_ event: NSEvent) -> Bool {
            // Arrows and select move and press; the rest are the pad's.
            if [123, 124, 125, 126, 36, 76, 49].contains(event.keyCode) { return false }
            return pad?.handle(event, back: true) ?? false
        }
    }

    /// A key of the pad: a digit on a round fill, Delete bare, Cancel as
    /// text.
    private final class PinKey: Box, Focusable {
        let digit: String?
        let isDelete: Bool
        private let theme: Theme
        private let action: @MainActor () -> Void
        private let content: NSView
        private var pressed = false
        var isEnabled = true { didSet { restyle() } }

        init(_ key: String, side: CGFloat, theme: Theme, action: @escaping @MainActor () -> Void) {
            self.theme = theme
            self.action = action
            digit = key.isEmpty || key == "⌫" ? nil : key
            isDelete = key == "⌫"
            switch key {
            case "":
                content = NSTextField.label(
                    MavioStrings.actionCancel, theme.fonts.callout, color: theme.colors.textPrimary)
            case "⌫":
                content = IconView("backspace", size: theme.isTV ? 32 : 24, color: theme.colors.textPrimary)
            default:
                let label = NSTextField.label(key, theme.fonts.body, color: theme.colors.textPrimary)
                let base = NSFont.systemFont(ofSize: theme.isTV ? 36 : 28, weight: .regular)
                label.font =
                    base.fontDescriptor.withDesign(.rounded).flatMap { NSFont(descriptor: $0, size: 0) } ?? base
                content = label
            }
            super.init(radius: side / 2)
            size(width: side, height: side)
            addSubview(content)
            content.center(in: self)
            setAccessibilityElement(true)
            setAccessibilityRole(.button)
            setAccessibilityLabel(
                key.isEmpty ? MavioStrings.actionCancel : isDelete ? MavioStrings.searchBackspace : key)
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
            fill = focused ? .white : (digit != nil ? theme.colors.fill : nil)
            let color: NSColor = focused ? .black : (isEnabled ? theme.colors.textPrimary : theme.colors.textDisabled)
            (content as? NSTextField)?.textColor = color
            (content as? IconView)?.contentTintColor = color
            alphaValue = pressed ? 0.8 : 1
            elevation =
                focused ? MavioNSShadow(color: theme.colors.tvFocusShadow, x: 0, y: 12, blur: 32, spread: 0) : nil
        }

        override func mouseDown(with event: NSEvent) {
            guard isEnabled else { return }
            pressed = true
            restyle()
        }

        override func mouseUp(with event: NSEvent) {
            pressed = false
            restyle()
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
