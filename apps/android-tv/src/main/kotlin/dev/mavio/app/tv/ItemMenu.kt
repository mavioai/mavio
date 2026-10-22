package dev.mavio.app.tv

import androidx.annotation.StringRes
import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.widthIn
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.material3.HorizontalDivider
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableIntStateOf
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.setValue
import androidx.compose.runtime.withFrameNanos
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.focus.FocusRequester
import androidx.compose.ui.focus.focusRequester
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.input.KeyboardType
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.Dp
import androidx.compose.ui.unit.dp
import dev.mavio.core.AppModel
import dev.mavio.core.Card
import dev.mavio.core.CardKind
import dev.mavio.core.Details
import dev.mavio.core.DeviceSession
import dev.mavio.core.ItemList
import dev.mavio.core.Source
import dev.mavio.core.addToList
import dev.mavio.core.createList
import dev.mavio.core.deleteList
import dev.mavio.core.identifiableKinds
import dev.mavio.core.issueTypes
import dev.mavio.core.itemLists
import dev.mavio.core.itemRights
import dev.mavio.core.listable
import dev.mavio.core.playOn
import dev.mavio.core.playableKinds
import dev.mavio.core.refreshItem
import dev.mavio.core.renameList
import dev.mavio.core.reportIssue
import dev.mavio.core.reportableKinds
import dev.mavio.design.MavioSpacing
import dev.mavio.ui.ButtonKind
import dev.mavio.ui.Focusable
import dev.mavio.ui.Group
import dev.mavio.ui.MavioIcon
import dev.mavio.ui.Pressable
import dev.mavio.ui.R
import dev.mavio.ui.RowDivider
import dev.mavio.ui.RowLabel
import dev.mavio.ui.SettingsButton
import dev.mavio.ui.SettingsField
import dev.mavio.ui.SwitchRow
import dev.mavio.ui.TvChoice
import dev.mavio.ui.TvDialog
import dev.mavio.ui.TvMenu
import dev.mavio.ui.TvMenuItem
import dev.mavio.ui.TvMenuLabel
import dev.mavio.ui.focused
import dev.mavio.ui.format
import dev.mavio.ui.range
import dev.mavio.ui.resolution
import dev.mavio.ui.selectable
import dev.mavio.ui.shakes
import dev.mavio.ui.sizeText
import dev.mavio.ui.surround
import dev.mavio.ui.text
import dev.mavio.ui.theme
import dev.mavio.ui.versionName
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.launch

/** How far a menu over the actions lies above them: a button's height and the gap. */
private val ABOVE: Dp = -(64.dp + MavioSpacing.s8)

/**
 * The version to play, then the same title on the user's other servers
 * (docs/tv.md §5): a glass pill naming the version, white with focus and
 * filled while its menu is open, which opens the menu above it at its end,
 * with focus on the version chosen.
 */
