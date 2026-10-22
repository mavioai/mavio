import Foundation
import MavioDesign
import MavioKit

// An item's metadata from its More menu, for administrators
// (docs/clients.md §6.3.22): its fields edited and locked, the item
// identified anew from the providers' search results, its metadata
// refreshed, and its images chosen from the providers' or uploaded. What
// the apps' dialogs share.

/// Whether the providers identify items of a kind.
public func identifiable(_ kind: Mavio_Library_V1_ItemKind) -> Bool {
    [.movie, .series, .musicArtist, .musicAlbum, .musicVideo, .book, .audiobook].contains(kind)
}

/// Whether a device plays items of a kind by themselves, as Watch Together
/// plays them.
public func playable(_ kind: CardKind) -> Bool {
    [.movie, .episode, .video, .musicVideo, .track, .audiobook].contains(kind)
}

/// The kind of an item's card as the server's.
public func itemKind(_ kind: CardKind) -> Mavio_Library_V1_ItemKind {
    switch kind {
    case .movie: .movie
    case .series: .series
    case .season: .season
    case .episode: .episode
    case .video: .video
    case .artist: .musicArtist
    case .album: .musicAlbum
    case .track: .track
    case .musicVideo: .musicVideo
    case .audiobook: .audiobook
    case .book: .book
    case .photoAlbum: .photoAlbum
    case .photo: .photo
    case .folder: .folder
    case .collection: .collection
    case .playlist: .playlist
    case .other: .unspecified
    }
}

/// What an administrator's More menu has for an item's metadata: editing it,
/// identifying it, refreshing it and its images; a playlist has none, a
/// collection is not refreshed, and only the kinds the providers know are
/// identified.
public enum MetadataAction: Hashable, Identifiable, Sendable {
    /// Editing its fields.
    case edit
    /// Identifying it from the providers' search results.
    case identify
    /// Refreshing its metadata.
    case refresh
    /// Choosing its images.
    case images

    /// The action itself.
    public var id: Self { self }

    /// The menu's title.
    public var title: String {
        switch self {
        case .edit: MavioStrings.metadataEdit
        case .identify: MavioStrings.metadataIdentify
        case .refresh: MavioStrings.metadataRefresh
        case .images: MavioStrings.metadataImages
        }
    }

    /// The actions on an item of a kind.
    public static func of(_ kind: Mavio_Library_V1_ItemKind) -> [MetadataAction] {
        guard kind != .playlist else { return [] }
        return [.edit] + (identifiable(kind) ? [.identify] : []) + (kind == .collection ? [] : [.refresh]) + [.images]
    }
}

/// The editable fields as a form holds them: text, and toggles.
public struct MetadataForm: Equatable, Sendable {
    /// The name.
    public var name = ""
    /// The name in its original language.
    public var originalTitle = ""
    /// The name it sorts by, empty for the name.
    public var sortName = ""
    /// The overview.
    public var overview = ""
    /// The tagline.
    public var tagline = ""
    /// The year.
    public var productionYear = ""
    /// A day as "2024-05-31", empty for none.
    public var premiereDate = ""
    /// A day as "2024-05-31", empty for none.
    public var endDate = ""
    /// Minutes.
    public var runtime = ""
    /// The official rating, as "PG-13".
    public var officialRating = ""
    /// A rating of the server's own.
    public var customRating = ""
    /// The community's rating, out of 10.
    public var communityRating = ""
    /// The critics' rating, out of 100.
    public var criticRating = ""
    /// Separated by commas, as the rest of the lists.
    public var genres = ""
    /// The tags.
    public var tags = ""
    /// The studios.
    public var studios = ""
    /// The countries it was made in.
    public var productionLocations = ""
    /// The artists.
    public var artists = ""
    /// The album's artists.
    public var albumArtists = ""
    /// The season's, episode's or track's number.
    public var indexNumber = ""
    /// An episode's season number.
    public var parentIndexNumber = ""
    /// An episode's number across seasons.
    public var absoluteNumber = ""
    /// The language the providers describe it in, as "en", empty for its
    /// library's.
    public var metadataLanguage = ""
    /// The country the providers describe it for, as "US", empty for its
    /// library's.
    public var metadataCountry = ""
    /// A series' status.
    public var seriesStatus = Mavio_Library_V1_SeriesStatus.unspecified
    /// The external IDs, by the kind's key.
    public var externalIds: [String: String] = [:]
    /// Whether every field is locked.
    public var locked = false
    /// The fields locked one by one, while not all are.
    public var lockedFields: [Mavio_Library_V1_MetadataField] = []

