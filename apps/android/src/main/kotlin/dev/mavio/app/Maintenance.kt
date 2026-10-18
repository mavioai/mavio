package dev.mavio.app

import android.content.Context
import android.icu.util.Measure
import android.icu.util.MeasureUnit
import android.icu.text.MeasureFormat
import android.net.Uri
import android.text.format.DateUtils
import android.text.format.Formatter
import androidx.activity.compose.rememberLauncherForActivityResult
import androidx.activity.result.contract.ActivityResultContracts
import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.text.selection.SelectionContainer
import androidx.compose.material3.DropdownMenu
import androidx.compose.material3.DropdownMenuItem
import androidx.compose.material3.HorizontalDivider
import androidx.compose.material3.LinearProgressIndicator
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableIntStateOf
import androidx.compose.runtime.mutableStateMapOf
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.runtime.setValue
import androidx.compose.runtime.snapshots.SnapshotStateMap
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.layout.layout
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.StrokeCap
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.platform.LocalLocale
import androidx.compose.ui.res.pluralStringResource
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import com.google.protobuf.Duration
import dev.mavio.core.AppModel
import dev.mavio.core.ServerRecord
import dev.mavio.core.call
import dev.mavio.core.instant
import dev.mavio.design.MavioSpacing
import dev.mavio.ui.ButtonKind
import dev.mavio.ui.MavioButton
import dev.mavio.ui.MavioIcon
import dev.mavio.ui.R
import dev.mavio.ui.StatusView
import dev.mavio.ui.text
import dev.mavio.ui.theme
import kotlinx.coroutines.CancellationException
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.delay
import kotlinx.coroutines.launch
import kotlinx.coroutines.withContext
import mavio.acquisition.v1.IssueOuterClass.AddCommentResponse
import mavio.acquisition.v1.IssueOuterClass.DeleteIssueResponse
import mavio.acquisition.v1.IssueOuterClass.GetIssueResponse
import mavio.acquisition.v1.IssueOuterClass.Issue
import mavio.acquisition.v1.IssueOuterClass.IssueSort
import mavio.acquisition.v1.IssueOuterClass.IssueStatus
import mavio.acquisition.v1.IssueOuterClass.IssueType
import mavio.acquisition.v1.IssueOuterClass.ListIssuesResponse
import mavio.acquisition.v1.IssueOuterClass.ReopenIssueResponse
import mavio.acquisition.v1.IssueOuterClass.ResolveIssueResponse
import mavio.acquisition.v1.addCommentRequest
import mavio.acquisition.v1.deleteIssueRequest
import mavio.acquisition.v1.getIssueRequest
import mavio.acquisition.v1.listIssuesRequest
import mavio.acquisition.v1.reopenIssueRequest
import mavio.acquisition.v1.resolveIssueRequest
import mavio.library.v1.ItemOuterClass.GetItemResponse
import mavio.library.v1.ItemOuterClass.Item
import mavio.library.v1.ItemOuterClass.ItemKind
import mavio.library.v1.getItemRequest
import mavio.playback.v1.Playback.PlayMethod
import mavio.session.v1.SessionOuterClass.ListSessionsResponse
import mavio.session.v1.SessionOuterClass.Session
import mavio.session.v1.listSessionsRequest
import mavio.system.v1.ActivityOuterClass.Activity
import mavio.system.v1.ActivityOuterClass.ListActivitiesResponse
import mavio.system.v1.ActivityOuterClass.Severity
import mavio.system.v1.BackupOuterClass.Backup
import mavio.system.v1.BackupOuterClass.CreateBackupResponse
import mavio.system.v1.BackupOuterClass.DeleteBackupResponse
import mavio.system.v1.BackupOuterClass.ListBackupsResponse
import mavio.system.v1.System.ListLogsResponse
import mavio.system.v1.System.LogLevel
import mavio.system.v1.System.LogRecord
import mavio.system.v1.TaskOuterClass.Job
import mavio.system.v1.TaskOuterClass.JobState
import mavio.system.v1.TaskOuterClass.ListJobsResponse
import mavio.system.v1.TaskOuterClass.ListTasksResponse
import mavio.system.v1.TaskOuterClass.RunTaskResponse
import mavio.system.v1.TaskOuterClass.Task
import mavio.system.v1.createBackupRequest
import mavio.system.v1.deleteBackupRequest
import mavio.system.v1.listActivitiesRequest
import mavio.system.v1.listBackupsRequest
import mavio.system.v1.listJobsRequest
import mavio.system.v1.listLogsRequest
import mavio.system.v1.listTasksRequest
import mavio.system.v1.runTaskRequest
import mavio.user.v1.UserOuterClass.ListUsersResponse
import mavio.user.v1.listUsersRequest
import java.net.HttpURLConnection
import java.net.URL
import java.text.DateFormat
import java.util.Date

// The settings' sections of watching over and keeping a server, as the web
// UI's on a phone: Activity, Issues, Tasks, Logs and Backups
// (docs/clients.md §6.3.6–§6.3.7, §6.3.14, §6.3.25).

/** Calls a procedure, keeping its answer or why it failed; a cancellation passes through. */
internal suspend fun <T> attempt(onFailure: (String) -> Unit, block: suspend () -> T): T? = try {
    block()
} catch (e: CancellationException) {
    throw e
} catch (e: Exception) {
    onFailure(e.reason)
    null
}

/** A line saying a list is empty, or why it could not be read. */
@Composable
internal fun Note(text: String, failed: Boolean = false) {
    Text(
        text,
        Modifier.fillMaxWidth().padding(horizontal = 16.dp, vertical = 12.dp),
        style = theme.type.callout.text,
        color = if (failed) theme.colors.danger else theme.colors.textSecondary,
    )
}

/** A "Show More" line. */
@Composable
private fun ShowMore(onClick: () -> Unit) {
    Text(
        stringResource(R.string.activity_show_more),
        Modifier.fillMaxWidth().clickable(onClick = onClick).heightIn(min = 44.dp).padding(horizontal = 16.dp, vertical = 12.dp),
        style = theme.type.body.text,
        color = theme.colors.accentText,
    )
}

/** A colored dot, of a severity or state. */
@Composable
private fun Dot(color: Color, modifier: Modifier = Modifier) {
    Box(modifier.size(8.dp).background(color, CircleShape))
}

