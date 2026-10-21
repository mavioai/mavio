#if !os(tvOS)
    import AVFoundation
    import Foundation
    import os

    /// A voice session from the client's side (docs/assistant.md §8, §9.3):
    /// one `AVAudioEngine` with voice processing on captures the microphone,
    /// echo cancelled, resampled to 16 kHz mono 16-bit PCM and sent in 20 ms
    /// frames on the session's WebSocket, and plays the answer's PCM, whose
    /// echo the voice processing removes. Capture starts at once, and what is
    /// captured is held until the WebSocket opens.
    @MainActor
    public final class VoiceSession {
        /// What a session tells as it goes.
        public struct Handlers {
            /// Every event the session sends, as its JSON object.
            public var event: @MainActor ([String: Any]) -> Void
            /// The session ended; unreachable when it closed before it was
            /// ready, error when it failed otherwise.
            public var closed: @MainActor (_ unreachable: Bool, _ error: String?) -> Void
            /// The microphone's first audio other than digital silence
            /// came: what is said from now on is heard.
            public var capturing: @MainActor () -> Void
            /// The microphone gave nothing but digital silence, as a muted
            /// input does; device names it.
            public var silent: @MainActor (_ device: String) -> Void

            /// The handlers of a session.
            public init(
                event: @escaping @MainActor ([String: Any]) -> Void,
                closed: @escaping @MainActor (Bool, String?) -> Void,
                capturing: @escaping @MainActor () -> Void,
                silent: @escaping @MainActor (String) -> Void
            ) {
                self.event = event
                self.closed = closed
                self.capturing = capturing
                self.silent = silent
            }
        }

        /// Why the microphone could not be opened.
        public enum Failure: Error {
            /// The user refused the microphone.
            case denied
            /// There is none, or it failed to open.
            case unavailable(String)
        }

        // How many of the microphone's 20 ms frames of digital silence tell
        // it is silent.
        private static let silentFrames = 100

        private let handlers: Handlers
        private let engine = AVAudioEngine()
        private let player = AVAudioPlayerNode()
        private let capture: Capture
        private let output = Meter()
        private var socket: URLSessionWebSocketTask?
        private var urlSession: URLSession?
        private var opened = false
        private var pending: [Data] = []
        private var ended = false
        private var captured = false
        // 20 ms frames of digital silence, until one was not.
        private var zeros = 0
        // Whether the session said it is ready: one closed before failed.
        private var ready = false
        // Push to talk: whether the user talks, and what the socket was told.
        private var talking = false
        private var told = false
        /// Whether the session listens only while the user holds to talk.
        public private(set) var holdsToTalk = false
        // The answers' speech: its rate and format, the samples of the run
        // queued and played, and a generation that drops the callbacks of
        // what an interrupt stopped.
        private var rate: Double = 24000
        private var format: AVAudioFormat
        private var queued = 0
        private var played = 0
        private var generation = 0

        #if os(iOS)
            /// The audio session's category and mode before the microphone
            /// opened, which a video playing had.
            private var previous: (AVAudioSession.Category, AVAudioSession.Mode)?
        #endif

        private init(handlers: Handlers, capture: Capture) {
            self.handlers = handlers
            self.capture = capture
            format = AVAudioFormat(standardFormatWithSampleRate: 24000, channels: 1)!
        }

        /// Opens the microphone; fails when it is refused or missing.
        public static func capture(_ handlers: Handlers) async throws -> VoiceSession {
            guard await AVAudioApplication.requestRecordPermission() else { throw Failure.denied }
            #if os(iOS)
                let audio = AVAudioSession.sharedInstance()
                let previous = (audio.category, audio.mode)
                do {
                    try audio.setCategory(
                        .playAndRecord, mode: .voiceChat, options: [.defaultToSpeaker, .allowBluetoothHFP])
                    try audio.setActive(true)
                } catch {
                    throw Failure.unavailable(error.localizedDescription)
                }
            #endif
            let capture = Capture()
            let session = VoiceSession(handlers: handlers, capture: capture)
            #if os(iOS)
                session.previous = previous
            #endif
            capture.deliver = { [weak session] pcm in
                DispatchQueue.main.async { MainActor.assumeIsolated { session?.audio(pcm) } }
            }
            // Voice processing takes up to a second or more to start: off the
            // main thread, the orb shows at once, loading meanwhile.
            let parts = Engine(
                engine: session.engine, player: session.player, capture: capture, output: session.output,
                format: session.format)
            do {
                try await withCheckedThrowingContinuation { (done: CheckedContinuation<Void, Error>) in
                    DispatchQueue.global(qos: .userInitiated).async {
                        do {
                            try Self.start(parts)
                            done.resume()
                        } catch {
                            done.resume(throwing: error)
                        }
                    }
                }
            } catch {
                session.release()
                throw Failure.unavailable(error.localizedDescription)
            }
            return session
        }

        /// The engine and what it plays and measures, set up together off
        /// the main thread before the session uses them.
        private struct Engine: @unchecked Sendable {
            let engine: AVAudioEngine
            let player: AVAudioPlayerNode
            let capture: Capture
            let output: Meter
            let format: AVAudioFormat
        }

        nonisolated private static func start(_ e: Engine) throws {
            let engine = e.engine
            // The output's unit made first: voice processing enabled before
            // it exists, the engine cannot start it (-10875).
            _ = engine.mainMixerNode
            let input = engine.inputNode
            // Voice processing cancels the echo of what the engine plays,
            // and suppresses noise and controls the gain.
            try input.setVoiceProcessingEnabled(true)
            let inFormat = input.outputFormat(forBus: 0)
            guard inFormat.sampleRate > 0, inFormat.channelCount > 0 else {
                throw Failure.unavailable("no microphone")
            }
            try e.capture.prepare(from: inFormat)
            input.installTap(onBus: 0, bufferSize: 1024, format: inFormat) {
                @Sendable [capture = e.capture] buffer, _ in
                capture.take(buffer)
            }
            engine.attach(e.player)
            engine.connect(e.player, to: engine.mainMixerNode, format: e.format)
            e.player.installTap(onBus: 0, bufferSize: 1024, format: e.format) {
                @Sendable [output = e.output] buffer, _ in
                output.measure(buffer)
            }
            engine.prepare()
            try engine.start()
            e.player.play()
        }

        /// How loud the microphone is now, from 0 to about 1.
        public func inputLevel() -> Double { ended ? 0 : capture.level }

        /// How loud the answer's speech is now, from 0 to about 1.
        public func outputLevel() -> Double { ended ? 0 : output.level }

        /// How much of the answer's speech has played and has come, in
        /// milliseconds.
        public func speech() -> (played: Double, total: Double) {
            (Double(played) / rate * 1000, Double(queued) / rate * 1000)
        }

        /// Connects to the session's WebSocket, once its URL is known.
        public func connect(_ url: @escaping @MainActor () async throws -> URL, pushToTalk: Bool) {
            holdsToTalk = pushToTalk
            Task { [weak self] in
                do {
                    let u = try await url()
                    self?.open(u)
                } catch {
                    self?.end(error: error.reason)
                }
            }
        }

        private func open(_ url: URL) {
            guard !ended else { return }
            let delegate = SocketDelegate(
                opened: { [weak self] in self?.socketOpened() },
                closed: { [weak self] in self?.socketClosed() })
            let session = URLSession(configuration: .default, delegate: delegate, delegateQueue: .main)
            urlSession = session
            let socket = session.webSocketTask(with: url)
            self.socket = socket
            socket.resume()
            receive(socket)
        }

        private func receive(_ socket: URLSessionWebSocketTask) {
            socket.receive { [weak self] result in
                DispatchQueue.main.async {
                    MainActor.assumeIsolated {
                        guard let self, self.socket === socket else { return }
                        switch result {
                        case .success(let message):
                            self.message(message)
                            self.receive(socket)
                        case .failure:
                            self.socketClosed()
                        }
                    }
                }
            }
        }

        private func socketOpened() {
            guard !ended else { return }
            opened = true
            tell()
            for pcm in pending { socket?.send(.data(pcm)) { _ in } }
            pending = []
        }

        private func socketClosed() {
            end(unreachable: !ready)
        }

        /// Push to talk: the user holds to talk, or lets go.
        public func talk(_ on: Bool) {
            talking = on
            if on { _ = drop() }
            if opened { tell() }
        }

        /// Sends an event, once the socket is open.
        public func send(_ event: [String: Any]) {
            guard opened, let data = try? JSONSerialization.data(withJSONObject: event),
                let text = String(data: data, encoding: .utf8)
            else { return }
            socket?.send(.string(text)) { _ in }
        }

        /// Ends the session.
        public func close() {
            send(["type": "close"])
            ready = true
            end()
        }

        private func tell() {
            guard holdsToTalk, talking != told else { return }
            told = talking
            send(["type": talking ? "ptt_start" : "ptt_end"])
        }

        private func audio(_ pcm: Data) {
            guard !ended else { return }
            let heard = pcm.contains { $0 != 0 }
            // Listening is told once the microphone gives more than digital
            // silence.
            if !captured && heard {
                captured = true
                handlers.capturing()
            }
            if zeros >= 0 {
                zeros = heard ? -1 : zeros + 1
                // A real microphone's noise is never all zeros for 2 s.
                if zeros == Self.silentFrames {
                    zeros = -1
                    handlers.silent(Self.microphoneName())
                }
            }
            // The socket opens once the plugin has made the session's
            // services, which can take seconds: what the user says meanwhile
            // is kept.
            if opened {
                socket?.send(.data(pcm)) { _ in }
            } else {
                pending.append(pcm)
            }
        }

        private static func microphoneName() -> String {
            #if os(macOS)
                AVCaptureDevice.default(for: .audio)?.localizedName ?? ""
            #else
                AVAudioSession.sharedInstance().currentRoute.inputs.first?.portName ?? ""
            #endif
        }

        private func message(_ message: URLSessionWebSocketTask.Message) {
            switch message {
            case .data(let pcm):
                push(pcm)
            case .string(let text):
                guard let data = text.data(using: .utf8),
                    let event = (try? JSONSerialization.jsonObject(with: data)) as? [String: Any],
                    let type = event["type"] as? String
                else { return }
                switch type {
                case "ready":
                    ready = true
                    if let r = event["output_sample_rate"] as? Double, r > 0 { setRate(r) }
                case "run":
                    // What the run plays is counted from here.
                    queued -= played
                    played = 0
                case "interrupt":
                    // The relay keeps the answer's speech as far as it was
                    // played (§8.8).
                    send(["type": "played", "ms": drop()])
                default:
                    break
                }
                handlers.event(event)
            @unknown default:
                break
            }
        }

        /// Plays the answer's speech at rate, from the session's ready event.
        private func setRate(_ r: Double) {
            guard r != rate, queued == played else { return }
            rate = r
            format = AVAudioFormat(standardFormatWithSampleRate: r, channels: 1)!
            player.removeTap(onBus: 0)
            engine.disconnectNodeOutput(player)
            engine.connect(player, to: engine.mainMixerNode, format: format)
            player.installTap(onBus: 0, bufferSize: 1024, format: format) { @Sendable [output] buffer, _ in
                output.measure(buffer)
            }
            if !player.isPlaying { player.play() }
        }

        /// Plays 16-bit PCM after what was queued.
        private func push(_ pcm: Data) {
            let count = pcm.count / 2
            guard !ended, count > 0,
                let buffer = AVAudioPCMBuffer(pcmFormat: format, frameCapacity: AVAudioFrameCount(count)),
                let channel = buffer.floatChannelData?[0]
            else { return }
            buffer.frameLength = AVAudioFrameCount(count)
            pcm.withUnsafeBytes { raw in
                let samples = raw.bindMemory(to: Int16.self)
                for i in 0..<count { channel[i] = Float(Int16(littleEndian: samples[i])) / 0x8000 }
            }
            queued += count
            let generation = generation
            player.scheduleBuffer(buffer, completionCallbackType: .dataPlayedBack) { @Sendable [weak self] _ in
                DispatchQueue.main.async {
                    MainActor.assumeIsolated {
                        guard let self, self.generation == generation else { return }
                        self.played = min(self.queued, self.played + count)
                    }
                }
            }
            if !player.isPlaying { player.play() }
        }

        /// Drops what was not played, returning how much of the run's
        /// speech was, in milliseconds.
        private func drop() -> Int {
            let ms = Int((Double(played) / rate * 1000).rounded())
            generation += 1
            player.stop()
            queued = 0
            played = 0
            if !ended { player.play() }
            return ms
        }

        private func release() {
            engine.inputNode.removeTap(onBus: 0)
            player.removeTap(onBus: 0)
            player.stop()
            engine.stop()
            #if os(iOS)
                let audio = AVAudioSession.sharedInstance()
                if let (category, mode) = previous, category == .playback {
                    // A video plays on, its audio as it was.
                    try? audio.setCategory(category, mode: mode)
                } else {
                    try? audio.setActive(false, options: .notifyOthersOnDeactivation)
                }
            #endif
        }

        private func end(unreachable: Bool = false, error: String? = nil) {
            guard !ended else { return }
            ended = true
            generation += 1
            release()
            socket?.cancel(with: .normalClosure, reason: nil)
            socket = nil
            urlSession?.invalidateAndCancel()
            urlSession = nil
            handlers.closed(unreachable, error)
        }

        isolated deinit {
            if !ended { release() }
        }
    }

    /// How loud speech is, from 0 to about 1: its RMS of roughly 0.01 to
    /// 0.3 spread out by its square root.
    private func loudness(_ buffer: AVAudioPCMBuffer) -> Double {
        guard let channel = buffer.floatChannelData?[0], buffer.frameLength > 0 else { return 0 }
        var sum: Float = 0
        for i in 0..<Int(buffer.frameLength) { sum += channel[i] * channel[i] }
        return min(1, Double((sum / Float(buffer.frameLength)).squareRoot().squareRoot()) * 1.5)
    }

    /// The level of the answer's speech, measured on the audio thread.
    private final class Meter: Sendable {
        private let value = OSAllocatedUnfairLock(initialState: 0.0)

        var level: Double { value.withLock { $0 } }

        func measure(_ buffer: AVAudioPCMBuffer) {
            let l = loudness(buffer)
            value.withLock { $0 = l }
        }
    }

    /// The microphone's audio, resampled to 16 kHz mono 16-bit PCM in 20 ms
    /// frames on the audio thread.
    private final class Capture: @unchecked Sendable {
        /// Takes each frame, on the audio thread.
        var deliver: @Sendable (Data) -> Void = { _ in }
        private var converter: AVAudioConverter?
        private let format = AVAudioFormat(
            commonFormat: .pcmFormatInt16, sampleRate: 16000, channels: 1, interleaved: true)!
        private var frame: [Int16] = []
        private let meter = Meter()

        var level: Double { meter.level }

        func prepare(from input: AVAudioFormat) throws {
            guard let converter = AVAudioConverter(from: input, to: format) else {
                throw VoiceSession.Failure.unavailable("the microphone's audio cannot be converted")
            }
            // The first channel is the voice processing's.
            converter.channelMap = [0]
            self.converter = converter
        }

        func take(_ buffer: AVAudioPCMBuffer) {
            meter.measure(buffer)
            guard let converter else { return }
            let capacity = AVAudioFrameCount(Double(buffer.frameLength) * 16000 / buffer.format.sampleRate) + 32
            guard let out = AVAudioPCMBuffer(pcmFormat: format, frameCapacity: capacity) else { return }
            var fed = false
            var error: NSError?
            converter.convert(to: out, error: &error) { _, status in
                if fed {
                    status.pointee = .noDataNow
                    return nil
                }
                fed = true
                status.pointee = .haveData
                return buffer
            }
            guard error == nil, let samples = out.int16ChannelData?[0] else { return }
            frame.append(contentsOf: UnsafeBufferPointer(start: samples, count: Int(out.frameLength)))
            while frame.count >= 320 {
                let pcm = frame.prefix(320).withUnsafeBufferPointer { Data(buffer: $0) }
                frame.removeFirst(320)
                deliver(pcm)
            }
        }
    }

    /// Tells the session when its WebSocket opens and closes, on the main
    /// queue.
    private final class SocketDelegate: NSObject, URLSessionWebSocketDelegate, Sendable {
        private let opened: @MainActor @Sendable () -> Void
        private let closed: @MainActor @Sendable () -> Void

        init(opened: @escaping @MainActor @Sendable () -> Void, closed: @escaping @MainActor @Sendable () -> Void) {
            self.opened = opened
            self.closed = closed
        }

        func urlSession(
            _ session: URLSession, webSocketTask: URLSessionWebSocketTask, didOpenWithProtocol protocol: String?
        ) {
            MainActor.assumeIsolated { opened() }
        }

        func urlSession(
            _ session: URLSession, webSocketTask: URLSessionWebSocketTask,
            didCloseWith closeCode: URLSessionWebSocketTask.CloseCode, reason: Data?
        ) {
            MainActor.assumeIsolated { closed() }
        }

        func urlSession(_ session: URLSession, task: URLSessionTask, didCompleteWithError error: (any Error)?) {
            MainActor.assumeIsolated { closed() }
        }
    }
#endif
