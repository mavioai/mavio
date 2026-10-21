import Foundation
import MavioDesign
import MavioKit
import Observation
import SwiftProtobuf

/// The player open, as the assistant controls it (docs/assistant.md §5.2).
@MainActor
public protocol AssistantPlayer: AnyObject {
    /// The server of what it plays.
    var assistantServerId: String { get }
    /// What it plays, for the requests' context.
    var assistantPlaying: Mavio_Assistant_V1_AskPlaying { get }
    /// Carries out a command, value as `ControlAction` gives it; false when
    /// the player cannot.
    func assistantControl(_ command: Mavio_Assistant_V1_ControlCommand, value: Double) -> Bool
    /// Picks streams by their index in the media, -1 for no subtitles; nil
    /// keeps one. False when the player cannot.
    func assistantStreams(audio: Int32?, subtitle: Int32?) -> Bool
}

/// What the app shows, as the assistant's requests tell it.
public struct AssistantScreen: Equatable, Sendable {
    /// The screen, as `AskContext` names it: "home", "item", "library",
    /// "search", "settings", "player"…
    public var name: String
    /// The server of what it shows, if any.
    public var serverId: String?
    /// The item it shows, if any.
    public var itemId: String
    /// The library it shows, if any.
    public var libraryId: String

    /// A screen of name showing an item or library of a server.
    public init(_ name: String, serverId: String? = nil, itemId: String = "", libraryId: String = "") {
        self.name = name
        self.serverId = serverId
        self.itemId = itemId
        self.libraryId = libraryId
    }
}

/// The app around the assistant: what it shows and plays, and the client
/// actions it carries out.
@MainActor
public protocol AssistantHost: AnyObject {
    /// The screen shown.
    var assistantScreen: AssistantScreen { get }
    /// The player open, if any.
    var assistantPlayer: (any AssistantPlayer)? { get }
    /// Plays the action's items; false when the app cannot.
    func assistantPlay(_ action: Mavio_Assistant_V1_PlayAction, serverId: String) -> Bool
    /// Opens the action's screen; false when the app cannot.
    func assistantNavigate(_ action: Mavio_Assistant_V1_NavigateAction, serverId: String) -> Bool
}

/// The assistant over the app (docs/assistant.md §9.1, §9.3;
/// libs/design/components.md, Assistant), as the web UI's: an orb floating
/// on its own shows the voice session's state, and cards beside it what the
/// user said, the answer with its words lit as they are spoken, the tools
/// run, results to choose from and a pending confirmation. The assistant
/// button, or the orb, tapped starts a voice session and tapped again ends
/// it; held, or a key held, it talks while held. The app carries out the
/// session's client actions. It goes away on its own a few seconds after an
/// answer with nothing to choose, or after the session ends; and when it is
/// dismissed.
@MainActor
@Observable
public final class Assistant {
    /// A tool run or running.
    public struct Activity: Identifiable, Equatable {
        /// The tool call's ID.
        public let id: String
        /// What it does, as the user reads it.
        public var summary: String
        /// Whether it ended, and whether it failed.
        public var done: Bool
        /// Whether it failed.
        public var failed: Bool
    }

    /// A confirmation the answer waits for.
    public struct Confirmation: Equatable {
        /// The confirmation's ID, which the answer names.
        public let id: String
        /// What it asks.
        public let text: String
    }

    /// The key held to talk, outside text fields.
    public static let talkKey = "v"
    /// How long the button or key must be held to talk while held, rather
    /// than start a conversation.
    private static let hold: Duration = .milliseconds(400)
    /// How long an answer with nothing to choose stays before the assistant
    /// goes away.
    private static let linger: Duration = .seconds(6)
    /// How long the orb and cards take to fade out.
    public static let leave: Duration = .milliseconds(400)

