#if os(macOS)
    import AppKit
    import MavioMac

    /// Dictation (docs/clients.md §6.3.26), as the web UI's: a field's text
    /// said rather than typed, heard by a voice session in dictation mode
    /// (docs/assistant.md §8.3), which sends the final transcripts of its
    /// turns and asks and speaks nothing.
    @MainActor final class Dictation {
        private let model: AppModel
        private let server: ServerRecord
        /// Whether it listens.
        private(set) var listening = false
        /// What it heard of the turn being said.
        private(set) var heard = ""
        /// Why it could not listen, until dismissed.
        private(set) var error: String?
        /// Called as what it shows changes.
        var onChange: (@MainActor () -> Void)?
        /// Called with each turn recognized.
        var onText: (@MainActor (String) -> Void)?
        private var session: VoiceSession?
        private var voice: Bool?

        /// Dictation on server.
        init(model: AppModel, server: ServerRecord) {
            self.model = model
            self.server = server
        }

        isolated deinit { session?.close() }

        /// Listens, or, while listening, stops.
        func toggle() {
            if let session {
                session.close()
            } else if !listening {
                listen()
            }
        }

        /// Clears the error.
        func dismiss() {
            error = nil
            onChange?()
        }

        /// Stops listening, dropping what is being said.
        func close() { session?.close() }

        private func set(listening: Bool, heard: String = "", error: String? = nil) {
            self.listening = listening
            self.heard = heard
            self.error = error
            onChange?()
        }

        private func listen() {
            set(listening: true)
            let server = server
            let model = model
            Task { [weak self] in
                if self?.voice == nil {
                    var request = Mavio_Assistant_V1_GetAssistantRequest()
                    request.language = Assistant.uiLanguage
                    let info = try? await model.call(
                        server, Procedure.assistant("GetAssistant"), request,
                        as: Mavio_Assistant_V1_GetAssistantResponse.self)
                    self?.voice = info.map { $0.enabled && $0.voice }
                }
                guard let self, listening else { return }
                guard voice == true else {
                    set(listening: false, error: MavioStrings.assistantNoVoice)
                    return
                }
                do {
                    let v = try await VoiceSession.capture(
                        .init(
                            event: { [weak self] e in self?.event(e) },
                            closed: { [weak self] unreachable, error in
                                guard let self else { return }
                                session = nil
                                set(
                                    listening: false,
                                    error: unreachable
                                        ? MavioStrings.homeServerUnreachable(server: server.name) : error)
                            },
                            capturing: {},
                            silent: { [weak self] device in
                                guard let self else { return }
                                error = MavioStrings.assistantSilentMicrophone(
                                    device: device.isEmpty ? MavioStrings.assistantMicrophone : device)
                                onChange?()
                            }))
                    guard listening else {
                        v.close()
                        return
                    }
                    session = v
                    var request = Mavio_Assistant_V1_StartVoiceSessionRequest()
                    request.language = Assistant.speechLanguage
                    request.mode = .dictation
                    v.connect(
                        {
                            let r = try await model.call(
                                server, Procedure.assistant("StartVoiceSession"), request,
                                as: Mavio_Assistant_V1_StartVoiceSessionResponse.self)
                            guard let url = Assistant.socketURL(r.url, server: server) else {
                                throw URLError(.badURL)
                            }
                            return url
                        }, pushToTalk: false)
                } catch VoiceSession.Failure.denied {
                    set(listening: false, error: MavioStrings.assistantMicrophoneDenied)
                } catch VoiceSession.Failure.unavailable(let reason) {
                    set(listening: false, error: MavioStrings.assistantNoMicrophone(error: reason))
                } catch {
                    set(listening: false, error: MavioStrings.assistantNoMicrophone(error: error.reason))
                }
            }
        }

        private func event(_ e: [String: Any]) {
            guard e["type"] as? String == "transcript", let text = e["text"] as? String else { return }
            if e["final"] as? Bool == true {
                heard = ""
                onChange?()
                let said = text.trimmingCharacters(in: .whitespacesAndNewlines)
                if !said.isEmpty { onText?(said) }
            } else {
                heard = text
                onChange?()
            }
        }
    }

    /// Text said joined to what was typed, a space between words of scripts
    /// that space them.
    func dictationJoined(_ typed: String, _ said: String) -> String {
        guard !typed.isEmpty else { return said }
        guard let last = typed.unicodeScalars.last, let first = said.unicodeScalars.first else {
            return typed + said
        }
        let spaced = { (s: Unicode.Scalar) -> Bool in
            if s.properties.numericType != nil { return true }
            switch s.value {
            // Latin, Greek and Cyrillic.
            case 0x41...0x5A, 0x61...0x7A, 0xC0...0x24F, 0x1E00...0x1EFF, 0x370...0x3FF, 0x1F00...0x1FFF,
                0x400...0x52F:
                return true
            default:
                return false
            }
        }
        let wordEnd = last.properties.isAlphabetic || last.properties.numericType != nil
        return wordEnd && spaced(first) ? typed + " " + said : typed + said
    }

    /// Bars rising and falling one after another, while dictation listens.
    final class VoiceBars: NSView {
        private let bars: [CALayer]

        /// Bars of color.
        init(color: NSColor) {
            bars = (0..<4).map { _ in CALayer() }
            super.init(frame: NSRect(x: 0, y: 0, width: 16, height: 14))
            translatesAutoresizingMaskIntoConstraints = false
            wantsLayer = true
            for (i, bar) in bars.enumerated() {
                bar.backgroundColor = color.cgColor
                bar.cornerRadius = 1.25
                bar.frame = NSRect(x: CGFloat(i) * 4.5, y: 0, width: 2.5, height: 14)
                layer?.addSublayer(bar)
            }
            NSLayoutConstraint.activate([
                widthAnchor.constraint(equalToConstant: 16), heightAnchor.constraint(equalToConstant: 14),
            ])
        }

        @available(*, unavailable)
        required init?(coder: NSCoder) { fatalError() }

        override func viewDidMoveToWindow() {
            super.viewDidMoveToWindow()
            for (i, bar) in bars.enumerated() {
                bar.removeAllAnimations()
                guard window != nil else { continue }
                let rise = CABasicAnimation(keyPath: "transform.scale.y")
                rise.fromValue = 0.3
                rise.toValue = 1
                rise.duration = 0.45
                rise.autoreverses = true
                rise.repeatCount = .infinity
                rise.timingFunction = CAMediaTimingFunction(name: .easeInEaseOut)
                rise.beginTime = CACurrentMediaTime() + Double(i) * 0.15
                bar.add(rise, forKey: "rise")
            }
        }
    }
#endif
