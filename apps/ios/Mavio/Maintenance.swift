import MavioKit
import MavioUI
import SwiftUI

// The settings' sections of watching over and keeping a server: Activity,
// Issues, Tasks, Logs and Backups (docs/clients.md §6.3.6–§6.3.7,
// §6.3.14, §6.3.25).

/// Calls action again and again, interval apart, while the view shows.
private struct Polling: ViewModifier {
    let interval: () -> Duration
    let action: @MainActor () async -> Void

    func body(content: Content) -> some View {
        content.task {
            while !Task.isCancelled {
                await action()
                try? await Task.sleep(for: interval())
            }
        }
    }
}

extension View {
    fileprivate func polling(
        every interval: @autoclosure @escaping () -> Duration, _ action: @escaping @MainActor () async -> Void
    )
        -> some View
    {
        modifier(Polling(interval: interval, action: action))
    }

    /// A settings page's list: on the background, with an inline title.
    fileprivate func settingsPage(_ title: String) -> some View {
        scrollContentBackground(.hidden)
            .navigationTitle(title)
            .navigationBarTitleDisplayMode(.inline)
    }
}

/// A row saying a list is empty, or why it could not be read.
private struct Note: View {
    let text: String
    var failed = false
    @Environment(\.theme) private var theme

    var body: some View {
        Text(text).foregroundStyle(failed ? theme.colors.danger : theme.colors.textSecondary)
    }
}

/// A colored dot, of a severity or state.
private struct Dot: View {
    let color: Color
    var pulsing = false

    var body: some View {
        Circle().fill(color).frame(width: 8, height: 8)
            .symbolEffect(.pulse, isActive: pulsing)
            .opacity(pulsing ? 0.8 : 1)
    }
}

/// A "Show More" row.
private struct ShowMore: View {
    let title: String
    let action: () -> Void

    var body: some View {
        Button(title, action: action)
    }
}

// MARK: Activity

/// Activity: who plays what on the server, and what happened on it lately.
struct ActivityView: View {
    let server: ServerRecord
    @Environment(AppModel.self) private var model
    @Environment(\.theme) private var theme
    @State private var sessions: [Mavio_Session_V1_Session]?
    @State private var sessionsFailure: String?
    @State private var activities: Mavio_System_V1_ListActivitiesResponse?
    @State private var failure: String?
    @State private var minimum = Mavio_System_V1_Severity.unspecified
    @State private var limit: Int32 = 20

    var body: some View {
        List {
            Section(MavioStrings.activityNowPlaying) {
                let playing = (sessions ?? []).filter { $0.hasNowPlaying && $0.nowPlaying.hasItem }
                ForEach(playing, id: \.id) { PlayingRow(session: $0) }
                if let sessionsFailure {
                    Note(text: sessionsFailure, failed: true)
                } else if sessions != nil && playing.isEmpty {
                    Note(text: MavioStrings.activityNothingPlaying)
                }
            }
            Section {
                ForEach(activities?.activities ?? [], id: \.id) { ActivityRow(activity: $0) }
                if let failure {
                    Note(text: failure, failed: true)
                } else if let activities, activities.activities.isEmpty {
                    Note(text: MavioStrings.activityNoActivity)
                }
                if let activities, activities.total > activities.activities.count {
                    ShowMore(title: MavioStrings.activityShowMore) { limit += 50 }
                }
            } header: {
                Text(MavioStrings.activityRecent)
            }
        }
        .settingsPage(MavioStrings.settingsActivity)
        .background(theme.colors.background)
        .toolbar {
            ToolbarItem(placement: .topBarTrailing) {
                Menu {
                    Picker(MavioStrings.activityActivityShown, selection: $minimum) {
                        Text(MavioStrings.activityActivityAll).tag(Mavio_System_V1_Severity.unspecified)
                        Text(MavioStrings.activityActivityWarnings).tag(Mavio_System_V1_Severity.warning)
                        Text(MavioStrings.activityActivityErrors).tag(Mavio_System_V1_Severity.error)
                    }
                } label: {
                    Icon("filter")
                }
                .accessibilityLabel(MavioStrings.activityActivityShown)
            }
        }
        .polling(every: .seconds(5)) { await loadSessions() }
        .task(id: "\(minimum.rawValue)-\(limit)") {
            while !Task.isCancelled {
                await loadActivities()
                try? await Task.sleep(for: .seconds(30))
            }
        }
        .pullToRefresh {
            await loadSessions()
            await loadActivities()
        }
    }

    private func loadSessions() async {
        var request = Mavio_Session_V1_ListSessionsRequest()
        request.allUsers = true
        do {
            sessions = try await model.call(
                server, Procedure.sessions("ListSessions"), request, as: Mavio_Session_V1_ListSessionsResponse.self
            ).sessions
            sessionsFailure = nil
        } catch is CancellationError {
        } catch {
            sessionsFailure = error.reason
        }
    }

    private func loadActivities() async {
        var request = Mavio_System_V1_ListActivitiesRequest()
        request.minSeverity = minimum
        request.limit = limit
        do {
            activities = try await model.call(
                server, Procedure.activity("ListActivities"), request, as: Mavio_System_V1_ListActivitiesResponse.self)
            failure = nil
        } catch is CancellationError {
        } catch {
            failure = error.reason
        }
    }
}

/// What a session plays: its item, who plays it where, how, and how far.
private struct PlayingRow: View {
    let session: Mavio_Session_V1_Session
    @Environment(\.theme) private var theme

