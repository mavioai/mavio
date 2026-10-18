import MavioKit
import MavioUI
import SwiftUI

/// The country of the streaming services until another is chosen, the one
/// with the most of them.
private let defaultRegion = "US"

/// A catalog's filters and sorts (docs/clients.md §6.3.28), as the web UI's
/// dialog: the order of its titles and the filters it takes — genres,
/// studios, keywords and streaming services by their values, the original
/// language, and ranges of years, runtime, rating and votes. Its values are
/// those of the catalog's kind of title, movies when it mixes them.
struct CatalogFiltersSheet: View {
    let catalog: Catalog
    let apply: (CatalogQuery) -> Void
    @Environment(\.theme) private var theme
    @Environment(\.dismiss) private var dismiss
    @State private var f: CatalogQuery

    init(catalog: Catalog, value: CatalogQuery, apply: @escaping (CatalogQuery) -> Void) {
        self.catalog = catalog
        self.apply = apply
        _f = State(initialValue: value)
    }

    private var mediaKind: MediaKind { f.kind == .unspecified ? .movie : f.kind }
    private var region: String { f.watchRegion.isEmpty ? defaultRegion : f.watchRegion }
    private func takes(_ filter: Mavio_Plugin_V1_DiscoverFilter) -> Bool { catalog.filters.contains(filter) }

    var body: some View {
        NavigationStack {
            ScrollView {
                VStack(alignment: .leading, spacing: MavioSpacing.s24) {
                    if !catalog.sorts.isEmpty { sortSection }
                    if takes(.genre) {
                        ListedValues(
                            serverId: f.serverId, catalog: catalog.id, filter: .genre, mediaKind: mediaKind,
                            title: MavioStrings.discoverGenres, value: $f.genres)
                    }
                    if takes(.studio), mediaKind == .movie {
                        SearchedValues(
                            serverId: f.serverId, catalog: catalog.id, filter: .studio, mediaKind: mediaKind,
                            title: MavioStrings.discoverStudios, value: $f.studios)
                    }
                    if mediaKind == .series, !f.networks.isEmpty {
                        SearchedValues(
                            serverId: f.serverId, catalog: catalog.id, filter: .network, mediaKind: mediaKind,
                            title: MavioStrings.discoverNetworks, value: $f.networks, searchable: false)
                    }
                    if takes(.keyword) {
                        SearchedValues(
                            serverId: f.serverId, catalog: catalog.id, filter: .keyword, mediaKind: mediaKind,
                            title: MavioStrings.discoverKeywords, value: $f.keywords)
                    }
                    if takes(.watchProvider) { providers }
                    if takes(.originalLanguage) { language }
                    if takes(.releaseDate) {
                        range(
                            MavioStrings.discoverReleased, $f.yearFrom, $f.yearTo,
                            max: Calendar.current.component(.year, from: .now) + 5)
                    }
                    if takes(.runtime) { range(MavioStrings.discoverRuntime, $f.runtimeMin, $f.runtimeMax, max: 1000) }
                    if takes(.rating) { rating }
                    if takes(.voteCount) {
                        FilterSection(title: MavioStrings.discoverVotesMin) {
                            NumberField(label: MavioStrings.discoverAny, value: $f.votesMin, max: 1_000_000)
                        }
                    }
                }
                .padding(theme.layout.margin)
            }
            .scrollDismissesKeyboard(.interactively)
            .background(theme.colors.background)
            .navigationTitle(MavioStrings.discoverFilters)
            .navigationBarTitleDisplayMode(.inline)
            .toolbar {
                ToolbarItem(placement: .cancellationAction) {
                    Button(MavioStrings.actionCancel) { dismiss() }
                }
                ToolbarItem(placement: .confirmationAction) {
                    Button(MavioStrings.discoverApply) {
                        apply(f)
                        dismiss()
                    }
                }
                ToolbarItem(placement: .bottomBar) {
                    Button(MavioStrings.discoverReset) { f = f.unfiltered }
                        .disabled(f == f.unfiltered)
                }
            }
        }
        .presentationDetents([.large])
        .presentationBackground(theme.colors.background)
    }

