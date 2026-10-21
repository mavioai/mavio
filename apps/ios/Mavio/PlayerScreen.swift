import AVKit
import MavioKit
import MavioScreens
import MavioPlayer
import MavioUI
import SwiftUI

/// The player of an item: starts the playback on the server, plays it on
/// the engine that suits it, reports progress and draws the on-screen
/// controller (docs/clients.md §6.5); a side panel picks the audio,
/// subtitles and quality and tells how the item plays, and an episode's end
/// starts the next.
struct PlayerScreen: View {
    /// What plays: the item opened, then the episodes after it.
    @State private var request: PlayRequest
    @Environment(AppModel.self) private var model
    @Environment(Assistant.self) private var assistant
    @Environment(Navigation.self) private var navigation
    @Environment(\.dismiss) private var dismiss
    /// The player as the assistant controls it.
    @State private var control = PlayerControl()
    @State private var player = Player()
    @State private var session: PlaybackSession?
    @State private var info: PlaybackInfo?
    @State private var source: Source?
    @State private var title: String
    @State private var audio: Int32?
    @State private var subtitle: Int32?
    @State private var failure: String?
    @State private var triedAether = false
    @State private var controls = true
    @State private var interactions = 0
    @State private var panel: PlayerPanel?
    /// The episode after an episode, which its end plays.
    @State private var next: Episode?
    /// An episode's series and number, over its title.
    @State private var series: String?
    @AppStorage(PlaybackQuality.key) private var quality = PlaybackQuality.auto
    @AppStorage(DeviceSetting.autoplayNext) private var autoplayNext = true
    @Environment(\.openURL) private var openURL
    @State private var scrubbing: TimeInterval?
    /// The picture where the scrubber is, while it is dragged.
    @State private var scrubImage: CGImage?
    @State private var closing = false
    /// The press held for double speed, and the speed it plays at after.
    @GestureState private var holding = false
    @State private var heldRate: Float?
    @State private var heldEnded = Date.distantPast
    /// The vertical swipe for brightness or volume, and the level it shows.
    @State private var swipe: SwipeStart?
    @State private var swipeIgnored = false
    @State private var level: LevelShown?
    @State private var volume = SystemVolume()

    init(request: PlayRequest) {
        _request = State(initialValue: request)
        _title = State(initialValue: request.title)
    }

