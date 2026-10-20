import MavioKit
import MavioUI
import SwiftUI

/// A person of the cast or crew on television: the round photo over the
/// name and role, which turn black on white with focus.
struct TVPersonCard: View {
    let name: String
    let photo: String?
    let role: String?
    @Environment(\.cardFocused) private var focused
    @Environment(\.theme) private var theme

    init(_ credit: CreditEntry, role: String?) {
        name = credit.name
        photo = credit.photo?.url
        self.role = role
    }

    var body: some View {
        VStack(spacing: MavioSpacing.s12) {
            ArtworkView(photo, title: name, icon: "profile")
                .frame(width: 150, height: 150)
                .clipShape(Circle())
            VStack(spacing: MavioSpacing.s2) {
                Text(name).type(theme.type.callout).fontWeight(.semibold)
                    .foregroundStyle(focused ? .black : theme.colors.textPrimary)
                Text(role ?? " ").type(theme.type.footnote)
                    .foregroundStyle(focused ? .black.opacity(0.6) : theme.colors.textSecondary)
            }
            .lineLimit(1)
            .multilineTextAlignment(.center)
        }
        .frame(width: 168)
    }
}

/// A credited person's card, opening their page when the server names them.
struct PersonLink: View {
    let serverId: String
    let credit: CreditEntry
    let role: String?

    var body: some View {
        if credit.personId.isEmpty {
            Button {
            } label: {
                TVPersonCard(credit, role: role)
            }
            .buttonStyle(PersonButtonStyle())
        } else {
            NavigationLink(value: Route.person(serverId: serverId, personId: credit.personId)) {
                TVPersonCard(credit, role: role)
            }
            .buttonStyle(PersonButtonStyle())
        }
    }
}

/// The card ending the cast's row, which opens the whole cast and crew.
struct SeeAll: View {
    @Environment(\.cardFocused) private var focused
    @Environment(\.theme) private var theme

    var body: some View {
        VStack(spacing: MavioSpacing.s12) {
            Icon("forward", size: 48)
                .foregroundStyle(focused ? .black : theme.colors.textPrimary)
                .frame(width: 150, height: 150)
                .background(Circle().fill(focused ? theme.colors.fill : theme.colors.surfaceRaised))
            Text(MavioStrings.creditsAll).type(theme.type.callout).fontWeight(.semibold)
                .foregroundStyle(focused ? .black : theme.colors.textPrimary)
            Text(" ").type(theme.type.footnote)
        }
        .frame(width: 168)
    }
}

/// A person's page: their photo, name, dates and biography, then the
/// movies, series and episodes of the server crediting them, each a row.
/// Links to other sites are left out, as nothing opens a web page (§1).
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
                    VStack(alignment: .leading, spacing: MavioSpacing.s48) {
                        PersonHeader(person: person)
                        titles(MavioStrings.personMovies, person.items.filter { $0.kind == .movie })
                        titles(MavioStrings.personSeries, person.items.filter { $0.kind == .series })
                        titles(MavioStrings.personEpisodes, person.items.filter { $0.kind == .episode }, wide: true)
                        if person.items.isEmpty {
                            Text(MavioStrings.personNone).type(theme.type.body)
                                .foregroundStyle(theme.colors.textSecondary)
                                .padding(.horizontal, theme.layout.margin)
                        }
                    }
                    .padding(.vertical, MavioSpacing.s48)
                }
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

    /// A row of one kind of title, its count beside its heading.
    @ViewBuilder private func titles(_ title: String, _ cards: [Card], wide: Bool = false) -> some View {
        if !cards.isEmpty {
            CardRow(title: "\(title)  \(cards.count)") {
                ForEach(cards, id: \.self) {
                    CardLink(card: $0, wide: wide, width: wide ? theme.layout.thumbnail : theme.layout.poster)
                }
            }
        }
    }

    private func load() async {
        do {
            person = try await model.client.person(serverId: serverId, personId: personId)
            failure = nil
        } catch is CancellationError {
        } catch {
            if person == nil { failure = error.reason }
        }
    }
}