@Composable
internal fun TvVersionMenu(
    details: Details,
    own: List<Source>,
    copies: List<Source>,
    source: Source?,
    modifier: Modifier = Modifier,
    onSource: (Source) -> Unit,
) {
    val pages = LocalPages.current
    val context = LocalContext.current
    var open by remember { mutableStateOf(false) }
    val button = remember { FocusRequester() }
    val chosen = remember { FocusRequester() }
    val state = remember { Focusable() }
    val focused = state.focused()
    val close = {
        open = false
        runCatching { button.requestFocus() }
        Unit
    }
    Box {
        Row(
            modifier
                .shakes()
                .focusRequester(button)
                .heightIn(min = 64.dp)
                .background(
                    when {
                        focused -> Color.White
                        open -> theme.colors.fill
                        else -> Color.White.copy(alpha = 0.2f)
                    },
                    CircleShape,
                )
                .selectable(state) { open = true }
                .padding(horizontal = MavioSpacing.s24),
            horizontalArrangement = Arrangement.spacedBy(MavioSpacing.s8),
            verticalAlignment = Alignment.CenterVertically,
        ) {
            val ink = if (focused) Color.Black else Color.White
            val style = theme.type.headline.text
            Text(stringResource(R.string.item_version), style = style, fontWeight = FontWeight.SemiBold, color = ink.copy(alpha = 0.7f))
            Text(
                source?.versionName ?: details.serverName,
                Modifier.widthIn(max = 320.dp),
                style = style,
                fontWeight = FontWeight.SemiBold,
                color = ink,
                maxLines = 1,
                overflow = TextOverflow.Ellipsis,
            )
            MavioIcon(R.drawable.ic_expand, size = 18.dp, tint = ink)
        }
        if (open) {
            TvMenu(chosen, 440.dp, Alignment.BottomEnd, ABOVE, onDismiss = close) {
                if (own.size > 1) {
                    TvMenuLabel(stringResource(R.string.item_versions))
                    own.forEachIndexed { i, s ->
                        val current = s == source || (source == null && i == 0)
                        TvMenuItem(
                            {
                                onSource(s)
                                close()
                            },
                            if (current) Modifier.focusRequester(chosen) else Modifier,
                        ) {
                            Column(Modifier.weight(1f)) {
                                Text(s.versionName, style = theme.type.headline.text, maxLines = 1, overflow = TextOverflow.Ellipsis)
                                Text(
                                    listOfNotNull(s.resolution, s.range, s.surround, s.format, s.sizeText(context)).joinToString(" · "),
                                    style = theme.type.footnote.text,
                                    color = theme.colors.textSecondary,
                                    maxLines = 1,
                                    overflow = TextOverflow.Ellipsis,
                                )
                            }
                            if (current) MavioIcon(R.drawable.ic_check, size = 24.dp, tint = theme.colors.accentText)
                        }
                    }
                }
                copies.forEachIndexed { i, copy ->
                    TvMenuItem(
                        {
                            close()
                            pages.openItem(copy.serverId, copy.itemId)
                        },
                        if (own.size <= 1 && i == 0) Modifier.focusRequester(chosen) else Modifier,
                    ) {
                        MavioIcon(R.drawable.ic_server, size = 24.dp, tint = theme.colors.textSecondary)
                        Text(
                            stringResource(R.string.item_on_server, copy.serverName),
                            Modifier.weight(1f),
                            style = theme.type.headline.text,
                            maxLines = 1,
                            overflow = TextOverflow.Ellipsis,
                        )
                        MavioIcon(R.drawable.ic_forward, size = 18.dp, tint = theme.colors.textTertiary)
                    }
                }
            }
        }
    }
}

/** A dialog of the More menu. */
private enum class ItemDialog(@StringRes val label: Int) {
    PlayOn(R.string.remote_play_on),
    Together(R.string.syncplay_watch_together),
    Playlist(R.string.lists_add_to_playlist),
    Report(R.string.issues_report),
    Collection(R.string.lists_add_to_collection),
    Refresh(R.string.metadata_refresh),
    Edit(R.string.metadata_edit),
    Identify(R.string.metadata_identify),
    Images(R.string.metadata_images),
    Rename(R.string.lists_rename),
    Delete(R.string.lists_delete),
}

/**
 * The More menu of an item's actions, as the web UI's (docs/tv.md §5): a
 * circle opening its menu above it, and the dialogs it opens: playing the
 * item on another device, watching it together, adding it to a playlist, reporting a problem,
 * and for administrators adding it to a collection, editing and refreshing
 * its metadata, identifying it and choosing its images; a playlist, or a collection for administrators, renamed or
 * deleted.
 */