    /// Whether the orb and cards show.
    public private(set) var open = false
    /// What the user said; partial while they speak.
    public private(set) var heard = ""
    /// When it was said, once said.
    public private(set) var heardAt: Date?
    /// A voice session runs.
    public private(set) var listening = false
    /// The microphone's audio has started coming.
    public private(set) var capturing = false
    /// The answer is being worked out.
    public private(set) var working = false
    /// The tools running or run.
    public private(set) var activity: [Activity] = []
    /// The answer, as it comes.
    public private(set) var answer = ""
    /// When the answer ended.
    public private(set) var answerAt: Date?
    /// Whether the answer is being spoken.
    public private(set) var speaking = false
    /// Whether the answer comes with speech, whose playing lights its words.
    public private(set) var voiced = false
    /// The user speaks: from user_started to user_stopped.
    public private(set) var userSpeaking = false
    /// The session listens only while held, and whether it is held.
    public private(set) var pushToTalk = false
    /// Whether the user holds to talk now.
    public private(set) var talking = false
    /// How many times the user barged in.
    public private(set) var interrupts = 0
    /// The orb and cards fade out.
    public private(set) var leaving = false
    /// Results shown, to choose from when choose is set.
    public private(set) var items: [Mavio_Library_V1_Item] = []
    /// Whether the results are to choose from.
    public private(set) var choose = false
    /// The confirmation the answer waits for.
    public private(set) var confirmation: Confirmation?
    /// Why the assistant failed, as it shows.
    public private(set) var error: String?
    /// The server whose assistant answers, when it is on.
    public private(set) var server: ServerRecord?
    /// The assistant of the server asked, while it is not on.
    public private(set) var off = false

    /// The app around the assistant.
    public weak var host: (any AssistantHost)?

    private let model: AppModel
    private var infos: [String: Mavio_Assistant_V1_GetAssistantResponse] = [:]
    @ObservationIgnored private var loading: Set<String> = []
    @ObservationIgnored private var session: AnyObject?
    // Names the session open, whose closing alone is heard.
    @ObservationIgnored private var sessionToken: UUID?
    @ObservationIgnored private var running: Task<Void, Never>?
    @ObservationIgnored private var holdTimer: Task<Void, Never>?
    @ObservationIgnored private var held: Bool?
    @ObservationIgnored private var turn = ""
    @ObservationIgnored private var lingerTimer: Task<Void, Never>?
    @ObservationIgnored private var leaveTimer: Task<Void, Never>?
    @ObservationIgnored private var failed: [String] = []

    /// The assistant of the app over model.
    public init(model: AppModel) {
        self.model = model
    }

    /// The UI's language, as the app's strings were chosen in.
    public static var uiLanguage: String { Bundle.main.preferredLocalizations.first ?? "en" }

    /// The language spoken to the assistant: the user's of the UI's
    /// language, else the UI's.
    public static var speechLanguage: String {
        let ui = uiLanguage
        let primary = { (l: String) in l.split(separator: "-").first.map(String.init) ?? l }
        return Locale.preferredLanguages.first { primary($0) == primary(ui) } ?? ui
    }

    // MARK: The server

    /// The server the assistant asks: the screen's, else the first signed
    /// in to.
    private var candidate: ServerRecord? {
        let signedIn = model.signedInServers
        if let id = host?.assistantScreen.serverId, let s = signedIn.first(where: { $0.id == id }) { return s }
        return signedIn.first
    }

    /// Reads again whether the assistant is on for the server the app
    /// shows, as it changes or appears.
    public func refresh() {
        guard let s = candidate else {
            server = nil
            return
        }
        if let info = infos[s.id] {
            server = info.enabled ? s : nil
        }
        guard !loading.contains(s.id) else { return }
        loading.insert(s.id)
        Task { [weak self] in
            var request = Mavio_Assistant_V1_GetAssistantRequest()
            request.language = Self.uiLanguage
            let info = try? await self?.model.call(
                s, Procedure.assistant("GetAssistant"), request, as: Mavio_Assistant_V1_GetAssistantResponse.self)
            guard let self else { return }
            loading.remove(s.id)
            if let info { infos[s.id] = info }
            if candidate?.id == s.id, let info = infos[s.id] {
                server = info.enabled ? s : nil
            }
        }
    }

