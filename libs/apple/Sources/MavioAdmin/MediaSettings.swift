import MavioKit
import MavioUI
import SwiftUI

// MARK: Libraries

/// A kind of library as the settings show it: its name, icon and color.
private struct LibraryKindStyle {
    let kind: Mavio_Library_V1_LibraryKind
    let title: String
    let color: Color

    static var all: [LibraryKindStyle] {
        [
            .init(kind: .movies, title: MavioStrings.librariesKindMovies, color: Color(hex: 0xFF9500)),
            .init(kind: .shows, title: MavioStrings.librariesKindShows, color: Color(hex: 0x007AFF)),
            .init(kind: .music, title: MavioStrings.librariesKindMusic, color: Color(hex: 0xFF2D55)),
            .init(kind: .musicVideos, title: MavioStrings.librariesKindMusicVideos, color: Color(hex: 0xAF52DE)),
            .init(kind: .homeVideos, title: MavioStrings.librariesKindHomeVideos, color: Color(hex: 0x34C759)),
            .init(kind: .books, title: MavioStrings.librariesKindBooks, color: Color(hex: 0xA2845E)),
            .init(kind: .photos, title: MavioStrings.librariesKindPhotos, color: Color(hex: 0x32ADE6)),
            .init(kind: .mixed, title: MavioStrings.librariesKindMixed, color: Color(hex: 0x8E8E93)),
        ]
    }

    static func of(_ kind: Mavio_Library_V1_LibraryKind) -> LibraryKindStyle {
        all.first { $0.kind == kind } ?? all[all.count - 1]
    }

    var icon: String {
        switch kind {
        case .movies: "movies"
        case .shows: "shows"
        case .music, .musicVideos: "music"
        case .homeVideos: "homeVideos"
        case .books: "books"
        case .photos: "photos"
        default: "folder"
        }
    }

    var video: Bool { [.movies, .shows, .musicVideos, .homeVideos, .mixed].contains(kind) }
    var music: Bool { kind == .music || kind == .mixed }
    var movies: Bool { kind == .movies || kind == .mixed }
    var shows: Bool { kind == .shows || kind == .mixed }
}

/// A tile of a color with an icon, as the settings' entries have.
struct ColorTile: View {
    let icon: String
    let color: Color
    var size: CGFloat = 28

    var body: some View {
        Icon(icon, fill: true, size: size * 0.6)
            .foregroundStyle(.white)
            .frame(width: size, height: size)
            .background(color, in: .rect(cornerRadius: size / 4))
    }
}

/// Libraries (docs/clients.md §6.3.4): the libraries of the server, to
/// scan, and a library's page to add, change and delete it.
struct ManagedLibrariesView: View {
    let server: ServerRecord
    @Environment(AppModel.self) private var model
    @Environment(\.theme) private var theme
    @State private var libraries: [Mavio_Library_V1_Library]?
    @State private var failure: String?
    /// What scanning a library told, by its ID.
    @State private var scanned: [String: String] = [:]
    @State private var scanning: Set<String> = []

    var body: some View {
        List {
            Section {
                Text(MavioStrings.librariesMessage).type(theme.type.callout).foregroundStyle(theme.colors.textSecondary)
                    .listRowBackground(Color.clear)
            }
            if let libraries {
                Section {
                    ForEach(libraries, id: \.id) { row($0) }
                    if libraries.isEmpty {
                        Text(MavioStrings.librariesNone).foregroundStyle(theme.colors.textSecondary)
                    }
                    NavigationLink {
                        ManagedLibraryView(server: server, library: nil, changed: reload)
                    } label: {
                        Label(MavioStrings.librariesAdd, systemImage: "plus")
                    }
                }
            } else if failure == nil {
                ProgressView().frame(maxWidth: .infinity).listRowBackground(Color.clear)
            }
            if let failure {
                Text(failure).foregroundStyle(theme.colors.danger)
            }
        }
        .hiddenListBackground()
        .background(theme.colors.background)
        .navigationTitle(MavioStrings.settingsLibraries)
        .inlineTitle()
        .task { await load() }
        .pullToRefresh { await load() }
        .onChange(of: scanningLibraries) { before, now in
            // A scan that ended says so until the page is left.
            for id in before.subtracting(now) {
                scanned[id] = MavioStrings.librariesScanDone
            }
        }
    }

    /// The libraries being scanned.
    private var scanningLibraries: Set<String> {
        Set(model.scanProgress[server.id].map { Array($0.keys) } ?? [])
    }

