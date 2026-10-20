#if os(macOS)
    import Foundation
    import MavioMac

    /// A request of a movie or of seasons of a series.
    typealias MediaRequest = Mavio_Acquisition_V1_Request

    /// The requests' calls, as the web UI's useRequestTitle and
    /// useRequestActions.
    @MainActor
    enum RequestsAPI {
        /// Requests a title; a series' seasons, or all of them when nil.
        static func create(_ model: AppModel, _ serverId: String, _ kind: MediaKind, _ tmdbId: Int32, seasons: [Int32]?)
            async throws
        {
            var r = Mavio_Acquisition_V1_CreateRequestRequest()
            r.mediaKind = kind
            r.tmdbID = tmdbId
            r.seasons = seasons ?? []
            r.allSeasons = kind == .series && seasons == nil
            _ = try await model.client.call(
                serverId, Procedure.requests("CreateRequest"), r, as: Mavio_Acquisition_V1_CreateRequestResponse.self)
        }

        /// Approves a pending request.
        static func approve(_ model: AppModel, _ serverId: String, _ id: String) async throws {
            var r = Mavio_Acquisition_V1_ApproveRequestRequest()
            r.id = id
            _ = try await model.client.call(
                serverId, Procedure.requests("ApproveRequest"), r, as: Mavio_Acquisition_V1_ApproveRequestResponse.self)
        }

        /// Declines a pending request.
        static func decline(_ model: AppModel, _ serverId: String, _ id: String) async throws {
            var r = Mavio_Acquisition_V1_DeclineRequestRequest()
            r.id = id
            _ = try await model.client.call(
                serverId, Procedure.requests("DeclineRequest"), r, as: Mavio_Acquisition_V1_DeclineRequestResponse.self)
        }

        /// Tries a failed request again.
        static func retry(_ model: AppModel, _ serverId: String, _ id: String) async throws {
            var r = Mavio_Acquisition_V1_RetryRequestRequest()
            r.id = id
            _ = try await model.client.call(
                serverId, Procedure.requests("RetryRequest"), r, as: Mavio_Acquisition_V1_RetryRequestResponse.self)
        }

        /// Changes the seasons of a pending series request.
        static func update(_ model: AppModel, _ serverId: String, _ id: String, seasons: [Int32]) async throws {
            var r = Mavio_Acquisition_V1_UpdateRequestRequest()
            r.id = id
            r.seasons = seasons
            _ = try await model.client.call(
                serverId, Procedure.requests("UpdateRequest"), r, as: Mavio_Acquisition_V1_UpdateRequestResponse.self)
        }

        /// Deletes a request.
        static func remove(_ model: AppModel, _ serverId: String, _ id: String) async throws {
            var r = Mavio_Acquisition_V1_DeleteRequestRequest()
            r.id = id
            _ = try await model.client.call(
                serverId, Procedure.requests("DeleteRequest"), r, as: Mavio_Acquisition_V1_DeleteRequestResponse.self)
        }

        /// Blocklists a title, or takes it off the blocklist.
        static func block(_ model: AppModel, _ serverId: String, _ kind: MediaKind, _ tmdbId: Int32, _ on: Bool)
            async throws
        {
            if on {
                var r = Mavio_Acquisition_V1_BlockTitleRequest()
                r.mediaKind = kind
                r.tmdbID = tmdbId
                _ = try await model.client.call(
                    serverId, Procedure.requests("BlockTitle"), r, as: Mavio_Acquisition_V1_BlockTitleResponse.self)
            } else {
                var r = Mavio_Acquisition_V1_UnblockTitleRequest()
                r.mediaKind = kind
                r.tmdbID = tmdbId
                _ = try await model.client.call(
                    serverId, Procedure.requests("UnblockTitle"), r, as: Mavio_Acquisition_V1_UnblockTitleResponse.self)
            }
        }

        /// Blocklists every movie of a collection, or takes them off it.
        static func blockCollection(_ model: AppModel, _ serverId: String, _ tmdbId: Int32, _ on: Bool) async throws {
            if on {
                var r = Mavio_Acquisition_V1_BlockCollectionRequest()
                r.tmdbCollectionID = tmdbId
                _ = try await model.client.call(
                    serverId, Procedure.requests("BlockCollection"), r,
                    as: Mavio_Acquisition_V1_BlockCollectionResponse.self)
            } else {
                var r = Mavio_Acquisition_V1_UnblockCollectionRequest()
                r.tmdbCollectionID = tmdbId
                _ = try await model.client.call(
                    serverId, Procedure.requests("UnblockCollection"), r,
                    as: Mavio_Acquisition_V1_UnblockCollectionResponse.self)
            }
        }

        /// The user signed in to a server.
        static func currentUser(_ model: AppModel, _ serverId: String) async -> Mavio_User_V1_User? {
            try? await model.client.call(
                serverId, Procedure.users("GetCurrentUser"), Mavio_User_V1_GetCurrentUserRequest(),
                as: Mavio_User_V1_GetCurrentUserResponse.self
            ).user
        }

        /// The names of a server's users by their IDs, for its
        /// administrators.
        static func userNames(_ model: AppModel, _ serverId: String) async -> [String: String] {
            let users =
                (try? await model.client.call(
                    serverId, Procedure.users("ListUsers"), Mavio_User_V1_ListUsersRequest(),
                    as: Mavio_User_V1_ListUsersResponse.self))?.users ?? []
            return Dictionary(users.map { ($0.id, $0.name) }, uniquingKeysWith: { a, _ in a })
        }

        /// The downloads of an item in the queue.
        static func queue(_ model: AppModel, _ serverId: String, _ itemId: String) async throws
            -> [Mavio_Acquisition_V1_QueueItem]
        {
            var r = Mavio_Acquisition_V1_ListQueueRequest()
            r.itemID = itemId
            r.limit = 50
            return try await model.client.call(
                serverId, Procedure.downloads("ListQueue"), r, as: Mavio_Acquisition_V1_ListQueueResponse.self
            ).items
        }

        /// How far an item's downloads have come, in percent; nil while
        /// none downloads.
        static func downloading(_ model: AppModel, _ serverId: String, _ itemId: String) async -> Int? {
            guard let items = try? await queue(model, serverId, itemId) else { return nil }
            let downloading = items.filter { $0.state == .downloading && $0.size > 0 }
            guard !downloading.isEmpty else { return nil }
            let size = Double(downloading.reduce(0) { $0 + $1.size })
            let left = Double(downloading.reduce(0) { $0 + $1.remainingSize })
            return Int((max(0, min(1, 1 - left / size)) * 100).rounded())
        }
    }

    /// The name of a request's status.
    func requestStatusName(_ status: Mavio_Acquisition_V1_RequestStatus) -> String {
        switch status {
        case .pending: MavioStrings.requestsPending
        case .approved: MavioStrings.requestsApproved
        case .completed: MavioStrings.requestsAvailable
        case .failed: MavioStrings.requestsFailed
        case .declined: MavioStrings.requestsDeclined
        default: ""
        }
    }

    /// The name of a download's state.
    func downloadStateName(_ s: Mavio_Acquisition_V1_DownloadState) -> String {
        switch s {
        case .downloading: MavioStrings.downloadsDownloading
        case .importBlocked: MavioStrings.downloadsImportBlocked
        case .importPending: MavioStrings.downloadsImportPending
        case .importing: MavioStrings.downloadsImporting
        case .imported: MavioStrings.downloadsImported
        case .failedPending, .failed: MavioStrings.downloadsFailed
        case .ignored: MavioStrings.downloadsIgnored
        default: ""
        }
    }

    /// The name of a download's status, when worth saying.
    func downloadStatusName(_ s: Mavio_Acquisition_V1_DownloadStatus) -> String? {
        switch s {
        case .queued: MavioStrings.downloadsQueued
        case .paused: MavioStrings.downloadsPaused
        case .warning: MavioStrings.downloadsWarning
        default: nil
        }
    }

    /// A size in bytes, in gigabytes from one, else in megabytes, as the web
    /// UI's.
    func sizeOf(_ bytes: Int64) -> String {
        let n = Double(bytes)
        let unit: UnitInformationStorage = n >= 1e9 ? .gigabytes : .megabytes
        return Measurement(value: n / (n >= 1e9 ? 1e9 : 1e6), unit: unit)
            .formatted(
                .measurement(
                    width: .abbreviated, usage: .asProvided,
                    numberFormatStyle: .number.precision(.fractionLength(0...1))))
    }

    /// A length of time, as "1 hr, 30 min".
    func lengthOf(_ seconds: TimeInterval) -> String {
        Duration.seconds(seconds).formatted(
            .units(allowed: [.days, .hours, .minutes, .seconds], width: .abbreviated, maximumUnitCount: 2))
    }
#endif
