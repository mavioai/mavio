package dev.mavio.app

import android.content.Context
import android.icu.text.MeasureFormat
import android.icu.util.Measure
import android.icu.util.MeasureUnit
import android.text.format.DateUtils
import android.text.format.Formatter
import androidx.annotation.DrawableRes
import androidx.annotation.StringRes
import androidx.compose.foundation.Canvas
import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.aspectRatio
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.lazy.LazyListScope
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateMapOf
import androidx.compose.runtime.mutableIntStateOf
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.unit.dp
import dev.mavio.core.AppModel
import dev.mavio.core.DeviceSession
import dev.mavio.core.ServerRecord
import dev.mavio.core.call
import dev.mavio.core.instant
import dev.mavio.design.MavioSpacing
import dev.mavio.ui.MavioIcon
import dev.mavio.ui.R
import dev.mavio.ui.StatusView
import dev.mavio.ui.text
import dev.mavio.ui.theme
import kotlinx.coroutines.CancellationException
import kotlinx.coroutines.delay
import kotlinx.coroutines.launch
import mavio.system.v1.PluginOuterClass.ContainerRuntime
import mavio.system.v1.PluginOuterClass.GetContainerRuntimeResponse
import mavio.system.v1.System.GetHealthResponse
import mavio.system.v1.System.GetSystemInfoResponse
import mavio.system.v1.System.GetSystemLoadResponse
import mavio.system.v1.System.LoadSample
import mavio.system.v1.getContainerRuntimeRequest
import mavio.system.v1.getHealthRequest
import mavio.system.v1.getSystemInfoRequest
import mavio.system.v1.getSystemLoadRequest
import mavio.user.v1.UserOuterClass.GetCurrentUserResponse
import mavio.user.v1.getCurrentUserRequest
import java.time.Duration
import java.time.Instant
import java.text.NumberFormat

/** The paths of the server's procedures the settings call. */
object Procedure {
    fun system(method: String) = "/mavio.system.v1.SystemService/$method"
    fun plugins(method: String) = "/mavio.system.v1.PluginManagementService/$method"
    fun users(method: String) = "/mavio.user.v1.UserService/$method"
    fun localization(method: String) = "/mavio.system.v1.LocalizationService/$method"
    fun auth(method: String) = "/mavio.auth.v1.AuthService/$method"
    fun activity(method: String) = "/mavio.system.v1.ActivityService/$method"
    fun tasks(method: String) = "/mavio.system.v1.TaskService/$method"
    fun backups(method: String) = "/mavio.system.v1.BackupService/$method"
    fun sessions(method: String) = "/mavio.session.v1.SessionService/$method"
    fun issues(method: String) = "/mavio.acquisition.v1.IssueService/$method"
    fun items(method: String) = "/mavio.library.v1.ItemService/$method"
    fun playlists(method: String) = "/mavio.library.v1.PlaylistService/$method"
    fun collections(method: String) = "/mavio.library.v1.CollectionService/$method"
    fun metadata(method: String) = "/mavio.library.v1.MetadataService/$method"
    fun libraries(method: String) = "/mavio.library.v1.LibraryService/$method"
    fun storage(method: String) = "/mavio.storage.v1.StorageService/$method"
}

/** A section of the Server group, as the web UI's. */
enum class ServerSection(@StringRes val title: Int, @DrawableRes val icon: Int, val color: Color, val forAnyone: Boolean = false) {
    Overview(R.string.settings_overview, R.drawable.ic_server_fill, Color(0xFF8E8E93)),
    General(R.string.settings_general, R.drawable.ic_settings_fill, Color(0xFF8E8E93)),
    Users(R.string.settings_users, R.drawable.ic_users_fill, Color(0xFF007AFF)),
    Devices(R.string.settings_devices, R.drawable.ic_devices_fill, Color(0xFF32ADE6), forAnyone = true),
    ApiKeys(R.string.settings_api_keys, R.drawable.ic_lock_fill, Color(0xFF636366)),
    Libraries(R.string.settings_libraries, R.drawable.ic_folder_fill, Color(0xFF007AFF)),
    Storage(R.string.settings_storage, R.drawable.ic_server_fill, Color(0xFF30B0C7)),
    Transcoding(R.string.settings_transcoding, R.drawable.ic_quality_fill, Color(0xFFFF2D55)),
    Activity(R.string.settings_activity, R.drawable.ic_activity_fill, Color(0xFFFF2D55)),
    Issues(R.string.settings_issues, R.drawable.ic_error_fill, Color(0xFFFF9500), forAnyone = true),
    Tasks(R.string.settings_tasks, R.drawable.ic_pending_fill, Color(0xFF5856D6)),
    Logs(R.string.settings_logs, R.drawable.ic_info_fill, Color(0xFF8E8E93)),
    Backups(R.string.settings_backups, R.drawable.ic_folder_fill, Color(0xFF5856D6)),
}