/** A choice of what a list shows, beside its title, as the web UI's select. */
@Composable
internal fun <T> HeaderChoice(choices: List<Pair<T, String>>, chosen: T, onChoose: (T) -> Unit) {
    var open by remember { mutableStateOf(false) }
    Box {
        Row(
            Modifier.clip(RoundedCornerShape(8.dp)).clickable { open = true }.padding(horizontal = 8.dp, vertical = 4.dp),
            horizontalArrangement = Arrangement.spacedBy(MavioSpacing.s4),
            verticalAlignment = Alignment.CenterVertically,
        ) {
            Text(choices.firstOrNull { it.first == chosen }?.second.orEmpty(), style = theme.type.callout.text, color = theme.colors.textSecondary)
            MavioIcon(R.drawable.ic_selector, size = 14.dp, tint = theme.colors.textSecondary)
        }
        DropdownMenu(open, onDismissRequest = { open = false }) {
            choices.forEach { (value, text) ->
                DropdownMenuItem(
                    text = { Text(text, style = theme.type.callout.text) },
                    onClick = {
                        open = false
                        onChoose(value)
                    },
                    trailingIcon = { if (value == chosen) MavioIcon(R.drawable.ic_check, size = 18.dp) },
                )
            }
        }
    }
}

/** A row's first line, with its details below. */
@Composable
internal fun Lines(title: String, detail: String?, modifier: Modifier = Modifier, maxLines: Int = Int.MAX_VALUE) {
    Column(modifier, verticalArrangement = Arrangement.spacedBy(2.dp)) {
        Text(title, style = theme.type.body.text, maxLines = maxLines)
        if (!detail.isNullOrEmpty()) {
            Text(detail, style = theme.type.footnote.text, color = theme.colors.textSecondary, maxLines = maxLines)
        }
    }
}

// Activity

/**
 * Activity (docs/clients.md §6.3.6): who plays what on the server, every
 * five seconds, and what happened on it lately, every thirty.
 */
@Composable
internal fun ActivityPage(model: AppModel, serverId: String) {
    var sessions by remember { mutableStateOf<List<Session>?>(null) }
    var sessionsFailure by remember { mutableStateOf<String?>(null) }
    var activities by remember { mutableStateOf<ListActivitiesResponse?>(null) }
    var failure by remember { mutableStateOf<String?>(null) }
    var minimum by rememberSaveable { mutableIntStateOf(Severity.SEVERITY_UNSPECIFIED_VALUE) }
    var limit by rememberSaveable { mutableIntStateOf(20) }
    LaunchedEffect(serverId) {
        while (true) {
            attempt({ sessionsFailure = it }) {
                sessions = model.client.call(serverId, Procedure.sessions("ListSessions"), listSessionsRequest { allUsers = true }, ListSessionsResponse::parseFrom).sessionsList
                sessionsFailure = null
            }
            delay(5_000)
        }
    }
    LaunchedEffect(serverId, minimum, limit) {
        while (true) {
            attempt({ failure = it }) {
                activities = model.client.call(
                    serverId,
                    Procedure.activity("ListActivities"),
                    listActivitiesRequest {
                        minSeverityValue = minimum
                        this.limit = limit
                    },
                    ListActivitiesResponse::parseFrom,
                )
                failure = null
            }
            delay(30_000)
        }
    }
    SettingsList(stringResource(R.string.settings_activity), stringResource(R.string.settings_activity_message)) {
        item {
            Group(stringResource(R.string.activity_now_playing)) {
                val playing = sessions.orEmpty().filter { it.hasNowPlaying() && it.nowPlaying.hasItem() }
                playing.forEachIndexed { i, s ->
                    if (i > 0) RowDivider(tiled = true)
                    PlayingRow(s)
                }
                when {
                    sessionsFailure != null -> Note(sessionsFailure!!, failed = true)
                    sessions != null && playing.isEmpty() -> Note(stringResource(R.string.activity_nothing_playing))
                }
            }
        }
        item {
            Group(
                stringResource(R.string.activity_recent),
                action = {
                    HeaderChoice(
                        listOf(
                            Severity.SEVERITY_UNSPECIFIED_VALUE to stringResource(R.string.activity_activity_all),
                            Severity.SEVERITY_WARNING_VALUE to stringResource(R.string.activity_activity_warnings),
                            Severity.SEVERITY_ERROR_VALUE to stringResource(R.string.activity_activity_errors),
                        ),
                        minimum,
                    ) { minimum = it }
                },
            ) {
                val list = activities?.activitiesList.orEmpty()
                list.forEachIndexed { i, a ->
                    if (i > 0) RowDivider()
                    ActivityRow(a)
                }
                when {
                    failure != null -> Note(failure!!, failed = true)
                    activities != null && list.isEmpty() -> Note(stringResource(R.string.activity_no_activity))
                }
                if ((activities?.total ?: 0) > list.size) {
                    RowDivider()
                    ShowMore { limit += 50 }
                }
            }
        }
    }
}

/** What a session plays: its item, who plays it where, how, and how far. */
@Composable
private fun PlayingRow(s: Session) {
    val np = s.nowPlaying
    val item = np.item
    val runtime = if (item.hasRuntime()) item.runtime.seconds else 0
    val position = if (np.hasPosition()) np.position.seconds else 0
    Column(Modifier.fillMaxWidth().padding(horizontal = 16.dp, vertical = 10.dp), verticalArrangement = Arrangement.spacedBy(MavioSpacing.s8)) {
        Row(horizontalArrangement = Arrangement.spacedBy(MavioSpacing.s12), verticalAlignment = Alignment.CenterVertically) {
            Box(
                Modifier.size(28.dp).background(if (np.paused) Color(0xFF8E8E93) else Color(0xFF34C759), RoundedCornerShape(7.dp)),
                Alignment.Center,
            ) {
                MavioIcon(if (np.paused) R.drawable.ic_pause_fill else R.drawable.ic_play_fill, size = 17.dp, tint = Color.White)
            }
            Lines(
                listOfNotNull(item.name, subtitleOf(item)).joinToString(" · "),
                listOf(s.userName.ifEmpty { s.pluginId }, s.deviceName, s.client).filter { it.isNotEmpty() }.joinToString(" · "),
                Modifier.weight(1f),
                maxLines = 1,
            )
            method(np.method)?.let { Text(stringResource(it), style = theme.type.footnote.text, color = theme.colors.textSecondary) }
        }
        if (runtime > 0) {
            Row(Modifier.padding(start = 40.dp), horizontalArrangement = Arrangement.spacedBy(MavioSpacing.s12), verticalAlignment = Alignment.CenterVertically) {
                LinearProgressIndicator(
                    { (position.toFloat() / runtime).coerceIn(0f, 1f) },
                    Modifier.weight(1f).height(4.dp),
                    color = theme.colors.accent,
                    trackColor = theme.colors.fill,
                    strokeCap = StrokeCap.Round,
                    gapSize = 0.dp,
                    drawStopIndicator = {},
                )
                Text("${clock(position)} / ${clock(runtime)}", style = theme.type.caption.text, color = theme.colors.textSecondary)
            }
        }
    }
}