    var body: some View {
        GeometryReader { geo in
            ZStack {
                PlayerSurface(player: player)
                Color.clear
                    .contentShape(.rect)
                    .gesture(
                        SpatialTapGesture(count: 2)
                            .onEnded { tap in
                                Task { await player.skip(by: tap.location.x < geo.size.width / 2 ? -10 : 10) }
                                poke()
                            }
                            .exclusively(
                                before: TapGesture().onEnded {
                                    // The release of a held press is no tap.
                                    guard heldRate == nil, Date.now.timeIntervalSince(heldEnded) > 0.3 else { return }
                                    // A tap closes the panel before it hides the controls.
                                    withAnimation(.easeOut(duration: MavioDuration.fast)) {
                                        if panel != nil { panel = nil } else { controls.toggle() }
                                    }
                                    poke()
                                })
                    )
                    .simultaneousGesture(
                        LongPressGesture(minimumDuration: 0.5)
                            .sequenced(before: DragGesture(minimumDistance: 0))
                            .updating($holding) { value, state, _ in
                                if case .second(true, _) = value { state = true }
                            }
                    )
                    .simultaneousGesture(
                        DragGesture(minimumDistance: 16)
                            .onChanged { swiped($0, in: geo.size) }
                            .onEnded { _ in endSwipe() }
                    )
                VolumeView(volume: volume).frame(width: 1, height: 1).allowsHitTesting(false)
                if player.isBuffering, failure == nil {
                    ProgressView().controlSize(.large).tint(.white)
                }
                if controls || failure != nil {
                    overlay.transition(.opacity)
                }
                Group {
                    if holding {
                        HeldSpeedIndicator()
                    } else if let level {
                        LevelIndicator(level: level)
                    }
                }
                .transition(.opacity)
                .frame(maxHeight: .infinity, alignment: .top)
                .padding(.top, 72)
                .allowsHitTesting(false)
                if let panel, controls {
                    PanelView(
                        tab: Binding(get: { panel }, set: { self.panel = $0 }), tabs: panelTabs,
                        width: min(360, geo.size.width - 32)
                    ) { panelContent($0) }
                    .frame(maxWidth: .infinity, maxHeight: .infinity, alignment: .bottomTrailing)
                    .padding(.trailing, MavioSpacing.s16)
                    .padding(.bottom, 120)
                    .padding(.top, 64)
                    .transition(.opacity.combined(with: .scale(scale: 0.96, anchor: .bottomTrailing)))
                }
            }
            .animation(.easeOut(duration: MavioDuration.fast), value: panel)
            .animation(.easeOut(duration: MavioDuration.fast), value: holding)
            .animation(.easeOut(duration: MavioDuration.fast), value: level == nil)
        }
        .background(.black)
        .preferredColorScheme(.dark)
        .statusBarHidden(!controls)
        .persistentSystemOverlays(.hidden)
        .task { await start(at: request.start) }
        .onAppear {
            bind()
            navigation.player = control
        }
        .onDisappear { if navigation.player === control { navigation.player = nil } }
        .task(id: ObjectIdentifier(session ?? Self.none)) { await report() }
        .task(id: hideKey) {
            guard controls, player.isPlaying, panel == nil, scrubbing == nil else { return }
            try? await Task.sleep(for: .seconds(MavioDuration.controlsHide))
            guard !Task.isCancelled else { return }
            withAnimation(.easeOut(duration: MavioDuration.normal)) { controls = false }
        }
        .onChange(of: player.failure) { _, failure in
            guard let failure else { return }
            // AVPlayer refused a file the server let it play as it is:
            // AetherEngine plays it instead.
            if player.engine == .avPlayer, let info, !isHLS(info), !triedAether {
                triedAether = true
                Task { await load(info, start: player.currentTime, aether: true) }
            } else {
                self.failure = failure
            }
        }
        .onChange(of: holding) { _, held in
            if held {
                heldRate = player.rate
                player.setRate(2)
            } else {
                player.setRate(heldRate ?? 1)
                heldRate = nil
                heldEnded = .now
            }
        }
        .onChange(of: player.ended) { _, ended in
            guard ended else { return }
            if let next, autoplayNext { play(next) } else { close() }
        }
    }

    private static let none = NSObject()

    private var hideKey: String {
        "\(controls) \(player.isPlaying) \(panel != nil) \(scrubbing != nil) \(interactions)"
    }

    private func poke() {
        interactions += 1
    }

    // MARK: Gestures

    /// Follows a vertical swipe, which sets the brightness on the left half
    /// and the volume on the right; one that starts sideways, or at the
    /// edges the system keeps, sets nothing.
    private func swiped(_ drag: DragGesture.Value, in size: CGSize) {
        guard !holding, panel == nil, !swipeIgnored else { return }
        if swipe == nil {
            let (dx, dy) = (drag.translation.width, drag.translation.height)
            let start = drag.startLocation
            guard abs(dy) > abs(dx) * 1.5, start.y > 60, start.y < size.height - 60 else {
                swipeIgnored = true
                return
            }
            let kind: SwipeLevel = start.x < size.width / 2 ? .brightness : .volume
            swipe = SwipeStart(kind: kind, value: kind == .brightness ? Brightness.value : volume.value)
        }
        guard let swipe else { return }
        let value = min(max(swipe.value - drag.translation.height / (size.height * 0.6), 0), 1)
        switch swipe.kind {
        case .brightness: Brightness.value = value
        case .volume: volume.value = value
        }
        level = LevelShown(kind: swipe.kind, value: value)
    }

    private func endSwipe() {
        let ended = swipe != nil
        (swipe, swipeIgnored) = (nil, false)
        guard ended else { return }
        Task {
            try? await Task.sleep(for: .milliseconds(700))
            if swipe == nil { level = nil }
        }
    }

    // MARK: Playback