/** A menu of the Server group's second level, with its sections. */
enum class ServerMenu(@StringRes val title: Int, @DrawableRes val icon: Int, val color: Color, val sections: List<ServerSection>) {
    Media(R.string.settings_media_menu, R.drawable.ic_movies_fill, Color(0xFFFF9F0A), listOf(ServerSection.Libraries, ServerSection.Storage, ServerSection.Transcoding)),
    Maintenance(
        R.string.settings_maintenance_menu,
        R.drawable.ic_admin_fill,
        Color(0xFF8E8E93),
        listOf(ServerSection.Activity, ServerSection.Issues, ServerSection.Tasks, ServerSection.Logs, ServerSection.Backups),
    ),
}

/** The Server group's first level: its sections and menus, in the order a server is set up and kept. */
private val TOP: List<Any> = listOf(
    ServerSection.Overview,
    ServerSection.General,
    ServerSection.Users,
    ServerSection.Devices,
    ServerSection.ApiKeys,
    ServerMenu.Media,
    ServerMenu.Maintenance,
)

/**
 * The Server group's first level for a user, administering the server or
 * not: others see the sections any user has, and a menu left with one
 * section is that section.
 */
internal fun serverEntries(admin: Boolean): List<Any> = TOP.mapNotNull { entry ->
    when (entry) {
        is ServerSection -> entry.takeIf { it.forAnyone || admin }
        is ServerMenu -> entry.sections.filter { it.forAnyone || admin }.let { if (it.size > 1) entry else it.firstOrNull() }
        else -> null
    }
}

/**
 * The settings' Server group, as the web UI's on a phone (docs/clients.md
 * §6.3): the picker of the servers signed in to, then the sections of the
 * server chosen, its administrators' with the devices signed in as the user
 * among them.
 */
internal fun LazyListScope.serverGroup(model: AppModel, servers: List<ServerRecord>) {
    if (servers.isEmpty()) return
    item(key = "server-group") {
        var serverId by rememberSaveable { mutableStateOf(servers.first().id) }
        val server = servers.firstOrNull { it.id == serverId } ?: servers.first()
        // Whether the user administers each server, once known, by server
        // and user, as switching users changes it; kept while pages open
        // over the settings, so that coming back finds the group as long
        // as it was, and where it was scrolled.
        var admin by rememberSaveable { mutableStateOf(HashMap<String, Boolean>()) }
        val adminKey = "${server.id}/${server.userId}"
        LaunchedEffect(adminKey) {
            runCatching {
                model.client.call(server.id, Procedure.users("GetCurrentUser"), getCurrentUserRequest {}, GetCurrentUserResponse::parseFrom)
            }.onSuccess { admin = HashMap(admin).apply { put(adminKey, it.user.admin) } }
        }
        val navigator = LocalNavigator.current
        Group(stringResource(R.string.settings_server_group)) {
            if (servers.size > 1) {
                ChoiceRow(stringResource(R.string.auth_server), servers.map { it.id to it.name }, server.id) { serverId = it }
                RowDivider()
            }
            val shown = serverEntries(admin[adminKey] == true)
            shown.forEachIndexed { i, entry ->
                if (i > 0) RowDivider(tiled = true)
                when (entry) {
                    is ServerSection -> EntryRow(stringResource(entry.title), entry.icon, entry.color) {
                        navigator.open(Route.ServerSetting(server.id, entry))
                    }
                    is ServerMenu -> EntryRow(stringResource(entry.title), entry.icon, entry.color) {
                        navigator.open(Route.ServerMenuPage(server.id, entry))
                    }
                }
            }
        }
    }
}