    private var info: Mavio_Assistant_V1_GetAssistantResponse? { server.flatMap { infos[$0.id] } }

    // MARK: Pressing

    /// The button, orb or key went down.
    public func press() {
        // A conversation ends when pressed again.
        if let v = voice, !v.holdsToTalk {
            close()
            return
        }
        held = false
        holdTimer?.cancel()
        holdTimer = Task { [weak self] in
            try? await Task.sleep(for: Self.hold)
            guard !Task.isCancelled, let self else { return }
            held = true
            if let s = voice {
                if s.holdsToTalk {
                    s.talk(true)
                    talking = true
                } else {
                    connect(pushToTalk: true)
                }
            }
        }
        if voice == nil { capture() }
    }

    /// The button, orb or key went up.
    public func release() {
        guard let h = held else { return }
        held = nil
        holdTimer?.cancel()
        let v = voice
        if h {
            v?.talk(false)
            talking = false
        }
        // A tap: a conversation starts, or a session held to talk ends.
        else if v?.holdsToTalk == true {
            close()
        } else if v != nil {
            connect(pushToTalk: false)
        }
    }

    /// A tap, as a keyboard's or remote's press: starts or ends a
    /// conversation.
    public func tap() {
        press()
        release()
    }

    /// Ends the session, and the orb and cards fade out.
    public func close() {
        lingerTimer?.cancel()
        running?.cancel()
        running = nil
        voice?.close()
        session = nil
        guard open else { return }
        leaving = true
        listening = false
        speaking = false
        userSpeaking = false
        leaveTimer?.cancel()
        leaveTimer = Task { [weak self] in
            try? await Task.sleep(for: Self.leave)
            guard !Task.isCancelled else { return }
            self?.reset()
        }
    }

    private func reset() {
        open = false
        heard = ""
        heardAt = nil
        listening = false
        capturing = false
        working = false
        activity = []
        answer = ""
        answerAt = nil
        speaking = false
        voiced = false
        userSpeaking = false
        pushToTalk = false
        talking = false
        interrupts = 0
        leaving = false
        items = []
        choose = false
        confirmation = nil
        error = nil
        off = false
    }

    /// Holds the assistant while the pointer is over a card or a card is
    /// touched.
    public func hold() { lingerTimer?.cancel() }

    /// Lets the assistant go away again once the pointer leaves the cards.
    public func letGo() {
        if voice == nil && !working { lingerThenClose() }
    }

    private func lingerThenClose() {
        lingerTimer?.cancel()
        lingerTimer = Task { [weak self] in
            try? await Task.sleep(for: Self.linger)
            guard !Task.isCancelled, let self else { return }
            if !listening && !working && confirmation == nil && !choose && error == nil { close() }
        }
    }

    private func show() {
        lingerTimer?.cancel()
        leaveTimer?.cancel()
        open = true
        leaving = false
    }

    // MARK: The orb

    /// The orb's state, from the session's.
    public var mode: OrbMode {
        if leaving { return .ended }
        if error != nil { return .error }
        if speaking { return .speaking }
        if userSpeaking { return .hearing }
        if working { return .thinking }
        if listening {
            if !capturing { return .opening }
            return pushToTalk && !talking ? .resting : .idle
        }
        return .resting
    }

    /// What the pill beside the orb says, if anything.
    public var status: String? {
        switch mode {
        case .opening: MavioStrings.stateLoading
        case .idle where heard.isEmpty: MavioStrings.assistantListening
        case .resting where pushToTalk: MavioStrings.assistantShortcut(key: Self.talkKey.uppercased())
        default: nil
        }
    }

    /// The audio the orb follows: the playback's while the answer is
    /// spoken, else the microphone's.
    public func level() -> Double {
        guard let v = voice else { return 0 }
        return speaking ? v.outputLevel() : v.inputLevel()
    }