private fun method(m: PlayMethod): Int? = when (m) {
    PlayMethod.PLAY_METHOD_DIRECT_PLAY -> R.string.activity_direct_play
    PlayMethod.PLAY_METHOD_DIRECT_STREAM -> R.string.activity_direct_stream
    PlayMethod.PLAY_METHOD_TRANSCODE -> R.string.activity_transcode
    else -> null
}

/** What an item is of, under its name: an episode's series and numbers, a season's series, music's artists. */
internal fun subtitleOf(item: Item): String? = when (item.kind) {
    ItemKind.ITEM_KIND_EPISODE -> {
        val n = when {
            item.hasParentIndexNumber() && item.hasIndexNumber() -> "S${item.parentIndexNumber}:E${item.indexNumber}"
            item.hasIndexNumber() -> "E${item.indexNumber}"
            else -> ""
        }
        listOf(item.seriesName, n).filter { it.isNotEmpty() }.joinToString(" · ").ifEmpty { null }
    }
    ItemKind.ITEM_KIND_SEASON -> item.seriesName.ifEmpty { null }
    ItemKind.ITEM_KIND_MUSIC_ALBUM, ItemKind.ITEM_KIND_TRACK ->
        item.albumArtistsList.ifEmpty { item.artistsList }.joinToString(", ").ifEmpty { null }
    else -> null
}

/** A position, as "1:02:03" or "2:03". */
private fun clock(seconds: Long): String {
    val s = seconds.coerceAtLeast(0)
    return if (s >= 3600) "%d:%02d:%02d".format(s / 3600, s / 60 % 60, s % 60) else "%d:%02d".format(s / 60, s % 60)
}

@Composable
private fun ActivityRow(a: Activity) {
    val color = when (a.severity) {
        Severity.SEVERITY_WARNING -> Color(0xFFFF9500)
        Severity.SEVERITY_ERROR -> Color(0xFFFF3B30)
        else -> theme.colors.textTertiary
    }
    Row(Modifier.fillMaxWidth().padding(horizontal = 16.dp, vertical = 10.dp), horizontalArrangement = Arrangement.spacedBy(MavioSpacing.s12)) {
        Dot(color, Modifier.padding(top = 8.dp))
        Lines(a.title, a.message, Modifier.weight(1f))
        if (a.hasTime()) Text(fromNow(a.time), style = theme.type.footnote.text, color = theme.colors.textSecondary)
    }
}

// Tasks

private fun JobState.color(): Color = when (this) {
    JobState.JOB_STATE_RUNNING -> Color(0xFF007AFF)
    JobState.JOB_STATE_SUCCEEDED -> Color(0xFF34C759)
    JobState.JOB_STATE_FAILED -> Color(0xFFFF3B30)
    else -> Color(0xFF8E8E93)
}

private fun JobState.title(): Int? = when (this) {
    JobState.JOB_STATE_PENDING -> R.string.tasks_pending
    JobState.JOB_STATE_RUNNING -> R.string.tasks_running
    JobState.JOB_STATE_SUCCEEDED -> R.string.tasks_succeeded
    JobState.JOB_STATE_FAILED -> R.string.tasks_failed
    else -> null
}

/** The jobs shown, by their states. */
private enum class JobFilter(val title: Int, val states: List<JobState>) {
    All(R.string.tasks_jobs_all, emptyList()),
    Active(R.string.tasks_jobs_active, listOf(JobState.JOB_STATE_PENDING, JobState.JOB_STATE_RUNNING)),
    Failed(R.string.tasks_jobs_failed, listOf(JobState.JOB_STATE_FAILED)),
    Succeeded(R.string.tasks_jobs_succeeded, listOf(JobState.JOB_STATE_SUCCEEDED)),
}

/**
 * Whether a task or job of a kind shows: those of acquiring media and of
 * plugins, which the app leaves out, do not (docs/clients.md §4.2).
 */
private fun shownKind(kind: String) = listOf("acquisition.", "requests.", "subtitles.", "plugin.").none { kind.startsWith(it) }

/** A length of time in its two largest units, as "1 day, 6 hr". */
private fun lengthOf(locale: java.util.Locale, d: Duration): String {
    var left = d.seconds
    val units = listOf(86_400L to MeasureUnit.DAY, 3_600L to MeasureUnit.HOUR, 60L to MeasureUnit.MINUTE, 1L to MeasureUnit.SECOND)
    val parts = units.mapNotNull { (size, unit) ->
        val n = left / size
        left %= size
        if (n > 0) Measure(n, unit) else null
    }.take(2).ifEmpty { listOf(Measure(0, MeasureUnit.SECOND)) }
    return MeasureFormat.getInstance(locale, MeasureFormat.FormatWidth.SHORT).formatMeasures(*parts.toTypedArray())
}

/**
 * Tasks (docs/clients.md §6.3.7): the server's recurring tasks with how
 * often they run, how their last run went and when they run next, to run
 * now; and the jobs of its queue, filtered by state.
 */
