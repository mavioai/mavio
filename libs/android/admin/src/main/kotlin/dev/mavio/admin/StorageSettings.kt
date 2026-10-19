package dev.mavio.admin

import android.annotation.SuppressLint
import android.content.Context
import android.icu.text.NumberFormat
import androidx.annotation.DrawableRes
import androidx.compose.foundation.Canvas
import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.aspectRatio
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.widthIn
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.text.selection.SelectionContainer
import androidx.compose.material3.LinearProgressIndicator
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableIntStateOf
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.geometry.Size
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.StrokeCap
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.platform.LocalFocusManager
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.text.SpanStyle
import androidx.compose.ui.text.buildAnnotatedString
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.text.input.KeyboardType
import androidx.compose.ui.text.withStyle
import androidx.compose.ui.unit.dp
import dev.mavio.core.AppModel
import dev.mavio.core.call
import dev.mavio.design.MavioSpacing
import dev.mavio.ui.ButtonKind
import dev.mavio.ui.Chevron
import dev.mavio.ui.ChoiceMenuBox
import dev.mavio.ui.ChoiceRow
import dev.mavio.ui.Group
import dev.mavio.ui.LinkRow
import dev.mavio.ui.LocalTelevision
import dev.mavio.ui.MavioIcon
import dev.mavio.ui.MavioSwitch
import dev.mavio.ui.Pressable
import dev.mavio.ui.PressableBox
import dev.mavio.ui.R
import dev.mavio.ui.RowDivider
import dev.mavio.ui.RowLabel
import dev.mavio.ui.SettingsButton
import dev.mavio.ui.SettingsField
import dev.mavio.ui.SettingsList
import dev.mavio.ui.StatusView
import dev.mavio.ui.SwitchRow
import dev.mavio.ui.ValueRow
import dev.mavio.ui.line
import dev.mavio.ui.rowPadding
import dev.mavio.ui.text
import dev.mavio.ui.theme
import java.text.Collator
import kotlinx.coroutines.async
import kotlinx.coroutines.coroutineScope
import kotlinx.coroutines.delay
import kotlinx.coroutines.launch
import mavio.storage.v1.Storage.ConfigQuestion
import mavio.storage.v1.Storage.ContinueRemoteResponse
import mavio.storage.v1.Storage.CreateRemoteResponse
import mavio.storage.v1.Storage.DeleteRemoteResponse
import mavio.storage.v1.Storage.Disk
import mavio.storage.v1.Storage.DiskKind
import mavio.storage.v1.Storage.Example
import mavio.storage.v1.Storage.GetRemoteUsageResponse
import mavio.storage.v1.Storage.GetStatusResponse
import mavio.storage.v1.Storage.ListDisksResponse
import mavio.storage.v1.Storage.ListFoldersResponse
import mavio.storage.v1.Storage.ListProvidersResponse
import mavio.storage.v1.Storage.ListRemotesResponse
import mavio.storage.v1.Storage.Option
import mavio.storage.v1.Storage.Provider
import mavio.storage.v1.Storage.Remote
import mavio.storage.v1.Storage.UpdateRemoteResponse
import mavio.storage.v1.continueRemoteRequest
import mavio.storage.v1.createRemoteRequest
import mavio.storage.v1.deleteRemoteRequest
import mavio.storage.v1.getRemoteUsageRequest
import mavio.storage.v1.getStatusRequest
import mavio.storage.v1.listDisksRequest
import mavio.storage.v1.listFoldersRequest
import mavio.storage.v1.listProvidersRequest
import mavio.storage.v1.listRemotesRequest
import mavio.storage.v1.updateRemoteRequest

// Storage (docs/clients.md §6.3.10), as the web UI's on a phone: the disks of
// the server's computer, the rclone the server runs or reaches, and its
// remotes — added with a kind's settings and the questions their
// configuration asks, such as authorizing a cloud drive — and their folders.
// The download clients of cloud drives' offline downloads feed acquisition
// and are left out (docs/clients.md §4.2).

private val Cloud = Color(0xFF30B0C7)
private val Green = Color(0xFF34C759)

/** The rclone remotes of a server, by name. */
private suspend fun remotes(model: AppModel, serverId: String): List<Remote> {
    val collator = Collator.getInstance()
    return model.client.call(serverId, Procedure.storage("ListRemotes"), listRemotesRequest {}, ListRemotesResponse::parseFrom)
        .remotesList.sortedWith { a, b -> collator.compare(a.name, b.name) }
}

/** The folders in a folder of a remote, "rclone:<remote>:<path>". */
private suspend fun remoteFolders(model: AppModel, serverId: String, path: String): List<String> =
    model.client.call(serverId, Procedure.storage("ListFolders"), listFoldersRequest { this.path = path }, ListFoldersResponse::parseFrom).foldersList

private suspend fun providers(model: AppModel, serverId: String): List<Provider> =
    model.client.call(serverId, Procedure.storage("ListProviders"), listProvidersRequest {}, ListProvidersResponse::parseFrom).providersList

/** A size in terabytes, gigabytes or megabytes as it is large. */
private fun capacity(bytes: Long): String {
    val n = bytes.toDouble()
    val (value, unit) = when {
        n >= 1e12 -> n / 1e12 to "TB"
        n >= 1e9 -> n / 1e9 to "GB"
        else -> n / 1e6 to "MB"
    }
    val format = NumberFormat.getInstance().apply { maximumFractionDigits = 1 }
    return "${format.format(value)} $unit"
}