    /// What a library's scan is doing, and how far it has come.
    private static func describe(_ p: ScanProgress) -> String {
        p.readingFolders
            ? MavioStrings.librariesScanFolders(done: "\(p.foldersDone)", total: "\(p.foldersTotal)")
            : MavioStrings.librariesScanItems(done: "\(p.itemsDone)", total: "\(p.itemsTotal)")
    }

    private func row(_ library: Mavio_Library_V1_Library) -> some View {
        let k = LibraryKindStyle.of(library.kind)
        // The link is behind the row, so that its Scan button ends it
        // rather than a chevron.
        return HStack(spacing: MavioSpacing.s12) {
            ColorTile(icon: k.icon, color: k.color)
            let progress = model.scanProgress[server.id]?[library.id]
            VStack(alignment: .leading, spacing: 2) {
                Text(library.name).lineLimit(1)
                Text(
                    progress.map(Self.describe)
                        ?? scanned[library.id]
                        ?? "\(k.title) · \(MavioStrings.librariesFolderCount(count: library.paths.count))"
                )
                .type(theme.type.footnote).foregroundStyle(theme.colors.textSecondary).lineLimit(1)
                .monospacedDigit()
                if let progress {
                    ProgressView(value: progress.fraction).padding(.top, MavioSpacing.s4)
                }
            }
            Spacer(minLength: 0)
            Button(scanning.contains(library.id) ? "…" : MavioStrings.librariesScan) { scan(library) }
                .buttonStyle(.bordered)
                .disabled(scanning.contains(library.id))
        }
        .background {
            NavigationLink {
                ManagedLibraryView(server: server, library: library, changed: reload)
            } label: {
                EmptyView()
            }
            .opacity(0)
        }
    }

    private func reload() {
        Task { await load() }
    }

    private func load() async {
        do {
            let listed = try await model.call(
                server, Procedure.libraries("ListLibraries"), Mavio_Library_V1_ListLibrariesRequest(),
                as: Mavio_Library_V1_ListLibrariesResponse.self
            ).libraries
            model.listedScans(server.id, listed)
            libraries =
                listed
                .filter { $0.kind != .collections && $0.kind != .playlists }
                .sorted { $0.name.localizedStandardCompare($1.name) == .orderedAscending }
            failure = nil
        } catch is CancellationError {
        } catch {
            failure = error.reason
        }
    }

    private func scan(_ library: Mavio_Library_V1_Library) {
        scanning.insert(library.id)
        Task {
            defer { scanning.remove(library.id) }
            scanned[library.id] = await model.scan(server, library.id)
        }
    }
}

extension AppModel {
    /// Asks a server to scan a library, telling what came of it.
    func scan(_ server: ServerRecord, _ library: String) async -> String {
        var request = Mavio_Library_V1_ScanLibraryRequest()
        request.id = library
        do {
            let r = try await call(
                server, Procedure.libraries("ScanLibrary"), request, as: Mavio_Library_V1_ScanLibraryResponse.self)
            return r.enqueued ? MavioStrings.librariesScanQueued : MavioStrings.librariesScanPending
        } catch {
            return error.reason
        }
    }
}

/// A library as its page edits it: the fields of LibrarySpec the page shows.
private struct LibraryDraft: Equatable {
    var name = ""
    var kind = Mavio_Library_V1_LibraryKind.movies
    var paths: [String] = []
    var storageClasses: [String: Mavio_Library_V1_StorageClass] = [:]
    var scanSeconds: Int64 = 12 * 3600
    var language = ""
    var country = ""
    var saveLocalMetadata = false
    var autoCollections = false
    var extractTrickplay = false
    var extractChapterImages = false
    var analyzeLoudness = false
    var downloadLyrics = false
    var missing = false
    var unaired = false
    var specials = false
    var graceDays: Int32 = 0

    init() {}

    init(_ l: Mavio_Library_V1_Library) {
        name = l.name
        kind = l.kind
        paths = l.paths
        storageClasses = l.storageClasses
        scanSeconds = l.hasScanInterval ? l.scanInterval.seconds : 0
        language = l.preferredLanguage
        country = l.metadataCountry
        saveLocalMetadata = l.saveLocalMetadata
        autoCollections = l.autoCollections
        extractTrickplay = l.extractTrickplay
        extractChapterImages = l.extractChapterImages
        analyzeLoudness = l.analyzeLoudness
        downloadLyrics = l.downloadLyrics
        missing = l.virtualEpisodes.missing
        unaired = l.virtualEpisodes.unaired
        specials = l.virtualEpisodes.specials
        graceDays = l.virtualEpisodes.graceDays
    }