@Composable
internal fun TasksPage(model: AppModel, serverId: String) {
    var tasks by remember { mutableStateOf<List<Task>?>(null) }
    var failure by remember { mutableStateOf<String?>(null) }
    var jobs by remember { mutableStateOf<ListJobsResponse?>(null) }
    var jobsFailure by remember { mutableStateOf<String?>(null) }
    var filter by rememberSaveable { mutableStateOf(JobFilter.All) }
    var limit by rememberSaveable { mutableIntStateOf(50) }
    // A notice of running a task, kept until its next run finishes: by task, the notice and the run it followed.
    val notices = remember { mutableStateMapOf<String, Pair<Notice, String>>() }
    var starting by remember { mutableStateOf<String?>(null) }
    val scope = rememberCoroutineScope()
    suspend fun loadTasks() {
        attempt({ failure = it }) {
            tasks = model.client.call(serverId, Procedure.tasks("ListTasks"), listTasksRequest {}, ListTasksResponse::parseFrom).tasksList
            failure = null
        }
    }
    suspend fun loadJobs() {
        attempt({ jobsFailure = it }) {
            jobs = model.client.call(
                serverId,
                Procedure.tasks("ListJobs"),
                listJobsRequest {
                    states += filter.states
                    this.limit = limit
                },
                ListJobsResponse::parseFrom,
            )
            jobsFailure = null
        }
    }
    LaunchedEffect(serverId) {
        while (true) {
            loadTasks()
            // Often while a task runs.
            delay(if (tasks?.any { it.running } == true) 2_000 else 15_000)
        }
    }
    LaunchedEffect(serverId, filter, limit) {
        while (true) {
            loadJobs()
            delay(5_000)
        }
    }
    fun run(task: Task) {
        val last = if (task.hasLastRun()) task.lastRun.id else ""
        starting = task.id
        scope.launch {
            try {
                val r = model.client.call(serverId, Procedure.tasks("RunTask"), runTaskRequest { id = task.id }, RunTaskResponse::parseFrom)
                notices[task.id] = Notice(if (r.enqueued) R.string.tasks_queued else R.string.tasks_already_queued) to last
            } catch (e: CancellationException) {
                throw e
            } catch (e: Exception) {
                notices[task.id] = Notice(text = e.reason) to last
            }
            starting = null
            loadTasks()
            loadJobs()
        }
    }
    SettingsList(stringResource(R.string.settings_tasks), stringResource(R.string.tasks_message)) {
        val own = tasks.orEmpty().filter { it.pluginId.isEmpty() && shownKind(it.kind) }.sortedWith(compareBy(String.CASE_INSENSITIVE_ORDER) { it.name })
        when {
            tasks == null && failure == null -> item { StatusView(stringResource(R.string.state_loading)) }
            failure != null && tasks == null -> item { StatusView(failure!!, icon = R.drawable.ic_error) }
            own.isNotEmpty() -> item {
                Group(stringResource(R.string.tasks_server)) {
                    own.forEachIndexed { i, t ->
                        if (i > 0) RowDivider()
                        TaskRow(t, notices, starting == t.id) { run(t) }
                    }
                }
            }
        }
        item {
            Group(
                stringResource(R.string.tasks_jobs),
                stringResource(R.string.tasks_jobs_footer),
                action = { HeaderChoice(JobFilter.entries.map { it to stringResource(it.title) }, filter) { filter = it } },
            ) {
                val shown = jobs?.jobsList.orEmpty().filter { shownKind(it.kind) }
                shown.forEachIndexed { i, j ->
                    if (i > 0) RowDivider()
                    JobRow(j)
                }
                when {
                    jobsFailure != null -> Note(jobsFailure!!, failed = true)
                    jobs != null && shown.isEmpty() -> Note(stringResource(R.string.tasks_no_jobs))
                }
                if ((jobs?.total ?: 0) > (jobs?.jobsCount ?: 0)) {
                    RowDivider()
                    ShowMore { limit += 100 }
                }
            }
        }
    }
}

/** What running a task did: a string's resource, or why it failed. */
private data class Notice(val res: Int = 0, val text: String? = null)

@Composable
private fun TaskRow(task: Task, notices: SnapshotStateMap<String, Pair<Notice, String>>, starting: Boolean, onRun: () -> Unit) {
    val last = if (task.hasLastRun()) task.lastRun else null
    val status = when {
        task.running -> stringResource(R.string.tasks_running_now)
        last != null -> listOfNotNull(
            last.state.title()?.let { stringResource(it) },
            (if (last.hasFinishTime()) last.finishTime else if (last.hasRunTime()) last.runTime else null)?.let(::fromNow),
        ).joinToString(" ")
        else -> stringResource(R.string.tasks_never_run)
    }
    val notice = notices[task.id]?.takeIf { it.second == (last?.id ?: "") }?.first?.let { it.text ?: stringResource(it.res) }
    val every = if (task.hasInterval()) stringResource(R.string.tasks_every, lengthOf(LocalLocale.current.platformLocale, task.interval)) else stringResource(R.string.tasks_on_demand)
    val next = if (!task.running && task.hasNextRunTime()) stringResource(R.string.tasks_next, fromNow(task.nextRunTime)) else null
    Row(
        Modifier.fillMaxWidth().padding(horizontal = 16.dp, vertical = 10.dp),
        horizontalArrangement = Arrangement.spacedBy(MavioSpacing.s12),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        Column(Modifier.weight(1f), verticalArrangement = Arrangement.spacedBy(2.dp)) {
            Text(task.name, style = theme.type.body.text)
            if (task.description.isNotEmpty()) {
                Text(task.description, style = theme.type.footnote.text, color = theme.colors.textSecondary)
            }
            Row(horizontalArrangement = Arrangement.spacedBy(MavioSpacing.s4), verticalAlignment = Alignment.CenterVertically) {
                Dot(if (task.running) JobState.JOB_STATE_RUNNING.color() else last?.state?.color() ?: theme.colors.fill)
                Text(
                    listOfNotNull(if (task.running) status else notice ?: status, every, next).joinToString(" · "),
                    style = theme.type.footnote.text,
                    color = theme.colors.textSecondary,
                )
            }
            if (last != null && last.state == JobState.JOB_STATE_FAILED && last.lastError.isNotEmpty() && !task.running) {
                Text(last.lastError, style = theme.type.footnote.text, color = theme.colors.danger)
            }
        }
        MavioButton(if (starting) "…" else stringResource(R.string.tasks_run), onRun, kind = ButtonKind.Secondary, enabled = !starting && !task.running)
    }
}

