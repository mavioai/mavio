import AVKit
import MavioKit
import MavioPlayer
import MavioUI
import SwiftUI

/// The player of an item: starts the playback on the server, plays it on
/// the engine that suits it, reports progress and draws the on-screen
/// controller (docs/clients.md §6.5).
struct PlayerScreen: View {
    let request: PlayRequest
    @Environment(AppModel.self) private var model
    @Environment(\.dismiss) private var dismiss
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
    @State private var picker: TrackPicker?
    @State private var scrubbing: TimeInterval?
    @State private var closing = false

    init(request: PlayRequest) {
        self.request = request
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
                                    withAnimation(.easeOut(duration: MavioDuration.fast)) { controls.toggle() }
                                    poke()
                                })
                    )
                if player.isBuffering, failure == nil {
                    ProgressView().controlSize(.large).tint(.white)
                }
                if controls || failure != nil {
                    overlay.transition(.opacity)
                }
            }
        }
        .background(.black)
        .preferredColorScheme(.dark)
        .statusBarHidden(!controls)
        .persistentSystemOverlays(.hidden)
        .sheet(item: $picker) { kind in
            pickerSheet(kind).presentationDetents([.medium, .large])
        }
        .task { await start(at: request.start) }
        .task(id: ObjectIdentifier(session ?? Self.none)) { await report() }
        .task(id: hideKey) {
            guard controls, player.isPlaying, picker == nil, scrubbing == nil else { return }
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
        .onChange(of: player.ended) { _, ended in
            if ended { close() }
        }
    }

    private static let none = NSObject()

    private var hideKey: String {
        "\(controls) \(player.isPlaying) \(picker != nil) \(scrubbing != nil) \(interactions)"
    }

    private func poke() {
        interactions += 1
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
            cachedSources = sources(details)
            source = cachedSources.first { $0.mediaSourceId == request.mediaSourceId } ?? cachedSources.first
        }
        do {
            let session = try await model.client.play(
                serverId: request.serverId, itemId: request.itemId,
                capabilities: DeviceProfile.capabilities(airPlay: airPlay),
                options: PlaybackOptions(
                    mediaSourceId: source?.mediaSourceId ?? request.mediaSourceId, audioStreamIndex: audio,
                    subtitleStreamIndex: subtitle, maxBitrate: nil, startPosition: position, download: false))
            let info = session.info()
            self.session = session
            self.info = info
            if let match = cachedSources.first(where: { $0.mediaSourceId == info.mediaSourceId }) { source = match }
            await load(info, start: info.startPosition ?? position ?? 0, aether: false)
        } catch is CancellationError {
        } catch {
            failure = error.localizedDescription
        }
    }

    @State private var cachedSources: [Source] = []

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
                hls: hls, container: source?.container,
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
                subtitleRendition: hls && sub >= 0 ? (track?.language ?? "und") : nil
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
            Text(title).type(MavioTypography.standard.headline).lineLimit(1)
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
            HStack {
                Text(clock(position))
                Spacer()
                Text("-" + clock(max(0, player.duration - position)))
            }
            .type(MavioTypography.standard.caption)
            .monospacedDigit()
            .foregroundStyle(.white.opacity(0.8))
            HStack(spacing: MavioSpacing.s8) {
                if let source, !source.audio.isEmpty, request.file == nil {
                    barButton("audio", label: MavioStrings.playerAudio) { picker = .audio }
                }
                if let info, !info.subtitles.isEmpty {
                    barButton(
                        "subtitles", fill: (info.subtitleStreamIndex ?? -1) >= 0, label: MavioStrings.playerSubtitles
                    ) { picker = .subtitles }
                }
                barButton("speed", label: MavioStrings.playerSpeed) { picker = .speed }
                Spacer()
                if player.canPictureInPicture {
                    barButton("pictureInPicture", label: MavioStrings.playerPictureInPicture) {
                        player.togglePictureInPicture()
                    }
                }
            }
        }
    }

    private func barButton(_ icon: String, fill: Bool = false, label: String, action: @escaping () -> Void)
        -> some View
    {
        Button {
            action()
            poke()
        } label: {
            Icon(icon, fill: fill, size: 24).frame(width: 44, height: 44).contentShape(.rect)
        }
        .accessibilityLabel(label)
    }

    private func clock(_ seconds: TimeInterval) -> String {
        guard seconds.isFinite else { return "0:00" }
        let s = Int(seconds)
        return s >= 3600
            ? String(format: "%d:%02d:%02d", s / 3600, s / 60 % 60, s % 60) : String(format: "%d:%02d", s / 60, s % 60)
    }

    // MARK: Pickers

    @ViewBuilder private func pickerSheet(_ kind: TrackPicker) -> some View {
        NavigationStack {
            List {
                switch kind {
                case .audio:
                    ForEach(source?.audio ?? [], id: \.index) { stream in
                        option(
                            audioLabel(stream), detail: audioDetail(stream),
                            selected: stream.index == info?.audioStreamIndex
                        ) {
                            audio = stream.index
                            restart()
                        }
                    }
                case .subtitles:
                    option(MavioStrings.playerSubtitlesOff, selected: (info?.subtitleStreamIndex ?? -1) < 0) {
                        subtitle = -1
                        restart()
                    }
                    ForEach(info?.subtitles ?? [], id: \.streamIndex) { track in
                        option(
                            language(track.language) ?? track.title ?? "#\(track.streamIndex)",
                            detail: [track.title, track.format.uppercased(), track.forced ? "Forced" : nil]
                                .compactMap { $0 }.joined(separator: " · "),
                            selected: track.streamIndex == info?.subtitleStreamIndex
                        ) {
                            subtitle = track.streamIndex
                            restart()
                        }
                    }
                case .speed:
                    ForEach([0.5, 0.75, 1, 1.25, 1.5, 2] as [Float], id: \.self) { rate in
                        option(rate == 1 ? "1×" : "\(rate.formatted())×", selected: player.rate == rate) {
                            player.setRate(rate)
                        }
                    }
                }
            }
            .navigationTitle(kind.title)
            .navigationBarTitleDisplayMode(.inline)
            .toolbar {
                ToolbarItem(placement: .confirmationAction) {
                    Button(MavioStrings.actionDone) { picker = nil }
                }
            }
        }
    }

    private func option(_ label: String, detail: String? = nil, selected: Bool, action: @escaping () -> Void)
        -> some View
    {
        Button {
            picker = nil
            action()
        } label: {
            HStack {
                VStack(alignment: .leading) {
                    Text(label)
                    if let detail, !detail.isEmpty {
                        Text(detail).font(.footnote).foregroundStyle(.secondary)
                    }
                }
                Spacer()
                if selected { Image(systemName: "checkmark").foregroundStyle(.tint) }
            }
            .contentShape(.rect)
        }
        .buttonStyle(.plain)
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

/// The pickers of the player.
enum TrackPicker: String, Identifiable {
    case audio, subtitles, speed

    var id: String { rawValue }

    var title: String {
        switch self {
        case .audio: MavioStrings.playerAudio
        case .subtitles: MavioStrings.playerSubtitles
        case .speed: MavioStrings.playerSpeed
        }
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
