import AVFoundation
import Foundation
import MavioKit
import Observation

/// The Assistant settings of This Device (docs/clients.md §6.3.29), as the
/// web UI's, for the iPhone and iPad app and the Apple TV app: whether the
/// server's assistant is on, the user's memories, each forgotten, a new one
/// remembered or all forgotten, and the user's conversations, each read
/// with its messages or deleted.
@MainActor
@Observable
public final class AssistantPrefs {
    /// What the server's assistant gives the user, nil until known.
    public private(set) var info: Mavio_Assistant_V1_GetAssistantResponse?
    /// Why the assistant could not be known.
    public private(set) var failure: String?
    /// The user's memories, nil until listed.
    public private(set) var memories: [Mavio_Assistant_V1_AssistantMemory]?
    /// Why the memories could not be listed or changed.
    public private(set) var memoryFailure: String?
    /// The user's conversations, newest first, nil until listed.
    public private(set) var conversations: [Mavio_Assistant_V1_AssistantConversation]?
    /// Why the conversations could not be listed.
    public private(set) var conversationsFailure: String?
    /// The conversation open, by its ID.
    public private(set) var shown: String?
    /// The messages of the conversation open, nil while they load.
    public private(set) var messages: [Mavio_Assistant_V1_AssistantMessage]?
    /// Why the conversation open could not be read or deleted.
    public private(set) var messagesFailure: String?
    /// Whether a change is under way.
    public private(set) var busy = false
    /// The kept speech of the answers.
    public let kept: KeptSpeech

    @ObservationIgnored private let model: AppModel
    @ObservationIgnored private let server: ServerRecord

    /// The Assistant settings of the user on server.
    public init(model: AppModel, server: ServerRecord) {
        self.model = model
        self.server = server
        kept = KeptSpeech(model: model, server: server)
    }

    /// Learns what the assistant gives the user, then lists the memories
    /// and conversations it keeps.
    public func load() async {
        var request = Mavio_Assistant_V1_GetAssistantRequest()
        request.language = Assistant.uiLanguage
        do {
            info = try await model.call(
                server, Procedure.assistant("GetAssistant"), request, as: Mavio_Assistant_V1_GetAssistantResponse.self)
            failure = nil
        } catch is CancellationError {
            return
        } catch {
            failure = error.reason
        }
        guard let info, info.enabled else { return }
        async let m: Void = loadMemories()
        async let c: Void = info.ai ? loadConversations() : ()
        _ = await (m, c)
    }

    private func loadMemories() async {
        guard info?.memory == true else { return }
        do {
            memories = try await model.call(
                server, Procedure.assistant("ListAssistantMemories"), Mavio_Assistant_V1_ListAssistantMemoriesRequest(),
                as: Mavio_Assistant_V1_ListAssistantMemoriesResponse.self
            ).memories
        } catch is CancellationError {
        } catch {
            memoryFailure = error.reason
        }
    }

    private func loadConversations() async {
        do {
            conversations = try await model.call(
                server, Procedure.assistant("ListAssistantConversations"),
                Mavio_Assistant_V1_ListAssistantConversationsRequest(),
                as: Mavio_Assistant_V1_ListAssistantConversationsResponse.self
            ).conversations
            conversationsFailure = nil
        } catch is CancellationError {
        } catch {
            conversationsFailure = error.reason
        }
    }

    /// Remembers text, returning whether it was.
    @discardableResult
    public func remember(_ text: String) async -> Bool {
        let text = text.trimmingCharacters(in: .whitespacesAndNewlines)
        guard !text.isEmpty else { return false }
        var request = Mavio_Assistant_V1_UpdateAssistantMemoryRequest()
        request.text = String(text.prefix(1000))
        return await change {
            _ = try await self.model.call(
                self.server, Procedure.assistant("UpdateAssistantMemory"), request,
                as: Mavio_Assistant_V1_UpdateAssistantMemoryResponse.self)
        }
    }

    /// Forgets the memories of ids, all of them for none.
    public func forget(_ ids: [String]) async {
        var request = Mavio_Assistant_V1_DeleteAssistantMemoriesRequest()
        request.ids = ids
        await change {
            _ = try await self.model.call(
                self.server, Procedure.assistant("DeleteAssistantMemories"), request,
                as: Mavio_Assistant_V1_DeleteAssistantMemoriesResponse.self)
        }
    }