/** The keys of the kinds of remotes whose names are not as they are. */
private val kindKeys = mapOf("b2" to "backblaze", "cloud115" to "cloud_one_fifteen", "cloud123" to "cloud_one_twenty_three", "s3" to "amazon")

/** A kind of remote's name in the UI's language, else as rclone describes it. */
// The names are the design system's strings by the kinds' keys.
@SuppressLint("DiscouragedApi")
private fun kindName(context: Context, providers: List<Provider>, type: String): String {
    val key = kindKeys[type] ?: type.trim().split(' ').joinToString("_") { it.lowercase() }
    val id = context.resources.getIdentifier("storage_provider_$key", "string", context.packageName)
    if (id != 0) return context.getString(id)
    return providers.firstOrNull { it.name == type }?.description?.ifEmpty { null } ?: type
}

/** A bar of how much of a disk or a remote is used, orange when nearly full and red when full, and its sizes. */
@Composable
private fun UsageBar(used: Long, total: Long) {
    val share = if (total > 0) (used.toFloat() / total).coerceIn(0f, 1f) else 0f
    Column(Modifier.padding(top = 4.dp), verticalArrangement = Arrangement.spacedBy(MavioSpacing.s4)) {
        LinearProgressIndicator(
            { share },
            Modifier.fillMaxWidth().height(4.dp),
            color = when {
                share >= 0.95f -> theme.colors.danger
                share >= 0.85f -> Color(0xFFFF9500)
                else -> Color(0xFF007AFF)
            },
            trackColor = theme.colors.fill,
            strokeCap = StrokeCap.Round,
            gapSize = 0.dp,
            drawStopIndicator = {},
        )
        Text(stringResource(R.string.storage_usage, capacity(used), capacity(total)), style = theme.type.footnote.text, color = theme.colors.textSecondary)
    }
}

/** A tile of a color with an icon. */
@Composable
private fun Tile(@DrawableRes icon: Int, color: Color, size: Int = 28) {
    Box(Modifier.size(size.dp).background(color, RoundedCornerShape((size / 4).dp)), Alignment.Center) {
        MavioIcon(icon, size = (size * 0.6).dp, tint = Color.White)
    }
}

/**
 * Storage: the disks with how much of each is used, rclone's remotes, each
 * with whether the server reaches it, to open and add, and rclone's status.
 */
@Composable
fun StoragePage(model: AppModel, serverId: String) {
    var status by remember { mutableStateOf<GetStatusResponse?>(null) }
    var statusFailure by remember { mutableStateOf<String?>(null) }
    var disks by remember { mutableStateOf<List<Disk>?>(null) }
    var disksFailure by remember { mutableStateOf<String?>(null) }
    var remotes by remember { mutableStateOf<List<Remote>?>(null) }
    var remotesFailure by remember { mutableStateOf<String?>(null) }
    var providers by remember { mutableStateOf<List<Provider>>(emptyList()) }
    val navigator = LocalAdmin.current
    val context = LocalContext.current
    suspend fun load() {
        coroutineScope {
            launch {
                attempt({ disksFailure = it }) {
                    disks = model.client.call(serverId, Procedure.storage("ListDisks"), listDisksRequest {}, ListDisksResponse::parseFrom).disksList
                    disksFailure = null
                }
            }
            attempt({ statusFailure = it }) {
                status = model.client.call(serverId, Procedure.storage("GetStatus"), getStatusRequest {}, GetStatusResponse::parseFrom)
                statusFailure = null
            }
            if (status?.available == true) {
                val p = async { runCatching { providers(model, serverId) }.getOrNull() }
                attempt({ remotesFailure = it }) {
                    remotes = remotes(model, serverId)
                    remotesFailure = null
                }
                p.await()?.let { providers = it }
            }
        }
    }
    LaunchedEffect(serverId) { load() }
    val available = status?.available == true
    SettingsList(stringResource(R.string.settings_storage), stringResource(R.string.storage_message), refresh = { load() }) {
        item {
            Group(stringResource(R.string.storage_disks), stringResource(R.string.storage_disks_footer)) {
                disks.orEmpty().forEachIndexed { i, d ->
                    if (i > 0) RowDivider(tiled = true)
                    DiskRow(d)
                }
                when {
                    disksFailure != null -> Note(disksFailure!!, failed = true)
                    disks?.isEmpty() == true -> Note(stringResource(R.string.storage_no_disks))
                    disks == null -> Note(stringResource(R.string.state_loading))
                }
            }
        }
        if (available) {
            item {
                Group(stringResource(R.string.storage_remotes), stringResource(R.string.storage_remotes_footer)) {
                    remotes.orEmpty().forEach { r ->
                        RemoteRow(model, serverId, r, kindName(context, providers, r.type)) { navigator.open(AdminPage.StorageRemote(serverId, r.name)) }
                        RowDivider(tiled = true)
                    }
                    if (remotes?.isEmpty() == true) {
                        Note(stringResource(R.string.storage_no_remotes))
                        RowDivider()
                    }
                    remotesFailure?.let {
                        Note(it, failed = true)
                        RowDivider()
                    }
                    LinkRow(stringResource(R.string.storage_add), R.drawable.ic_add) { navigator.open(AdminPage.NewRemote(serverId)) }
                }
            }
        }
        item {
            Group(stringResource(R.string.storage_rclone)) {
                val text = statusFailure ?: status?.let {
                    if (it.available) stringResource(R.string.storage_available) else it.error.ifEmpty { stringResource(R.string.storage_unavailable) }
                } ?: "…"
                Row(
                    Modifier.fillMaxWidth().padding(horizontal = rowPadding, vertical = 14.dp),
                    horizontalArrangement = Arrangement.spacedBy(MavioSpacing.s8),
                    verticalAlignment = Alignment.CenterVertically,
                ) {
                    Text(stringResource(R.string.storage_status), Modifier.weight(1f), style = theme.type.body.text)
                    Box(
                        Modifier.size(8.dp).background(
                            when {
                                status == null && statusFailure == null -> theme.colors.fill
                                available -> Green
                                else -> theme.colors.danger
                            },
                            CircleShape,
                        ),
                    )
                    Text(text, Modifier.widthIn(max = 220.dp), style = theme.type.body.text, color = theme.colors.textSecondary)
                }
                status?.takeIf { it.version.isNotEmpty() }?.let {
                    RowDivider()
                    ValueRow(stringResource(R.string.storage_version), it.version)
                }
                status?.takeIf { it.available }?.let {
                    RowDivider()
                    Row(
                        Modifier.fillMaxWidth().padding(horizontal = rowPadding, vertical = 10.dp),
                        horizontalArrangement = Arrangement.spacedBy(MavioSpacing.s12),
                        verticalAlignment = Alignment.CenterVertically,
                    ) {
                        RowLabel(
                            stringResource(R.string.storage_rcd),
                            stringResource(if (it.managed) R.string.storage_managed_detail else R.string.storage_external_detail),
                            Modifier.weight(1f),
                        )
                        Text(
                            stringResource(if (it.managed) R.string.storage_managed else R.string.storage_external),
                            style = theme.type.body.text,
                            color = theme.colors.textSecondary,
                        )
                    }
                }
            }
        }
    }
}