@Composable
internal fun TvMoreMenu(model: AppModel, details: Details, modifier: Modifier = Modifier) {
    val card = details.card
    if (details.offline) return
    // What the user may do on the server, once known.
    var admin by remember(card.serverId) { mutableStateOf(false) }
    var mayReport by remember(card.serverId) { mutableStateOf(false) }
    LaunchedEffect(card.serverId) {
        runCatching { model.itemRights(card.serverId) }.onSuccess {
            admin = it.admin
            mayReport = it.mayReport
        }
    }
    val own = card.kind == CardKind.PLAYLIST || (card.kind == CardKind.COLLECTION && admin)
    // The entries in groups, separated, as the web UI's: playing, adding to
    // lists, metadata, reporting, and renaming or deleting a list, the deletion last.
    val groups = listOf(
        if (card.kind in playableKinds) listOf(ItemDialog.PlayOn, ItemDialog.Together) else emptyList(),
        if (listable(card.kind)) listOfNotNull(ItemDialog.Playlist, ItemDialog.Collection.takeIf { admin }) else emptyList(),
        if (admin && card.kind != CardKind.PLAYLIST) {
            listOfNotNull(
                ItemDialog.Edit,
                ItemDialog.Images,
                ItemDialog.Identify.takeIf { card.kind in identifiableKinds },
                ItemDialog.Refresh.takeIf { card.kind != CardKind.COLLECTION },
            )
        } else {
            emptyList()
        },
        if (card.kind in reportableKinds && mayReport) listOf(ItemDialog.Report) else emptyList(),
        if (own) listOf(ItemDialog.Rename, ItemDialog.Delete) else emptyList(),
    ).filter { it.isNotEmpty() }
    if (groups.isEmpty()) return
    val pages = LocalPages.current
    var open by remember { mutableStateOf(false) }
    var dialog by remember { mutableStateOf<ItemDialog?>(null) }
    val button = remember { FocusRequester() }
    val first = remember { FocusRequester() }
    // Outlives the dialogs, which close as their work starts.
    val scope = rememberCoroutineScope()
    val back = {
        open = false
        dialog = null
        runCatching { button.requestFocus() }
        Unit
    }
    Box {
        TvCircleButton(R.drawable.ic_more, stringResource(R.string.lists_more), modifier.focusRequester(button)) { open = true }
        if (open) {
            TvMenu(first, 360.dp, Alignment.BottomStart, ABOVE, onDismiss = back) {
                groups.forEachIndexed { g, group ->
                    if (g > 0) HorizontalDivider(Modifier.padding(vertical = MavioSpacing.s4), color = theme.colors.border)
                    group.forEachIndexed { i, e ->
                        TvMenuItem(
                            {
                                open = false
                                when (e) {
                                    ItemDialog.Edit -> pages.openMetadata(MetadataPage.Edit, card)
                                    ItemDialog.Identify -> pages.openMetadata(MetadataPage.Identify, card)
                                    ItemDialog.Images -> pages.openMetadata(MetadataPage.Images, card)
                                    else -> dialog = e
                                }
                            },
                            if (g == 0 && i == 0) Modifier.focusRequester(first) else Modifier,
                        ) {
                            // Plain names, without the ellipsis some strings end in.
                            Text(
                                stringResource(e.label).removeSuffix("…"),
                                style = theme.type.headline.text,
                                color = if (e == ItemDialog.Delete) theme.colors.danger else theme.colors.textPrimary,
                                maxLines = 1,
                            )
                        }
                    }
                }
            }
        }
    }
    when (dialog) {
        ItemDialog.PlayOn -> PlayOnDialog(model, details, back)
        ItemDialog.Together -> SyncPlayDialog(model, card.serverId, details, back)
        ItemDialog.Playlist -> AddDialog(model, card, collection = false, back)
        ItemDialog.Collection -> AddDialog(model, card, collection = true, back)
        ItemDialog.Report -> ReportDialog(model, card, back)
        ItemDialog.Refresh -> RefreshDialog(model, card, back)
        ItemDialog.Rename -> RenameDialog(model, card, back)
        ItemDialog.Delete -> DeleteDialog(model, card, scope, back)
        ItemDialog.Edit, ItemDialog.Identify, ItemDialog.Images, null -> {}
    }
}

/** A row of a dialog's list: its icon, title and detail, with what follows it. */
@Composable
internal fun DialogRow(
    icon: Int,
    title: String,
    detail: String? = null,
    modifier: Modifier = Modifier,
    onClick: () -> Unit,
    trailing: @Composable () -> Unit = {},
) {
    Pressable(onClick, modifier.fillMaxWidth()) {
        Row(
            Modifier.fillMaxWidth().heightIn(min = 72.dp).padding(horizontal = MavioSpacing.s20, vertical = MavioSpacing.s8),
            horizontalArrangement = Arrangement.spacedBy(MavioSpacing.s16),
            verticalAlignment = Alignment.CenterVertically,
        ) {
            MavioIcon(icon, size = 24.dp, tint = theme.colors.textSecondary)
            RowLabel(title, detail, Modifier.weight(1f))
            trailing()
        }
    }
}