    /// How much of the answer's speech has played and has come, in
    /// milliseconds.
    public func speech() -> (played: Double, total: Double)? { voice?.speech() }

    /// Whether the answer's words are lit as they are spoken.
    public var answerLive: Bool { voiced && (speaking || working || answerAt == nil) }

    // MARK: Answering

    /// Answers the pending confirmation by its buttons.
    public func confirm(_ confirmed: Bool) {
        guard let c = confirmation else { return }
        var answer = Mavio_Assistant_V1_ConfirmationAnswer()
        answer.id = c.id
        answer.confirmed = confirmed
        var request = Mavio_Assistant_V1_AskRequest()
        request.answer = answer
        ask(request, choosing: false)
    }

    /// Chooses a result, or opens it when there is nothing to choose.
    public func choose(_ item: Mavio_Library_V1_Item) {
        if choose {
            var request = Mavio_Assistant_V1_AskRequest()
            request.choice = item.id
            ask(request, choosing: true)
        } else if let server {
            var action = Mavio_Assistant_V1_NavigateAction()
            action.target = .item
            action.id = item.id
            _ = host?.assistantNavigate(action, serverId: server.id)
        }
    }

    /// Asks with text the user dictated, as Apple TV's dictation returns it
    /// (docs/assistant.md §9.3), shown as a voice session's turn.
    public func ask(dictated text: String) {
        guard !text.isEmpty else { return }
        heard = text
        heardAt = Date()
        var request = Mavio_Assistant_V1_AskRequest()
        request.text = text
        request.spoken = true
        ask(request, choosing: false)
    }

    /// Tells the session what the app shows, as it changes.
    public func contextChanged() {
        guard let v = voice, let data = try? context().jsonUTF8Data(),
            let json = try? JSONSerialization.jsonObject(with: data)
        else { return }
        v.send(["type": "context", "context": json])
    }

    /// What the app shows and plays, as the requests' context.
    private func context() -> Mavio_Assistant_V1_AskContext {
        var c = Mavio_Assistant_V1_AskContext()
        var languages: [String] = []
        for l in [Self.speechLanguage, Self.uiLanguage] where !languages.contains(l) { languages.append(l) }
        c.languages = languages
        let screen = host?.assistantScreen ?? AssistantScreen("")
        c.screen = screen.name
        c.itemID = screen.itemId
        c.libraryID = screen.libraryId
        c.timeZone = TimeZone.current.identifier
        if let p = host?.assistantPlayer { c.playing = p.assistantPlaying }
        return c
    }

    private func ask(_ request: Mavio_Assistant_V1_AskRequest, choosing: Bool) {
        guard let server else { return }
        show()
        running?.cancel()
        voiced = false
        working = true
        activity = []
        answer = ""
        answerAt = nil
        error = nil
        confirmation = nil
        if choosing {
            items = []
            choose = false
        }
        var request = request
        request.context = context()
        request.failedActions = failed
        failed = []
        let events = model.client.callStream(
            server.id, Procedure.assistant("Ask"), request, as: Mavio_Assistant_V1_AskResponse.self)
        running = Task { [weak self] in
            // Whether the answer waits for the user: a confirmation, or
            // results to choose from.
            var waits = false
            do {
                for try await e in events {
                    guard let self, !Task.isCancelled else { return }
                    switch e.event {
                    case .understood(let u):
                        if !u.text.isEmpty { heard = u.text }
                    case .tool(let t):
                        tool(Activity(id: t.id, summary: t.summary, done: t.done, failed: t.failed))
                    case .action(let a):
                        if !perform(a, serverId: server.id) { failed.append(a.id) }
                        if case .showItems(let s) = a.kind, s.choose { waits = true }
                    case .confirmation(let c):
                        waits = true
                        confirmation = Confirmation(id: c.id, text: c.text)
                    case .answer(let text):
                        answer += text
                    case .done(let d):
                        if !d.answer.isEmpty { answer = d.answer }
                    case nil:
                        break
                    }
                }
                guard let self, !Task.isCancelled else { return }
                working = false
                answerAt = Date()
                if !failed.isEmpty && answer.isEmpty { error = MavioStrings.assistantCantDo }
                if !waits { lingerThenClose() }
            } catch {
                guard let self, !Task.isCancelled else { return }
                working = false
                self.error = error.reason
            }
        }
    }