    var body: some View {
        let np = session.nowPlaying
        let item = np.item
        let runtime = item.hasRuntime ? item.runtime.timeInterval : 0
        let position = np.hasPosition ? np.position.timeInterval : 0
        VStack(alignment: .leading, spacing: MavioSpacing.s8) {
            HStack(spacing: MavioSpacing.s12) {
                Icon(np.paused ? "pause" : "play", fill: true, size: 17)
                    .foregroundStyle(.white)
                    .frame(width: 28, height: 28)
                    .background(np.paused ? Color(hex: 0x8E8E93) : Color(hex: 0x34C759), in: .rect(cornerRadius: 7))
                VStack(alignment: .leading, spacing: 2) {
                    Text([item.name, subtitleOf(item)].compactMap(\.self).joined(separator: " · "))
                        .lineLimit(1)
                    Text(
                        [
                            session.userName.isEmpty ? session.pluginID : session.userName, session.deviceName,
                            session.client,
                        ]
                        .filter { !$0.isEmpty }.joined(separator: " · ")
                    )
                    .type(theme.type.footnote).foregroundStyle(theme.colors.textSecondary).lineLimit(1)
                }
                Spacer(minLength: 0)
                if let method = Self.method(np.method) {
                    Text(method).type(theme.type.footnote).foregroundStyle(theme.colors.textSecondary)
                }
            }
            if runtime > 0 {
                HStack(spacing: MavioSpacing.s12) {
                    ProgressView(value: min(1, position / runtime))
                        .tint(theme.colors.accent)
                    Text("\(clock(position)) / \(clock(runtime))")
                        .type(theme.type.caption).monospacedDigit()
                        .foregroundStyle(theme.colors.textSecondary)
                }
                .padding(.leading, 40)
            }
        }
        .padding(.vertical, 2)
    }

    private static func method(_ m: Mavio_Playback_V1_PlayMethod) -> String? {
        switch m {
        case .directPlay: MavioStrings.activityDirectPlay
        case .directStream: MavioStrings.activityDirectStream
        case .transcode: MavioStrings.activityTranscode
        default: nil
        }
    }
}

