package dev.mavio.app

import androidx.annotation.StringRes
import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.ColumnScope
import androidx.compose.foundation.layout.ExperimentalLayoutApi
import androidx.compose.foundation.layout.FlowRow
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.text.KeyboardOptions
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.DropdownMenu
import androidx.compose.material3.DropdownMenuItem
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.ModalBottomSheet
import androidx.compose.material3.Switch
import androidx.compose.material3.Text
import androidx.compose.material3.rememberModalBottomSheetState
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
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.input.KeyboardType
import androidx.compose.ui.unit.dp
import com.google.protobuf.duration
import dev.mavio.core.AppModel
import dev.mavio.core.Card
import dev.mavio.core.CardKind
import dev.mavio.core.Details
import dev.mavio.core.DeviceSession
import dev.mavio.core.call
import dev.mavio.design.MavioSpacing
import dev.mavio.ui.ButtonKind
import dev.mavio.ui.CircleButton
import dev.mavio.ui.MavioButton
import dev.mavio.ui.MavioIcon
import dev.mavio.ui.R
import dev.mavio.ui.text
import dev.mavio.ui.theme
import kotlinx.coroutines.launch
import mavio.acquisition.v1.IssueOuterClass.CreateIssueResponse
import mavio.acquisition.v1.IssueOuterClass.IssueType
import mavio.acquisition.v1.createIssueRequest
import mavio.library.v1.Collection.AddToCollectionResponse
import mavio.library.v1.Collection.CreateCollectionResponse
import mavio.library.v1.Collection.DeleteCollectionResponse
import mavio.library.v1.Collection.UpdateCollectionResponse
import mavio.library.v1.ItemOuterClass.ItemKind
import mavio.library.v1.ItemOuterClass.ListItemsResponse
import mavio.library.v1.ItemOuterClass.SortField
import mavio.library.v1.Metadata.RefreshItemResponse
import mavio.library.v1.Playlist.AddToPlaylistResponse
import mavio.library.v1.Playlist.CreatePlaylistResponse
import mavio.library.v1.Playlist.DeletePlaylistResponse
import mavio.library.v1.Playlist.ListPlaylistsResponse
import mavio.library.v1.Playlist.UpdatePlaylistResponse
import mavio.library.v1.addToCollectionRequest
import mavio.library.v1.addToPlaylistRequest
import mavio.library.v1.createCollectionRequest
import mavio.library.v1.createPlaylistRequest
import mavio.library.v1.deleteCollectionRequest
import mavio.library.v1.deletePlaylistRequest
import mavio.library.v1.listItemsRequest
import mavio.library.v1.listPlaylistsRequest
import mavio.library.v1.refreshItemRequest
import mavio.library.v1.sortSpec
import mavio.library.v1.updateCollectionRequest
import mavio.library.v1.updatePlaylistRequest
import mavio.session.v1.SessionOuterClass.SendCommandResponse
import mavio.session.v1.command
import mavio.session.v1.sendCommandRequest
import mavio.user.v1.UserOuterClass.GetCurrentUserResponse
import mavio.user.v1.getCurrentUserRequest

/** Kinds a device plays, which Play On offers. */
private val playable = setOf(CardKind.MOVIE, CardKind.EPISODE, CardKind.VIDEO, CardKind.MUSIC_VIDEO, CardKind.TRACK, CardKind.AUDIOBOOK)

/** Kinds problems are reported with. */
private val reportable = setOf(CardKind.MOVIE, CardKind.SERIES, CardKind.SEASON, CardKind.EPISODE)

/** Kinds the providers identify. */
private val identifiable = setOf(
    CardKind.MOVIE, CardKind.SERIES, CardKind.ARTIST, CardKind.ALBUM, CardKind.MUSIC_VIDEO, CardKind.BOOK, CardKind.AUDIOBOOK,
)

/** Whether the item can go into a playlist or collection. */
private fun listable(kind: CardKind) = kind != CardKind.PLAYLIST && kind != CardKind.COLLECTION && kind != CardKind.FOLDER

