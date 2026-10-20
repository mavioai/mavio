#if os(macOS)
    import AppKit
    import MavioMac

    /// Controls another of the user's devices, as the web UI's remote: what
    /// it plays with its position, the playback's buttons, the volume, and a
    /// message to show on it. It shows in a small window of its own, driven
    /// by the pointer.
    final class RemotePage: Page {
        private let server: ServerRecord
        private let sessionId: String
        private var device: DeviceSession?
        private var loaded = false
        private var failure: String?
        private var polling: Task<Void, Never>?
        private var form: FormPage?
        private var playingSection: FormSection?
        private var controlsSection: FormSection?
        private let failureLabel: NSTextField
        private let messageField = NSTextField()
        private var watcher: FieldWatcher?
        private var sendButton: MavioButton?
        private var sent = false
        private var volume = 100.0
        private var dragging = false

        /// How often what the device plays is read again.
        private static let refresh = Duration.seconds(2)

        /// The remote of the session sessionId on server.
        init(context: MacContext, server: ServerRecord, sessionId: String) {
            self.server = server
            self.sessionId = sessionId
            failureLabel = .label("", context.theme.fonts.footnote, color: context.theme.colors.danger, lines: 0)
            super.init(context: context)
        }

        override func viewDidLoad() {
            super.viewDidLoad()
            title = MavioStrings.remoteTitle
            messageField.placeholderString = MavioStrings.remoteMessage
            messageField.bezelStyle = .roundedBezel
            messageField.translatesAutoresizingMaskIntoConstraints = false
            watcher = FieldWatcher(
                messageField,
                onChange: { [weak self] text in
                    guard let self else { return }
                    if text.count > 1000 { messageField.stringValue = String(text.prefix(1000)) }
                    sent = false
                    updateSend()
                },
                onSubmit: { [weak self] in self?.sendMessage() })
            polling = Task { [weak self] in
                while !Task.isCancelled {
                    await self?.load()
                    try? await Task.sleep(for: Self.refresh)
                }
            }
        }

        isolated deinit { polling?.cancel() }

        private func load() async {
            do {
                let devices = try await model.client.devices(serverId: server.id)
                device = devices.first { $0.id == sessionId }
                loaded = true
            } catch is CancellationError {
                return
            } catch {
                failure = error.reason
            }
            render()
        }

        // MARK: Content

        private func render() {
            // The slider being dragged keeps its place until it is let go.
            guard !dragging else { return }
            title = device.map(Self.shownName) ?? MavioStrings.remoteTitle
            failureLabel.stringValue = failure ?? ""
            failureLabel.isHidden = failure == nil
            guard loaded || failure != nil else { return }
            let form: FormPage
            if let existing = self.form {
                form = existing
            } else {
                form = FormPage(theme: theme)
                self.form = form
                show(withDone(form))
            }
            var sections: [NSView] = []
            if let device {
                if !device.online {
                    sections.append(note(MavioStrings.remoteOffline))
                } else if let playing = device.playing {
                    sections.append(FormSection(rows: [nowPlaying(playing)], theme: theme))
                } else {
                    sections.append(note(MavioStrings.remoteIdle))
                }
                if device.online {
                    sections.append(FormSection(MavioStrings.remoteVolume, rows: [volumeRow()], theme: theme))
                    sections.append(FormSection(MavioStrings.remoteMessage, rows: [messageRow()], theme: theme))
                }
            }
            sections.append(failureLabel)
            form.setSections(sections)
        }

        /// The page with Done at its bottom, which closes the window.
        private func withDone(_ form: FormPage) -> NSView {
            let done = MavioButton(MavioStrings.actionDone, kind: .primary, theme: theme) { [weak self] in
                self?.view.window?.performClose(nil)
            }
            let bar = NSStackView.horizontal([Spacer(), done], spacing: 0)
            let holder = NSView()
            holder.translatesAutoresizingMaskIntoConstraints = false
            holder.addFilling(bar, insets: .symmetric(horizontal: MavioSpacing.s16, vertical: MavioSpacing.s12))
            let stack = NSStackView.vertical([form, holder], spacing: 0)
            form.widthAnchor.constraint(equalTo: stack.widthAnchor).isActive = true
            holder.widthAnchor.constraint(equalTo: stack.widthAnchor).isActive = true
            return stack
        }

        private func note(_ text: String) -> NSView {
            FormSection(
                rows: [SettingsParts.text(text, color: theme.colors.textSecondary, theme: theme)], theme: theme)
        }

        private func nowPlaying(_ p: RemotePlayback) -> NSView {
            let runtime = p.card.runtime ?? 0
            let position = runtime > 0 ? min(p.position, runtime) : p.position
            let art = ArtworkView(
                (p.card.thumb ?? p.card.backdrop ?? p.card.poster)?.url, title: p.card.title, icon: p.card.kind.icon,
                theme: theme)
            art.size(width: 96, height: 54)
            art.wantsLayer = true
            art.layer?.cornerRadius = 6
            art.layer?.masksToBounds = true
            var labels: [NSView] = [
                NSTextField.label(p.card.title, theme.fonts.headline, color: theme.colors.textPrimary)
            ]
            if let subtitle = p.card.subtitle {
                labels.append(NSTextField.label(subtitle, theme.fonts.footnote, color: theme.colors.textSecondary))
            }
            let info = NSStackView.vertical(labels, spacing: 2)
            info.setContentCompressionResistancePriority(.init(249), for: .horizontal)
            var views: [NSView] = [NSStackView.horizontal([art, info], spacing: MavioSpacing.s12)]
            if runtime > 0 {
                let elapsed = clock(position)
                let slider = SeekSlider(value: position, max: runtime) { [weak self] value, ended in
                    guard let self else { return }
                    dragging = !ended
                    elapsed.stringValue = Self.clock(value)
                    // The position goes once the slider is let go.
                    if ended { send(.seek(position: value)) }
                }
                slider.setAccessibilityLabel(MavioStrings.remotePosition)
                let clocks = NSStackView.horizontal([elapsed, Spacer(), clock(runtime)], spacing: 0)
                let bar = NSStackView.vertical([slider, clocks], spacing: MavioSpacing.s4)
                slider.widthAnchor.constraint(equalTo: bar.widthAnchor).isActive = true
                clocks.widthAnchor.constraint(equalTo: bar.widthAnchor).isActive = true
                views.append(bar)
            }
            let controls = NSStackView.horizontal(
                [
                    control("previous", MavioStrings.remotePrevious) { [weak self] in self?.send(.previous) },
                    control("seekBack", MavioStrings.playerSeekBack(count: 10)) { [weak self] in
                        self?.send(.seek(position: max(0, p.position - 10)))
                    },
                    control(
                        p.paused ? "play" : "pause", p.paused ? MavioStrings.playerPlay : MavioStrings.playerPause,
                        large: true
                    ) { [weak self] in self?.send(p.paused ? .unpause : .pause) },
                    control("seekForward", MavioStrings.playerSeekForward(count: 10)) { [weak self] in
                        self?.send(.seek(position: p.position + 10))
                    },
                    control("next", MavioStrings.remoteNext) { [weak self] in self?.send(.next) },
                ], spacing: MavioSpacing.s8)
            controls.distribution = .equalSpacing
            views.append(controls)
            views.append(
                MavioButton(MavioStrings.remoteStop, kind: .destructive, theme: theme) { [weak self] in
                    self?.send(.stop)
                })
            let stack = NSStackView.vertical(views, spacing: MavioSpacing.s16, alignment: .centerX)
            for view in views.dropLast() { view.widthAnchor.constraint(equalTo: stack.widthAnchor).isActive = true }
            return SettingsParts.padded(stack, theme: theme)
        }

        private func volumeRow() -> NSView {
            let value = NSTextField.label("\(Int(volume))", theme.fonts.footnote, color: theme.colors.textSecondary)
            value.font = .monospacedDigitSystemFont(ofSize: theme.fonts.footnote.size, weight: .regular)
            value.alignment = .right
            value.size(width: 32)
            let slider = SeekSlider(value: volume, max: 100) { [weak self] level, ended in
                guard let self else { return }
                volume = level.rounded()
                value.stringValue = "\(Int(volume))"
                dragging = !ended
                if ended { send(.volume(level: UInt32(volume), muted: false)) }
            }
            slider.setAccessibilityLabel(MavioStrings.remoteVolume)
            let mute = control("mute", MavioStrings.playerMute) { [weak self] in
                self?.send(.volume(level: nil, muted: true))
            }
            return SettingsParts.padded(
                NSStackView.horizontal([mute, slider, value], spacing: MavioSpacing.s12), theme: theme)
        }

        private func messageRow() -> NSView {
            let send = MavioButton(
                sent ? MavioStrings.remoteSent : MavioStrings.remoteSend, kind: .link, theme: theme
            ) { [weak self] in self?.sendMessage() }
            sendButton = send
            updateSend()
            messageField.setContentHuggingPriority(.init(1), for: .horizontal)
            return SettingsParts.padded(
                NSStackView.horizontal([messageField, send], spacing: MavioSpacing.s12), theme: theme)
        }

        private func updateSend() {
            sendButton?.title = sent ? MavioStrings.remoteSent : MavioStrings.remoteSend
            sendButton?.isEnabled = !messageField.stringValue.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty
        }

        private func control(
            _ icon: String, _ label: String, large: Bool = false, action: @escaping @MainActor () -> Void
        ) -> MavioButton {
            let button = MavioButton(
                icon: icon, kind: large ? .circle : .plain, large: large, theme: theme, action: action)
            button.setAccessibilityLabel(label)
            button.toolTip = label
            return button
        }

        private func clock(_ seconds: TimeInterval) -> NSTextField {
            let label = NSTextField.label(Self.clock(seconds), theme.fonts.caption, color: theme.colors.textSecondary)
            label.font = .monospacedDigitSystemFont(ofSize: theme.fonts.caption.size, weight: .regular)
            return label
        }

        // MARK: Commands

        private func sendMessage() {
            let message = messageField.stringValue.trimmingCharacters(in: .whitespacesAndNewlines)
            guard !message.isEmpty else { return }
            Task { [weak self] in
                guard let self, await run(.message(text: message)) else { return }
                messageField.stringValue = ""
                sent = true
                updateSend()
            }
        }

        private func send(_ command: RemoteCommand) {
            Task { [weak self] in
                guard let self, await run(command) else { return }
                await load()
            }
        }

        private func run(_ command: RemoteCommand) async -> Bool {
            do {
                try await model.client.sendCommand(serverId: server.id, sessionId: sessionId, command: command)
                failure = nil
                return true
            } catch {
                failure = error.reason
                render()
                return false
            }
        }

        /// The device's name, else its client's.
        private static func shownName(_ device: DeviceSession) -> String {
            device.deviceName.isEmpty ? (device.client.isEmpty ? "—" : device.client) : device.deviceName
        }

        private static func clock(_ seconds: TimeInterval) -> String {
            let s = Int(max(0, seconds))
            return s >= 3600
                ? String(format: "%d:%02d:%02d", s / 3600, s / 60 % 60, s % 60)
                : String(format: "%d:%02d", s / 60, s % 60)
        }
    }

    /// A slider that tells its value as it is dragged, and once more when it
    /// is let go.
    private final class SeekSlider: NSSlider {
        private let onChange: @MainActor (Double, _ ended: Bool) -> Void

        init(value: Double, max: Double, onChange: @escaping @MainActor (Double, _ ended: Bool) -> Void) {
            self.onChange = onChange
            super.init(frame: .zero)
            translatesAutoresizingMaskIntoConstraints = false
            minValue = 0
            maxValue = max
            doubleValue = value
            isContinuous = true
            target = self
            action = #selector(changed)
            setContentHuggingPriority(.init(1), for: .horizontal)
        }

        @available(*, unavailable)
        required init?(coder: NSCoder) { fatalError() }

        @objc private func changed() {
            let ended = NSApp.currentEvent.map { $0.type == .leftMouseUp || $0.type == .keyDown } ?? true
            onChange(doubleValue, ended)
        }
    }
#endif