@Composable
private fun JobRow(job: Job) {
    val time = when {
        job.hasFinishTime() -> job.finishTime
        job.hasRunTime() -> job.runTime
        job.hasCreateTime() -> job.createTime
        else -> null
    }
    val whenText = if (job.state == JobState.JOB_STATE_PENDING) {
        if (job.hasRunTime()) stringResource(R.string.tasks_runs, fromNow(job.runTime)) else null
    } else {
        time?.let(::fromNow)
    }
    Column(Modifier.fillMaxWidth().padding(horizontal = 16.dp, vertical = 10.dp), verticalArrangement = Arrangement.spacedBy(2.dp)) {
        Text(job.kind, style = theme.type.body.text)
        Row(horizontalArrangement = Arrangement.spacedBy(MavioSpacing.s4), verticalAlignment = Alignment.CenterVertically) {
            Dot(job.state.color())
            Text(
                listOfNotNull(
                    job.state.title()?.let { stringResource(it) },
                    if (job.hasProgress()) "${(job.progress * 100).toInt()}%" else null,
                    whenText,
                    if (job.attempts > 1) stringResource(R.string.tasks_attempts, job.attempts.toString(), job.maxAttempts.toString()) else null,
                ).joinToString(" · "),
                style = theme.type.footnote.text,
                color = theme.colors.textSecondary,
            )
        }
        if (job.lastError.isNotEmpty()) Text(job.lastError, style = theme.type.footnote.text, color = theme.colors.danger)
    }
}

// Logs

private val logLevels = listOf(LogLevel.LOG_LEVEL_DEBUG, LogLevel.LOG_LEVEL_INFO, LogLevel.LOG_LEVEL_WARN, LogLevel.LOG_LEVEL_ERROR)

private fun levelName(level: LogLevel): Int = when (level) {
    LogLevel.LOG_LEVEL_DEBUG -> R.string.logs_debug
    LogLevel.LOG_LEVEL_WARN -> R.string.logs_warn
    LogLevel.LOG_LEVEL_ERROR -> R.string.logs_error
    else -> R.string.logs_info
}

/** Logs (docs/clients.md §6.3.25): the server's recent log records at a level or above, from a period, filtered by words. */
@Composable
internal fun LogsPage(model: AppModel, serverId: String) {
    var records by remember { mutableStateOf<List<LogRecord>?>(null) }
    var failure by remember { mutableStateOf<String?>(null) }
    var level by rememberSaveable { mutableStateOf(LogLevel.LOG_LEVEL_INFO) }
    var hours by rememberSaveable { mutableIntStateOf(24) }
    var filter by rememberSaveable { mutableStateOf("") }
    var fetching by remember { mutableStateOf(false) }
    var reload by remember { mutableIntStateOf(0) }
    LaunchedEffect(serverId, level, hours, reload) {
        fetching = true
        attempt({ failure = it }) {
            records = model.client.call(
                serverId,
                Procedure.system("ListLogs"),
                listLogsRequest {
                    minLevel = level
                    limit = 1000
                    if (hours > 0) maxAge = Duration.newBuilder().setSeconds(hours * 3600L).build()
                },
                ListLogsResponse::parseFrom,
            ).recordsList
            failure = null
        }
        fetching = false
    }
    val words = filter.lowercase().split(Regex("\\s+")).filter { it.isNotEmpty() }
    val shown = records.orEmpty().filter { r ->
        val text = (listOf(r.message) + r.attributesMap.flatMap { listOf(it.key, it.value) }).joinToString(" ").lowercase()
        words.all { text.contains(it) }
    }
    SettingsList(stringResource(R.string.settings_logs), stringResource(R.string.logs_message)) {
        item {
            Column(verticalArrangement = Arrangement.spacedBy(MavioSpacing.s8)) {
                Group {
                    ChoiceRow(stringResource(R.string.logs_level), logLevels.map { it to stringResource(levelName(it)) }, level) { level = it }
                    RowDivider()
                    ChoiceRow(
                        stringResource(R.string.logs_period),
                        listOf(1 to stringResource(R.string.logs_last_hour), 24 to stringResource(R.string.logs_last_day), 0 to stringResource(R.string.logs_all)),
                        hours,
                    ) { hours = it }
                }
                Row(horizontalArrangement = Arrangement.spacedBy(MavioSpacing.s8), verticalAlignment = Alignment.CenterVertically) {
                    SettingsField(filter, { filter = it }, stringResource(R.string.logs_filter), Modifier.weight(1f))
                    MavioButton(if (fetching) "…" else stringResource(R.string.logs_refresh), { reload++ }, kind = ButtonKind.Secondary, enabled = !fetching)
                }
            }
        }
        item {
            Group(footer = if (records != null) pluralStringResource(R.plurals.logs_count, shown.size, shown.size) else null) {
                when {
                    records == null && failure == null -> Note(stringResource(R.string.state_loading))
                    failure != null -> Note(failure!!, failed = true)
                    shown.isEmpty() -> Note(stringResource(R.string.logs_none))
                }
            }
        }
        // Each record an item of its own, a day of them being long, drawn as rows of one card.
        items(shown.size) { i ->
            LogRow(shown[i], first = i == 0, last = i == shown.size - 1)
        }
    }
}

private val logClock: DateFormat = DateFormat.getDateTimeInstance(DateFormat.SHORT, DateFormat.MEDIUM)