    var valid: Bool {
        !name.trimmingCharacters(in: .whitespaces).isEmpty && !paths.isEmpty && (0...3650).contains(graceDays)
    }

    /// The spec of the draft, keeping what the page does not show of the library.
    func spec(keeping l: Mavio_Library_V1_Library?) -> Mavio_Library_V1_LibrarySpec {
        var s = Mavio_Library_V1_LibrarySpec()
        s.name = name.trimmingCharacters(in: .whitespaces)
        s.kind = kind
        s.paths = paths
        if scanSeconds > 0 {
            var d = Google_Protobuf_Duration()
            d.seconds = scanSeconds
            s.scanInterval = d
        }
        s.preferredLanguage = language
        s.metadataCountry = country
        s.saveLocalMetadata = saveLocalMetadata
        s.autoCollections = autoCollections
        s.extractTrickplay = extractTrickplay
        s.extractChapterImages = extractChapterImages
        s.analyzeLoudness = analyzeLoudness
        s.downloadLyrics = downloadLyrics
        if let l {
            if l.hasProviderOrder { s.providerOrder = l.providerOrder }
            if l.hasHubs { s.hubs = l.hubs }
        }
        var v = Mavio_Library_V1_VirtualEpisodes()
        v.missing = missing
        v.unaired = unaired
        v.specials = specials
        v.graceDays = graceDays
        s.virtualEpisodes = v
        s.storageClasses = storageClasses.filter { paths.contains($0.key) && $0.value != .unspecified }
        return s
    }
}

/// A library's page, or a new library's when library is nil: its name and
/// kind, its folders with their storage, its metadata, how often it is
/// scanned and what scans make, and the missing episodes of its series.
private struct ManagedLibraryView: View {
    let server: ServerRecord
    /// The library as saved; nil until a new one is added.
    @State var library: Mavio_Library_V1_Library?
    /// Called when the library was added, changed or deleted.
    let changed: () -> Void
    @Environment(AppModel.self) private var model
    @Environment(\.theme) private var theme
    @Environment(\.dismiss) private var dismiss
    @State private var draft = LibraryDraft()
    @State private var loaded = false
    @State private var languages: [Choice] = []
    @State private var countries: [Choice] = []
    @State private var typed = ""
    @State private var saving = false
    @State private var scanning = false
    @State private var failure: String?
    @State private var notice: String?
    @State private var confirming = false

    init(server: ServerRecord, library: Mavio_Library_V1_Library?, changed: @escaping () -> Void) {
        self.server = server
        self._library = State(initialValue: library)
        self.changed = changed
    }

    private static let intervals: [(seconds: Int64, title: String)] = [
        (0, MavioStrings.librariesScanNever),
        (3600, MavioStrings.librariesScanHourly),
        (6 * 3600, MavioStrings.librariesScanEveryHours(hours: "6")),
        (12 * 3600, MavioStrings.librariesScanEveryHours(hours: "12")),
        (24 * 3600, MavioStrings.librariesScanDaily),
        (7 * 24 * 3600, MavioStrings.librariesScanWeekly),
    ]

    private var saved: LibraryDraft { library.map(LibraryDraft.init) ?? LibraryDraft() }