    /// The fields locked one by one, with their names.
    public static let lockable: [(Mavio_Library_V1_MetadataField, String)] = [
        (.name, MavioStrings.metadataLockName), (.overview, MavioStrings.metadataOverview),
        (.genres, MavioStrings.metadataGenres), (.tags, MavioStrings.metadataTags),
        (.studios, MavioStrings.metadataStudios), (.productionLocations, MavioStrings.metadataLocations),
        (.officialRating, MavioStrings.metadataOfficialRating), (.runtime, MavioStrings.metadataRuntime),
        (.cast, MavioStrings.metadataCast),
    ]

    /// The statuses of a series, with their names.
    public static let statuses: [(Mavio_Library_V1_SeriesStatus, String)] = [
        (.unspecified, MavioStrings.metadataStatusUnknown), (.continuing, MavioStrings.metadataContinuing),
        (.ended, MavioStrings.metadataEnded), (.unreleased, MavioStrings.metadataUnreleased),
    ]

    private static let days: DateFormatter = {
        let f = DateFormatter()
        f.locale = Locale(identifier: "en_US_POSIX")
        f.timeZone = TimeZone(identifier: "UTC")
        f.dateFormat = "yyyy-MM-dd"
        return f
    }()

    /// A day of the form as a date, nil when empty or not a day.
    public static func date(_ day: String) -> Date? { days.date(from: day.trimmingCharacters(in: .whitespaces)) }

    /// A date as a day of the form.
    public static func day(_ date: Date?) -> String { date.map(days.string) ?? "" }

    /// An empty form.
    public init() {}

    /// The form of an item's fields.
    public init(_ item: Mavio_Library_V1_Item) {
        let list = { (v: [String]) in v.joined(separator: ", ") }
        let number = { (n: Int32) in n == 0 ? "" : "\(n)" }
        let rating = { (n: Double) in n == 0 ? "" : n.formatted(.number.grouping(.never)) }
        name = item.name
        originalTitle = item.originalTitle
        sortName = item.sortName
        overview = item.overview
        tagline = item.tagline
        productionYear = number(item.productionYear)
        premiereDate = item.hasPremiereDate ? Self.day(item.premiereDate.date) : ""
        endDate = item.hasEndDate ? Self.day(item.endDate.date) : ""
        runtime = item.hasRuntime ? "\(Int((item.runtime.timeInterval / 60).rounded()))" : ""
        officialRating = item.officialRating
        customRating = item.customRating
        communityRating = rating(item.communityRating)
        criticRating = rating(item.criticRating)
        genres = list(item.genres)
        tags = list(item.tags)
        studios = list(item.studios)
        productionLocations = list(item.productionLocations)
        artists = list(item.artists)
        albumArtists = list(item.albumArtists)
        indexNumber = item.hasIndexNumber ? "\(item.indexNumber)" : ""
        parentIndexNumber = item.hasParentIndexNumber ? "\(item.parentIndexNumber)" : ""
        absoluteNumber = item.hasAbsoluteNumber ? "\(item.absoluteNumber)" : ""
        metadataLanguage = item.metadataLanguage
        metadataCountry = item.metadataCountry
        seriesStatus = item.seriesStatus
        externalIds = item.externalIds
        locked = item.locked
        lockedFields = item.lockedFields
    }