@Composable
private fun LogRow(record: LogRecord, first: Boolean, last: Boolean) {
    val color = when (record.level) {
        LogLevel.LOG_LEVEL_DEBUG -> theme.colors.textTertiary
        LogLevel.LOG_LEVEL_WARN -> Color(0xFFFF9F0A)
        LogLevel.LOG_LEVEL_ERROR -> theme.colors.danger
        else -> theme.colors.textSecondary
    }
    val corner = 14.dp
    val shape = RoundedCornerShape(
        topStart = if (first) corner else 0.dp,
        topEnd = if (first) corner else 0.dp,
        bottomStart = if (last) corner else 0.dp,
        bottomEnd = if (last) corner else 0.dp,
    )
    val gap = MavioSpacing.s24
    // Reported shorter by the list's spacing, so the next row starts where this one ends.
    val joined = Modifier.layout { measurable, constraints ->
        val placeable = measurable.measure(constraints)
        layout(placeable.width, if (last) placeable.height else (placeable.height - gap.roundToPx()).coerceAtLeast(0)) { placeable.place(0, 0) }
    }
    Column(joined.fillMaxWidth().clip(shape).background(theme.colors.surface)) {
        if (!first) HorizontalDivider(Modifier.padding(start = 16.dp), color = theme.colors.border)
        SelectionContainer {
            Column(Modifier.fillMaxWidth().padding(horizontal = 16.dp, vertical = 10.dp), verticalArrangement = Arrangement.spacedBy(2.dp)) {
                Row(horizontalArrangement = Arrangement.spacedBy(MavioSpacing.s8)) {
                    record.time.instant?.let {
                        Text(logClock.format(Date(it.toEpochMilli())), style = theme.type.footnote.text, color = theme.colors.textSecondary)
                    }
                    Text(stringResource(levelName(record.level)), style = theme.type.footnote.text, fontWeight = FontWeight.SemiBold, color = color)
                }
                Text(record.message, style = theme.type.callout.text)
                val attrs = record.attributesMap.entries.sortedBy { it.key }
                if (attrs.isNotEmpty()) {
                    Text(
                        attrs.joinToString("  ") { "${it.key}=${it.value}" },
                        style = theme.type.caption.text.copy(fontFamily = FontFamily.Monospace),
                        color = theme.colors.textSecondary,
                    )
                }
            }
        }
    }
}

// Backups

/** Backups (docs/clients.md §6.3.14): the server's backups, made, saved to the device's files and deleted here. */
@Composable
internal fun BackupsPage(model: AppModel, serverId: String) {
    var backups by remember { mutableStateOf<List<Backup>?>(null) }
    var failure by remember { mutableStateOf<String?>(null) }
    var creating by remember { mutableStateOf(false) }
    var actionFailure by remember { mutableStateOf<String?>(null) }
    var removing by remember { mutableStateOf<Backup?>(null) }
    var downloading by remember { mutableStateOf<String?>(null) }
    val scope = rememberCoroutineScope()
    val context = LocalContext.current
    suspend fun load() {
        attempt({ failure = it }) {
            backups = model.client.call(serverId, Procedure.backups("ListBackups"), listBackupsRequest {}, ListBackupsResponse::parseFrom).backupsList
            failure = null
        }
    }
    LaunchedEffect(serverId) { load() }
    // Where the user chose to save the backup being downloaded.
    val save = rememberLauncherForActivityResult(ActivityResultContracts.CreateDocument("application/zip")) { uri ->
        val name = downloading
        if (uri == null || name == null) {
            downloading = null
            return@rememberLauncherForActivityResult
        }
        scope.launch {
            attempt({ actionFailure = it }) {
                download(model, context, serverId, name, uri)
                actionFailure = null
            }
            downloading = null
        }
    }
    SettingsList(stringResource(R.string.settings_backups), stringResource(R.string.backups_message)) {
        item {
            Group(footer = stringResource(R.string.backups_restore_footer)) {
                Row(
                    Modifier.fillMaxWidth().padding(horizontal = 16.dp, vertical = 12.dp),
                    horizontalArrangement = Arrangement.spacedBy(MavioSpacing.s12),
                    verticalAlignment = Alignment.CenterVertically,
                ) {
                    Lines(
                        stringResource(R.string.backups_create),
                        stringResource(if (creating) R.string.backups_creating else R.string.backups_create_detail),
                        Modifier.weight(1f),
                    )
                    MavioButton(
                        if (creating) "…" else stringResource(R.string.backups_create_action),
                        {
                            creating = true
                            scope.launch {
                                attempt({ actionFailure = it }) {
                                    model.client.call(serverId, Procedure.backups("CreateBackup"), createBackupRequest {}, CreateBackupResponse::parseFrom)
                                    actionFailure = null
                                }
                                creating = false
                                load()
                            }
                        },
                        enabled = !creating,
                    )
                }
                actionFailure?.let { Note(it, failed = true) }
            }
        }
        item {
            Group(stringResource(R.string.backups_list)) {
                backups.orEmpty().forEachIndexed { i, b ->
                    if (i > 0) RowDivider()
                    Row(
                        Modifier.fillMaxWidth().padding(start = 16.dp, end = 8.dp, top = 8.dp, bottom = 8.dp),
                        horizontalArrangement = Arrangement.spacedBy(MavioSpacing.s8),
                        verticalAlignment = Alignment.CenterVertically,
                    ) {
                        Lines(
                            b.name,
                            listOfNotNull(
                                if (b.hasCreateTime()) fromNow(b.createTime) else null,
                                Formatter.formatShortFileSize(context, b.sizeBytes),
                                b.version.ifEmpty { null }?.let { stringResource(R.string.backups_version, it) },
                            ).joinToString(" · "),
                            Modifier.weight(1f),
                        )
                        IconAction(R.drawable.ic_downloads, stringResource(R.string.backups_download), enabled = downloading == null) {
                            downloading = b.name
                            save.launch(b.name)
                        }
                        IconAction(R.drawable.ic_close, stringResource(R.string.backups_remove), tint = theme.colors.danger) { removing = b }
                    }
                }
                when {
                    failure != null -> Note(failure!!, failed = true)
                    backups == null -> Note(stringResource(R.string.state_loading))
                    backups!!.isEmpty() -> Note(stringResource(R.string.backups_none))
                }
            }
        }
    }
    removing?.let { b ->
        ConfirmDialog(
            stringResource(R.string.backups_remove_title, b.name),
            stringResource(R.string.backups_remove_message),
            stringResource(R.string.backups_remove),
            onConfirm = {
                scope.launch {
                    attempt({ actionFailure = it }) {
                        model.client.call(serverId, Procedure.backups("DeleteBackup"), deleteBackupRequest { name = b.name }, DeleteBackupResponse::parseFrom)
                    }
                    load()
                }
            },
            onDismiss = { removing = null },
        )
    }
}

/** An action shown as its icon alone. */
@Composable
private fun IconAction(icon: Int, label: String, enabled: Boolean = true, tint: Color = theme.colors.accentText, onClick: () -> Unit) {
    Box(
        Modifier.size(40.dp).clip(CircleShape).clickable(enabled = enabled, onClickLabel = label, onClick = onClick),
        Alignment.Center,
    ) {
        MavioIcon(icon, size = 20.dp, tint = if (enabled) tint else theme.colors.textTertiary)
    }
}

