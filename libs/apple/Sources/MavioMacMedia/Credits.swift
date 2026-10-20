#if os(macOS)
    import AppKit
    import MavioMac

    /// An item's whole cast and crew (libs/design/components.md, Credits
    /// Screen): the cast in billing order, the guest stars, then the crew by
    /// job; or the crew alone, which View Full Crew opens.
    final class CreditsPage: Page {
        private let serverId: String
        private let itemId: String
        private let crew: Bool
        private var credits: [CreditEntry]?
        private var observer: Observer?
        private var loadTask: Task<Void, Never>?
        private var revision: Int?

        /// The credits of an item of a server, or its crew alone.
        init(serverId: String, itemId: String, crew: Bool, context: MacContext) {
            self.serverId = serverId
            self.itemId = itemId
            self.crew = crew
            super.init(context: context)
        }

        override func viewDidLoad() {
            super.viewDidLoad()
            title = crew ? MavioStrings.creditsFullCrew : MavioStrings.itemCast
            show(LoadingView())
            observer = Observer { [weak self] in
                guard let self else { return }
                let now = self.model.revision(self.serverId)
                guard now != self.revision else { return }
                self.revision = now
                self.load()
            }
        }

        private func load() {
            loadTask?.cancel()
            loadTask = Task { [weak self] in
                guard let self else { return }
                do {
                    let details = try await self.model.client.details(serverId: self.serverId, itemId: self.itemId)
                    guard !Task.isCancelled else { return }
                    self.credits = self.crew ? details.credits.filter { !$0.job.isCast } : details.credits
                    self.render()
                } catch is CancellationError {
                } catch {
                    guard !Task.isCancelled, self.credits == nil else { return }
                    self.showError(error.reason) { [weak self] in self?.load() }
                }
            }
        }

        private func render() {
            guard let credits else { return }
            var views: [NSView] = []
            if theme.isTV {
                views.append(
                    margined(
                        NSTextField.label(title ?? "", theme.fonts.title2, color: theme.colors.textPrimary),
                        top: MavioSpacing.s32))
            }
            if credits.isEmpty {
                views.append(
                    margined(
                        NSTextField.label(MavioStrings.creditsNone, theme.fonts.body, color: theme.colors.textSecondary)
                    ))
            }
            for job in [CreditJob.actor, .guestStar] + CreditJob.crewOrder {
                let shown = credits.filter { $0.job == job }
                guard !shown.isEmpty else { continue }
                let grid = FlowGrid(
                    minimum: theme.isTV ? 180 : 104, gutter: theme.isTV ? theme.layout.gutter : MavioSpacing.s16,
                    rowSpacing: MavioSpacing.s24)
                grid.setItems(
                    shown.map { c in
                        personCard(c, serverId: serverId, role: c.job == .other || c.job.isCast ? c.role : nil)
                    })
                let stack = NSStackView.vertical([heading(job.label), grid], spacing: MavioSpacing.s12)
                grid.widthAnchor.constraint(equalTo: stack.widthAnchor).isActive = true
                views.append(margined(stack))
            }
            show(ScrollPage(views, spacing: MavioSpacing.s24, insets: .symmetric(vertical: MavioSpacing.s16)))
            FocusEngine.active(for: view)?.schedule()
        }

        isolated deinit {
            observer?.cancel()
            loadTask?.cancel()
        }
    }
#endif