    private var sortSection: some View {
        FilterSection(title: MavioStrings.discoverSortBy) {
            HStack(spacing: MavioSpacing.s8) {
                MenuPicker(
                    selection: $f.sort,
                    options: ([.unspecified] + catalog.sorts).map { ($0, sortName($0)) })
                if f.sort != .unspecified {
                    MenuPicker(
                        selection: $f.ascending,
                        options: [(false, MavioStrings.discoverDescending), (true, MavioStrings.discoverAscending)])
                }
            }
        }
    }

    private var providers: some View {
        VStack(alignment: .leading, spacing: MavioSpacing.s8) {
            ListedValues(
                serverId: f.serverId, catalog: catalog.id, filter: .watchProvider, mediaKind: mediaKind,
                region: region, title: MavioStrings.discoverWatchProviders,
                value: Binding(
                    get: { f.watchProviders },
                    set: {
                        f.watchProviders = $0
                        f.watchRegion = region
                    })
            ) {
                MenuPicker(
                    selection: Binding(
                        get: { region },
                        set: {
                            f.watchRegion = $0
                            f.watchProviders = []
                        }),
                    options: Localized.countries)
            }
        }
    }

    private var language: some View {
        FilterSection(title: MavioStrings.discoverOriginalLanguage) {
            MenuPicker(selection: $f.originalLanguage, options: [("", MavioStrings.discoverAny)] + Localized.languages)
        }
    }

    private var rating: some View {
        FilterSection(title: MavioStrings.discoverRatingMin) {
            MenuPicker(
                selection: $f.ratingMin,
                options: (0..<10).map { ($0, $0 == 0 ? MavioStrings.discoverAny : $0.formatted()) })
        }
    }

    private func range(_ title: String, _ from: Binding<Int>, _ to: Binding<Int>, max: Int) -> some View {
        FilterSection(title: title) {
            HStack(spacing: MavioSpacing.s8) {
                NumberField(label: MavioStrings.discoverFrom, value: from, max: max)
                Text("–").foregroundStyle(theme.colors.textSecondary)
                NumberField(label: MavioStrings.discoverTo, value: to, max: max)
            }
        }
    }

    private func sortName(_ sort: Mavio_Plugin_V1_DiscoverSort) -> String {
        switch sort {
        case .popularity: MavioStrings.discoverSortPopularity
        case .releaseDate: MavioStrings.discoverSortReleaseDate
        case .rating: MavioStrings.discoverSortRating
        case .voteCount: MavioStrings.discoverSortVotes
        case .title: MavioStrings.discoverSortTitle
        default: MavioStrings.discoverSortDefault
        }
    }
}

/// The languages and countries to choose from, by their codes, named in the
/// app's language.
private enum Localized {
    private static let locale = Locale(identifier: MavioStrings.bundle.preferredLocalizations.first ?? "en")

    /// ISO 639-1 languages.
    static let languages: [(String, String)] = Locale.LanguageCode.isoLanguageCodes
        .map(\.identifier)
        .filter { $0.count == 2 }
        .compactMap { code in locale.localizedString(forLanguageCode: code).map { (code, $0) } }
        .sorted { $0.1.localizedCompare($1.1) == .orderedAscending }

    /// ISO 3166-1 countries.
    static let countries: [(String, String)] = Locale.Region.isoRegions
        .map(\.identifier)
        .filter { $0.count == 2 && $0.allSatisfy(\.isLetter) }
        .compactMap { code in locale.localizedString(forRegionCode: code).map { (code, $0) } }
        .sorted { $0.1.localizedCompare($1.1) == .orderedAscending }
}

/// A heading over a filter's controls.
private struct FilterSection<Content: View>: View {
    let title: String
    @ViewBuilder let content: Content
    @Environment(\.theme) private var theme