/** A disk of the server's computer: its name or path, its kind and file system, and how much of it is used. */
@Composable
private fun DiskRow(disk: Disk) {
    val (icon, color, label) = when (disk.kind) {
        DiskKind.DISK_KIND_SSD -> Triple(R.drawable.ic_disk_fill, Color(0xFF5E5CE6), R.string.storage_disk_ssd)
        DiskKind.DISK_KIND_HDD -> Triple(R.drawable.ic_disk_fill, Color(0xFF5E5CE6), R.string.storage_disk_hdd)
        DiskKind.DISK_KIND_NETWORK -> Triple(R.drawable.ic_network_fill, Color(0xFF007AFF), R.string.storage_disk_network)
        DiskKind.DISK_KIND_CLOUD -> Triple(R.drawable.ic_cloud_fill, Cloud, R.string.storage_disk_cloud)
        else -> Triple(R.drawable.ic_disk_fill, Color(0xFF8E8E93), R.string.storage_disk_unknown)
    }
    Row(Modifier.fillMaxWidth().padding(horizontal = rowPadding, vertical = 10.dp), horizontalArrangement = Arrangement.spacedBy(MavioSpacing.s16)) {
        Tile(icon, color)
        Column(Modifier.weight(1f), verticalArrangement = Arrangement.spacedBy(2.dp)) {
            Row(horizontalArrangement = Arrangement.spacedBy(MavioSpacing.s12), verticalAlignment = Alignment.CenterVertically) {
                Text(disk.name.ifEmpty { disk.path }, Modifier.weight(1f), style = theme.type.body.text, maxLines = 1)
                Text(
                    listOf(stringResource(label), disk.fileSystem).filter { it.isNotEmpty() }.joinToString(" · "),
                    style = theme.type.footnote.text,
                    color = theme.colors.textSecondary,
                    maxLines = 1,
                )
            }
            if (disk.name.isNotEmpty()) Text(disk.path, style = theme.type.footnote.text, color = theme.colors.textSecondary, maxLines = 1)
            if (disk.hasTotalBytes()) {
                UsageBar(disk.usedBytes, disk.totalBytes)
            } else {
                Text(stringResource(R.string.storage_no_answer), style = theme.type.footnote.text, color = theme.colors.textTertiary)
            }
        }
    }
}

/**
 * A remote in the list, with a dot telling whether the server reaches it:
 * listing its root, green when that succeeds and red, with the error, when
 * it fails, as a remote whose login expired does; and how much of it is
 * used, where its backend tells.
 */
