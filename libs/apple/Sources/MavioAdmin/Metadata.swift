import MavioCommon
import MavioKit
import MavioUI
import SwiftUI

#if os(iOS)
    import PhotosUI
#endif

// An item's metadata from its More menu, for the server's administrators
// (docs/clients.md §6.3.22), on the iPhone and iPad and on Apple TV: its
// fields edited and locked, the item identified from the providers'
// search results, its metadata refreshed, and its images chosen from the
// providers' or, on the iPhone and iPad, uploaded.

/// The dialog of a metadata action on an item.
public struct ItemMetadataView: View {
    let serverId: String
    let itemId: String
    let name: String
    let action: MetadataAction

    /// The dialog of action on an item of a server, named name.
    public init(serverId: String, itemId: String, name: String, action: MetadataAction) {
        (self.serverId, self.itemId, self.name, self.action) = (serverId, itemId, name, action)
    }

    /// The view's content.
    public var body: some View {
        NavigationStack {
            switch action {
            case .edit: EditMetadataView(serverId: serverId, itemId: itemId, name: name)
            case .identify: IdentifyView(serverId: serverId, itemId: itemId, name: name)
            case .refresh: RefreshItemView(serverId: serverId, itemId: itemId, name: name)
            case .images: ItemImagesView(serverId: serverId, itemId: itemId, name: name)
            }
        }
    }
}

/// A row saying why something could not be done.
private struct Failure: View {
    let text: String
    @Environment(\.theme) private var theme

    var body: some View {
        Text(text).foregroundStyle(theme.colors.danger)
    }
}

extension View {
    /// A metadata dialog's form: on the background, with an inline title.
    fileprivate func metadataPage(_ title: String) -> some View {
        modifier(MetadataPage(title: title))
    }
}

private struct MetadataPage: ViewModifier {
    let title: String
    @Environment(\.theme) private var theme

    func body(content: Content) -> some View {
        content.hiddenListBackground().background(theme.colors.background).navigationTitle(title).inlineTitle()
    }
}

// MARK: Edit

/// Edits an item's fields, external IDs and locks; saves only those
/// changed.
private struct EditMetadataView: View {
    let serverId: String
    let itemId: String
    let name: String
    @Environment(AppModel.self) private var model
    @Environment(\.dismiss) private var dismiss
    @State private var kind = Mavio_Library_V1_ItemKind.unspecified
    @State private var before: MetadataForm?
    @State private var form = MetadataForm()
    @State private var idKinds: [Mavio_Library_V1_ExternalIdKind] = []
    @State private var languages: [Choice] = []
    @State private var countries: [Choice] = []
    @State private var busy = false
    @State private var failure: String?

    var body: some View {
        Group {
            if before != nil { fields } else if let failure { Form { Failure(text: failure) } } else { ProgressView() }
        }
        .metadataPage(MavioStrings.metadataEditTitle(name: name))
        .toolbar {
            ToolbarItem(placement: .cancellationAction) {
                Button(MavioStrings.actionCancel) { dismiss() }
            }
            ToolbarItem(placement: .confirmationAction) {
                Button(MavioStrings.metadataSave, action: save)
                    .disabled(before == nil || busy || form.name.trimmingCharacters(in: .whitespaces).isEmpty)
            }
        }
        .interactiveDismissDisabled(before != nil && form != before)
        .task {
            // Once: coming back from a picker's page keeps what was chosen.
            guard before == nil else { return }
            do {
                let item = try await model.item(serverId, itemId)
                kind = item.kind
                form = MetadataForm(item)
                before = form
                async let kinds = (try? model.externalIdKinds(serverId, item.kind)) ?? []
                if identifiable(item.kind) { (languages, countries) = await model.localizationChoices(serverId) }
                idKinds = await kinds
            } catch {
                failure = error.reason
            }
        }
    }

    private var video: Bool { [.movie, .episode, .video, .musicVideo].contains(kind) }
    private var music: Bool { [.musicAlbum, .track, .musicVideo].contains(kind) }