/// A person's photo beside their name, dates and biography; the biography
/// is a button that unfolds and folds it.
private struct PersonHeader: View {
    let person: PersonDetails
    @Environment(\.theme) private var theme
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
        HStack(alignment: .top, spacing: MavioSpacing.s48) {
            ArtworkView(person.photo?.url, title: person.name, icon: "profile")
                .aspectRatio(2 / 3, contentMode: .fit)
                .frame(width: theme.layout.poster)
                .clipShape(.rect(cornerRadius: MavioRadius.lg))
            VStack(alignment: .leading, spacing: MavioSpacing.s16) {
                Text(person.name).type(theme.type.display).foregroundStyle(theme.colors.textPrimary)
                ForEach(facts, id: \.self) { fact in
                    Text(fact).type(theme.type.callout).foregroundStyle(theme.colors.textSecondary)
                }
                if let overview = person.overview {
                    Button {
                        withAnimation { more.toggle() }
                    } label: {
                        Text(overview).type(theme.type.body).lineLimit(more ? nil : 6)
                            .multilineTextAlignment(.leading)
                            .frame(maxWidth: .infinity, alignment: .leading)
                            .padding(.vertical, MavioSpacing.s16)
                    }
                    .buttonStyle(RowButtonStyle())
                    .padding(.horizontal, -MavioSpacing.s24)
                    .accessibilityHint(more ? MavioStrings.personLess : MavioStrings.personMore)
                }
            }
            .frame(maxWidth: 960, alignment: .leading)
        }
        .frame(maxWidth: .infinity, alignment: .leading)
        .padding(.horizontal, theme.layout.margin)
        .focusSection()
    }

    /// Whole years between two dates.
    private func years(from: Date, to: Date) -> Int {
        Calendar.current.dateComponents([.year], from: from, to: to).year ?? 0
    }
}

/// An item's whole cast and crew (§6.2): under its name the cast in
/// billing order, the guest stars, then the crew by job, each in a grid;
/// or the crew alone. Focus starts on the first person.
struct CreditsView: View {
    let serverId: String
    let itemId: String
    let crew: Bool
    @Environment(AppModel.self) private var model
    @Environment(\.theme) private var theme
    @State private var credits: [CreditEntry]?
    @State private var failure: String?

    var body: some View {
        Group {
            if let credits {
                ScrollView {
                    LazyVStack(alignment: .leading, spacing: MavioSpacing.s32) {
                        Text(crew ? MavioStrings.creditsFullCrew : MavioStrings.itemCast)
                            .type(theme.type.title1).fontWeight(.bold)
                            .foregroundStyle(theme.colors.textPrimary)
                        if credits.isEmpty {
                            Text(MavioStrings.creditsNone).type(theme.type.body)
                                .foregroundStyle(theme.colors.textSecondary)
                        }
                        ForEach([CreditJob.actor, .guestStar] + CreditJob.crewOrder, id: \.self) { job in
                            people(job, credits.filter { $0.job == job })
                        }
                    }
                    .padding(.horizontal, theme.layout.margin)
                    .padding(.vertical, MavioSpacing.s48)
                }
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

    /// A job's heading over its people, in a grid.
    @ViewBuilder private func people(_ job: CreditJob, _ shown: [CreditEntry]) -> some View {
        if !shown.isEmpty {
            VStack(alignment: .leading, spacing: MavioSpacing.s16) {
                Text(job.label).type(theme.type.title3).foregroundStyle(theme.colors.textPrimary)
                LazyVGrid(
                    columns: [GridItem(.adaptive(minimum: 200), spacing: theme.layout.gutter, alignment: .top)],
                    alignment: .leading, spacing: MavioSpacing.s24
                ) {
                    ForEach(Array(shown.enumerated()), id: \.offset) { _, c in
                        // The cast's characters, and what the crew's Other did.
                        PersonLink(serverId: serverId, credit: c, role: job == .other || job.isCast ? c.role : nil)
                    }
                }
                .focusSection()
            }
        }
    }

    private func load() async {
        do {
            let details = try await model.client.details(serverId: serverId, itemId: itemId)
            credits = crew ? details.credits.filter { !$0.job.isCast } : details.credits
            failure = nil
        } catch is CancellationError {
        } catch {
            if credits == nil { failure = error.reason }
        }
    }
}
