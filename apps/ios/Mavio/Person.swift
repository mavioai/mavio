import MavioKit
import MavioUI
import SwiftUI

/// A person's page (docs/clients.md §6.3.19), as the web UI's, opened from
/// the cast and crew of an item's details: their photo, dates, biography
/// and links, and the movies, series and episodes of the server crediting
/// them.
struct PersonView: View {
    let serverId: String
    let personId: String
    @Environment(AppModel.self) private var model
    @Environment(\.theme) private var theme
    @State private var person: PersonDetails?
    @State private var failure: String?

    var body: some View {
        Group {
            if let person {
                ScrollView {
                    VStack(alignment: .leading, spacing: 0) {
                        PersonHeader(person: person)
                        titles(MavioStrings.personMovies, person.items.filter { $0.kind == .movie })
                        titles(MavioStrings.personSeries, person.items.filter { $0.kind == .series })
                        titles(MavioStrings.personEpisodes, person.items.filter { $0.kind == .episode }, wide: true)
                        if person.items.isEmpty {
                            Text(MavioStrings.personNone).type(theme.type.body)
                                .foregroundStyle(theme.colors.textSecondary)
                                .padding(.horizontal, theme.layout.margin)
                                .padding(.top, MavioSpacing.s32)
                        }
                    }
                    .padding(.bottom, MavioSpacing.s40)
                }
                .refreshable { await load() }
            } else if let failure {
                StatusView(
                    icon: "error", title: MavioStrings.stateError, message: failure,
                    action: (MavioStrings.actionRetry, { Task { await load() } }))
            } else {
                ProgressView().frame(maxWidth: .infinity, maxHeight: .infinity)
            }
        }
        .navigationTitle(person?.name ?? "")
        .navigationBarTitleDisplayMode(.inline)
        .task(id: model.revision(serverId)) { await load() }
    }

    /// A heading with its count over the cards of one kind.
    @ViewBuilder private func titles(_ title: String, _ cards: [Card], wide: Bool = false) -> some View {
        if !cards.isEmpty {
            VStack(alignment: .leading, spacing: MavioSpacing.s12) {
                HStack(alignment: .firstTextBaseline, spacing: MavioSpacing.s8) {
                    Text(title).type(theme.type.title3)
                    Text(String(cards.count)).type(theme.type.callout).foregroundStyle(theme.colors.textSecondary)
                }
                .padding(.horizontal, theme.layout.margin)
                CardGrid(items: cards, wide: wide) { CardLink(card: $0, wide: wide) }
            }
            .padding(.top, MavioSpacing.s32)
        }
    }

    private func load() async {
        do {
            person = try await model.client.person(serverId: serverId, personId: personId)
            failure = nil
        } catch is CancellationError {
        } catch {
            if person == nil { failure = error.localizedDescription }
        }
    }
}

/// A person's photo, name, dates, biography and links; beside each other on
/// an iPad, one above the other on a phone.
private struct PersonHeader: View {
    let person: PersonDetails
    @Environment(\.theme) private var theme
    @Environment(\.horizontalSizeClass) private var sizeClass
    @State private var more = false

    private var facts: [String] {
        let day = { (d: Date) in d.formatted(date: .long, time: .omitted) }
        return [
            person.born.map {
                MavioStrings.personBorn(date: day($0), age: String(years(from: $0, to: person.died ?? .now)))
            },
            person.birthPlace,
            person.died.map { MavioStrings.personDied(date: day($0)) },
        ].compactMap(\.self)
    }

    var body: some View {
        let phone = sizeClass != .regular
        let layout =
            phone
            ? AnyLayout(VStackLayout(alignment: .leading, spacing: MavioSpacing.s24))
            : AnyLayout(HStackLayout(alignment: .top, spacing: MavioSpacing.s24))
        layout {
            ArtworkView(person.photo?.url, title: person.name, icon: "profile")
                .aspectRatio(2 / 3, contentMode: .fit)
                .frame(width: phone ? 160 : 208)
                .clipShape(.rect(cornerRadius: 14))
            VStack(alignment: .leading, spacing: MavioSpacing.s12) {
                Text(person.name).type(theme.type.title1)
                ForEach(facts, id: \.self) { fact in
                    Text(fact).type(theme.type.callout).foregroundStyle(theme.colors.textSecondary)
                }
                if let overview = person.overview {
                    VStack(alignment: .leading, spacing: MavioSpacing.s4) {
                        Text(overview).type(theme.type.body).lineLimit(more ? nil : 5)
                        Button(more ? MavioStrings.personLess : MavioStrings.personMore) {
                            withAnimation { more.toggle() }
                        }
                        .type(theme.type.callout).fontWeight(.semibold)
                        .foregroundStyle(theme.colors.accentText)
                        .buttonStyle(.plain)
                    }
                }
                if !person.links.isEmpty {
                    Flow(spacing: MavioSpacing.s8, lineSpacing: MavioSpacing.s8) {
                        ForEach(person.links, id: \.url) { link in
                            if let url = URL(string: link.url) {
                                Link(destination: url) {
                                    HStack(spacing: 6) {
                                        Text(link.name).foregroundStyle(theme.colors.textPrimary)
                                        Icon("external", size: 14).foregroundStyle(theme.colors.textSecondary)
                                    }
                                    .type(theme.type.footnote).fontWeight(.semibold)
                                    .padding(.horizontal, 14)
                                    .padding(.vertical, 6)
                                    .background(theme.colors.fill, in: Capsule())
                                }
                            }
                        }
                    }
                    .padding(.top, MavioSpacing.s4)
                }
            }
            .frame(maxWidth: 768, alignment: .leading)
        }
        .frame(maxWidth: .infinity, alignment: .leading)
        .padding(.horizontal, theme.layout.margin)
        .padding(.top, MavioSpacing.s24)
    }

    /// Whole years between two dates.
    private func years(from: Date, to: Date) -> Int {
        Calendar.current.dateComponents([.year], from: from, to: to).year ?? 0
    }
}

/// A credited person's card, opening their page when the server names them.
struct PersonLink: View {
    let serverId: String
    let credit: CreditEntry
    let role: String?

    var body: some View {
        if credit.personId.isEmpty {
            PersonCard(credit, role: role)
        } else {
            NavigationLink(value: Route.person(serverId: serverId, personId: credit.personId)) {
                PersonCard(credit, role: role)
            }
            .buttonStyle(.plain)
        }
    }
}