/** A section of the Server group. */
@Composable
fun ServerSettingScreen(model: AppModel, route: Route.ServerSetting) {
    when (route.section) {
        ServerSection.Overview -> OverviewPage(model, route.serverId)
        ServerSection.General -> GeneralPage(model, route.serverId)
        ServerSection.Devices -> DevicesPage(model, route.serverId)
        ServerSection.ApiKeys -> ApiKeysPage(model, route.serverId)
        ServerSection.Users -> UsersPage(model, route.serverId)
        ServerSection.Libraries -> LibrariesPage(model, route.serverId)
        ServerSection.Storage -> StoragePage(model, route.serverId)
        ServerSection.Transcoding -> TranscodingPage(model, route.serverId)
        ServerSection.Activity -> ActivityPage(model, route.serverId)
        ServerSection.Issues -> IssuesPage(model, route.serverId)
        ServerSection.Tasks -> TasksPage(model, route.serverId)
        ServerSection.Logs -> LogsPage(model, route.serverId)
        ServerSection.Backups -> BackupsPage(model, route.serverId)
    }
}

/** A menu's page: its sections, as the Server group lists its own; only an administrator sees one. */
@Composable
fun ServerMenuScreen(route: Route.ServerMenuPage) {
    val navigator = LocalNavigator.current
    SettingsList(stringResource(route.menu.title)) {
        item {
            Group {
                route.menu.sections.forEachIndexed { i, section ->
                    if (i > 0) RowDivider(tiled = true)
                    EntryRow(stringResource(section.title), section.icon, section.color) {
                        navigator.open(Route.ServerSetting(route.serverId, section))
                    }
                }
            }
        }
    }
}

/** What went wrong, as the server or the core told it. */
internal val Throwable.reason: String get() = message ?: toString()

/**
 * Overview (docs/clients.md §6.3.6): whether the server serves, its version
 * and what it runs on, and the load of its computer, sampled every two
 * seconds while the page shows.
 */
