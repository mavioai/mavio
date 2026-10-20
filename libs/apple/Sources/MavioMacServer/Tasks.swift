#if os(macOS)
    import AppKit
    import MavioMac

    extension Mavio_System_V1_JobState {
        fileprivate var color: NSColor {
            switch self {
            case .running: NSColor(hex: 0x007AFF)
            case .succeeded: NSColor(hex: 0x34C759)
            case .failed: NSColor(hex: 0xFF3B30)
            default: NSColor(hex: 0x8E8E93)
            }
        }

        @MainActor fileprivate var title: String {
            switch self {
            case .pending: MavioStrings.tasksPending
            case .running: MavioStrings.tasksRunning
            case .succeeded: MavioStrings.tasksSucceeded
            case .failed: MavioStrings.tasksFailed
            default: ""
            }
        }
    }

    /// Tasks: the server's recurring tasks, its own, with how often they
    /// run, how their last run went and when they run next, to run now; and
    /// the jobs of its queue, filtered by state.
    final class TasksPage: ServerPage {
        private var tasks: [Mavio_System_V1_Task]?
        private var failure: String?
        private var jobs: Mavio_System_V1_ListJobsResponse?
        private var jobsFailure: String?
        private var filter = JobFilter.all
        private var limit: Int32 = 50
        /// A notice of running a task, kept until its next run finishes: by
        /// task, the notice and the run it followed.
        private var notices: [String: (text: String, run: String)] = [:]
        private var starting: String?
        private lazy var form = FormPage(title: MavioStrings.settingsTasks, theme: theme)
        private lazy var failureSection = section()
        private lazy var ownSection = section(MavioStrings.tasksServer)
        private lazy var jobsSection = section(nil, footer: MavioStrings.tasksJobsFooter)

        private enum JobFilter: CaseIterable {
            case all, active, failed, succeeded

            var states: [Mavio_System_V1_JobState] {
                switch self {
                case .all: []
                case .active: [.pending, .running]
                case .failed: [.failed]
                case .succeeded: [.succeeded]
                }
            }

            @MainActor var title: String {
                switch self {
                case .all: MavioStrings.tasksJobsAll
                case .active: MavioStrings.tasksJobsActive
                case .failed: MavioStrings.tasksJobsFailed
                case .succeeded: MavioStrings.tasksJobsSucceeded
                }
            }
        }

        override func viewDidLoad() {
            super.viewDidLoad()
            title = MavioStrings.settingsTasks
            let filters = JobFilter.allCases
            let popup = ActionPopUp(options: filters.map(\.title), selected: 0) { [weak self] i in
                guard let self else { return }
                filter = filters[i]
                pollJobs()
            }
            popup.setAccessibilityLabel(MavioStrings.tasksJobsShown)
            let jobsHeader = NSStackView.horizontal(
                [
                    NSTextField.label(MavioStrings.tasksJobs, theme.fonts.headline, color: theme.colors.textPrimary),
                    Spacer(), FocusFrame(popup, theme: theme),
                ], spacing: MavioSpacing.s8)
            form.add(failureSection)
            form.add(ownSection)
            let holder = NSView()
            holder.translatesAutoresizingMaskIntoConstraints = false
            holder.addFilling(jobsHeader, insets: .symmetric(horizontal: MavioSpacing.s4))
            let jobs = NSStackView.vertical([holder, jobsSection], spacing: MavioSpacing.s8)
            for view in [holder, jobsSection] {
                view.widthAnchor.constraint(equalTo: jobs.widthAnchor).isActive = true
            }
            form.add(jobs)
            renderTasks()
            renderJobs()
            show(form)
        }

        override func appeared() {
            // Often while a task runs.
            poll(
                "tasks",
                every: { [weak self] in self?.tasks?.contains(where: \.running) == true ? .seconds(2) : .seconds(15) }
            ) { [weak self] in await self?.loadTasks() }
            pollJobs()
        }

        private func pollJobs() {
            poll("jobs", every: { .seconds(5) }) { [weak self] in await self?.loadJobs() }
        }

        /// Whether a task or job of a kind shows: those of acquiring media
        /// and of plugins, which the app leaves out, do not (docs/clients.md
        /// §4.2).
        private static func shown(_ kind: String) -> Bool {
            !["acquisition.", "requests.", "subtitles.", "plugin."].contains { kind.hasPrefix($0) }
        }

        private func renderTasks() {
            failureSection.setRows(failure.map { [note($0, failed: true)] } ?? [])
            failureSection.isHidden = failure == nil
            let own = (tasks ?? []).filter { $0.pluginID.isEmpty && Self.shown($0.kind) }.sorted {
                $0.name.localizedStandardCompare($1.name) == .orderedAscending
            }
            ownSection.setRows(own.map(taskRow))
            ownSection.isHidden = own.isEmpty
            refocus()
        }

        private func renderJobs() {
            let shownJobs = (jobs?.jobs ?? []).filter { Self.shown($0.kind) }
            var rows: [NSView] = shownJobs.map(jobRow)
            if let jobsFailure {
                rows.append(note(jobsFailure, failed: true))
            } else if jobs != nil, shownJobs.isEmpty {
                rows.append(note(MavioStrings.tasksNoJobs))
            } else if jobs == nil {
                rows.append(padded(LoadingView()))
            }
            if let jobs, jobs.total > jobs.jobs.count {
                rows.append(
                    FormRow.action(MavioStrings.tasksShowMore, theme: theme) { [weak self] in
                        guard let self else { return }
                        limit += 100
                        pollJobs()
                    })
            }
            jobsSection.setRows(rows)
            refocus()
        }

        private func taskRow(_ task: Mavio_System_V1_Task) -> NSView {
            let last = task.hasLastRun ? task.lastRun : nil
            // A running task tells what it is doing, untranslated, and a run
            // that succeeded what it did.
            let status =
                task.running
                ? [
                    MavioStrings.tasksRunningNow, task.progress > 0 ? "\(Int((task.progress * 100).rounded()))%" : "",
                    task.status,
                ].filter { !$0.isEmpty }.joined(separator: " · ")
                : last.map { l in
                    [l.state.title, (l.finishTime.time ?? l.runTime.time).map(fromNow) ?? ""].filter { !$0.isEmpty }
                        .joined(separator: " ")
                } ?? MavioStrings.tasksNeverRun
            let notice = notices[task.id].flatMap { $0.run == (last?.id ?? "") ? $0.text : nil }
            let every =
                task.hasInterval
                ? MavioStrings.tasksEvery(length: lengthOf(task.interval.timeInterval)) : MavioStrings.tasksOnDemand
            let next = !task.running ? task.nextRunTime.time.map { MavioStrings.tasksNext(when: fromNow($0)) } : nil
            var lines: [NSView] = [
                NSTextField.label(
                    task.name, theme.isTV ? theme.fonts.headline : theme.fonts.body, color: theme.colors.textPrimary,
                    lines: 0)
            ]
            if !task.description_p.isEmpty {
                lines.append(
                    NSTextField.label(
                        task.description_p, theme.fonts.footnote, color: theme.colors.textSecondary, lines: 0))
            }
            let mark = dot(
                task.running ? Mavio_System_V1_JobState.running.color : last?.state.color ?? theme.colors.fill, size: 8)
            if task.running { pulse(mark) }
            lines.append(
                NSStackView.horizontal(
                    [
                        mark,
                        NSTextField.label(
                            ([task.running ? status : (notice ?? status), every] + [next].compactMap(\.self))
                                .joined(separator: " · "), theme.fonts.footnote, color: theme.colors.textSecondary,
                            lines: 0),
                    ], spacing: MavioSpacing.s4))
            if let last, last.state == .failed, !last.lastError.isEmpty, !task.running {
                lines.append(
                    NSTextField.label(
                        last.lastError, theme.fonts.footnote, color: theme.colors.danger, lines: 0, selectable: true))
            }
            if let last, last.state == .succeeded, !last.result.isEmpty, !task.running {
                lines.append(
                    NSTextField.label(
                        last.result, theme.fonts.footnote, color: theme.colors.textSecondary, lines: 0,
                        selectable: true))
            }
            let run = button(starting == task.id ? "…" : MavioStrings.tasksRun) { [weak self] in self?.run(task) }
            run.isEnabled = starting != task.id && !task.running
            return ControlRow(NSStackView.vertical(lines, spacing: 2), controls: [run], theme: theme)
        }

        private func jobRow(_ job: Mavio_System_V1_Job) -> NSView {
            let when: String? =
                job.state == .pending
                ? job.runTime.time.map { MavioStrings.tasksRuns(when: fromNow($0)) }
                : (job.finishTime.time ?? job.runTime.time ?? job.createTime.time).map(fromNow)
            let state =
                ([job.state.title]
                + [
                    job.state == .running && job.progress > 0 ? "\(Int(job.progress * 100))%" : nil,
                    job.status.isEmpty ? nil : job.status, when,
                    job.attempts > 1
                        ? MavioStrings.tasksAttempts(attempts: "\(job.attempts)", max: "\(job.maxAttempts)") : nil,
                ].compactMap(\.self)).joined(separator: " · ")
            var lines: [NSView] = [
                NSTextField.label(job.kind, theme.fonts.body, color: theme.colors.textPrimary, lines: 0),
                NSStackView.horizontal(
                    [
                        dot(job.state.color),
                        NSTextField.label(state, theme.fonts.footnote, color: theme.colors.textSecondary, lines: 0),
                    ], spacing: MavioSpacing.s4),
            ]
            if !job.lastError.isEmpty {
                lines.append(
                    NSTextField.label(
                        job.lastError, theme.fonts.footnote, color: theme.colors.danger, lines: 0, selectable: true))
            }
            if job.state == .succeeded, !job.result.isEmpty {
                lines.append(
                    NSTextField.label(
                        job.result, theme.fonts.footnote, color: theme.colors.textSecondary, lines: 0,
                        selectable: true))
            }
            return padded(NSStackView.vertical(lines, spacing: 2))
        }

        private func run(_ task: Mavio_System_V1_Task) {
            let last = task.hasLastRun ? task.lastRun.id : ""
            starting = task.id
            renderTasks()
            run { [weak self] in
                guard let self else { return }
                var request = Mavio_System_V1_RunTaskRequest()
                request.id = task.id
                do {
                    let r = try await model.call(
                        server, Procedure.tasks("RunTask"), request, as: Mavio_System_V1_RunTaskResponse.self)
                    notices[task.id] = (r.enqueued ? MavioStrings.tasksQueued : MavioStrings.tasksAlreadyQueued, last)
                } catch {
                    notices[task.id] = (error.reason, last)
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
                return
            } catch {
                failure = error.reason
            }
            renderTasks()
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
                return
            } catch {
                jobsFailure = error.reason
            }
            renderJobs()
        }
    }
#endif