@Composable
private fun RemoteRow(model: AppModel, serverId: String, remote: Remote, kind: String, onClick: () -> Unit) {
    // null while checking, else the error of listing its root, empty when reached.
    var reached by remember(remote.name) { mutableStateOf<String?>(null) }
    var usage by remember(remote.name) { mutableStateOf<GetRemoteUsageResponse?>(null) }
    LaunchedEffect(serverId, remote.name) {
        launch {
            usage = runCatching {
                model.client.call(serverId, Procedure.storage("GetRemoteUsage"), getRemoteUsageRequest { this.remote = remote.name }, GetRemoteUsageResponse::parseFrom)
            }.getOrNull()
        }
        attempt({ reached = it }) {
            remoteFolders(model, serverId, "rclone:${remote.name}:")
            reached = ""
        }
    }
    val used = usage?.let {
        when {
            it.hasUsedBytes() -> it.usedBytes
            it.hasTotalBytes() && it.hasFreeBytes() -> it.totalBytes - it.freeBytes
            else -> null
        }
    }
    Pressable(onClick, Modifier.fillMaxWidth()) {
        Row(
            Modifier.fillMaxWidth().padding(horizontal = rowPadding, vertical = 10.dp),
            horizontalArrangement = Arrangement.spacedBy(MavioSpacing.s16),
        ) {
            Tile(R.drawable.ic_cloud_fill, Cloud)
            Column(Modifier.weight(1f), verticalArrangement = Arrangement.spacedBy(2.dp)) {
                Row(horizontalArrangement = Arrangement.spacedBy(MavioSpacing.s8), verticalAlignment = Alignment.CenterVertically) {
                    Text(remote.name, style = theme.type.body.text, maxLines = 1)
                    Box(
                        Modifier.size(6.dp).background(
                            when {
                                reached == null -> theme.colors.fill
                                reached!!.isEmpty() -> Green
                                else -> theme.colors.danger
                            },
                            CircleShape,
                        ),
                    )
                }
                Text(kind, style = theme.type.footnote.text, color = theme.colors.textSecondary, maxLines = 1)
                val failed = reached?.takeIf { it.isNotEmpty() }
                when {
                    failed != null -> Text(failed, style = theme.type.footnote.text, color = theme.colors.danger, maxLines = 2)
                    used != null && usage!!.hasTotalBytes() && usage!!.totalBytes > 0 -> UsageBar(used, usage!!.totalBytes)
                    used != null -> Text(stringResource(R.string.storage_used, capacity(used)), style = theme.type.footnote.text, color = theme.colors.textSecondary)
                }
            }
            Box(Modifier.padding(top = 4.dp)) { Chevron() }
        }
    }
}

// Options

/** Whether an rclone provider filter, "AWS,Ceph" or "!AWS,Ceph", takes a provider. */
private fun providerMatches(filter: String, provider: String): Boolean {
    if (filter.isEmpty()) return true
    val negated = filter.startsWith("!")
    val list = filter.removePrefix("!").split(',').map { it.trim() }
    return (provider in list) != negated
}

/** The provider an S3-like kind's values choose, which its options depend on. */
private fun providerOf(options: List<Option>, values: Map<String, String>): String =
    values["provider"] ?: options.firstOrNull { it.name == "provider" }?.defaultValue.orEmpty()

/** The options of a kind shown for the values set: those its provider takes. */
private fun shownOptions(options: List<Option>, values: Map<String, String>, advanced: Boolean): List<Option> {
    val provider = providerOf(options, values)
    return options.filter { (advanced || !it.advanced || it.required) && providerMatches(it.provider, provider) }
}

/** The values to send: those set, differing from their defaults. */
private fun changedValues(options: List<Option>, values: Map<String, String>): Map<String, String> = values.filter { (k, v) ->
    val o = options.firstOrNull { it.name == k } ?: return@filter v.isNotEmpty()
    v != o.defaultValue && (v.isNotEmpty() || o.defaultValue.isEmpty()) && v.isNotBlank()
}

private fun exampleLabel(e: Example): String {
    val help = e.help.lineSequence().firstOrNull().orEmpty()
    return if (help.isEmpty()) e.value else "${e.value} — $help"
}

/** The control of an option's value: a switch, a choice among its examples, or a field suggesting them. */
@Composable
private fun OptionControl(option: Option, value: String, onChange: (String) -> Unit, provider: String, placeholder: String? = null) {
    val examples = option.examplesList.filter { providerMatches(it.provider, provider) }
    val current = value.ifEmpty { option.defaultValue }
    val tv = LocalTelevision.current
    when {
        option.type == "bool" -> PressableBox({ onChange(if (current == "true") "false" else "true") }, shape = CircleShape) {
            MavioSwitch(current == "true", if (tv) null else { on -> onChange(if (on) "true" else "false") })
        }
        examples.isNotEmpty() && option.exclusive -> ChoiceMenuBox(examples.map { it.value to exampleLabel(it) }, current, onChange, Modifier.fillMaxWidth()) { open ->
            PressableBox(open, Modifier.fillMaxWidth()) {
                Row(
                    Modifier.fillMaxWidth().clip(RoundedCornerShape(10.dp)).background(theme.colors.fill)
                        .padding(horizontal = 12.dp, vertical = 12.dp),
                    verticalAlignment = Alignment.CenterVertically,
                ) {
                    Text(
                        examples.firstOrNull { it.value == current }?.let(::exampleLabel) ?: current.ifEmpty { "—" },
                        Modifier.weight(1f),
                        style = theme.type.body.text,
                        maxLines = 1,
                    )
                    MavioIcon(R.drawable.ic_selector, size = 16.dp, tint = theme.colors.textSecondary)
                }
            }
        }
        else -> Row(horizontalArrangement = Arrangement.spacedBy(MavioSpacing.s4), verticalAlignment = Alignment.CenterVertically) {
            SettingsField(
                value,
                onChange,
                null,
                Modifier.weight(1f),
                placeholder ?: option.defaultValue.ifEmpty { null },
                secret = option.password,
                keyboard = if (option.type == "int") KeyboardType.Number else KeyboardType.Text,
            )
            if (examples.isNotEmpty() && !option.password) {
                ChoiceMenuBox(examples.map { it.value to exampleLabel(it) }, null, onChange) { open ->
                    PressableBox(open, Modifier.size(40.dp), onClickLabel = option.name, shape = CircleShape) {
                        MavioIcon(R.drawable.ic_expand, size = 16.dp, tint = theme.colors.textSecondary)
                    }
                }
            }
        }
    }
}

