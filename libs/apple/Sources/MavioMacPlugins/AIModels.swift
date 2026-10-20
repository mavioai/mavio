#if os(macOS)
    import AppKit
    import MavioMac

    /// A model as a choice of the AI settings' pickers, its value
    /// `<prefix><provider>/<model>`, as the web UI's model picker lists it.
    func aiModelItem(_ m: Mavio_Plugin_V1_AiModel, providerNames: [String: String], prefix: String = "") -> PickerItem {
        PickerItem(
            value: "\(prefix)\(m.provider)/\(m.id)", label: m.name.isEmpty ? m.id : m.name, detail: aiModelDetail(m),
            provider: m.provider, providerName: providerNames[m.provider] ?? m.provider)
    }

    /// A model's details on one line, as the web UI's: its ID when it has a
    /// name, context window, reasoning, images and cost.
    func aiModelDetail(_ m: Mavio_Plugin_V1_AiModel) -> String {
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
                MavioStrings.aiCost(input: m.cost.input.formatted(dollars), output: m.cost.output.formatted(dollars)))
        }
        return parts.joined(separator: " · ")
    }

    /// Whether an error says that no AI gateway plugin runs, as the server
    /// answers without one.
    func isNoAiGateway(_ error: any Error) -> Bool {
        if case .FailedPrecondition = error as? CoreError { return true }
        return false
    }

    extension AppModel {
        /// The models of a server's usable providers as the model pickers
        /// list them, by `<prefix><provider>/<model>`; none when no AI
        /// gateway plugin runs, nil when they could not be read.
        func aiModelItems(_ server: ServerRecord, prefix: String = "") async -> [PickerItem]? {
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
                return models.map { aiModelItem($0, providerNames: names, prefix: prefix) }
            } catch {
                return isNoAiGateway(error) ? [] : nil
            }
        }
    }

    /// A row opening a page, as the web UI's PageLink: its icon on a colored
    /// tile, its label and its detail, and a chevron.
    @MainActor func pageLinkRow(
        icon: String, color: NSColor, label: String, detail: String?, theme: Theme,
        action: @escaping @MainActor () -> Void
    ) -> FormRow {
        FormRow.link(
            label, subtitle: detail?.isEmpty == false ? detail : nil, icon: icon, tint: color, theme: theme,
            action: action)
    }
#endif
