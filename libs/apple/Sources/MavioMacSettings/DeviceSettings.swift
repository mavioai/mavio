#if os(macOS)
    import AppKit
    import MavioMac

    /// Appearance, as the web UI's: the theme as three pictures, television
    /// mode, and the language, which the Mac sets for each app in Language
    /// & Region.
    final class AppearancePage: Page {
        private var form: FormPage?

        override func viewDidLoad() {
            super.viewDidLoad()
            title = MavioStrings.settingsAppearance
            let form = FormPage(title: MavioStrings.settingsAppearance, theme: theme)
            self.form = form
            show(form)
            render()
        }

        private func render() {
            let current =
                Appearance(rawValue: UserDefaults.standard.string(forKey: DeviceSetting.appearance) ?? "") ?? .system
            let options = ThemeChoice(selected: current, theme: theme) { [weak self] option in
                UserDefaults.standard.set(option.rawValue, forKey: DeviceSetting.appearance)
                self?.render()
            }
            let themes = FormSection(footer: MavioStrings.settingsAppearanceMessage, rows: [options], theme: theme)
            let tv = FormRow.toggle(
                MavioStrings.settingsTvMode, subtitle: MavioStrings.settingsTvModeMessage,
                isOn: UserDefaults.standard.bool(forKey: DeviceSetting.tvMode), theme: theme
            ) { on in
                // The app observes the setting, and switches its layout.
                UserDefaults.standard.set(on, forKey: DeviceSetting.tvMode)
            }
            let tvSection = FormSection(
                footer: theme.isTV ? MavioStrings.settingsTvModeNote : nil, rows: [tv], theme: theme)
            let language = FormRow(
                MavioStrings.settingsLanguage,
                accessory: NSStackView.horizontal(
                    [
                        NSTextField.label(Self.language, theme.fonts.body, color: theme.colors.textSecondary),
                        IconView("external", size: 16, color: theme.colors.textTertiary),
                    ], spacing: MavioSpacing.s4),
                theme: theme
            ) {
                if let url = URL(string: "x-apple.systempreferences:com.apple.Localization-Settings.extension") {
                    NSWorkspace.shared.open(url)
                }
            }
            form?.setSections([themes, tvSection, FormSection(rows: [language], theme: theme)])
            FocusEngine.active(for: view)?.schedule()
        }

        /// The language the app shows, in itself.
        private static var language: String {
            let code = Bundle.main.preferredLocalizations.first ?? "en"
            return Locale(identifier: code).localizedString(forIdentifier: code)?.capitalized(
                with: Locale(identifier: code))
                ?? code
        }
    }

    /// The three themes as pictures side by side, the current one ringed;
    /// in television mode focus enters at it.
    private final class ThemeChoice: NSView, FocusChoice {
        private var options: [ThemeOption] = []
        private let selected: Appearance

        init(selected: Appearance, theme: Theme, choose: @escaping @MainActor (Appearance) -> Void) {
            self.selected = selected
            super.init(frame: .zero)
            translatesAutoresizingMaskIntoConstraints = false
            options = Appearance.allCases.map { option in
                ThemeOption(option, on: option == selected, theme: theme) { choose(option) }
            }
            let row = NSStackView.horizontal(options, spacing: MavioSpacing.s16, alignment: .top)
            row.distribution = .fillEqually
            addFilling(row, insets: .symmetric(horizontal: MavioSpacing.s16, vertical: MavioSpacing.s16))
        }

        @available(*, unavailable)
        required init?(coder: NSCoder) { fatalError() }

        var currentChoice: NSView? { options.first { $0.option == selected } }
    }

    /// A theme: a small window drawn in it over its name.
    private final class ThemeOption: NSView, Focusable {
        let option: Appearance
        private let theme: Theme
        private let on: Bool
        private let action: @MainActor () -> Void
        private let preview: ThemePreview
        private let label: NSTextField

        init(_ appearance: Appearance, on: Bool, theme: Theme, action: @escaping @MainActor () -> Void) {
            self.option = appearance
            self.theme = theme
            self.on = on
            self.action = action
            preview = ThemePreview(appearance, theme: theme)
            label = .label(appearance.label, theme.fonts.callout, color: theme.colors.textSecondary)
            super.init(frame: .zero)
            translatesAutoresizingMaskIntoConstraints = false
            let stack = NSStackView.vertical([preview, label], spacing: MavioSpacing.s8, alignment: .centerX)
            addFilling(stack)
            preview.widthAnchor.constraint(equalTo: stack.widthAnchor).isActive = true
            preview.aspect(16 / 10)
            setAccessibilityElement(true)
            setAccessibilityRole(.radioButton)
            setAccessibilityLabel(appearance.label)
            setAccessibilityValue(on)
            restyle()
        }

        @available(*, unavailable)
        required init?(coder: NSCoder) { fatalError() }

        var isFocused = false { didSet { restyle() } }

        func activate() { action() }

        private func restyle() {
            let focused = isFocused && theme.isTV
            preview.ring = focused ? .white : on ? theme.colors.accent : theme.colors.border
            preview.ringWidth = focused ? 4 : on ? 2 : 1
            let base = theme.fonts.callout
            label.font = .systemFont(ofSize: base.size, weight: on ? .semibold : .regular)
            label.textColor = on || focused ? theme.colors.textPrimary : theme.colors.textSecondary
            wantsLayer = true
            if theme.isTV {
                NSAnimationContext.runAnimationGroup { context in
                    context.duration = MavioDuration.fast
                    context.allowsImplicitAnimation = true
                    layer?.transform = focused ? CATransform3DMakeScale(1.05, 1.05, 1) : CATransform3DIdentity
                }
            }
        }

        override func mouseDown(with event: NSEvent) {}

        override func mouseUp(with event: NSEvent) {
            if bounds.contains(convert(event.locationInWindow, from: nil)) { action() }
        }

        override func viewWillMove(toWindow newWindow: NSWindow?) {
            super.viewWillMove(toWindow: newWindow)
            focusWillMove(to: newWindow)
        }

        override func accessibilityPerformPress() -> Bool {
            action()
            return true
        }
    }

    /// A small window in a theme, as the web UI draws it; System shows both
    /// halves.
    private final class ThemePreview: NSView {
        private let option: Appearance
        var ring: NSColor = .clear { didSet { needsDisplay = true } }
        var ringWidth: CGFloat = 1 { didSet { needsDisplay = true } }

        init(_ appearance: Appearance, theme: Theme) {
            self.option = appearance
            super.init(frame: .zero)
            translatesAutoresizingMaskIntoConstraints = false
        }

        @available(*, unavailable)
        required init?(coder: NSCoder) { fatalError() }

        override var isFlipped: Bool { true }

        override func draw(_ dirtyRect: NSRect) {
            let outline = NSBezierPath(roundedRect: bounds, xRadius: 12, yRadius: 12)
            NSGraphicsContext.saveGraphicsState()
            outline.addClip()
            switch option {
            case .system:
                let half = bounds.width / 2
                pane(NSRect(x: 0, y: 0, width: half, height: bounds.height), dark: false)
                pane(NSRect(x: half, y: 0, width: bounds.width - half, height: bounds.height), dark: true)
            case .light: pane(bounds, dark: false)
            case .dark: pane(bounds, dark: true)
            }
            NSGraphicsContext.restoreGraphicsState()
            let inset = bounds.insetBy(dx: ringWidth / 2, dy: ringWidth / 2)
            let border = NSBezierPath(roundedRect: inset, xRadius: 12 - ringWidth / 2, yRadius: 12 - ringWidth / 2)
            border.lineWidth = ringWidth
            ring.setStroke()
            border.stroke()
        }

        private func pane(_ rect: NSRect, dark: Bool) {
            NSColor(hex: dark ? 0x1C1C1E : 0xF2F2F7).setFill()
            rect.fill()
            let inner = rect.insetBy(dx: 8, dy: 8)
            let sidebar = NSRect(x: inner.minX, y: inner.minY, width: (rect.width - 16) / 4, height: inner.height)
            NSColor(hex: dark ? 0x2C2C2E : 0xFFFFFF).setFill()
            NSBezierPath(roundedRect: sidebar, xRadius: 4, yRadius: 4).fill()
            let x = sidebar.maxX + 6
            let hero = NSRect(x: x, y: inner.minY, width: inner.maxX - x, height: (rect.height - 16) / 2)
            let gradient = NSGradient(colors: [
                NSColor(hex: 0xFFD34E), NSColor(hex: 0xFF9A2E), NSColor(hex: 0xF0412F),
            ])
            gradient?.draw(in: NSBezierPath(roundedRect: hero, xRadius: 4, yRadius: 4), angle: -45)
            let line = NSRect(x: x, y: hero.maxY + 6, width: (rect.width - 16) * 0.5, height: 6)
            NSColor(hex: dark ? 0x3A3A3C : 0xD1D1D6).setFill()
            NSBezierPath(roundedRect: line, xRadius: 3, yRadius: 3).fill()
        }
    }

    /// Playback, as the web UI's: the highest quality streamed to this
    /// device, which the player's panel also sets, whether the next episode
    /// plays when one ends, and whether each video has a window of its own.
    final class PlaybackPage: Page {
        override func viewDidLoad() {
            super.viewDidLoad()
            title = MavioStrings.settingsPlayback
            let defaults = UserDefaults.standard
            let quality = PlaybackQuality(rawValue: defaults.string(forKey: PlaybackQuality.key) ?? "") ?? .auto
            let rows: [NSView] = [
                FormRow.choice(
                    MavioStrings.settingsMaxQuality, options: PlaybackQuality.allCases.map { ($0.label, $0) },
                    selected: quality, theme: theme
                ) { defaults.set($0.rawValue, forKey: PlaybackQuality.key) },
                FormRow.toggle(
                    MavioStrings.settingsAutoplayNext,
                    isOn: defaults.object(forKey: DeviceSetting.autoplayNext) as? Bool ?? true, theme: theme
                ) { defaults.set($0, forKey: DeviceSetting.autoplayNext) },
                FormRow.toggle(
                    MavioStrings.settingsPlayerWindows, subtitle: MavioStrings.settingsPlayerWindowsMessage,
                    isOn: defaults.bool(forKey: DeviceSetting.playerWindows), theme: theme
                ) { defaults.set($0, forKey: DeviceSetting.playerWindows) },
            ]
            let form = FormPage(title: MavioStrings.settingsPlayback, theme: theme)
            form.add(FormSection(footer: MavioStrings.settingsAppearanceMessage, rows: rows, theme: theme))
            show(form)
        }
    }

    /// Subtitles and Audio, as the web UI's: the user's languages and when
    /// subtitles show, read from the first server signed in to and saved to
    /// each of them.
    final class SubtitlesAudioPage: Page {
        private var saved: PlaybackPreferences?
        private var audio = ""
        private var subtitles = ""
        private var mode = SubtitleMode.default
        private var failure: String?
        private var saving = false
        private var loading: Task<Void, Never>?
        private var form: FormPage?
        private var saveButton: MavioButton?
        private var fields: [NSTextField] = []
        private var watchers: [FieldWatcher] = []

        /// The modes in the web UI's order.
        private static let modes: [(String, SubtitleMode)] = [
            (MavioStrings.settingsSubtitleModeDefault, .default),
            (MavioStrings.settingsSubtitleModeAlways, .always),
            (MavioStrings.settingsSubtitleModeForeign, .foreign),
            (MavioStrings.settingsSubtitleModeSmart, .smart),
            (MavioStrings.settingsSubtitleModeForced, .forced),
            (MavioStrings.settingsSubtitleModeNone, .never),
        ]

        override func viewDidLoad() {
            super.viewDidLoad()
            title = MavioStrings.settingsSubtitlesAudio
            load()
        }

        isolated deinit { loading?.cancel() }

        private func load() {
            show(LoadingView())
            loading = Task { [weak self] in
                guard let model = self?.model else { return }
                do {
                    let prefs = try await model.client.playbackPreferences()
                    self?.loaded(prefs)
                } catch is CancellationError {
                } catch {
                    self?.showError(error.reason) { [weak self] in self?.load() }
                }
            }
        }

        private func loaded(_ prefs: PlaybackPreferences) {
            saved = prefs
            audio = prefs.audioLanguages.joined(separator: ", ")
            subtitles = prefs.subtitleLanguages.joined(separator: ", ")
            mode = prefs.subtitleMode
            render()
        }

        private func render() {
            let form = FormPage(title: MavioStrings.settingsSubtitlesAudio, theme: theme)
            self.form = form
            fields = []
            watchers = []
            let languages = FormSection(
                MavioStrings.settingsSubtitlesAudioMessage, footer: MavioStrings.settingsLanguagesHint,
                rows: [
                    field(MavioStrings.settingsAudioLanguages, text: audio) { [weak self] in self?.audio = $0 },
                    field(MavioStrings.settingsSubtitleLanguages, text: subtitles) { [weak self] in self?.subtitles = $0
                    },
                ], theme: theme)
            let modeRow = FormRow.choice(
                MavioStrings.settingsSubtitleMode, options: Self.modes, selected: mode, theme: theme
            ) { [weak self] in
                self?.mode = $0
                self?.update()
            }
            form.add(languages)
            form.add(FormSection(rows: [modeRow], theme: theme))
            let save = MavioButton(MavioStrings.actionSave, kind: .primary, large: true, theme: theme) {
                [weak self] in self?.save()
            }
            saveButton = save
            var actions: [NSView] = [save]
            if saving {
                let spinner = NSProgressIndicator()
                spinner.style = .spinning
                spinner.controlSize = .small
                spinner.startAnimation(nil)
                actions.append(spinner)
            }
            if let failure {
                actions.append(NSTextField.label(failure, theme.fonts.callout, color: theme.colors.danger, lines: 0))
            }
            form.add(NSStackView.horizontal(actions, spacing: MavioSpacing.s12))
            show(form)
            update()
            FocusEngine.active(for: view)?.schedule()
        }

        private func field(_ label: String, text: String, onChange: @escaping @MainActor (String) -> Void) -> NSView {
            let field = NSTextField(string: text)
            field.placeholderString = "chi, eng"
            field.bezelStyle = .roundedBezel
            field.translatesAutoresizingMaskIntoConstraints = false
            fields.append(field)
            watchers.append(
                FieldWatcher(field) { [weak self] value in
                    onChange(value)
                    self?.update()
                })
            let title = NSTextField.label(label, theme.fonts.footnote, color: theme.colors.textSecondary)
            let box = FocusField(field, theme: theme)
            let stack = NSStackView.vertical([title, box], spacing: MavioSpacing.s4)
            box.widthAnchor.constraint(equalTo: stack.widthAnchor).isActive = true
            return SettingsParts.padded(stack, theme: theme)
        }

        /// Colors a field whose codes do not parse, and enables Save once
        /// something changed and every field parses.
        private func update() {
            for (field, text) in zip(fields, [audio, subtitles]) {
                field.textColor = languageCodes(text) == nil ? theme.colors.danger : theme.colors.textPrimary
            }
            saveButton?.isEnabled =
                !saving && changed && languageCodes(audio) != nil && languageCodes(subtitles) != nil
        }

        private var changed: Bool {
            guard let saved else { return false }
            return languageCodes(audio) != saved.audioLanguages
                || languageCodes(subtitles) != saved.subtitleLanguages || mode != saved.subtitleMode
        }

        private func save() {
            guard var prefs = saved, let a = languageCodes(audio), let s = languageCodes(subtitles) else { return }
            prefs.audioLanguages = a
            prefs.subtitleLanguages = s
            prefs.subtitleMode = mode
            view.window?.makeFirstResponder(nil)
            saving = true
            failure = nil
            render()
            loading = Task { [weak self] in
                guard let model = self?.model else { return }
                do {
                    try await model.client.setPlaybackPreferences(preferences: prefs)
                    self?.saving = false
                    self?.loaded(prefs)
                } catch {
                    self?.saving = false
                    self?.failure = error.reason
                    self?.render()
                }
            }
        }
    }
#endif