/** Downloads a backup's file with the session's access token into the document the user chose. */
private suspend fun download(model: AppModel, context: Context, serverId: String, name: String, to: Uri) {
    val target = model.client.serverRequest(serverId, "system/backups/${Uri.encode(name)}")
    withContext(Dispatchers.IO) {
        val connection = URL(target.url).openConnection() as HttpURLConnection
        try {
            connection.setRequestProperty("Authorization", "Bearer ${target.token}")
            if (connection.responseCode !in 200..299) error("HTTP ${connection.responseCode}")
            val out = context.contentResolver.openOutputStream(to) ?: error("cannot write $to")
            out.use { o -> connection.inputStream.use { it.copyTo(o) } }
        } finally {
            connection.disconnect()
        }
    }
}

// Issues

private fun IssueType.title(): Int? = when (this) {
    IssueType.ISSUE_TYPE_VIDEO -> R.string.issues_video
    IssueType.ISSUE_TYPE_AUDIO -> R.string.issues_audio
    IssueType.ISSUE_TYPE_SUBTITLES -> R.string.issues_subtitles
    IssueType.ISSUE_TYPE_OTHER -> R.string.issues_other
    else -> null
}

/** The season or episode of a series an issue is about. */
@Composable
private fun Issue.numbers(): String? = when {
    seasonNumber <= 0 -> null
    episodeNumber > 0 -> "S%02dE%02d".format(seasonNumber, episodeNumber)
    else -> stringResource(R.string.discover_season, seasonNumber.toString())
}

/** The names of items and users issues name, read once. */
private class IssueNames {
    val items = mutableStateMapOf<String, String>()
    val users = mutableStateMapOf<String, String>()
    private val asked = mutableSetOf<String>()

    suspend fun item(id: String, model: AppModel, serverId: String) {
        if (id.isEmpty() || !asked.add(id)) return
        runCatching { model.client.call(serverId, Procedure.items("GetItem"), getItemRequest { this.id = id }, GetItemResponse::parseFrom) }
            .getOrNull()?.let { items[id] = it.item.name }
    }

    /** Everyone's names, for an administrator. */
    suspend fun loadUsers(model: AppModel, serverId: String) {
        if (!asked.add("users")) return
        runCatching { model.client.call(serverId, Procedure.users("ListUsers"), listUsersRequest {}, ListUsersResponse::parseFrom) }
            .getOrNull()?.usersList?.forEach { users.putIfAbsent(it.id, it.name) }
    }

    /** A user's name: the user's own, everyone's for an administrator, else "Administrator". */
    fun user(context: Context, id: String, server: ServerRecord?): String =
        if (server != null && id == server.userId) server.userName else users[id] ?: context.getString(R.string.issues_administrator)
}

/** The issues' statuses shown. */
private enum class IssueFilter(val title: Int, val statuses: List<IssueStatus>) {
    Open(R.string.issues_open, listOf(IssueStatus.ISSUE_STATUS_OPEN)),
    Resolved(R.string.issues_resolved, listOf(IssueStatus.ISSUE_STATUS_RESOLVED)),
    All(R.string.issues_all, emptyList()),
}

private const val ISSUES_PAGE = 50

/**
 * Issues: the problems reported with the server's items, the user's own or
 * everyone's for its administrators, open ones at first; each opens its
 * page.
 */
@Composable
internal fun IssuesPage(model: AppModel, serverId: String) {
    var issues by remember { mutableStateOf<List<Issue>?>(null) }
    var total by remember { mutableIntStateOf(0) }
    var failure by remember { mutableStateOf<String?>(null) }
    var filter by rememberSaveable { mutableStateOf(IssueFilter.Open) }
    val names = remember { IssueNames() }
    val server = model.servers.collectAsState().value.firstOrNull { it.id == serverId }
    val scope = rememberCoroutineScope()
    val context = LocalContext.current
    val navigator = LocalNavigator.current
    suspend fun load(offset: Int) {
        attempt({ failure = it }) {
            val r = model.client.call(
                serverId,
                Procedure.issues("ListIssues"),
                listIssuesRequest {
                    statuses += filter.statuses
                    sort = IssueSort.ISSUE_SORT_MODIFIED
                    limit = ISSUES_PAGE
                    this.offset = offset
                },
                ListIssuesResponse::parseFrom,
            )
            issues = if (offset == 0) r.issuesList else issues.orEmpty() + r.issuesList
            total = r.total
            failure = null
            r.issuesList.forEach { names.item(it.itemId, model, serverId) }
        }
    }
    // Again on coming back, for what changed on an issue's page.
    LaunchedEffect(serverId, filter) { load(0) }
    LaunchedEffect(serverId) { names.loadUsers(model, serverId) }
    SettingsList(stringResource(R.string.settings_issues), stringResource(R.string.issues_intro)) {
        item {
            Segmented(IssueFilter.entries.map { stringResource(it.title) }, filter.ordinal) {
                filter = IssueFilter.entries[it]
                issues = null
            }
        }
        item {
            val list = issues
            Group(footer = if (!list.isNullOrEmpty()) pluralStringResource(R.plurals.issues_count, total, total) else null) {
                list.orEmpty().forEachIndexed { i, issue ->
                    if (i > 0) RowDivider()
                    Row(
                        Modifier.fillMaxWidth().clickable { navigator.open(Route.IssuePage(serverId, issue.id)) }.padding(horizontal = 16.dp, vertical = 10.dp),
                        horizontalArrangement = Arrangement.spacedBy(MavioSpacing.s12),
                        verticalAlignment = Alignment.CenterVertically,
                    ) {
                        Lines(
                            listOfNotNull(names.items[issue.itemId], issue.type.title()?.let { stringResource(it) }, issue.numbers())
                                .filter { it.isNotEmpty() }.joinToString(" · "),
                            listOfNotNull(
                                issue.commentsList.firstOrNull()?.message,
                                names.user(context, issue.createdBy, server),
                                if (issue.hasUpdateTime()) fromNow(issue.updateTime) else null,
                            ).filter { it.isNotEmpty() }.joinToString(" · "),
                            Modifier.weight(1f),
                            maxLines = 1,
                        )
                        IssueBadge(issue.status)
                        Chevron()
                    }
                }
                if (list != null && list.size < total) {
                    RowDivider()
                    ShowMore { scope.launch { load(list.size) } }
                }
                when {
                    failure != null -> Note(failure!!, failed = true)
                    list == null -> Note(stringResource(R.string.state_loading))
                    list.isEmpty() -> Note(stringResource(R.string.issues_none))
                }
            }
        }
    }
}