/// What an item is of, under its name: an episode's series and numbers, a
/// season's series, music's artists.
func subtitleOf(_ item: Mavio_Library_V1_Item) -> String? {
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

/// A position, as "1:02:03" or "2:03".
private func clock(_ seconds: TimeInterval) -> String {
    let s = Int(max(0, seconds))
    return s >= 3600
        ? String(format: "%d:%02d:%02d", s / 3600, s / 60 % 60, s % 60) : String(format: "%d:%02d", s / 60, s % 60)
}

private struct ActivityRow: View {
    let activity: Mavio_System_V1_Activity
    @Environment(\.theme) private var theme

    var body: some View {
        HStack(alignment: .firstTextBaseline, spacing: MavioSpacing.s12) {
            Dot(color: color).accessibilityLabel(severity)
            VStack(alignment: .leading, spacing: 2) {
                Text(activity.title)
                if !activity.message.isEmpty {
                    Text(activity.message).type(theme.type.footnote).foregroundStyle(theme.colors.textSecondary)
                }
            }
            Spacer(minLength: 0)
            if let time = activity.time.time {
                Text(fromNow(time)).type(theme.type.footnote).foregroundStyle(theme.colors.textSecondary)
            }
        }
    }

    private var color: Color {
        switch activity.severity {
        case .warning: Color(hex: 0xFF9500)
        case .error: Color(hex: 0xFF3B30)
        default: theme.colors.textTertiary
        }
    }

    private var severity: String {
        switch activity.severity {
        case .warning: MavioStrings.activitySeverityWarning
        case .error: MavioStrings.activitySeverityError
        default: MavioStrings.activitySeverityInfo
        }
    }
}

// MARK: Tasks

extension Mavio_System_V1_JobState {
    fileprivate var color: Color {
        switch self {
        case .running: Color(hex: 0x007AFF)
        case .succeeded: Color(hex: 0x34C759)
        case .failed: Color(hex: 0xFF3B30)
        default: Color(hex: 0x8E8E93)
        }
    }

    fileprivate var title: String {
        switch self {
        case .pending: MavioStrings.tasksPending
        case .running: MavioStrings.tasksRunning
        case .succeeded: MavioStrings.tasksSucceeded
        case .failed: MavioStrings.tasksFailed
        default: ""
        }
    }
}

/// Tasks: the server's recurring tasks, its own and its plugins', with how
/// often they run, how their last run went and when they run next, to run
/// now; and the jobs of its queue, filtered by state.
struct TasksView: View {
    let server: ServerRecord
    @Environment(AppModel.self) private var model
    @Environment(\.theme) private var theme
    @State private var tasks: [Mavio_System_V1_Task]?
    @State private var failure: String?
    @State private var jobs: Mavio_System_V1_ListJobsResponse?
    @State private var jobsFailure: String?
    @State private var filter = JobFilter.all
    @State private var limit: Int32 = 50
    /// A notice of running a task, kept until its next run finishes: by task, the notice and the run it followed.
    @State private var notices: [String: (text: String, run: String)] = [:]
    @State private var starting: String?

    enum JobFilter: CaseIterable {
        case all, active, failed, succeeded

        var states: [Mavio_System_V1_JobState] {
            switch self {
            case .all: []
            case .active: [.pending, .running]
            case .failed: [.failed]
            case .succeeded: [.succeeded]
            }
        }

        var title: String {
            switch self {
            case .all: MavioStrings.tasksJobsAll
            case .active: MavioStrings.tasksJobsActive
            case .failed: MavioStrings.tasksJobsFailed
            case .succeeded: MavioStrings.tasksJobsSucceeded
            }
        }
    }

    var body: some View {
        let byName = { (a: Mavio_System_V1_Task, b: Mavio_System_V1_Task) in
            a.name.localizedStandardCompare(b.name) == .orderedAscending
        }
        let own = (tasks ?? []).filter { $0.pluginID.isEmpty && Self.shown($0.kind) }.sorted(by: byName)
        let shownJobs = (jobs?.jobs ?? []).filter { Self.shown($0.kind) }
        List {
            if let failure {
                Note(text: failure, failed: true)
            }
            if !own.isEmpty {
                Section(MavioStrings.tasksServer) { ForEach(own, id: \.id, content: taskRow) }
            }
            Section {
                ForEach(shownJobs, id: \.id) { JobRow(job: $0) }
                if let jobsFailure {
                    Note(text: jobsFailure, failed: true)
                } else if jobs != nil, shownJobs.isEmpty {
                    Note(text: MavioStrings.tasksNoJobs)
                }
                if let jobs, jobs.total > jobs.jobs.count {
                    ShowMore(title: MavioStrings.tasksShowMore) { limit += 100 }
                }
            } header: {
                HStack {
                    Text(MavioStrings.tasksJobs)
                    Spacer()
                    Picker(MavioStrings.tasksJobsShown, selection: $filter) {
                        ForEach(JobFilter.allCases, id: \.self) { Text($0.title).tag($0) }
                    }
                    .textCase(nil)
                }
            } footer: {
                Text(MavioStrings.tasksJobsFooter)
            }
        }
        .settingsPage(MavioStrings.settingsTasks)
        .background(theme.colors.background)
        // Often while a task runs.
        .polling(every: tasks?.contains(where: \.running) == true ? .seconds(2) : .seconds(15)) { await loadTasks() }
        .task(id: "\(filter)-\(limit)") {
            while !Task.isCancelled {
                await loadJobs()
                try? await Task.sleep(for: .seconds(5))
            }
        }
        .pullToRefresh {
            await loadTasks()
            await loadJobs()
        }
    }

    /// Whether a task or job of a kind shows: those of acquiring media and
    /// of plugins, which the app leaves out, do not (docs/clients.md §4.2).
    private static func shown(_ kind: String) -> Bool {
        !["acquisition.", "requests.", "subtitles.", "plugin."].contains { kind.hasPrefix($0) }
    }

    private func taskRow(_ task: Mavio_System_V1_Task) -> some View {
        let last = task.hasLastRun ? task.lastRun : nil
        let status =
            task.running
            ? MavioStrings.tasksRunningNow
            : last.map { l in
                [l.state.title, (l.finishTime.time ?? l.runTime.time).map(fromNow) ?? ""].filter { !$0.isEmpty }
                    .joined(separator: " ")
            } ?? MavioStrings.tasksNeverRun
        let notice = notices[task.id].flatMap { $0.run == (last?.id ?? "") ? $0.text : nil }
        let every =
            task.hasInterval
            ? MavioStrings.tasksEvery(length: lengthOf(task.interval.timeInterval)) : MavioStrings.tasksOnDemand
        let next = !task.running ? task.nextRunTime.time.map { MavioStrings.tasksNext(when: fromNow($0)) } : nil
        return HStack(spacing: MavioSpacing.s12) {
            VStack(alignment: .leading, spacing: 2) {
                Text(task.name)
                if !task.description_p.isEmpty {
                    Text(task.description_p).type(theme.type.footnote).foregroundStyle(theme.colors.textSecondary)
                }
                HStack(spacing: MavioSpacing.s4) {
                    Dot(
                        color: task.running
                            ? Mavio_System_V1_JobState.running.color : last?.state.color ?? theme.colors.fill,
                        pulsing: task.running)
                    Text(
                        ([task.running ? status : (notice ?? status), every] + [next].compactMap(\.self)).joined(
                            separator: " · "))
                }
                .type(theme.type.footnote).foregroundStyle(theme.colors.textSecondary)
                if let last, last.state == .failed, !last.lastError.isEmpty, !task.running {
                    Text(last.lastError).type(theme.type.footnote).foregroundStyle(theme.colors.danger)
                }
            }
            Spacer(minLength: 0)
            Button(starting == task.id ? "…" : MavioStrings.tasksRun) { run(task) }
                .buttonStyle(.bordered)
                .disabled(starting == task.id || task.running)
        }
    }

    private func run(_ task: Mavio_System_V1_Task) {
        let run = task.hasLastRun ? task.lastRun.id : ""
        starting = task.id
        Task {
            var request = Mavio_System_V1_RunTaskRequest()
            request.id = task.id
            do {
                let r = try await model.call(
                    server, Procedure.tasks("RunTask"), request, as: Mavio_System_V1_RunTaskResponse.self)
                notices[task.id] = (r.enqueued ? MavioStrings.tasksQueued : MavioStrings.tasksAlreadyQueued, run)
            } catch {
                notices[task.id] = (error.reason, run)
            }
            starting = nil
            await loadTasks()
            await loadJobs()
        }
    }

    private func loadTasks() async {
        do {
            tasks = try await model.call(
                server, Procedure.tasks("ListTasks"), Mavio_System_V1_ListTasksRequest(),
                as: Mavio_System_V1_ListTasksResponse.self
            ).tasks
            failure = nil
        } catch is CancellationError {
        } catch {
            failure = error.reason
        }
    }

    private func loadJobs() async {
        var request = Mavio_System_V1_ListJobsRequest()
        request.states = filter.states
        request.limit = limit
        do {
            jobs = try await model.call(
                server, Procedure.tasks("ListJobs"), request, as: Mavio_System_V1_ListJobsResponse.self)
            jobsFailure = nil
        } catch is CancellationError {
        } catch {
            jobsFailure = error.reason
        }
    }
}

private struct JobRow: View {
    let job: Mavio_System_V1_Job
    @Environment(\.theme) private var theme

    var body: some View {
        let when: String? =
            job.state == .pending
            ? job.runTime.time.map { MavioStrings.tasksRuns(when: fromNow($0)) }
            : (job.finishTime.time ?? job.runTime.time ?? job.createTime.time).map(fromNow)
        VStack(alignment: .leading, spacing: 2) {
            Text(job.kind)
            HStack(spacing: MavioSpacing.s4) {
                Dot(color: job.state.color)
                Text(
                    ([job.state.title]
                        + [
                            job.hasProgress ? "\(Int(job.progress * 100))%" : nil, when,
                            job.attempts > 1
                                ? MavioStrings.tasksAttempts(attempts: "\(job.attempts)", max: "\(job.maxAttempts)")
                                : nil,
                        ].compactMap(\.self)).joined(separator: " · "))
            }
            .type(theme.type.footnote).foregroundStyle(theme.colors.textSecondary)
            if !job.lastError.isEmpty {
                Text(job.lastError).type(theme.type.footnote).foregroundStyle(theme.colors.danger)
            }
        }
    }
}

// MARK: Logs

/// Logs: the server's recent log records at a level or above, from a
/// period, filtered by words.
struct LogsView: View {
    let server: ServerRecord
    @Environment(AppModel.self) private var model
    @Environment(\.theme) private var theme
    @State private var records: [Mavio_System_V1_LogRecord]?
    @State private var failure: String?
    @State private var level = Mavio_System_V1_LogLevel.info
    @State private var hours = 24
    @State private var filter = ""

    private static let levels: [Mavio_System_V1_LogLevel] = [.debug, .info, .warn, .error]

    var body: some View {
        let words = filter.lowercased().split(whereSeparator: \.isWhitespace)
        let shown = (records ?? []).filter { r in
            let text = ([r.message] + r.attributes.flatMap { [$0.key, $0.value] }).joined(separator: " ").lowercased()
            return words.allSatisfy { text.contains($0) }
        }
        List {
            Section {
                ForEach(Array(shown.enumerated()), id: \.offset) { LogRow(record: $0.element) }
                if let failure {
                    Note(text: failure, failed: true)
                } else if records != nil && shown.isEmpty {
                    Note(text: MavioStrings.logsNone)
                }
            } footer: {
                if records != nil { Text(MavioStrings.logsCount(count: shown.count)) }
            }
        }
        .settingsPage(MavioStrings.settingsLogs)
        .background(theme.colors.background)
        .searchable(text: $filter, prompt: MavioStrings.logsFilter)
        .toolbar {
            ToolbarItem(placement: .topBarTrailing) {
                Menu {
                    Picker(MavioStrings.logsLevel, selection: $level) {
                        ForEach(Self.levels, id: \.self) { Text(Self.name($0)).tag($0) }
                    }
                    .pickerStyle(.menu)
                    Picker(MavioStrings.logsPeriod, selection: $hours) {
                        Text(MavioStrings.logsLastHour).tag(1)
                        Text(MavioStrings.logsLastDay).tag(24)
                        Text(MavioStrings.logsAll).tag(0)
                    }
                    .pickerStyle(.menu)
                } label: {
                    Icon("filter")
                }
                .accessibilityLabel(MavioStrings.logsLevel)
            }
        }
        .overlay { if records == nil && failure == nil { ProgressView() } }
        .task(id: "\(level.rawValue)-\(hours)") { await load() }
        .pullToRefresh { await load() }
    }

    static func name(_ level: Mavio_System_V1_LogLevel) -> String {
        switch level {
        case .debug: MavioStrings.logsDebug
        case .warn: MavioStrings.logsWarn
        case .error: MavioStrings.logsError
        default: MavioStrings.logsInfo
        }
    }

    private func load() async {
        var request = Mavio_System_V1_ListLogsRequest()
        request.minLevel = level
        request.limit = 1000
        if hours > 0 {
            request.maxAge = Google_Protobuf_Duration(seconds: Int64(hours * 3600))
        }
        do {
            records = try await model.call(
                server, Procedure.system("ListLogs"), request, as: Mavio_System_V1_ListLogsResponse.self
            ).records
            failure = nil
        } catch is CancellationError {
        } catch {
            failure = error.reason
        }
    }
}

private struct LogRow: View {
    let record: Mavio_System_V1_LogRecord
    @Environment(\.theme) private var theme

    var body: some View {
        VStack(alignment: .leading, spacing: 2) {
            HStack(spacing: MavioSpacing.s8) {
                if let time = record.time.time {
                    Text(time.formatted(date: .numeric, time: .standard)).monospacedDigit()
                        .foregroundStyle(theme.colors.textSecondary)
                }
                Text(LogsView.name(record.level)).fontWeight(.semibold).foregroundStyle(color)
            }
            .type(theme.type.footnote)
            Text(record.message).type(theme.type.callout)
            let attrs = record.attributes.sorted { $0.key < $1.key }
            if !attrs.isEmpty {
                Text(attrs.map { "\($0.key)=\($0.value)" }.joined(separator: "  "))
                    .type(theme.type.caption).monospaced()
                    .foregroundStyle(theme.colors.textSecondary)
            }
        }
        .textSelection(.enabled)
    }

    private var color: Color {
        switch record.level {
        case .debug: theme.colors.textTertiary
        case .warn: Color(hex: 0xFF9F0A)
        case .error: theme.colors.danger
        default: theme.colors.textSecondary
        }
    }
}

// MARK: Backups

/// Backups: the server's backups, made, saved to the device's files and
/// deleted here.
struct BackupsView: View {
    let server: ServerRecord
    @Environment(AppModel.self) private var model
    @Environment(\.theme) private var theme
    @State private var backups: [Mavio_System_V1_Backup]?
    @State private var failure: String?
    @State private var creating = false
    @State private var createFailure: String?
    @State private var removing: Mavio_System_V1_Backup?
    @State private var confirming = false
    @State private var downloading: String?
    /// A backup's file downloaded, being saved where the user chooses.
    @State private var saving: URL?

    var body: some View {
        List {
            Section {
                HStack(spacing: MavioSpacing.s12) {
                    VStack(alignment: .leading, spacing: 2) {
                        Text(MavioStrings.backupsCreate)
                        Text(creating ? MavioStrings.backupsCreating : MavioStrings.backupsCreateDetail)
                            .type(theme.type.footnote).foregroundStyle(theme.colors.textSecondary)
                    }
                    Spacer(minLength: 0)
                    Button(creating ? "…" : MavioStrings.backupsCreateAction, action: create)
                        .buttonStyle(.borderedProminent)
                        .disabled(creating)
                }
                if let createFailure {
                    Note(text: createFailure, failed: true)
                }
            } footer: {
                Text(MavioStrings.backupsRestoreFooter)
            }
            Section(MavioStrings.backupsList) {
                ForEach(backups ?? [], id: \.name) { b in
                    HStack(spacing: MavioSpacing.s12) {
                        VStack(alignment: .leading, spacing: 2) {
                            Text(b.name)
                            Text(
                                [
                                    b.createTime.time.map(fromNow),
                                    b.sizeBytes.formatted(.byteCount(style: .file)),
                                    b.version.isEmpty ? nil : MavioStrings.backupsVersion(version: b.version),
                                ].compactMap(\.self).joined(separator: " · ")
                            )
                            .type(theme.type.footnote).foregroundStyle(theme.colors.textSecondary)
                        }
                        Spacer(minLength: 0)
                        Button {
                            download(b)
                        } label: {
                            if downloading == b.name {
                                ProgressView()
                            } else {
                                Icon("downloads")
                            }
                        }
                        .buttonStyle(.borderless)
                        .disabled(downloading != nil)
                        .accessibilityLabel(MavioStrings.backupsDownload)
                    }
                    .swipeActions {
                        // Not destructive: the row stays until deleting it is confirmed.
                        Button(MavioStrings.backupsRemove) { confirm(b) }
                            .tint(theme.colors.danger)
                    }
                    .contextMenu {
                        Button(MavioStrings.backupsDownload) { download(b) }
                        Button(MavioStrings.backupsRemove, role: .destructive) { confirm(b) }
                    }
                }
                if let failure {
                    Note(text: failure, failed: true)
                } else if let backups, backups.isEmpty {
                    Note(text: MavioStrings.backupsNone)
                }
            }
        }
        .settingsPage(MavioStrings.settingsBackups)
        .background(theme.colors.background)
        .confirmationDialog(
            MavioStrings.backupsRemoveTitle(name: removing?.name ?? ""), isPresented: $confirming,
            titleVisibility: .visible
        ) {
            Button(MavioStrings.backupsRemove, role: .destructive) {
                if let removing { remove(removing) }
            }
        } message: {
            Text(MavioStrings.backupsRemoveMessage)
        }
        .fileMover(isPresented: Binding(get: { saving != nil }, set: { if !$0 { saving = nil } }), file: saving) { _ in
            saving = nil
        }
        .task { await load() }
        .pullToRefresh { await load() }
    }

    private func load() async {
        do {
            backups = try await model.call(
                server, Procedure.backups("ListBackups"), Mavio_System_V1_ListBackupsRequest(),
                as: Mavio_System_V1_ListBackupsResponse.self
            ).backups
            failure = nil
        } catch is CancellationError {
        } catch {
            failure = error.reason
        }
    }

    private func create() {
        creating = true
        Task {
            do {
                _ = try await model.call(
                    server, Procedure.backups("CreateBackup"), Mavio_System_V1_CreateBackupRequest(),
                    as: Mavio_System_V1_CreateBackupResponse.self)
                createFailure = nil
            } catch {
                createFailure = error.reason
            }
            creating = false
            await load()
        }
    }

    private func confirm(_ b: Mavio_System_V1_Backup) {
        removing = b
        confirming = true
    }

    private func remove(_ b: Mavio_System_V1_Backup) {
        Task {
            var request = Mavio_System_V1_DeleteBackupRequest()
            request.name = b.name
            do {
                _ = try await model.call(
                    server, Procedure.backups("DeleteBackup"), request, as: Mavio_System_V1_DeleteBackupResponse.self)
                await load()
            } catch {
                createFailure = error.reason
            }
        }
    }

    /// Downloads a backup's file with the session's access token, then
    /// asks where to save it.
    private func download(_ b: Mavio_System_V1_Backup) {
        downloading = b.name
        Task {
            defer { downloading = nil }
            do {
                let target = try await model.client.serverRequest(
                    serverId: server.id, path: "system/backups/\(b.name)")
                guard let url = URL(string: target.url) else { return }
                var request = URLRequest(url: url)
                request.setValue("Bearer \(target.token)", forHTTPHeaderField: "Authorization")
                let (file, response) = try await URLSession.shared.download(for: request)
                if let http = response as? HTTPURLResponse, !(200..<300).contains(http.statusCode) {
                    throw URLError(.badServerResponse)
                }
                let named = FileManager.default.temporaryDirectory.appending(path: b.name)
                try? FileManager.default.removeItem(at: named)
                try FileManager.default.moveItem(at: file, to: named)
                saving = named
            } catch {
                failure = error.reason
            }
        }
    }
}

// MARK: Issues

extension Mavio_Acquisition_V1_IssueType {
    var title: String {
        switch self {
        case .video: MavioStrings.issuesVideo
        case .audio: MavioStrings.issuesAudio
        case .subtitles: MavioStrings.issuesSubtitles
        case .other: MavioStrings.issuesOther
        default: ""
        }
    }
}

extension Mavio_Acquisition_V1_Issue {
    /// The season or episode of a series it is about.
    var numbers: String? {
        guard seasonNumber > 0 else { return nil }
        return episodeNumber > 0
            ? String(format: "S%02dE%02d", seasonNumber, episodeNumber)
            : MavioStrings.discoverSeason(number: "\(seasonNumber)")
    }
}

/// The names of items and users issues name, read once.
@MainActor @Observable
final class IssueNames {
    var items: [String: String] = [:]
    var users: [String: String] = [:]
    private var asked: Set<String> = []

    func item(_ id: String, _ model: AppModel, _ server: ServerRecord) async {
        guard !id.isEmpty, asked.insert(id).inserted else { return }
        var request = Mavio_Library_V1_GetItemRequest()
        request.id = id
        if let r = try? await model.call(
            server, Procedure.items("GetItem"), request, as: Mavio_Library_V1_GetItemResponse.self)
        {
            items[id] = r.item.name
        }
    }

    /// Everyone's names for an administrator.
    func loadUsers(_ model: AppModel, _ server: ServerRecord) async {
        guard asked.insert("users").inserted else { return }
        if let r = try? await model.call(
            server, Procedure.users("ListUsers"), Mavio_User_V1_ListUsersRequest(),
            as: Mavio_User_V1_ListUsersResponse.self)
        {
            users = Dictionary(r.users.map { ($0.id, $0.name) }, uniquingKeysWith: { a, _ in a })
        }
    }

    /// A user's name: the user's own, everyone's for an administrator, else "Administrator".
    func user(_ id: String, _ server: ServerRecord) -> String {
        id == server.userId ? server.userName : users[id] ?? MavioStrings.issuesAdministrator
    }
}

/// Issues: the problems reported with the server's items — the user's own,
/// or everyone's for its administrators — open ones at first.
struct IssuesView: View {
    let server: ServerRecord
    @Environment(AppModel.self) private var model
    @Environment(\.theme) private var theme
    @State private var issues: [Mavio_Acquisition_V1_Issue]?
    @State private var total: Int32 = 0
    @State private var failure: String?
    @State private var filter = Filter.open
    @State private var names = IssueNames()

    enum Filter: CaseIterable {
        case open, resolved, all

        var statuses: [Mavio_Acquisition_V1_IssueStatus] {
            switch self {
            case .open: [.open]
            case .resolved: [.resolved]
            case .all: []
            }
        }

        var title: String {
            switch self {
            case .open: MavioStrings.issuesOpen
            case .resolved: MavioStrings.issuesResolved
            case .all: MavioStrings.issuesAll
            }
        }
    }

    private static let page: Int32 = 50

    var body: some View {
        List {
            Section {
                Picker(MavioStrings.issuesFilter, selection: $filter) {
                    ForEach(Filter.allCases, id: \.self) { Text($0.title).tag($0) }
                }
                .pickerStyle(.segmented)
                .listRowBackground(Color.clear)
                .listRowInsets(EdgeInsets())
            } footer: {
                Text(MavioStrings.issuesIntro)
            }
            Section {
                ForEach(issues ?? [], id: \.id) { issue in
                    NavigationLink {
                        IssueView(server: server, id: issue.id, names: names) { changed in
                            replace(changed, id: issue.id)
                        }
                    } label: {
                        IssueRow(issue: issue, item: names.items[issue.itemID], by: names.user(issue.createdBy, server))
                    }
                    .task { await names.item(issue.itemID, model, server) }
                    .onAppear {
                        if issue.id == issues?.last?.id, let issues, issues.count < total {
                            Task { await load(offset: Int32(issues.count)) }
                        }
                    }
                }
                if let failure {
                    Note(text: failure, failed: true)
                } else if let issues, issues.isEmpty {
                    Note(text: MavioStrings.issuesNone)
                }
            } footer: {
                if let issues, !issues.isEmpty { Text(MavioStrings.issuesCount(count: Int(total))) }
            }
        }
        .settingsPage(MavioStrings.settingsIssues)
        .background(theme.colors.background)
        .task(id: filter) {
            issues = nil
            await load(offset: 0)
        }
        .task { await names.loadUsers(model, server) }
        .pullToRefresh { await load(offset: 0) }
    }

    /// An issue changed on its page, or deleted when nil.
    private func replace(_ changed: Mavio_Acquisition_V1_Issue?, id: String) {
        guard let i = issues?.firstIndex(where: { $0.id == id }) else { return }
        if let changed { issues?[i] = changed } else { issues?.remove(at: i) }
    }

    private func load(offset: Int32) async {
        var request = Mavio_Acquisition_V1_ListIssuesRequest()
        request.statuses = filter.statuses
        request.sort = .modified
        request.limit = Self.page
        request.offset = offset
        do {
            let r = try await model.call(
                server, Procedure.issues("ListIssues"), request, as: Mavio_Acquisition_V1_ListIssuesResponse.self)
            issues = offset == 0 ? r.issues : (issues ?? []) + r.issues
            total = r.total
            failure = nil
        } catch is CancellationError {
        } catch {
            failure = error.reason
        }
    }
}

/// An issue's status, as a badge.
private struct StatusBadge: View {
    let status: Mavio_Acquisition_V1_IssueStatus
    @Environment(\.theme) private var theme

    var body: some View {
        let open = status == .open
        Text(open ? MavioStrings.issuesOpen : MavioStrings.issuesResolved)
            .type(theme.type.caption).fontWeight(.semibold)
            .padding(.horizontal, MavioSpacing.s8)
            .padding(.vertical, 2)
            .foregroundStyle(open ? theme.colors.onButton : .white)
            .background(open ? theme.colors.button : Color(hex: 0x34C759), in: .capsule)
    }
}

/// An issue in a list: its item, kind, numbers and status, what it says,
/// who reported it and when it last changed.
struct IssueRow: View {
    let issue: Mavio_Acquisition_V1_Issue
    let item: String?
    let by: String
    @Environment(\.theme) private var theme

    var body: some View {
        HStack(spacing: MavioSpacing.s12) {
            VStack(alignment: .leading, spacing: 2) {
                Text(
                    [item, issue.type.title, issue.numbers].compactMap(\.self).filter { !$0.isEmpty }.joined(
                        separator: " · ")
                )
                .lineLimit(1)
                Text(
                    [issue.comments.first?.message, by, issue.updateTime.time.map(fromNow)].compactMap(\.self)
                        .filter { !$0.isEmpty }.joined(separator: " · ")
                )
                .type(theme.type.footnote).foregroundStyle(theme.colors.textSecondary).lineLimit(1)
            }
            Spacer(minLength: 0)
            StatusBadge(status: issue.status)
        }
    }
}

/// An issue: its item, kind and status, its comments, a comment to add,
/// and resolving, reopening or deleting it.
struct IssueView: View {
    let server: ServerRecord
    let id: String
    let names: IssueNames
    /// The issue as it changed, or nil once deleted.
    let changed: (Mavio_Acquisition_V1_Issue?) -> Void
    @Environment(AppModel.self) private var model
    @Environment(\.theme) private var theme
    @Environment(\.dismiss) private var dismiss
    @State private var issue: Mavio_Acquisition_V1_Issue?
    @State private var failure: String?
    @State private var comment = ""
    @State private var busy = false
    @State private var deleting = false

    var body: some View {
        List {
            if let i = issue {
                Section {
                    HStack(spacing: MavioSpacing.s8) {
                        StatusBadge(status: i.status)
                        Text(
                            [i.numbers, MavioStrings.issuesReportedBy(name: names.user(i.createdBy, server))]
                                .compactMap(\.self).joined(separator: " · ")
                        )
                        .type(theme.type.footnote).foregroundStyle(theme.colors.textSecondary)
                    }
                }
                Section {
                    ForEach(i.comments, id: \.id) { c in
                        VStack(alignment: .leading, spacing: 2) {
                            Text(
                                [names.user(c.userID, server), c.createTime.time.map(fromNow)].compactMap(\.self)
                                    .joined(separator: " · ")
                            )
                            .type(theme.type.footnote).foregroundStyle(theme.colors.textSecondary)
                            Text(c.message).textSelection(.enabled)
                        }
                    }
                }
                Section {
                    TextField(MavioStrings.issuesComment, text: $comment, axis: .vertical)
                        .lineLimit(2...6)
                    Button(MavioStrings.issuesCommentSend, action: send)
                        .disabled(comment.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty || busy)
                }
                Section {
                    Button(i.status == .open ? MavioStrings.issuesResolve : MavioStrings.issuesReopen) {
                        act(i.status == .open ? "ResolveIssue" : "ReopenIssue")
                    }
                    .disabled(busy)
                    Button(MavioStrings.issuesDelete, role: .destructive) { deleting = true }
                        .disabled(busy)
                }
            }
            if let failure {
                Note(text: failure, failed: true)
            }
        }
        .overlay { if issue == nil && failure == nil { ProgressView() } }
        .settingsPage(
            issue.map { i in [i.type.title, names.items[i.itemID]].compactMap(\.self).joined(separator: " · ") }
                ?? MavioStrings.issuesTitle
        )
        .background(theme.colors.background)
        .confirmationDialog(MavioStrings.issuesConfirmDelete, isPresented: $deleting, titleVisibility: .visible) {
            Button(MavioStrings.issuesDelete, role: .destructive, action: remove)
        }
        .task { await load() }
        .pullToRefresh { await load() }
    }

    private func load() async {
        var request = Mavio_Acquisition_V1_GetIssueRequest()
        request.id = id
        do {
            issue = try await model.call(
                server, Procedure.issues("GetIssue"), request, as: Mavio_Acquisition_V1_GetIssueResponse.self
            ).issue
            failure = nil
            if let issue { await names.item(issue.itemID, model, server) }
        } catch is CancellationError {
        } catch {
            failure = error.reason
        }
    }

    private func send() {
        let message = comment.trimmingCharacters(in: .whitespacesAndNewlines)
        var request = Mavio_Acquisition_V1_AddCommentRequest()
        request.id = id
        request.message = message
        perform {
            let r = try await model.call(
                server, Procedure.issues("AddComment"), request, as: Mavio_Acquisition_V1_AddCommentResponse.self)
            comment = ""
            return r.issue
        }
    }

    /// Resolves or reopens the issue.
    private func act(_ method: String) {
        perform {
            if method == "ResolveIssue" {
                var request = Mavio_Acquisition_V1_ResolveIssueRequest()
                request.id = id
                return try await model.call(
                    server, Procedure.issues(method), request, as: Mavio_Acquisition_V1_ResolveIssueResponse.self
                ).issue
            }
            var request = Mavio_Acquisition_V1_ReopenIssueRequest()
            request.id = id
            return try await model.call(
                server, Procedure.issues(method), request, as: Mavio_Acquisition_V1_ReopenIssueResponse.self
            ).issue
        }
    }

    private func remove() {
        busy = true
        Task {
            var request = Mavio_Acquisition_V1_DeleteIssueRequest()
            request.id = id
            do {
                _ = try await model.call(
                    server, Procedure.issues("DeleteIssue"), request, as: Mavio_Acquisition_V1_DeleteIssueResponse.self)
                changed(nil)
                dismiss()
            } catch {
                failure = error.reason
            }
            busy = false
        }
    }

    private func perform(_ work: @escaping @MainActor () async throws -> Mavio_Acquisition_V1_Issue) {
        busy = true
        Task {
            do {
                let i = try await work()
                issue = i
                changed(i)
                failure = nil
            } catch {
                failure = error.reason
            }
            busy = false
        }
    }
}

/// Reports a problem with a movie, series, season or episode: its kind, for
/// a series the season and episode, 0 for all, and what is wrong; the issue
/// opens once reported.
struct ReportIssueView: View {
    let server: ServerRecord
    let itemId: String
    let name: String
    let series: Bool
    /// Called once the issue is reported.
    let reported: () -> Void
    @Environment(AppModel.self) private var model
    @Environment(\.theme) private var theme
    @Environment(\.dismiss) private var dismiss
    @State private var type = Mavio_Acquisition_V1_IssueType.video
    @State private var season = 0
    @State private var episode = 0
    @State private var message = ""
    @State private var busy = false
    @State private var failure: String?
    @State private var created: String?
    @State private var names = IssueNames()

    private static let types: [Mavio_Acquisition_V1_IssueType] = [.video, .audio, .subtitles, .other]

    var body: some View {
        NavigationStack {
            Group {
                if let created {
                    IssueView(server: server, id: created, names: names) { _ in }
                } else {
                    form
                }
            }
            .toolbar {
                if created == nil {
                    ToolbarItem(placement: .cancellationAction) {
                        Button(MavioStrings.actionCancel) { dismiss() }
                    }
                    ToolbarItem(placement: .confirmationAction) {
                        Button(MavioStrings.issuesSend, action: send)
                            .disabled(message.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty || busy)
                    }
                } else {
                    ToolbarItem(placement: .confirmationAction) {
                        Button(MavioStrings.actionDone) { dismiss() }
                    }
                }
            }
        }
    }

    private var form: some View {
        Form {
            Section(MavioStrings.issuesType) {
                Picker(MavioStrings.issuesType, selection: $type) {
                    ForEach(Self.types, id: \.self) { Text($0.title).tag($0) }
                }
                .pickerStyle(.segmented)
                .labelsHidden()
                .listRowBackground(Color.clear)
                .listRowInsets(EdgeInsets())
            }
            if series {
                Section {
                    Stepper(value: $season, in: 0...999) {
                        LabeledContent(MavioStrings.issuesSeason, value: String(season))
                    }
                    Stepper(value: $episode, in: 0...9999) {
                        LabeledContent(MavioStrings.issuesEpisode, value: String(season == 0 ? 0 : episode))
                    }
                    .disabled(season == 0)
                } footer: {
                    Text(MavioStrings.issuesAllNumbers)
                }
            }
            Section {
                TextField(MavioStrings.issuesMessage, text: $message, axis: .vertical)
                    .lineLimit(4...10)
            }
            if let failure {
                Note(text: failure, failed: true)
            }
        }
        .settingsPage(MavioStrings.issuesReportTitle(name: name))
        .background(theme.colors.background)
    }

    private func send() {
        var request = Mavio_Acquisition_V1_CreateIssueRequest()
        request.itemID = itemId
        request.type = type
        request.seasonNumber = series ? Int32(season) : 0
        request.episodeNumber = series && season > 0 ? Int32(episode) : 0
        request.message = message.trimmingCharacters(in: .whitespacesAndNewlines)
        busy = true
        Task {
            do {
                let r = try await model.call(
                    server, Procedure.issues("CreateIssue"), request, as: Mavio_Acquisition_V1_CreateIssueResponse.self)
                reported()
                if r.hasIssue { created = r.issue.id } else { dismiss() }
            } catch {
                failure = error.reason
            }
            busy = false
        }
    }
}

/// An item's open issues the user may see, on its details; each opens.
struct ItemIssues: View {
    let server: ServerRecord
    let itemId: String
    /// Changes when an issue was reported, to read them again.
    let version: Int
    @Environment(AppModel.self) private var model
    @Environment(\.theme) private var theme
    @State private var issues: [Mavio_Acquisition_V1_Issue] = []
    @State private var names = IssueNames()
    @State private var open: String?

    var body: some View {
        // A stack even when empty, as the task reading the issues needs a view.
        VStack(alignment: .leading, spacing: 0) {
            if !issues.isEmpty {
                VStack(alignment: .leading, spacing: MavioSpacing.s12) {
                    Text(MavioStrings.issuesItemTitle).type(theme.type.title3)
                    VStack(spacing: 0) {
                        ForEach(issues, id: \.id) { issue in
                            if issue.id != issues.first?.id { Divider().padding(.leading, MavioSpacing.s16) }
                            Button {
                                open = issue.id
                            } label: {
                                IssueRow(issue: issue, item: nil, by: names.user(issue.createdBy, server))
                                    .padding(.horizontal, MavioSpacing.s16)
                                    .padding(.vertical, MavioSpacing.s12)
                                    .contentShape(.rect)
                            }
                            .buttonStyle(.plain)
                        }
                    }
                    .background(theme.colors.surface, in: .rect(cornerRadius: MavioRadius.lg))
                }
                .padding(.horizontal, theme.layout.margin)
                .padding(.top, MavioSpacing.s24)
            }
        }
        .task(id: version) { await load() }
        .sheet(item: Binding(get: { open.map(OpenIssue.init) }, set: { open = $0?.id })) { opened in
            NavigationStack {
                IssueView(server: server, id: opened.id, names: names) { _ in }
                    .toolbar {
                        ToolbarItem(placement: .confirmationAction) {
                            Button(MavioStrings.actionDone) { open = nil }
                        }
                    }
            }
            .onDisappear { Task { await load() } }
        }
    }

    private struct OpenIssue: Identifiable {
        let id: String
    }

    private func load() async {
        var request = Mavio_Acquisition_V1_ListIssuesRequest()
        request.itemID = itemId
        request.statuses = [.open]
        request.sort = .modified
        request.limit = 50
        guard
            let r = try? await model.call(
                server, Procedure.issues("ListIssues"), request, as: Mavio_Acquisition_V1_ListIssuesResponse.self)
        else { return }
        issues = r.issues
        await names.loadUsers(model, server)
    }
}