/** An option: its name, its whole help and its control. */
@Composable
private fun OptionField(option: Option, value: String, onChange: (String) -> Unit, provider: String, placeholder: String? = null) {
    val help = option.help.trim()
    val inline = option.type == "bool"
    val danger = theme.colors.danger
    Column(Modifier.fillMaxWidth().padding(horizontal = rowPadding, vertical = 10.dp), verticalArrangement = Arrangement.spacedBy(MavioSpacing.s8)) {
        Row(verticalAlignment = Alignment.CenterVertically) {
            Column(Modifier.weight(1f), verticalArrangement = Arrangement.spacedBy(2.dp)) {
                Text(
                    buildAnnotatedString {
                        withStyle(SpanStyle(fontFamily = FontFamily.Monospace)) { append(option.name) }
                        if (option.required) withStyle(SpanStyle(color = danger)) { append(" *") }
                    },
                    style = theme.type.body.text,
                )
                if (help.isNotEmpty()) Text(help, style = theme.type.caption.text, color = theme.colors.textSecondary)
            }
            if (inline) OptionControl(option, value, onChange, provider, placeholder)
        }
        if (!inline) OptionControl(option, value, onChange, provider, placeholder)
    }
}

/** A remote's settings: its kind's options, with those rarely changed behind a switch. */
private fun androidx.compose.foundation.lazy.LazyListScope.optionsGroup(
    options: List<Option>,
    values: Map<String, String>,
    onChange: (String, String) -> Unit,
    advanced: Boolean,
    onAdvanced: (Boolean) -> Unit,
    placeholder: @Composable (Option) -> String? = { null },
) {
    item {
        val provider = providerOf(options, values)
        Group(stringResource(R.string.storage_settings), stringResource(R.string.storage_settings_footer)) {
            val shown = shownOptions(options, values, advanced)
            shown.forEachIndexed { i, o ->
                if (i > 0) RowDivider()
                OptionField(o, values[o.name].orEmpty(), { onChange(o.name, it) }, provider, placeholder(o))
            }
            if (options.any { it.advanced && !it.required }) {
                if (shown.isNotEmpty()) RowDivider()
                SwitchRow(stringResource(R.string.storage_advanced), advanced) { onAdvanced(it) }
            }
        }
    }
}

// Remotes

private val remoteName = Regex("[A-Za-z0-9_][A-Za-z0-9_.+@-]*")

/**
 * The page adding a remote: its name, its kind and the kind's settings,
 * then the questions of its configuration, and the remote's page once it is
 * added.
 */
@Composable
fun NewRemoteScreen(model: AppModel, route: AdminPage.NewRemote) {
    val serverId = route.serverId
    var providers by remember { mutableStateOf<List<Provider>>(emptyList()) }
    var taken by remember { mutableStateOf<List<String>>(emptyList()) }
    var name by remember { mutableStateOf("") }
    var type by remember { mutableStateOf("") }
    var values by remember { mutableStateOf<Map<String, String>>(emptyMap()) }
    var advanced by remember { mutableStateOf(false) }
    var question by remember { mutableStateOf<ConfigQuestion?>(null) }
    var created by remember { mutableStateOf(false) }
    var saving by remember { mutableStateOf(false) }
    var failure by remember { mutableStateOf<String?>(null) }
    val scope = rememberCoroutineScope()
    val navigator = LocalAdmin.current
    val focus = LocalFocusManager.current
    val context = LocalContext.current
    LaunchedEffect(serverId) {
        attempt({ failure = it }) {
            val list = providers(model, serverId)
            val collator = Collator.getInstance()
            providers = list.sortedWith { a, b -> collator.compare(kindName(context, list, a.name), kindName(context, list, b.name)) }
        }
        runCatching { taken = remotes(model, serverId).map { it.name } }
    }
    val trimmed = name.trim()
    if (created) {
        RemotePage(model, serverId, trimmed)
        return
    }
    question?.let { q ->
        RemoteQuestions(model, serverId, trimmed, q, done = { created = true }) {
            // Cancelling the questions of a new remote deletes it.
            scope.launch {
                runCatching {
                    model.client.call(serverId, Procedure.storage("DeleteRemote"), deleteRemoteRequest { this.name = trimmed }, DeleteRemoteResponse::parseFrom)
                }
                navigator.back()
            }
        }
        return
    }
    val provider = providers.firstOrNull { it.name == type }
    val valid = remoteName.matches(trimmed) && trimmed !in taken
    val missing = provider == null ||
        shownOptions(provider.optionsList, values, false).any { it.required && (values[it.name] ?: it.defaultValue).isEmpty() }
    SettingsList(stringResource(R.string.storage_new)) {
        item {
            Group {
                TextRow(
                    stringResource(R.string.storage_name),
                    name,
                    { name = it },
                    detail = stringResource(if (trimmed in taken) R.string.storage_name_taken else R.string.storage_name_detail),
                    error = trimmed.isNotEmpty() && !valid,
                )
                RowDivider()
                ChoiceRow(
                    stringResource(R.string.storage_kind),
                    listOf("" to stringResource(R.string.storage_choose_kind)) + providers.map { it.name to "${kindName(context, providers, it.name)} (${it.name})" },
                    type,
                ) {
                    type = it
                    values = emptyMap()
                }
            }
        }
        provider?.let { p ->
            optionsGroup(p.optionsList, values, { k, v -> values = values + (k to v) }, advanced, { advanced = it })
        }
        item {
            Row(horizontalArrangement = Arrangement.spacedBy(MavioSpacing.s12), verticalAlignment = Alignment.CenterVertically) {
                SettingsButton(
                    if (saving) "…" else stringResource(R.string.storage_create),
                    {
                        val p = provider ?: return@SettingsButton
                        focus.clearFocus()
                        saving = true
                        failure = null
                        scope.launch {
                            attempt({ failure = it }) {
                                val r = model.client.call(
                                    serverId,
                                    Procedure.storage("CreateRemote"),
                                    createRemoteRequest {
                                        this.name = trimmed
                                        this.type = type
                                        parameters.putAll(changedValues(p.optionsList, values))
                                    },
                                    CreateRemoteResponse::parseFrom,
                                )
                                if (r.hasQuestion()) question = r.question else created = true
                            }
                            saving = false
                        }
                    },
                    enabled = valid && !missing && !saving,
                )
                failure?.let { Text(it, style = theme.type.callout.text, color = theme.colors.danger) }
            }
        }
    }
}

