#if os(macOS)
    import AppKit
    import MavioMac
    import MavioPlayer

    /// The player of a window of its own (docs/clients.md §4.4, §6.6): the
    /// picture, and over it while the pointer moves or the playback is paused
    /// a controller of the title, the timeline, play and pause, seeking, the
    /// next episode, the subtitles, the audio, Picture in Picture and full
    /// screen. Space and K play and pause, the arrows and J and L seek, F
    /// puts the window in full screen and Escape takes it out; a click plays
    /// and pauses, a double click puts the window in full screen.
    ///
    /// In television mode (docs/tv.md §7) the controller is the title, the
    /// timeline and a row of buttons — the next episode, the subtitles and
    /// the audio — whose pickers open as a side panel: down shows it, left
    /// and right seek by 10 seconds, select pauses and plays, and back closes
    /// the panel, then hides the controller, then closes the player.
    ///
    /// The playback stops, reporting where, as the page leaves its window.
    @MainActor
    final class PlayerPage: Page {
        /// The pickers of television mode's side panel, and of the desktop's
        /// menus.
        private enum Picker { case subtitles, audio }

        /// What television mode's focus is on: nothing, the picture taking
        /// the keys; a button of the controller's row; a choice of the panel.
        private enum Focus: Equatable {
            case picture
            case row(Int)
            case choice(Int)
        }

        private let close: @MainActor () -> Void
        private var request: PlayRequest
        private let player = Player()
        private var session: PlaybackSession? { didSet { startReporting() } }
        private var info: PlaybackInfo?
        private var source: Source?
        private var sources: [Source] = []
        private var itemTitle: String
        private var series: String?
        private var next: Episode?
        private var artwork: URL?
        private var audio: Int32?
        private var subtitle: Int32?
        private var failure: String?
        private var triedAether = false
        private var controls = true
        private var controllerShown = true
        private var menuOpen = false
        private var scrubbing: TimeInterval?
        private var stopped = false
        private var wasPlaying = false
        private var lastFailure: String?
        private var lastEnded = false
        private var panel: Picker?
        private var focus = Focus.picture

        private var loadTask: Task<Void, Never>?
        private var reportTask: Task<Void, Never>?
        private var hideTask: Task<Void, Never>?
        private var clickTask: Task<Void, Never>?
        private var stateObserver: Observer?
        private var timeObserver: Observer?

        // MARK: Views

        private let picture: PictureView
        private let spinner = NSProgressIndicator()
        private let controller = PassThroughView()
        private let statusHolder = PassThroughView()
        private let seriesLabel: NSTextField
        private let titleLabel: NSTextField
        // The desktop's controller.
        private let slider = NSSlider()
        private let elapsedLabel: NSTextField
        private let remainingLabel: NSTextField
        private let playButton: IconButton
        private let nextButton: IconButton
        private let subtitlesButton: IconButton
        private let audioButton: IconButton
        private let pictureInPictureButton: IconButton
        // Television mode's controller.
        private let timeline: TimelineBar
        private let tvRow: NSStackView
        private let tvNext: MavioButton
        private let tvSubtitles: MavioButton
        private let tvAudio: MavioButton
        private let panelView = NSVisualEffectView()
        private let panelPage: ScrollPage
        private var panelRows: [ChoiceRow] = []

        /// The player of request in context; close closes its window.
        init(context: MacContext, request: PlayRequest, close: @escaping @MainActor () -> Void) {
            self.close = close
            self.request = request
            itemTitle = request.title
            let theme = context.theme
            let tv = theme.isTV
            picture = PictureView(player: player)
            seriesLabel = .label("", theme.fonts.callout, color: .white.withAlphaComponent(0.7))
            titleLabel = .label(request.title, theme.fonts.title2, color: .white)
            if !tv { titleLabel.font = .systemFont(ofSize: theme.fonts.title2.size, weight: .bold) }
            let digits = NSFont.monospacedDigitSystemFont(ofSize: theme.fonts.footnote.size, weight: .regular)
            elapsedLabel = .label("0:00", theme.fonts.footnote, color: .white.withAlphaComponent(0.85))
            remainingLabel = .label("-0:00", theme.fonts.footnote, color: .white.withAlphaComponent(0.85))
            elapsedLabel.font = digits
            remainingLabel.font = digits
            playButton = IconButton("pause", MavioStrings.playerPause, size: 28) {}
            nextButton = IconButton("next", MavioStrings.playerNextEpisode) {}
            subtitlesButton = IconButton("subtitles", MavioStrings.playerSubtitles, fill: false) {}
            audioButton = IconButton("audio", MavioStrings.playerAudio, fill: false) {}
            pictureInPictureButton = IconButton("pictureInPicture", MavioStrings.playerPictureInPicture) {}
            timeline = TimelineBar(theme: theme)
            tvNext = MavioButton(MavioStrings.playerNextEpisode, icon: "next", kind: .artworkSecondary, theme: theme)
            tvSubtitles = MavioButton(
                MavioStrings.playerSubtitles, icon: "subtitles", kind: .artworkSecondary, theme: theme)
            tvAudio = MavioButton(MavioStrings.playerAudio, icon: "audio", kind: .artworkSecondary, theme: theme)
            tvRow = .horizontal([tvNext, tvSubtitles, tvAudio], spacing: MavioSpacing.s24)
            if tv, let assistant = context.assistant {
                tvRow.addArrangedSubview(
                    AssistantButton(
                        assistant: assistant, theme: theme, side: 64, size: 30, color: .white,
                        fill: NSColor.white.withAlphaComponent(0.15)))
            }
            panelPage = ScrollPage([], spacing: MavioSpacing.s8, insets: .all(MavioSpacing.s48))
            super.init(context: context)
        }

        isolated deinit { stop() }

        // MARK: Building

        override func loadView() {
            let root = PlayerRootView()
            root.frame = NSRect(x: 0, y: 0, width: 1280, height: 720)
            root.appearance = NSAppearance(named: .darkAqua)
            root.wantsLayer = true
            root.layer?.backgroundColor = .black
            root.onKey = { [weak self] in self?.key($0) ?? false }
            root.onMove = { [weak self] in
                guard let self, !self.theme.isTV else { return }
                self.poke()
            }
            root.onClick = { [weak self] in self?.clicked($0) }
            root.onWindow = { [weak self] window in
                guard let self else { return }
                if let window {
                    window.makeFirstResponder(self.view)
                } else {
                    // The window closed, or another video took its place.
                    self.stop()
                }
            }
            view = root
            root.addFilling(picture)
            spinner.style = .spinning
            spinner.controlSize = .large
            spinner.isDisplayedWhenStopped = false
            root.addSubview(spinner)
            spinner.center(in: root)
            root.addFilling(controller)
            if theme.isTV { buildTV() } else { buildDesktop() }
            controller.addFilling(statusHolder)
            if theme.isTV { buildPanel(in: root) }
        }

        private func buildDesktop() {
            controller.addFilling(
                ShadeView([(0, 0.5), (0.2, 0), (0.6, 0), (1, 0.8)]))
            let titles = NSStackView.vertical([seriesLabel, titleLabel], spacing: MavioSpacing.s2)
            slider.controlSize = .small
            slider.minValue = 0
            slider.maxValue = 1
            slider.isContinuous = true
            slider.refusesFirstResponder = true
            slider.trackFillColor = .white
            slider.target = self
            slider.action = #selector(scrub)
            slider.setAccessibilityLabel(MavioStrings.playerPosition)
            slider.setContentHuggingPriority(.init(1), for: .horizontal)
            slider.setContentCompressionResistancePriority(.init(1), for: .horizontal)
            for label in [elapsedLabel, remainingLabel] {
                label.setContentCompressionResistancePriority(.required, for: .horizontal)
                label.setContentHuggingPriority(.required, for: .horizontal)
            }
            let timeRow = NSStackView.horizontal([elapsedLabel, slider, remainingLabel], spacing: MavioSpacing.s12)
            let seekBack = IconButton("seekBack", MavioStrings.playerSeekBack(count: 10)) { [weak self] in
                self?.seek(by: -10)
            }
            let seekForward = IconButton("seekForward", MavioStrings.playerSeekForward(count: 10)) { [weak self] in
                self?.seek(by: 10)
            }
            let fullScreen = IconButton("fullScreen", MavioStrings.playerFullScreen) { [weak self] in
                self?.toggleFullScreen()
            }
            playButton.onPress = { [weak self] in self?.toggle() }
            nextButton.onPress = { [weak self] in
                guard let self, let next = self.next else { return }
                self.play(next)
            }
            subtitlesButton.onPress = { [weak self] in
                guard let self else { return }
                self.showMenu(.subtitles, from: self.subtitlesButton)
            }
            audioButton.onPress = { [weak self] in
                guard let self else { return }
                self.showMenu(.audio, from: self.audioButton)
            }
            pictureInPictureButton.onPress = { [weak self] in self?.player.togglePictureInPicture() }
            let buttons = NSStackView.horizontal(
                [
                    seekBack, playButton, seekForward, nextButton, Spacer(),
                ] + assistantButton() + [subtitlesButton, audioButton, pictureInPictureButton, fullScreen],
                spacing: MavioSpacing.s16)
            let bottom = NSStackView.vertical([titles, timeRow, buttons], spacing: MavioSpacing.s12)
            controller.addSubview(bottom)
            NSLayoutConstraint.activate([
                bottom.leadingAnchor.constraint(equalTo: controller.leadingAnchor, constant: MavioSpacing.s24),
                bottom.trailingAnchor.constraint(equalTo: controller.trailingAnchor, constant: -MavioSpacing.s24),
                bottom.bottomAnchor.constraint(equalTo: controller.bottomAnchor, constant: -MavioSpacing.s20),
                bottom.topAnchor.constraint(greaterThanOrEqualTo: controller.topAnchor, constant: MavioSpacing.s24),
                timeRow.widthAnchor.constraint(equalTo: bottom.widthAnchor),
                buttons.widthAnchor.constraint(equalTo: bottom.widthAnchor),
            ])
        }

        private func buildTV() {
            controller.addFilling(ShadeView([(0.5, 0), (1, 0.8)]))
            let titles = NSStackView.vertical([seriesLabel, titleLabel], spacing: MavioSpacing.s4)
            tvNext.onPress = { [weak self] in
                guard let self, let next = self.next else { return }
                self.play(next)
            }
            tvSubtitles.onPress = { [weak self] in self?.open(.subtitles) }
            tvAudio.onPress = { [weak self] in self?.open(.audio) }
            let bottom = NSStackView.vertical([titles, timeline, tvRow], spacing: MavioSpacing.s24)
            controller.addSubview(bottom)
            NSLayoutConstraint.activate([
                bottom.leadingAnchor.constraint(equalTo: controller.leadingAnchor, constant: theme.layout.margin),
                bottom.trailingAnchor.constraint(equalTo: controller.trailingAnchor, constant: -theme.layout.margin),
                bottom.bottomAnchor.constraint(equalTo: controller.bottomAnchor, constant: -60),
                bottom.topAnchor.constraint(greaterThanOrEqualTo: controller.topAnchor, constant: theme.layout.margin),
                timeline.widthAnchor.constraint(equalTo: bottom.widthAnchor),
            ])
        }

        private func buildPanel(in root: NSView) {
            panelView.material = .hudWindow
            panelView.blendingMode = .withinWindow
            panelView.state = .active
            panelView.translatesAutoresizingMaskIntoConstraints = false
            panelView.isHidden = true
            panelView.setAccessibilityLabel(MavioStrings.playerOptions)
            root.addSubview(panelView)
            panelView.addFilling(panelPage)
            NSLayoutConstraint.activate([
                panelView.topAnchor.constraint(equalTo: root.topAnchor),
                panelView.bottomAnchor.constraint(equalTo: root.bottomAnchor),
                panelView.trailingAnchor.constraint(equalTo: root.trailingAnchor),
                panelView.widthAnchor.constraint(equalToConstant: 620),
            ])
        }

        override func viewDidLoad() {
            super.viewDidLoad()
            updateDetails()
            stateObserver = Observer { [weak self] in self?.playerChanged() }
            timeObserver = Observer { [weak self] in self?.timeChanged() }
            loadTask = Task { [weak self] in
                guard let self else { return }
                await self.start(at: self.request.start)
            }
        }

        override func viewDidAppear() {
            super.viewDidAppear()
            view.window?.makeFirstResponder(view)
        }

        // MARK: Following the player

        /// Follows the player's state: the button, the spinner, the
        /// controller, and the failures and the end, as they come.
        private func playerChanged() {
            let playing = player.isPlaying
            let buffering = player.isBuffering
            let failed = player.failure
            let ended = player.ended
            pictureInPictureButton.isHidden = theme.isTV || !player.canPictureInPicture
            playButton.set(
                playing ? "pause" : "play", playing ? MavioStrings.playerPause : MavioStrings.playerPlay)
            if buffering && failure == nil { spinner.startAnimation(nil) } else { spinner.stopAnimation(nil) }
            if playing != wasPlaying {
                wasPlaying = playing
                updateController()
                scheduleHide()
            }
            if failed != lastFailure {
                lastFailure = failed
                if let failed { Task { [weak self] in self?.playerFailed(failed) } }
            }
            if ended != lastEnded {
                lastEnded = ended
                if ended { Task { [weak self] in self?.playerEnded() } }
            }
        }

        private func timeChanged() {
            let duration = player.duration
            let position = scrubbing ?? player.currentTime
            if theme.isTV {
                timeline.set(position: position, duration: duration)
                return
            }
            elapsedLabel.stringValue = clock(position)
            remainingLabel.stringValue = "-" + clock(max(duration - position, 0))
            guard scrubbing == nil else { return }
            slider.maxValue = max(duration, 1)
            slider.doubleValue = min(position, max(duration, 1))
        }

        private func playerFailed(_ message: String) {
            // AVPlayer refused a file the server let it play as it is:
            // AetherEngine plays it instead.
            if player.engine == .avPlayer, let info, !isHLS(info), !triedAether {
                triedAether = true
                Task { [weak self] in
                    guard let self else { return }
                    await self.load(info, start: self.player.currentTime, aether: true)
                }
            } else {
                setFailure(message)
            }
        }

        private func playerEnded() {
            guard !stopped else { return }
            let autoplayNext = UserDefaults.standard.object(forKey: DeviceSetting.autoplayNext) as? Bool ?? true
            if let next, autoplayNext { play(next) } else { close() }
        }

        /// Shows what the title, the next episode and the tracks are.
        private func updateDetails() {
            if let series, !series.isEmpty {
                seriesLabel.stringValue = series
                seriesLabel.isHidden = false
            } else {
                seriesLabel.isHidden = true
            }
            titleLabel.stringValue = itemTitle
            let hasSubtitles = !(info?.subtitles.isEmpty ?? true)
            let hasAudio = (source?.audio.count ?? 0) > 1
            nextButton.isHidden = next == nil
            subtitlesButton.isHidden = !hasSubtitles
            audioButton.isHidden = !hasAudio
            tvNext.isHidden = next == nil
            tvSubtitles.isHidden = !hasSubtitles
            tvAudio.isHidden = !hasAudio
            tvRow.isHidden = next == nil && !hasSubtitles && !hasAudio
            if case .row = focus, rowButtons.isEmpty { setFocus(.picture) }
        }

        private func setFailure(_ message: String?) {
            failure = message
            for view in statusHolder.subviews { view.removeFromSuperview() }
            if let message {
                let status = StatusView(
                    icon: "error", title: MavioStrings.stateError, message: message,
                    action: (
                        MavioStrings.actionRetry,
                        { [weak self] in
                            guard let self else { return }
                            self.loadTask = Task { [weak self] in
                                guard let self else { return }
                                await self.start(at: self.player.currentTime)
                            }
                        }
                    ), theme: theme)
                statusHolder.addSubview(status)
                NSLayoutConstraint.activate([
                    status.centerXAnchor.constraint(equalTo: statusHolder.centerXAnchor),
                    status.topAnchor.constraint(equalTo: statusHolder.topAnchor),
                    status.bottomAnchor.constraint(equalTo: statusHolder.bottomAnchor),
                    status.widthAnchor.constraint(lessThanOrEqualToConstant: 480),
                    status.widthAnchor.constraint(lessThanOrEqualTo: statusHolder.widthAnchor),
                ])
                let wide = status.widthAnchor.constraint(equalToConstant: 480)
                wide.priority = .defaultHigh
                wide.isActive = true
                spinner.stopAnimation(nil)
            }
            updateController()
            scheduleHide()
        }

        // MARK: The controller

        /// Shows the controller, and hides it again once nothing happens
        /// for a while.
        private func poke() {
            if !controls {
                controls = true
                updateController()
            }
            scheduleHide()
        }

        private func setControls(_ shown: Bool) {
            controls = shown
            if !shown, theme.isTV { setFocus(.picture) }
            updateController()
            scheduleHide()
        }

        private func updateController() {
            let visible = controls || failure != nil || (!theme.isTV && !player.isPlaying)
            guard visible != controllerShown else { return }
            controllerShown = visible
            if visible { controller.isHidden = false }
            nonisolated(unsafe) weak var page = self
            NSAnimationContext.runAnimationGroup { context in
                context.duration = MavioDuration.fast
                context.timingFunction = CAMediaTimingFunction(name: .easeOut)
                controller.animator().alphaValue = visible ? 1 : 0
            } completionHandler: {
                MainActor.assumeIsolated {
                    guard let page, !page.controllerShown else { return }
                    page.controller.isHidden = true
                }
            }
        }

        /// Hides the controller after a while of playing with nothing
        /// happening, unless a menu, the scrubber or the panel is in use.
        private func scheduleHide() {
            hideTask?.cancel()
            guard controls, player.isPlaying, !menuOpen, scrubbing == nil, failure == nil, panel == nil else {
                return
            }
            hideTask = Task { [weak self] in
                try? await Task.sleep(for: .seconds(MavioDuration.controlsHide))
                guard !Task.isCancelled, let self else { return }
                self.setControls(false)
                if !self.theme.isTV { NSCursor.setHiddenUntilMouseMoves(true) }
            }
        }

        // MARK: Keys and pointer

        private func key(_ event: NSEvent) -> Bool {
            if !event.modifierFlags.intersection([.command, .control, .option]).isEmpty { return false }
            return theme.isTV ? tvKey(event) : desktopKey(event)
        }

        private func desktopKey(_ event: NSEvent) -> Bool {
            switch event.keyCode {
            case 123: seek(by: -5)
            case 124: seek(by: 5)
            case 53:
                guard let window = view.window, window.styleMask.contains(.fullScreen) else { return false }
                window.toggleFullScreen(nil)
            default:
                switch event.charactersIgnoringModifiers?.lowercased() {
                case " ", "k": toggle()
                case "j": seek(by: -10)
                case "l": seek(by: 10)
                case "f": toggleFullScreen()
                default: return false
                }
            }
            return true
        }

        /// The remote's keys (docs/tv.md §7): the arrows, select (Return,
        /// Enter or Space) and back (Escape or Backspace).
        private func tvKey(_ event: NSEvent) -> Bool {
            switch event.keyCode {
            case 123, 124:
                let step = event.keyCode == 123 ? -1 : 1
                switch focus {
                case let .row(index):
                    let buttons = rowButtons
                    if buttons.indices.contains(index + step) { setFocus(.row(index + step)) }
                    poke()
                case .choice:
                    break
                case .picture:
                    seek(by: TimeInterval(10 * step))
                }
            case 125:
                switch focus {
                case .picture:
                    if controls, !rowButtons.isEmpty { setFocus(.row(0)) }
                    poke()
                case .row:
                    poke()
                case let .choice(index):
                    if panelRows.indices.contains(index + 1) { setFocus(.choice(index + 1)) }
                }
            case 126:
                switch focus {
                case .row:
                    setFocus(.picture)
                    poke()
                case let .choice(index):
                    if index > 0 { setFocus(.choice(index - 1)) }
                case .picture:
                    poke()
                }
            case 36, 76, 49:
                switch focus {
                case .picture: select()
                case let .row(index):
                    if rowButtons.indices.contains(index) { rowButtons[index].activate() }
                case let .choice(index):
                    if panelRows.indices.contains(index) { panelRows[index].activate() }
                }
            case 53, 51:
                back()
            default:
                return false
            }
            return true
        }

        /// Select: pausing shows the controller for as long as it is
        /// paused; playing on hides it.
        private func select() {
            player.toggle()
            if player.isPlaying {
                setControls(false)
            } else {
                poke()
            }
        }

        private func back() {
            if let open = panel {
                closePanel()
                setFocus(.row(rowButtons.firstIndex { $0 === (open == .audio ? tvAudio : tvSubtitles) } ?? 0))
            } else if controls, failure == nil {
                setControls(false)
            } else {
                close()
            }
        }

        /// The buttons of television mode's row that show.
        private var rowButtons: [MavioButton] {
            [tvNext, tvSubtitles, tvAudio].filter { !$0.isHidden }
        }

        private func setFocus(_ new: Focus) {
            for button in [tvNext, tvSubtitles, tvAudio] { button.isFocused = false }
            for row in panelRows { row.isFocused = false }
            focus = new
            switch new {
            case .picture: break
            case let .row(index):
                if rowButtons.indices.contains(index) { rowButtons[index].isFocused = true }
            case let .choice(index):
                if panelRows.indices.contains(index) {
                    panelRows[index].isFocused = true
                    panelRows[index].scrollToVisible(panelRows[index].bounds)
                }
            }
        }

        /// A click plays and pauses, once it is not a double click, which
        /// puts the window in full screen.
        private func clicked(_ count: Int) {
            clickTask?.cancel()
            if count >= 2 {
                toggleFullScreen()
                return
            }
            clickTask = Task { [weak self] in
                try? await Task.sleep(for: .seconds(NSEvent.doubleClickInterval))
                guard !Task.isCancelled else { return }
                self?.toggle()
            }
        }

        private func toggle() {
            player.toggle()
            poke()
        }

        private func seek(by seconds: TimeInterval) {
            Task { await player.skip(by: seconds) }
            poke()
        }

        private func toggleFullScreen() {
            view.window?.toggleFullScreen(nil)
        }

        /// The assistant button of the desktop's row, as the web UI's.
        private func assistantButton() -> [NSView] {
            guard let assistant = context.assistant else { return [] }
            return [AssistantButton(assistant: assistant, theme: theme, side: 36, size: 22, color: .white)]
        }

        @objc private func scrub(_ sender: NSSlider) {
            let target = sender.doubleValue
            scrubbing = target
            timeChanged()
            guard NSApp.currentEvent?.type == .leftMouseUp else {
                hideTask?.cancel()
                return
            }
            Task { [weak self] in
                guard let self else { return }
                await self.player.seek(to: target)
                self.scrubbing = nil
                self.timeChanged()
                self.scheduleHide()
            }
        }

        // MARK: Tracks

        /// The desktop's menu of the subtitles or the audio, under its
        /// button.
        private func showMenu(_ which: Picker, from button: NSButton) {
            let menu = NSMenu()
            switch which {
            case .subtitles:
                let current = info?.subtitleStreamIndex ?? -1
                menu.addItem(.sectionHeader(title: MavioStrings.playerSubtitles))
                menu.addItem(choice(MavioStrings.playerSubtitlesOff, current < 0) { $0.subtitle = -1 })
                for track in info?.subtitles ?? [] {
                    let label = trackLabel(track.language, track.title, track.format, track.streamIndex)
                    menu.addItem(choice(label, track.streamIndex == current) { $0.subtitle = track.streamIndex })
                }
            case .audio:
                let current = info?.audioStreamIndex ?? -1
                menu.addItem(.sectionHeader(title: MavioStrings.playerAudio))
                for stream in source?.audio ?? [] {
                    let label = trackLabel(stream.language, stream.title, stream.codec, stream.index)
                    menu.addItem(choice(label, stream.index == current) { $0.audio = stream.index })
                }
            }
            menuOpen = true
            hideTask?.cancel()
            menu.popUp(positioning: nil, at: NSPoint(x: 0, y: button.bounds.height + 4), in: button)
            menuOpen = false
            poke()
        }

        private func choice(_ label: String, _ checked: Bool, pick: @escaping @MainActor (PlayerPage) -> Void)
            -> NSMenuItem
        {
            let item = MenuAction(label) { [weak self] in
                guard let self else { return }
                pick(self)
                self.restart()
            }
            item.state = checked ? .on : .off
            return item
        }

        /// Opens television mode's side panel of a picker, focus on its
        /// current choice.
        private func open(_ which: Picker) {
            panel = which
            hideTask?.cancel()
            var rows: [ChoiceRow] = []
            var current = 0
            switch which {
            case .subtitles:
                let chosen = info?.subtitleStreamIndex ?? -1
                rows.append(panelChoice(MavioStrings.playerSubtitlesOff, nil, chosen < 0) { $0.subtitle = -1 })
                for track in info?.subtitles ?? [] {
                    if track.streamIndex == chosen { current = rows.count }
                    rows.append(
                        panelChoice(
                            language(track.language) ?? track.title ?? "#\(track.streamIndex)",
                            [track.title, track.format.uppercased()].compactMap(\.self).joined(separator: " · "),
                            track.streamIndex == chosen
                        ) { $0.subtitle = track.streamIndex })
                }
            case .audio:
                let chosen = info?.audioStreamIndex
                for stream in source?.audio ?? [] {
                    if stream.index == chosen { current = rows.count }
                    rows.append(
                        panelChoice(
                            language(stream.language) ?? stream.title ?? "#\(stream.index)",
                            [stream.title, stream.codec.uppercased()].compactMap(\.self).joined(separator: " · "),
                            stream.index == chosen
                        ) { $0.audio = stream.index })
                }
            }
            panelRows = rows
            let heading = NSTextField.label(
                which == .audio ? MavioStrings.playerAudio : MavioStrings.playerSubtitles, theme.fonts.title3,
                color: .white)
            panelPage.setViews([])
            panelPage.add(heading, fullWidth: false)
            panelPage.stack.setCustomSpacing(MavioSpacing.s16 + MavioSpacing.s8, after: heading)
            for row in rows { panelPage.add(row) }
            panelView.alphaValue = 0
            panelView.isHidden = false
            NSAnimationContext.runAnimationGroup { context in
                context.duration = MavioDuration.fast
                panelView.animator().alphaValue = 1
            }
            view.layoutSubtreeIfNeeded()
            setFocus(.choice(current))
        }

        private func closePanel() {
            panel = nil
            panelView.isHidden = true
            panelRows = []
            panelPage.setViews([])
            scheduleHide()
        }

        private func panelChoice(
            _ label: String, _ detail: String?, _ checked: Bool, pick: @escaping @MainActor (PlayerPage) -> Void
        ) -> ChoiceRow {
            let row = ChoiceRow(label, detail: detail, checked: checked, theme: theme)
            row.onPress = { [weak self] in
                guard let self else { return }
                pick(self)
                self.closePanel()
                self.setFocus(.picture)
                self.restart()
            }
            return row
        }

        private func trackLabel(_ code: String?, _ title: String?, _ format: String, _ index: Int32) -> String {
            let name = language(code) ?? title ?? "#\(index)"
            let detail = [code.flatMap(language) != nil ? title : nil, format.uppercased()].compactMap(\.self)
                .filter { !$0.isEmpty }
            return ([name] + detail).joined(separator: " · ")
        }

        private func language(_ code: String?) -> String? {
            guard let code, !code.isEmpty, code != "und" else { return nil }
            return Locale.current.localizedString(forLanguageCode: code) ?? code
        }

        // MARK: Playback

        private func isHLS(_ info: PlaybackInfo) -> Bool {
            info.url.range(of: #"\.m3u8(\?|$)"#, options: .regularExpression) != nil
        }

        /// Starts the playback on the server, at position, with the selected
        /// streams; a playback already going stops first. A downloaded file
        /// plays without the server.
        private func start(at position: TimeInterval?) async {
            setFailure(nil)
            if let file = request.file {
                await player.load(
                    Media(
                        url: file,
                        engine: PlayerEngine.choose(
                            hls: false, container: file.pathExtension.lowercased(), audioCodec: nil,
                            selectsTracks: false, avPlayerContainers: DeviceProfile.avPlayerContainers,
                            avPlayerAudioCodecs: DeviceProfile.avPlayerAudioCodecs),
                        start: position ?? 0))
                player.nowPlaying.show(NowPlaying.Item(title: itemTitle))
                return
            }
            if let old = session {
                session = nil
                try? await old.stop(position: player.currentTime)
            }
            if source == nil,
                let details = try? await model.client.details(serverId: request.serverId, itemId: request.itemId)
            {
                itemTitle = details.card.title
                if details.card.kind == .episode {
                    next = details.next
                    let number = details.episodeNumber.map {
                        MavioStrings.itemEpisodeShort(season: String(details.seasonNumber ?? 0), episode: String($0))
                    }
                    series = [details.series?.name, number].compactMap(\.self).joined(separator: " · ")
                }
                sources = details.sources.filter { $0.serverId == request.serverId && $0.itemId == request.itemId }
                source = sources.first { $0.mediaSourceId == request.mediaSourceId } ?? sources.first
                artwork = (details.card.thumb ?? details.card.poster).flatMap { URL(string: $0.url) }
                player.nowPlaying.next = next.map { episode in
                    { [weak self] in self?.play(episode) }
                }
                updateDetails()
            }
            let local = model.server(request.serverId).map { isLocalAddress(address: $0.address) } ?? false
            let quality =
                UserDefaults.standard.string(forKey: PlaybackQuality.key).flatMap(PlaybackQuality.init(rawValue:))
                ?? .auto
            do {
                let session = try await model.client.play(
                    serverId: request.serverId, itemId: request.itemId,
                    capabilities: DeviceProfile.capabilities(airPlay: false),
                    options: PlaybackOptions(
                        mediaSourceId: source?.mediaSourceId, audioStreamIndex: audio, subtitleStreamIndex: subtitle,
                        maxBitrate: quality.bitrate(local: local), startPosition: position, download: false))
                guard !stopped else {
                    try? await session.stop(position: position)
                    return
                }
                let info = session.info()
                self.session = session
                self.info = info
                if let match = sources.first(where: { $0.mediaSourceId == info.mediaSourceId }) { source = match }
                updateDetails()
                await load(info, start: info.startPosition ?? position ?? 0, aether: false)
                player.nowPlaying.show(NowPlaying.Item(title: itemTitle, subtitle: series, artwork: artwork))
                view.window?.title = itemTitle
            } catch is CancellationError {
            } catch {
                setFailure(error.reason)
            }
        }

        /// Plays what the server returned on the engine that suits it.
        private func load(_ info: PlaybackInfo, start: TimeInterval, aether: Bool) async {
            guard !stopped, let url = URL(string: info.url) else { return }
            let hls = isHLS(info)
            let audioStreams = source?.audio ?? []
            let defaultAudio = (audioStreams.first(where: \.default) ?? audioStreams.first)?.index
            let selectedAudio = info.audioStreamIndex
            let sub = info.subtitleStreamIndex ?? -1
            let track = info.subtitles.first { $0.streamIndex == sub }
            let engine =
                aether
                ? PlayerEngine.aether
                : PlayerEngine.choose(
                    hls: hls, subtitleFile: track?.url != nil, container: source?.container,
                    audioCodec: audioStreams.first { $0.index == selectedAudio ?? defaultAudio ?? -1 }?.codec,
                    selectsTracks: sub >= 0 || (selectedAudio != nil && selectedAudio != defaultAudio),
                    avPlayerContainers: DeviceProfile.avPlayerContainers,
                    avPlayerAudioCodecs: DeviceProfile.avPlayerAudioCodecs)
            let onAether = engine == .aether
            await player.load(
                Media(
                    url: url,
                    engine: engine,
                    start: start,
                    audioStream: onAether ? selectedAudio.map(Int.init) : nil,
                    subtitleStream: onAether && sub >= 0 && track?.url == nil ? Int(sub) : nil,
                    subtitleFile: onAether
                        ? track?.url.flatMap(URL.init(string:)).map {
                            ExternalSubtitle(url: $0, language: track?.language, title: track?.title)
                        } : nil,
                    subtitleRendition: hls && sub >= 0 && track?.url == nil ? (track?.language ?? "und") : nil
                ))
        }

        /// Reports the position while the playback lasts; the core sends it
        /// every ten seconds and on pause and seek.
        private func startReporting() {
            reportTask?.cancel()
            guard let session, !stopped else {
                reportTask = nil
                return
            }
            let player = player
            reportTask = Task {
                while !Task.isCancelled {
                    try? await session.update(position: player.currentTime, paused: !player.isPlaying)
                    try? await Task.sleep(for: .seconds(1))
                }
            }
        }

        /// Starts again where it is with the tracks chosen.
        private func restart() {
            let position = player.currentTime
            let playing = player.isPlaying
            triedAether = false
            loadTask = Task { [weak self] in
                guard let self else { return }
                await self.start(at: position)
                // A paused video stays paused across a change of track.
                if !playing { self.player.pause() }
            }
        }

        /// Plays the episode after this one in the player's place.
        private func play(_ episode: Episode) {
            guard !stopped else { return }
            let position = player.ended ? nil : player.currentTime
            if let old = session {
                session = nil
                let serverId = request.serverId
                let model = model
                Task {
                    try? await old.stop(position: position)
                    model.bump(serverId)
                }
            }
            request = PlayRequest(serverId: request.serverId, itemId: episode.card.itemId, title: episode.card.title)
            itemTitle = episode.card.title
            (info, source, sources, audio, subtitle, next, series) = (nil, nil, [], nil, nil, nil, nil)
            triedAether = false
            if panel != nil { closePanel() }
            setFocus(.picture)
            updateDetails()
            loadTask = Task { [weak self] in await self?.start(at: episode.position) }
        }

        /// Stops the playback, reporting where it stopped.
        private func stop() {
            guard !stopped else { return }
            stopped = true
            for task in [loadTask, reportTask, hideTask, clickTask] { task?.cancel() }
            stateObserver?.cancel()
            timeObserver?.cancel()
            let position = player.ended ? nil : player.currentTime
            player.stop()
            picture.release()
            let session = session
            let serverId = request.serverId
            let model = model
            Task {
                try? await session?.stop(position: position)
                model.bump(serverId)
            }
        }
    }

    /// The player's root view: it takes the keys, and the clicks and the
    /// pointer's moves that nothing over the picture takes; a drag moves the
    /// window.
    @MainActor
    private final class PlayerRootView: FlippedView {
        /// Handles a key, returning whether it did.
        var onKey: (@MainActor (NSEvent) -> Bool)?
        /// The pointer moved.
        var onMove: (@MainActor () -> Void)?
        /// A click, of its count.
        var onClick: (@MainActor (Int) -> Void)?
        /// The view moved into a window, or out of one.
        var onWindow: (@MainActor (NSWindow?) -> Void)?
        private var tracking: NSTrackingArea?
        private var down: NSEvent?
        private var dragged = false
        private var wasInWindow = false

        override var acceptsFirstResponder: Bool { true }
        override var mouseDownCanMoveWindow: Bool { false }

        override func keyDown(with event: NSEvent) {
            if onKey?(event) != true { super.keyDown(with: event) }
        }

        override func updateTrackingAreas() {
            super.updateTrackingAreas()
            if let tracking { removeTrackingArea(tracking) }
            let area = NSTrackingArea(
                rect: bounds, options: [.mouseMoved, .mouseEnteredAndExited, .activeInKeyWindow, .inVisibleRect],
                owner: self)
            addTrackingArea(area)
            tracking = area
        }

        override func mouseMoved(with event: NSEvent) { onMove?() }
        override func mouseEntered(with event: NSEvent) { onMove?() }

        override func mouseDown(with event: NSEvent) {
            window?.makeFirstResponder(self)
            down = event
            dragged = false
        }

        override func mouseDragged(with event: NSEvent) {
            guard !dragged, let down else { return }
            dragged = true
            window?.performDrag(with: down)
        }

        override func mouseUp(with event: NSEvent) {
            defer { down = nil }
            guard !dragged else { return }
            onClick?(event.clickCount)
        }

        override func viewDidMoveToWindow() {
            super.viewDidMoveToWindow()
            if window != nil {
                wasInWindow = true
                onWindow?(window)
            } else if wasInWindow {
                onWindow?(nil)
            }
        }
    }

    extension PlayerPage: AssistantPlayer {
        var assistantServerId: String { request.serverId }

        var assistantPlaying: Mavio_Assistant_V1_AskPlaying {
            Assistant.playing(
                itemId: request.itemId, position: player.currentTime, paused: !player.isPlaying,
                speed: Double(player.rate), audio: info?.audioStreamIndex, subtitle: info?.subtitleStreamIndex)
        }

        func assistantControl(_ command: Mavio_Assistant_V1_ControlCommand, value: Double) -> Bool {
            switch command {
            case .pause: player.pause()
            case .resume: player.play()
            case .stop: close()
            case .seek: Task { await player.seek(to: value) }
            case .seekBy: seek(by: value)
            case .speed:
                guard value > 0 else { return false }
                player.setRate(Float(value))
            case .next:
                guard let next else { return false }
                play(next)
            case .fullscreen:
                guard let window = view.window else { return false }
                if !window.styleMask.contains(.fullScreen) { window.toggleFullScreen(nil) }
            default: return false
            }
            poke()
            return true
        }

        func assistantStreams(audio: Int32?, subtitle: Int32?) -> Bool {
            guard info != nil else { return false }
            if let audio { self.audio = audio }
            if let subtitle { self.subtitle = subtitle }
            restart()
            return true
        }
    }
#endif