/** An issue's status, as a badge. */
@Composable
private fun IssueBadge(status: IssueStatus) {
    val open = status == IssueStatus.ISSUE_STATUS_OPEN
    Text(
        stringResource(if (open) R.string.issues_open else R.string.issues_resolved),
        Modifier.background(if (open) theme.colors.accent else Color(0xFF34C759), CircleShape).padding(horizontal = 8.dp, vertical = 2.dp),
        style = theme.type.caption.text,
        fontWeight = FontWeight.SemiBold,
        color = if (open) theme.colors.onAccent else Color.White,
    )
}

/** An issue: its item, kind and status, its comments, a comment to add, and resolving, reopening or deleting it. */
@Composable
fun IssueScreen(model: AppModel, route: Route.IssuePage) {
    val serverId = route.serverId
    var issue by remember { mutableStateOf<Issue?>(null) }
    var failure by remember { mutableStateOf<String?>(null) }
    var comment by rememberSaveable { mutableStateOf("") }
    var busy by remember { mutableStateOf(false) }
    var deleting by remember { mutableStateOf(false) }
    val names = remember { IssueNames() }
    val server = model.servers.collectAsState().value.firstOrNull { it.id == serverId }
    val scope = rememberCoroutineScope()
    val context = LocalContext.current
    val navigator = LocalNavigator.current
    LaunchedEffect(serverId, route.id) {
        attempt({ failure = it }) {
            val i = model.client.call(serverId, Procedure.issues("GetIssue"), getIssueRequest { id = route.id }, GetIssueResponse::parseFrom).issue
            issue = i
            failure = null
            names.item(i.itemId, model, serverId)
        }
        names.loadUsers(model, serverId)
    }
    fun perform(work: suspend () -> Issue) {
        busy = true
        scope.launch {
            attempt({ failure = it }) {
                issue = work()
                failure = null
            }
            busy = false
        }
    }
    val i = issue
    val title = i?.let { listOfNotNull(it.type.title()?.let { t -> stringResource(t) }, names.items[it.itemId]).joinToString(" · ") }
        ?: stringResource(R.string.issues_title)
    SettingsList(title) {
        if (i == null) {
            item { if (failure == null) StatusView(stringResource(R.string.state_loading)) else StatusView(failure!!, icon = R.drawable.ic_error) }
            return@SettingsList
        }
        item {
            Row(horizontalArrangement = Arrangement.spacedBy(MavioSpacing.s8), verticalAlignment = Alignment.CenterVertically) {
                IssueBadge(i.status)
                Text(
                    listOfNotNull(i.numbers(), stringResource(R.string.issues_reported_by, names.user(context, i.createdBy, server))).joinToString(" · "),
                    style = theme.type.footnote.text,
                    color = theme.colors.textSecondary,
                )
            }
        }
        item {
            Group {
                i.commentsList.forEachIndexed { n, c ->
                    if (n > 0) RowDivider()
                    SelectionContainer {
                        Lines(
                            c.message,
                            listOfNotNull(names.user(context, c.userId, server), if (c.hasCreateTime()) fromNow(c.createTime) else null).joinToString(" · "),
                            Modifier.fillMaxWidth().padding(horizontal = 16.dp, vertical = 10.dp),
                        )
                    }
                }
            }
        }
        item {
            Column(verticalArrangement = Arrangement.spacedBy(MavioSpacing.s8)) {
                SettingsField(comment, { comment = it }, stringResource(R.string.issues_comment))
                MavioButton(
                    stringResource(R.string.issues_comment_send),
                    {
                        val message = comment.trim()
                        perform {
                            val r = model.client.call(
                                serverId,
                                Procedure.issues("AddComment"),
                                addCommentRequest {
                                    id = route.id
                                    this.message = message
                                },
                                AddCommentResponse::parseFrom,
                            )
                            comment = ""
                            r.issue
                        }
                    },
                    enabled = comment.isNotBlank() && !busy,
                )
            }
        }
        item {
            Group(footer = failure) {
                val open = i.status == IssueStatus.ISSUE_STATUS_OPEN
                ActionRow(stringResource(if (open) R.string.issues_resolve else R.string.issues_reopen), theme.colors.accentText, !busy) {
                    perform {
                        if (open) {
                            model.client.call(serverId, Procedure.issues("ResolveIssue"), resolveIssueRequest { id = route.id }, ResolveIssueResponse::parseFrom).issue
                        } else {
                            model.client.call(serverId, Procedure.issues("ReopenIssue"), reopenIssueRequest { id = route.id }, ReopenIssueResponse::parseFrom).issue
                        }
                    }
                }
                RowDivider()
                ActionRow(stringResource(R.string.issues_delete), theme.colors.danger, !busy) { deleting = true }
            }
        }
    }
    if (deleting) {
        ConfirmDialog(
            stringResource(R.string.issues_confirm_delete),
            null,
            stringResource(R.string.issues_delete),
            onConfirm = {
                busy = true
                scope.launch {
                    attempt({ failure = it }) {
                        model.client.call(serverId, Procedure.issues("DeleteIssue"), deleteIssueRequest { id = route.id }, DeleteIssueResponse::parseFrom)
                        navigator.back()
                    }
                    busy = false
                }
            },
            onDismiss = { deleting = false },
        )
    }
}

/** An action of a group, in its color. */
@Composable
internal fun ActionRow(label: String, color: Color, enabled: Boolean, onClick: () -> Unit) {
    Text(
        label,
        Modifier.fillMaxWidth().clickable(enabled = enabled, onClick = onClick).heightIn(min = 48.dp).padding(horizontal = 16.dp, vertical = 13.dp),
        style = theme.type.body.text,
        color = if (enabled) color else theme.colors.textTertiary,
    )
}