/** A dialog of the More menu. */
private enum class ItemDialog(@StringRes val label: Int) {
    PlayOn(R.string.remote_play_on),
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
 * The More menu of an item's actions, as the web UI's, and the dialogs it
 * opens: playing it on another device, adding it to a playlist, reporting a
 * problem, and for administrators adding it to a collection, editing and
 * refreshing its metadata, identifying it and choosing its images; a playlist, or a collection for administrators, renamed or
 * deleted.
 */
@Composable
internal fun MoreMenu(model: AppModel, details: Details) {
    val card = details.card
    if (details.offline) return
    // What the user may do on the server, once known.
    var admin by remember(card.serverId) { mutableStateOf(false) }
    var mayReport by remember(card.serverId) { mutableStateOf(false) }
    LaunchedEffect(card.serverId) {
        runCatching {
            model.client.call(card.serverId, Procedure.users("GetCurrentUser"), getCurrentUserRequest {}, GetCurrentUserResponse::parseFrom).user
        }.onSuccess {
            admin = it.admin
            mayReport = it.admin || it.policy.reportIssues
        }
    }
    val own = card.kind == CardKind.PLAYLIST || (card.kind == CardKind.COLLECTION && admin)
    val entries = buildList {
        if (card.kind in playable) add(ItemDialog.PlayOn)
        if (listable(card.kind)) add(ItemDialog.Playlist)
        if (card.kind in reportable && mayReport) add(ItemDialog.Report)
        if (listable(card.kind) && admin) add(ItemDialog.Collection)
        if (admin && card.kind != CardKind.PLAYLIST) {
            add(ItemDialog.Edit)
            if (card.kind in identifiable) add(ItemDialog.Identify)
            if (card.kind != CardKind.COLLECTION) add(ItemDialog.Refresh)
            add(ItemDialog.Images)
        }
        if (own) {
            add(ItemDialog.Rename)
            add(ItemDialog.Delete)
        }
    }
    if (entries.isEmpty()) return
    val navigator = LocalNavigator.current
    var open by remember { mutableStateOf(false) }
    var dialog by remember { mutableStateOf<ItemDialog?>(null) }
    // Outlives the dialogs, which close as their work starts.
    val scope = rememberCoroutineScope()
    Box {
        CircleButton(R.drawable.ic_more, stringResource(R.string.lists_more), { open = true })
        DropdownMenu(open, onDismissRequest = { open = false }, containerColor = theme.colors.surfaceOverlay) {
            entries.forEach { e ->
                DropdownMenuItem(
                    text = {
                        Text(
                            stringResource(e.label),
                            style = theme.type.callout.text,
                            color = if (e == ItemDialog.Delete) theme.colors.danger else theme.colors.textPrimary,
                        )
                    },
                    onClick = {
                        open = false
                        when (e) {
                            ItemDialog.Edit -> navigator.open(Route.EditMetadata(card.serverId, card.itemId))
                            ItemDialog.Identify -> navigator.open(Route.Identify(card.serverId, card.itemId))
                            ItemDialog.Images -> navigator.open(Route.ItemImages(card.serverId, card.itemId))
                            else -> dialog = e
                        }
                    },
                )
            }
        }
    }
    val close = { dialog = null }
    when (dialog) {
        ItemDialog.PlayOn -> PlayOnSheet(model, details, close)
        ItemDialog.Playlist -> AddSheet(model, card, collection = false, close)
        ItemDialog.Collection -> AddSheet(model, card, collection = true, close)
        ItemDialog.Report -> ReportSheet(model, card, close)
        ItemDialog.Refresh -> RefreshSheet(model, card, close)
        ItemDialog.Rename -> RenameSheet(model, card, close)
        ItemDialog.Delete -> DeleteDialog(model, card, scope, close)
        ItemDialog.Edit, ItemDialog.Identify, ItemDialog.Images, null -> {}
    }
}

/** A sheet with a title over its content, scrolling when long. */
@OptIn(ExperimentalMaterial3Api::class)
@Composable
private fun Sheet(title: String, onDismiss: () -> Unit, content: @Composable ColumnScope.() -> Unit) {
    ModalBottomSheet(onDismiss, sheetState = rememberModalBottomSheetState(skipPartiallyExpanded = true), containerColor = theme.colors.background) {
        Column(
            Modifier.fillMaxWidth().verticalScroll(rememberScrollState()).padding(horizontal = theme.layout.margin).padding(bottom = MavioSpacing.s24),
            verticalArrangement = Arrangement.spacedBy(MavioSpacing.s12),
        ) {
            Text(title, style = theme.type.title3.text)
            content()
        }
    }
}

/** A row of a sheet's list: its icon, title and detail, with what follows it. */
@Composable
private fun SheetRow(icon: Int, title: String, detail: String? = null, enabled: Boolean = true, onClick: () -> Unit, trailing: @Composable () -> Unit = {}) {
    Row(
        Modifier
            .fillMaxWidth()
            .heightIn(min = 48.dp)
            .clip(RoundedCornerShape(10.dp))
            .clickable(enabled = enabled, onClick = onClick)
            .padding(horizontal = MavioSpacing.s8, vertical = 6.dp),
        horizontalArrangement = Arrangement.spacedBy(MavioSpacing.s12),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        MavioIcon(icon, size = 20.dp, tint = theme.colors.textSecondary)
        Column(Modifier.weight(1f)) {
            Text(title, style = theme.type.body.text, maxLines = 1)
            detail?.let { Text(it, style = theme.type.footnote.text, color = theme.colors.textSecondary, maxLines = 1) }
        }
        trailing()
    }
}

/** An error under a sheet's content. */
@Composable
private fun Failure(text: String?) {
    text?.let { Text(it, style = theme.type.footnote.text, color = theme.colors.danger) }
}

/** Picks one of the user's other devices online to play the item on, then controls it. */
@Composable
private fun PlayOnSheet(model: AppModel, details: Details, onDismiss: () -> Unit) {
    val card = details.card
    var targets by remember { mutableStateOf<List<DeviceSession>?>(null) }
    var failure by remember { mutableStateOf<String?>(null) }
    var busy by remember { mutableStateOf(false) }
    var controlling by remember { mutableStateOf<String?>(null) }
    val scope = rememberCoroutineScope()
    LaunchedEffect(Unit) {
        runCatching { model.client.devices(card.serverId) }
            .onSuccess { list -> targets = list.filter { it.online && !it.current } }
            .onFailure { failure = it.reason }
    }
    controlling?.let { id ->
        RemoteSheet(model, card.serverId, id, onDismiss)
        return
    }
    // Where it was left, unless it was finished.
    val position = details.position?.takeIf { !card.played && !it.isZero }
    Sheet(stringResource(R.string.remote_play_on_title, card.title), onDismiss) {
        targets?.forEach { d ->
            SheetRow(
                R.drawable.ic_devices,
                deviceName(d),
                d.playing?.let { stringResource(R.string.remote_playing, it.card.title) } ?: d.client,
                enabled = !busy,
                onClick = {
                    busy = true
                    scope.launch {
                        runCatching {
                            model.client.call(
                                card.serverId,
                                Procedure.sessions("SendCommand"),
                                sendCommandRequest {
                                    sessionId = d.id
                                    command = command {
                                        play = mavio.session.v1.play {
                                            itemIds += card.itemId
                                            position?.let { startPosition = duration { seconds = it.seconds } }
                                        }
                                    }
                                },
                                SendCommandResponse::parseFrom,
                            )
                        }.onSuccess { controlling = d.id }.onFailure { failure = it.reason }
                        busy = false
                    }
                },
            )
        }
        if (targets?.isEmpty() == true) {
            Text(stringResource(R.string.remote_no_devices), style = theme.type.callout.text, color = theme.colors.textSecondary)
        }
        Failure(failure)
    }
}

/** Adds the item to one of the playlists or collections, or to a new one. */
@Composable
private fun AddSheet(model: AppModel, card: Card, collection: Boolean, onDismiss: () -> Unit) {
    val serverId = card.serverId
    var lists by remember { mutableStateOf<List<Pair<String, String>>?>(null) }
    var added by remember { mutableStateOf<String?>(null) }
    var name by remember { mutableStateOf("") }
    var failure by remember { mutableStateOf<String?>(null) }
    var busy by remember { mutableStateOf(false) }
    var reload by remember { mutableIntStateOf(0) }
    val scope = rememberCoroutineScope()
    LaunchedEffect(reload) {
        runCatching {
            if (collection) {
                model.client.call(
                    serverId,
                    Procedure.items("ListItems"),
                    listItemsRequest {
                        kinds += ItemKind.ITEM_KIND_COLLECTION
                        sort += sortSpec { field = SortField.SORT_FIELD_NAME }
                    },
                    ListItemsResponse::parseFrom,
                ).itemsList
            } else {
                model.client.call(serverId, Procedure.playlists("ListPlaylists"), listPlaylistsRequest {}, ListPlaylistsResponse::parseFrom).playlistsList
            }
        }.onSuccess { items -> lists = items.map { it.id to it.name } }.onFailure { failure = it.reason }
    }
    fun run(block: suspend () -> Unit) {
        busy = true
        scope.launch {
            runCatching { block() }.onSuccess { failure = null }.onFailure { failure = it.reason }
            busy = false
        }
    }
    Sheet(stringResource(if (collection) R.string.lists_add_to_collection else R.string.lists_add_to_playlist), onDismiss) {
        Text(
            added?.let { stringResource(R.string.lists_added, card.title, it) } ?: card.title,
            style = theme.type.callout.text,
            color = theme.colors.textSecondary,
        )
        lists?.forEach { (id, listName) ->
            SheetRow(
                if (collection) R.drawable.ic_collections else R.drawable.ic_playlists,
                listName,
                enabled = !busy,
                onClick = {
                    run {
                        if (collection) {
                            model.client.call(
                                serverId,
                                Procedure.collections("AddToCollection"),
                                addToCollectionRequest {
                                    this.id = id
                                    itemIds += card.itemId
                                },
                                AddToCollectionResponse::parseFrom,
                            )
                        } else {
                            model.client.call(
                                serverId,
                                Procedure.playlists("AddToPlaylist"),
                                addToPlaylistRequest {
                                    this.id = id
                                    itemIds += card.itemId
                                },
                                AddToPlaylistResponse::parseFrom,
                            )
                        }
                        added = listName
                        model.bump(serverId)
                    }
                },
            ) { if (added == listName) MavioIcon(R.drawable.ic_check, size = 16.dp, tint = theme.colors.accent) }
        }
        Row(horizontalArrangement = Arrangement.spacedBy(MavioSpacing.s8), verticalAlignment = Alignment.CenterVertically) {
            SettingsField(
                name,
                { if (it.length <= 200) name = it },
                null,
                Modifier.weight(1f),
                placeholder = stringResource(if (collection) R.string.lists_new_collection else R.string.lists_new_playlist),
            )
            MavioButton(stringResource(R.string.lists_create), {
                val n = name.trim()
                run {
                    if (collection) {
                        model.client.call(
                            serverId,
                            Procedure.collections("CreateCollection"),
                            createCollectionRequest {
                                this.name = n
                                itemIds += card.itemId
                            },
                            CreateCollectionResponse::parseFrom,
                        )
                    } else {
                        model.client.call(
                            serverId,
                            Procedure.playlists("CreatePlaylist"),
                            createPlaylistRequest {
                                this.name = n
                                itemIds += card.itemId
                            },
                            CreatePlaylistResponse::parseFrom,
                        )
                    }
                    added = n
                    name = ""
                    reload++
                    model.bump(serverId)
                }
            }, kind = ButtonKind.Secondary, enabled = name.isNotBlank() && !busy)
        }
        Failure(failure)
        Box(Modifier.fillMaxWidth(), Alignment.CenterEnd) { MavioButton(stringResource(R.string.keys_done), onDismiss) }
    }
}

/** The kinds of problems, as the report's choice names them. */
private val issueTypes = listOf(
    IssueType.ISSUE_TYPE_VIDEO to R.string.issues_video,
    IssueType.ISSUE_TYPE_AUDIO to R.string.issues_audio,
    IssueType.ISSUE_TYPE_SUBTITLES to R.string.issues_subtitles,
    IssueType.ISSUE_TYPE_OTHER to R.string.issues_other,
)

/** Reports a problem with the item; a series' with a season and episode, then opens the issue. */
@OptIn(ExperimentalLayoutApi::class)
@Composable
private fun ReportSheet(model: AppModel, card: Card, onDismiss: () -> Unit) {
    val navigator = LocalNavigator.current
    var type by remember { mutableStateOf(IssueType.ISSUE_TYPE_VIDEO) }
    var season by remember { mutableStateOf("") }
    var episode by remember { mutableStateOf("") }
    var message by remember { mutableStateOf("") }
    var failure by remember { mutableStateOf<String?>(null) }
    var busy by remember { mutableStateOf(false) }
    val scope = rememberCoroutineScope()
    val series = card.kind == CardKind.SERIES
    val colors = theme.colors
    Sheet(stringResource(R.string.issues_report_title, card.title), onDismiss) {
        Text(stringResource(R.string.issues_type), style = theme.type.footnote.text, color = colors.textSecondary)
        FlowRow(horizontalArrangement = Arrangement.spacedBy(MavioSpacing.s8), verticalArrangement = Arrangement.spacedBy(MavioSpacing.s8)) {
            issueTypes.forEach { (t, label) ->
                val active = t == type
                Text(
                    stringResource(label),
                    Modifier
                        .clip(CircleShape)
                        .background(if (active) colors.button else colors.fill)
                        .clickable { type = t }
                        .padding(horizontal = 14.dp, vertical = 6.dp),
                    style = theme.type.callout.text,
                    fontWeight = FontWeight.Medium,
                    color = if (active) colors.onButton else colors.textPrimary,
                )
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
            Text(stringResource(R.string.issues_all_numbers), style = theme.type.footnote.text, color = colors.textSecondary)
        }
        androidx.compose.material3.OutlinedTextField(
            message,
            { if (it.length <= 10000) message = it },
            Modifier.fillMaxWidth().heightIn(min = 112.dp),
            placeholder = { Text(stringResource(R.string.issues_message)) },
            keyboardOptions = KeyboardOptions.Default,
        )
        Failure(failure)
        Row(Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.spacedBy(MavioSpacing.s8, Alignment.End)) {
            MavioButton(stringResource(R.string.action_cancel), onDismiss, kind = ButtonKind.Secondary)
            MavioButton(stringResource(R.string.issues_send), {
                busy = true
                scope.launch {
                    val s = if (series) season.toIntOrNull() ?: 0 else 0
                    runCatching {
                        model.client.call(
                            card.serverId,
                            Procedure.issues("CreateIssue"),
                            createIssueRequest {
                                itemId = card.itemId
                                this.type = type
                                seasonNumber = s
                                episodeNumber = if (s > 0) episode.toIntOrNull() ?: 0 else 0
                                this.message = message.trim()
                            },
                            CreateIssueResponse::parseFrom,
                        )
                    }.onSuccess {
                        onDismiss()
                        if (it.hasIssue()) navigator.open(Route.IssuePage(card.serverId, it.issue.id))
                    }.onFailure { failure = it.reason }
                    busy = false
                }
            }, enabled = message.isNotBlank() && !busy)
        }
    }
}

/** Refreshes the item's metadata, keeping it or replacing it all. */
@Composable
private fun RefreshSheet(model: AppModel, card: Card, onDismiss: () -> Unit) {
    var replace by remember { mutableStateOf(false) }
    var failure by remember { mutableStateOf<String?>(null) }
    var busy by remember { mutableStateOf(false) }
    val scope = rememberCoroutineScope()
    Sheet(stringResource(R.string.metadata_refresh_title, card.title), onDismiss) {
        Row(verticalAlignment = Alignment.CenterVertically) {
            Text(stringResource(R.string.metadata_replace), Modifier.weight(1f), style = theme.type.callout.text)
            Switch(replace, { replace = it })
        }
        Text(
            stringResource(if (replace) R.string.metadata_replace_footer else R.string.metadata_refresh_footer),
            style = theme.type.footnote.text,
            color = theme.colors.textSecondary,
        )
        Failure(failure)
        Row(Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.spacedBy(MavioSpacing.s8, Alignment.End)) {
            MavioButton(stringResource(R.string.action_cancel), onDismiss, kind = ButtonKind.Secondary)
            MavioButton(stringResource(if (busy) R.string.metadata_refreshing else R.string.metadata_refresh), {
                busy = true
                scope.launch {
                    runCatching {
                        model.client.call(
                            card.serverId,
                            Procedure.metadata("RefreshItem"),
                            refreshItemRequest {
                                id = card.itemId
                                replaceMetadata = replace
                            },
                            RefreshItemResponse::parseFrom,
                        )
                    }.onSuccess {
                        model.bump(card.serverId)
                        onDismiss()
                    }.onFailure { failure = it.reason }
                    busy = false
                }
            }, enabled = !busy)
        }
    }
}

/** Renames a playlist or collection. */
@Composable
private fun RenameSheet(model: AppModel, card: Card, onDismiss: () -> Unit) {
    var name by remember { mutableStateOf(card.title) }
    var failure by remember { mutableStateOf<String?>(null) }
    var busy by remember { mutableStateOf(false) }
    val scope = rememberCoroutineScope()
    Sheet(stringResource(R.string.lists_rename), onDismiss) {
        SettingsField(name, { if (it.length <= 200) name = it }, stringResource(R.string.lists_name))
        Failure(failure)
        Row(Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.spacedBy(MavioSpacing.s8, Alignment.End)) {
            MavioButton(stringResource(R.string.action_cancel), onDismiss, kind = ButtonKind.Secondary)
            MavioButton(stringResource(R.string.lists_save), {
                busy = true
                scope.launch {
                    val n = name.trim()
                    runCatching {
                        if (card.kind == CardKind.PLAYLIST) {
                            model.client.call(
                                card.serverId,
                                Procedure.playlists("UpdatePlaylist"),
                                updatePlaylistRequest {
                                    id = card.itemId
                                    this.name = n
                                },
                                UpdatePlaylistResponse::parseFrom,
                            )
                        } else {
                            model.client.call(
                                card.serverId,
                                Procedure.collections("UpdateCollection"),
                                updateCollectionRequest {
                                    id = card.itemId
                                    this.name = n
                                },
                                UpdateCollectionResponse::parseFrom,
                            )
                        }
                    }.onSuccess {
                        model.bump(card.serverId)
                        onDismiss()
                    }.onFailure { failure = it.reason }
                    busy = false
                }
            }, enabled = name.isNotBlank() && !busy)
        }
    }
}

/** Deletes a playlist or collection, then goes back. */
@Composable
private fun DeleteDialog(model: AppModel, card: Card, scope: kotlinx.coroutines.CoroutineScope, onDismiss: () -> Unit) {
    val navigator = LocalNavigator.current
    val context = LocalContext.current
    val playlist = card.kind == CardKind.PLAYLIST
    ConfirmDialog(
        stringResource(R.string.lists_delete_title, card.title),
        stringResource(if (playlist) R.string.lists_delete_playlist else R.string.lists_delete_collection),
        stringResource(R.string.lists_delete),
        onConfirm = {
            scope.launch {
                runCatching {
                    if (playlist) {
                        model.client.call(card.serverId, Procedure.playlists("DeletePlaylist"), deletePlaylistRequest { id = card.itemId }, DeletePlaylistResponse::parseFrom)
                    } else {
                        model.client.call(
                            card.serverId,
                            Procedure.collections("DeleteCollection"),
                            deleteCollectionRequest { id = card.itemId },
                            DeleteCollectionResponse::parseFrom,
                        )
                    }
                }.onSuccess {
                    model.bump(card.serverId)
                    navigator.back()
                }.onFailure { android.widget.Toast.makeText(context, it.reason, android.widget.Toast.LENGTH_LONG).show() }
            }
        },
        onDismiss = onDismiss,
    )
}