    private func isHLS(_ info: PlaybackInfo) -> Bool {
        info.url.range(of: #"\.m3u8(\?|$)"#, options: .regularExpression) != nil
    }

    private var airPlay: Bool {
        AVAudioSession.sharedInstance().currentRoute.outputs.contains { $0.portType == .airPlay }
    }

    /// Starts the playback on the server, at position, with the selected
    /// streams; a playback already going stops first.
    private func start(at position: TimeInterval?) async {
        failure = nil
        if let file = request.file {
            await player.load(
                Media(
                    url: file,
                    engine: PlayerEngine.choose(
                        hls: false, container: file.pathExtension.lowercased(), audioCodec: nil, selectsTracks: false,
                        avPlayerContainers: DeviceProfile.avPlayerContainers,
                        avPlayerAudioCodecs: DeviceProfile.avPlayerAudioCodecs),
                    start: position ?? 0))
            player.nowPlaying.show(NowPlaying.Item(title: title))
            return
        }
        if let old = session {
            session = nil
            try? await old.stop(position: player.currentTime)
        }
        if source == nil,
            let details = try? await model.client.details(serverId: request.serverId, itemId: request.itemId)
        {
            title = details.card.title
            if details.card.kind == .episode {
                next = details.next
                let number = details.episodeNumber.map {
                    MavioStrings.itemEpisodeShort(season: String(details.seasonNumber ?? 0), episode: String($0))
                }
                series = [details.series?.name, number].compactMap(\.self).joined(separator: " · ")
            }
            cachedSources = sources(details)
            source = cachedSources.first { $0.mediaSourceId == request.mediaSourceId } ?? cachedSources.first
            artwork = (details.card.thumb ?? details.card.poster).flatMap { URL(string: $0.url) }
            player.nowPlaying.next = next.map { episode in { play(episode) } }
        }
        let local = model.server(request.serverId).map { isLocalAddress(address: $0.address) } ?? false
        do {
            let session = try await model.client.play(
                serverId: request.serverId, itemId: request.itemId,
                capabilities: DeviceProfile.capabilities(airPlay: airPlay),
                options: PlaybackOptions(
                    mediaSourceId: source?.mediaSourceId ?? request.mediaSourceId, audioStreamIndex: audio,
                    subtitleStreamIndex: subtitle, maxBitrate: quality.bitrate(local: local), startPosition: position,
                    download: false
                ))
            let info = session.info()
            self.session = session
            self.info = info
            if let match = cachedSources.first(where: { $0.mediaSourceId == info.mediaSourceId }) { source = match }
            await load(info, start: info.startPosition ?? position ?? 0, aether: false)
            player.nowPlaying.show(NowPlaying.Item(title: title, subtitle: series, artwork: artwork))
        } catch is CancellationError {
        } catch {
            failure = error.localizedDescription
        }
    }

    @State private var cachedSources: [Source] = []
    /// The item's picture, which Now Playing shows.
    @State private var artwork: URL?

    /// The sources of this item on this server.
    private func sources(_ details: Details) -> [Source] {
        details.sources.filter { $0.serverId == request.serverId && $0.itemId == request.itemId }
    }

    /// Plays what the server returned on the engine that suits it.
    private func load(_ info: PlaybackInfo, start: TimeInterval, aether: Bool) async {
        guard let url = URL(string: info.url) else { return }
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
    private func report() async {
        guard let session else { return }
        while !Task.isCancelled {
            try? await session.update(position: player.currentTime, paused: !player.isPlaying)
            try? await Task.sleep(for: .seconds(1))
        }
    }

    /// Plays the episode after this one in the player's place, the
    /// playback of this one ending first.
    private func play(_ episode: Episode) {
        let position = player.ended ? nil : player.currentTime
        if let old = session {
            session = nil
            let serverId = request.serverId
            Task {
                try? await old.stop(position: position)
                model.bump(serverId)
            }
        }
        request = PlayRequest(serverId: request.serverId, itemId: episode.card.itemId, title: episode.card.title)
        title = episode.card.title
        (info, source, cachedSources, audio, subtitle, next, series) = (nil, nil, [], nil, nil, nil, nil)
        (failure, triedAether, panel) = (nil, false, nil)
        Task { await start(at: episode.position) }
    }

    private func close() {
        guard !closing else { return }
        closing = true
        let position = player.ended ? nil : player.currentTime
        player.stop()
        let session = session
        let client = model.client
        let request = request
        Task {
            if let session {
                try? await session.stop(position: position)
            } else if request.file != nil {
                try? await client.playedOffline(serverId: request.serverId, itemId: request.itemId, position: position)
            }
            model.bump(request.serverId)
        }
        dismiss()
    }

    // MARK: Controls

    private var overlay: some View {
        ZStack {
            LinearGradient(
                stops: [
                    .init(color: .black.opacity(0.6), location: 0), .init(color: .clear, location: 0.25),
                    .init(color: .clear, location: 0.65), .init(color: .black.opacity(0.7), location: 1),
                ], startPoint: .top, endPoint: .bottom
            )
            .allowsHitTesting(false)
            .ignoresSafeArea()
            VStack(spacing: 0) {
                topBar
                Spacer()
                if let failure {
                    StatusView(
                        icon: "error", title: MavioStrings.stateError, message: failure,
                        action: (MavioStrings.actionRetry, { Task { await start(at: player.currentTime) } }))
                } else {
                    center
                }
                Spacer()
                bottomBar
            }
            .padding(.horizontal, MavioSpacing.s16)
            .padding(.vertical, MavioSpacing.s8)
        }
        .foregroundStyle(.white)
    }

    private var topBar: some View {
        HStack(spacing: MavioSpacing.s16) {
            Button(action: close) {
                Icon("close", size: 24).frame(width: 44, height: 44).contentShape(.rect)
            }
            .accessibilityLabel(MavioStrings.playerBack)
            VStack(alignment: .leading, spacing: 0) {
                if let series, !series.isEmpty {
                    Text(series).type(MavioTypography.standard.footnote).foregroundStyle(.white.opacity(0.7))
                }
                Text(title).type(MavioTypography.standard.headline)
            }
            .lineLimit(1)
            Spacer()
            RoutePicker().frame(width: 44, height: 44)
        }
    }

    private var center: some View {
        HStack(spacing: MavioSpacing.s48) {
            Button {
                Task { await player.skip(by: -10) }
                poke()
            } label: {
                Icon("seekBack", size: 36).frame(width: 56, height: 56).contentShape(.rect)
            }
            .accessibilityLabel(MavioStrings.playerSeekBack(count: 10))
            Button {
                player.toggle()
                poke()
            } label: {
                Icon(player.isPlaying ? "pause" : "play", fill: true, size: 56).frame(width: 72, height: 72)
                    .contentShape(.rect)
            }
            .accessibilityLabel(player.isPlaying ? MavioStrings.playerPause : MavioStrings.playerPlay)
            Button {
                Task { await player.skip(by: 10) }
                poke()
            } label: {
                Icon("seekForward", size: 36).frame(width: 56, height: 56).contentShape(.rect)
            }
            .accessibilityLabel(MavioStrings.playerSeekForward(count: 10))
        }
    }

    private var bottomBar: some View {
        let duration = max(player.duration, 1)
        let position = scrubbing ?? player.currentTime
        return VStack(spacing: MavioSpacing.s4) {
            Slider(
                value: Binding(get: { min(position, duration) }, set: { scrubbing = $0 }), in: 0...duration
            ) { editing in
                if !editing, let target = scrubbing {
                    Task {
                        await player.seek(to: target)
                        scrubbing = nil
                    }
                }
            }
            .tint(MavioColors.dark.progress)
            .overlay {
                if let scrubbing {
                    GeometryReader { geo in
                        let x = geo.size.width * min(scrubbing / duration, 1)
                        let half = ScrubPreview.width / 2 + 4
                        ScrubPreview(image: scrubImage, time: clock(scrubbing))
                            .fixedSize()
                            .position(x: min(max(x, half), geo.size.width - half), y: 0)
                            .alignmentGuide(VerticalAlignment.center) { $0[.bottom] }
                            .offset(y: scrubImage == nil ? -28 : -76)
                    }
                }
            }
            // A picture for every two seconds the thumb crosses.
            .task(id: scrubbing.map { Int($0 / 2) }) {
                guard let scrubbing else {
                    scrubImage = nil
                    return
                }
                try? await Task.sleep(for: .milliseconds(60))
                guard !Task.isCancelled, let image = await player.thumbnail(at: scrubbing) else { return }
                if !Task.isCancelled { scrubImage = image }
            }
            HStack {
                Text(clock(position))
                Spacer()
                Text("-" + clock(max(0, player.duration - position)))
            }
            .type(MavioTypography.standard.caption)
            .monospacedDigit()
            .foregroundStyle(.white.opacity(0.8))
            HStack(spacing: MavioSpacing.s8) {
                if let next {
                    Button {
                        play(next)
                    } label: {
                        Label {
                            Text(MavioStrings.playerNextEpisode)
                        } icon: {
                            Icon("next", fill: true, size: 16)
                        }
                        .type(MavioTypography.standard.footnote).fontWeight(.semibold)
                        .padding(.horizontal, 14)
                        .frame(height: 36)
                        .glass(in: .capsule)
                    }
                    .buttonStyle(.plain)
                }
                Spacer()
                AssistantButton(assistant: assistant, size: 24)
                if let info, !info.subtitles.isEmpty {
                    barButton(
                        "subtitles", fill: (info.subtitleStreamIndex ?? -1) >= 0, active: panel == .subtitles,
                        label: MavioStrings.playerSubtitles
                    ) { toggle(.subtitles) }
                }
                barButton(
                    "settings", active: panel != nil && panel != .subtitles, label: MavioStrings.playerOptions
                ) { toggle(.audio) }
                if player.canPictureInPicture {
                    barButton("pictureInPicture", label: MavioStrings.playerPictureInPicture) {
                        player.togglePictureInPicture()
                    }
                }
            }
        }
    }

    /// Opens the panel on tab, or closes it when it shows a tab of that button.
    private func toggle(_ tab: PlayerPanel) {
        let open = tab == .subtitles ? panel == .subtitles : panel != nil && panel != .subtitles
        panel = open ? nil : tab
    }

    private func barButton(
        _ icon: String, fill: Bool = false, active: Bool = false, label: String, action: @escaping () -> Void
    ) -> some View {
        Button {
            action()
            poke()
        } label: {
            Icon(icon, fill: fill, size: 24)
                .frame(width: 44, height: 44)
                .background(active ? .white.opacity(0.2) : .clear, in: .circle)
                .contentShape(.circle)
        }
        .accessibilityLabel(label)
        .accessibilityAddTraits(active ? .isSelected : [])
    }

    private func clock(_ seconds: TimeInterval) -> String {
        guard seconds.isFinite else { return "0:00" }
        let s = Int(seconds)
        return s >= 3600
            ? String(format: "%d:%02d:%02d", s / 3600, s / 60 % 60, s % 60) : String(format: "%d:%02d", s / 60, s % 60)
    }

    // MARK: Panel

    /// The panel's tabs: a downloaded file has only its speed to choose.
    private var panelTabs: [PlayerPanel] {
        request.file != nil
            ? [.audio] : (info?.subtitles.isEmpty ?? true) ? [.audio, .quality, .info] : PlayerPanel.allCases
    }

    @ViewBuilder private func panelContent(_ tab: PlayerPanel) -> some View {
        switch tab {
        case .audio:
            if request.file == nil {
                ForEach(source?.audio ?? [], id: \.index) { stream in
                    PanelChoice(
                        label: audioLabel(stream), detail: audioDetail(stream),
                        checked: stream.index == info?.audioStreamIndex
                    ) {
                        audio = stream.index
                        restart()
                    }
                }
            }
            PanelHeading(MavioStrings.playerSpeed)
            Flow(spacing: 6, lineSpacing: 6) {
                ForEach([0.5, 0.75, 1, 1.25, 1.5, 2] as [Float], id: \.self) { rate in
                    let on = player.rate == rate
                    Button {
                        player.setRate(rate)
                        poke()
                    } label: {
                        Text("\(rate.formatted())×")
                            .type(MavioTypography.standard.footnote).fontWeight(.semibold).monospacedDigit()
                            .foregroundStyle(on ? .black : .white)
                            .padding(.horizontal, MavioSpacing.s12)
                            .frame(height: 32)
                            .background(on ? .white : .white.opacity(0.1), in: .capsule)
                    }
                    .buttonStyle(.plain)
                    .accessibilityAddTraits(on ? .isSelected : [])
                }
            }
            .padding(.horizontal, 10)
        case .subtitles:
            PanelChoice(label: MavioStrings.playerSubtitlesOff, checked: (info?.subtitleStreamIndex ?? -1) < 0) {
                subtitle = -1
                restart()
            }
            ForEach(info?.subtitles ?? [], id: \.streamIndex) { track in
                PanelChoice(
                    label: language(track.language) ?? track.title ?? "#\(track.streamIndex)",
                    detail: [track.title, track.format.uppercased(), track.forced ? "Forced" : nil]
                        .compactMap { $0 }.joined(separator: " · "),
                    checked: track.streamIndex == info?.subtitleStreamIndex
                ) {
                    subtitle = track.streamIndex
                    restart()
                }
            }
        case .quality:
            ForEach(PlaybackQuality.allCases, id: \.self) { q in
                PanelChoice(label: q.label, checked: q == quality) {
                    guard q != quality else { return }
                    quality = q
                    restart()
                }
            }
        case .info:
            if let info {
                VStack(alignment: .leading, spacing: MavioSpacing.s16) {
                    PanelFact(MavioStrings.playerMethod, method(info.kind))
                    if let source {
                        PanelFact(
                            MavioStrings.playerSource,
                            [source.name, source.container?.split(separator: ",").first.map { $0.uppercased() }]
                                .compactMap(\.self).joined(separator: " · "))
                    }
                    VStack(alignment: .leading, spacing: 2) {
                        if let url = URL(string: info.url) {
                            PanelAction(MavioStrings.playerOpenExternal, icon: "external") { openURL(url) }
                        }
                        PanelAction(MavioStrings.playerCopyLink, icon: "copy") {
                            UIPasteboard.general.string = info.url
                        }
                    }
                }
                .padding(.horizontal, 10)
                .padding(.top, MavioSpacing.s4)
            }
        }
    }

    private func method(_ kind: PlayKind) -> String {
        switch kind {
        case .directPlay: MavioStrings.playerDirectPlay
        case .directStream: MavioStrings.playerDirectStream
        case .transcode: MavioStrings.playerTranscode
        }
    }

    /// Lets the assistant control the player (docs/assistant.md §5.2).
    private func bind() {
        control.playing = {
            Assistant.playing(
                itemId: request.itemId, position: player.currentTime, paused: !player.isPlaying,
                speed: Double(player.rate), audio: info?.audioStreamIndex, subtitle: info?.subtitleStreamIndex)
        }
        control.control = { command, value in
            switch command {
            case .pause: player.pause()
            case .resume: player.play()
            case .stop: close()
            case .seek: Task { await player.seek(to: value) }
            case .seekBy: Task { await player.skip(by: value) }
            case .speed:
                guard value > 0 else { return false }
                player.setRate(Float(value))
            case .next:
                guard let next else { return false }
                play(next)
            // The player fills the screen already.
            case .fullscreen: break
            default: return false
            }
            poke()
            return true
        }
        control.streams = { audio, subtitle in
            guard info != nil else { return false }
            if let audio { self.audio = audio }
            if let subtitle { self.subtitle = subtitle }
            restart()
            return true
        }
        control.close = { close() }
        control.assistantServerId = request.serverId
    }

    private func restart() {
        let position = player.currentTime
        let playing = player.isPlaying
        triedAether = false
        Task {
            await start(at: position)
            // A paused video stays paused across a change of track.
            if !playing { player.pause() }
        }
    }

    private func language(_ code: String?) -> String? {
        guard let code, !code.isEmpty, code != "und" else { return nil }
        return Locale.current.localizedString(forLanguageCode: code) ?? code
    }

    private func audioLabel(_ stream: AudioStream) -> String {
        language(stream.language) ?? stream.title ?? "#\(stream.index)"
    }

    private func audioDetail(_ stream: AudioStream) -> String {
        let channels: String? =
            switch stream.channels {
            case 0: nil
            case 1: "Mono"
            case 2: "Stereo"
            case 6: "5.1"
            case 8: "7.1"
            default: "\(stream.channels) ch"
            }
        return [stream.title, stream.codec.uppercased(), channels].compactMap { $0 }.joined(separator: " · ")
    }
}

/// The tabs of the player's panel.
enum PlayerPanel: CaseIterable, Identifiable {
    case audio, subtitles, quality, info

    var id: Self { self }

    var title: String {
        switch self {
        case .audio: MavioStrings.playerAudio
        case .subtitles: MavioStrings.playerSubtitles
        case .quality: MavioStrings.playerQuality
        case .info: MavioStrings.playerInfo
        }
    }
}

/// The panel over the player's lower right: its tabs over the choices of
/// the one shown, in glass.
private struct PanelView<Content: View>: View {
    @Binding var tab: PlayerPanel
    let tabs: [PlayerPanel]
    let width: CGFloat
    @ViewBuilder let content: (PlayerPanel) -> Content

    var body: some View {
        VStack(spacing: 0) {
            if tabs.count > 1 {
                HStack(spacing: 2) {
                    ForEach(tabs) { t in
                        Button {
                            tab = t
                        } label: {
                            Text(t.title)
                                .type(MavioTypography.standard.footnote).fontWeight(.semibold)
                                .foregroundStyle(.white.opacity(t == tab ? 1 : 0.7))
                                .frame(maxWidth: .infinity, minHeight: 32)
                                .background(t == tab ? .white.opacity(0.16) : .clear, in: .capsule)
                                .contentShape(.capsule)
                        }
                        .buttonStyle(.plain)
                        .accessibilityAddTraits(t == tab ? .isSelected : [])
                    }
                }
                .padding(3)
                .background(.white.opacity(0.06), in: .capsule)
                .padding(MavioSpacing.s12)
            }
            ViewThatFits(in: .vertical) {
                list
                ScrollView { list }.scrollIndicators(.hidden)
            }
        }
        .frame(width: width)
        .glass(in: RoundedRectangle(cornerRadius: 18))
        .overlay(RoundedRectangle(cornerRadius: 18).strokeBorder(.white.opacity(0.1)))
        .environment(\.colorScheme, .dark)
        .accessibilityElement(children: .contain)
        .accessibilityLabel(MavioStrings.playerOptions)
    }

    private var list: some View {
        VStack(alignment: .leading, spacing: 2) { content(tab) }
            .padding(.horizontal, MavioSpacing.s8)
            .padding(.top, tabs.count > 1 ? 0 : MavioSpacing.s8)
            .padding(.bottom, MavioSpacing.s12)
            .frame(maxWidth: .infinity, alignment: .leading)
    }
}

/// A choice of the panel, checked when it is the current one.
private struct PanelChoice: View {
    let label: String
    var detail: String? = nil
    let checked: Bool
    let action: () -> Void

    var body: some View {
        Button(action: action) {
            HStack(spacing: MavioSpacing.s12) {
                VStack(alignment: .leading, spacing: 2) {
                    Text(label).type(MavioTypography.standard.callout)
                    if let detail, !detail.isEmpty {
                        Text(detail).type(MavioTypography.standard.footnote).foregroundStyle(.white.opacity(0.6))
                    }
                }
                Spacer(minLength: 0)
                if checked { Icon("check", size: 18) }
            }
            .foregroundStyle(.white)
            .padding(.horizontal, 10)
            .padding(.vertical, MavioSpacing.s8)
            .frame(minHeight: 44)
            .background(checked ? .white.opacity(0.1) : .clear, in: .rect(cornerRadius: 10))
            .contentShape(.rect)
        }
        .buttonStyle(.plain)
        .accessibilityAddTraits(checked ? .isSelected : [])
    }
}

/// A heading between the panel's choices.
private struct PanelHeading: View {
    let text: String
    init(_ text: String) { self.text = text }

    var body: some View {
        Text(text.uppercased())
            .type(MavioTypography.standard.caption).fontWeight(.semibold)
            .foregroundStyle(.white.opacity(0.6))
            .padding(.horizontal, 10)
            .padding(.top, MavioSpacing.s12)
            .padding(.bottom, MavioSpacing.s4)
            .accessibilityAddTraits(.isHeader)
    }
}

/// A fact of the panel's Info: its label over its value.
private struct PanelFact: View {
    let label: String
    let value: String
    init(_ label: String, _ value: String) { (self.label, self.value) = (label, value) }

    var body: some View {
        VStack(alignment: .leading, spacing: 2) {
            Text(label).type(MavioTypography.standard.footnote).foregroundStyle(.white.opacity(0.6))
            Text(value).type(MavioTypography.standard.callout).foregroundStyle(.white)
        }
    }
}

/// An action of the panel's Info.
private struct PanelAction: View {
    let label: String
    let icon: String
    let action: () -> Void
    init(_ label: String, icon: String, action: @escaping () -> Void) {
        (self.label, self.icon, self.action) = (label, icon, action)
    }

    var body: some View {
        Button(action: action) {
            Label {
                Text(label)
            } icon: {
                Icon(icon, size: 18)
            }
            .type(MavioTypography.standard.callout)
            .foregroundStyle(.white)
            .frame(maxWidth: .infinity, minHeight: 40, alignment: .leading)
            .padding(.horizontal, 10)
            .background(.white.opacity(0.08), in: .rect(cornerRadius: 10))
            .contentShape(.rect)
        }
        .buttonStyle(.plain)
    }
}

/// The system's AirPlay picker.
struct RoutePicker: UIViewRepresentable {
    func makeUIView(context: Context) -> AVRoutePickerView {
        let view = AVRoutePickerView()
        view.tintColor = .white
        view.prioritizesVideoDevices = true
        return view
    }

    func updateUIView(_ view: AVRoutePickerView, context: Context) {}
}