    @discardableResult
    private func change(_ work: () async throws -> Void) async -> Bool {
        busy = true
        memoryFailure = nil
        defer { busy = false }
        do {
            try await work()
        } catch is CancellationError {
            return false
        } catch {
            memoryFailure = error.reason
            return false
        }
        await loadMemories()
        return true
    }

    /// Opens a conversation and reads its messages, or closes it when open.
    public func toggle(_ id: String) async {
        kept.stop()
        guard shown != id else {
            shown = nil
            return
        }
        shown = id
        messages = nil
        messagesFailure = nil
        var request = Mavio_Assistant_V1_GetAssistantConversationRequest()
        request.id = id
        do {
            let r = try await model.call(
                server, Procedure.assistant("GetAssistantConversation"), request,
                as: Mavio_Assistant_V1_GetAssistantConversationResponse.self)
            if shown == id { messages = r.messages }
        } catch is CancellationError {
        } catch {
            if shown == id { messagesFailure = error.reason }
        }
    }

    /// Deletes a conversation.
    public func delete(_ id: String) async {
        var request = Mavio_Assistant_V1_DeleteAssistantConversationRequest()
        request.id = id
        busy = true
        defer { busy = false }
        do {
            _ = try await model.call(
                server, Procedure.assistant("DeleteAssistantConversation"), request,
                as: Mavio_Assistant_V1_DeleteAssistantConversationResponse.self)
            kept.stop()
            shown = nil
            await loadConversations()
        } catch is CancellationError {
        } catch {
            messagesFailure = error.reason
        }
    }
}

extension Mavio_Assistant_V1_AssistantConversation {
    /// The conversation's name and detail: the assistant's conversations
    /// have no titles, so they go by when they were, and where.
    public var label: (title: String, detail: String) {
        let when = hasUpdateTime ? updateTime.date.formatted(date: .abbreviated, time: .shortened) : ""
        let detail = [deviceName, title.isEmpty ? "" : when].filter { !$0.isEmpty }.joined(separator: " · ")
        return (title.isEmpty ? when : title, detail)
    }
}

/// Plays the speech the server kept of the assistant's answers
/// (docs/assistant.md §8.8), one answer at a time: only those files, never
/// a voice speaking again.
@MainActor
@Observable
public final class KeptSpeech: NSObject, AVAudioPlayerDelegate {
    /// The answer playing, by its key.
    public private(set) var at: String?

    @ObservationIgnored private let model: AppModel
    @ObservationIgnored private let server: ServerRecord
    @ObservationIgnored private var player: AVAudioPlayer?
    @ObservationIgnored private var loading: Task<Void, Never>?

    /// The kept speech of server's answers.
    public init(model: AppModel, server: ServerRecord) {
        self.model = model
        self.server = server
    }

    isolated deinit {
        loading?.cancel()
        player?.stop()
    }

    /// Plays the speech at url as the answer key, or stops it when it plays.
    public func toggle(_ key: String, url: String) {
        let again = at == key
        stop()
        guard !again else { return }
        at = key
        // The encodings AVAudioPlayer plays, the smallest first.
        let path = url + (url.contains("?") ? "&" : "?") + "formats=aac,mp3,flac,wav"
        let (model, server) = (model, server)
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
                #if !os(macOS)
                    try? AVAudioSession.sharedInstance().setCategory(.playback, mode: .spokenAudio)
                    try? AVAudioSession.sharedInstance().setActive(true)
                #endif
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
    public func stop() {
        loading?.cancel()
        loading = nil
        player?.stop()
        player = nil
        at = nil
    }

    private func finish() {
        player = nil
        at = nil
    }

    /// The answer played to its end.
    public nonisolated func audioPlayerDidFinishPlaying(_ player: AVAudioPlayer, successfully _: Bool) {
        let id = ObjectIdentifier(player)
        MainActor.assumeIsolated {
            if self.player.map(ObjectIdentifier.init) == id { finish() }
        }
    }
}