@Composable
private fun OverviewPage(model: AppModel, serverId: String) {
    var info by remember { mutableStateOf<GetSystemInfoResponse?>(null) }
    var serving by remember { mutableStateOf<Boolean?>(null) }
    var runtime by remember { mutableStateOf<ContainerRuntime?>(null) }
    var failure by remember { mutableStateOf<String?>(null) }
    var samples by remember { mutableStateOf<List<LoadSample>>(emptyList()) }
    var loadFailure by remember { mutableStateOf<String?>(null) }
    val client = model.client
    LaunchedEffect(serverId) {
        launch {
            serving = runCatching {
                client.call(serverId, Procedure.system("GetHealth"), getHealthRequest {}, GetHealthResponse::parseFrom)
            }.getOrNull()?.status == GetHealthResponse.Status.STATUS_SERVING
        }
        launch {
            runtime = runCatching {
                client.call(serverId, Procedure.plugins("GetContainerRuntime"), getContainerRuntimeRequest {}, GetContainerRuntimeResponse::parseFrom)
            }.getOrNull()?.runtime
        }
        try {
            info = client.call(serverId, Procedure.system("GetSystemInfo"), getSystemInfoRequest {}, GetSystemInfoResponse::parseFrom)
            failure = null
        } catch (e: CancellationException) {
            throw e
        } catch (e: Exception) {
            failure = e.reason
        }
    }
    LaunchedEffect(serverId) {
        while (true) {
            try {
                samples = client.call(serverId, Procedure.system("GetSystemLoad"), getSystemLoadRequest {}, GetSystemLoadResponse::parseFrom).samplesList
                loadFailure = null
            } catch (e: CancellationException) {
                throw e
            } catch (e: Exception) {
                loadFailure = e.reason
            }
            delay(2_000)
        }
    }
    val context = LocalContext.current
    SettingsList(stringResource(R.string.settings_overview)) {
        item {
            Group(stringResource(R.string.overview_server)) {
                Row(
                    Modifier.fillMaxWidth().heightIn(min = 48.dp).padding(horizontal = 16.dp),
                    horizontalArrangement = Arrangement.spacedBy(MavioSpacing.s8),
                    verticalAlignment = Alignment.CenterVertically,
                ) {
                    Text(stringResource(R.string.overview_status), Modifier.weight(1f), style = theme.type.body.text)
                    val s = serving
                    Box(Modifier.size(8.dp).background(if (s == null) theme.colors.fill else if (s) Color(0xFF34C759) else theme.colors.danger, CircleShape))
                    Text(
                        if (s == null) "…" else stringResource(if (s) R.string.overview_serving else R.string.overview_not_serving),
                        style = theme.type.body.text,
                        color = theme.colors.textSecondary,
                    )
                }
                info?.let { info ->
                    RowDivider()
                    ValueRow(stringResource(R.string.overview_version), info.version)
                    RowDivider()
                    ValueRow(stringResource(R.string.overview_platform), "${info.os}/${info.arch}")
                    RowDivider()
                    ValueRow(stringResource(R.string.overview_database), database(info.database))
                    RowDivider()
                    ValueRow(
                        stringResource(R.string.overview_ffmpeg),
                        info.ffmpegVersion.ifEmpty { stringResource(R.string.overview_ffmpeg_missing) },
                    )
                }
                runtime?.let { r ->
                    RowDivider()
                    Row(Modifier.fillMaxWidth().padding(horizontal = 16.dp, vertical = 12.dp), horizontalArrangement = Arrangement.spacedBy(MavioSpacing.s12)) {
                        Text(stringResource(R.string.overview_container_engine), style = theme.type.body.text)
                        Column(Modifier.weight(1f), horizontalAlignment = Alignment.End) {
                            Text(engine(context, r), style = theme.type.body.text, color = theme.colors.textSecondary, textAlign = TextAlign.End)
                            if (r.available) Text(r.endpoint, style = theme.type.footnote.text, color = theme.colors.textTertiary, textAlign = TextAlign.End)
                        }
                    }
                }
                info?.startTime?.instant?.let { start ->
                    RowDivider()
                    ValueRow(stringResource(R.string.overview_uptime), uptime(Duration.between(start, Instant.now())))
                }
                failure?.let {
                    RowDivider()
                    Text(it, Modifier.padding(16.dp), style = theme.type.callout.text, color = theme.colors.danger)
                }
            }
        }
        item {
            Column(verticalArrangement = Arrangement.spacedBy(MavioSpacing.s8)) {
                Text(stringResource(R.string.overview_load), Modifier.padding(horizontal = 4.dp), style = theme.type.headline.text)
                val error = loadFailure
                if (error != null) {
                    Group { Text(error, Modifier.padding(16.dp), style = theme.type.callout.text, color = theme.colors.danger) }
                } else {
                    LoadCards(samples)
                }
                Text(
                    stringResource(R.string.overview_load_footer),
                    Modifier.padding(horizontal = 4.dp),
                    style = theme.type.footnote.text,
                    color = theme.colors.textSecondary,
                )
            }
        }
    }
}

private fun database(name: String) = when (name) {
    "postgres" -> "PostgreSQL"
    "sqlite" -> "SQLite"
    else -> name
}

private fun engine(context: Context, r: ContainerRuntime): String {
    if (!r.available) return context.getString(R.string.overview_container_engine_none, r.reason)
    val name = mapOf("docker" to "Docker", "podman" to "Podman")[r.engine] ?: r.engine
    return listOf("$name ${r.version}", "API ${r.apiVersion}", if (r.rootless) context.getString(R.string.overview_container_engine_rootless) else "")
        .filter { it.isNotEmpty() }
        .joinToString(" · ")
}

/** A length of time in its two largest units of days, hours and minutes, narrow. */
private fun uptime(d: Duration): String {
    val parts = listOf(d.toDays() to MeasureUnit.DAY, d.toHours() % 24 to MeasureUnit.HOUR, d.toMinutes() % 60 to MeasureUnit.MINUTE)
        .dropWhile { it.first == 0L }
        .take(2)
        .ifEmpty { listOf(0L to MeasureUnit.MINUTE) }
    return MeasureFormat.getInstance(java.util.Locale.getDefault(), MeasureFormat.FormatWidth.NARROW)
        .formatMeasures(*parts.map { Measure(it.first, it.second) }.toTypedArray())
}

