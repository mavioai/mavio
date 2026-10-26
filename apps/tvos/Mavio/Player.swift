import MavioCommon
import MavioKit
import MavioPlayer
import MavioUI
import SwiftUI

/// The player (docs/tv.md §7): the picture, and a controller of the title,
/// the timeline and a row of buttons — the Watch Together group's chip
/// while the player follows one, the next episode, the subtitles and the
/// audio — that the remote's keys show; select pauses and plays, left
/// and right seek by 10 seconds, and back dismisses the assistant, then
/// closes the panel, then the controller, then the player. The assistant
/// shows over it and controls it.
struct PlayerScreen: View {
    @State private var request: PlayRequest
    @Environment(AppModel.self) private var model
    @Environment(\.dismiss) private var dismiss
    @Environment(Assistant.self) private var assistant
    @Environment(Router.self) private var router
    /// The player as the assistant controls it.
    @State private var control = PlayerControl()
    @State private var player = Player()
    @State private var session: PlaybackSession?
    @State private var info: PlaybackInfo?
    @State private var source: Source?
    @State private var sources: [Source] = []
    @State private var title: String
    @State private var series: String?
    @State private var next: Episode?
    @State private var artwork: URL?
    @State private var audio: Int32?
    @State private var subtitle: Int32?
    @State private var failure: String?
    @State private var triedAether = false
    @State private var controls = true
    @State private var pokes = 0
    @State private var panel: Panel?
    @State private var closing = false
    /// Follows the Watch Together group while it plays the item.
    @State private var follower: GroupFollower?
    @State private var together = false
    /// The screen's bottom, and the top of the controls on it while they
    /// show, which the subtitles keep above.
    @State private var screenBottom: CGFloat = 0
    @State private var controlsTop: CGFloat?
    @FocusState private var focus: Focus?
    @AppStorage(PlaybackQuality.key) private var quality = PlaybackQuality.auto
    @AppStorage(DeviceSetting.autoplayNext) private var autoplayNext = true

    private enum Focus: Hashable {
        case picture, group, next, subtitles, audio, choice(Int32)
    }

    private enum Panel { case subtitles, audio }

    init(request: PlayRequest) {
        _request = State(initialValue: request)
        _title = State(initialValue: request.title)
    }