/** A remote's page. */
@Composable
fun StorageRemoteScreen(model: AppModel, route: AdminPage.StorageRemote) {
    RemotePage(model, route.serverId, route.name)
}

/** Secrets are never listed: an empty one keeps what is set. */
private fun secret(o: Option) = o.password || o.sensitive

/**
 * A remote's page: its kind's settings, configuring it again, its folders
 * and deleting it; the questions of its configuration take the page while
 * they are answered.
 */
@Composable
private fun RemotePage(model: AppModel, serverId: String, name: String) {
    var remote by remember { mutableStateOf<Remote?>(null) }
    var gone by remember { mutableStateOf(false) }
    var providers by remember { mutableStateOf<List<Provider>>(emptyList()) }
    var values by remember { mutableStateOf<Map<String, String>>(emptyMap()) }
    var advanced by remember { mutableStateOf(false) }
    var question by remember { mutableStateOf<ConfigQuestion?>(null) }
    var saving by remember { mutableStateOf(false) }
    var notice by remember { mutableStateOf<String?>(null) }
    var failure by remember { mutableStateOf<String?>(null) }
    var confirming by remember { mutableStateOf(false) }
    var reload by remember { mutableIntStateOf(0) }
    val scope = rememberCoroutineScope()
    val navigator = LocalAdmin.current
    val focus = LocalFocusManager.current
    val context = LocalContext.current
    val savedNotice = stringResource(R.string.storage_saved)
    val reconfiguredNotice = stringResource(R.string.storage_reconfigured)
    LaunchedEffect(serverId, name, reload) {
        attempt({ failure = it }) {
            coroutineScope {
                val p = async { providers(model, serverId) }
                val found = remotes(model, serverId).firstOrNull { it.name == name }
                providers = p.await()
                remote = found
                gone = found == null
                values = found?.parametersMap.orEmpty()
                failure = null
            }
        }
    }
    question?.let { q ->
        RemoteQuestions(
            model,
            serverId,
            name,
            q,
            done = {
                question = null
                notice = savedNotice
                reload++
            },
        ) { question = null }
        return
    }
    val r = remote
    val provider = providers.firstOrNull { it.name == r?.type }
    val changes = if (r == null || provider == null) {
        emptyMap()
    } else {
        values.filter { (k, v) ->
            val o = provider.optionsList.firstOrNull { it.name == k }
            if (o != null && secret(o)) v.isNotEmpty() else v != r.parametersMap[k].orEmpty()
        }
    }
    fun update(parameters: Map<String, String>, done: String) {
        focus.clearFocus()
        saving = true
        failure = null
        scope.launch {
            attempt({ failure = it }) {
                val u = model.client.call(
                    serverId,
                    Procedure.storage("UpdateRemote"),
                    updateRemoteRequest {
                        this.name = name
                        this.parameters.putAll(parameters)
                    },
                    UpdateRemoteResponse::parseFrom,
                )
                if (u.hasQuestion()) {
                    question = u.question
                } else {
                    if (u.hasRemote()) remote = u.remote
                    values = remote?.parametersMap.orEmpty()
                    notice = done
                }
            }
            saving = false
        }
    }
    SettingsList(name) {
        if (r == null) {
            item {
                when {
                    gone -> StatusView(stringResource(R.string.storage_no_remote))
                    failure != null -> StatusView(failure!!, icon = R.drawable.ic_error)
                    else -> StatusView(stringResource(R.string.state_loading))
                }
            }
            return@SettingsList
        }
        item {
            Row(horizontalArrangement = Arrangement.spacedBy(MavioSpacing.s12), verticalAlignment = Alignment.CenterVertically) {
                Tile(R.drawable.ic_cloud_fill, Cloud, 40)
                Text(kindName(context, providers, r.type), style = theme.type.title3.text, color = theme.colors.textSecondary)
            }
        }
        provider?.let { p ->
            optionsGroup(
                p.optionsList,
                values,
                { k, v ->
                    values = values + (k to v)
                    notice = null
                },
                advanced,
                { advanced = it },
            ) { o -> if (secret(o)) stringResource(R.string.storage_secret_kept) else null }
        }
        item {
            Row(horizontalArrangement = Arrangement.spacedBy(MavioSpacing.s12), verticalAlignment = Alignment.CenterVertically) {
                SettingsButton(if (saving) "…" else stringResource(R.string.action_save), { update(changes, savedNotice) }, enabled = changes.isNotEmpty() && !saving)
                SettingsButton(stringResource(R.string.storage_reconfigure), { update(emptyMap(), reconfiguredNotice) }, kind = ButtonKind.Secondary, enabled = !saving)
            }
        }
        if (failure != null || notice != null && changes.isEmpty()) {
            item {
                Text(
                    failure ?: notice!!,
                    style = theme.type.callout.text,
                    color = if (failure != null) theme.colors.danger else theme.colors.textSecondary,
                )
            }
        }
        item { RemoteFolders(model, serverId, r.name) }
        item {
            Group {
                ActionRow(stringResource(R.string.storage_remove), theme.colors.danger, true) { confirming = true }
            }
        }
    }
    if (confirming) {
        ConfirmDialog(
            stringResource(R.string.storage_remove_confirm, name),
            stringResource(R.string.storage_remove_detail),
            stringResource(R.string.storage_remove),
            onConfirm = {
                scope.launch {
                    attempt({ failure = it }) {
                        model.client.call(serverId, Procedure.storage("DeleteRemote"), deleteRemoteRequest { this.name = name }, DeleteRemoteResponse::parseFrom)
                        navigator.back()
                    }
                }
            },
            onDismiss = { confirming = false },
        )
    }
}