/**
 * The cards of the load of the server's computer: its CPU, memory, disks
 * and network, each with what it is now, three figures more and a graph of
 * the last two minutes.
 */
@Composable
private fun LoadCards(samples: List<LoadSample>) {
    val context = LocalContext.current
    val now = samples.lastOrNull()
    fun bytes(n: Long) = Formatter.formatFileSize(context, n)
    fun rate(n: Double) = Formatter.formatFileSize(context, n.toLong()) + "/s"
    val percent = remember { NumberFormat.getPercentInstance() }
    Column(verticalArrangement = Arrangement.spacedBy(MavioSpacing.s12)) {
        val c = now?.takeIf { it.hasCpu() }?.cpu
        LoadCard(
            R.drawable.ic_cpu_fill, Color(0xFF007AFF), stringResource(R.string.overview_cpu),
            told = now == null || c != null,
            value = c?.let { percent.format(it.user + it.system) },
            stats = listOf(
                stringResource(R.string.overview_cpu_user) to c?.let { percent.format(it.user) },
                stringResource(R.string.overview_cpu_system) to c?.let { percent.format(it.system) },
                stringResource(R.string.overview_by_server) to c?.let { percent.format(it.server) },
            ),
            graph = samples.map { if (it.hasCpu()) it.cpu.user + it.cpu.system else null },
        )
        val m = now?.takeIf { it.hasMemory() }?.memory
        LoadCard(
            R.drawable.ic_memory_fill, Color(0xFF34C759), stringResource(R.string.overview_memory),
            badge = m?.takeIf { it.totalBytes > 0 }?.let { it.usedBytes.toDouble() / it.totalBytes >= 0.9 },
            told = now == null || m != null,
            value = m?.let { bytes(it.usedBytes) },
            after = m?.let { stringResource(R.string.overview_memory_of, bytes(it.totalBytes)) },
            stats = listOf(
                stringResource(R.string.overview_memory_available) to m?.let { bytes(it.totalBytes - it.usedBytes) },
                stringResource(R.string.overview_swap) to m?.let { if (it.swapTotalBytes > 0) bytes(it.swapUsedBytes) else "–" },
                stringResource(R.string.overview_by_server) to m?.let { bytes(it.serverBytes) },
            ),
            graph = samples.map { s -> s.memory.takeIf { s.hasMemory() && it.totalBytes > 0 }?.let { it.usedBytes.toDouble() / it.totalBytes } },
        )
        val d = now?.takeIf { it.hasDisk() }?.disk
        LoadCard(
            R.drawable.ic_disk_fill, Color(0xFFFF9500), stringResource(R.string.overview_disk),
            told = now == null || d != null,
            value = d?.let { if (it.hasTotalBytes()) bytes(it.availableBytes) else "–" },
            after = d?.takeIf { it.hasTotalBytes() }?.let { stringResource(R.string.overview_disk_available, bytes(it.totalBytes)) },
            stats = listOf(
                stringResource(R.string.overview_read) to d?.let { if (it.hasReadRate()) rate(it.readRate) else "–" },
                stringResource(R.string.overview_write) to d?.let { if (it.hasWriteRate()) rate(it.writeRate) else "–" },
                stringResource(R.string.overview_written) to d?.let { if (it.hasWrittenBytes()) bytes(it.writtenBytes) else "–" },
            ),
            graph = scaled(samples.map { if (it.hasDisk()) it.disk.readRate + it.disk.writeRate else null }, floor = 1e6),
        )
        val n = now?.takeIf { it.hasNetwork() }?.network
        LoadCard(
            R.drawable.ic_network_fill, Color(0xFF30B0C7), stringResource(R.string.overview_network),
            told = now == null || n != null,
            value = n?.let { rate(it.receiveRate) },
            after = n?.let { stringResource(R.string.overview_download) },
            stats = listOf(
                stringResource(R.string.overview_upload) to n?.let { rate(it.sendRate) },
                stringResource(R.string.overview_received) to n?.let { bytes(it.receivedBytes) },
                stringResource(R.string.overview_sent) to n?.let { bytes(it.sentBytes) },
            ),
            graph = scaled(samples.map { if (it.hasNetwork()) it.network.receiveRate + it.network.sendRate else null }, floor = 1e5),
        )
    }
}

