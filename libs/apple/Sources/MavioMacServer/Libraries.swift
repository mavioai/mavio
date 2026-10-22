#if os(macOS)
    import AppKit
    import MavioMac

    /// A kind of library as the settings show it: its name, icon and color.
    struct LibraryKindStyle {
        let kind: Mavio_Library_V1_LibraryKind
        let title: String
        let color: UInt32

        static var all: [LibraryKindStyle] {
            [
                .init(kind: .movies, title: MavioStrings.librariesKindMovies, color: 0xFF9500),
                .init(kind: .shows, title: MavioStrings.librariesKindShows, color: 0x007AFF),
                .init(kind: .music, title: MavioStrings.librariesKindMusic, color: 0xFF2D55),
                .init(kind: .musicVideos, title: MavioStrings.librariesKindMusicVideos, color: 0xAF52DE),
                .init(kind: .homeVideos, title: MavioStrings.librariesKindHomeVideos, color: 0x34C759),
                .init(kind: .books, title: MavioStrings.librariesKindBooks, color: 0xA2845E),
                .init(kind: .photos, title: MavioStrings.librariesKindPhotos, color: 0x32ADE6),
                .init(kind: .mixed, title: MavioStrings.librariesKindMixed, color: 0x8E8E93),
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

    // MARK: Libraries

    /// Libraries (docs/clients.md §6.3.4): the libraries of the server, to
    /// scan, and a library's page to add, change and delete it. In
    /// television mode a library's row opens its page alone, where it is
    /// scanned.
    final class LibrariesPage: ServerPage {
        private var libraries: [Mavio_Library_V1_Library]?
        private var failure: String?
        /// What scanning a library told, by its ID.
        private var scanned: [String: String] = [:]
        private var scanning: Set<String> = []
        /// The libraries being scanned, as last seen.
        private var scans: Set<String> = []
        private var parts: [String: (detail: NSTextField, bar: ShareBar, button: MavioButton?)] = [:]
        private var observer: Observer?
        private lazy var form = FormPage(
            title: MavioStrings.settingsLibraries, message: MavioStrings.librariesMessage, theme: theme)
        private lazy var list = section()

        override func viewDidLoad() {
            super.viewDidLoad()
            title = MavioStrings.settingsLibraries
            form.add(list)
            observer = Observer { [weak self] in self?.scanChanged() }
        }

        override func appeared() {
            run { [weak self] in await self?.load() }
        }

        private func reload() {
            run { [weak self] in await self?.load() }
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
                return
            } catch {
                failure = error.reason
            }
            render()
        }

        private func render() {
            guard let libraries else {
                if let failure {
                    showError(failure) { [weak self] in
                        self?.show(LoadingView())
                        self?.reload()
                    }
                }
                return
            }
            parts = [:]
            var rows: [NSView] = libraries.map(row)
            if libraries.isEmpty { rows.append(note(MavioStrings.librariesNone)) }
            rows.append(
                FormRow.link(MavioStrings.librariesAdd, icon: "add", tint: theme.colors.accent, theme: theme) {
                    [weak self] in self?.open(nil)
                })
            if let failure { rows.append(note(failure, failed: true)) }
            list.setRows(rows)
            show(form)
            refocus()
        }

        private func row(_ library: Mavio_Library_V1_Library) -> NSView {
            let k = LibraryKindStyle.of(library.kind)
            let name = NSTextField.label(
                library.name, theme.isTV ? theme.fonts.headline : theme.fonts.body, color: theme.colors.textPrimary)
            name.lineBreakMode = .byTruncatingTail
            let detail = NSTextField.label("", theme.fonts.footnote, color: theme.colors.textSecondary)
            detail.lineBreakMode = .byTruncatingTail
            let bar = ShareBar(0, color: theme.colors.accent, track: theme.colors.fill)
            let lines = NSStackView.vertical([name, detail, bar], spacing: 2)
            bar.widthAnchor.constraint(equalTo: lines.widthAnchor).isActive = true
            let content = NSStackView.horizontal(
                [FormRow.tile(k.icon, tint: NSColor(hex: k.color), theme: theme), lines], spacing: MavioSpacing.s12)
            var scan: MavioButton?
            if !theme.isTV {
                scan = button(MavioStrings.librariesScan) { [weak self] in self?.scan(library) }
            }
            let row = ContentRow(content, accessory: scan, chevron: theme.isTV, theme: theme) { [weak self] in
                self?.open(library)
            }
            row.setAccessibilityLabel(library.name)
            parts[library.id] = (detail, bar, scan)
            update(library)
            return row
        }

        /// Shows a library's scan, or what the last one told, or its kind
        /// and folders.
        private func update(_ library: Mavio_Library_V1_Library) {
            guard let part = parts[library.id] else { return }
            let k = LibraryKindStyle.of(library.kind)
            let progress = model.scanProgress[server.id]?[library.id]
            part.detail.stringValue =
                progress.map(Self.describe)
                ?? scanned[library.id]
                ?? "\(k.title) · \(MavioStrings.librariesFolderCount(count: library.paths.count))"
            part.detail.font = .monospacedDigitSystemFont(ofSize: theme.fonts.footnote.size, weight: .regular)
            part.bar.isHidden = progress == nil
            part.bar.share = progress?.fraction ?? 0
            part.button?.title = scanning.contains(library.id) ? "…" : MavioStrings.librariesScan
            part.button?.isEnabled = !scanning.contains(library.id)
        }

        private func scanChanged() {
            let now = Set(model.scanProgress[server.id].map { Array($0.keys) } ?? [])
            // A scan that ended says so until the page is left.
            for id in scans.subtracting(now) { scanned[id] = MavioStrings.librariesScanDone }
            scans = now
            for library in libraries ?? [] { update(library) }
        }

        /// What a library's scan is doing, and how far it has come.
        private static func describe(_ p: ScanProgress) -> String {
            p.readingFolders
                ? MavioStrings.librariesScanFolders(done: "\(p.foldersDone)", total: "\(p.foldersTotal)")
                : MavioStrings.librariesScanItems(done: "\(p.itemsDone)", total: "\(p.itemsTotal)")
        }

        private func open(_ library: Mavio_Library_V1_Library?) {
            push(
                LibraryPage(context: context, server: server, library: library) { [weak self] in self?.reload() })
        }

        private func scan(_ library: Mavio_Library_V1_Library) {
            scanning.insert(library.id)
            update(library)
            run { [weak self] in
                guard let self else { return }
                scanned[library.id] = await model.scan(server, library.id)
                scanning.remove(library.id)
                update(library)
            }
        }

        isolated deinit {
            observer?.cancel()
        }
    }

    // MARK: Library

    /// A library as its page edits it: the fields of LibrarySpec the page
    /// shows.
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

        /// The spec of the draft, keeping what the page does not show of the
        /// library.
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

    /// A library's page, or a new library's when library is nil: its name
    /// and kind, its folders with their storage, its metadata, how often it
    /// is scanned and what scans make, and the missing episodes of its
    /// series.
    final class LibraryPage: ServerPage {
        /// The library as saved; nil until a new one is added.
        private var library: Mavio_Library_V1_Library?
        /// Called when the library was added, changed or deleted.
        private let changed: @MainActor () -> Void
        private var draft = LibraryDraft()
        private var languages: [(code: String, name: String)] = []
        private var countries: [(code: String, name: String)] = []
        private var typed = ""
        private var saving = false
        private var scanning = false
        private var failure: String?
        private var notice: String?
        private var started = false
        private lazy var form = FormPage(theme: theme)
        private lazy var saveButton = saveButton { [weak self] in self?.save() }
        private var addButton: MavioButton?
        private var noticeSection: FormSection?

        private static let intervals: [(seconds: Int64, title: String)] = [
            (0, MavioStrings.librariesScanNever),
            (3600, MavioStrings.librariesScanHourly),
            (6 * 3600, MavioStrings.librariesScanEveryHours(hours: "6")),
            (12 * 3600, MavioStrings.librariesScanEveryHours(hours: "12")),
            (24 * 3600, MavioStrings.librariesScanDaily),
            (7 * 24 * 3600, MavioStrings.librariesScanWeekly),
        ]

        /// The page of library of server, of a new one when nil; changed is
        /// called when it was added, changed or deleted.
        init(
            context: MacContext, server: ServerRecord, library: Mavio_Library_V1_Library?,
            changed: @escaping @MainActor () -> Void
        ) {
            self.library = library
            self.changed = changed
            super.init(context: context, server: server)
        }

        private var saved: LibraryDraft { library.map(LibraryDraft.init) ?? LibraryDraft() }

        override func viewDidLoad() {
            super.viewDidLoad()
            title = library?.name ?? MavioStrings.librariesAdd
            draft = saved
            rebuild()
            show(form)
        }

        override func appeared() {
            guard !started else { return }
            started = true
            run { [weak self] in
                guard let self else { return }
                (languages, countries) = await model.localizationChoices(server)
                endEditing()
                rebuild()
            }
        }

        /// Changes the draft without rebuilding, for fields and switches.
        private func edit(_ change: (inout LibraryDraft) -> Void) {
            change(&draft)
            refresh()
        }

        /// Changes the draft and rebuilds, for choices.
        private func choose(_ change: (inout LibraryDraft) -> Void) {
            change(&draft)
            rebuild()
        }

        private func refresh() {
            saveButton.isEnabled = draft != saved && draft.valid && !saving
            saveButton.title =
                saving ? "…" : library == nil ? MavioStrings.librariesCreate : MavioStrings.actionSave
            addButton?.isEnabled = !typed.trimmingCharacters(in: .whitespaces).isEmpty
            noticeSection?.isHidden = noticeRows.isEmpty
            noticeSection?.setRows(noticeRows)
        }

        private var noticeRows: [NSView] {
            var rows: [NSView] = []
            if let notice, draft == saved { rows.append(note(notice)) }
            if let failure { rows.append(note(failure, failed: true)) }
            return rows
        }

        private func rebuild() {
            let k = LibraryKindStyle.of(draft.kind)
            let head = NSStackView.horizontal(
                [
                    colorTile(k.icon, k.color, size: 40),
                    NSTextField.label(
                        library?.name ?? MavioStrings.librariesAdd, theme.fonts.title3,
                        color: theme.colors.textPrimary),
                    Spacer(), saveButton,
                ], spacing: MavioSpacing.s12)
            var views: [NSView] = [head]
            views.append(
                section(
                    nil,
                    [
                        fieldRow(MavioStrings.librariesName, text: draft.name) { [weak self] v in
                            self?.edit { $0.name = v }
                        },
                        choiceRow(
                            MavioStrings.librariesKind, nil, LibraryKindStyle.all.map { ($0.title, $0.kind) },
                            draft.kind
                        ) { [weak self] v in self?.choose { $0.kind = v } },
                    ]))
            views.append(folders())
            var metadata: [NSView] = [
                choiceRow(
                    MavioStrings.librariesLanguage, nil,
                    [(MavioStrings.librariesServerDefault, "")]
                        + choices(languages, draft.language).map { ($0.name, $0.code) }, draft.language
                ) { [weak self] v in self?.choose { $0.language = v } },
                choiceRow(
                    MavioStrings.librariesCountry, nil,
                    [(MavioStrings.librariesServerDefault, "")]
                        + choices(countries, draft.country).map { ($0.name, $0.code) }, draft.country
                ) { [weak self] v in self?.choose { $0.country = v } },
                toggle(
                    MavioStrings.librariesSaveLocalMetadata, MavioStrings.librariesSaveLocalMetadataDetail,
                    draft.saveLocalMetadata
                ) { $0.saveLocalMetadata = $1 },
            ]
            if k.movies {
                metadata.append(
                    toggle(
                        MavioStrings.librariesAutoCollections, MavioStrings.librariesAutoCollectionsDetail,
                        draft.autoCollections
                    ) { $0.autoCollections = $1 })
            }
            if k.music {
                metadata.append(
                    toggle(MavioStrings.librariesDownloadLyrics, nil, draft.downloadLyrics) { $0.downloadLyrics = $1 })
            }
            views.append(section(MavioStrings.librariesMetadata, metadata))
            var scanningRows: [NSView] = [
                choiceRow(
                    MavioStrings.librariesScanInterval, nil, intervals.map { ($0.title, $0.seconds) },
                    draft.scanSeconds
                ) { [weak self] v in self?.choose { $0.scanSeconds = v } }
            ]
            if k.video {
                scanningRows.append(
                    toggle(
                        MavioStrings.librariesTrickplay, MavioStrings.librariesTrickplayDetail, draft.extractTrickplay
                    ) { $0.extractTrickplay = $1 })
                scanningRows.append(
                    toggle(MavioStrings.librariesChapterImages, nil, draft.extractChapterImages) {
                        $0.extractChapterImages = $1
                    })
            }
            if k.music {
                scanningRows.append(
                    toggle(MavioStrings.librariesLoudness, MavioStrings.librariesLoudnessDetail, draft.analyzeLoudness)
                    { $0.analyzeLoudness = $1 })
            }
            views.append(section(MavioStrings.librariesScanning, scanningRows))
            if k.shows {
                var missing: [NSView] = [
                    toggleRow(MavioStrings.librariesMissingAired, nil, draft.missing) { [weak self] v in
                        self?.choose { $0.missing = v }
                    },
                    toggle(MavioStrings.librariesMissingUnaired, nil, draft.unaired) { $0.unaired = $1 },
                    toggle(MavioStrings.librariesMissingSpecials, nil, draft.specials) { $0.specials = $1 },
                ]
                if !draft.missing {
                    missing.append(
                        numberRow(
                            MavioStrings.librariesGraceDays, MavioStrings.librariesGraceDaysDetail,
                            value: Double(draft.graceDays), range: 0...3650
                        ) { [weak self] v in
                            self?.edit { $0.graceDays = v.map { Int32(clamping: Int64($0.rounded())) } ?? -1 }
                        })
                }
                views.append(
                    section(
                        MavioStrings.librariesMissingEpisodes, footer: MavioStrings.librariesMissingEpisodesFooter,
                        missing))
            }
            if let library {
                views.append(
                    section(
                        nil,
                        [
                            FormRow.action(scanning ? "…" : MavioStrings.librariesScan, theme: theme) {
                                [weak self] in self?.scan(library)
                            },
                            FormRow.action(MavioStrings.actionDelete, destructive: true, theme: theme) {
                                [weak self] in self?.confirmDelete(library)
                            },
                        ]))
            }
            let noticeSection = section()
            self.noticeSection = noticeSection
            views.append(noticeSection)
            form.setSections(views)
            refresh()
            refocus()
        }

        private func toggle(
            _ title: String, _ detail: String?, _ isOn: Bool, _ set: @escaping (inout LibraryDraft, Bool) -> Void
        ) -> FormRow {
            toggleRow(title, detail, isOn) { [weak self] v in self?.edit { set(&$0, v) } }
        }

        private var intervals: [(seconds: Int64, title: String)] {
            Self.intervals.contains { $0.seconds == draft.scanSeconds }
                ? Self.intervals
                : Self.intervals + [
                    (draft.scanSeconds, MavioStrings.librariesScanEveryHours(hours: String(draft.scanSeconds / 3600)))
                ]
        }

        private func folders() -> NSView {
            var rows: [NSView] = draft.paths.map(folderRow)
            let (frame, input) = field(typed, placeholder: MavioStrings.librariesFolderPath, width: nil) {
                [weak self] v in
                self?.typed = v
                self?.refresh()
            }
            input.onSubmit = { [weak self] in self?.addTyped() }
            let add = button(MavioStrings.librariesAddFolder) { [weak self] in self?.addTyped() }
            addButton = add
            rows.append(ControlRow(frame, controls: [add], theme: theme))
            rows.append(
                FormRow.link(MavioStrings.librariesBrowse, icon: "folder", tint: theme.colors.accent, theme: theme) {
                    [weak self] in
                    guard let self else { return }
                    push(
                        FolderPickerPage(context: context, server: server, chosen: draft.paths) { [weak self] path in
                            self?.add(path)
                        })
                })
            return section(MavioStrings.librariesFolders, footer: MavioStrings.librariesFoldersFooter, rows)
        }

        private func folderRow(_ path: String) -> NSView {
            let label = NSTextField.label(
                path, theme.fonts.body, color: theme.colors.textPrimary, lines: 0, selectable: true)
            let options: [(String, Mavio_Library_V1_StorageClass)] = [
                (auto(path), .unspecified),
                (MavioStrings.librariesStorageLocal, .local),
                (MavioStrings.librariesStorageNetwork, .network),
                (MavioStrings.librariesStorageCloud, .cloud),
            ]
            let selected = draft.storageClasses[path] ?? .unspecified
            let popup = ActionPopUp(
                options: options.map(\.0), selected: options.firstIndex { $0.1 == selected } ?? 0
            ) { [weak self] i in self?.edit { $0.storageClasses[path] = options[i].1 } }
            popup.font = theme.fonts.callout.font
            popup.controlSize = .small
            let storage = NSStackView.horizontal(
                [
                    NSTextField.label(
                        MavioStrings.librariesStorageClass, theme.fonts.callout, color: theme.colors.textSecondary),
                    FocusFrame(popup, theme: theme),
                ], spacing: MavioSpacing.s8)
            let content = NSStackView.vertical([label, storage], spacing: MavioSpacing.s4)
            let remove = MavioButton(icon: "close", kind: .plain, theme: theme) { [weak self] in
                self?.choose { $0.paths.removeAll { $0 == path } }
            }
            remove.setAccessibilityLabel(MavioStrings.librariesRemoveFolder)
            remove.toolTip = MavioStrings.librariesRemoveFolder
            return ControlRow(content, controls: [remove], theme: theme)
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

        private func addTyped() {
            let path = typed
            typed = ""
            add(path)
        }

        private func add(_ path: String) {
            var p = path.trimmingCharacters(in: .whitespaces)
            while p.count > 1, p.hasSuffix("/") { p.removeLast() }
            if !p.isEmpty, !draft.paths.contains(p) { draft.paths.append(p) }
            rebuild()
        }

        private func save() {
            // A field still being typed in would change the draft once saved.
            endEditing()
            saving = true
            failure = nil
            refresh()
            let draft = draft
            run { [weak self] in
                guard let self else { return }
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
                    self.draft = saved
                    notice = MavioStrings.librariesSaved
                    title = library?.name ?? MavioStrings.librariesAdd
                    changed()
                } catch {
                    failure = error.reason
                }
                saving = false
                rebuild()
            }
        }

        private func scan(_ library: Mavio_Library_V1_Library) {
            scanning = true
            rebuild()
            run { [weak self] in
                guard let self else { return }
                notice = await model.scan(server, library.id)
                scanning = false
                rebuild()
            }
        }

        private func confirmDelete(_ library: Mavio_Library_V1_Library) {
            confirm(
                MavioStrings.librariesDeleteTitle(name: library.name), message: MavioStrings.librariesDeleteMessage,
                button: MavioStrings.actionDelete
            ) { [weak self] in self?.delete(library) }
        }

        private func delete(_ library: Mavio_Library_V1_Library) {
            run { [weak self] in
                guard let self else { return }
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
                    refresh()
                }
            }
        }
    }

    // MARK: Folder picker

    /// The folders of the server's file system and of its rclone remotes, to
    /// walk and choose one from.
    final class FolderPickerPage: ServerPage {
        private let chosen: [String]
        private let choose: @MainActor (String) -> Void
        private var path = ""
        private var here = ""
        private var above = ""
        private var folders: [(path: String, name: String)]?
        private var failure: String?
        private var loading: Task<Void, Never>?
        private lazy var form = FormPage(theme: theme)
        private lazy var chooseButton = button(MavioStrings.librariesChooseFolder, kind: .primary) { [weak self] in
            guard let self else { return }
            choose(here)
            dismiss()
        }
        private lazy var hereLabel = NSTextField.label(
            "", theme.fonts.callout, color: theme.colors.textSecondary, lines: 0, selectable: true)
        private lazy var list = section()

        /// A picker of the folders of server, those already chosen not to be
        /// chosen again, calling choose with the one chosen.
        init(
            context: MacContext, server: ServerRecord, chosen: [String], choose: @escaping @MainActor (String) -> Void
        ) {
            self.chosen = chosen
            self.choose = choose
            super.init(context: context, server: server)
        }

        override func viewDidLoad() {
            super.viewDidLoad()
            title = MavioStrings.librariesBrowse
            form.add(heading(MavioStrings.librariesBrowse, actions: [chooseButton]))
            form.add(section(nil, [padded(hereLabel)]))
            form.add(list)
            show(form)
            go(to: "")
        }

        private var remote: Bool { path.hasPrefix("remote:") }

        private func go(to path: String) {
            self.path = path
            loading?.cancel()
            loading = Task { [weak self] in await self?.load() }
        }

        private func render() {
            hereLabel.stringValue = here.isEmpty ? MavioStrings.librariesRoots : here
            chooseButton.isEnabled = !here.isEmpty && !chosen.contains(here)
            var rows: [NSView] = []
            if !here.isEmpty {
                let back = NSStackView.horizontal(
                    [
                        IconView("back", size: 18, color: theme.colors.accentText),
                        NSTextField.label(
                            MavioStrings.librariesParentFolder, theme.isTV ? theme.fonts.headline : theme.fonts.body,
                            color: theme.colors.accentText),
                    ], spacing: MavioSpacing.s12)
                let above = above
                rows.append(ContentRow(back, chevron: false, theme: theme) { [weak self] in self?.go(to: above) })
            }
            for f in folders ?? [] {
                let label = NSStackView.horizontal(
                    [
                        IconView("folder", size: 18, color: theme.colors.textSecondary),
                        NSTextField.label(
                            f.name, theme.isTV ? theme.fonts.headline : theme.fonts.body,
                            color: theme.colors.textPrimary),
                    ], spacing: MavioSpacing.s12)
                rows.append(ContentRow(label, theme: theme) { [weak self] in self?.go(to: f.path) })
            }
            if let failure {
                rows.append(note(failure, failed: true))
            } else if folders?.isEmpty == true {
                rows.append(note(MavioStrings.librariesNoFolders))
            } else if folders == nil {
                rows.append(padded(LoadingView()))
            }
            list.setRows(rows)
            refocus()
        }

        private func load() async {
            folders = nil
            failure = nil
            render()
            do {
                if remote {
                    var request = Mavio_Storage_V1_ListFoldersRequest()
                    request.path = path
                    let r = try await model.call(
                        server, Procedure.storage("ListFolders"), request,
                        as: Mavio_Storage_V1_ListFoldersResponse.self)
                    here = path
                    above = Self.remoteParent(path)
                    folders = r.folders.map { f in
                        let cut = f.lastIndex(of: "/") ?? f.lastIndex(of: ":") ?? f.startIndex
                        return (f, cut == f.startIndex ? f : String(f[f.index(after: cut)...]))
                    }
                } else {
                    var request = Mavio_System_V1_ListDirectoryRequest()
                    request.path = path
                    let r = try await model.call(
                        server, Procedure.system("ListDirectory"), request,
                        as: Mavio_System_V1_ListDirectoryResponse.self)
                    here = r.path
                    above = r.parent
                    var list = r.entries.filter(\.isDir).map { (path: $0.path, name: $0.name) }
                    if path.isEmpty { list += await remotes() }
                    folders = list
                }
            } catch is CancellationError {
                return
            } catch {
                failure = error.reason
            }
            render()
        }

        /// The roots of the rclone remotes, when the server reaches rclone.
        private func remotes() async -> [(path: String, name: String)] {
            guard
                let status = try? await model.call(
                    server, Procedure.storage("GetStatus"), Mavio_Storage_V1_GetStatusRequest(),
                    as: Mavio_Storage_V1_GetStatusResponse.self), status.available,
                let r = try? await model.call(
                    server, Procedure.storage("ListRemotes"), Mavio_Storage_V1_ListRemotesRequest(),
                    as: Mavio_Storage_V1_ListRemotesResponse.self)
            else { return [] }
            return r.remotes.map { ("remote:\($0.name):", "remote:\($0.name):") }
        }

        /// The folder above a remote's: the remote's root, or the roots above
        /// it.
        private static func remoteParent(_ path: String) -> String {
            let rest = path.dropFirst("remote:".count)
            guard let colon = rest.firstIndex(of: ":") else { return "" }
            let inside = rest[rest.index(after: colon)...]
            if inside.isEmpty { return "" }
            if let slash = path.lastIndex(of: "/") { return String(path[..<slash]) }
            return String(path[...colon])
        }

        isolated deinit {
            loading?.cancel()
        }
    }
#endif
