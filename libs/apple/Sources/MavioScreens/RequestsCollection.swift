#if os(macOS)
    import AppKit
    import MavioKit
    import MavioUI
    import SwiftUI

    // A discovered collection's requests on the Mac alone (docs/clients.md
    // §6.3.2), as the web UI's collection page: requesting the movies of it
    // chosen, and administrators blocklisting all of them.

    /// The actions of a collection's header: Request, choosing the movies
    /// not requested yet, and for administrators Block or Unblock; below
    /// them why blocking failed.
    struct RequestsCollectionActions: View {
        let serverId: String
        /// The collection's TMDB ID.
        let tmdbId: Int32
        let collection: Mavio_Acquisition_V1_Collection
        /// Whether every movie of it is blocklisted.
        let blocked: Bool
        /// Reads the collection again.
        let reload: () async -> Void
        @Environment(AppModel.self) private var model
        @Environment(\.theme) private var theme
        @State private var admin = false
        @State private var asking = false
        @State private var blocking = false
        @State private var failure: String?

        var body: some View {
            VStack(alignment: .leading, spacing: MavioSpacing.s12) {
                Flow(spacing: MavioSpacing.s12, lineSpacing: MavioSpacing.s12) {
                    if collection.parts.contains(where: \.requestable) {
                        Button {
                            asking = true
                        } label: {
                            Label {
                                Text(MavioStrings.collectionRequest).lineLimit(1)
                            } icon: {
                                Icon("add", size: 20)
                            }
                        }
                        .buttonStyle(MavioButtonStyle(.artworkPrimary))
                    }
                    if admin {
                        Button(action: block) {
                            Label {
                                Text(blocked ? MavioStrings.collectionUnblock : MavioStrings.collectionBlock)
                                    .lineLimit(1)
                            } icon: {
                                Icon(blocked ? "check" : "close", size: 20)
                            }
                        }
                        .buttonStyle(MavioButtonStyle(.artworkSecondary))
                        .disabled(blocking)
                    }
                }
                if let failure {
                    Text(failure).type(theme.type.callout).foregroundStyle(.white)
                }
            }
            .task(id: serverId) { admin = await RequestsAPI.currentUser(model, serverId)?.admin == true }
            .sheet(isPresented: $asking) {
                CollectionRequestSheet(serverId: serverId, collection: collection, reload: reload)
                    .environment(
                        \.colorScheme,
                        NSApp.effectiveAppearance.bestMatch(from: [.darkAqua, .aqua]) == .darkAqua ? .dark : .light)
            }
        }

        private func block() {
            var unblock = Mavio_Acquisition_V1_UnblockCollectionRequest()
            unblock.tmdbCollectionID = tmdbId
            var block = Mavio_Acquisition_V1_BlockCollectionRequest()
            block.tmdbCollectionID = tmdbId
            blocking = true
            Task {
                do {
                    if blocked {
                        _ = try await model.client.call(
                            serverId, Procedure.requests("UnblockCollection"), unblock,
                            as: Mavio_Acquisition_V1_UnblockCollectionResponse.self)
                    } else {
                        _ = try await model.client.call(
                            serverId, Procedure.requests("BlockCollection"), block,
                            as: Mavio_Acquisition_V1_BlockCollectionResponse.self)
                    }
                    failure = nil
                } catch {
                    failure = error.reason
                }
                blocking = false
                await reload()
            }
        }
    }

    /// The sheet choosing the movies of a collection to request, all those
    /// not requested yet by default.
    private struct CollectionRequestSheet: View {
        let serverId: String
        let collection: Mavio_Acquisition_V1_Collection
        let reload: () async -> Void
        @Environment(AppModel.self) private var model
        @Environment(\.theme) private var theme
        @Environment(\.dismiss) private var dismiss
        @State private var chosen: Set<Int32>
        @State private var busy = false
        @State private var failure: String?

        init(serverId: String, collection: Mavio_Acquisition_V1_Collection, reload: @escaping () async -> Void) {
            (self.serverId, self.collection, self.reload) = (serverId, collection, reload)
            _chosen = State(initialValue: Set(collection.parts.filter(\.requestable).map(\.tmdbId)))
        }

        private var open: [Int32] { collection.parts.filter(\.requestable).map(\.tmdbId) }

        var body: some View {
            NavigationStack {
                List {
                    Toggle(
                        isOn: Binding(get: { chosen.count == open.count }, set: { chosen = $0 ? Set(open) : [] })
                    ) {
                        Text(MavioStrings.collectionAllMovies).type(theme.type.headline)
                    }
                    ForEach(collection.parts, id: \.key) { p in
                        Toggle(
                            isOn: Binding(
                                get: { chosen.contains(p.tmdbId) },
                                set: { if $0 { chosen.insert(p.tmdbId) } else { chosen.remove(p.tmdbId) } })
                        ) {
                            HStack(spacing: MavioSpacing.s12) {
                                Text(p.name).lineLimit(1)
                                Spacer(minLength: 0)
                                if p.year > 0 {
                                    Text(String(p.year)).type(theme.type.callout)
                                        .foregroundStyle(theme.colors.textSecondary)
                                }
                                if let status = statusLabel(p.status) {
                                    Text(status).type(theme.type.footnote).foregroundStyle(theme.colors.textSecondary)
                                }
                            }
                        }
                        .disabled(!p.requestable)
                    }
                    if let failure {
                        Text(MavioStrings.discoverRequestFailed(reason: failure))
                            .type(theme.type.callout).foregroundStyle(theme.colors.danger)
                    }
                }
                .toggleStyle(.checkbox)
                .navigationTitle(MavioStrings.discoverRequestTitle(name: collection.name))
                .toolbar {
                    ToolbarItem(placement: .cancellationAction) {
                        Button(MavioStrings.actionCancel) { dismiss() }
                    }
                    ToolbarItem(placement: .confirmationAction) {
                        Button(MavioStrings.collectionRequestMovies(count: chosen.count), action: submit)
                            .disabled(busy || chosen.isEmpty)
                    }
                }
            }
            .frame(minWidth: 420, minHeight: 360)
        }

        /// Requests each movie chosen in turn, closing once all are; the
        /// first that fails stops and says why.
        private func submit() {
            busy = true
            let ids = collection.parts.map(\.tmdbId).filter(chosen.contains)
            Task {
                do {
                    for id in ids { try await RequestsAPI.create(model, serverId, .movie, id, seasons: nil) }
                    busy = false
                    await reload()
                    dismiss()
                } catch {
                    failure = error.reason
                    busy = false
                    await reload()
                }
            }
        }
    }
#endif