    var body: some View {
        VStack(alignment: .leading, spacing: MavioSpacing.s8) {
            Text(title).type(theme.type.headline)
            content
        }
        .frame(maxWidth: .infinity, alignment: .leading)
    }
}

/// A menu choosing one of its options, showing the chosen one.
private struct MenuPicker<Value: Hashable>: View {
    @Binding var selection: Value
    let options: [(Value, String)]
    @Environment(\.theme) private var theme

    var body: some View {
        Menu {
            Picker("", selection: $selection) {
                ForEach(options, id: \.0) { Text($0.1).tag($0.0) }
            }
        } label: {
            HStack(spacing: 6) {
                Text(options.first { $0.0 == selection }?.1 ?? "").lineLimit(1)
                Icon("selector", size: 14).foregroundStyle(theme.colors.textSecondary)
            }
            .type(theme.type.callout)
            .foregroundStyle(theme.colors.textPrimary)
            .padding(.horizontal, 14)
            .frame(height: 36)
            .background(theme.colors.fill, in: .capsule)
        }
    }
}

/// A whole number; empty for 0, any.
private struct NumberField: View {
    let label: String
    @Binding var value: Int
    let max: Int
    @Environment(\.theme) private var theme

    var body: some View {
        TextField(
            label,
            text: Binding(
                get: { value > 0 ? String(value) : "" },
                set: { value = Swift.min(Int($0.filter(\.isNumber).prefix(7)) ?? 0, max) })
        )
        .keyboardType(.numberPad)
        .type(theme.type.body)
        .padding(.horizontal, 14)
        .frame(width: 120, height: 40)
        .background(theme.colors.fill, in: .rect(cornerRadius: 10))
    }
}

/// A value to toggle, with its logo when it has one.
private struct Chip: View {
    let label: String
    var image: String?
    let on: Bool
    var removable = false
    let toggle: () -> Void
    @Environment(\.theme) private var theme

    var body: some View {
        Button(action: toggle) {
            HStack(spacing: 6) {
                if let image {
                    ArtworkView(image, title: label)
                        .frame(width: 20, height: 20)
                        .clipShape(.rect(cornerRadius: 4))
                }
                Text(label).lineLimit(1)
                if removable { Icon("close", size: 14) }
            }
            .type(theme.type.footnote).fontWeight(.semibold)
            .foregroundStyle(on ? theme.colors.onButton : theme.colors.textPrimary)
            .padding(.horizontal, 12)
            .frame(minHeight: 32)
            .background(on ? theme.colors.button : theme.colors.fill, in: .capsule)
        }
        .buttonStyle(.plain)
        .accessibilityAddTraits(on ? .isSelected : [])
    }
}

/// Asks a catalog's provider for a filter's values.
@MainActor
private func filterValues(
    _ model: AppModel, serverId: String, catalog: String, filter: Mavio_Plugin_V1_DiscoverFilter,
    mediaKind: MediaKind, query: String = "", region: String = ""
) async throws -> [Mavio_Plugin_V1_FilterValue] {
    var r = Mavio_Acquisition_V1_ListFilterValuesRequest()
    r.catalog = catalog
    r.filter = filter
    r.mediaKind = mediaKind
    r.query = query
    r.language = discoverLanguage
    r.region = region
    let response: Mavio_Acquisition_V1_ListFilterValuesResponse = try await model.client.call(
        serverId, Procedure.discover("ListFilterValues"), r)
    return response.values
}

/// Values listed whole, as genres and streaming services are, to pick
/// from; with a control choosing them, as the country of streaming
/// services, the section shows even when none is listed.
private struct ListedValues<Control: View>: View {
    let serverId: String
    let catalog: String
    let filter: Mavio_Plugin_V1_DiscoverFilter
    let mediaKind: MediaKind
    var region = ""
    let title: String
    @Binding var value: [String]
    @ViewBuilder var control: Control
    @Environment(AppModel.self) private var model
    @State private var values: [Mavio_Plugin_V1_FilterValue]?