    private func tool(_ t: Activity) {
        activity = activity.filter { $0.id != t.id } + [t]
    }

    /// Carries out a client action; false when the app cannot.
    private func perform(_ a: Mavio_Assistant_V1_ClientAction, serverId: String) -> Bool {
        switch a.kind {
        case .play(let p):
            return host?.assistantPlay(p, serverId: serverId) ?? false
        case .control(let c):
            return host?.assistantPlayer?.assistantControl(c.command, value: c.value) ?? false
        case .streams(let s):
            // The players shift no subtitles.
            guard let p = host?.assistantPlayer, s.subtitleOffset == 0 else { return false }
            return p.assistantStreams(
                audio: s.hasAudioStreamIndex ? s.audioStreamIndex : nil,
                subtitle: s.hasSubtitleStreamIndex ? s.subtitleStreamIndex : nil)
        case .navigate(let n):
            return host?.assistantNavigate(n, serverId: serverId) ?? false
        case .showItems(let s):
            items = s.items
            choose = s.choose
            return true
        case nil:
            return false
        }
    }

    // MARK: Voice

    private var voice: VoiceSession? { session as? VoiceSession }

    /// Opens the microphone, and the voice session once it is known
    /// whether the user holds to talk.
    private func capture() {
        guard let server else { return }
        show()
        running?.cancel()
        guard info?.voice == true else {
            error = MavioStrings.assistantNoVoice
            return
        }
        turn = ""
        listening = true
        capturing = false
        userSpeaking = false
        speaking = false
        voiced = false
        pushToTalk = false
        talking = false
        heard = ""
        heardAt = nil
        answer = ""
        answerAt = nil
        activity = []
        error = nil
        if !choose { items = [] }
        Task { [weak self] in
            do {
                let token = UUID()
                let v = try await VoiceSession.capture(
                    .init(
                        event: { [weak self] e in self?.voiceEvent(e, serverId: server.id) },
                        closed: { [weak self] unreachable, error in
                            guard let self, sessionToken == token else { return }
                            session = nil
                            listening = false
                            capturing = false
                            speaking = false
                            userSpeaking = false
                            working = false
                            if unreachable {
                                self.error = MavioStrings.homeServerUnreachable(server: server.name)
                            } else if let error {
                                self.error = error
                            } else {
                                lingerThenClose()
                            }
                        },
                        capturing: { [weak self] in self?.capturing = true },
                        silent: { [weak self] device in
                            self?.error = MavioStrings.assistantSilentMicrophone(
                                device: device.isEmpty ? MavioStrings.assistantMicrophone : device)
                        }))
                guard let self else {
                    v.close()
                    return
                }
                guard listening, !leaving else {
                    v.close()
                    return
                }
                session = v
                sessionToken = token
                // Let go before the microphone opened: a tap.
                if held == nil {
                    connect(pushToTalk: false)
                } else if held == true {
                    connect(pushToTalk: true)
                }
            } catch {
                guard let self else { return }
                listening = false
                if case VoiceSession.Failure.denied = error {
                    self.error = MavioStrings.assistantMicrophoneDenied
                } else if case VoiceSession.Failure.unavailable(let reason) = error {
                    self.error = MavioStrings.assistantNoMicrophone(error: reason)
                } else {
                    self.error = MavioStrings.assistantNoMicrophone(error: error.reason)
                }
            }
        }
    }

