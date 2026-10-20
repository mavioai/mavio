import MavioKit
import MavioUI
import SwiftUI

/// A discovered person's page (docs/clients.md §6.3.2), as Seerr's, opened
/// from the cast and crew of a discovered title: their photo, dates, other
/// names, links and biography, then the movies and series they play in and
/// work on, the department they are known for first.
struct DiscoveredPersonView: View {
    let serverId: String
    let tmdbId: Int32
    @Environment(AppModel.self) private var model
    @Environment(\.theme) private var theme
    @Environment(\.horizontalSizeClass) private var sizeClass
    @State private var person: Mavio_Acquisition_V1_Person?
    @State private var failure: String?
    @State private var more = false
    @State private var kinds = Kinds.all

    private enum Kinds: Hashable { case all, movies, series }

    var body: some View {
        Group {
            if let person {
                ScrollView {
                    VStack(alignment: .leading, spacing: 0) {
                        header(person)
                        let cast = section(MavioStrings.personAppearances, person.cast)
                        let crew = section(MavioStrings.personCrew, person.crew)
                        if person.knownForDepartment == "Acting" || person.knownForDepartment.isEmpty {
                            cast
                            crew
                        } else {
                            crew
                            cast
                        }
                    }
                    .padding(.bottom, MavioSpacing.s40)
                }
                .pullToRefresh { await load() }
            } else if let failure {
                StatusView(
                    icon: "error", title: MavioStrings.stateError, message: failure,
                    action: (MavioStrings.actionRetry, { Task { await load() } }))
            } else {
                ProgressView().frame(maxWidth: .infinity, maxHeight: .infinity)
            }
        }
        .navigationTitle(person?.name ?? "")
        .inlineTitle()
        .task { await load() }
    }

    private func header(_ p: Mavio_Acquisition_V1_Person) -> some View {
        let phone = sizeClass != .regular
        let layout =
            phone
            ? AnyLayout(VStackLayout(alignment: .center, spacing: MavioSpacing.s24))
            : AnyLayout(HStackLayout(alignment: .top, spacing: MavioSpacing.s24))
        let facts = [
            dayText(p.birthday).map { MavioStrings.personBorn(date: $0, age: String(age(p.birthday, p.deathday))) },
            p.placeOfBirth.isEmpty ? nil : p.placeOfBirth,
            dayText(p.deathday).map { MavioStrings.personDied(date: $0) },
        ].compactMap(\.self)
        var links: [ExternalLink] = []
        if let id = p.externalIds["tmdb"], let url = URL(string: "https://www.themoviedb.org/person/\(id)") {
            links.append(ExternalLink(name: "TMDB", url: url, logo: "tmdb", width: 32))
        }
        if let id = p.externalIds["imdb"], let url = URL(string: "https://www.imdb.com/name/\(id)") {
            links.append(ExternalLink(name: "IMDb", url: url, logo: "imdb", width: 32))
        }
        return layout {
            ArtworkView(sized(p.imageURL, 342), title: p.name, icon: "profile")
                .frame(width: phone ? 144 : 176, height: phone ? 144 : 176)
                .clipShape(.circle)
            VStack(alignment: phone ? .center : .leading, spacing: MavioSpacing.s12) {
                Text(p.name).type(theme.type.title1)
                if !facts.isEmpty {
                    Text(facts.joined(separator: " | ")).type(theme.type.callout)
                        .foregroundStyle(theme.colors.textSecondary)
                }
                if !p.alsoKnownAs.isEmpty {
                    Text(MavioStrings.personAlsoKnownAs(names: p.alsoKnownAs.joined(separator: ", ")))
                        .type(theme.type.callout).foregroundStyle(theme.colors.textSecondary)
                }
                if !links.isEmpty { ExternalLinks(links: links, alignment: phone ? .center : .leading) }
                if !p.biography.isEmpty {
                    VStack(alignment: .leading, spacing: MavioSpacing.s4) {
                        Text(p.biography).type(theme.type.body).lineLimit(more ? nil : 6)
                            .multilineTextAlignment(.leading)
                        Button(more ? MavioStrings.personLess : MavioStrings.personMore) {
                            withAnimation { more.toggle() }
                        }
                        .type(theme.type.callout).fontWeight(.semibold)
                        .foregroundStyle(theme.colors.accentText)
                        .buttonStyle(.plain)
                    }
                    .frame(maxWidth: .infinity, alignment: .leading)
                }
                Picker(MavioStrings.personKinds, selection: $kinds) {
                    Text(MavioStrings.personAllKinds).tag(Kinds.all)
                    Text(MavioStrings.personMovies).tag(Kinds.movies)
                    Text(MavioStrings.personSeries).tag(Kinds.series)
                }
                .pickerStyle(.segmented)
                .fixedSize()
            }
            .multilineTextAlignment(phone ? .center : .leading)
            .frame(maxWidth: 768, alignment: .leading)
        }
        .frame(maxWidth: .infinity, alignment: phone ? .center : .leading)
        .padding(.horizontal, theme.layout.margin)
        .padding(.top, MavioSpacing.s24)
    }