/** A remote's folders, to walk and to see that it is reached. */
@Composable
private fun RemoteFolders(model: AppModel, serverId: String, remote: String) {
    val root = "rclone:$remote:"
    var here by remember(remote) { mutableStateOf(root) }
    var folders by remember { mutableStateOf<List<String>?>(null) }
    var failure by remember { mutableStateOf<String?>(null) }
    LaunchedEffect(here) {
        folders = null
        failure = null
        attempt({ failure = it }) { folders = remoteFolders(model, serverId, here) }
    }
    Group(stringResource(R.string.storage_folders), stringResource(R.string.storage_folders_footer)) {
        SelectionContainer {
            Text(here, Modifier.padding(horizontal = rowPadding, vertical = 12.dp), style = theme.type.callout.text, color = theme.colors.textSecondary)
        }
        if (here != root) {
            RowDivider()
            LinkRow(stringResource(R.string.libraries_parent_folder), R.drawable.ic_back) {
                here = if ('/' in here) here.substringBeforeLast('/') else root
            }
        }
        folders.orEmpty().forEach { f ->
            RowDivider()
            LinkRow(f.substring(maxOf(f.lastIndexOf('/'), f.lastIndexOf(':')) + 1), R.drawable.ic_folder) { here = f }
        }
        when {
            failure != null -> Note(failure!!, failed = true)
            folders?.isEmpty() == true -> Note(stringResource(R.string.libraries_no_folders))
            folders == null -> Note(stringResource(R.string.state_loading))
        }
    }
}

// Questions

/**
 * A QR code an option's help draws for a terminal: rows of half blocks,
 * each character two modules high, a block where a module is light; dark
 * holds its modules, true where dark.
 */
private class HelpQR(val before: String, val dark: List<List<Boolean>>, val after: String) {
    companion object {
        fun of(help: String): HelpQR? {
            val lines = help.split("\n")
            fun art(l: String) = l.length >= 21 && l.all { it in " █▀▄" } && l.any { it in "█▀▄" }
            val start = lines.indexOfFirst(::art).takeIf { it >= 0 } ?: return null
            var end = start
            while (end < lines.size && art(lines[end])) end++
            val dark = mutableListOf<List<Boolean>>()
            for (i in start until end) {
                val l = lines[i]
                dark += l.map { it != '█' && it != '▀' }
                // An odd last row is drawn as the top halves of its line alone.
                if (i < end - 1 || l.any { it == '█' || it == '▄' }) dark += l.map { it != '█' && it != '▄' }
            }
            if (dark.size < 21) return null
            return HelpQR(lines.take(start).joinToString("\n").trim(), dark, lines.drop(end).joinToString("\n").trim())
        }
    }
}