    /// Starts the session on the server: a conversation, or talking
    /// while held.
    private func connect(pushToTalk: Bool) {
        guard let v = voice, let server else { return }
        var request = Mavio_Assistant_V1_StartVoiceSessionRequest()
        request.language = Self.speechLanguage
        request.mode = pushToTalk ? .pushToTalk : .conversation
        request.context = context()
        let model = model
        v.connect(
            {
                let r = try await model.call(
                    server, Procedure.assistant("StartVoiceSession"), request,
                    as: Mavio_Assistant_V1_StartVoiceSessionResponse.self)
                guard let url = Self.socketURL(r.url, server: server) else { throw URLError(.badURL) }
                return url
            }, pushToTalk: pushToTalk)
        self.pushToTalk = pushToTalk
        talking = pushToTalk
        if pushToTalk { v.talk(true) }
    }

    /// The session's WebSocket URL, from the path the server gives.
    public static func socketURL(_ path: String, server: ServerRecord) -> URL? {
        var text = path
        if !path.hasPrefix("http") && !path.hasPrefix("ws") {
            var base = server.address
            while base.hasSuffix("/") { base.removeLast() }
            text = base + (path.hasPrefix("/") ? path : "/" + path)
        }
        if text.hasPrefix("https:") {
            text = "wss:" + text.dropFirst(6)
        } else if text.hasPrefix("http:") {
            text = "ws:" + text.dropFirst(5)
        }
        return URL(string: text)
    }

    /// Carries out what the voice session tells (docs/assistant.md §8.6).
    private func voiceEvent(_ e: [String: Any], serverId: String) {
        let text = e["text"] as? String ?? ""
        switch e["type"] as? String {
        case "user_started":
            lingerTimer?.cancel()
            userSpeaking = true
        case "user_stopped":
            userSpeaking = false
        case "interrupt":
            interrupts += 1
            speaking = false
            userSpeaking = true
        case "transcript":
            let t = text.trimmingCharacters(in: .whitespaces)
            let final = e["final"] as? Bool ?? false
            if final { turn = [turn, t].filter { !$0.isEmpty }.joined(separator: " ") }
            heard = final ? turn : [turn, t].filter { !$0.isEmpty }.joined(separator: " ")
            heardAt = nil
        case "run":
            turn = ""
            if !text.isEmpty { heard = text }
            heardAt = Date()
            working = true
            userSpeaking = false
            voiced = false
            activity = []
            answer = ""
            answerAt = nil
            error = nil
            confirmation = nil
            if !choose { items = [] }
        case "intent":
            if !text.isEmpty { heard = text }
        case "tool":
            tool(
                Activity(
                    id: "\(e["id"] ?? "")", summary: e["summary"] as? String ?? "",
                    done: e["done"] as? Bool ?? false, failed: e["failed"] as? Bool ?? false))
        case "action":
            guard let json = e["action"], let data = try? JSONSerialization.data(withJSONObject: json) else { break }
            var options = JSONDecodingOptions()
            options.ignoreUnknownFields = true
            if let a = try? Mavio_Assistant_V1_ClientAction(jsonUTF8Data: data, options: options) {
                _ = perform(a, serverId: serverId)
            }
        case "confirm":
            confirmation = Confirmation(id: "\(e["id"] ?? "")", text: text)
        case "answer":
            answer += text
        case "end":
            working = false
            if let a = e["answer"] as? String, !a.isEmpty { answer = a }
            answerAt = Date()
            if e["status"] as? String == "failed" && answer.isEmpty { error = MavioStrings.assistantCantDo }
        case "bot_started":
            speaking = true
            voiced = true
        case "bot_stopped":
            speaking = false
        case "error":
            working = false
            let message = e["message"] as? String ?? ""
            error = message.isEmpty ? "\(e["code"] ?? "")" : message
        default:
            break
        }
    }
}

