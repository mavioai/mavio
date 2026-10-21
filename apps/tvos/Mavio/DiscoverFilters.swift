import MavioKit
import MavioUI
import SwiftUI

/// A catalog's filters and sorts as a dialog (docs/tv.md §6.3): its
/// sections scroll above Reset and Apply — the order of its titles, the
/// genres, studios, keywords and streaming services by their values, the
/// original language, and ranges of years, runtime, rating and votes. A
/// long list of values, as the streaming services are, shows its first 30
/// and those picked until Show More. Its values are those of the catalog's
/// kind of title, movies when it mixes them.
struct CatalogFiltersDialog: View {
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
    private var region: String { f.watchRegion.isEmpty ? defaultWatchRegion : f.watchRegion }
    private func takes(_ filter: Mavio_Plugin_V1_DiscoverFilter) -> Bool { catalog.filters.contains(filter) }

    var body: some View {
        VStack(alignment: .leading, spacing: MavioSpacing.s32) {
            Text(MavioStrings.discoverFilters).type(theme.type.title2).foregroundStyle(theme.colors.textPrimary)
            ScrollView {
                VStack(alignment: .leading, spacing: MavioSpacing.s40) {
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
                .padding(MavioSpacing.s24)
                .frame(maxWidth: .infinity, alignment: .leading)
            }
            HStack(spacing: MavioSpacing.s24) {
                Button(MavioStrings.discoverReset) { f = f.unfiltered }
                    .disabled(f == f.unfiltered)
                Spacer()
                Button(MavioStrings.actionCancel) { dismiss() }
                Button(MavioStrings.discoverApply) {
                    apply(f)
                    dismiss()
                }
            }
            .buttonStyle(TVButtonStyle())
            .focusSection()
        }
        .padding(MavioSpacing.s48)
        .frame(width: 1240)
        .frame(maxHeight: 960)
        .background(theme.colors.surface, in: .rect(cornerRadius: 40))
        .frame(maxWidth: .infinity, maxHeight: .infinity)
        .background(theme.colors.scrim)
    }

    private var sortSection: some View {
        FilterSection(title: MavioStrings.discoverSortBy) {
            HStack(spacing: MavioSpacing.s24) {
                ChoiceMenu(
                    title: MavioStrings.discoverSortBy, selection: $f.sort,
                    options: ([.unspecified] + catalog.sorts).map { ($0, discoverSortName($0)) })
                if f.sort != .unspecified {
                    ChoiceMenu(
                        title: MavioStrings.discoverSortBy, selection: $f.ascending,
                        options: [(false, MavioStrings.discoverDescending), (true, MavioStrings.discoverAscending)])
                }
            }
        }
    }

    private var providers: some View {
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
            ChoiceMenu(
                title: MavioStrings.discoverWatchProviders,
                selection: Binding(
                    get: { region },
                    set: {
                        f.watchRegion = $0
                        f.watchProviders = []
                    }),
                options: DiscoverLocales.countries)
        }
    }

    private var language: some View {
        FilterSection(title: MavioStrings.discoverOriginalLanguage) {
            ChoiceMenu(
                title: MavioStrings.discoverOriginalLanguage, selection: $f.originalLanguage,
                options: [("", MavioStrings.discoverAny)] + DiscoverLocales.languages)
        }
    }

    private var rating: some View {
        FilterSection(title: MavioStrings.discoverRatingMin) {
            ChoiceMenu(
                title: MavioStrings.discoverRatingMin, selection: $f.ratingMin,
                options: (0..<10).map { ($0, $0 == 0 ? MavioStrings.discoverAny : $0.formatted()) })
        }
    }

    private func range(_ title: String, _ from: Binding<Int>, _ to: Binding<Int>, max: Int) -> some View {
        FilterSection(title: title) {
            HStack(spacing: MavioSpacing.s16) {
                NumberField(label: MavioStrings.discoverFrom, value: from, max: max)
                Text(verbatim: "–").foregroundStyle(theme.colors.textSecondary)
                NumberField(label: MavioStrings.discoverTo, value: to, max: max)
            }
        }
    }
}

/// A heading over a filter's controls, which are a section of focus of
/// their own.
private struct FilterSection<Content: View>: View {
    let title: String
    @ViewBuilder let content: Content
    @Environment(\.theme) private var theme

    var body: some View {
        VStack(alignment: .leading, spacing: MavioSpacing.s16) {
            Text(title).type(theme.type.headline).foregroundStyle(theme.colors.textSecondary)
            content
        }
        .frame(maxWidth: .infinity, alignment: .leading)
        .focusSection()
    }
}

/// A menu choosing one of its options, showing the chosen one, as the
/// library's order.
private struct ChoiceMenu<Value: Hashable>: View {
    let title: String
    @Binding var selection: Value
    let options: [(Value, String)]