/** An error under a dialog's content. */
@Composable
internal fun DialogFailure(text: String?) {
    text?.let { Text(it, style = theme.type.callout.text, color = theme.colors.danger) }
}

/** A dialog's buttons at its end. */
@Composable
internal fun DialogButtons(content: @Composable () -> Unit) {
    Row(Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.spacedBy(MavioSpacing.s12, Alignment.End)) { content() }
}

/** Picks one of the user's other devices online to play the item on. */
@Composable
private fun PlayOnDialog(model: AppModel, details: Details, onDismiss: () -> Unit) {
    val card = details.card
    var targets by remember { mutableStateOf<List<DeviceSession>?>(null) }
    var failure by remember { mutableStateOf<String?>(null) }
    var busy by remember { mutableStateOf(false) }
    var sent by remember { mutableStateOf<String?>(null) }
    val scope = rememberCoroutineScope()
    LaunchedEffect(Unit) {
        runCatching { model.client.devices(card.serverId) }
            .onSuccess { list -> targets = list.filter { it.online && !it.current } }
            .onFailure { failure = it.message ?: it.toString() }
    }
    // Focus moves to the first device once they are known.
    val device = remember { FocusRequester() }
    LaunchedEffect(targets) {
        if (!targets.isNullOrEmpty()) {
            withFrameNanos {}
            runCatching { device.requestFocus() }
        }
    }
    // Where it was left, unless it was finished.
    val position = details.position?.takeIf { !card.played && !it.isZero }
    TvDialog(stringResource(R.string.remote_play_on_title, card.title), onDismiss) {
        val shown = targets
        if (shown != null && shown.isNotEmpty()) {
            Group {
                shown.forEachIndexed { i, d ->
                    if (i > 0) RowDivider()
                    DialogRow(
                        R.drawable.ic_devices,
                        d.deviceName.ifEmpty { d.client.ifEmpty { "—" } },
                        if (sent == d.id) stringResource(R.string.remote_sent) else d.playing?.let { stringResource(R.string.remote_playing, it.card.title) } ?: d.client,
                        if (i == 0) Modifier.focusRequester(device) else Modifier,
                        onClick = {
                            if (busy) return@DialogRow
                            busy = true
                            scope.launch {
                                runCatching { model.playOn(card, d.id, position) }
                                    .onSuccess {
                                        sent = d.id
                                        failure = null
                                    }
                                    .onFailure { failure = it.message ?: it.toString() }
                                busy = false
                            }
                        },
                    )
                }
            }
        }
        if (shown?.isEmpty() == true) {
            Text(stringResource(R.string.remote_no_devices), style = theme.type.body.text, color = theme.colors.textSecondary)
        }
        DialogFailure(failure)
        DialogButtons { SettingsButton(stringResource(R.string.keys_done), onDismiss) }
    }
}