    /// The fields of the form changed from before, as the server's metadata
    /// and the paths of its update mask.
    public func changes(from before: MetadataForm) -> (Mavio_Library_V1_ItemMetadata, [String]) {
        let unlist = { (v: String) in
            v.split(separator: ",").map { $0.trimmingCharacters(in: .whitespaces) }.filter { !$0.isEmpty }
        }
        let trim = { (v: String) in v.trimmingCharacters(in: .whitespacesAndNewlines) }
        var m = Mavio_Library_V1_ItemMetadata()
        var paths: [String] = []
        func field(_ path: String, _ changed: Bool, _ set: () -> Void) {
            guard changed else { return }
            paths.append(path)
            set()
        }
        func int(_ v: String) -> Int32? { Int32(v.trimmingCharacters(in: .whitespaces)) }
        field("name", name != before.name) { m.name = trim(name) }
        field("original_title", originalTitle != before.originalTitle) { m.originalTitle = trim(originalTitle) }
        field("sort_name", sortName != before.sortName) { m.sortName = trim(sortName) }
        field("overview", overview != before.overview) { m.overview = trim(overview) }
        field("tagline", tagline != before.tagline) { m.tagline = trim(tagline) }
        field("production_year", productionYear != before.productionYear) {
            m.productionYear = int(productionYear) ?? 0
        }
        field("premiere_date", premiereDate != before.premiereDate) {
            if let d = Self.date(premiereDate) { m.premiereDate = Google_Protobuf_Timestamp(date: d) }
        }
        field("end_date", endDate != before.endDate) {
            if let d = Self.date(endDate) { m.endDate = Google_Protobuf_Timestamp(date: d) }
        }
        field("runtime", runtime != before.runtime) {
            if let minutes = Double(runtime.trimmingCharacters(in: .whitespaces)) {
                m.runtime = Google_Protobuf_Duration(timeInterval: minutes * 60)
            }
        }
        field("official_rating", officialRating != before.officialRating) { m.officialRating = trim(officialRating) }
        field("custom_rating", customRating != before.customRating) { m.customRating = trim(customRating) }
        field("community_rating", communityRating != before.communityRating) {
            m.communityRating = Double(communityRating.trimmingCharacters(in: .whitespaces)) ?? 0
        }
        field("critic_rating", criticRating != before.criticRating) {
            m.criticRating = Double(criticRating.trimmingCharacters(in: .whitespaces)) ?? 0
        }
        field("genres", genres != before.genres) { m.genres = unlist(genres) }
        field("tags", tags != before.tags) { m.tags = unlist(tags) }
        field("studios", studios != before.studios) { m.studios = unlist(studios) }
        field("production_locations", productionLocations != before.productionLocations) {
            m.productionLocations = unlist(productionLocations)
        }
        field("artists", artists != before.artists) { m.artists = unlist(artists) }
        field("album_artists", albumArtists != before.albumArtists) { m.albumArtists = unlist(albumArtists) }
        field("index_number", indexNumber != before.indexNumber) {
            if let n = int(indexNumber) { m.indexNumber = n }
        }
        field("parent_index_number", parentIndexNumber != before.parentIndexNumber) {
            if let n = int(parentIndexNumber) { m.parentIndexNumber = n }
        }
        field("absolute_number", absoluteNumber != before.absoluteNumber) {
            if let n = int(absoluteNumber) { m.absoluteNumber = n }
        }
        field("metadata_language", metadataLanguage != before.metadataLanguage) {
            m.metadataLanguage = metadataLanguage
        }
        field("metadata_country", metadataCountry != before.metadataCountry) { m.metadataCountry = metadataCountry }
        field("series_status", seriesStatus != before.seriesStatus) { m.seriesStatus = seriesStatus }
        field("external_ids", externalIds != before.externalIds) {
            m.externalIds = externalIds.mapValues { $0.trimmingCharacters(in: .whitespaces) }
        }
        field("locked", locked != before.locked) { m.locked = locked }
        field("locked_fields", lockedFields != before.lockedFields) { m.lockedFields = lockedFields }
        return (m, paths)
    }
}

/// The kinds of images an item's are edited by, with their names.
public let metadataImageKinds: [(Mavio_Library_V1_ImageKind, String)] = [
    (.primary, MavioStrings.metadataPrimary), (.backdrop, MavioStrings.metadataBackdrop),
    (.logo, MavioStrings.metadataLogo), (.thumb, MavioStrings.metadataThumb),
    (.banner, MavioStrings.metadataBanner), (.art, MavioStrings.metadataArt), (.disc, MavioStrings.metadataDisc),
]

/// Whether images of a kind are wide, drawn at 16:9 rather than 2:3.
public func wideImages(_ kind: Mavio_Library_V1_ImageKind) -> Bool {
    [.backdrop, .thumb, .banner, .logo, .art].contains(kind)
}