    var body: some View {
        Menu {
            Picker(title, selection: $selection) {
                ForEach(options, id: \.0) { Text($0.1).tag($0.0) }
            }
        } label: {
            HStack(spacing: MavioSpacing.s12) {
                Text(options.first { $0.0 == selection }?.1 ?? "").lineLimit(1)
                Icon("selector", size: 24)
            }
        }
        .buttonStyle(TVButtonStyle())
    }
}

/// A whole number; empty for 0, any. Select opens the keyboard.
private struct NumberField: View {
    let label: String
    @Binding var value: Int
    let max: Int

    var body: some View {
        TextField(
            label,
            text: Binding(
                get: { value > 0 ? String(value) : "" },
                set: { value = Swift.min(Int($0.filter(\.isNumber).prefix(7)) ?? 0, max) })
        )
        .keyboardType(.numberPad)
        .frame(width: 280)
    }
}

/// A value to toggle, with its logo when it has one: filled with
/// `color.button` while picked, white with focus.
private struct Chip: View {
    let label: String
    var image: String?
    let on: Bool
    var removable = false
    let toggle: () -> Void

    var body: some View {
        Button(action: toggle) {
            HStack(spacing: MavioSpacing.s8) {
                if let image {
                    ArtworkView(image, title: label)
                        .frame(width: 32, height: 32)
                        .clipShape(.rect(cornerRadius: 6))
                }
                Text(label).lineLimit(1)
                if removable { Icon("close", size: 20) }
            }
        }
        .buttonStyle(ValueChipStyle(on: on))
        .accessibilityAddTraits(on ? .isSelected : [])
    }
}

private struct ValueChipStyle: ButtonStyle {
    let on: Bool

    func makeBody(configuration: Configuration) -> some View {
        ChipFocus(label: configuration.label, on: on)
    }
}

private struct ChipFocus<Label: View>: View {
    let label: Label
    let on: Bool
    @Environment(\.isFocused) private var focused
    @Environment(\.theme) private var theme

    var body: some View {
        label
            .type(theme.type.callout).fontWeight(.semibold)
            .foregroundStyle(focused ? .black : on ? theme.colors.onButton : theme.colors.textPrimary)
            .padding(.horizontal, MavioSpacing.s20)
            .frame(minHeight: 52)
            .background {
                Capsule().fill(focused ? .white : on ? theme.colors.button : theme.colors.fill)
                    .shadow(color: focused ? theme.colors.tvFocusShadow : .clear, radius: 12, y: 8)
            }
            .scaleEffect(focused ? theme.layout.focusScale : 1)
            .animation(focusAnimation, value: focused)
    }
}

/// Values listed whole, as genres and streaming services are, to pick
/// from: the first 30 and those picked until Show More. With a control
/// choosing them, as the country of streaming services, the section shows
/// even when none is listed.
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
    @State private var all = false

    var body: some View {
        if values?.isEmpty != true || Control.self != EmptyView.self {
            FilterSection(title: title) {
                control
                if let values {
                    let shown =
                        all
                        ? values
                        : values.enumerated().filter { $0.offset < 30 || value.contains($0.element.id) }.map(\.element)
                    Flow(spacing: MavioSpacing.s16, lineSpacing: MavioSpacing.s16) {
                        ForEach(shown, id: \.id) { v in
                            let on = value.contains(v.id)
                            Chip(label: v.name, image: v.imageURL.isEmpty ? nil : v.imageURL, on: on) {
                                value = on ? value.filter { $0 != v.id } : value + [v.id]
                            }
                        }
                        if shown.count < values.count {
                            Chip(label: MavioStrings.discoverShowMore, on: false) { all = true }
                        }
                    }
                } else {
                    ProgressView()
                }
            }
            .task(id: region) {
                values = nil
                all = false
                values =
                    (try? await model.filterValues(
                        serverId: serverId, catalog: catalog, filter: filter, mediaKind: mediaKind, region: region))
                    ?? []
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
    @State private var text = ""
    @State private var found: [Mavio_Plugin_V1_FilterValue] = []

    var body: some View {
        FilterSection(title: title) {
            if !value.isEmpty {
                Flow(spacing: MavioSpacing.s16, lineSpacing: MavioSpacing.s16) {
                    ForEach(value, id: \.id) { v in
                        Chip(label: v.name, on: true, removable: true) {
                            value.removeAll { $0.id == v.id }
                        }
                    }
                }
            }
            if searchable {
                TextField(MavioStrings.discoverSearchValues(filter: title), text: $text)
                    .autocorrectionDisabled()
                    .textInputAutocapitalization(.never)
                    .frame(width: 640)
                let chosen = Set(value.map(\.id))
                let shown = found.filter { !chosen.contains($0.id) }.prefix(20)
                if !shown.isEmpty {
                    Flow(spacing: MavioSpacing.s16, lineSpacing: MavioSpacing.s16) {
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
                found = try await model.filterValues(
                    serverId: serverId, catalog: catalog, filter: filter, mediaKind: mediaKind, query: query)
            } catch is CancellationError {
            } catch {
                found = []
            }
        }
    }
}