/// The answer's words, each with when it has been said, roughly, in
/// milliseconds from the speech's start: they are lit against how much of
/// the speech has played.
public enum SpokenText {
    /// The words and the spaces between them of text, each with when it has
    /// been said.
    public static func segments(_ text: String) -> [(text: String, at: Double)] {
        var words: [Range<String.Index>] = []
        text.enumerateSubstrings(in: text.startIndex..., options: [.byWords, .substringNotRequired]) { _, r, _, _ in
            words.append(r)
        }
        var parts: [Substring] = []
        var at = text.startIndex
        for r in words {
            if at < r.lowerBound { parts.append(text[at..<r.lowerBound]) }
            parts.append(text[r])
            at = r.upperBound
        }
        if at < text.endIndex { parts.append(text[at...]) }
        var ms = 0.0
        return parts.map { p in
            ms += sayMs(p)
            return (String(p), ms)
        }
    }

    // How long a character takes to say, roughly.
    private static func sayMs(_ s: Substring) -> Double {
        var ms = 0.0
        for ch in s.unicodeScalars {
            switch ch.properties.generalCategory {
            case .otherLetter where cjk(ch): ms += 230
            case .uppercaseLetter, .lowercaseLetter, .titlecaseLetter, .modifierLetter, .otherLetter,
                .decimalNumber, .letterNumber, .otherNumber:
                ms += 62
            default:
                if ",.;:!?，。；：！？、".unicodeScalars.contains(ch) { ms += 180 }
            }
        }
        return ms
    }

    private static func cjk(_ c: Unicode.Scalar) -> Bool {
        switch c.value {
        case 0x3040...0x30FF, 0x3400...0x4DBF, 0x4E00...0x9FFF, 0xAC00...0xD7AF, 0xF900...0xFAFF, 0x20000...0x2FFFF:
            true
        default: false
        }
    }
}

extension Assistant {
    /// What a player plays, as the requests' context tells it: the streams
    /// by their index in the media, nil for none.
    public static func playing(
        itemId: String, position: TimeInterval, paused: Bool, speed: Double, audio: Int32?, subtitle: Int32?,
        volume: Int32 = 100, muted: Bool = false
    ) -> Mavio_Assistant_V1_AskPlaying {
        var p = Mavio_Assistant_V1_AskPlaying()
        p.itemID = itemId
        p.position = Google_Protobuf_Duration(timeInterval: max(0, position))
        p.paused = paused
        p.speed = speed
        p.audioStreamIndex = audio ?? -1
        p.subtitleStreamIndex = subtitle ?? -1
        p.volume = volume
        p.muted = muted
        return p
    }

    /// The poster of a result, maxWidth points wide at most at scale, as
    /// the server signed it.
    public func poster(_ item: Mavio_Library_V1_Item, maxWidth: Int, scale: Double = 2) -> String? {
        guard let server,
            let image = item.images.filter({ $0.kind == .primary && !$0.url.isEmpty }).min(by: { $0.index < $1.index })
        else { return nil }
        var url = image.url
        if !url.hasPrefix("http") {
            var base = server.address
            while base.hasSuffix("/") { base.removeLast() }
            url = base + (url.hasPrefix("/") ? url : "/" + url)
        }
        return url + "?maxWidth=\(Int((Double(maxWidth) * min(scale, 3)).rounded()))"
    }

    /// What a result's row says under its title: the series and episode,
    /// the artists, or the year.
    public static func subtitle(_ item: Mavio_Library_V1_Item) -> String? {
        switch item.kind {
        case .episode:
            let n =
                item.parentIndexNumber > 0 && item.indexNumber > 0
                ? "S\(item.parentIndexNumber):E\(item.indexNumber)"
                : item.indexNumber > 0 ? "E\(item.indexNumber)" : ""
            let s = [item.seriesName, n].filter { !$0.isEmpty }.joined(separator: " · ")
            return s.isEmpty ? nil : s
        case .season:
            return item.seriesName.isEmpty ? nil : item.seriesName
        case .musicAlbum, .track:
            let s = (item.albumArtists.isEmpty ? item.artists : item.albumArtists).joined(separator: ", ")
            return s.isEmpty ? nil : s
        default:
            return item.productionYear > 0 ? String(item.productionYear) : nil
        }
    }
}
