#if os(macOS)
    import AppKit
    import MavioMac
    import UniformTypeIdentifiers

    // An item's metadata from its More menu, for the server's
    // administrators (docs/clients.md §6.3.22), as the web UI's dialogs: its
    // fields edited and locked, the item identified from the providers'
    // search results, its metadata refreshed, and its images chosen from the
    // providers' or uploaded. In television mode the dialogs are panels over
    // the page, their fields typed on its keyboard.

    /// Opens the dialog of a metadata action on an item of a server over
    /// page; changed is called once the item changed.
    @MainActor
    func openMetadata(
        _ action: MetadataAction, serverId: String, itemId: String, name: String, over page: Page,
        changed: @escaping @MainActor () -> Void
    ) {
        switch action {
        case .edit:
            EditMetadataDialog(page: page, serverId: serverId, itemId: itemId, name: name, changed: changed).present()
        case .identify:
            IdentifyDialog(page: page, serverId: serverId, itemId: itemId, name: name, changed: changed).present()
        case .refresh:
            RefreshDialog(page: page, serverId: serverId, itemId: itemId, name: name, changed: changed).present()
        case .images:
            ImagesDialog(page: page, serverId: serverId, itemId: itemId, name: name, changed: changed).present()
        }
    }

    /// What the metadata dialogs share: the dialog over its page, the item,
    /// and keeping themselves while shown.
    @MainActor
    private class MetadataDialog {
        let dialog: Dialog
        weak var page: Page?
        let model: AppModel
        let serverId: String
        let itemId: String
        let changed: @MainActor () -> Void
        let failure: NSTextField
        var tasks: [Task<Void, Never>] = []
        private var retained: MetadataDialog?

        init(
            title: String, width: CGFloat = 560, page: Page, serverId: String, itemId: String,
            changed: @escaping @MainActor () -> Void
        ) {
            self.page = page
            model = page.model
            self.serverId = serverId
            self.itemId = itemId
            self.changed = changed
            dialog = Dialog(title: title, width: width, theme: page.theme)
            failure = .label("", page.theme.fonts.footnote, color: page.theme.colors.danger, lines: 0)
            failure.isHidden = true
        }

        isolated deinit { tasks.forEach { $0.cancel() } }

        var theme: Theme { dialog.theme }

        func present() {
            guard let page else { return }
            retained = self
            dialog.onClose = { [weak self] in self?.retained = nil }
            dialog.present(over: page)
            start()
        }

        /// What the dialog does as it shows.
        func start() {}

        func fail(_ message: String?) {
            failure.stringValue = message ?? ""
            failure.isHidden = message == nil
        }

        func run(_ action: @escaping @MainActor () async throws -> Void, then done: @escaping @MainActor () -> Void) {
            fail(nil)
            tasks.append(
                Task { [weak self] in
                    do {
                        try await action()
                        guard !Task.isCancelled else { return }
                        done()
                    } catch is CancellationError {
                    } catch {
                        self?.fail(error.reason)
                    }
                })
        }

        func cancelButton(_ title: String = MavioStrings.actionCancel) -> MavioButton {
            MavioButton(title, kind: .secondary, theme: theme) { [weak self] in self?.dialog.close() }
        }

        /// A row of a field of text.
        func field(_ title: String, _ input: TextInput, width: CGFloat = 280) -> NSView {
            ControlRow(title, controls: [FocusFrame(input, theme: theme).size(width: width)], theme: theme)
        }

        func note(_ text: String) -> NSTextField {
            .label(text, theme.fonts.callout, color: theme.colors.textSecondary, lines: 0)
        }
    }

    // MARK: Edit

    /// Edits an item's fields, external IDs and locks; saves only those
    /// changed.
    private final class EditMetadataDialog: MetadataDialog {
        private var kind = Mavio_Library_V1_ItemKind.unspecified
        private var before: MetadataForm?
        private var form = MetadataForm()
        private var idKinds: [Mavio_Library_V1_ExternalIdKind] = []
        private var inputs: [(WritableKeyPath<MetadataForm, String>, TextInput)] = []
        private var idInputs: [(String, TextInput)] = []
        private var save: MavioButton?

        init(page: Page, serverId: String, itemId: String, name: String, changed: @escaping @MainActor () -> Void) {
            super.init(
                title: MavioStrings.metadataEditTitle(name: name), width: 640, page: page, serverId: serverId,
                itemId: itemId, changed: changed)
            let save = MavioButton(MavioStrings.metadataSave, kind: .primary, theme: theme) { [weak self] in
                self?.submit()
            }
            save.isEnabled = false
            self.save = save
            dialog.setButtons(trailing: [cancelButton(), save])
            dialog.setContent([LoadingView()])
        }

        override func start() {
            let (model, serverId, itemId) = (model, serverId, itemId)
            tasks.append(
                Task { [weak self] in
                    do {
                        let item = try await model.item(serverId, itemId)
                        let kinds = (try? await model.externalIdKinds(serverId, item.kind)) ?? []
                        guard let self, !Task.isCancelled else { return }
                        self.kind = item.kind
                        self.form = MetadataForm(item)
                        self.before = self.form
                        self.idKinds = kinds
                        self.render()
                    } catch is CancellationError {
                    } catch {
                        guard let self else { return }
                        self.fail(error.reason)
                        self.dialog.setContent([self.failure])
                    }
                })
        }

        private var video: Bool { [.movie, .episode, .video, .musicVideo].contains(kind) }
        private var music: Bool { [.musicAlbum, .track, .musicVideo].contains(kind) }

        /// Keeps what the fields hold in the form.
        private func harvest() {
            for (path, input) in inputs { form[keyPath: path] = input.text }
            for (key, input) in idInputs { form.externalIds[key] = input.text }
        }

        private func render() {
            harvest()
            inputs = []
            idInputs = []
            var main: [NSView] = [
                text(MavioStrings.metadataName, \.name),
                text(MavioStrings.metadataOriginalTitle, \.originalTitle),
                text(MavioStrings.metadataSortName, \.sortName, placeholder: form.name),
                text(MavioStrings.metadataOverview, \.overview, lines: 6),
            ]
            if kind == .movie || kind == .series { main.append(text(MavioStrings.metadataTagline, \.tagline)) }
            var numbers: [NSView] = []
            if kind == .season { numbers.append(text(MavioStrings.metadataSeasonNumber, \.indexNumber, width: 120)) }
            if kind == .episode {
                numbers.append(text(MavioStrings.metadataSeasonNumber, \.parentIndexNumber, width: 120))
            }
            if kind == .episode || kind == .track {
                numbers.append(
                    text(
                        kind == .track ? MavioStrings.metadataTrackNumber : MavioStrings.metadataEpisodeNumber,
                        \.indexNumber, width: 120))
            }
            if kind == .episode {
                numbers.append(text(MavioStrings.metadataAbsoluteNumber, \.absoluteNumber, width: 120))
            }
            numbers.append(text(MavioStrings.metadataYear, \.productionYear, width: 120))
            numbers.append(
                text(
                    kind == .series ? MavioStrings.metadataFirstAired : MavioStrings.metadataReleaseDate,
                    \.premiereDate, placeholder: "YYYY-MM-DD", width: 160))
            if kind == .series {
                numbers.append(text(MavioStrings.metadataEndDate, \.endDate, placeholder: "YYYY-MM-DD", width: 160))
                let statuses = MetadataForm.statuses
                let popup = ActionPopUp(
                    options: statuses.map(\.1), selected: statuses.firstIndex { $0.0 == form.seriesStatus } ?? 0
                ) { [weak self] i in self?.form.seriesStatus = statuses[i].0 }
                numbers.append(
                    ControlRow(
                        MavioStrings.metadataStatus, controls: [FocusFrame(popup, theme: theme)], theme: theme))
            }
            if video { numbers.append(text(MavioStrings.metadataRuntimeMinutes, \.runtime, width: 120)) }
            let ratings: [NSView] = [
                text(MavioStrings.metadataOfficialRating, \.officialRating, width: 160),
                text(MavioStrings.metadataCustomRating, \.customRating, width: 160),
                text(MavioStrings.metadataCommunityRating, \.communityRating, width: 120),
                text(MavioStrings.metadataCriticRating, \.criticRating, width: 120),
            ]
            var lists: [NSView] = []
            let commas = MavioStrings.metadataCommas
            if music { lists.append(text(MavioStrings.metadataArtists, \.artists, placeholder: commas)) }
            if kind == .musicAlbum {
                lists.append(text(MavioStrings.metadataAlbumArtists, \.albumArtists, placeholder: commas))
            }
            lists += [
                text(MavioStrings.metadataGenres, \.genres, placeholder: commas),
                text(MavioStrings.metadataTags, \.tags, placeholder: commas),
                text(MavioStrings.metadataStudios, \.studios, placeholder: commas),
                text(MavioStrings.metadataLocations, \.productionLocations, placeholder: commas),
            ]
            var sections: [NSView] = [
                FormSection(rows: main, theme: theme), FormSection(rows: numbers, theme: theme),
                FormSection(rows: ratings, theme: theme), FormSection(rows: lists, theme: theme),
            ]
            if !idKinds.isEmpty {
                let rows = idKinds.map { k -> NSView in
                    let input = TextInput(form.externalIds[k.key] ?? "")
                    input.onChange = { [weak self] _ in self?.update() }
                    idInputs.append((k.key, input))
                    return field(k.name, input, width: 200)
                }
                sections.append(FormSection(MavioStrings.metadataExternalIds, rows: rows, theme: theme))
            }
            var locks: [NSView] = [
                FormRow.toggle(MavioStrings.metadataLockAll, isOn: form.locked, theme: theme) { [weak self] on in
                    guard let self else { return }
                    self.harvest()
                    self.form.locked = on
                    self.render()
                }
            ]
            if !form.locked {
                for (lockable, label) in MetadataForm.lockable {
                    locks.append(
                        FormRow.toggle(label, isOn: form.lockedFields.contains(lockable), theme: theme) {
                            [weak self] on in
                            guard let self else { return }
                            self.form.lockedFields =
                                on
                                ? self.form.lockedFields + [lockable] : self.form.lockedFields.filter { $0 != lockable }
                            self.update()
                        })
                }
            }
            sections.append(
                FormSection(
                    MavioStrings.metadataLocks, footer: MavioStrings.metadataLocksFooter, rows: locks, theme: theme))
            sections.append(failure)
            dialog.setContent(sections)
            update()
        }

        private func text(
            _ title: String, _ path: WritableKeyPath<MetadataForm, String>, placeholder: String? = nil,
            lines: Int = 1, width: CGFloat = 320
        ) -> NSView {
            let input = TextInput(form[keyPath: path], placeholder: placeholder, lines: lines)
            input.onChange = { [weak self] _ in self?.update() }
            inputs.append((path, input))
            return field(title, input, width: width)
        }

        /// Saving needs a name.
        private func update() {
            let name = inputs.first { $0.0 == \MetadataForm.name }?.1.text ?? form.name
            save?.isEnabled = before != nil && !name.trimmingCharacters(in: .whitespaces).isEmpty
        }

        private func submit() {
            guard let before else { return }
            harvest()
            save?.isEnabled = false
            let (model, serverId, itemId, form) = (model, serverId, itemId, form)
            run {
                try await model.updateItem(serverId, itemId, form, from: before)
            } then: { [weak self] in
                self?.changed()
                self?.dialog.close()
            }
            tasks.append(Task { [weak self] in self?.update() })
        }
    }

    // MARK: Identify

    /// Searches the providers for what the item is by a name and year, the
    /// item's at first, and identifies it as a result.
    private final class IdentifyDialog: MetadataDialog {
        private let query = TextInput()
        private let year = TextInput()
        private var results: [Mavio_Library_V1_RemoteSearchResult]?
        private var searching = false
        private var busy = false

        init(page: Page, serverId: String, itemId: String, name: String, changed: @escaping @MainActor () -> Void) {
            super.init(
                title: MavioStrings.metadataIdentifyTitle(name: name), width: 640, page: page, serverId: serverId,
                itemId: itemId, changed: changed)
            query.text = name
            query.onSubmit = { [weak self] in self?.search() }
            year.onSubmit = { [weak self] in self?.search() }
            dialog.setButtons(trailing: [cancelButton()])
            render()
        }

        override func start() {
            let (model, serverId, itemId) = (model, serverId, itemId)
            tasks.append(
                Task { [weak self] in
                    if let item = try? await model.item(serverId, itemId) {
                        guard let self else { return }
                        self.query.text = item.name
                        self.year.text = item.productionYear > 0 ? "\(item.productionYear)" : ""
                    }
                    self?.search()
                })
        }

        private func render() {
            let searchButton = MavioButton(MavioStrings.metadataSearch, kind: .primary, theme: theme) {
                [weak self] in self?.search()
            }
            searchButton.isEnabled = !searching
            var views: [NSView] = [
                FormSection(
                    rows: [
                        field(MavioStrings.metadataName, query), field(MavioStrings.metadataYear, year, width: 120),
                    ],
                    theme: theme),
                NSStackView.horizontal([Spacer(), searchButton], spacing: 0),
                failure,
            ]
            if searching {
                views.append(note(MavioStrings.metadataSearching))
            } else if let results {
                if results.isEmpty { views.append(note(MavioStrings.metadataNoResults)) }
                views += results.map(row)
            }
            dialog.setContent(views)
        }

        private func row(_ r: Mavio_Library_V1_RemoteSearchResult) -> NSView {
            let poster = ArtworkView(r.imageURL.isEmpty ? nil : r.imageURL, theme: theme)
            poster.radius = MavioRadius.sm
            poster.size(width: theme.isTV ? 72 : 48).aspect(2 / 3)
            var lines: [NSView] = [
                NSTextField.label(
                    r.year > 0 ? "\(r.name) (\(r.year))" : r.name, theme.fonts.headline,
                    color: theme.colors.textPrimary),
                NSTextField.label(remoteResultIds(r), theme.fonts.footnote, color: theme.colors.textSecondary),
            ]
            if !r.overview.isEmpty {
                lines.append(
                    NSTextField.label(r.overview, theme.fonts.footnote, color: theme.colors.textSecondary, lines: 2))
            }
            let text = NSStackView.vertical(lines, spacing: 2)
            text.setHuggingPriority(.init(1), for: .horizontal)
            let content = NSStackView.horizontal([poster, text], spacing: MavioSpacing.s12, alignment: .top)
            let row = Pressable(
                content, radius: MavioRadius.md, insets: .all(MavioSpacing.s8), theme: theme
            ) { [weak self] in self?.identify(r) }
            row.setAccessibilityLabel(r.name)
            row.setAccessibilityHelp(MavioStrings.metadataIdentify)
            return row
        }

        private func search() {
            let name = query.text.trimmingCharacters(in: .whitespaces)
            guard !name.isEmpty, !searching else { return }
            searching = true
            fail(nil)
            render()
            let (model, serverId, itemId, year) = (model, serverId, itemId, Int32(year.text))
            tasks.append(
                Task { [weak self] in
                    let found: [Mavio_Library_V1_RemoteSearchResult]
                    var failed: String?
                    do {
                        found = try await model.searchRemote(serverId, itemId, name: name, year: year)
                    } catch {
                        found = []
                        failed = error.reason
                    }
                    guard let self, !Task.isCancelled else { return }
                    self.searching = false
                    self.results = found
                    self.render()
                    self.fail(failed)
                })
        }

        private func identify(_ r: Mavio_Library_V1_RemoteSearchResult) {
            guard !busy else { return }
            busy = true
            let (model, serverId, itemId) = (model, serverId, itemId)
            run {
                try await model.identify(serverId, itemId, as: r)
            } then: { [weak self] in
                self?.changed()
                self?.dialog.close()
            }
            tasks.append(Task { [weak self] in self?.busy = false })
        }
    }

    // MARK: Refresh

    /// Refreshes the item's metadata, filling what is missing or replacing
    /// all of it.
    private final class RefreshDialog: MetadataDialog {
        private var replace = false
        private var section: FormSection?
        private var refresh: MavioButton?

        init(page: Page, serverId: String, itemId: String, name: String, changed: @escaping @MainActor () -> Void) {
            super.init(
                title: MavioStrings.metadataRefreshTitle(name: name), page: page, serverId: serverId, itemId: itemId,
                changed: changed)
            let section = FormSection(footer: MavioStrings.metadataRefreshFooter, theme: theme)
            section.setRows([
                FormRow.toggle(MavioStrings.metadataReplace, isOn: false, theme: theme) { [weak self] on in
                    self?.replace = on
                    self?.section?.footer = on ? MavioStrings.metadataReplaceFooter : MavioStrings.metadataRefreshFooter
                }
            ])
            self.section = section
            let refresh = MavioButton(MavioStrings.metadataRefresh, kind: .primary, theme: theme) { [weak self] in
                self?.submit()
            }
            self.refresh = refresh
            dialog.setButtons(trailing: [cancelButton(), refresh])
            dialog.setContent([section, failure])
        }

        private func submit() {
            refresh?.isEnabled = false
            refresh?.title = MavioStrings.metadataRefreshing
            let (model, serverId, itemId, replace) = (model, serverId, itemId, replace)
            run {
                do {
                    try await model.refreshMetadata(serverId, itemId, replace: replace)
                } catch {
                    self.refresh?.isEnabled = true
                    self.refresh?.title = MavioStrings.metadataRefresh
                    throw error
                }
            } then: { [weak self] in
                self?.changed()
                self?.dialog.close()
            }
        }
    }

    // MARK: Images

    /// The item's images of a kind, each deleted after confirmation, and
    /// the providers' images of the kind, one chosen to replace the first;
    /// on the desktop an image file is uploaded instead.
    private final class ImagesDialog: MetadataDialog {
        private var kind = Mavio_Library_V1_ImageKind.primary
        private var current: [Mavio_Library_V1_Image] = []
        private var remote: [Mavio_Library_V1_RemoteImage]?
        private var busy = false
        private var loadTask: Task<Void, Never>?

        init(page: Page, serverId: String, itemId: String, name: String, changed: @escaping @MainActor () -> Void) {
            super.init(
                title: MavioStrings.metadataImagesTitle(name: name), width: 760, page: page, serverId: serverId,
                itemId: itemId, changed: changed)
            let done = MavioButton(MavioStrings.actionDone, kind: .primary, theme: theme) { [weak self] in
                self?.dialog.close()
            }
            dialog.setButtons(trailing: [done])
        }

        override func start() {
            render()
            loadRemote()
            loadCurrent()
        }

        private var wide: Bool { wideImages(kind) }

        private func render() {
            let kinds = metadataImageKinds
            let popup = ActionPopUp(options: kinds.map(\.1), selected: kinds.firstIndex { $0.0 == kind } ?? 0) {
                [weak self] i in
                guard let self, kinds[i].0 != self.kind else { return }
                self.kind = kinds[i].0
                self.current = []
                self.remote = nil
                self.render()
                self.loadRemote()
                self.loadCurrent()
            }
            var controls: [NSView] = [FocusFrame(popup, theme: theme)]
            if !theme.isTV {
                let upload = MavioButton(
                    MavioStrings.metadataUploadImage, icon: "photos", kind: .secondary, theme: theme
                ) {
                    [weak self] in self?.upload()
                }
                upload.isEnabled = !busy
                controls.append(upload)
            }
            var views: [NSView] = [
                ControlRow(MavioStrings.metadataImageKind, controls: controls, theme: theme), failure,
                heading(MavioStrings.metadataCurrent),
            ]
            if current.isEmpty {
                views.append(note(MavioStrings.metadataNoImage))
            } else {
                views.append(
                    grid(
                        current.map { image in
                            tile(
                                model.server(serverId)?.imageURL(image.url, maxWidth: 640)?.absoluteString,
                                caption: image.width > 0 && image.height > 0
                                    ? "\(image.width)×\(image.height)" : MavioStrings.metadataDeleteImage,
                                label: MavioStrings.metadataDeleteImage
                            ) { [weak self] in self?.confirmDelete(image) }
                        }))
            }
            views.append(heading(MavioStrings.metadataFromProviders))
            if let remote {
                if remote.isEmpty {
                    views.append(note(MavioStrings.metadataNoRemoteImages))
                } else {
                    views.append(
                        grid(
                            remote.map { image in
                                tile(image.url, caption: remoteImageCaption(image), label: remoteImageCaption(image)) {
                                    [weak self] in self?.choose(image)
                                }
                            }))
                }
            } else {
                views.append(note(MavioStrings.metadataSearching))
            }
            dialog.setContent(views)
        }

        private func heading(_ text: String) -> NSTextField {
            .label(text, theme.fonts.headline, color: theme.colors.textPrimary)
        }

        private func grid(_ tiles: [NSView]) -> NSView {
            let base: CGFloat = wide ? 200 : 120
            let grid = FlowGrid(
                minimum: theme.isTV ? base * 1.4 : base, gutter: MavioSpacing.s12, rowSpacing: MavioSpacing.s12)
            grid.setItems(tiles)
            return grid
        }

        private func tile(
            _ url: String?, caption: String, label: String, action: @escaping @MainActor () -> Void
        ) -> NSView {
            let art = ArtworkView(url, theme: theme)
            art.radius = MavioRadius.sm
            art.aspect(wide ? 16 / 9 : 2 / 3)
            let text = NSTextField.label(caption, theme.fonts.caption, color: theme.colors.textSecondary)
            let stack = NSStackView.vertical([art, text], spacing: MavioSpacing.s4)
            art.widthAnchor.constraint(equalTo: stack.widthAnchor).isActive = true
            let tile = Pressable(stack, look: .lift, radius: MavioRadius.sm, theme: theme) { [weak self] in
                guard self?.busy == false else { return }
                action()
            }
            tile.setAccessibilityLabel(label)
            return tile
        }

        private func loadCurrent() {
            let (model, serverId, itemId, kind) = (model, serverId, itemId, kind)
            loadTask?.cancel()
            loadTask = Task { [weak self] in
                guard let item = try? await model.item(serverId, itemId), let self, !Task.isCancelled else { return }
                self.current = item.images.filter { $0.kind == kind && !$0.url.isEmpty }
                self.render()
            }
        }

        private func loadRemote() {
            let (model, serverId, itemId, kind) = (model, serverId, itemId, kind)
            tasks.append(
                Task { [weak self] in
                    var failed: String?
                    let images: [Mavio_Library_V1_RemoteImage]
                    do {
                        images = try await model.remoteImages(serverId, itemId, kind)
                    } catch {
                        images = []
                        failed = error.reason
                    }
                    guard let self, !Task.isCancelled, self.kind == kind else { return }
                    self.remote = images
                    self.render()
                    self.fail(failed)
                })
        }

        /// Runs a change of the images, then reads them again.
        private func change(_ action: @escaping @MainActor () async throws -> Void) {
            busy = true
            render()
            run {
                defer { self.busy = false }
                try await action()
            } then: { [weak self] in
                self?.changed()
                self?.loadCurrent()
            }
            tasks.append(Task { [weak self] in self?.render() })
        }

        private func choose(_ image: Mavio_Library_V1_RemoteImage) {
            let (model, serverId, itemId, kind) = (model, serverId, itemId, kind)
            change { try await model.setItemImage(serverId, itemId, kind, url: image.url) }
        }

        private func upload() {
            guard let window = page?.view.window else { return }
            let panel = NSOpenPanel()
            panel.allowedContentTypes = [.image]
            panel.allowsMultipleSelection = false
            panel.beginSheetModal(for: dialog.presenter?.view.window ?? window) { [weak self] response in
                guard response == .OK, let url = panel.url else { return }
                MainActor.assumeIsolated {
                    guard let self else { return }
                    let (model, serverId, itemId, kind) = (self.model, self.serverId, self.itemId, self.kind)
                    self.change {
                        let data = try Data(contentsOf: url)
                        try await model.setItemImage(serverId, itemId, kind, data: data)
                    }
                }
            }
        }

        private func confirmDelete(_ image: Mavio_Library_V1_Image) {
            guard let presenter = dialog.presenter else { return }
            let confirm = Dialog(title: MavioStrings.metadataDeleteImage, width: 420, theme: theme)
            let delete = MavioButton(MavioStrings.metadataConfirmDelete, kind: .destructive, theme: theme) {
                [weak self, weak confirm] in
                confirm?.close()
                guard let self else { return }
                let (model, serverId, itemId) = (self.model, self.serverId, self.itemId)
                self.change { try await model.deleteItemImage(serverId, itemId, imageId: image.id) }
            }
            let cancel = MavioButton(MavioStrings.actionCancel, kind: .secondary, theme: theme) { [weak confirm] in
                confirm?.close()
            }
            confirm.setButtons(trailing: [cancel, delete])
            confirm.present(over: presenter)
        }
    }
#endif