    private var fields: some View {
        Form {
            Section {
                text(MavioStrings.metadataName, $form.name)
                text(MavioStrings.metadataOriginalTitle, $form.originalTitle)
                text(MavioStrings.metadataSortName, $form.sortName, prompt: form.name)
                LabeledContent(MavioStrings.metadataOverview) {
                    TextField(MavioStrings.metadataOverview, text: $form.overview, axis: .vertical)
                        .lineLimit(3...10)
                }
                if kind == .movie || kind == .series { text(MavioStrings.metadataTagline, $form.tagline) }
            }
            Section {
                if kind == .season { number(MavioStrings.metadataSeasonNumber, $form.indexNumber) }
                if kind == .episode { number(MavioStrings.metadataSeasonNumber, $form.parentIndexNumber) }
                if kind == .episode || kind == .track {
                    number(
                        kind == .track ? MavioStrings.metadataTrackNumber : MavioStrings.metadataEpisodeNumber,
                        $form.indexNumber)
                }
                if kind == .episode { number(MavioStrings.metadataAbsoluteNumber, $form.absoluteNumber) }
                number(MavioStrings.metadataYear, $form.productionYear)
                day(
                    kind == .series ? MavioStrings.metadataFirstAired : MavioStrings.metadataReleaseDate,
                    $form.premiereDate)
                if kind == .series {
                    day(MavioStrings.metadataEndDate, $form.endDate)
                    Picker(MavioStrings.metadataStatus, selection: $form.seriesStatus) {
                        ForEach(MetadataForm.statuses, id: \.0) { Text($0.1).tag($0.0) }
                    }
                }
                if video { number(MavioStrings.metadataRuntimeMinutes, $form.runtime) }
            }
            Section {
                text(MavioStrings.metadataOfficialRating, $form.officialRating)
                text(MavioStrings.metadataCustomRating, $form.customRating)
                number(MavioStrings.metadataCommunityRating, $form.communityRating, decimals: true)
                number(MavioStrings.metadataCriticRating, $form.criticRating, decimals: true)
            }
            Section {
                if music { text(MavioStrings.metadataArtists, $form.artists, prompt: MavioStrings.metadataCommas) }
                if kind == .musicAlbum {
                    text(MavioStrings.metadataAlbumArtists, $form.albumArtists, prompt: MavioStrings.metadataCommas)
                }
                text(MavioStrings.metadataGenres, $form.genres, prompt: MavioStrings.metadataCommas)
                text(MavioStrings.metadataTags, $form.tags, prompt: MavioStrings.metadataCommas)
                text(MavioStrings.metadataStudios, $form.studios, prompt: MavioStrings.metadataCommas)
                text(MavioStrings.metadataLocations, $form.productionLocations, prompt: MavioStrings.metadataCommas)
            }
            // The language and country the providers describe it in, for
            // the kinds they identify, its library's by default.
            if identifiable(kind) {
                Section {
                    SearchingPicker(
                        MavioStrings.librariesLanguage, selection: $form.metadataLanguage,
                        options: [("", MavioStrings.metadataLibraryDefault)]
                            + choices(languages, form.metadataLanguage).map { ($0.code, $0.name) })
                    SearchingPicker(
                        MavioStrings.librariesCountry, selection: $form.metadataCountry,
                        options: [("", MavioStrings.metadataLibraryDefault)]
                            + choices(countries, form.metadataCountry).map { ($0.code, $0.name) })
                }
            }
            if !idKinds.isEmpty {
                Section(MavioStrings.metadataExternalIds) {
                    ForEach(idKinds, id: \.key) { k in
                        text(
                            k.name,
                            Binding(get: { form.externalIds[k.key] ?? "" }, set: { form.externalIds[k.key] = $0 }))
                    }
                }
            }
            Section {
                Toggle(MavioStrings.metadataLockAll, isOn: $form.locked)
                if !form.locked {
                    ForEach(MetadataForm.lockable, id: \.0) { field, label in
                        Toggle(
                            label,
                            isOn: Binding(
                                get: { form.lockedFields.contains(field) },
                                set: { on in
                                    form.lockedFields =
                                        on ? form.lockedFields + [field] : form.lockedFields.filter { $0 != field }
                                }))
                    }
                }
            } header: {
                Text(MavioStrings.metadataLocks)
            } footer: {
                Text(MavioStrings.metadataLocksFooter)
            }
            if let failure { Failure(text: failure) }
        }
    }

