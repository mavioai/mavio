import MavioKit
import MavioUI
import SwiftUI

/// An item's whole cast and crew (libs/design/components.md, Credits
/// Screen): the cast in billing order, the guest stars, then the crew by
/// job; or the crew alone, which View Full Crew opens.
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
                    LazyVStack(alignment: .leading, spacing: MavioSpacing.s24) {
                        if credits.isEmpty {
                            Text(MavioStrings.creditsNone).type(theme.type.body)
                                .foregroundStyle(theme.colors.textSecondary)
                                .padding(.horizontal, theme.layout.margin)
                        }
                        ForEach([CreditJob.actor, .guestStar] + CreditJob.crewOrder, id: \.self) { job in
                            people(job.label, credits.filter { $0.job == job })
                        }
                    }
                    .padding(.vertical, MavioSpacing.s16)
                }
            } else if failure != nil {
                StatusView(
                    icon: "error", title: MavioStrings.stateError, message: failure,
                    action: (MavioStrings.actionRetry, { Task { await load() } }))
            } else {
                ProgressView().frame(maxWidth: .infinity, maxHeight: .infinity)
            }
        }
        .navigationTitle(crew ? MavioStrings.creditsFullCrew : MavioStrings.itemCast)
        .navigationBarTitleDisplayMode(.large)
        .task(id: model.revision(serverId)) { await load() }
    }

    /// A heading over its people, in a grid.
    @ViewBuilder private func people(_ title: String, _ shown: [CreditEntry]) -> some View {
        if !shown.isEmpty {
            VStack(alignment: .leading, spacing: MavioSpacing.s12) {
                Text(title).type(theme.type.title3)
                LazyVGrid(
                    columns: [GridItem(.adaptive(minimum: 104), spacing: MavioSpacing.s16, alignment: .top)],
                    spacing: MavioSpacing.s24
                ) {
                    ForEach(Array(shown.enumerated()), id: \.offset) { _, c in
                        PersonLink(serverId: serverId, credit: c, role: c.job == .other || c.job.isCast ? c.role : nil)
                    }
                }
            }
            .padding(.horizontal, theme.layout.margin)
        }
    }

    private func load() async {
        do {
            let details = try await model.client.details(serverId: serverId, itemId: itemId)
            credits = crew ? details.credits.filter { !$0.job.isCast } : details.credits
            failure = nil
        } catch is CancellationError {
        } catch {
            if credits == nil { failure = error.localizedDescription }
        }
    }
}