/// What a remote image's caption says: its provider, size and language.
public func remoteImageCaption(_ image: Mavio_Library_V1_RemoteImage) -> String {
    [image.provider, image.width > 0 && image.height > 0 ? "\(image.width)×\(image.height)" : "", image.language]
        .filter { !$0.isEmpty }.joined(separator: " · ")
}

/// What a search result's line says: its provider and external IDs.
public func remoteResultIds(_ result: Mavio_Library_V1_RemoteSearchResult) -> String {
    ([result.provider] + result.externalIds.sorted { $0.key < $1.key }.map { "\($0.key) \($0.value)" })
        .joined(separator: " · ")
}

extension ServerRecord {
    /// The URL of an image path the server signed, maxWidth pixels wide at
    /// most.
    public func imageURL(_ path: String, maxWidth: Int? = nil) -> URL? {
        var url = path
        if !url.hasPrefix("http") {
            var base = address
            while base.hasSuffix("/") { base.removeLast() }
            url = base + (url.hasPrefix("/") ? url : "/" + url)
        }
        if let maxWidth { url += "?maxWidth=\(maxWidth)" }
        return URL(string: url)
    }
}

extension AppModel {
    private func metadata<Response: SwiftProtobuf.Message>(
        _ serverId: String, _ method: String, _ request: some SwiftProtobuf.Message, as _: Response.Type
    ) async throws -> Response {
        try await client.call(serverId, Procedure.metadata(method), request, as: Response.self)
    }

    /// The languages and countries of a server's metadata, named in the
    /// user's language where it can, by name, without repeats.
    public func localizationChoices(_ serverId: String) async -> (
        languages: [(code: String, name: String)], countries: [(code: String, name: String)]
    ) {
        async let langs = try? client.call(
            serverId, Procedure.localization("ListLanguages"), Mavio_System_V1_ListLanguagesRequest(),
            as: Mavio_System_V1_ListLanguagesResponse.self)
        async let ctrs = try? client.call(
            serverId, Procedure.localization("ListCountries"), Mavio_System_V1_ListCountriesRequest(),
            as: Mavio_System_V1_ListCountriesResponse.self)
        let locale = Locale.current
        let languages = Self.named((await langs)?.languages.map { ($0.code, $0.name) } ?? []) {
            locale.localizedString(forLanguageCode: $0)
        }
        let countries = Self.named((await ctrs)?.countries.map { ($0.code, $0.name) } ?? []) {
            locale.localizedString(forRegionCode: $0)
        }
        return (languages, countries)
    }

    private static func named(_ list: [(String, String)], _ of: (String) -> String?) -> [(
        code: String, name: String
    )] {
        var seen = Set<String>()
        return
            list
            .filter { !$0.0.isEmpty && seen.insert($0.0).inserted }
            .map { (code: $0.0, name: of($0.0) ?? $0.1) }
            .sorted { $0.name.localizedStandardCompare($1.name) == .orderedAscending }
    }

    /// An item as the server has it.
    public func item(_ serverId: String, _ itemId: String) async throws -> Mavio_Library_V1_Item {
        var request = Mavio_Library_V1_GetItemRequest()
        request.id = itemId
        return try await client.call(
            serverId, Procedure.items("GetItem"), request, as: Mavio_Library_V1_GetItemResponse.self
        ).item
    }

    /// The kinds of external IDs items of a kind carry.
    public func externalIdKinds(_ serverId: String, _ kind: Mavio_Library_V1_ItemKind) async throws
        -> [Mavio_Library_V1_ExternalIdKind]
    {
        var request = Mavio_Library_V1_ListExternalIdKindsRequest()
        request.itemKind = kind
        return try await metadata(
            serverId, "ListExternalIdKinds", request, as: Mavio_Library_V1_ListExternalIdKindsResponse.self
        ).kinds
    }

    /// Saves the fields of an item's form changed from before.
    public func updateItem(_ serverId: String, _ itemId: String, _ form: MetadataForm, from before: MetadataForm)
        async throws
    {
        let (metadata, paths) = form.changes(from: before)
        guard !paths.isEmpty else { return }
        var request = Mavio_Library_V1_UpdateItemRequest()
        request.id = itemId
        request.metadata = metadata
        request.updateMask = Google_Protobuf_FieldMask(protoPaths: paths)
        _ = try await self.metadata(serverId, "UpdateItem", request, as: Mavio_Library_V1_UpdateItemResponse.self)
        bump(serverId)
    }

