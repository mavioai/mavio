package dev.mavio.app

import androidx.annotation.DrawableRes
import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.ExperimentalLayoutApi
import androidx.compose.foundation.layout.FlowRow
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.aspectRatio
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.offset
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.alpha
import androidx.compose.ui.draw.clip
import androidx.compose.ui.draw.rotate
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import dev.mavio.admin.Procedure
import dev.mavio.admin.reason
import dev.mavio.core.AppModel
import dev.mavio.core.Card
import dev.mavio.core.Details
import dev.mavio.core.call
import dev.mavio.design.MavioRadius
import dev.mavio.design.MavioSpacing
import dev.mavio.ui.Artwork
import dev.mavio.ui.ButtonKind
import dev.mavio.ui.ItemCard
import dev.mavio.ui.MavioButton
import dev.mavio.ui.MavioIcon
import dev.mavio.ui.R
import dev.mavio.ui.itemIcon
import dev.mavio.ui.minutesText
import dev.mavio.ui.subtitleOf
import dev.mavio.ui.text
import dev.mavio.ui.theme
import kotlinx.coroutines.launch
import mavio.library.v1.Collection.RemoveFromCollectionResponse
import mavio.library.v1.Playlist.ListPlaylistEntriesResponse
import mavio.library.v1.Playlist.MovePlaylistEntryResponse
import mavio.library.v1.Playlist.RemoveFromPlaylistResponse
import mavio.library.v1.listPlaylistEntriesRequest
import mavio.library.v1.movePlaylistEntryRequest
import mavio.library.v1.removeFromCollectionRequest
import mavio.library.v1.removeFromPlaylistRequest
import mavio.user.v1.UserOuterClass.GetCurrentUserResponse
import mavio.user.v1.getCurrentUserRequest

/** A playlist's entry: its id in the playlist and the item's card. */
private data class Entry(val id: String, val card: Card)

/**
 * A playlist's entries in order, as the web UI's: each played, moved up or
 * down, or removed in place; the details load again after each change.
 */
@Composable
internal fun PlaylistEntries(model: AppModel, details: Details) {
    val playlist = details.card
    val revisions by model.revisions.collectAsStateWithLifecycle()
    val scope = rememberCoroutineScope()
    var entries by remember(playlist.itemId) { mutableStateOf<List<Entry>?>(null) }
    var busy by remember { mutableStateOf(false) }
    var failure by remember { mutableStateOf<String?>(null) }
    // The entries' ids, paired with the details' cards of their items.
    LaunchedEffect(playlist.serverId, playlist.itemId, revisions[playlist.serverId]) {
        runCatching {
            model.client.call(
                playlist.serverId,
                Procedure.playlists("ListPlaylistEntries"),
                listPlaylistEntriesRequest { id = playlist.itemId },
                ListPlaylistEntriesResponse::parseFrom,
            ).entriesList
        }.onSuccess { list ->
            val cards = details.children.associateBy { it.itemId }
            entries = list.mapNotNull { e -> cards[e.item.id]?.let { Entry(e.id, it) } }
            failure = null
        }.onFailure { failure = it.reason }
    }
    val change: (suspend () -> Unit) -> Unit = { action ->
        busy = true
        scope.launch {
            runCatching { action() }
                .onSuccess { model.bump(playlist.serverId) }
                .onFailure { failure = it.reason }
            busy = false
        }
    }
    val shown = entries ?: return
    Column(Modifier.padding(horizontal = theme.layout.margin, vertical = MavioSpacing.s8)) {
        failure?.let { Text(it, Modifier.padding(bottom = MavioSpacing.s8), style = theme.type.callout.text, color = theme.colors.danger) }
        if (shown.isEmpty()) {
            Text(stringResource(R.string.lists_empty_playlist), style = theme.type.body.text, color = theme.colors.textSecondary)
        }
        shown.forEachIndexed { i, e ->
            EntryRow(
                e.card,
                i,
                busy = busy,
                last = i == shown.lastIndex,
                onMove = { to ->
                    change {
                        model.client.call(
                            playlist.serverId,
                            Procedure.playlists("MovePlaylistEntry"),
                            movePlaylistEntryRequest {
                                id = playlist.itemId
                                entryId = e.id
                                position = to
                            },
                            MovePlaylistEntryResponse::parseFrom,
                        )
                    }
                },
                onRemove = {
                    change {
                        model.client.call(
                            playlist.serverId,
                            Procedure.playlists("RemoveFromPlaylist"),
                            removeFromPlaylistRequest {
                                id = playlist.itemId
                                entryIds += e.id
                            },
                            RemoveFromPlaylistResponse::parseFrom,
                        )
                    }
                },
            )
        }
    }
}