    @ViewBuilder private func section(_ title: String, _ all: [Mavio_Acquisition_V1_PersonTitle]) -> some View {
        let shown = all.filter {
            kinds == .all || ($0.title.mediaKind == .series) == (kinds == .series)
        }
        if !shown.isEmpty {
            VStack(alignment: .leading, spacing: MavioSpacing.s12) {
                HStack(alignment: .firstTextBaseline, spacing: MavioSpacing.s8) {
                    Text(title).type(theme.type.title3)
                    Text(String(shown.count)).type(theme.type.callout).foregroundStyle(theme.colors.textSecondary)
                }
                .padding(.horizontal, theme.layout.margin)
                CardGrid(items: Array(shown.indices)) { i in
                    DiscoveredTitleCard(serverId: serverId, title: shown[i].title, role: shown[i].role)
                }
            }
            .padding(.top, MavioSpacing.s32)
        }
    }

    private func load() async {
        var r = Mavio_Acquisition_V1_GetPersonRequest()
        r.tmdbID = tmdbId
        r.language = discoverLanguage
        do {
            person =
                try await model.client.call(
                    serverId, Procedure.discover("GetPerson"), r, as: Mavio_Acquisition_V1_GetPersonResponse.self
                ).person
            failure = nil
        } catch is CancellationError {
        } catch {
            if person == nil { failure = error.reason }
        }
    }

    /// Whole years between two calendar dates, the second today when empty.
    private func age(_ from: String, _ to: String) -> Int {
        let parse = { (s: String) in try? Date(s + "T00:00:00Z", strategy: .iso8601) }
        guard let a = parse(from) else { return 0 }
        var utc = Calendar(identifier: .gregorian)
        utc.timeZone = .gmt
        return utc.dateComponents([.year], from: a, to: parse(to) ?? .now).year ?? 0
    }
}

/// A discovered collection's page (docs/clients.md §6.3.2), as Seerr's,
/// without requests: its backdrop, name, number of movies, what the server
/// has of them and overview, then its movies in release order.
struct DiscoveredCollectionView: View {
    let serverId: String
    let tmdbId: Int32
    @Environment(AppModel.self) private var model
    @Environment(\.theme) private var theme
    @Environment(\.horizontalSizeClass) private var sizeClass
    @State private var collection: Mavio_Acquisition_V1_Collection?
    @State private var failure: String?

    private var phone: Bool { sizeClass != .regular }

