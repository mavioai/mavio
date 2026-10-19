package dev.mavio.core

import com.google.protobuf.duration
import java.time.Duration
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

/** What the user may do with items on a server: administer it, and report problems. */
data class ItemRights(val admin: Boolean, val mayReport: Boolean)

/** A playlist or collection to add an item to. */
data class ItemList(val id: String, val name: String)

/** The kinds of problems a report names, in the order its choice shows them. */
val issueTypes = listOf(IssueType.ISSUE_TYPE_VIDEO, IssueType.ISSUE_TYPE_AUDIO, IssueType.ISSUE_TYPE_SUBTITLES, IssueType.ISSUE_TYPE_OTHER)

/** Kinds a device plays, which Play On offers. */
val playableKinds = setOf(CardKind.MOVIE, CardKind.EPISODE, CardKind.VIDEO, CardKind.MUSIC_VIDEO, CardKind.TRACK, CardKind.AUDIOBOOK)

/** Kinds problems are reported with. */
val reportableKinds = setOf(CardKind.MOVIE, CardKind.SERIES, CardKind.SEASON, CardKind.EPISODE)

/** Kinds the providers identify. */
val identifiableKinds = setOf(
    CardKind.MOVIE, CardKind.SERIES, CardKind.ARTIST, CardKind.ALBUM, CardKind.MUSIC_VIDEO, CardKind.BOOK, CardKind.AUDIOBOOK,
)

/** Whether an item of [kind] can go into a playlist or collection. */
fun listable(kind: CardKind) = kind != CardKind.PLAYLIST && kind != CardKind.COLLECTION && kind != CardKind.FOLDER

private fun users(method: String) = "/mavio.user.v1.UserService/$method"
private fun sessions(method: String) = "/mavio.session.v1.SessionService/$method"
private fun issues(method: String) = "/mavio.acquisition.v1.IssueService/$method"
private fun items(method: String) = "/mavio.library.v1.ItemService/$method"
private fun playlists(method: String) = "/mavio.library.v1.PlaylistService/$method"
private fun collections(method: String) = "/mavio.library.v1.CollectionService/$method"
private fun metadata(method: String) = "/mavio.library.v1.MetadataService/$method"

/** What the user may do with items on [serverId]. */
suspend fun AppModel.itemRights(serverId: String): ItemRights {
    val user = client.call(serverId, users("GetCurrentUser"), getCurrentUserRequest {}, GetCurrentUserResponse::parseFrom).user
    return ItemRights(user.admin, user.admin || user.policy.reportIssues)
}

/** The playlists, or the collections by name, of [serverId]. */
suspend fun AppModel.itemLists(serverId: String, collection: Boolean): List<ItemList> =
    if (collection) {
        client.call(
            serverId,
            items("ListItems"),
            listItemsRequest {
                kinds += ItemKind.ITEM_KIND_COLLECTION
                sort += sortSpec { field = SortField.SORT_FIELD_NAME }
            },
            ListItemsResponse::parseFrom,
        ).itemsList.map { ItemList(it.id, it.name) }
    } else {
        client.call(serverId, playlists("ListPlaylists"), listPlaylistsRequest {}, ListPlaylistsResponse::parseFrom)
            .playlistsList.map { ItemList(it.id, it.name) }
    }

/** Adds [card] to the playlist or collection [listId]. */
suspend fun AppModel.addToList(card: Card, listId: String, collection: Boolean) {
    if (collection) {
        client.call(
            card.serverId,
            collections("AddToCollection"),
            addToCollectionRequest {
                id = listId
                itemIds += card.itemId
            },
            AddToCollectionResponse::parseFrom,
        )
    } else {
        client.call(
            card.serverId,
            playlists("AddToPlaylist"),
            addToPlaylistRequest {
                id = listId
                itemIds += card.itemId
            },
            AddToPlaylistResponse::parseFrom,
        )
    }
    bump(card.serverId)
}

/** Creates a playlist or collection named [name] holding [card]. */
suspend fun AppModel.createList(card: Card, name: String, collection: Boolean) {
    if (collection) {
        client.call(
            card.serverId,
            collections("CreateCollection"),
            createCollectionRequest {
                this.name = name
                itemIds += card.itemId
            },
            CreateCollectionResponse::parseFrom,
        )
    } else {
        client.call(
            card.serverId,
            playlists("CreatePlaylist"),
            createPlaylistRequest {
                this.name = name
                itemIds += card.itemId
            },
            CreatePlaylistResponse::parseFrom,
        )
    }
    bump(card.serverId)
}

/**
 * Reports a problem of [type] with [card], a series' with a season and,
 * within it, an episode when they are not zero; returns the issue's ID.
 */
suspend fun AppModel.reportIssue(card: Card, type: IssueType, season: Int, episode: Int, message: String): String? {
    val response = client.call(
        card.serverId,
        issues("CreateIssue"),
        createIssueRequest {
            itemId = card.itemId
            this.type = type
            seasonNumber = season
            episodeNumber = if (season > 0) episode else 0
            this.message = message
        },
        CreateIssueResponse::parseFrom,
    )
    return if (response.hasIssue()) response.issue.id else null
}

/** Refreshes [card]'s metadata, keeping it or, with [replace], replacing it all. */
suspend fun AppModel.refreshItem(card: Card, replace: Boolean) {
    client.call(
        card.serverId,
        metadata("RefreshItem"),
        refreshItemRequest {
            id = card.itemId
            replaceMetadata = replace
        },
        RefreshItemResponse::parseFrom,
    )
    bump(card.serverId)
}

/** Renames the playlist or collection [card]. */
suspend fun AppModel.renameList(card: Card, name: String) {
    if (card.kind == CardKind.PLAYLIST) {
        client.call(
            card.serverId,
            playlists("UpdatePlaylist"),
            updatePlaylistRequest {
                id = card.itemId
                this.name = name
            },
            UpdatePlaylistResponse::parseFrom,
        )
    } else {
        client.call(
            card.serverId,
            collections("UpdateCollection"),
            updateCollectionRequest {
                id = card.itemId
                this.name = name
            },
            UpdateCollectionResponse::parseFrom,
        )
    }
    bump(card.serverId)
}

/** Deletes the playlist or collection [card]. */
suspend fun AppModel.deleteList(card: Card) {
    if (card.kind == CardKind.PLAYLIST) {
        client.call(card.serverId, playlists("DeletePlaylist"), deletePlaylistRequest { id = card.itemId }, DeletePlaylistResponse::parseFrom)
    } else {
        client.call(card.serverId, collections("DeleteCollection"), deleteCollectionRequest { id = card.itemId }, DeleteCollectionResponse::parseFrom)
    }
    bump(card.serverId)
}

/** Asks the device of session [sessionId] to play [card] from [position]. */
suspend fun AppModel.playOn(card: Card, sessionId: String, position: Duration?) {
    client.call(
        card.serverId,
        sessions("SendCommand"),
        sendCommandRequest {
            this.sessionId = sessionId
            command = command {
                play = mavio.session.v1.play {
                    itemIds += card.itemId
                    position?.let { startPosition = duration { seconds = it.seconds } }
                }
            }
        },
        SendCommandResponse::parseFrom,
    )
}