    private func text(_ label: String, _ value: Binding<String>, prompt: String? = nil) -> some View {
        LabeledContent(label) {
            TextField(label, text: value, prompt: prompt.map { Text($0) }).multilineTextAlignment(.trailing)
        }
    }

    private func number(_ label: String, _ value: Binding<String>, decimals: Bool = false) -> some View {
        LabeledContent(label) {
            TextField(label, text: value).multilineTextAlignment(.trailing).numberKeyboard(decimals: decimals)
        }
    }

    /// A day: a date picker that can be cleared, or on Apple TV, which has
    /// none, a field of the day's digits.
    @ViewBuilder
    private func day(_ label: String, _ value: Binding<String>) -> some View {
        #if os(tvOS)
            text(label, value, prompt: "YYYY-MM-DD")
        #else
            if let date = MetadataForm.date(value.wrappedValue) {
                HStack {
                    DatePicker(
                        label,
                        selection: Binding(get: { date }, set: { value.wrappedValue = MetadataForm.day($0) }),
                        displayedComponents: .date
                    )
                    .environment(\.timeZone, TimeZone(identifier: "UTC")!)
                    Button {
                        value.wrappedValue = ""
                    } label: {
                        Icon("close", size: 16)
                    }
                    .buttonStyle(.borderless)
                }
            } else {
                LabeledContent(label) {
                    Button("—") { value.wrappedValue = MetadataForm.day(.now) }.accessibilityLabel(label)
                }
            }
        #endif
    }

    private func save() {
        guard let before else { return }
        busy = true
        failure = nil
        Task {
            do {
                try await model.updateItem(serverId, itemId, form, from: before)
                dismiss()
            } catch {
                failure = error.reason
            }
            busy = false
        }
    }
}

// MARK: Identify

/// Searches the providers for what the item is by a name and year, the
/// item's at first, and identifies it as a result.
private struct IdentifyView: View {
    let serverId: String
    let itemId: String
    let name: String
    @Environment(AppModel.self) private var model
    @Environment(\.theme) private var theme
    @Environment(\.dismiss) private var dismiss
    @State private var query = ""
    @State private var year = ""
    @State private var results: [Mavio_Library_V1_RemoteSearchResult]?
    @State private var searching = false
    @State private var busy = false
    @State private var failure: String?

    var body: some View {
        Form {
            Section {
                LabeledContent(MavioStrings.metadataName) {
                    TextField(MavioStrings.metadataName, text: $query).multilineTextAlignment(.trailing)
                        .onSubmit(search)
                }
                LabeledContent(MavioStrings.metadataYear) {
                    TextField(MavioStrings.metadataYear, text: $year).multilineTextAlignment(.trailing)
                        .numberKeyboard()
                }
                Button(MavioStrings.metadataSearch, action: search)
                    .disabled(searching || query.trimmingCharacters(in: .whitespaces).isEmpty)
            }
            Section {
                if searching {
                    Text(MavioStrings.metadataSearching).foregroundStyle(theme.colors.textSecondary)
                } else if let results {
                    if results.isEmpty {
                        Text(MavioStrings.metadataNoResults).foregroundStyle(theme.colors.textSecondary)
                    }
                    ForEach(Array(results.enumerated()), id: \.offset) { _, r in row(r) }
                }
            }
            if let failure { Failure(text: failure) }
        }
        .metadataPage(MavioStrings.metadataIdentifyTitle(name: name))
        .toolbar {
            ToolbarItem(placement: .cancellationAction) {
                Button(MavioStrings.actionCancel) { dismiss() }
            }
        }
        .task {
            guard results == nil else { return }
            if let item = try? await model.item(serverId, itemId) {
                query = item.name
                year = item.productionYear > 0 ? "\(item.productionYear)" : ""
            } else {
                query = name
            }
            search()
        }
    }