    var body: some View {
        Group {
            if let c = collection {
                ScrollView {
                    VStack(alignment: .leading, spacing: 0) {
                        header(c)
                        if !c.overview.isEmpty {
                            Text(c.overview).type(theme.type.body)
                                .padding(.horizontal, theme.layout.margin)
                                .padding(.top, MavioSpacing.s24)
                        }
                        CardGrid(items: c.parts.map(\.key)) { key in
                            if let part = c.parts.first(where: { $0.key == key }) {
                                DiscoveredTitleCard(serverId: serverId, title: part)
                            }
                        }
                        .padding(.top, MavioSpacing.s24)
                    }
                    .padding(.bottom, MavioSpacing.s40)
                }
                .pullToRefresh { await load() }
                .background(theme.colors.background)
                .navigationTitle(c.name)
                .modifier(HeroChrome(title: c.name, phone: phone))
            } else if let failure {
                StatusView(
                    icon: "error", title: MavioStrings.stateError, message: failure,
                    action: (MavioStrings.actionRetry, { Task { await load() } }))
            } else {
                ProgressView().frame(maxWidth: .infinity, maxHeight: .infinity)
            }
        }
        .task(id: model.revision(serverId)) { await load() }
    }

    private func header(_ c: Mavio_Acquisition_V1_Collection) -> some View {
        let artwork = Group {
            if let backdrop = sized(c.backdropURL, 1280) {
                ArtworkView(backdrop, icon: "collections")
            } else {
                theme.colors.surface
            }
        }
        let facts = [MavioStrings.collectionMovieCount(count: c.parts.count), statusLabel(status(c.parts))]
            .compactMap(\.self)
        return Color.clear
            .frame(maxWidth: .infinity)
            .modifier(HeaderSize(phone: phone))
            .background { artwork }
            .overlay {
                LinearGradient(
                    stops: [
                        .init(color: .black, location: 0), .init(color: .black.opacity(0.5), location: 0.5),
                        .init(color: .clear, location: 1),
                    ],
                    startPoint: .bottom, endPoint: .top)
            }
            .overlay(alignment: .bottomLeading) {
                HStack(alignment: .bottom, spacing: MavioSpacing.s40) {
                    if !phone, let poster = sized(c.posterURL, 500) {
                        ArtworkView(poster, title: c.name)
                            .aspectRatio(2 / 3, contentMode: .fit)
                            .frame(width: 200)
                            .clipShape(.rect(cornerRadius: MavioRadius.lg))
                            .shadow(color: .black.opacity(0.36), radius: 16, y: 12)
                    }
                    VStack(alignment: .leading, spacing: MavioSpacing.s12) {
                        Text(c.name).font(.system(size: phone ? 34 : 52, weight: .bold))
                        Text(facts.joined(separator: " · ")).type(theme.type.callout).foregroundStyle(
                            .white.opacity(0.8))
                    }
                    .foregroundStyle(.white)
                    .frame(maxWidth: 640, alignment: .leading)
                }
                .padding(.horizontal, phone ? theme.layout.margin : MavioSpacing.s40)
                .padding(.bottom, phone ? MavioSpacing.s24 : MavioSpacing.s40)
            }
            .clipped()
            .heroBleed(phone: phone, aspect: 1) { artwork }
            .environment(\.colorScheme, .dark)
    }

    /// What the server has of a collection, as Seerr marks it: available
    /// when it has every movie, partially available when some, requested
    /// when every other movie is requested, blocklisted when every movie is.
    private func status(_ parts: [DiscoveredTitle]) -> MediaStatus {
        guard !parts.isEmpty else { return .unspecified }
        if parts.allSatisfy({ $0.status == .blocklisted }) { return .blocklisted }
        if parts.allSatisfy({ $0.status == .available }) { return .available }
        if parts.contains(where: { $0.status == .available || $0.status == .partiallyAvailable }) {
            return .partiallyAvailable
        }
        if parts.allSatisfy({ $0.status == .requested || $0.status == .wanted }) { return .requested }
        return .unspecified
    }

    private func load() async {
        do {
            collection = try await model.collection(serverId, tmdbId)
            failure = nil
        } catch is CancellationError {
        } catch {
            if collection == nil { failure = error.reason }
        }
    }
}