/** A QR code, drawn dark on light whatever the theme, for a phone to scan. */
@Composable
private fun QRCode(dark: List<List<Boolean>>, modifier: Modifier = Modifier) {
    val width = dark.maxOfOrNull { it.size } ?: 0
    Box(modifier.widthIn(max = 224.dp).clip(RoundedCornerShape(8.dp)).background(Color.White).padding(12.dp)) {
        Canvas(Modifier.fillMaxWidth().aspectRatio(width.toFloat() / dark.size.coerceAtLeast(1))) {
            val module = minOf(size.width / width, size.height / dark.size)
            dark.forEachIndexed { y, row ->
                row.forEachIndexed { x, on ->
                    if (on) drawRect(Color.Black, Offset(x * module, y * module), Size(module + 0.5f, module + 0.5f))
                }
            }
        }
    }
}

/**
 * The questions of a remote's configuration — a yes or no, a choice, a
 * value, or the token of authorizing a cloud drive — answered until none
 * is left. A question drawing a QR code, as 115's login asks, is answered
 * yes again and again until the phone confirms the login, its code kept on
 * screen while rclone asks again without it. Cancelling waits for the
 * answer being sent, then calls cancel.
 */
@Composable
private fun RemoteQuestions(model: AppModel, serverId: String, name: String, question: ConfigQuestion, done: () -> Unit, cancel: () -> Unit) {
    var q by remember { mutableStateOf(question) }
    var answer by remember { mutableStateOf(question.option.defaultValue) }
    var scan by remember { mutableStateOf<Pair<String, HelpQR>?>(null) }
    var pending by remember { mutableStateOf(false) }
    var failure by remember { mutableStateOf<String?>(null) }
    var cancelled by remember { mutableStateOf(false) }
    val scope = rememberCoroutineScope()
    val focus = LocalFocusManager.current
    val option = if (q.hasOption()) q.option else null
    val drawn = option?.let { HelpQR.of(it.help) }
    val qr = drawn ?: scan?.takeIf { it.first == q.state }?.second
    val confirm = option?.type == "bool"
    val waiting = qr != null && confirm && failure == null && !cancelled
    fun send(value: String) {
        focus.clearFocus()
        pending = true
        failure = null
        val wasDrawn = drawn
        scope.launch {
            try {
                val r = model.client.call(
                    serverId,
                    Procedure.storage("ContinueRemote"),
                    continueRemoteRequest {
                        this.name = name
                        state = q.state
                        this.answer = value
                    },
                    ContinueRemoteResponse::parseFrom,
                )
                when {
                    cancelled -> cancel()
                    !r.hasQuestion() -> done()
                    else -> {
                        if (wasDrawn != null) scan = q.state to wasDrawn
                        q = r.question
                        answer = r.question.option.defaultValue
                    }
                }
            } catch (e: kotlinx.coroutines.CancellationException) {
                throw e
            } catch (e: Exception) {
                if (cancelled) cancel() else failure = e.reason
            } finally {
                pending = false
            }
        }
    }
    // Each answer waits for the last; a code waiting to be scanned is answered yes again every few seconds.
    LaunchedEffect(q.state, waiting, pending) {
        if (!waiting || pending) return@LaunchedEffect
        delay(2_500)
        send("true")
    }
    SettingsList(name, stringResource(R.string.storage_question_message)) {
        item {
            Group(footer = if (option?.name == "config_token") stringResource(R.string.storage_question_footer) else null) {
                Column(Modifier.fillMaxWidth().padding(16.dp), verticalArrangement = Arrangement.spacedBy(MavioSpacing.s12)) {
                    if (option != null) {
                        if (qr != null) {
                            if (qr.before.isNotEmpty()) Text(qr.before, style = theme.type.body.text)
                            QRCode(qr.dark, Modifier.align(Alignment.CenterHorizontally))
                            if (!cancelled) {
                                Text(
                                    if (waiting) stringResource(R.string.storage_waiting_scan) else qr.after,
                                    style = theme.type.footnote.text,
                                    color = theme.colors.textSecondary,
                                )
                            }
                        } else {
                            val help = option.help.trim()
                            if (help.isNotEmpty()) SelectionContainer { Text(help, style = theme.type.body.text) }
                        }
                        if (!confirm) OptionControl(option, answer, { answer = it }, "")
                    } else {
                        Text(stringResource(R.string.storage_question_none), style = theme.type.body.text, color = theme.colors.textSecondary)
                    }
                    if (q.error.isNotEmpty()) Text(q.error, style = theme.type.body.text, color = theme.colors.danger)
                }
            }
        }
        item {
            Column(verticalArrangement = Arrangement.spacedBy(MavioSpacing.s12)) {
                Row(horizontalArrangement = Arrangement.spacedBy(MavioSpacing.s12)) {
                    if (!waiting && !cancelled) {
                        if (confirm) {
                            SettingsButton(stringResource(R.string.storage_yes), { send("true") }, enabled = !pending)
                            SettingsButton(stringResource(R.string.storage_no), { send("false") }, kind = ButtonKind.Secondary, enabled = !pending)
                        } else {
                            SettingsButton(if (pending) "…" else stringResource(R.string.storage_continue), { send(answer) }, enabled = !pending)
                        }
                    }
                    SettingsButton(
                        stringResource(if (cancelled) R.string.storage_cancelling else R.string.action_cancel),
                        {
                            cancelled = true
                            if (!pending) cancel()
                        },
                        kind = ButtonKind.Secondary,
                        enabled = !cancelled,
                    )
                }
                failure?.let { Text(it, style = theme.type.callout.text, color = theme.colors.danger) }
            }
        }
    }
}
