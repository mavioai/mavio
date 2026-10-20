import AppKit
import MavioKit
import MavioPlayer
import MavioScreens
import MavioUI
import SwiftUI

/// What a window of the player's plays: replaced by the next video
/// played, unless each has a window of its own, and nil once it closes.
@MainActor
@Observable
final class PlayerSlot {
    var request: PlayRequest?

    init(request: PlayRequest) { self.request = request }
}

/// A window of the player's: the player of its slot's request, a new one
/// for each, which stops what played as it leaves.
struct PlayerSlotView: View {
    let slot: PlayerSlot
    /// Closes the window.
    let close: () -> Void

    var body: some View {
        ZStack {
            Color.black
            if let request = slot.request {
                PlayerView(request: request, close: close).id(request.id)
            }
        }
        .ignoresSafeArea()
    }
}

/// The player of a window of its own (docs/clients.md §4.4, §6.6): the
/// picture, and over it while the pointer moves or the playback is paused a
/// controller of the title, the timeline, play and pause, seeking, the next
/// episode, the subtitles, the audio and full screen. Space plays and
/// pauses, the arrows and J and L seek, F puts the window in full screen
/// and Escape takes it out. Closing the window stops the playback.
struct PlayerView: View {
    /// Closes the player's window.
    let close: () -> Void
    @State private var request: PlayRequest
    @Environment(AppModel.self) private var model
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
    @State private var menuOpen = false
    @State private var scrubbing: TimeInterval?
    @State private var stopped = false
    @FocusState private var focused: Bool
    @AppStorage(PlaybackQuality.key) private var quality = PlaybackQuality.auto
    @AppStorage(DeviceSetting.autoplayNext) private var autoplayNext = true

    init(request: PlayRequest, close: @escaping () -> Void) {
        self.close = close
        _request = State(initialValue: request)
        _title = State(initialValue: request.title)
    }

    var body: some View {
        ZStack {
            PlayerSurface(player: player)
                .contentShape(.rect)
                .onTapGesture(count: 2) { toggleFullScreen() }
                .onTapGesture { toggle() }
            if player.isBuffering, failure == nil {
                ProgressView().controlSize(.large).tint(.white)
            }
            if controls || failure != nil || !player.isPlaying {
                controller.transition(.opacity)
            }
        }
        .animation(.easeOut(duration: MavioDuration.fast), value: controls)
        .background(.black)
        .ignoresSafeArea()
        .preferredColorScheme(.dark)
        .environment(\.theme, Theme(colors: .dark, layout: .desktop, type: .desktop))
        .onContinuousHover { phase in
            if case .active = phase { poke() }
        }
        .focusable()
        .focusEffectDisabled()
        .focused($focused)
        .onAppear { focused = true }
        .onKeyPress(action: key)
        .task { await start(at: request.start) }
        .task(id: session.map(ObjectIdentifier.init)) { await report() }
        .task(id: "\(pokes) \(controls) \(player.isPlaying) \(menuOpen) \(scrubbing != nil)") {
            guard controls, player.isPlaying, !menuOpen, scrubbing == nil, failure == nil else { return }
            try? await Task.sleep(for: .seconds(MavioDuration.controlsHide))
            guard !Task.isCancelled else { return }
            controls = false
            NSCursor.setHiddenUntilMouseMoves(true)
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
            guard ended else { return }
            if let next, autoplayNext { play(next) } else { close() }
        }
        // The window closing takes the view away, which stops the playback.
        .onDisappear(perform: stop)
    }

    // MARK: Keys and pointer

    private func poke() {
        controls = true
        pokes += 1
    }

    private func toggle() {
        player.toggle()
        poke()
    }

    private func key(_ press: KeyPress) -> KeyPress.Result {
        switch press.key {
        case .space, "k": toggle()
        case .leftArrow: seek(by: -5)
        case .rightArrow: seek(by: 5)
        case "j": seek(by: -10)
        case "l": seek(by: 10)
        case "f": toggleFullScreen()
        case .escape:
            guard let window = NSApp.keyWindow, window.styleMask.contains(.fullScreen) else { return .ignored }
            window.toggleFullScreen(nil)
        default: return .ignored
        }
        return .handled
    }

    private func seek(by seconds: TimeInterval) {
        Task { await player.skip(by: seconds) }
        poke()
    }

    private func toggleFullScreen() {
        NSApp.keyWindow?.toggleFullScreen(nil)
    }

    // MARK: Controller

    private var hasSubtitles: Bool { !(info?.subtitles.isEmpty ?? true) }
    private var hasAudio: Bool { (source?.audio.count ?? 0) > 1 }

    private var controller: some View {
        ZStack {
            LinearGradient(
                stops: [
                    .init(color: .black.opacity(0.5), location: 0), .init(color: .clear, location: 0.2),
                    .init(color: .clear, location: 0.6), .init(color: .black.opacity(0.8), location: 1),
                ],
                startPoint: .top, endPoint: .bottom
            )
            .allowsHitTesting(false)
            if let failure {
                StatusView(
                    icon: "error", title: MavioStrings.stateError, message: failure,
                    action: (MavioStrings.actionRetry, { Task { await start(at: player.currentTime) } })
                )
                .frame(maxWidth: 480)
            }
            VStack(alignment: .leading, spacing: MavioSpacing.s12) {
                Spacer()
                VStack(alignment: .leading, spacing: MavioSpacing.s2) {
                    if let series, !series.isEmpty {
                        Text(series).font(.callout).foregroundStyle(.white.opacity(0.7))
                    }
                    Text(title).font(.title2.weight(.bold)).foregroundStyle(.white)
                }
                timeline
                buttons
            }
            .padding(.horizontal, MavioSpacing.s24)
            .padding(.bottom, MavioSpacing.s20)
        }
    }