    /// Reads an item's files and the providers again, keeping what it has
    /// or replacing all of its metadata.
    public func refreshMetadata(_ serverId: String, _ itemId: String, replace: Bool) async throws {
        var request = Mavio_Library_V1_RefreshItemRequest()
        request.id = itemId
        request.replaceMetadata = replace
        _ = try await metadata(serverId, "RefreshItem", request, as: Mavio_Library_V1_RefreshItemResponse.self)
        bump(serverId)
    }

    /// Searches the providers for what an item is, by a name and year.
    public func searchRemote(_ serverId: String, _ itemId: String, name: String, year: Int32?) async throws
        -> [Mavio_Library_V1_RemoteSearchResult]
    {
        var request = Mavio_Library_V1_SearchRemoteRequest()
        request.itemID = itemId
        request.name = name
        if let year { request.year = year }
        return try await metadata(serverId, "SearchRemote", request, as: Mavio_Library_V1_SearchRemoteResponse.self)
            .results
    }

    /// Identifies an item as a search result, replacing its external IDs,
    /// metadata and images with the providers'.
    public func identify(_ serverId: String, _ itemId: String, as result: Mavio_Library_V1_RemoteSearchResult)
        async throws
    {
        var request = Mavio_Library_V1_IdentifyItemRequest()
        request.itemID = itemId
        request.externalIds = result.externalIds
        _ = try await metadata(serverId, "IdentifyItem", request, as: Mavio_Library_V1_IdentifyItemResponse.self)
        bump(serverId)
    }

    /// The providers' images of a kind for an item.
    public func remoteImages(_ serverId: String, _ itemId: String, _ kind: Mavio_Library_V1_ImageKind) async throws
        -> [Mavio_Library_V1_RemoteImage]
    {
        var request = Mavio_Library_V1_ListRemoteImagesRequest()
        request.itemID = itemId
        request.kind = kind
        return try await metadata(
            serverId, "ListRemoteImages", request, as: Mavio_Library_V1_ListRemoteImagesResponse.self
        ).images
    }

    /// Replaces an item's first image of a kind with the image at a URL.
    public func setItemImage(_ serverId: String, _ itemId: String, _ kind: Mavio_Library_V1_ImageKind, url: String)
        async throws
    {
        var request = Mavio_Library_V1_SetItemImageRequest()
        request.itemID = itemId
        request.kind = kind
        request.url = url
        _ = try await metadata(serverId, "SetItemImage", request, as: Mavio_Library_V1_SetItemImageResponse.self)
        bump(serverId)
    }

    /// Replaces an item's first image of a kind with an uploaded file.
    public func setItemImage(_ serverId: String, _ itemId: String, _ kind: Mavio_Library_V1_ImageKind, data: Data)
        async throws
    {
        var request = Mavio_Library_V1_SetItemImageRequest()
        request.itemID = itemId
        request.kind = kind
        request.data = data
        _ = try await metadata(serverId, "SetItemImage", request, as: Mavio_Library_V1_SetItemImageResponse.self)
        bump(serverId)
    }

    /// Deletes an item's image.
    public func deleteItemImage(_ serverId: String, _ itemId: String, imageId: String) async throws {
        var request = Mavio_Library_V1_DeleteItemImageRequest()
        request.itemID = itemId
        request.imageID = imageId
        _ = try await metadata(
            serverId, "DeleteItemImage", request, as: Mavio_Library_V1_DeleteItemImageResponse.self)
        bump(serverId)
    }

    /// Whether the user is an administrator of a server, who edits its
    /// items' metadata, and whether they may report issues.
    public func userRights(_ serverId: String) async -> (admin: Bool, reportIssues: Bool) {
        guard
            let user = try? await client.call(
                serverId, Procedure.users("GetCurrentUser"), Mavio_User_V1_GetCurrentUserRequest(),
                as: Mavio_User_V1_GetCurrentUserResponse.self
            ).user
        else { return (false, false) }
        return (user.admin, user.admin || user.policy.reportIssues)
    }
}

/// An entry of an item's More menu named as the web UI's, without the
/// ellipsis some strings end in.
public func menuName(_ name: String) -> String {
    name.hasSuffix("…") ? String(name.dropLast()) : name
}