/** Rates as shares of the highest of them, or of floor when all are lower. */
private fun scaled(values: List<Double?>, floor: Double): List<Double?> {
    val top = maxOf(floor, values.filterNotNull().maxOrNull() ?: 0.0)
    return values.map { it?.div(top) }
}

/**
 * A card of the load: its tile and title, what it is now, large, three
 * figures more and the graph of the last samples. told is false where the
 * server's system does not tell it; while no sample came, the figures wait.
 */
@Composable
private fun LoadCard(
    @DrawableRes icon: Int,
    color: Color,
    title: String,
    told: Boolean,
    value: String?,
    stats: List<Pair<String, String?>>,
    graph: List<Double?>,
    badge: Boolean? = null,
    after: String? = null,
) {
    val colors = theme.colors
    Group {
        Column(Modifier.padding(16.dp), verticalArrangement = Arrangement.spacedBy(MavioSpacing.s12)) {
            Row(horizontalArrangement = Arrangement.spacedBy(MavioSpacing.s8), verticalAlignment = Alignment.CenterVertically) {
                Box(Modifier.size(24.dp).background(color, RoundedCornerShape(6.dp)), Alignment.Center) {
                    MavioIcon(icon, size = 14.dp, tint = Color.White)
                }
                Text(title, Modifier.weight(1f), style = theme.type.headline.text, color = color)
                if (badge != null) {
                    Text(
                        stringResource(if (badge) R.string.overview_memory_high else R.string.overview_memory_normal),
                        Modifier
                            .background((if (badge) Color(0xFFFF9500) else Color(0xFF34C759)).copy(alpha = 0.15f), CircleShape)
                            .padding(horizontal = 8.dp, vertical = 2.dp),
                        style = theme.type.caption.text,
                        color = if (badge) Color(0xFFC93400) else Color(0xFF248A3D),
                    )
                }
            }
            if (!told) {
                Text(stringResource(R.string.overview_load_unknown), style = theme.type.callout.text, color = colors.textSecondary)
                return@Column
            }
            Row(horizontalArrangement = Arrangement.spacedBy(MavioSpacing.s8), verticalAlignment = Alignment.Bottom) {
                Text(
                    value ?: "…",
                    style = theme.type.title2.text,
                    fontWeight = FontWeight.SemiBold,
                    color = if (value == null) colors.textTertiary else colors.textPrimary,
                )
                if (value != null && after != null) {
                    Text(after, Modifier.padding(bottom = 3.dp), style = theme.type.footnote.text, color = colors.textSecondary)
                }
            }
            Row(horizontalArrangement = Arrangement.spacedBy(MavioSpacing.s8)) {
                stats.forEach { (label, v) ->
                    Column(Modifier.weight(1f)) {
                        Text(label, style = theme.type.caption.text, color = colors.textSecondary, maxLines = 1)
                        Text(v ?: "…", style = theme.type.footnote.text, maxLines = 1)
                    }
                }
            }
            Dots(graph, color)
        }
    }
}

/** The samples a graph has room for: two minutes', as the server keeps. */
private const val COLUMNS = 60
private const val ROWS = 6

/**
 * A graph of dots: a column per sample, the latest at the right, lit from
 * the bottom as high as its value, from 0 to 1; columns without a sample
 * stay unlit.
 */
@Composable
private fun Dots(values: List<Double?>, color: Color) {
    val unlit = theme.colors.textSecondary.copy(alpha = 0.12f)
    Canvas(Modifier.fillMaxWidth().aspectRatio(COLUMNS.toFloat() / ROWS)) {
        val cell = size.width / COLUMNS
        val shown = values.takeLast(COLUMNS)
        val offset = COLUMNS - shown.size
        for (x in 0 until COLUMNS) {
            val v = if (x >= offset) shown[x - offset] else null
            // A value above nothing lights a dot, so that activity shows.
            val lit = v?.let { if (it > 0) maxOf(1, Math.round(minOf(1.0, it) * ROWS).toInt()) else 0 } ?: 0
            for (y in 0 until ROWS) {
                drawCircle(
                    if (ROWS - y <= lit) color else unlit,
                    radius = cell * 0.28f,
                    center = Offset((x + 0.5f) * cell, (y + 0.5f) * cell),
                )
            }
        }
    }
}

