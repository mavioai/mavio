#if os(macOS)
    import AppKit
    import MavioMac

    /// A dot that pulses, as a task that runs shows.
    @MainActor func pulse(_ view: NSView) {
        view.wantsLayer = true
        let animation = CABasicAnimation(keyPath: "opacity")
        animation.fromValue = 1
        animation.toValue = 0.3
        animation.duration = 0.8
        animation.autoreverses = true
        animation.repeatCount = .infinity
        view.layer?.add(animation, forKey: "pulse")
    }

    /// A position, as "1:02:03" or "2:03".
    func clock(_ seconds: TimeInterval) -> String {
        let s = Int(max(0, seconds))
        return s >= 3600
            ? String(format: "%d:%02d:%02d", s / 3600, s / 60 % 60, s % 60) : String(format: "%d:%02d", s / 60, s % 60)
    }

    /// What an item is of, under its name: an episode's series and numbers,
    /// a season's series, music's artists.
    private func subtitleOf(_ item: Mavio_Library_V1_Item) -> String? {
        switch item.kind {
        case .episode:
            let n =
                item.hasParentIndexNumber && item.hasIndexNumber
                ? "S\(item.parentIndexNumber):E\(item.indexNumber)" : item.hasIndexNumber ? "E\(item.indexNumber)" : ""
            let s = [item.seriesName, n].filter { !$0.isEmpty }.joined(separator: " · ")
            return s.isEmpty ? nil : s
        case .season: return item.seriesName.isEmpty ? nil : item.seriesName
        case .musicAlbum, .track:
            let s = (item.albumArtists.isEmpty ? item.artists : item.albumArtists).joined(separator: ", ")
            return s.isEmpty ? nil : s
        default: return nil
        }
    }

    /// Activity: who plays what on the server, and what happened on it
    /// lately, at a severity or above.
    final class ActivityPage: ServerPage {
        private var sessions: [Mavio_Session_V1_Session]?
        private var sessionsFailure: String?
        private var activities: Mavio_System_V1_ListActivitiesResponse?
        private var failure: String?
        private var minimum = Mavio_System_V1_Severity.unspecified
        private var limit: Int32 = 20
        private lazy var form = FormPage(theme: theme)
        private lazy var playingSection = section(MavioStrings.activityNowPlaying)
        private lazy var recentSection = section(MavioStrings.activityRecent)

        override func viewDidLoad() {
            super.viewDidLoad()
            title = MavioStrings.settingsActivity
            let options: [(String, Mavio_System_V1_Severity)] = [
                (MavioStrings.activityActivityAll, .unspecified),
                (MavioStrings.activityActivityWarnings, .warning),
                (MavioStrings.activityActivityErrors, .error),
            ]
            let popup = ActionPopUp(options: options.map(\.0), selected: 0) { [weak self] i in
                guard let self else { return }
                minimum = options[i].1
                pollActivities()
            }
            popup.setAccessibilityLabel(MavioStrings.activityActivityShown)
            popup.toolTip = MavioStrings.activityActivityShown
            form.add(heading(MavioStrings.settingsActivity, actions: [FocusFrame(popup, theme: theme)]))
            form.add(playingSection)
            form.add(recentSection)
            renderPlaying()
            renderRecent()
            show(form)
        }

        override func appeared() {
            poll("sessions", every: { .seconds(5) }) { [weak self] in await self?.loadSessions() }
            pollActivities()
        }

        private func pollActivities() {
            poll("activities", every: { .seconds(30) }) { [weak self] in await self?.loadActivities() }
        }

        private func renderPlaying() {
            let playing = (sessions ?? []).filter { $0.hasNowPlaying && $0.nowPlaying.hasItem }
            var rows: [NSView] = playing.map(playingRow)
            if let sessionsFailure {
                rows.append(note(sessionsFailure, failed: true))
            } else if sessions != nil && playing.isEmpty {
                rows.append(note(MavioStrings.activityNothingPlaying))
            } else if sessions == nil {
                rows.append(padded(LoadingView()))
            }
            playingSection.setRows(rows)
            refocus()
        }

        private func renderRecent() {
            var rows: [NSView] = (activities?.activities ?? []).map(activityRow)
            if let failure {
                rows.append(note(failure, failed: true))
            } else if let activities, activities.activities.isEmpty {
                rows.append(note(MavioStrings.activityNoActivity))
            } else if activities == nil {
                rows.append(padded(LoadingView()))
            }
            if let activities, activities.total > activities.activities.count {
                rows.append(
                    FormRow.action(MavioStrings.activityShowMore, theme: theme) { [weak self] in
                        guard let self else { return }
                        limit += 50
                        pollActivities()
                    })
            }
            recentSection.setRows(rows)
            refocus()
        }

        /// What a session plays: its item, who plays it where, how, and how
        /// far.
        private func playingRow(_ session: Mavio_Session_V1_Session) -> NSView {
            let np = session.nowPlaying
            let item = np.item
            let runtime = item.hasRuntime ? item.runtime.timeInterval : 0
            let position = np.hasPosition ? np.position.timeInterval : 0
            let tile = Box(fill: NSColor(hex: np.paused ? 0x8E8E93 : 0x34C759), radius: 7)
            tile.size(width: 28, height: 28)
            let icon = IconView(np.paused ? "pause" : "play", fill: true, size: 17, color: .white)
            tile.addSubview(icon)
            icon.center(in: tile)
            let name = NSTextField.label(
                [item.name, subtitleOf(item)].compactMap(\.self).joined(separator: " · "), theme.fonts.body,
                color: theme.colors.textPrimary)
            name.lineBreakMode = .byTruncatingTail
            let who = NSTextField.label(
                [session.userName.isEmpty ? session.pluginID : session.userName, session.deviceName, session.client]
                    .filter { !$0.isEmpty }.joined(separator: " · "), theme.fonts.footnote,
                color: theme.colors.textSecondary)
            who.lineBreakMode = .byTruncatingTail
            var head: [NSView] = [tile, NSStackView.vertical([name, who], spacing: 2), Spacer()]
            if let method = Self.method(np.method) {
                head.append(NSTextField.label(method, theme.fonts.footnote, color: theme.colors.textSecondary))
            }
            let top = NSStackView.horizontal(head, spacing: MavioSpacing.s12)
            var lines: [NSView] = [top]
            if runtime > 0 {
                let bar = ShareBar(min(1, position / runtime), color: theme.colors.accent, track: theme.colors.fill)
                let time = NSTextField.label(
                    "\(clock(position)) / \(clock(runtime))", theme.fonts.caption, color: theme.colors.textSecondary)
                time.font = .monospacedDigitSystemFont(ofSize: theme.fonts.caption.size, weight: .regular)
                let progress = NSStackView.horizontal([bar, time], spacing: MavioSpacing.s12)
                let inset = NSView()
                inset.translatesAutoresizingMaskIntoConstraints = false
                inset.addFilling(progress, insets: NSEdgeInsets(top: 0, left: 40, bottom: 0, right: 0))
                lines.append(inset)
            }
            let stack = NSStackView.vertical(lines, spacing: MavioSpacing.s8)
            for line in lines { line.widthAnchor.constraint(equalTo: stack.widthAnchor).isActive = true }
            return padded(stack)
        }

        private static func method(_ m: Mavio_Playback_V1_PlayMethod) -> String? {
            switch m {
            case .directPlay: MavioStrings.activityDirectPlay
            case .directStream: MavioStrings.activityDirectStream
            case .transcode: MavioStrings.activityTranscode
            default: nil
            }
        }

        private func activityRow(_ activity: Mavio_System_V1_Activity) -> NSView {
            let (color, severity): (NSColor, String) =
                switch activity.severity {
                case .warning: (NSColor(hex: 0xFF9500), MavioStrings.activitySeverityWarning)
                case .error: (NSColor(hex: 0xFF3B30), MavioStrings.activitySeverityError)
                default: (theme.colors.textTertiary, MavioStrings.activitySeverityInfo)
                }
            let mark = dot(color)
            mark.setAccessibilityElement(true)
            mark.setAccessibilityLabel(severity)
            var lines: [NSView] = [
                NSTextField.label(activity.title, theme.fonts.body, color: theme.colors.textPrimary, lines: 0)
            ]
            if !activity.message.isEmpty {
                lines.append(
                    NSTextField.label(
                        activity.message, theme.fonts.footnote, color: theme.colors.textSecondary, lines: 0,
                        selectable: true))
            }
            var views: [NSView] = [mark, NSStackView.vertical(lines, spacing: 2), Spacer()]
            if let time = activity.time.time {
                views.append(NSTextField.label(fromNow(time), theme.fonts.footnote, color: theme.colors.textSecondary))
            }
            return padded(NSStackView.horizontal(views, spacing: MavioSpacing.s12, alignment: .firstBaseline))
        }

        private func loadSessions() async {
            var request = Mavio_Session_V1_ListSessionsRequest()
            request.allUsers = true
            do {
                sessions = try await model.call(
                    server, Procedure.sessions("ListSessions"), request,
                    as: Mavio_Session_V1_ListSessionsResponse.self
                ).sessions
                sessionsFailure = nil
            } catch is CancellationError {
                return
            } catch {
                sessionsFailure = error.reason
            }
            renderPlaying()
        }

        private func loadActivities() async {
            var request = Mavio_System_V1_ListActivitiesRequest()
            request.minSeverity = minimum
            request.limit = limit
            do {
                activities = try await model.call(
                    server, Procedure.activity("ListActivities"), request,
                    as: Mavio_System_V1_ListActivitiesResponse.self)
                failure = nil
            } catch is CancellationError {
                return
            } catch {
                failure = error.reason
            }
            renderRecent()
        }
    }
#endif