@Composable
private fun EntryRow(card: Card, index: Int, busy: Boolean, last: Boolean, onMove: (Int) -> Unit, onRemove: () -> Unit) {
    val navigator = LocalNavigator.current
    val colors = theme.colors
    Row(
        Modifier.fillMaxWidth().padding(vertical = MavioSpacing.s8),
        verticalAlignment = Alignment.CenterVertically,
        horizontalArrangement = Arrangement.spacedBy(MavioSpacing.s8),
    ) {
        Row(
            Modifier
                .weight(1f)
                .clip(RoundedCornerShape(MavioRadius.md))
                .clickable { navigator.play(card.serverId, card.itemId, card.title, null, null) },
            verticalAlignment = Alignment.CenterVertically,
            horizontalArrangement = Arrangement.spacedBy(MavioSpacing.s12),
        ) {
            Text(
                (index + 1).toString(),
                Modifier.width(20.dp),
                style = theme.type.callout.text,
                color = colors.textTertiary,
            )
            Artwork(
                (card.thumb ?: card.poster)?.url,
                Modifier
                    .width(80.dp)
                    .aspectRatio(16f / 9f)
                    .clip(RoundedCornerShape(MavioRadius.md)),
                icon = itemIcon(card.kind),
            )
            Column(Modifier.weight(1f)) {
                Text(card.title, style = theme.type.body.text, maxLines = 1, overflow = TextOverflow.Ellipsis)
                val sub = listOfNotNull(subtitleOf(card), card.runtime?.let { minutesText(it.toMinutes()) }).joinToString(" · ")
                if (sub.isNotEmpty()) {
                    Text(sub, style = theme.type.footnote.text, color = colors.textSecondary, maxLines = 1, overflow = TextOverflow.Ellipsis)
                }
            }
        }
        SmallButton(R.drawable.ic_expand, stringResource(R.string.lists_move_up), enabled = !busy && index > 0, flipped = true) { onMove(index - 1) }
        SmallButton(R.drawable.ic_expand, stringResource(R.string.lists_move_down), enabled = !busy && !last) { onMove(index + 1) }
        SmallButton(R.drawable.ic_close, stringResource(R.string.lists_remove), enabled = !busy, onClick = onRemove)
    }
}

@Composable
private fun SmallButton(@DrawableRes icon: Int, label: String, enabled: Boolean, flipped: Boolean = false, onClick: () -> Unit) {
    Box(
        Modifier
            .size(36.dp)
            .clip(CircleShape)
            .clickable(enabled = enabled, onClick = onClick)
            .alpha(if (enabled) 1f else 0.3f),
        Alignment.Center,
    ) {
        MavioIcon(
            icon,
            Modifier.size(16.dp).rotate(if (flipped) 180f else 0f),
            tint = theme.colors.textSecondary,
            contentDescription = label,
        )
    }
}

/**
 * A collection's members, as the web UI's: administrators remove them after
 * tapping Edit; the details load again after each removal.
 */
@OptIn(ExperimentalLayoutApi::class)
@Composable
internal fun CollectionMembers(model: AppModel, details: Details) {
    val collection = details.card
    val navigator = LocalNavigator.current
    val scope = rememberCoroutineScope()
    var admin by remember(collection.serverId) { mutableStateOf(false) }
    var editing by remember { mutableStateOf(false) }
    var busy by remember { mutableStateOf(false) }
    var failure by remember { mutableStateOf<String?>(null) }
    LaunchedEffect(collection.serverId) {
        runCatching {
            model.client.call(collection.serverId, Procedure.users("GetCurrentUser"), getCurrentUserRequest {}, GetCurrentUserResponse::parseFrom).user
        }.onSuccess { admin = it.admin }
    }
    Column(Modifier.padding(vertical = MavioSpacing.s8)) {
        if (details.children.isEmpty()) {
            Text(
                stringResource(R.string.lists_empty_collection),
                Modifier.padding(horizontal = theme.layout.margin),
                style = theme.type.body.text,
                color = theme.colors.textSecondary,
            )
            return@Column
        }
        if (admin) {
            Box(Modifier.fillMaxWidth().padding(horizontal = theme.layout.margin), Alignment.CenterEnd) {
                MavioButton(
                    stringResource(if (editing) R.string.keys_done else R.string.lists_edit),
                    { editing = !editing },
                    kind = ButtonKind.Secondary,
                )
            }
        }
        failure?.let {
            Text(it, Modifier.padding(horizontal = theme.layout.margin, vertical = MavioSpacing.s8), style = theme.type.callout.text, color = theme.colors.danger)
        }
        FlowRow(
            Modifier.padding(horizontal = theme.layout.margin, vertical = MavioSpacing.s12),
            horizontalArrangement = Arrangement.spacedBy(theme.layout.gutter),
            verticalArrangement = Arrangement.spacedBy(MavioSpacing.s16),
        ) {
            details.children.forEach { c ->
                Box {
                    ItemCard(c, Modifier.clickable { navigator.open(c) })
                    if (editing) {
                        Box(
                            Modifier
                                .align(Alignment.TopEnd)
                                .offset(8.dp, (-8).dp)
                                .size(32.dp)
                                .clip(CircleShape)
                                .background(theme.colors.danger)
                                .clickable(enabled = !busy) {
                                    busy = true
                                    scope.launch {
                                        runCatching {
                                            model.client.call(
                                                collection.serverId,
                                                Procedure.collections("RemoveFromCollection"),
                                                removeFromCollectionRequest {
                                                    id = collection.itemId
                                                    itemIds += c.itemId
                                                },
                                                RemoveFromCollectionResponse::parseFrom,
                                            )
                                        }.onSuccess {
                                            failure = null
                                            model.bump(collection.serverId)
                                        }.onFailure { failure = it.reason }
                                        busy = false
                                    }
                                },
                            Alignment.Center,
                        ) {
                                                        MavioIcon(
                                R.drawable.ic_close,
                                Modifier.size(16.dp),
                                tint = Color.White,
                                contentDescription = stringResource(R.string.lists_remove_named, c.title),
                            )
                        }
                    }
                }
            }
        }
    }
}