    private func row(_ r: Mavio_Library_V1_RemoteSearchResult) -> some View {
        Button {
            identify(r)
        } label: {
            HStack(alignment: .top, spacing: MavioSpacing.s12) {
                ArtworkView(r.imageURL.isEmpty ? nil : r.imageURL)
                    .aspectRatio(2 / 3, contentMode: .fit)
                    .frame(width: 56)
                    .clipShape(RoundedRectangle(cornerRadius: MavioRadius.sm))
                VStack(alignment: .leading, spacing: 2) {
                    Text(r.year > 0 ? "\(r.name) (\(r.year))" : r.name).type(theme.type.headline)
                    Text(remoteResultIds(r)).type(theme.type.footnote).foregroundStyle(theme.colors.textSecondary)
                    if !r.overview.isEmpty {
                        Text(r.overview).type(theme.type.footnote).foregroundStyle(theme.colors.textSecondary)
                            .lineLimit(2)
                    }
                }
                .frame(maxWidth: .infinity, alignment: .leading)
            }
        }
        .disabled(busy)
        .accessibilityHint(MavioStrings.metadataIdentify)
    }

    private func search() {
        let name = query.trimmingCharacters(in: .whitespaces)
        guard !name.isEmpty else { return }
        searching = true
        failure = nil
        Task {
            do {
                results = try await model.searchRemote(serverId, itemId, name: name, year: Int32(year))
            } catch {
                results = []
                failure = error.reason
            }
            searching = false
        }
    }

    private func identify(_ r: Mavio_Library_V1_RemoteSearchResult) {
        busy = true
        failure = nil
        Task {
            do {
                try await model.identify(serverId, itemId, as: r)
                dismiss()
            } catch {
                failure = error.reason
            }
            busy = false
        }
    }
}

// MARK: Refresh

/// Refreshes the item's metadata, filling what is missing or replacing all
/// of it.
private struct RefreshItemView: View {
    let serverId: String
    let itemId: String
    let name: String
    @Environment(AppModel.self) private var model
    @Environment(\.dismiss) private var dismiss
    @State private var replace = false
    @State private var busy = false
    @State private var failure: String?

    var body: some View {
        Form {
            Section {
                Toggle(MavioStrings.metadataReplace, isOn: $replace)
            } footer: {
                Text(replace ? MavioStrings.metadataReplaceFooter : MavioStrings.metadataRefreshFooter)
            }
            if let failure { Failure(text: failure) }
        }
        .metadataPage(MavioStrings.metadataRefreshTitle(name: name))
        .toolbar {
            ToolbarItem(placement: .cancellationAction) {
                Button(MavioStrings.actionCancel) { dismiss() }
            }
            ToolbarItem(placement: .confirmationAction) {
                Button(busy ? MavioStrings.metadataRefreshing : MavioStrings.metadataRefresh) {
                    busy = true
                    failure = nil
                    Task {
                        do {
                            try await model.refreshMetadata(serverId, itemId, replace: replace)
                            dismiss()
                        } catch {
                            failure = error.reason
                        }
                        busy = false
                    }
                }
                .disabled(busy)
            }
        }
    }
}

// MARK: Images

/// The item's images of a kind, each deleted after confirmation, and the
/// providers' images of the kind, one chosen to replace the first; on the
/// iPhone and iPad an image from the photo library is uploaded instead.
private struct ItemImagesView: View {
    let serverId: String
    let itemId: String
    let name: String
    @Environment(AppModel.self) private var model
    @Environment(\.theme) private var theme
    @Environment(\.dismiss) private var dismiss
    @State private var kind = Mavio_Library_V1_ImageKind.primary
    @State private var current: [Mavio_Library_V1_Image] = []
    @State private var remote: [Mavio_Library_V1_RemoteImage]?
    @State private var busy = false
    @State private var failure: String?
    @State private var deleting: Mavio_Library_V1_Image?
    @State private var version = 0
    #if os(iOS)
        @State private var photo: PhotosPickerItem?
    #endif

