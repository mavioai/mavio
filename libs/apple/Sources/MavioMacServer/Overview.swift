#if os(macOS)
    import AppKit
    import MavioMac

    /// Overview (docs/clients.md §6.3.6): whether the server serves, its
    /// version and what it runs on, and the load of its computer, sampled
    /// every two seconds while the page shows.
    final class OverviewPage: ServerPage {
        private var info: Mavio_System_V1_GetSystemInfoResponse?
        private var serving: Bool?
        private var runtime: Mavio_System_V1_ContainerRuntime?
        private var samples: [Mavio_System_V1_LoadSample] = []
        private var failure: String?
        private var loadFailure: String?
        private lazy var form = FormPage(title: MavioStrings.settingsOverview, theme: theme)
        private lazy var serverSection = section(MavioStrings.overviewServer)
        private lazy var loadSection = section(MavioStrings.overviewLoad, footer: MavioStrings.overviewLoadFooter)

        override func viewDidLoad() {
            super.viewDidLoad()
            title = MavioStrings.settingsOverview
            form.add(serverSection)
            form.add(loadSection)
            render()
            show(form)
        }

        override func appeared() {
            run { [weak self] in await self?.loadInfo() }
            poll(every: { .seconds(2) }) { [weak self] in await self?.loadSamples() }
        }

        private func render() {
            renderServer()
            renderLoad()
        }

        private func renderServer() {
            var rows: [NSView] = []
            let status = NSStackView.horizontal(
                [
                    dot(
                        serving == nil
                            ? theme.colors.fill : serving == true ? NSColor(hex: 0x34C759) : theme.colors.danger),
                    NSTextField.label(
                        serving == nil
                            ? "…" : serving == true ? MavioStrings.overviewServing : MavioStrings.overviewNotServing,
                        theme.fonts.body, color: theme.colors.textSecondary),
                ], spacing: MavioSpacing.s8)
            rows.append(FormRow(MavioStrings.overviewStatus, accessory: status, theme: theme))
            if let info {
                rows.append(valueRow(MavioStrings.overviewVersion, info.version))
                rows.append(valueRow(MavioStrings.overviewPlatform, "\(info.os)/\(info.arch)"))
                rows.append(valueRow(MavioStrings.overviewDatabase, Self.database(info.database)))
                rows.append(
                    valueRow(
                        MavioStrings.overviewFfmpeg,
                        info.ffmpegVersion.isEmpty ? MavioStrings.overviewFfmpegMissing : info.ffmpegVersion))
            }
            if let runtime {
                var lines = [
                    NSTextField.label(
                        Self.engine(runtime), theme.fonts.body, color: theme.colors.textSecondary, lines: 0)
                ]
                if runtime.available {
                    lines.append(
                        NSTextField.label(
                            runtime.endpoint, theme.fonts.footnote, color: theme.colors.textSecondary, selectable: true)
                    )
                }
                for line in lines { line.alignment = .right }
                rows.append(
                    FormRow(
                        MavioStrings.overviewContainerEngine,
                        accessory: NSStackView.vertical(lines, spacing: 2, alignment: .trailing), theme: theme))
            }
            if let start = info?.startTime.time {
                rows.append(
                    valueRow(
                        MavioStrings.overviewUptime,
                        Duration.seconds(Date.now.timeIntervalSince(start)).formatted(
                            .units(allowed: [.days, .hours, .minutes], width: .narrow, maximumUnitCount: 2))))
            }
            if let failure { rows.append(note(failure, failed: true)) }
            serverSection.setRows(rows)
        }

        private func renderLoad() {
            if let loadFailure {
                loadSection.setRows([note(loadFailure, failed: true)])
            } else {
                loadSection.setRows(LoadCards.cards(samples, theme: theme).map(padded))
            }
        }

        private func loadInfo() async {
            let client = model.client
            let id = server.id
            async let health = try? client.call(
                id, Procedure.system("GetHealth"), Mavio_System_V1_GetHealthRequest(),
                as: Mavio_System_V1_GetHealthResponse.self)
            async let engine = try? client.call(
                id, Procedure.plugins("GetContainerRuntime"), Mavio_System_V1_GetContainerRuntimeRequest(),
                as: Mavio_System_V1_GetContainerRuntimeResponse.self)
            do {
                info = try await client.call(
                    id, Procedure.system("GetSystemInfo"), Mavio_System_V1_GetSystemInfoRequest(),
                    as: Mavio_System_V1_GetSystemInfoResponse.self)
                failure = nil
            } catch {
                failure = error.reason
            }
            serving = await health.map { $0.status == .serving } ?? false
            runtime = await engine?.runtime
            renderServer()
        }

        private func loadSamples() async {
            do {
                samples = try await model.client.call(
                    server.id, Procedure.system("GetSystemLoad"), Mavio_System_V1_GetSystemLoadRequest(),
                    as: Mavio_System_V1_GetSystemLoadResponse.self
                ).samples
                loadFailure = nil
            } catch is CancellationError {
                return
            } catch {
                loadFailure = error.reason
            }
            renderLoad()
        }

        private static func database(_ name: String) -> String {
            switch name {
            case "postgres": "PostgreSQL"
            case "sqlite": "SQLite"
            default: name
            }
        }

        private static func engine(_ r: Mavio_System_V1_ContainerRuntime) -> String {
            guard r.available else { return MavioStrings.overviewContainerEngineNone(reason: r.reason) }
            let name = ["docker": "Docker", "podman": "Podman"][r.engine] ?? r.engine
            return [
                "\(name) \(r.version)", "API \(r.apiVersion)",
                r.rootless ? MavioStrings.overviewContainerEngineRootless : "",
            ]
            .filter { !$0.isEmpty }.joined(separator: " · ")
        }
    }

    /// The cards of the load of the server's computer: its CPU, memory,
    /// disks and network, each with what it is now, three figures more and
    /// a graph of the last two minutes.
    @MainActor
    private enum LoadCards {
        static func cards(_ samples: [Mavio_System_V1_LoadSample], theme: Theme) -> [NSView] {
            let now = samples.last
            let c = now.flatMap { $0.hasCpu ? $0.cpu : nil }
            let cpu = LoadCard(
                icon: "cpu", color: NSColor(hex: 0x007AFF), title: MavioStrings.overviewCpu, badge: nil,
                told: now == nil || c != nil, value: c.map { percent($0.user + $0.system) }, after: nil,
                stats: [
                    (MavioStrings.overviewCpuUser, c.map { percent($0.user) }),
                    (MavioStrings.overviewCpuSystem, c.map { percent($0.system) }),
                    (MavioStrings.overviewByServer, c.map { percent($0.server) }),
                ],
                graph: samples.map { $0.hasCpu ? $0.cpu.user + $0.cpu.system : nil }, theme: theme)
            let m = now.flatMap { $0.hasMemory ? $0.memory : nil }
            let share = m.flatMap { $0.totalBytes > 0 ? Double($0.usedBytes) / Double($0.totalBytes) : nil }
            let memory = LoadCard(
                icon: "memory", color: NSColor(hex: 0x34C759), title: MavioStrings.overviewMemory,
                badge: share.map { $0 >= 0.9 }, told: now == nil || m != nil, value: m.map { bytes($0.usedBytes) },
                after: m.map { MavioStrings.overviewMemoryOf(total: bytes($0.totalBytes)) },
                stats: [
                    (MavioStrings.overviewMemoryAvailable, m.map { bytes($0.totalBytes - $0.usedBytes) }),
                    (MavioStrings.overviewSwap, m.map { $0.swapTotalBytes > 0 ? bytes($0.swapUsedBytes) : "–" }),
                    (MavioStrings.overviewByServer, m.map { bytes($0.serverBytes) }),
                ],
                graph: samples.map {
                    $0.hasMemory && $0.memory.totalBytes > 0
                        ? Double($0.memory.usedBytes) / Double($0.memory.totalBytes) : nil
                }, theme: theme)
            let d = now.flatMap { $0.hasDisk ? $0.disk : nil }
            let disk = LoadCard(
                icon: "disk", color: NSColor(hex: 0xFF9500), title: MavioStrings.overviewDisk, badge: nil,
                told: now == nil || d != nil, value: d.map { $0.hasTotalBytes ? bytes($0.availableBytes) : "–" },
                after: d.flatMap {
                    $0.hasTotalBytes ? MavioStrings.overviewDiskAvailable(total: bytes($0.totalBytes)) : nil
                },
                stats: [
                    (MavioStrings.overviewRead, d.map { $0.hasReadRate ? rate($0.readRate) : "–" }),
                    (MavioStrings.overviewWrite, d.map { $0.hasWriteRate ? rate($0.writeRate) : "–" }),
                    (MavioStrings.overviewWritten, d.map { $0.hasWrittenBytes ? bytes($0.writtenBytes) : "–" }),
                ],
                graph: scaled(samples.map { $0.hasDisk ? $0.disk.readRate + $0.disk.writeRate : nil }, floor: 1e6),
                theme: theme)
            let n = now.flatMap { $0.hasNetwork ? $0.network : nil }
            let network = LoadCard(
                icon: "network", color: NSColor(hex: 0x30B0C7), title: MavioStrings.overviewNetwork, badge: nil,
                told: now == nil || n != nil, value: n.map { rate($0.receiveRate) },
                after: n.map { _ in MavioStrings.overviewDownload },
                stats: [
                    (MavioStrings.overviewUpload, n.map { rate($0.sendRate) }),
                    (MavioStrings.overviewReceived, n.map { bytes($0.receivedBytes) }),
                    (MavioStrings.overviewSent, n.map { bytes($0.sentBytes) }),
                ],
                graph: scaled(
                    samples.map { $0.hasNetwork ? $0.network.receiveRate + $0.network.sendRate : nil }, floor: 1e5),
                theme: theme)
            return [cpu, memory, disk, network]
        }

        static func percent(_ share: Double) -> String {
            share.formatted(.percent.precision(.fractionLength(0)))
        }

        static func bytes(_ count: Int64) -> String {
            count.formatted(.byteCount(style: .decimal))
        }

        /// Bytes per second in kilobytes, megabytes or gigabytes as it is
        /// large, a decimal only below ten.
        static func rate(_ bytes: Double) -> String {
            let (n, unit): (Double, UnitInformationStorage) =
                bytes >= 1e9
                ? (bytes / 1e9, .gigabytes) : bytes >= 1e6 ? (bytes / 1e6, .megabytes) : (bytes / 1e3, .kilobytes)
            let value = Measurement(value: n, unit: unit).formatted(
                .measurement(
                    width: .abbreviated, usage: .asProvided,
                    numberFormatStyle: .number.precision(.fractionLength(n < 10 ? 1 : 0))))
            return "\(value)/s"
        }

        /// Rates as shares of the highest of them, or of floor when all are
        /// lower.
        static func scaled(_ values: [Double?], floor: Double) -> [Double?] {
            let top = max(floor, values.compactMap(\.self).max() ?? 0)
            return values.map { $0.map { $0 / top } }
        }
    }

    /// A card of the load: its tile and title, what it is now, large, three
    /// figures more and the graph of the last samples. told is false where
    /// the server's system does not tell it; while no sample came, the
    /// figures wait.
    private final class LoadCard: NSView {
        init(
            icon: String, color: NSColor, title: String, badge high: Bool?, told: Bool, value: String?,
            after: String?, stats: [(String, String?)], graph: [Double?], theme: Theme
        ) {
            super.init(frame: .zero)
            translatesAutoresizingMaskIntoConstraints = false
            let tile = Box(fill: color, radius: 6)
            tile.size(width: 24, height: 24)
            let image = IconView(icon, fill: true, size: 14, color: .white)
            tile.addSubview(image)
            image.center(in: tile)
            var head: [NSView] = [tile, NSTextField.label(title, theme.fonts.headline, color: color), Spacer()]
            if let high {
                head.append(
                    MavioMacServer.badge(
                        high ? MavioStrings.overviewMemoryHigh : MavioStrings.overviewMemoryNormal, theme.fonts.caption,
                        color: NSColor(hex: high ? 0xC93400 : 0x248A3D),
                        fill: NSColor(hex: high ? 0xFF9500 : 0x34C759, alpha: 0.15), radius: 999,
                        horizontal: MavioSpacing.s8, vertical: 2))
            }
            let header = NSStackView.horizontal(head, spacing: MavioSpacing.s8)
            var views: [NSView] = [header]
            if told {
                let big = NSTextField.label(
                    value ?? "…", theme.fonts.title2,
                    color: value == nil ? theme.colors.textTertiary : theme.colors.textPrimary)
                big.font = .monospacedDigitSystemFont(ofSize: theme.fonts.title2.size, weight: .semibold)
                var line: [NSView] = [big]
                if value != nil, let after {
                    line.append(NSTextField.label(after, theme.fonts.footnote, color: theme.colors.textSecondary))
                }
                views.append(NSStackView.horizontal(line, spacing: MavioSpacing.s8, alignment: .firstBaseline))
                let columns = stats.map { label, v in
                    let figure = NSTextField.label(v ?? "…", theme.fonts.footnote, color: theme.colors.textPrimary)
                    figure.font = .monospacedDigitSystemFont(ofSize: theme.fonts.footnote.size, weight: .regular)
                    return NSStackView.vertical(
                        [NSTextField.label(label, theme.fonts.caption, color: theme.colors.textSecondary), figure],
                        spacing: 2)
                }
                let row = NSStackView.horizontal(columns, spacing: MavioSpacing.s8, alignment: .top)
                row.distribution = .fillEqually
                views.append(row)
                views.append(DotsGraph(values: graph, color: color, unlit: theme.colors.textSecondary))
            } else {
                views.append(
                    NSTextField.label(
                        MavioStrings.overviewLoadUnknown, theme.fonts.callout, color: theme.colors.textSecondary))
            }
            let stack = NSStackView.vertical(views, spacing: MavioSpacing.s12)
            addFilling(stack, insets: .symmetric(vertical: MavioSpacing.s4))
            for view in views { view.widthAnchor.constraint(equalTo: stack.widthAnchor).isActive = true }
            setAccessibilityElement(true)
            setAccessibilityRole(.group)
            setAccessibilityLabel([title, value ?? ""].joined(separator: " "))
        }

        @available(*, unavailable)
        required init?(coder: NSCoder) { fatalError() }
    }

    /// A graph of dots: a column per sample, the latest at the right, lit
    /// from the bottom as high as its value, from 0 to 1; columns without a
    /// sample stay unlit.
    private final class DotsGraph: NSView {
        /// The samples a graph has room for: two minutes', as the server keeps.
        private static let columns = 60
        private static let rows = 6
        private let values: [Double?]
        private let color: NSColor
        private let unlit: NSColor

        init(values: [Double?], color: NSColor, unlit: NSColor) {
            self.values = values
            self.color = color
            self.unlit = unlit.withAlphaComponent(0.12)
            super.init(frame: .zero)
            translatesAutoresizingMaskIntoConstraints = false
            aspect(CGFloat(Self.columns) / CGFloat(Self.rows))
            setAccessibilityElement(false)
        }

        @available(*, unavailable)
        required init?(coder: NSCoder) { fatalError() }

        override var isFlipped: Bool { true }

        override func draw(_ dirtyRect: NSRect) {
            let cell = bounds.width / CGFloat(Self.columns)
            let shown = values.suffix(Self.columns)
            let offset = Self.columns - shown.count
            for x in 0..<Self.columns {
                let v = x >= offset ? shown[shown.startIndex + x - offset] : nil
                // A value above nothing lights a dot, so that activity shows.
                let lit = v.map { $0 > 0 ? max(1, Int((min(1, $0) * Double(Self.rows)).rounded())) : 0 } ?? 0
                for y in 0..<Self.rows {
                    let on = Self.rows - y <= lit
                    let r = cell * 0.28
                    let center = CGPoint(x: (CGFloat(x) + 0.5) * cell, y: (CGFloat(y) + 0.5) * cell)
                    (on ? color : unlit).setFill()
                    NSBezierPath(ovalIn: CGRect(x: center.x - r, y: center.y - r, width: r * 2, height: r * 2)).fill()
                }
            }
        }
    }
#endif