    var body: some View {
        let k = LibraryKindStyle.of(draft.kind)
        List {
            Section {
                TextRow(label: MavioStrings.librariesName, text: $draft.name)
                Picker(MavioStrings.librariesKind, selection: $draft.kind) {
                    ForEach(LibraryKindStyle.all, id: \.kind) { Text($0.title).tag($0.kind) }
                }
            } header: {
                HStack(spacing: MavioSpacing.s12) {
                    ColorTile(icon: k.icon, color: k.color, size: 40)
                    Text(library?.name ?? MavioStrings.librariesAdd)
                        .type(theme.type.title3).foregroundStyle(theme.colors.textPrimary)
                }
                .textCase(nil)
                .padding(.bottom, MavioSpacing.s8)
            }
            folders
            Section(MavioStrings.librariesMetadata) {
                Picker(MavioStrings.librariesLanguage, selection: $draft.language) {
                    Text(MavioStrings.librariesServerDefault).tag("")
                    ForEach(choices(languages, draft.language), id: \.code) { Text($0.name).tag($0.code) }
                }
                .listPicker()
                Picker(MavioStrings.librariesCountry, selection: $draft.country) {
                    Text(MavioStrings.librariesServerDefault).tag("")
                    ForEach(choices(countries, draft.country), id: \.code) { Text($0.name).tag($0.code) }
                }
                .listPicker()
                toggle(
                    $draft.saveLocalMetadata, MavioStrings.librariesSaveLocalMetadata,
                    MavioStrings.librariesSaveLocalMetadataDetail)
                if k.movies {
                    toggle(
                        $draft.autoCollections, MavioStrings.librariesAutoCollections,
                        MavioStrings.librariesAutoCollectionsDetail)
                }
                if k.music { toggle($draft.downloadLyrics, MavioStrings.librariesDownloadLyrics) }
            }
            Section(MavioStrings.librariesScanning) {
                Picker(MavioStrings.librariesScanInterval, selection: $draft.scanSeconds) {
                    ForEach(intervals, id: \.seconds) { Text($0.title).tag($0.seconds) }
                }
                if k.video {
                    toggle(
                        $draft.extractTrickplay, MavioStrings.librariesTrickplay, MavioStrings.librariesTrickplayDetail)
                    toggle($draft.extractChapterImages, MavioStrings.librariesChapterImages)
                }
                if k.music {
                    toggle($draft.analyzeLoudness, MavioStrings.librariesLoudness, MavioStrings.librariesLoudnessDetail)
                }
            }
            if k.shows {
                Section {
                    toggle($draft.missing, MavioStrings.librariesMissingAired)
                    toggle($draft.unaired, MavioStrings.librariesMissingUnaired)
                    toggle($draft.specials, MavioStrings.librariesMissingSpecials)
                    if !draft.missing {
                        NumberRow(
                            label: MavioStrings.librariesGraceDays, detail: MavioStrings.librariesGraceDaysDetail,
                            value: $draft.graceDays.double, range: 0...3650)
                    }
                } header: {
                    Text(MavioStrings.librariesMissingEpisodes)
                } footer: {
                    Text(MavioStrings.librariesMissingEpisodesFooter)
                }
            }
            if let library {
                Section {
                    Button(scanning ? "…" : MavioStrings.librariesScan) { scan(library) }
                        .disabled(scanning)
                    Button(MavioStrings.actionDelete, role: .destructive) { confirming = true }
                        .confirmationDialog(
                            MavioStrings.librariesDeleteTitle(name: library.name), isPresented: $confirming,
                            titleVisibility: .visible
                        ) {
                            Button(MavioStrings.actionDelete, role: .destructive, action: delete)
                        } message: {
                            Text(MavioStrings.librariesDeleteMessage)
                        }
                }
            }
            if notice != nil && draft == saved || failure != nil {
                Section {
                    if let notice, draft == saved {
                        Text(notice).foregroundStyle(theme.colors.textSecondary)
                    }
                    if let failure {
                        Text(failure).foregroundStyle(theme.colors.danger)
                    }
                }
            }
        }
        .scrollDismissesKeyboard(.interactively)
        .hiddenListBackground()
        .background(theme.colors.background)
        .navigationTitle(library?.name ?? MavioStrings.librariesAdd)
        .inlineTitle()
        .toolbar {
            if saving {
                ProgressView()
            } else {
                Button(library == nil ? MavioStrings.librariesCreate : MavioStrings.actionSave, action: save)
                    .disabled(draft == saved || !draft.valid)
            }
        }
        .task {
            if !loaded {
                draft = saved
                loaded = true
            }
            (languages, countries) = await model.localization(server)
        }
    }

    private var intervals: [(seconds: Int64, title: String)] {
        Self.intervals.contains { $0.seconds == draft.scanSeconds }
            ? Self.intervals
            : Self.intervals + [
                (draft.scanSeconds, MavioStrings.librariesScanEveryHours(hours: String(draft.scanSeconds / 3600)))
            ]
    }