    var body: some View {
        if values?.isEmpty != true || Control.self != EmptyView.self {
            FilterSection(title: title) {
                control
                if let values {
                    let chips = Flow(spacing: MavioSpacing.s8, lineSpacing: MavioSpacing.s8) {
                        ForEach(values, id: \.id) { v in
                            let on = value.contains(v.id)
                            Chip(label: v.name, image: v.imageURL.isEmpty ? nil : v.imageURL, on: on) {
                                value = on ? value.filter { $0 != v.id } : value + [v.id]
                            }
                        }
                    }
                    // Long lists, as streaming services, scroll in a box.
                    if values.count > 30 {
                        ScrollView { chips.frame(maxWidth: .infinity, alignment: .leading) }
                            .frame(height: 200)
                    } else {
                        chips
                    }
                } else {
                    ProgressView()
                }
            }
            .task(id: region) {
                values = nil
                values =
                    (try? await filterValues(
                        model, serverId: serverId, catalog: catalog, filter: filter, mediaKind: mediaKind,
                        region: region)) ?? []
            }
        }
    }
}

extension ListedValues where Control == EmptyView {
    init(
        serverId: String, catalog: String, filter: Mavio_Plugin_V1_DiscoverFilter, mediaKind: MediaKind,
        title: String, value: Binding<[String]>
    ) {
        self.init(
            serverId: serverId, catalog: catalog, filter: filter, mediaKind: mediaKind, title: title, value: value
        ) { EmptyView() }
    }
}

/// Values found by name, as studios and keywords are: the chosen ones, a
/// field, and what it finds.
private struct SearchedValues: View {
    let serverId: String
    let catalog: String
    let filter: Mavio_Plugin_V1_DiscoverFilter
    let mediaKind: MediaKind
    let title: String
    @Binding var value: [NamedValue]
    /// False for values looked up by ID only, as networks are: just the
    /// chosen ones.
    var searchable = true
    @Environment(AppModel.self) private var model
    @Environment(\.theme) private var theme
    @State private var text = ""
    @State private var found: [Mavio_Plugin_V1_FilterValue] = []

    var body: some View {
        FilterSection(title: title) {
            if !value.isEmpty {
                Flow(spacing: MavioSpacing.s8, lineSpacing: MavioSpacing.s8) {
                    ForEach(value, id: \.id) { v in
                        Chip(label: v.name, on: true, removable: true) {
                            value.removeAll { $0.id == v.id }
                        }
                    }
                }
            }
            if searchable {
                TextField(MavioStrings.discoverSearchValues(filter: title), text: $text)
                    .textInputAutocapitalization(.never)
                    .autocorrectionDisabled()
                    .type(theme.type.body)
                    .padding(.horizontal, 14)
                    .frame(height: 40)
                    .background(theme.colors.fill, in: .rect(cornerRadius: 10))
                let chosen = Set(value.map(\.id))
                let shown = found.filter { !chosen.contains($0.id) }.prefix(20)
                if !shown.isEmpty {
                    Flow(spacing: MavioSpacing.s8, lineSpacing: MavioSpacing.s8) {
                        ForEach(Array(shown), id: \.id) { v in
                            Chip(label: v.name, on: false) {
                                value.append(NamedValue(id: v.id, name: v.name))
                                text = ""
                            }
                        }
                    }
                }
            }
        }
        .task(id: text) {
            let query = text.trimmingCharacters(in: .whitespaces)
            guard !query.isEmpty else {
                found = []
                return
            }
            do {
                try await Task.sleep(for: .milliseconds(300))
                found = try await filterValues(
                    model, serverId: serverId, catalog: catalog, filter: filter, mediaKind: mediaKind, query: query)
            } catch is CancellationError {
            } catch {
                found = []
            }
        }
    }
}