/** Adds the item to one of the playlists or collections, or to a new one. */
@Composable
private fun AddDialog(model: AppModel, card: Card, collection: Boolean, onDismiss: () -> Unit) {
    var lists by remember { mutableStateOf<List<ItemList>?>(null) }
    var added by remember { mutableStateOf<String?>(null) }
    var name by remember { mutableStateOf("") }
    var failure by remember { mutableStateOf<String?>(null) }
    var busy by remember { mutableStateOf(false) }
    var reload by remember { mutableIntStateOf(0) }
    val scope = rememberCoroutineScope()
    LaunchedEffect(reload) {
        runCatching { model.itemLists(card.serverId, collection) }.onSuccess { lists = it }.onFailure { failure = it.message ?: it.toString() }
    }
    fun run(block: suspend () -> Unit) {
        if (busy) return
        busy = true
        scope.launch {
            runCatching { block() }.onSuccess { failure = null }.onFailure { failure = it.message ?: it.toString() }
            busy = false
        }
    }
    TvDialog(stringResource(if (collection) R.string.lists_add_to_collection else R.string.lists_add_to_playlist), onDismiss) {
        Text(
            added?.let { stringResource(R.string.lists_added, card.title, it) } ?: card.title,
            style = theme.type.body.text,
            color = theme.colors.textSecondary,
        )
        val shown = lists.orEmpty()
        if (shown.isNotEmpty()) {
            // A long list scrolls within the dialog.
            Box(Modifier.heightIn(max = 360.dp)) {
                LazyColumn(Modifier.fillMaxWidth().background(theme.colors.surface, androidx.compose.foundation.shape.RoundedCornerShape(14.dp))) {
                    items(shown, key = { it.id }) { l ->
                        DialogRow(
                            if (collection) R.drawable.ic_collections else R.drawable.ic_playlists,
                            l.name,
                            onClick = {
                                run {
                                    model.addToList(card, l.id, collection)
                                    added = l.name
                                }
                            },
                        ) { if (added == l.name) MavioIcon(R.drawable.ic_check, size = 24.dp, tint = theme.colors.accent) }
                    }
                }
            }
        }
        Row(horizontalArrangement = Arrangement.spacedBy(MavioSpacing.s12), verticalAlignment = Alignment.Bottom) {
            SettingsField(
                name,
                { if (it.length <= 200) name = it },
                null,
                Modifier.weight(1f),
                placeholder = stringResource(if (collection) R.string.lists_new_collection else R.string.lists_new_playlist),
            )
            SettingsButton(stringResource(R.string.lists_create), {
                val n = name.trim()
                if (n.isNotEmpty()) {
                    run {
                        model.createList(card, n, collection)
                        added = n
                        name = ""
                        reload++
                    }
                }
            }, kind = ButtonKind.Secondary, enabled = name.isNotBlank() && !busy)
        }
        DialogFailure(failure)
        DialogButtons { SettingsButton(stringResource(R.string.keys_done), onDismiss) }
    }
}

/** The kinds of problems, as the report's choice names them. */
private val issueLabels = issueTypes.zip(
    listOf(R.string.issues_video, R.string.issues_audio, R.string.issues_subtitles, R.string.issues_other),
)

/** Reports a problem with the item; a series' with a season and episode. */
@Composable
private fun ReportDialog(model: AppModel, card: Card, onDismiss: () -> Unit) {
    var type by remember { mutableStateOf(issueTypes.first()) }
    var season by remember { mutableStateOf("") }
    var episode by remember { mutableStateOf("") }
    var message by remember { mutableStateOf("") }
    var failure by remember { mutableStateOf<String?>(null) }
    var busy by remember { mutableStateOf(false) }
    val scope = rememberCoroutineScope()
    val series = card.kind == CardKind.SERIES
    TvDialog(stringResource(R.string.issues_report_title, card.title), onDismiss, width = 760.dp) {
        Text(stringResource(R.string.issues_type), style = theme.type.callout.text, color = theme.colors.textSecondary)
        Row(horizontalArrangement = Arrangement.spacedBy(MavioSpacing.s8)) {
            issueLabels.forEach { (t, label) ->
                TvChoice(stringResource(label), t == type, horizontal = 24.dp, onFocus = false) { type = t }
            }
        }
        if (series) {
            val digits = { s: String -> s.filter(Char::isDigit).take(4) }
            Row(horizontalArrangement = Arrangement.spacedBy(MavioSpacing.s12)) {
                SettingsField(season, { season = digits(it) }, stringResource(R.string.issues_season), Modifier.weight(1f), keyboard = KeyboardType.Number)
                SettingsField(
                    if ((season.toIntOrNull() ?: 0) == 0) "" else episode,
                    { episode = digits(it) },
                    stringResource(R.string.issues_episode),
                    Modifier.weight(1f),
                    keyboard = KeyboardType.Number,
                )
            }
            Text(stringResource(R.string.issues_all_numbers), style = theme.type.callout.text, color = theme.colors.textSecondary)
        }
        SettingsField(message, { if (it.length <= 10000) message = it }, null, placeholder = stringResource(R.string.issues_message))
        DialogFailure(failure)
        DialogButtons {
            SettingsButton(stringResource(R.string.action_cancel), onDismiss, kind = ButtonKind.Secondary)
            SettingsButton(stringResource(R.string.issues_send), {
                busy = true
                scope.launch {
                    runCatching {
                        model.reportIssue(card, type, if (series) season.toIntOrNull() ?: 0 else 0, episode.toIntOrNull() ?: 0, message.trim())
                    }.onSuccess { onDismiss() }.onFailure { failure = it.message ?: it.toString() }
                    busy = false
                }
            }, enabled = message.isNotBlank() && !busy)
        }
    }
}

