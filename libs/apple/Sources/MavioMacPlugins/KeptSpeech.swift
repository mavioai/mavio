#if os(macOS)
    import AVFoundation
    import AppKit
    import MavioMac

    /// Plays the speech the server kept of the assistant's answers
    /// (docs/assistant.md §8.8), one answer at a time: only those files,
    /// never a voice speaking again.
    @MainActor final class KeptSpeech: NSObject, AVAudioPlayerDelegate {
        private let model: AppModel
        private let server: ServerRecord
        /// The answer playing, by its key.
        private(set) var at: String?
        /// Called as the answer playing changes.
        var onChange: (@MainActor () -> Void)?
        private var player: AVAudioPlayer?
        private var loading: Task<Void, Never>?

        /// The kept speech of server's answers.
        init(model: AppModel, server: ServerRecord) {
            self.model = model
            self.server = server
        }

        isolated deinit {
            loading?.cancel()
            player?.stop()
        }

        /// Plays the speech at url as the answer key, or stops it when it
        /// plays.
        func toggle(_ key: String, url: String) {
            let again = at == key
            stop()
            guard !again else { return }
            at = key
            onChange?()
            // The encodings AVAudioPlayer plays, the smallest first.
            let path = url + (url.contains("?") ? "&" : "?") + "formats=aac,mp3,flac,wav"
            let model = model
            let server = server
            loading = Task { [weak self] in
                do {
                    let target = try await model.client.serverRequest(serverId: server.id, path: path)
                    guard let address = URL(string: target.url) else { throw URLError(.badURL) }
                    var request = URLRequest(url: address)
                    request.setValue("Bearer \(target.token)", forHTTPHeaderField: "Authorization")
                    let (data, response) = try await URLSession.shared.data(for: request)
                    if let http = response as? HTTPURLResponse, !(200..<300).contains(http.statusCode) {
                        throw URLError(.badServerResponse)
                    }
                    guard let self, !Task.isCancelled, at == key else { return }
                    let player = try AVAudioPlayer(data: data)
                    player.delegate = self
                    self.player = player
                    if !player.play() { finish() }
                } catch {
                    guard let self, !Task.isCancelled, at == key else { return }
                    finish()
                }
            }
        }

        /// Stops what plays.
        func stop() {
            loading?.cancel()
            loading = nil
            player?.stop()
            player = nil
            if at != nil {
                at = nil
                onChange?()
            }
        }

        private func finish() {
            player = nil
            at = nil
            onChange?()
        }

        nonisolated func audioPlayerDidFinishPlaying(_ player: AVAudioPlayer, successfully flag: Bool) {
            let id = ObjectIdentifier(player)
            MainActor.assumeIsolated {
                if self.player.map(ObjectIdentifier.init) == id { finish() }
            }
        }

        nonisolated func audioPlayerDecodeErrorDidOccur(_ player: AVAudioPlayer, error: (any Error)?) {
            let id = ObjectIdentifier(player)
            MainActor.assumeIsolated {
                if self.player.map(ObjectIdentifier.init) == id { finish() }
            }
        }
    }
#endif