    var body: some View {
        ZStack {
            PlayerSurface(player: player, subtitleLift: subtitleLift)
                .onGeometryChange(for: CGFloat.self) {
                    $0.frame(in: .global).maxY
                } action: {
                    screenBottom = $0
                }
            // The picture takes the remote's keys while nothing else has focus.
            Button(action: select) { Color.clear }
                .buttonStyle(Bare())
                .focused($focus, equals: .picture)
                .onMoveCommand(perform: move)
            if player.isBuffering, failure == nil {
                ProgressView().tint(.white)
            }
            if controls || failure != nil {
                controller.transition(.opacity)
            }
            if let panel {
                side(panel)
                    .frame(maxWidth: .infinity, maxHeight: .infinity, alignment: .trailing)
                    .transition(.move(edge: .trailing).combined(with: .opacity))
            }
            AssistantOverlay()
        }
        .animation(.easeOut(duration: MavioDuration.fast), value: controls)
        .animation(.easeOut(duration: MavioDuration.fast), value: panel)
        .background(.black)
        .ignoresSafeArea()
        .preferredColorScheme(.dark)
        .defaultFocus($focus, .picture)
        .onPlayPauseCommand(perform: select)
        .onExitCommand(perform: back)
        .task { await start(at: request.start) }
        .onAppear {
            bind()
            router.player = control
        }
        .onDisappear { if router.player === control { router.player = nil } }
        .task(id: session.map(ObjectIdentifier.init)) { await report() }
        .task(id: "\(pokes) \(controls) \(player.isPlaying) \(panel != nil)") {
            guard controls, player.isPlaying, panel == nil, failure == nil else { return }
            try? await Task.sleep(for: .seconds(MavioDuration.controlsHide))
            guard !Task.isCancelled else { return }
            controls = false
            if focus != .picture { focus = .picture }
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
        .onChange(of: player.ended) { _, ended in
            // The group moves on at the end.
            guard ended, follower?.group == nil else { return }
            if let next, autoplayNext { play(next) } else { close() }
        }
        .fullScreenCover(isPresented: $together) {
            if let server = model.server(request.serverId) {
                SyncPlayView(server: server, watching: true) { router.play($0) }
            }
        }
    }

    // MARK: Keys

    private func poke() {
        controls = true
        pokes += 1
    }

    /// Select and play/pause: pausing shows the controller for as long as it
    /// is paused; playing on hides it.
    private func select() {
        player.toggle()
        if player.isPlaying {
            controls = false
        } else {
            poke()
        }
    }

    private func move(_ direction: MoveCommandDirection) {
        switch direction {
        case .left, .right:
            Task { await player.skip(by: direction == .left ? -10 : 10) }
            poke()
        case .down:
            if controls, hasButtons {
                focus = grouped ? .group : hasNext ? .next : hasSubtitles ? .subtitles : .audio
            }
            poke()
        default:
            poke()
        }
    }

    private func back() {
        if assistant.open {
            assistant.close()
        } else if let open = panel {
            panel = nil
            focus = open == .audio ? .audio : .subtitles
        } else if controls, failure == nil {
            controls = false
            focus = .picture
        } else {
            close()
        }
    }

    // MARK: Controller

    private var hasSubtitles: Bool { !(info?.subtitles.isEmpty ?? true) }
    private var hasAudio: Bool { (source?.audio.count ?? 0) > 1 }
    private var hasButtons: Bool { grouped || hasNext || hasSubtitles || hasAudio }
    /// Whether the player follows a group.
    private var grouped: Bool { follower?.group != nil }
    /// Whether there is something to play next: the group's next entry, or
    /// the next episode.
    private var hasNext: Bool { next != nil || grouped && follower?.hasNext == true }

    /// Plays the next episode, or in a group the group's next entry or the
    /// next episode in the group.
    private func playNext() {
        if let follower, follower.group != nil {
            follower.next(next.map { [$0.card.itemId] } ?? [])
        } else if let next {
            play(next)
        }
    }

    /// Follows the group as the player loads an item.
    private func follow() {
        follower?.stop()
        let follower = GroupFollower(
            model: model, player: player, serverId: request.serverId, itemId: request.itemId,
            onStop: { close() })
        follower.start()
        self.follower = follower
    }

    private var controller: some View {
        ZStack(alignment: .bottom) {
            LinearGradient(
                stops: [.init(color: .clear, location: 0.5), .init(color: .black.opacity(0.8), location: 1)],
                startPoint: .top, endPoint: .bottom)
            if let failure {
                StatusView(
                    icon: "error", title: MavioStrings.stateError, message: failure,
                    action: (MavioStrings.actionRetry, { Task { await start(at: player.currentTime) } })
                )
                .frame(maxHeight: .infinity)
            }
            VStack(alignment: .leading, spacing: MavioSpacing.s24) {
                VStack(alignment: .leading, spacing: MavioSpacing.s4) {
                    if let series, !series.isEmpty {
                        Text(series).type(MavioTypography.tv.callout).foregroundStyle(.white.opacity(0.7))
                    }
                    Text(title).type(MavioTypography.tv.title2).foregroundStyle(.white)
                }
                Timeline(
                    position: player.currentTime, duration: player.duration,
                    chapters: source?.chapters.map(\.start) ?? [])
                if hasButtons {
                    HStack(spacing: MavioSpacing.s24) {
                        if let group = follower?.group {
                            Button {
                                together = true
                            } label: {
                                Label {
                                    Text(groupChip(group)).lineLimit(1)
                                } icon: {
                                    Icon("users", fill: true, size: 28)
                                }
                            }
                            .focused($focus, equals: .group)
                        }
                        if hasNext {
                            Button(action: playNext) {
                                Label {
                                    Text(MavioStrings.playerNextEpisode)
                                } icon: {
                                    Icon("next", size: 28)
                                }
                            }
                            .focused($focus, equals: .next)
                        }
                        if hasSubtitles {
                            Button {
                                open(.subtitles)
                            } label: {
                                Label {
                                    Text(MavioStrings.playerSubtitles)
                                } icon: {
                                    Icon("subtitles", size: 28)
                                }
                            }
                            .focused($focus, equals: .subtitles)
                        }
                        if hasAudio {
                            Button {
                                open(.audio)
                            } label: {
                                Label {
                                    Text(MavioStrings.playerAudio)
                                } icon: {
                                    Icon("audio", size: 28)
                                }
                            }
                            .focused($focus, equals: .audio)
                        }
                    }
                    .buttonStyle(TVButtonStyle())
                    .focusSection()
                    .onMoveCommand { if $0 == .up { focus = .picture } else { poke() } }
                }
            }
            .onGeometryChange(for: CGFloat.self) {
                $0.frame(in: .global).minY
            } action: {
                controlsTop = $0
            }
            .onDisappear { controlsTop = nil }
            .padding(.horizontal, 80)
            .padding(.bottom, 60)
        }
    }

    /// How far above the screen's bottom the subtitles keep, over the
    /// controls while they show.
    private var subtitleLift: CGFloat {
        guard controls, failure == nil, let controlsTop else { return 0 }
        return max(screenBottom - controlsTop + MavioSpacing.s16, 0)
    }

    private func open(_ which: Panel) {
        panel = which
        let current = which == .audio ? info?.audioStreamIndex : info?.subtitleStreamIndex
        focus = .choice(current ?? -1)
    }

    @ViewBuilder private func side(_ which: Panel) -> some View {
        ScrollView {
            VStack(alignment: .leading, spacing: MavioSpacing.s8) {
                Text(which == .audio ? MavioStrings.playerAudio : MavioStrings.playerSubtitles)
                    .type(MavioTypography.tv.title3).foregroundStyle(.white)
                    .padding(.bottom, MavioSpacing.s16)
                switch which {
                case .subtitles:
                    choice(-1, MavioStrings.playerSubtitlesOff, nil, (info?.subtitleStreamIndex ?? -1) < 0) {
                        subtitle = -1
                    }
                    ForEach(info?.subtitles ?? [], id: \.streamIndex) { track in
                        choice(
                            track.streamIndex, language(track.language) ?? track.title ?? "#\(track.streamIndex)",
                            [track.title, track.format.uppercased()].compactMap(\.self).joined(separator: " · "),
                            track.streamIndex == info?.subtitleStreamIndex
                        ) { subtitle = track.streamIndex }
                    }
                case .audio:
                    ForEach(source?.audio ?? [], id: \.index) { stream in
                        choice(
                            stream.index, language(stream.language) ?? stream.title ?? "#\(stream.index)",
                            [stream.title, stream.codec.uppercased()].compactMap(\.self).joined(separator: " · "),
                            stream.index == info?.audioStreamIndex
                        ) { audio = stream.index }
                    }
                }
            }
            .padding(MavioSpacing.s48)
        }
        .frame(width: 620)
        .frame(maxHeight: .infinity)
        .background(.ultraThinMaterial)
        .focusSection()
    }

    private func choice(
        _ index: Int32, _ label: String, _ detail: String?, _ checked: Bool, pick: @escaping () -> Void
    ) -> some View {
        Button {
            pick()
            panel = nil
            focus = .picture
            restart()
        } label: {
            HStack {
                VStack(alignment: .leading, spacing: 2) {
                    Text(label)
                    if let detail, !detail.isEmpty { Text(detail).type(MavioTypography.tv.footnote).opacity(0.7) }
                }
                Spacer()
                if checked { Icon("check", size: 28) }
            }
            .padding(.vertical, MavioSpacing.s8)
        }
        .buttonStyle(RowButtonStyle())
        .focused($focus, equals: .choice(index))
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
    /// streams; a playback already going stops first.
    private func start(at position: TimeInterval?) async {
        failure = nil
        follow()
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
            sources = details.sources.filter { $0.serverId == request.serverId && $0.itemId == request.itemId }
            source = sources.first { $0.mediaSourceId == request.mediaSourceId } ?? sources.first
            artwork = (details.card.thumb ?? details.card.poster).flatMap { URL(string: $0.url) }
            player.nowPlaying.next = next.map { _ in { playNext() } }
        }
        let local = model.server(request.serverId).map { isLocalAddress(address: $0.address) } ?? false
        do {
            let session = try await model.client.play(
                serverId: request.serverId, itemId: request.itemId,
                capabilities: DeviceProfile.capabilities(airPlay: false),
                options: PlaybackOptions(
                    mediaSourceId: source?.mediaSourceId, audioStreamIndex: audio, subtitleStreamIndex: subtitle,
                    maxBitrate: quality.bitrate(local: local), startPosition: position, download: false))
            let info = session.info()
            self.session = session
            self.info = info
            if let match = sources.first(where: { $0.mediaSourceId == info.mediaSourceId }) { source = match }
            await load(info, start: info.startPosition ?? position ?? 0, aether: false)
            player.nowPlaying.show(NowPlaying.Item(title: title, subtitle: series, artwork: artwork))
        } catch is CancellationError {
        } catch {
            failure = error.reason
        }
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
                guard value > 0, !grouped else { return false }
                player.setRate(Float(value))
            case .next:
                guard hasNext else { return false }
                playNext()
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

    /// Plays the episode after this one in the player's place.
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
        (info, source, sources, audio, subtitle, next, series) = (nil, nil, [], nil, nil, nil, nil)
        (failure, triedAether, panel) = (nil, false, nil)
        focus = .picture
        Task { await start(at: episode.position) }
    }

    private func close() {
        guard !closing else { return }
        closing = true
        follower?.stop()
        let position = player.ended ? nil : player.currentTime
        player.stop()
        let session = session
        let serverId = request.serverId
        Task {
            try? await session?.stop(position: position)
            model.bump(serverId)
        }
        dismiss()
    }
}

/// The timeline: the position, the bar with its chapters' gaps and the time
/// left.
private struct Timeline: View {
    let position: TimeInterval
    let duration: TimeInterval
    let chapters: [TimeInterval]

    var body: some View {
        VStack(spacing: MavioSpacing.s8) {
            SeekTrack(
                fraction: duration > 0 ? position / duration : 0,
                chapters: duration > 0 ? chapters.map { $0 / duration } : [], height: 10, knob: 0,
                showsKnob: false)
            HStack {
                Text(clock(position))
                Spacer()
                Text("-" + clock(max(duration - position, 0)))
            }
            .type(MavioTypography.tv.footnote)
            .monospacedDigit()
            .foregroundStyle(.white.opacity(0.8))
        }
    }

    private func clock(_ t: TimeInterval) -> String {
        let s = Int(t)
        return s >= 3600
            ? String(format: "%d:%02d:%02d", s / 3600, s / 60 % 60, s % 60) : String(format: "%d:%02d", s / 60, s % 60)
    }
}

/// A button that shows nothing of its own, as the picture is.
private struct Bare: ButtonStyle {
    func makeBody(configuration: Configuration) -> some View {
        configuration.label
    }
}