    var body: some View {
        Form {
            Section {
                Picker(MavioStrings.metadataImageKind, selection: $kind) {
                    ForEach(metadataImageKinds, id: \.0) { Text($0.1).tag($0.0) }
                }
                #if os(iOS)
                    PhotosPicker(MavioStrings.metadataUploadImage, selection: $photo, matching: .images)
                        .disabled(busy)
                #endif
                if busy { ProgressView() }
            }
            if let failure { Failure(text: failure) }
            Section(MavioStrings.metadataCurrent) {
                if current.isEmpty {
                    Text(MavioStrings.metadataNoImage).foregroundStyle(theme.colors.textSecondary)
                } else {
                    grid {
                        ForEach(current, id: \.id) { image in
                            Button {
                                deleting = image
                            } label: {
                                tile(
                                    model.server(serverId)?.imageURL(image.url, maxWidth: 640)?.absoluteString,
                                    caption: image.width > 0 && image.height > 0
                                        ? "\(image.width)×\(image.height)" : MavioStrings.metadataDeleteImage)
                            }
                            .accessibilityLabel(MavioStrings.metadataDeleteImage)
                        }
                    }
                }
            }
            Section(MavioStrings.metadataFromProviders) {
                if let remote {
                    if remote.isEmpty {
                        Text(MavioStrings.metadataNoRemoteImages).foregroundStyle(theme.colors.textSecondary)
                    } else {
                        grid {
                            ForEach(remote, id: \.url) { image in
                                Button {
                                    run { try await model.setItemImage(serverId, itemId, kind, url: image.url) }
                                } label: {
                                    tile(image.url, caption: remoteImageCaption(image))
                                }
                                .disabled(busy)
                            }
                        }
                    }
                } else {
                    Text(MavioStrings.metadataSearching).foregroundStyle(theme.colors.textSecondary)
                }
            }
        }
        .metadataPage(MavioStrings.metadataImagesTitle(name: name))
        .toolbar {
            ToolbarItem(placement: .confirmationAction) {
                Button(MavioStrings.keysDone) { dismiss() }
            }
        }
        .confirmationDialog(
            MavioStrings.metadataDeleteImage,
            isPresented: Binding(get: { deleting != nil }, set: { if !$0 { deleting = nil } }),
            titleVisibility: .visible
        ) {
            Button(MavioStrings.metadataConfirmDelete, role: .destructive) {
                guard let image = deleting else { return }
                run { try await model.deleteItemImage(serverId, itemId, imageId: image.id) }
            }
            Button(MavioStrings.actionCancel, role: .cancel) {}
        }
        .task(id: version) {
            if let item = try? await model.item(serverId, itemId) {
                current = item.images.filter { $0.kind == kind && !$0.url.isEmpty }
            }
        }
        .task(id: kind) {
            remote = nil
            current = []
            version += 1
            do {
                remote = try await model.remoteImages(serverId, itemId, kind)
            } catch {
                remote = []
                failure = error.reason
            }
        }
        #if os(iOS)
            .onChange(of: photo) {
                guard let photo else { return }
                self.photo = nil
                run {
                    guard let data = try await photo.loadTransferable(type: Data.self) else { return }
                    try await model.setItemImage(serverId, itemId, kind, data: data)
                }
            }
        #endif
    }

    private var wide: Bool { wideImages(kind) }

    private func grid(@ViewBuilder _ content: () -> some View) -> some View {
        #if os(tvOS)
            let columns = wide ? 3 : 5
        #else
            let columns = wide ? 2 : 3
        #endif
        return LazyVGrid(
            columns: Array(
                repeating: GridItem(.flexible(), spacing: MavioSpacing.s12, alignment: .top), count: columns),
            spacing: MavioSpacing.s12, content: content
        )
        .padding(.vertical, MavioSpacing.s8)
    }

    private func tile(_ url: String?, caption: String) -> some View {
        VStack(alignment: .leading, spacing: MavioSpacing.s4) {
            ArtworkView(url)
                .aspectRatio(wide ? 16 / 9 : 2 / 3, contentMode: .fit)
                .clipShape(RoundedRectangle(cornerRadius: MavioRadius.sm))
            Text(caption).type(theme.type.caption).foregroundStyle(theme.colors.textSecondary).lineLimit(1)
        }
        .contentShape(Rectangle())
    }

    private func run(_ action: @escaping () async throws -> Void) {
        busy = true
        failure = nil
        Task {
            do {
                try await action()
                version += 1
            } catch {
                failure = error.reason
            }
            busy = false
        }
    }
}
