#if os(macOS)
    import MavioKit
    import MavioUI
    import SwiftUI

    /// A choice of the AI settings' searching pickers, as the web UI's model picker lists it: its value, its
    /// name, a line of details under it, and its provider's ID and name; no provider for a choice of none.
    struct AiPickerOption: Hashable, Identifiable {
        var value: String
        var label: String
        var detail: String?
        var provider: String?
        var providerName: String?
        /// A badge after its name, as a custom provider's.
        var badge: String?

        var id: String { value }

        /// The model as a choice, its value `<prefix><provider>/<model>`.
        init(model m: Mavio_Plugin_V1_AiModel, providerNames: [String: String], prefix: String = "") {
            value = "\(prefix)\(m.provider)/\(m.id)"
            label = m.name.isEmpty ? m.id : m.name
            detail = Self.detail(of: m)
            provider = m.provider
            providerName = providerNames[m.provider] ?? m.provider
        }

        init(
            value: String, label: String, detail: String? = nil, provider: String? = nil,
            providerName: String? = nil, badge: String? = nil
        ) {
            (self.value, self.label, self.detail) = (value, label, detail)
            (self.provider, self.providerName, self.badge) = (provider, providerName, badge)
        }

        /// A model's details on one line, as the web UI's: its ID when it has a name, context window, reasoning,
        /// images and cost.
        static func detail(of m: Mavio_Plugin_V1_AiModel) -> String {
            var parts: [String] = []
            if m.id != m.name { parts.append(m.id) }
            if m.contextWindow > 0 {
                parts.append(
                    MavioStrings.aiContextWindow(tokens: m.contextWindow.formatted(.number.notation(.compactName))))
            }
            if m.reasoning { parts.append(MavioStrings.aiReasoning) }
            if m.images { parts.append(MavioStrings.aiImages) }
            if m.hasCost, m.cost.input > 0 || m.cost.output > 0 {
                let dollars = FloatingPointFormatStyle<Double>.Currency(code: "USD").precision(.fractionLength(0...2))
                parts.append(
                    MavioStrings.aiCost(
                        input: m.cost.input.formatted(dollars), output: m.cost.output.formatted(dollars)))
            }
            return parts.joined(separator: " · ")
        }

        /// Whether its name, value or provider holds what was typed.
        func matches(_ query: String) -> Bool {
            let q = query.trimmingCharacters(in: .whitespaces)
            return q.isEmpty || [label, value, providerName ?? ""].contains { $0.localizedCaseInsensitiveContains(q) }
        }
    }

    extension AppModel {
        /// The models of a server's usable providers as the model pickers list them, by `<prefix><provider>/<model>`;
        /// none when no AI gateway plugin runs, nil when they could not be read.
        func aiModelOptions(_ server: ServerRecord, prefix: String = "") async -> [AiPickerOption]? {
            var request = Mavio_System_V1_ListAiModelsRequest()
            request.available = true
            async let providers = try? call(
                server, Procedure.aiProviders("ListAiProviders"), Mavio_System_V1_ListAiProvidersRequest(),
                as: Mavio_System_V1_ListAiProvidersResponse.self)
            do {
                let models = try await call(
                    server, Procedure.aiProviders("ListAiModels"), request,
                    as: Mavio_System_V1_ListAiModelsResponse.self
                ).models
                let names = Dictionary(
                    ((await providers)?.providers ?? []).map { ($0.id, $0.name) }, uniquingKeysWith: { a, _ in a })
                return models.map { AiPickerOption(model: $0, providerNames: names, prefix: prefix) }
            } catch {
                return error.isNoAiGateway ? [] : nil
            }
        }
    }

    extension Error {
        /// Whether the error says that no AI gateway plugin runs, as the server answers without one.
        var isNoAiGateway: Bool {
            if case .FailedPrecondition = self as? CoreError { return true }
            return false
        }
    }

    /// The logo of an AI provider, from libs/design's: in its own colors, or the text's for a monochrome one;
    /// a chip's icon for a provider without, as the web UI's ProviderLogo.
    struct AiProviderLogo: View {
        let provider: String
        var size: CGFloat = 16
        @Environment(\.theme) private var theme

        var body: some View {
            if let name = MavioProviderLogos.assetName(for: provider) {
                Image(name, bundle: MavioStrings.bundle)
                    .resizable()
                    .scaledToFit()
                    .frame(width: size, height: size)
                    .foregroundStyle(theme.colors.textPrimary)
                    .accessibilityHidden(true)
            } else {
                Icon("cpu", size: size).foregroundStyle(theme.colors.textTertiary).accessibilityHidden(true)
            }
        }
    }

    /// A choice as the pickers' rows show it first: its provider's logo, its name and, smaller and quieter, its
    /// provider's name, as the web UI's ModelLabel.
    struct AiOptionLabel: View {
        let option: AiPickerOption
        @Environment(\.theme) private var theme

        var body: some View {
            HStack(spacing: MavioSpacing.s8) {
                if let provider = option.provider { AiProviderLogo(provider: provider) }
                Text(option.label).lineLimit(1).truncationMode(.tail)
                if let name = option.providerName {
                    Text(name).font(.system(size: 11)).foregroundStyle(theme.colors.textTertiary).lineLimit(1)
                }
                if let badge = option.badge {
                    Text(badge)
                        .type(theme.type.caption)
                        .foregroundStyle(theme.colors.textSecondary)
                        .padding(.horizontal, 6)
                        .padding(.vertical, 1)
                        .background(theme.colors.fill, in: .capsule)
                }
            }
        }
    }

    /// A choice among many, as the web UI's model picker and searching picker: a button showing the one chosen,
    /// whose popover searches them, each row with its details under it.
    struct AiSearchPicker: View {
        let label: String
        let options: [AiPickerOption]
        @Binding var value: String
        let placeholder: String
        var disabled = false
        var width: CGFloat = 288
        @Environment(\.theme) private var theme
        @State private var open = false
        @State private var query = ""

        var body: some View {
            Button {
                query = ""
                open = true
            } label: {
                HStack(spacing: MavioSpacing.s8) {
                    if let chosen = options.first(where: { $0.value == value }) {
                        AiOptionLabel(option: chosen)
                    } else {
                        Text(placeholder).foregroundStyle(theme.colors.textTertiary).lineLimit(1)
                    }
                    Spacer(minLength: 0)
                    Icon("selector", size: 14).foregroundStyle(theme.colors.textSecondary)
                }
                .frame(width: width - 24)
                .foregroundStyle(theme.colors.textPrimary)
            }
            .buttonStyle(.bordered)
            .disabled(disabled)
            .accessibilityLabel(label)
            .popover(isPresented: $open, arrowEdge: .bottom) { list }
        }

        private var list: some View {
            let shown = options.filter { $0.matches(query) }
            return VStack(spacing: 0) {
                TextField(MavioStrings.navSearch, text: $query)
                    .textFieldStyle(.roundedBorder)
                    .padding(MavioSpacing.s8)
                Divider()
                if shown.isEmpty {
                    Text(MavioStrings.schemaNoMatches)
                        .type(theme.type.callout)
                        .foregroundStyle(theme.colors.textSecondary)
                        .frame(maxWidth: .infinity, maxHeight: .infinity)
                } else {
                    ScrollView {
                        LazyVStack(alignment: .leading, spacing: 0) {
                            ForEach(shown) { option in
                                Button {
                                    value = option.value
                                    open = false
                                } label: {
                                    row(option)
                                }
                                .buttonStyle(.plain)
                            }
                        }
                        .padding(.vertical, MavioSpacing.s4)
                    }
                }
            }
            .frame(width: max(width, 340), height: 380)
        }

        private func row(_ option: AiPickerOption) -> some View {
            HStack(spacing: MavioSpacing.s8) {
                VStack(alignment: .leading, spacing: 2) {
                    AiOptionLabel(option: option)
                    if let detail = option.detail, !detail.isEmpty {
                        Text(detail)
                            .type(theme.type.caption)
                            .foregroundStyle(theme.colors.textSecondary)
                            .lineLimit(1)
                    }
                }
                Spacer(minLength: 0)
                if option.value == value {
                    Icon("check", size: 14).foregroundStyle(theme.colors.accent)
                }
            }
            .padding(.horizontal, MavioSpacing.s12)
            .padding(.vertical, 6)
            .frame(maxWidth: .infinity, alignment: .leading)
            .contentShape(.rect)
            .background(option.value == value ? theme.colors.fill : .clear)
        }
    }

    /// A row opening a page, as the web UI's PageLink: its icon on a colored tile, its label and its detail.
    struct AiPageLabel: View {
        let icon: String
        let color: Color
        let label: String
        var detail: String?
        @Environment(\.theme) private var theme

        var body: some View {
            HStack(spacing: MavioSpacing.s12) {
                ColorTile(icon: icon, color: color)
                VStack(alignment: .leading, spacing: 2) {
                    Text(label).lineLimit(1)
                    if let detail, !detail.isEmpty {
                        Text(detail)
                            .type(theme.type.footnote)
                            .foregroundStyle(theme.colors.textSecondary)
                            .lineLimit(2)
                    }
                }
            }
        }
    }
#endif