/** Devices: the devices signed in to the server as the user, each but this one signed out from here. */
@Composable
private fun DevicesPage(model: AppModel, serverId: String) {
    var devices by remember { mutableStateOf<List<DeviceSession>?>(null) }
    var failure by remember { mutableStateOf<String?>(null) }
    var busy by remember { mutableStateOf(false) }
    var controlling by remember { mutableStateOf<String?>(null) }
    var reload by remember { mutableIntStateOf(0) }
    val scope = rememberCoroutineScope()
    suspend fun load() {
        try {
            devices = model.client.devices(serverId)
            failure = null
        } catch (e: CancellationException) {
            throw e
        } catch (e: Exception) {
            failure = e.reason
        }
    }
    LaunchedEffect(serverId, reload) { load() }
    val context = LocalContext.current
    controlling?.let {
        RemoteSheet(model, serverId, it) {
            controlling = null
            reload++
        }
    }
    SettingsList(stringResource(R.string.settings_devices), stringResource(R.string.settings_devices_message)) {
        val list = devices
        when {
            list == null && failure == null -> item { StatusView(stringResource(R.string.state_loading)) }
            list != null -> item {
                Group(footer = failure) {
                    list.forEachIndexed { i, device ->
                        if (i > 0) RowDivider()
                        Row(
                            Modifier.fillMaxWidth().padding(horizontal = 16.dp, vertical = 10.dp),
                            horizontalArrangement = Arrangement.spacedBy(MavioSpacing.s12),
                            verticalAlignment = Alignment.CenterVertically,
                        ) {
                            Box(Modifier.size(36.dp).background(theme.colors.fill, RoundedCornerShape(9.dp)), Alignment.Center) {
                                MavioIcon(R.drawable.ic_devices, size = 20.dp)
                            }
                            Column(Modifier.weight(1f)) {
                                Text(deviceName(device), style = theme.type.body.text, maxLines = 1)
                                Text(
                                    deviceDetail(context, device),
                                    style = theme.type.footnote.text,
                                    color = theme.colors.textSecondary,
                                    maxLines = 1,
                                )
                            }
                            if (device.current) {
                                Text(stringResource(R.string.settings_this_device), style = theme.type.callout.text, color = theme.colors.textSecondary)
                            } else {
                                if (device.online) {
                                    Text(
                                        stringResource(R.string.remote_control),
                                        Modifier.clickable { controlling = device.id }.padding(vertical = 6.dp),
                                        style = theme.type.callout.text,
                                        color = theme.colors.accentText,
                                    )
                                }
                                Text(
                                    stringResource(R.string.auth_sign_out),
                                    Modifier
                                        .clickable(enabled = !busy) {
                                            busy = true
                                            scope.launch {
                                                try {
                                                    model.client.signOutDevice(serverId, device.id)
                                                } catch (e: CancellationException) {
                                                    throw e
                                                } catch (e: Exception) {
                                                    failure = e.reason
                                                }
                                                load()
                                                busy = false
                                            }
                                        }
                                        .padding(vertical = 6.dp),
                                    style = theme.type.callout.text,
                                    color = theme.colors.danger,
                                )
                            }
                        }
                    }
                }
            }
            else -> item { StatusView(failure.orEmpty(), icon = R.drawable.ic_error) }
        }
    }
}

/** A device's name, or its client's when it has none. */
internal fun deviceName(d: DeviceSession): String = d.deviceName.ifEmpty { d.client.ifEmpty { "—" } }

/** A device's client and what it does: what it plays, online, or when it was last seen. */
private fun deviceDetail(context: Context, d: DeviceSession): String {
    val state = when {
        d.playing != null -> context.getString(R.string.remote_playing, d.playing!!.card.title)
        d.online -> context.getString(R.string.remote_online)
        else -> d.lastSeen?.let {
            DateUtils.formatDateTime(
                context,
                it.toEpochMilli(),
                DateUtils.FORMAT_SHOW_DATE or DateUtils.FORMAT_SHOW_TIME or DateUtils.FORMAT_ABBREV_MONTH,
            )
        }
    }
    return listOfNotNull(d.client, state).filter { it.isNotEmpty() }.joinToString(" · ")
}