    @ViewBuilder private var folders: some View {
        Section {
            ForEach(draft.paths, id: \.self) { path in
                VStack(alignment: .leading, spacing: MavioSpacing.s4) {
                    HStack(alignment: .firstTextBaseline) {
                        Text(path).selectable()
                        Spacer(minLength: MavioSpacing.s8)
                        Button {
                            draft.paths.removeAll { $0 == path }
                        } label: {
                            Icon("close", size: 14).foregroundStyle(theme.colors.textSecondary)
                        }
                        .buttonStyle(.borderless)
                        .accessibilityLabel(MavioStrings.librariesRemoveFolder)
                    }
                    Picker(MavioStrings.librariesStorageClass, selection: storageClass(path)) {
                        Text(auto(path)).tag(Mavio_Library_V1_StorageClass.unspecified)
                        Text(MavioStrings.librariesStorageLocal).tag(Mavio_Library_V1_StorageClass.local)
                        Text(MavioStrings.librariesStorageNetwork).tag(Mavio_Library_V1_StorageClass.network)
                        Text(MavioStrings.librariesStorageCloud).tag(Mavio_Library_V1_StorageClass.cloud)
                    }
                    .type(theme.type.callout)
                    .foregroundStyle(theme.colors.textSecondary)
                }
                .rowActions(menu: true) {
                    Button(MavioStrings.librariesRemoveFolder, role: .destructive) {
                        draft.paths.removeAll { $0 == path }
                    }
                }
            }
            HStack(spacing: MavioSpacing.s12) {
                TextField(MavioStrings.librariesFolderPath, text: $typed)
                    .noCapitals()
                    .autocorrectionDisabled()
                    .onSubmit(addTyped)
                    .submitLabel(.done)
                Button(MavioStrings.librariesAddFolder, action: addTyped)
                    .buttonStyle(.bordered)
                    .disabled(typed.trimmingCharacters(in: .whitespaces).isEmpty)
            }
            NavigationLink {
                FolderPicker(server: server, chosen: draft.paths) { add($0) }
            } label: {
                Label(MavioStrings.librariesBrowse, systemImage: "folder")
            }
        } header: {
            Text(MavioStrings.librariesFolders)
        } footer: {
            Text(MavioStrings.librariesFoldersFooter)
        }
    }

    private func storageClass(_ path: String) -> Binding<Mavio_Library_V1_StorageClass> {
        Binding(
            get: { draft.storageClasses[path] ?? .unspecified },
            set: { draft.storageClasses[path] = $0 })
    }

    /// Automatic storage, with what the server finds it is.
    private func auto(_ path: String) -> String {
        switch library?.effectiveStorageClasses[path] {
        case .local: MavioStrings.librariesStorageAutoIs(storage: MavioStrings.librariesStorageLocal)
        case .network: MavioStrings.librariesStorageAutoIs(storage: MavioStrings.librariesStorageNetwork)
        case .cloud: MavioStrings.librariesStorageAutoIs(storage: MavioStrings.librariesStorageCloud)
        default: MavioStrings.librariesStorageAuto
        }
    }

    private func toggle(_ on: Binding<Bool>, _ label: String, _ detail: String? = nil) -> some View {
        Toggle(isOn: on) { RowLabel(label: label, detail: detail) }
    }

    private func addTyped() {
        add(typed)
        typed = ""
    }

    private func add(_ path: String) {
        var p = path.trimmingCharacters(in: .whitespaces)
        while p.count > 1, p.hasSuffix("/") { p.removeLast() }
        if !p.isEmpty, !draft.paths.contains(p) { draft.paths.append(p) }
    }

    private func save() {
        // A field still being typed in would change the draft once saved.
        endEditing()
        saving = true
        failure = nil
        Task {
            defer { saving = false }
            do {
                if let library {
                    var request = Mavio_Library_V1_UpdateLibraryRequest()
                    request.id = library.id
                    request.spec = draft.spec(keeping: library)
                    self.library = try await model.call(
                        server, Procedure.libraries("UpdateLibrary"), request,
                        as: Mavio_Library_V1_UpdateLibraryResponse.self
                    ).library
                } else {
                    var request = Mavio_Library_V1_CreateLibraryRequest()
                    request.spec = draft.spec(keeping: nil)
                    self.library = try await model.call(
                        server, Procedure.libraries("CreateLibrary"), request,
                        as: Mavio_Library_V1_CreateLibraryResponse.self
                    ).library
                }
                draft = saved
                notice = MavioStrings.librariesSaved
                changed()
            } catch {
                failure = error.reason
            }
        }
    }

    private func scan(_ library: Mavio_Library_V1_Library) {
        scanning = true
        Task {
            notice = await model.scan(server, library.id)
            scanning = false
        }
    }

    private func delete() {
        guard let library else { return }
        Task {
            var request = Mavio_Library_V1_DeleteLibraryRequest()
            request.id = library.id
            do {
                _ = try await model.call(
                    server, Procedure.libraries("DeleteLibrary"), request,
                    as: Mavio_Library_V1_DeleteLibraryResponse.self)
                changed()
                dismiss()
            } catch {
                failure = error.reason
            }
        }
    }
}