    private var timeline: some View {
        let duration = max(player.duration, 1)
        let position = scrubbing ?? player.currentTime
        return HStack(spacing: MavioSpacing.s12) {
            Text(clock(position))
            Slider(
                value: Binding(get: { min(position, duration) }, set: { scrubbing = $0 }), in: 0...duration
            ) { editing in
                guard !editing, let target = scrubbing else { return }
                Task {
                    await player.seek(to: target)
                    scrubbing = nil
                }
            }
            .controlSize(.small)
            .tint(.white)
            Text("-" + clock(max(player.duration - position, 0)))
        }
        .font(.footnote.monospacedDigit())
        .foregroundStyle(.white.opacity(0.85))
    }

    private var buttons: some View {
        HStack(spacing: MavioSpacing.s16) {
            button("seekBack", MavioStrings.playerSeekBack(count: 10)) { seek(by: -10) }
            button(
                player.isPlaying ? "pause" : "play",
                player.isPlaying ? MavioStrings.playerPause : MavioStrings.playerPlay, size: 28
            ) {
                toggle()
            }
            button("seekForward", MavioStrings.playerSeekForward(count: 10)) { seek(by: 10) }
            if let next {
                button("next", MavioStrings.playerNextEpisode) { play(next) }
            }
            Spacer()
            if hasSubtitles {
                trackMenu("subtitles", MavioStrings.playerSubtitles) {
                    Picker(MavioStrings.playerSubtitles, selection: subtitleChoice) {
                        Text(MavioStrings.playerSubtitlesOff).tag(Int32(-1))
                        ForEach(info?.subtitles ?? [], id: \.streamIndex) { track in
                            Text(trackLabel(track.language, track.title, track.format, track.streamIndex))
                                .tag(track.streamIndex)
                        }
                    }
                    .pickerStyle(.inline)
                }
            }
            if hasAudio {
                trackMenu("audio", MavioStrings.playerAudio) {
                    Picker(MavioStrings.playerAudio, selection: audioChoice) {
                        ForEach(source?.audio ?? [], id: \.index) { stream in
                            Text(trackLabel(stream.language, stream.title, stream.codec, stream.index))
                                .tag(stream.index)
                        }
                    }
                    .pickerStyle(.inline)
                }
            }
            button("fullScreen", MavioStrings.playerFullScreen, action: toggleFullScreen)
        }
        .foregroundStyle(.white)
    }

    private func button(_ icon: String, _ label: String, size: CGFloat = 22, action: @escaping () -> Void)
        -> some View
    {
        Button(action: action) {
            Icon(icon, fill: true, size: size).frame(width: 36, height: 36).contentShape(.rect)
        }
        .buttonStyle(.plain)
        .help(label)
        .accessibilityLabel(label)
    }

    private func trackMenu(_ icon: String, _ label: String, @ViewBuilder content: () -> some View) -> some View {
        Menu(content: content) {
            Icon(icon, size: 22).frame(width: 36, height: 36).contentShape(.rect)
        }
        .menuStyle(.button)
        .buttonStyle(.plain)
        .menuIndicator(.hidden)
        .fixedSize()
        .help(label)
        .accessibilityLabel(label)
        .onHover { menuOpen = $0 }
    }

    private var subtitleChoice: Binding<Int32> {
        Binding(
            get: { info?.subtitleStreamIndex ?? -1 },
            set: {
                subtitle = $0
                restart()
            })
    }

    private var audioChoice: Binding<Int32> {
        Binding(
            get: { info?.audioStreamIndex ?? -1 },
            set: {
                audio = $0
                restart()
            })
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

    private func clock(_ t: TimeInterval) -> String {
        let s = Int(t)
        return s >= 3600
            ? String(format: "%d:%02d:%02d", s / 3600, s / 60 % 60, s % 60) : String(format: "%d:%02d", s / 60, s % 60)
    }

    // MARK: Playback

    private func isHLS(_ info: PlaybackInfo) -> Bool {
        info.url.range(of: #"\.m3u8(\?|$)"#, options: .regularExpression) != nil
    }

    /// Starts the playback on the server, at position, with the selected
    /// streams; a playback already going stops first. A downloaded file
    /// plays without the server.
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
            sources = details.sources.filter { $0.serverId == request.serverId && $0.itemId == request.itemId }
            source = sources.first { $0.mediaSourceId == request.mediaSourceId } ?? sources.first
            artwork = (details.card.thumb ?? details.card.poster).flatMap { URL(string: $0.url) }
            player.nowPlaying.next = next.map { episode in { play(episode) } }
        }
        let local = model.server(request.serverId).map { isLocalAddress(address: $0.address) } ?? false
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
            await load(info, start: info.startPosition ?? position ?? 0, aether: false)
            player.nowPlaying.show(NowPlaying.Item(title: title, subtitle: series, artwork: artwork))
            NSApp.keyWindow?.title = title
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
        (failure, triedAether) = (nil, false)
        Task { await start(at: episode.position) }
    }

    /// Stops the playback, reporting where it stopped.
    private func stop() {
        guard !stopped else { return }
        stopped = true
        let position = player.ended ? nil : player.currentTime
        player.stop()
        let session = session
        let serverId = request.serverId
        Task {
            try? await session?.stop(position: position)
            model.bump(serverId)
        }
    }
}