/** Refreshes the item's metadata, keeping it or replacing it all. */
@Composable
private fun RefreshDialog(model: AppModel, card: Card, onDismiss: () -> Unit) {
    var replace by remember { mutableStateOf(false) }
    var failure by remember { mutableStateOf<String?>(null) }
    var busy by remember { mutableStateOf(false) }
    val scope = rememberCoroutineScope()
    TvDialog(stringResource(R.string.metadata_refresh_title, card.title), onDismiss) {
        Group(footer = stringResource(if (replace) R.string.metadata_replace_footer else R.string.metadata_refresh_footer)) {
            SwitchRow(stringResource(R.string.metadata_replace), replace) { replace = it }
        }
        DialogFailure(failure)
        DialogButtons {
            SettingsButton(stringResource(R.string.action_cancel), onDismiss, kind = ButtonKind.Secondary)
            SettingsButton(stringResource(if (busy) R.string.metadata_refreshing else R.string.metadata_refresh), {
                busy = true
                scope.launch {
                    runCatching { model.refreshItem(card, replace) }.onSuccess { onDismiss() }.onFailure { failure = it.message ?: it.toString() }
                    busy = false
                }
            }, enabled = !busy)
        }
    }
}

/** Renames a playlist or collection. */
@Composable
private fun RenameDialog(model: AppModel, card: Card, onDismiss: () -> Unit) {
    var name by remember { mutableStateOf(card.title) }
    var failure by remember { mutableStateOf<String?>(null) }
    var busy by remember { mutableStateOf(false) }
    val scope = rememberCoroutineScope()
    TvDialog(stringResource(R.string.lists_rename), onDismiss) {
        SettingsField(name, { if (it.length <= 200) name = it }, stringResource(R.string.lists_name))
        DialogFailure(failure)
        DialogButtons {
            SettingsButton(stringResource(R.string.action_cancel), onDismiss, kind = ButtonKind.Secondary)
            SettingsButton(stringResource(R.string.lists_save), {
                busy = true
                scope.launch {
                    runCatching { model.renameList(card, name.trim()) }.onSuccess { onDismiss() }.onFailure { failure = it.message ?: it.toString() }
                    busy = false
                }
            }, enabled = name.isNotBlank() && !busy)
        }
    }
}

/** Deletes a playlist or collection, then goes back. */
@Composable
private fun DeleteDialog(model: AppModel, card: Card, scope: CoroutineScope, onDismiss: () -> Unit) {
    val pages = LocalPages.current
    var failure by remember { mutableStateOf<String?>(null) }
    val playlist = card.kind == CardKind.PLAYLIST
    TvDialog(stringResource(R.string.lists_delete_title, card.title), onDismiss) {
        Text(
            stringResource(if (playlist) R.string.lists_delete_playlist else R.string.lists_delete_collection),
            style = theme.type.body.text,
            color = theme.colors.textSecondary,
        )
        DialogFailure(failure)
        DialogButtons {
            SettingsButton(stringResource(R.string.action_cancel), onDismiss, kind = ButtonKind.Secondary)
            SettingsButton(stringResource(R.string.lists_delete), {
                scope.launch {
                    runCatching { model.deleteList(card) }
                        .onSuccess {
                            onDismiss()
                            pages.back()
                        }
                        .onFailure { failure = it.message ?: it.toString() }
                }
            })
        }
    }
}