/// The folders of the server's file system and of its rclone remotes, to
/// walk and choose one from.
private struct FolderPicker: View {
    let server: ServerRecord
    let chosen: [String]
    let choose: (String) -> Void
    @Environment(AppModel.self) private var model
    @Environment(\.theme) private var theme
    @Environment(\.dismiss) private var dismiss
    @State private var path = ""
    @State private var here = ""
    @State private var parent = ""
    @State private var folders: [(path: String, name: String)]?
    @State private var failure: String?

    private var remote: Bool { path.hasPrefix("rclone:") }

    var body: some View {
        List {
            Section {
                HStack(spacing: MavioSpacing.s12) {
                    Text(here.isEmpty ? MavioStrings.librariesRoots : here)
                        .type(theme.type.callout).foregroundStyle(theme.colors.textSecondary)
                    Spacer(minLength: 0)
                }
            }
            Section {
                if !here.isEmpty {
                    Button {
                        path = parent
                    } label: {
                        Label(MavioStrings.librariesParentFolder, systemImage: "chevron.backward")
                    }
                }
                ForEach(folders ?? [], id: \.path) { f in
                    Button {
                        path = f.path
                    } label: {
                        Label {
                            Text(f.name).foregroundStyle(theme.colors.textPrimary)
                        } icon: {
                            Icon("folder", size: 18).foregroundStyle(theme.colors.textSecondary)
                        }
                    }
                }
                if let failure {
                    Text(failure).foregroundStyle(theme.colors.danger)
                } else if folders?.isEmpty == true {
                    Text(MavioStrings.librariesNoFolders).foregroundStyle(theme.colors.textSecondary)
                } else if folders == nil {
                    ProgressView().frame(maxWidth: .infinity)
                }
            }
        }
        .hiddenListBackground()
        .background(theme.colors.background)
        .navigationTitle(MavioStrings.librariesBrowse)
        .inlineTitle()
        .toolbar {
            Button(MavioStrings.librariesChooseFolder) {
                choose(here)
                dismiss()
            }
            .disabled(here.isEmpty || chosen.contains(here))
        }
        .task(id: path) { await load() }
    }

    private func load() async {
        folders = nil
        failure = nil
        do {
            if remote {
                var request = Mavio_Storage_V1_ListFoldersRequest()
                request.path = path
                let r = try await model.call(
                    server, Procedure.storage("ListFolders"), request, as: Mavio_Storage_V1_ListFoldersResponse.self)
                here = path
                parent = Self.remoteParent(path)
                folders = r.folders.map { f in
                    let cut = f.contains("/") ? f.lastIndex(of: "/")! : f.lastIndex(of: ":")!
                    return (f, String(f[f.index(after: cut)...]))
                }
            } else {
                var request = Mavio_System_V1_ListDirectoryRequest()
                request.path = path
                let r = try await model.call(
                    server, Procedure.system("ListDirectory"), request, as: Mavio_System_V1_ListDirectoryResponse.self)
                here = r.path
                parent = r.parent
                var list = r.entries.filter(\.isDir).map { ($0.path, $0.name) }
                if path.isEmpty { list += await remotes() }
                folders = list
            }
        } catch is CancellationError {
        } catch {
            failure = error.reason
        }
    }

    /// The roots of the rclone remotes, when the server reaches rclone.
    private func remotes() async -> [(String, String)] {
        guard
            let status = try? await model.call(
                server, Procedure.storage("GetStatus"), Mavio_Storage_V1_GetStatusRequest(),
                as: Mavio_Storage_V1_GetStatusResponse.self), status.available,
            let r = try? await model.call(
                server, Procedure.storage("ListRemotes"), Mavio_Storage_V1_ListRemotesRequest(),
                as: Mavio_Storage_V1_ListRemotesResponse.self)
        else { return [] }
        return r.remotes.map { ("rclone:\($0.name):", "rclone:\($0.name):") }
    }

    /// The folder above a remote's: the remote's root, or the roots above it.
    private static func remoteParent(_ path: String) -> String {
        let rest = path.dropFirst("rclone:".count)
        guard let colon = rest.firstIndex(of: ":") else { return "" }
        let inside = rest[rest.index(after: colon)...]
        if inside.isEmpty { return "" }
        if let slash = path.lastIndex(of: "/") { return String(path[..<slash]) }
        return String(path[...colon])
    }
}
