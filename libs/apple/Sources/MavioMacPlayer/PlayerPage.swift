#if os(macOS)
    import AppKit
    import MavioMac
    import MavioPlayer

    /// The player of a window of its own (docs/clients.md §4.4, §6.6): the
    /// picture, and over it while the pointer moves or the playback is paused
    /// a controller of the title, the timeline, play and pause, seeking, the
    /// next episode, the Watch Together group's chip while the player
    /// follows one, the subtitles, the audio, Picture in Picture and full
    /// screen, with the volume, the speed, and the window floating on top.
    /// Space and K play and pause, left and right and J and L seek, up and
    /// down set the volume and M mutes, [ and ] change the speed and \
    /// resets it, Z and X move the subtitles, S saves a screenshot in the
    /// Pictures folder and ⇧S copies it, F puts the window in full screen
    /// and Escape takes it out; a click plays and pauses, a double click
    /// puts the window in full screen, the wheel and trackpad seek and set
    /// the volume, and a pinch resizes the window. The window keeps the
    /// video's aspect, and a paused picture's text can be selected.
    ///
    /// In television mode (docs/tv.md §7) the controller is the title, the
    /// timeline and a row of buttons — the group's chip, the next episode,
    /// the subtitles and the audio — whose pickers open as a side panel: down shows it, left
    /// and right seek by 10 seconds, select pauses and plays, and back closes
    /// the panel, then hides the controller, then closes the player.
    ///
    /// The playback stops, reporting where, as the page leaves its window.
    @MainActor
    final class PlayerPage: Page {
        /// The pickers of television mode's side panel, and of the desktop's
        /// menus.
        private enum Picker { case subtitles, audio, speed }

        /// What a gesture of the wheel or trackpad changes.
        private enum ScrollAxis { case seek, volume }

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
        /// Whether a drag of the seek bar paused what played, which plays
        /// on as it ends.
        private var resumeAfterScrub = false
        /// What a gesture of the wheel or trackpad changes, taken at its
        /// start.
        private var scrollAxis: ScrollAxis?
        /// Where the wheel or trackpad seeks to.
        private var scrollTarget: TimeInterval?
        /// Whether the wheel's or trackpad's seek is on its way.
        private var scrollSeeking = false
        /// Whether the gesture seeking has ended, for the playback to go on.
        private var scrollEnded = true
        /// Whether a trackpad's seek paused what played, which plays on as
        /// it ends.
        private var resumeAfterScroll = false
        /// The video's size the window was last fitted to.
        private var fittedSize = CGSize.zero
        /// How far a pinch has gone in full screen, and whether it entered
        /// or left full screen already.
        private var pinch: CGFloat = 0
        private var pinchDone = false
        private var stopped = false
        private var wasPlaying = false
        private var wasBuffering = false
        private var lastFailure: String?
        private var lastEnded = false
        private var panel: Picker?
        private var focus = Focus.picture
        /// Follows the Watch Together group while it plays the item.
        private var follower: GroupFollower?
        private var groupObserver: Observer?

        private var loadTask: Task<Void, Never>?
        private var reportTask: Task<Void, Never>?
        private var hideTask: Task<Void, Never>?
        private var clickTask: Task<Void, Never>?
        private var previewTask: Task<Void, Never>?
        private var stateObserver: Observer?
        private var timeObserver: Observer?

        // MARK: Views

        private let picture: PictureView
        private let liveText = LiveTextView()
        /// Where the paused picture's text was looked for, nil while it plays.
        private var liveTextAt: TimeInterval?
        private let spinner = NSProgressIndicator()
        private let controller = PassThroughView()
        private let statusHolder = PassThroughView()
        private let seriesLabel: NSTextField
        private let titleLabel: NSTextField
        // The desktop's controller.
        private let seekBar = SeekBar()
        private let preview: ScrubPreview
        private let notice: PlayerNotice
        private let muteButton: IconButton
        private let speedButton: IconButton
        private let floatButton: IconButton
        private let volumeSlider = NSSlider()
        /// The controls the pointer keeps showing while it is over them.
        private var controlsArea: NSView?
        private var previewLeading: NSLayoutConstraint?
        private var previewAbove: NSLayoutConstraint?
        private var previewBelow: NSLayoutConstraint?
        private let elapsedLabel: NSTextField
        private let remainingLabel: NSTextField
        private let playButton: IconButton
        private let nextButton: IconButton
        private let groupButton: MavioButton
        private let subtitlesButton: IconButton
        private let audioButton: IconButton
        private let pictureInPictureButton: IconButton
        // Television mode's controller.
        private let timeline: TimelineBar
        private let tvRow: NSStackView
        private let tvGroup: MavioButton
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
            groupButton = MavioButton("", icon: "users", kind: .artworkSecondary, theme: theme)
            subtitlesButton = IconButton("subtitles", MavioStrings.playerSubtitles, fill: false) {}
            audioButton = IconButton("audio", MavioStrings.playerAudio, fill: false) {}
            pictureInPictureButton = IconButton("pictureInPicture", MavioStrings.playerPictureInPicture) {}
            preview = ScrubPreview(theme: theme)
            notice = PlayerNotice(theme: theme)
            muteButton = IconButton("volume", MavioStrings.playerMute) {}
            speedButton = IconButton("speed", MavioStrings.playerSpeed, fill: false) {}
            floatButton = IconButton("pin", MavioStrings.playerFloatOnTop, fill: false) {}
            timeline = TimelineBar(theme: theme)
            tvGroup = MavioButton("", icon: "users", kind: .artworkSecondary, theme: theme)
            tvNext = MavioButton(MavioStrings.playerNextEpisode, icon: "next", kind: .artworkSecondary, theme: theme)
            tvSubtitles = MavioButton(
                MavioStrings.playerSubtitles, icon: "subtitles", kind: .artworkSecondary, theme: theme)
            tvAudio = MavioButton(MavioStrings.playerAudio, icon: "audio", kind: .artworkSecondary, theme: theme)
            tvRow = .horizontal([tvGroup, tvNext, tvSubtitles, tvAudio], spacing: MavioSpacing.s24)
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
            root.onExit = { [weak self] in
                guard let self, !self.theme.isTV else { return }
                self.pointerLeft()
            }
            root.onMagnify = { [weak self] in
                guard let self, !self.theme.isTV else { return }
                self.pinched($0)
            }
            root.onScroll = { [weak self] in
                guard let self, !self.theme.isTV else { return }
                self.scrolled($0)
            }
            root.onWindow = { [weak self] window in
                guard let self else { return }
                if let window {
                    window.makeFirstResponder(self.view)
                    if !self.theme.isTV { self.applyFloating() }
                } else {
                    // The window closed, or another video took its place.
                    self.stop()
                }
            }
            view = root
            root.addFilling(picture)
            if !theme.isTV, LiveTextView.isSupported { root.addFilling(liveText) }
            spinner.style = .spinning
            spinner.controlSize = .large
            spinner.isDisplayedWhenStopped = false
            root.addSubview(spinner)
            spinner.center(in: root)
            root.addFilling(controller)
            if theme.isTV { buildTV() } else { buildDesktop() }
            controller.addFilling(statusHolder)
            if theme.isTV { buildPanel(in: root) }
            root.addSubview(notice)
            NSLayoutConstraint.activate([
                notice.topAnchor.constraint(equalTo: root.topAnchor, constant: 40),
                notice.leadingAnchor.constraint(equalTo: root.leadingAnchor, constant: MavioSpacing.s20),
            ])
        }

        private func buildDesktop() {
            controller.addFilling(
                ShadeView([(0, 0.5), (0.2, 0), (0.6, 0), (1, 0.8)]))
            let titles = NSStackView.vertical([seriesLabel, titleLabel], spacing: MavioSpacing.s2)
            seekBar.onHover = { [weak self] in self?.hover($0, at: $1) }
            seekBar.onScrubStart = { [weak self] in self?.scrubStarted() }
            seekBar.onScrub = { [weak self] in
                self?.scrubbing = $0
                self?.timeChanged()
            }
            seekBar.onScrubEnd = { [weak self] in self?.scrubEnded(at: $0) }
            for label in [elapsedLabel, remainingLabel] {
                label.setContentCompressionResistancePriority(.required, for: .horizontal)
                label.setContentHuggingPriority(.required, for: .horizontal)
            }
            let timeRow = NSStackView.horizontal([elapsedLabel, seekBar, remainingLabel], spacing: MavioSpacing.s12)
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
            nextButton.onPress = { [weak self] in self?.playNext() }
            groupButton.onPress = { [weak self] in self?.openGroup() }
            subtitlesButton.onPress = { [weak self] in
                guard let self else { return }
                self.showMenu(.subtitles, from: self.subtitlesButton)
            }
            audioButton.onPress = { [weak self] in
                guard let self else { return }
                self.showMenu(.audio, from: self.audioButton)
            }
            pictureInPictureButton.onPress = { [weak self] in self?.player.togglePictureInPicture() }
            muteButton.onPress = { [weak self] in self?.toggleMute() }
            speedButton.onPress = { [weak self] in
                guard let self else { return }
                self.showMenu(.speed, from: self.speedButton)
            }
            floatButton.onPress = { [weak self] in self?.toggleFloating() }
            volumeSlider.controlSize = .small
            volumeSlider.minValue = 0
            volumeSlider.maxValue = 1
            volumeSlider.isContinuous = true
            volumeSlider.refusesFirstResponder = true
            volumeSlider.trackFillColor = .white
            volumeSlider.target = self
            volumeSlider.action = #selector(volumeSlid)
            volumeSlider.setAccessibilityLabel(MavioStrings.playerVolume)
            volumeSlider.widthAnchor.constraint(equalToConstant: 80).isActive = true
            let buttons = NSStackView.horizontal(
                [
                    seekBack, playButton, seekForward, muteButton, volumeSlider, nextButton, groupButton, Spacer(),
                ] + assistantButton() + [
                    speedButton, subtitlesButton, audioButton, floatButton, pictureInPictureButton, fullScreen,
                ],
                spacing: MavioSpacing.s16)
            let bottom = NSStackView.vertical([titles, timeRow, buttons], spacing: MavioSpacing.s12)
            controlsArea = bottom
            controller.addSubview(bottom)
            NSLayoutConstraint.activate([
                bottom.leadingAnchor.constraint(equalTo: controller.leadingAnchor, constant: MavioSpacing.s24),
                bottom.trailingAnchor.constraint(equalTo: controller.trailingAnchor, constant: -MavioSpacing.s24),
                bottom.bottomAnchor.constraint(equalTo: controller.bottomAnchor, constant: -MavioSpacing.s20),
                bottom.topAnchor.constraint(greaterThanOrEqualTo: controller.topAnchor, constant: MavioSpacing.s24),
                timeRow.widthAnchor.constraint(equalTo: bottom.widthAnchor),
                buttons.widthAnchor.constraint(equalTo: bottom.widthAnchor),
            ])
            controller.addSubview(preview)
            previewLeading = preview.leadingAnchor.constraint(equalTo: controller.leadingAnchor)
            previewAbove = preview.bottomAnchor.constraint(equalTo: seekBar.topAnchor, constant: -MavioSpacing.s8)
            previewBelow = preview.topAnchor.constraint(equalTo: seekBar.bottomAnchor, constant: MavioSpacing.s8)
            previewLeading?.isActive = true
            previewAbove?.isActive = true
        }

        private func buildTV() {
            controller.addFilling(ShadeView([(0.5, 0), (1, 0.8)]))
            let titles = NSStackView.vertical([seriesLabel, titleLabel], spacing: MavioSpacing.s4)
            tvNext.onPress = { [weak self] in self?.playNext() }
            tvGroup.onPress = { [weak self] in self?.openGroup() }
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
            let defaults = UserDefaults.standard
            player.setVolume(defaults.object(forKey: DeviceSetting.playerVolume) as? Float ?? 1)
            player.setMuted(defaults.bool(forKey: DeviceSetting.playerMuted))
            player.setAudioOffset(defaults.double(forKey: DeviceSetting.audioOffset))
            updateVolume()
            updateDetails()
            stateObserver = Observer { [weak self] in self?.playerChanged() }
            groupObserver = Observer { [weak self] in
                guard let self else { return }
                _ = self.follower?.group
                self.updateDetails()
            }
            timeObserver = Observer { [weak self] in self?.timeChanged() }
            loadTask = Task { [weak self] in
                guard let self else { return }
                await self.start(at: self.request.start)
            }
        }

        override func viewDidLayout() {
            super.viewDidLayout()
            liftSubtitles()
        }

        override func viewDidAppear() {
            super.viewDidAppear()
            view.window?.makeFirstResponder(view)
        }

        // MARK: Following the player

        /// Follows the player's state: the button, the spinner, the
        /// controller, and the failures and the end, as they come.
        private func playerChanged() {
            let size = player.videoSize
            if size.width > 0, size.height > 0, size != fittedSize {
                Task { [weak self] in self?.fitWindow(to: size) }
            }
            let playing = player.isPlaying
            let buffering = player.isBuffering
            let failed = player.failure
            let ended = player.ended
            pictureInPictureButton.isHidden = theme.isTV || !player.canPictureInPicture
            playButton.set(
                playing ? "pause" : "play", playing ? MavioStrings.playerPause : MavioStrings.playerPlay)
            if buffering && failure == nil { spinner.startAnimation(nil) } else { spinner.stopAnimation(nil) }
            if playing != wasPlaying || buffering != wasBuffering {
                wasBuffering = buffering
                Task { [weak self] in
                    guard let self else { return }
                    self.updateLiveText(at: self.player.currentTime)
                }
            }
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
            updateLiveText(at: position)
            // The time is told only while the controller shows it.
            guard controllerShown else { return }
            if theme.isTV {
                timeline.set(position: position, duration: duration)
                return
            }
            elapsedLabel.stringValue = clock(position)
            remainingLabel.stringValue = "-" + clock(max(duration - position, 0))
            guard scrubbing == nil else { return }
            seekBar.set(position: position, duration: duration)
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
            // The group moves on at the end.
            guard follower?.group == nil else { return }
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
            let group = follower?.group
            nextButton.isHidden = !hasNext
            subtitlesButton.isHidden = !hasSubtitles
            audioButton.isHidden = !hasAudio
            groupButton.isHidden = group == nil
            tvGroup.isHidden = group == nil
            if let group {
                groupButton.title = groupChip(group)
                tvGroup.title = groupChip(group)
            }
            tvNext.isHidden = !hasNext
            tvSubtitles.isHidden = !hasSubtitles
            tvAudio.isHidden = !hasAudio
            tvRow.isHidden = group == nil && !hasNext && !hasSubtitles && !hasAudio
            seekBar.set(chapters: source?.chapters.map(\.start) ?? [])
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
            if visible {
                controller.isHidden = false
                timeChanged()
            }
            liftSubtitles()
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
                if self.pointerOverControls {
                    self.scheduleHide()
                    return
                }
                self.setControls(false)
                if !self.theme.isTV { NSCursor.setHiddenUntilMouseMoves(true) }
            }
        }

        /// Keeps text subtitles above the controls while they show.
        private func liftSubtitles() {
            guard !theme.isTV, let area = controlsArea else { return }
            let top = picture.convert(area.bounds, from: area).minY
            picture.subtitleLift = controllerShown ? picture.bounds.height - top + MavioSpacing.s8 : 0
        }

        /// Whether the pointer is over the controls, which stay while it is.
        private var pointerOverControls: Bool {
            guard !theme.isTV, let area = controlsArea, let window = view.window, window.isKeyWindow else {
                return false
            }
            let point = area.convert(window.mouseLocationOutsideOfEventStream, from: nil)
            return area.bounds.contains(point)
        }

        /// The pointer left the window: the controls hide at once while it
        /// plays.
        private func pointerLeft() {
            guard controls, player.isPlaying, !menuOpen, scrubbing == nil, failure == nil else { return }
            hideTask?.cancel()
            setControls(false)
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
            case 126: changeVolume(by: 0.05)
            case 125: changeVolume(by: -0.05)
            case 53:
                guard let window = view.window, window.styleMask.contains(.fullScreen) else { return false }
                window.toggleFullScreen(nil)
            default:
                switch event.charactersIgnoringModifiers?.lowercased() {
                case " ", "k": toggle()
                case "j": seek(by: -10)
                case "l": seek(by: 10)
                case "f": toggleFullScreen()
                case "m": toggleMute()
                case "s": screenshot(copy: event.modifierFlags.contains(.shift))
                case "[": stepRate(up: false)
                case "]": stepRate(up: true)
                case "\\": setRate(1)
                case "z" where player.canOffset: offsetSubtitles(to: player.subtitleOffset - 0.1)
                case "x" where player.canOffset: offsetSubtitles(to: player.subtitleOffset + 0.1)
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
            [tvGroup, tvNext, tvSubtitles, tvAudio].filter { !$0.isHidden }
        }

        private func setFocus(_ new: Focus) {
            for button in [tvGroup, tvNext, tvSubtitles, tvAudio] { button.isFocused = false }
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
                guard !Task.isCancelled, let self else { return }
                self.toggle()
                // The click shows the pointer no more than a move would.
                if !self.controls { NSCursor.setHiddenUntilMouseMoves(true) }
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

        // MARK: Offsets, speed and floating

        /// Shows the subtitles seconds later, or earlier.
        private func offsetSubtitles(to seconds: TimeInterval) {
            player.subtitleOffset = (seconds * 10).rounded() / 10
            notice.show("subtitles", "\(MavioStrings.playerSubtitleOffset) \(Self.offsetLabel(player.subtitleOffset))")
        }

        /// Has the sound come seconds later, or earlier, kept for the
        /// next videos as it is the setup's.
        private func offsetAudio(to seconds: TimeInterval) {
            let offset = (seconds * 10).rounded() / 10
            player.setAudioOffset(offset)
            UserDefaults.standard.set(offset, forKey: DeviceSetting.audioOffset)
            notice.show("audio", "\(MavioStrings.playerAudioOffset) \(Self.offsetLabel(offset))")
        }

        private func setRate(_ rate: Float) {
            guard !grouped else { return }
            player.setRate(min(max(rate, 0.25), 4))
            notice.show("speed", Self.rateLabel(player.rate))
        }

        /// Moves the speed to the next of the menu's speeds up or down.
        private func stepRate(up: Bool) {
            let rates: [Float] = [0.5, 0.75, 1, 1.25, 1.5, 1.75, 2]
            let rate = player.rate
            let next = up ? rates.first { $0 > rate + 0.01 } : rates.last { $0 < rate - 0.01 }
            setRate(next ?? rate)
        }

        /// Keeps the window above the others, or no longer.
        private func toggleFloating() {
            let floats = !UserDefaults.standard.bool(forKey: DeviceSetting.playerFloats)
            UserDefaults.standard.set(floats, forKey: DeviceSetting.playerFloats)
            applyFloating()
        }

        private func applyFloating() {
            let floats = UserDefaults.standard.bool(forKey: DeviceSetting.playerFloats)
            view.window?.level = floats ? .floating : .normal
            floatButton.set("pin", MavioStrings.playerFloatOnTop, fill: floats)
        }

        private static func offsetLabel(_ seconds: TimeInterval) -> String {
            seconds == 0 ? "0 s" : String(format: "%+.1f s", seconds)
        }

        private static func rateLabel(_ rate: Float) -> String {
            rate == 1 ? "1×" : "\(rate.formatted(.number.precision(.fractionLength(0...2))))×"
        }

        // MARK: Live Text and screenshots

        /// Looks for the text of the picture once it stays paused at a
        /// place, and lets it go as it plays or moves.
        private func updateLiveText(at position: TimeInterval) {
            guard !theme.isTV, LiveTextView.isSupported else { return }
            guard !player.isPlaying, !player.isBuffering, player.duration > 0, scrubbing == nil else {
                if liveTextAt != nil {
                    liveTextAt = nil
                    liveText.clear()
                }
                return
            }
            if let at = liveTextAt, abs(at - position) < 0.05 { return }
            liveTextAt = position
            liveText.analyze { [weak self] in await self?.player.frame() }
        }

        /// Saves the picture in the Pictures folder, or copies it.
        private func screenshot(copy: Bool) {
            let name = "\(itemTitle) \(clock(player.currentTime))"
            Task { [weak self] in
                guard let self, let image = await self.player.frame() else { return }
                if copy {
                    NSPasteboard.general.clearContents()
                    NSPasteboard.general.writeObjects([NSImage(cgImage: image, size: .zero)])
                    self.notice.show("copy", MavioStrings.playerScreenshotCopied)
                    return
                }
                guard let data = NSBitmapImageRep(cgImage: image).representation(using: .png, properties: [:]),
                    let folder = FileManager.default.urls(for: .picturesDirectory, in: .userDomainMask).first
                else { return }
                let base = name.replacing("/", with: "-").replacing(":", with: ".")
                var url = folder.appending(path: base + ".png")
                var n = 2
                while FileManager.default.fileExists(atPath: url.path(percentEncoded: false)) {
                    url = folder.appending(path: "\(base) \(n).png")
                    n += 1
                }
                do {
                    try data.write(to: url)
                    self.notice.show("photos", MavioStrings.playerScreenshotSaved)
                } catch {
                    self.notice.show("error", error.localizedDescription)
                }
            }
        }

        // MARK: The window

        /// Locks the window to the video's aspect and sizes it: at a pixel
        /// a pixel for its first video, keeping its area for the next,
        /// about its center and within the screen's visible frame.
        private func fitWindow(to size: CGSize) {
            guard let window = view.window, !window.styleMask.contains(.fullScreen) else { return }
            let first = fittedSize == .zero
            fittedSize = size
            window.contentAspectRatio = size
            let current = window.contentRect(forFrameRect: window.frame).size
            var target: CGSize
            if first {
                target = window.convertFromBacking(NSRect(origin: .zero, size: size)).size
            } else {
                let scale = (current.width * current.height / (size.width * size.height)).squareRoot()
                target = CGSize(width: size.width * scale, height: size.height * scale)
            }
            resize(window, to: target)
        }

        /// Resizes the window's content about its center to size, of the
        /// video's aspect, no smaller than the window allows and no larger
        /// than the screen's visible frame.
        private func resize(_ window: NSWindow, to size: CGSize) {
            guard size.width > 0, size.height > 0 else { return }
            var target = size
            if let visible = (window.screen ?? NSScreen.main)?.visibleFrame {
                let most = window.contentRect(forFrameRect: visible).size
                let shrink = min(1, most.width / target.width, most.height / target.height)
                target = CGSize(width: target.width * shrink, height: target.height * shrink)
            }
            let least = window.contentMinSize
            let grow = max(1, least.width / target.width, least.height / target.height)
            target = CGSize(width: (target.width * grow).rounded(), height: (target.height * grow).rounded())
            let old = window.frame
            var frame = window.frameRect(forContentRect: NSRect(origin: .zero, size: target))
            frame.origin = NSPoint(x: old.midX - frame.width / 2, y: old.midY - frame.height / 2)
            if let visible = (window.screen ?? NSScreen.main)?.visibleFrame {
                frame.origin.x = min(max(frame.minX, visible.minX), visible.maxX - frame.width)
                frame.origin.y = min(max(frame.minY, visible.minY), visible.maxY - frame.height)
            }
            window.setFrame(frame, display: true)
        }

        /// A pinch resizes the window about its center; pinching out past
        /// the screen enters full screen, and pinching in full screen
        /// leaves it.
        private func pinched(_ event: NSEvent) {
            guard let window = view.window else { return }
            if event.phase.contains(.began) {
                pinch = 0
                pinchDone = false
            }
            guard !pinchDone, event.phase.intersection([.ended, .cancelled]).isEmpty else { return }
            if window.styleMask.contains(.fullScreen) {
                pinch += event.magnification
                if pinch < -0.15 {
                    pinchDone = true
                    window.toggleFullScreen(nil)
                }
                return
            }
            let current = window.contentRect(forFrameRect: window.frame).size
            let factor = 1 + event.magnification
            let target = CGSize(width: current.width * factor, height: current.height * factor)
            if event.magnification > 0, let visible = window.screen?.visibleFrame {
                let most = window.contentRect(forFrameRect: visible).size
                if current.width >= most.width - 1 || current.height >= most.height - 1 {
                    pinch += event.magnification
                    if pinch > 0.15 {
                        pinchDone = true
                        window.toggleFullScreen(nil)
                    }
                    return
                }
            }
            resize(window, to: target)
        }

        // MARK: The volume

        /// Changes the volume by an amount, unmuting, with a tap of the
        /// trackpad as it reaches all of it.
        private func changeVolume(by amount: Float) {
            let old = player.isMuted ? 0 : player.volume
            let new = min(max(old + amount, 0), 1)
            player.setVolume(new)
            if new == 1, old < 1 {
                NSHapticFeedbackManager.defaultPerformer.perform(.levelChange, performanceTime: .now)
            }
            volumeChanged()
        }

        private func toggleMute() {
            player.setMuted(!player.isMuted)
            volumeChanged()
        }

        @objc private func volumeSlid(_ sender: NSSlider) {
            player.setVolume(Float(sender.doubleValue))
            volumeChanged()
        }

        /// Keeps the volume for the next videos and tells it.
        private func volumeChanged() {
            UserDefaults.standard.set(player.volume, forKey: DeviceSetting.playerVolume)
            UserDefaults.standard.set(player.isMuted, forKey: DeviceSetting.playerMuted)
            updateVolume()
            let level = player.isMuted ? 0 : Double(player.volume)
            notice.show(level == 0 ? "mute" : "volume", "\(Int((level * 100).rounded()))%", level: level)
        }

        private func updateVolume() {
            let muted = player.isMuted || player.volume == 0
            muteButton.set(muted ? "mute" : "volume", muted ? MavioStrings.playerUnmute : MavioStrings.playerMute)
            volumeSlider.doubleValue = player.isMuted ? 0 : Double(player.volume)
        }

        // MARK: The wheel and trackpad

        /// The wheel and trackpad: sideways seeks and up and down sets the
        /// volume, which of them taken at a gesture's start, and over the
        /// seek bar always seeking. A trackpad's seek pauses until the
        /// fingers lift; the momentum after moves nothing.
        private func scrolled(_ event: NSEvent) {
            guard event.momentumPhase.isEmpty else { return }
            let gesture = !event.phase.isEmpty
            if event.phase.contains(.began) { scrollAxis = nil }
            if gesture, !event.phase.intersection([.ended, .cancelled]).isEmpty {
                scrollAxis = nil
                scrollEnded = true
                finishScroll()
                return
            }
            // How the fingers or the wheel moved, whichever way the system scrolls.
            let inverted = event.isDirectionInvertedFromDevice
            let right = Double(inverted ? event.scrollingDeltaX : -event.scrollingDeltaX)
            let up = Double(inverted ? -event.scrollingDeltaY : event.scrollingDeltaY)
            guard right != 0 || up != 0 else { return }
            let overBar =
                controllerShown && seekBar.bounds.contains(seekBar.convert(event.locationInWindow, from: nil))
            if !gesture || scrollAxis == nil {
                scrollAxis = overBar || abs(right) > abs(up) ? .seek : .volume
            }
            let precise = event.hasPreciseScrollingDeltas
            switch scrollAxis {
            case .seek:
                let delta = overBar && abs(up) > abs(right) ? up : right
                if scrollTarget == nil {
                    scrollEnded = !gesture
                    resumeAfterScroll = gesture && player.isPlaying && !grouped
                    if resumeAfterScroll { player.pause() }
                }
                let duration = player.duration
                let target = min(
                    max((scrollTarget ?? player.currentTime) + delta * (precise ? 0.25 : 5), 0),
                    duration > 0 ? duration : .infinity)
                scrollTarget = target
                notice.show(
                    delta >= 0 ? "seekForward" : "seekBack", "\(clock(target)) / \(clock(duration))",
                    level: duration > 0 ? target / duration : nil)
                seekToScrollTarget()
            case .volume:
                changeVolume(by: Float(up) * (precise ? 0.005 : 0.05))
            case nil:
                break
            }
        }

        /// Seeks to where the wheel or trackpad points, one seek at a time,
        /// the last place it pointed to once the one on its way is done.
        private func seekToScrollTarget() {
            guard !scrollSeeking, let target = scrollTarget else { return }
            scrollSeeking = true
            Task { [weak self] in
                guard let self else { return }
                await self.player.seek(to: target)
                self.scrollSeeking = false
                if let latest = self.scrollTarget, latest != target {
                    self.seekToScrollTarget()
                } else {
                    self.finishScroll()
                }
            }
        }

        /// Once the gesture has ended and its seek is done, plays on what
        /// it paused.
        private func finishScroll() {
            guard scrollEnded, !scrollSeeking else { return }
            scrollTarget = nil
            if resumeAfterScroll { player.play() }
            resumeAfterScroll = false
        }

        // MARK: Seeking on the bar

        /// A drag of the seek bar pauses what plays on its own, until it
        /// ends.
        private func scrubStarted() {
            hideTask?.cancel()
            scrubbing = player.currentTime
            resumeAfterScrub = player.isPlaying && !grouped
            if resumeAfterScrub { player.pause() }
        }

        private func scrubEnded(at target: TimeInterval) {
            scrubbing = target
            timeChanged()
            Task { [weak self] in
                guard let self else { return }
                await self.player.seek(to: target)
                self.scrubbing = nil
                if self.resumeAfterScrub { self.player.play() }
                self.resumeAfterScrub = false
                self.timeChanged()
                self.scheduleHide()
            }
        }

        /// Shows over the pointer's time on the seek bar its picture, the
        /// chapter's name and the time, above the bar or, where that does
        /// not fit, below it; nil hides them.
        private func hover(_ time: TimeInterval?, at x: CGFloat) {
            previewTask?.cancel()
            guard let time else {
                preview.isHidden = true
                return
            }
            let chapter = source?.chapters.last { $0.start <= time }?.name
            preview.set(time: time, chapter: chapter)
            if preview.isHidden { preview.set(picture: nil) }
            preview.isHidden = false
            place(preview: x)
            previewTask = Task { [weak self] in
                guard let self else { return }
                let picture = await self.player.thumbnail(at: time)
                guard !Task.isCancelled else { return }
                if let picture {
                    self.preview.set(picture: picture)
                    self.place(preview: x)
                }
            }
        }

        private func place(preview x: CGFloat) {
            controller.layoutSubtreeIfNeeded()
            let size = preview.fittingSize
            let center = seekBar.convert(NSPoint(x: x, y: 0), to: controller).x
            let margin = MavioSpacing.s8
            previewLeading?.constant = min(
                max(center - size.width / 2, margin), max(controller.bounds.width - size.width - margin, margin))
            // The controller is flipped: the bar's top is how much room there is above it.
            let above = seekBar.convert(seekBar.bounds, to: controller).minY - MavioSpacing.s8 >= size.height + margin
            previewAbove?.isActive = above
            previewBelow?.isActive = !above
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
                if player.canOffset {
                    menu.addItem(.separator())
                    addOffset(to: menu, MavioStrings.playerSubtitleOffset, player.subtitleOffset) {
                        $0.offsetSubtitles(to: $1)
                    }
                }
            case .audio:
                let current = info?.audioStreamIndex ?? -1
                menu.addItem(.sectionHeader(title: MavioStrings.playerAudio))
                for stream in source?.audio ?? [] {
                    let label = trackLabel(stream.language, stream.title, stream.codec, stream.index)
                    menu.addItem(choice(label, stream.index == current) { $0.audio = stream.index })
                }
                if player.canOffset {
                    menu.addItem(.separator())
                    addOffset(to: menu, MavioStrings.playerAudioOffset, player.audioOffset) {
                        $0.offsetAudio(to: $1)
                    }
                }
                if player.canChooseAudioDevice {
                    menu.addItem(.separator())
                    menu.addItem(.sectionHeader(title: MavioStrings.playerAudioDevice))
                    let current = player.audioDevice
                    let system = MenuAction(MavioStrings.playerAudioDeviceSystem) { [weak self] in
                        self?.player.setAudioDevice(nil)
                    }
                    system.state = current == nil ? .on : .off
                    menu.addItem(system)
                    for device in AudioDevice.outputs() {
                        let item = MenuAction(device.name) { [weak self] in self?.player.setAudioDevice(device.id) }
                        item.state = device.id == current ? .on : .off
                        menu.addItem(item)
                    }
                }
            case .speed:
                menu.addItem(.sectionHeader(title: MavioStrings.playerSpeed))
                for rate: Float in [0.5, 0.75, 1, 1.25, 1.5, 1.75, 2] {
                    let item = MenuAction(Self.rateLabel(rate)) { [weak self] in self?.setRate(rate) }
                    item.state = rate == player.rate ? .on : .off
                    item.isEnabled = !grouped
                    menu.addItem(item)
                }
            }
            menuOpen = true
            hideTask?.cancel()
            menu.popUp(positioning: nil, at: NSPoint(x: 0, y: button.bounds.height + 4), in: button)
            menuOpen = false
            poke()
        }

        /// An offset's header with its value, and the items moving it a
        /// tenth of a second earlier or later and back to none.
        private func addOffset(
            to menu: NSMenu, _ title: String, _ value: TimeInterval,
            set: @escaping @MainActor (PlayerPage, TimeInterval) -> Void
        ) {
            menu.addItem(.sectionHeader(title: "\(title): \(Self.offsetLabel(value))"))
            menu.addItem(
                MenuAction(MavioStrings.playerOffsetEarlier) { [weak self] in
                    guard let self else { return }
                    set(self, value - 0.1)
                })
            menu.addItem(
                MenuAction(MavioStrings.playerOffsetLater) { [weak self] in
                    guard let self else { return }
                    set(self, value + 0.1)
                })
            let reset = MenuAction(MavioStrings.playerOffsetReset) { [weak self] in
                guard let self else { return }
                set(self, 0)
            }
            reset.isEnabled = value != 0
            menu.addItem(reset)
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
            case .speed:
                // Television mode has no speed picker.
                break
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
            follow()
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
                player.trickplay = TrickplaySheets.choose(
                    details.trickplay.map {
                        TrickplaySheets(
                            width: Int($0.width), height: Int($0.height), columns: Int($0.columns),
                            rows: Int($0.rows), count: Int($0.count), interval: $0.interval, url: $0.url)
                    }, wide: Int(ScrubPreview.width * 2))
                player.nowPlaying.next = next.map { _ in
                    { [weak self] in self?.playNext() }
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

        /// What the player plays.
        var playing: PlayRequest { request }

        /// Follows the group as the player loads an item.
        private func follow() {
            follower?.stop()
            let follower = GroupFollower(
                model: model, player: player, serverId: request.serverId, itemId: request.itemId,
                onStop: { [weak self] in self?.close() })
            follower.start()
            self.follower = follower
        }

        /// Whether the player follows a group.
        private var grouped: Bool { follower?.group != nil }

        /// Whether there is something to play next: the group's next entry,
        /// or the next episode.
        private var hasNext: Bool { next != nil || grouped && follower?.hasNext == true }

        /// Plays the next episode, or in a group the group's next entry or
        /// the next episode in the group.
        private func playNext() {
            if let follower, follower.group != nil {
                follower.next(next.map { [$0.card.itemId] } ?? [])
            } else if let next {
                play(next)
            }
        }

        /// Opens the group's dialog over the player.
        private func openGroup() {
            guard let server = model.server(request.serverId) else { return }
            SyncPlayDialog(context: context, server: server, watching: true).present(over: self)
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
            player.trickplay = nil
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
            for task in [loadTask, reportTask, hideTask, clickTask, previewTask] { task?.cancel() }
            stateObserver?.cancel()
            timeObserver?.cancel()
            groupObserver?.cancel()
            follower?.stop()
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
        /// The wheel or trackpad scrolled.
        var onScroll: (@MainActor (NSEvent) -> Void)?
        /// The trackpad pinched.
        var onMagnify: (@MainActor (NSEvent) -> Void)?
        /// The pointer left the view.
        var onExit: (@MainActor () -> Void)?
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
        override func scrollWheel(with event: NSEvent) { onScroll?(event) }
        override func magnify(with event: NSEvent) { onMagnify?(event) }
        override func mouseExited(with event: NSEvent) { onExit?() }

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
                guard value > 0, !grouped else { return false }
                player.setRate(Float(value))
            case .next:
                guard hasNext else { return false }
                playNext()
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
