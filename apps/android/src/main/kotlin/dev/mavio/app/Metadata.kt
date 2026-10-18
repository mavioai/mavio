package dev.mavio.app

import androidx.activity.compose.rememberLauncherForActivityResult
import androidx.activity.result.PickVisualMediaRequest
import androidx.activity.result.contract.ActivityResultContracts
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.aspectRatio
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.text.BasicTextField
import androidx.compose.material3.CircularProgressIndicator
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
import androidx.compose.ui.graphics.SolidColor
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.input.KeyboardType
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import com.google.protobuf.MessageLite
import com.google.protobuf.duration
import com.google.protobuf.fieldMask
import com.google.protobuf.timestamp
import dev.mavio.core.AppModel
import dev.mavio.core.call
import dev.mavio.core.instant
import dev.mavio.design.MavioRadius
import dev.mavio.design.MavioSpacing
import dev.mavio.ui.Artwork
import dev.mavio.ui.ButtonKind
import dev.mavio.ui.MavioButton
import dev.mavio.ui.R
import dev.mavio.ui.text
import dev.mavio.ui.theme
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.launch
import kotlinx.coroutines.withContext
import mavio.library.v1.ItemOuterClass.GetItemResponse
import mavio.library.v1.ItemOuterClass.Image
import mavio.library.v1.ItemOuterClass.ImageKind
import mavio.library.v1.ItemOuterClass.Item
import mavio.library.v1.ItemOuterClass.ItemKind
import mavio.library.v1.ItemOuterClass.MetadataField
import mavio.library.v1.ItemOuterClass.SeriesStatus
import mavio.library.v1.Metadata.DeleteItemImageResponse
import mavio.library.v1.Metadata.ExternalIdKind
import mavio.library.v1.Metadata.IdentifyItemResponse
import mavio.library.v1.Metadata.ListExternalIdKindsResponse
import mavio.library.v1.Metadata.ListRemoteImagesResponse
import mavio.library.v1.Metadata.RemoteImage
import mavio.library.v1.Metadata.RemoteSearchResult
import mavio.library.v1.Metadata.SearchRemoteResponse
import mavio.library.v1.Metadata.SetItemImageResponse
import mavio.library.v1.Metadata.UpdateItemResponse
import mavio.library.v1.deleteItemImageRequest
import mavio.library.v1.getItemRequest
import mavio.library.v1.identifyItemRequest
import mavio.library.v1.itemMetadata
import mavio.library.v1.listExternalIdKindsRequest
import mavio.library.v1.listRemoteImagesRequest
import mavio.library.v1.searchRemoteRequest
import mavio.library.v1.setItemImageRequest
import mavio.library.v1.updateItemRequest
import java.math.BigDecimal
import java.time.LocalDate
import java.time.ZoneOffset

// An item's metadata from its More menu, for administrators, as the web
// UI's (docs/clients.md §6.3.22): its fields edited and locked, the item
// identified anew from the providers' search results, and its images chosen
// from the providers' or uploaded.

/** An item as the server has it, and the base URL its images' paths are relative to. */
private class Loaded(val item: Item, val base: String) {
    fun url(path: String) = if ("://" in path) path else base.trimEnd('/') + "/" + path.trimStart('/')
}

private suspend fun load(model: AppModel, serverId: String, itemId: String): Loaded {
    val r = model.client.callServer(serverId, Procedure.items("GetItem"), getItemRequest { id = itemId }.toByteArray())
    return Loaded(GetItemResponse.parseFrom(r.body).item, r.baseUrl)
}

private suspend fun <T : MessageLite> metadata(model: AppModel, serverId: String, method: String, request: MessageLite, parse: (ByteArray) -> T): T =
    model.client.call(serverId, Procedure.metadata(method), request, parse)

/** The item loaded, or the page's progress or failure while it is not. */
@Composable
private fun WithItem(model: AppModel, serverId: String, itemId: String, attempt: Int = 0, content: @Composable (Loaded) -> Unit) {
    var loaded by remember(serverId, itemId) { mutableStateOf<Loaded?>(null) }
    var failure by remember(serverId, itemId) { mutableStateOf<String?>(null) }
    LaunchedEffect(serverId, itemId, attempt) {
        runCatching { load(model, serverId, itemId) }
            .onSuccess { loaded = it; failure = null }
            .onFailure { failure = it.reason }
    }
    val shown = loaded
    when {
        shown != null -> content(shown)
        failure != null -> SettingsList(stringResource(R.string.state_error), failure) {}
        else -> Box(Modifier.fillMaxSize(), Alignment.Center) { CircularProgressIndicator() }
    }
}

// Edit

/** The text fields of the form by their ItemMetadata names, which are the update mask's paths. */
private typealias Fields = Map<String, String>

private fun list(v: List<String>) = v.joinToString(", ")
private fun unlist(v: String) = v.split(",").map { it.trim() }.filter { it.isNotEmpty() }
private fun num(n: Int) = if (n != 0) n.toString() else ""
private fun num(n: Double) = if (n != 0.0) BigDecimal.valueOf(n).stripTrailingZeros().toPlainString() else ""
private fun day(t: com.google.protobuf.Timestamp) = t.instant?.atOffset(ZoneOffset.UTC)?.toLocalDate()?.toString().orEmpty()

private fun fieldsOf(item: Item): Fields = mapOf(
    "name" to item.name,
    "original_title" to item.originalTitle,
    "sort_name" to item.sortName,
    "overview" to item.overview,
    "tagline" to item.tagline,
    "production_year" to num(item.productionYear),
    "premiere_date" to day(item.premiereDate),
    "end_date" to day(item.endDate),
    "runtime" to if (item.hasRuntime()) Math.round(item.runtime.seconds / 60.0).toString() else "",
    "official_rating" to item.officialRating,
    "custom_rating" to item.customRating,
    "community_rating" to num(item.communityRating),
    "critic_rating" to num(item.criticRating),
    "genres" to list(item.genresList),
    "tags" to list(item.tagsList),
    "studios" to list(item.studiosList),
    "production_locations" to list(item.productionLocationsList),
    "artists" to list(item.artistsList),
    "album_artists" to list(item.albumArtistsList),
    "index_number" to num(item.indexNumber),
    "parent_index_number" to num(item.parentIndexNumber),
    "absolute_number" to num(item.absoluteNumber),
    "series_status" to item.seriesStatusValue.toString(),
)

/** Whether a field's text is one the server takes; empty always is. */
private fun valid(path: String, v: String): Boolean {
    val t = v.trim()
    if (t.isEmpty()) return true
    return when (path) {
        "production_year" -> t.toIntOrNull()?.let { it in 0..9999 } == true
        "runtime", "index_number", "parent_index_number", "absolute_number" -> t.toIntOrNull()?.let { it >= 0 } == true
        "community_rating" -> t.toDoubleOrNull()?.let { it in 0.0..10.0 } == true
        "critic_rating" -> t.toDoubleOrNull()?.let { it in 0.0..100.0 } == true
        "premiere_date", "end_date" -> runCatching { LocalDate.parse(t) }.isSuccess
        else -> true
    }
}

private fun date(v: String) = v.trim().takeIf { it.isNotEmpty() }?.let {
    timestamp { seconds = LocalDate.parse(it).atStartOfDay(ZoneOffset.UTC).toEpochSecond() }
}

/** The fields that may be locked against refreshes, and their labels. */
private val lockable = listOf(
    MetadataField.METADATA_FIELD_NAME to R.string.metadata_lock_name,
    MetadataField.METADATA_FIELD_OVERVIEW to R.string.metadata_overview,
    MetadataField.METADATA_FIELD_GENRES to R.string.metadata_genres,
    MetadataField.METADATA_FIELD_TAGS to R.string.metadata_tags,
    MetadataField.METADATA_FIELD_STUDIOS to R.string.metadata_studios,
    MetadataField.METADATA_FIELD_PRODUCTION_LOCATIONS to R.string.metadata_locations,
    MetadataField.METADATA_FIELD_OFFICIAL_RATING to R.string.metadata_official_rating,
    MetadataField.METADATA_FIELD_RUNTIME to R.string.metadata_runtime,
    MetadataField.METADATA_FIELD_CAST to R.string.metadata_cast,
)

/** Edits an item's fields and locks, as the web UI's Edit Metadata; saves only those changed. */
@Composable
fun EditMetadataScreen(model: AppModel, route: Route.EditMetadata) {
    WithItem(model, route.serverId, route.itemId) { loaded -> EditForm(model, route.serverId, loaded.item) }
}

@Composable
private fun EditForm(model: AppModel, serverId: String, item: Item) {
    val navigator = LocalNavigator.current
    val scope = rememberCoroutineScope()
    val before = remember(item) { fieldsOf(item) }
    var fields by remember(item) { mutableStateOf(before) }
    var ids by remember(item) { mutableStateOf(item.externalIdsMap.toMap()) }
    var locked by remember(item) { mutableStateOf(item.locked) }
    var locks by remember(item) { mutableStateOf(item.lockedFieldsList.toSet()) }
    var kinds by remember { mutableStateOf<List<ExternalIdKind>>(emptyList()) }
    var saving by remember { mutableStateOf(false) }
    var failure by remember { mutableStateOf<String?>(null) }
    LaunchedEffect(item.kind) {
        runCatching {
            metadata(model, serverId, "ListExternalIdKinds", listExternalIdKindsRequest { itemKind = item.kind }, ListExternalIdKindsResponse::parseFrom).kindsList
        }.onSuccess { kinds = it }
    }
    val paths = buildList {
        before.keys.filter { before[it] != fields[it] }.forEach { add(it) }
        if (ids.filterValues { it.isNotBlank() } != item.externalIdsMap.filterValues { it.isNotBlank() }) add("external_ids")
        if (locked != item.locked) add("locked")
        if (locks != item.lockedFieldsList.toSet()) add("locked_fields")
    }
    val invalid = fields.any { (k, v) -> !valid(k, v) }
    val k = item.kind
    val video = k in setOf(ItemKind.ITEM_KIND_MOVIE, ItemKind.ITEM_KIND_EPISODE, ItemKind.ITEM_KIND_VIDEO, ItemKind.ITEM_KIND_MUSIC_VIDEO)
    val music = k in setOf(ItemKind.ITEM_KIND_MUSIC_ALBUM, ItemKind.ITEM_KIND_TRACK, ItemKind.ITEM_KIND_MUSIC_VIDEO)

    @Composable
    fun field(path: String, label: String, placeholder: String? = null, keyboard: KeyboardType = KeyboardType.Text) {
        val v = fields[path].orEmpty()
        TextRow(label, v, { fields = fields + (path to it) }, placeholder = placeholder, keyboard = keyboard, error = !valid(path, v), fieldWidth = 190)
    }

    val commas = stringResource(R.string.metadata_commas)
    SettingsList(stringResource(R.string.metadata_edit_title, item.name)) {
        item {
            Group {
                field("name", stringResource(R.string.metadata_name))
                RowDivider()
                field("original_title", stringResource(R.string.metadata_original_title))
                RowDivider()
                field("sort_name", stringResource(R.string.metadata_sort_name), placeholder = fields["name"])
            }
        }
        item {
            Group(stringResource(R.string.metadata_overview)) {
                AreaField(fields["overview"].orEmpty()) { fields = fields + ("overview" to it) }
            }
        }
        item {
            Group {
                if (k == ItemKind.ITEM_KIND_MOVIE || k == ItemKind.ITEM_KIND_SERIES) {
                    field("tagline", stringResource(R.string.metadata_tagline))
                    RowDivider()
                }
                if (k == ItemKind.ITEM_KIND_SEASON) {
                    field("index_number", stringResource(R.string.metadata_season_number), keyboard = KeyboardType.Number)
                    RowDivider()
                }
                if (k == ItemKind.ITEM_KIND_EPISODE) {
                    field("parent_index_number", stringResource(R.string.metadata_season_number), keyboard = KeyboardType.Number)
                    RowDivider()
                }
                if (k == ItemKind.ITEM_KIND_EPISODE || k == ItemKind.ITEM_KIND_TRACK) {
                    val label = if (k == ItemKind.ITEM_KIND_TRACK) R.string.metadata_track_number else R.string.metadata_episode_number
                    field("index_number", stringResource(label), keyboard = KeyboardType.Number)
                    RowDivider()
                }
                if (k == ItemKind.ITEM_KIND_EPISODE) {
                    field("absolute_number", stringResource(R.string.metadata_absolute_number), keyboard = KeyboardType.Number)
                    RowDivider()
                }
                field("production_year", stringResource(R.string.metadata_year), keyboard = KeyboardType.Number)
                RowDivider()
                val premiere = if (k == ItemKind.ITEM_KIND_SERIES) R.string.metadata_first_aired else R.string.metadata_release_date
                field("premiere_date", stringResource(premiere), placeholder = "YYYY-MM-DD")
                if (k == ItemKind.ITEM_KIND_SERIES) {
                    RowDivider()
                    field("end_date", stringResource(R.string.metadata_end_date), placeholder = "YYYY-MM-DD")
                    RowDivider()
                    ChoiceRow(
                        stringResource(R.string.metadata_status),
                        listOf(
                            SeriesStatus.SERIES_STATUS_UNSPECIFIED to stringResource(R.string.metadata_status_unknown),
                            SeriesStatus.SERIES_STATUS_CONTINUING to stringResource(R.string.metadata_continuing),
                            SeriesStatus.SERIES_STATUS_ENDED to stringResource(R.string.metadata_ended),
                            SeriesStatus.SERIES_STATUS_UNRELEASED to stringResource(R.string.metadata_unreleased),
                        ),
                        SeriesStatus.forNumber(fields["series_status"]?.toIntOrNull() ?: 0) ?: SeriesStatus.SERIES_STATUS_UNSPECIFIED,
                    ) { fields = fields + ("series_status" to it.number.toString()) }
                }
                if (video) {
                    RowDivider()
                    field("runtime", stringResource(R.string.metadata_runtime_minutes), keyboard = KeyboardType.Number)
                }
                RowDivider()
                field("official_rating", stringResource(R.string.metadata_official_rating))
                RowDivider()
                field("custom_rating", stringResource(R.string.metadata_custom_rating))
                RowDivider()
                field("community_rating", stringResource(R.string.metadata_community_rating), keyboard = KeyboardType.Decimal)
                RowDivider()
                field("critic_rating", stringResource(R.string.metadata_critic_rating), keyboard = KeyboardType.Decimal)
            }
        }
        item {
            Group {
                if (music) {
                    field("artists", stringResource(R.string.metadata_artists), placeholder = commas)
                    RowDivider()
                }
                if (k == ItemKind.ITEM_KIND_MUSIC_ALBUM) {
                    field("album_artists", stringResource(R.string.metadata_album_artists), placeholder = commas)
                    RowDivider()
                }
                field("genres", stringResource(R.string.metadata_genres), placeholder = commas)
                RowDivider()
                field("tags", stringResource(R.string.metadata_tags), placeholder = commas)
                RowDivider()
                field("studios", stringResource(R.string.metadata_studios), placeholder = commas)
                RowDivider()
                field("production_locations", stringResource(R.string.metadata_locations), placeholder = commas)
            }
        }
        if (kinds.isNotEmpty()) item {
            Group(stringResource(R.string.metadata_external_ids)) {
                kinds.forEachIndexed { i, x ->
                    if (i > 0) RowDivider()
                    TextRow(x.name, ids[x.key].orEmpty(), { ids = ids + (x.key to it) }, fieldWidth = 190)
                }
            }
        }
        item {
            Group(stringResource(R.string.metadata_locks), footer = stringResource(R.string.metadata_locks_footer)) {
                SwitchRow(stringResource(R.string.metadata_lock_all), locked) { locked = it }
                if (!locked) {
                    lockable.forEach { (f, label) ->
                        RowDivider()
                        SwitchRow(stringResource(label), f in locks) { on -> locks = if (on) locks + f else locks - f }
                    }
                }
            }
        }
        item {
            Column(verticalArrangement = Arrangement.spacedBy(MavioSpacing.s8)) {
                failure?.let { Text(it, style = theme.type.footnote.text, color = theme.colors.danger) }
                MavioButton(
                    if (saving) "…" else stringResource(R.string.metadata_save),
                    {
                        if (paths.isEmpty()) {
                            navigator.back()
                            return@MavioButton
                        }
                        saving = true
                        scope.launch {
                            runCatching {
                                metadata(
                                    model,
                                    serverId,
                                    "UpdateItem",
                                    updateItemRequest {
                                        id = item.id
                                        metadata = metadataOf(fields, ids, locked, locks)
                                        updateMask = fieldMask { this.paths += paths }
                                    },
                                    UpdateItemResponse::parseFrom,
                                )
                            }.onSuccess {
                                model.bump(serverId)
                                navigator.back()
                            }.onFailure { failure = it.reason }
                            saving = false
                        }
                    },
                    enabled = !saving && !invalid && fields["name"].orEmpty().isNotBlank(),
                )
            }
        }
    }
}

private fun metadataOf(f: Fields, ids: Map<String, String>, locked: Boolean, locks: Set<MetadataField>) = itemMetadata {
    fun s(path: String) = f[path].orEmpty().trim()
    name = s("name")
    originalTitle = s("original_title")
    sortName = s("sort_name")
    overview = s("overview")
    tagline = s("tagline")
    productionYear = s("production_year").toIntOrNull() ?: 0
    date(s("premiere_date"))?.let { premiereDate = it }
    date(s("end_date"))?.let { endDate = it }
    s("runtime").toLongOrNull()?.let { runtime = duration { seconds = it * 60 } }
    officialRating = s("official_rating")
    customRating = s("custom_rating")
    communityRating = s("community_rating").toDoubleOrNull() ?: 0.0
    criticRating = s("critic_rating").toDoubleOrNull() ?: 0.0
    genres += unlist(s("genres"))
    tags += unlist(s("tags"))
    studios += unlist(s("studios"))
    productionLocations += unlist(s("production_locations"))
    artists += unlist(s("artists"))
    albumArtists += unlist(s("album_artists"))
    indexNumber = s("index_number").toIntOrNull() ?: 0
    parentIndexNumber = s("parent_index_number").toIntOrNull() ?: 0
    absoluteNumber = s("absolute_number").toIntOrNull() ?: 0
    seriesStatusValue = s("series_status").toIntOrNull() ?: 0
    externalIds.putAll(ids.mapValues { it.value.trim() }.filterValues { it.isNotEmpty() })
    this.locked = locked
    lockedFields += locks
}

/** A field of several lines, filling its group. */
@Composable
private fun AreaField(value: String, onChange: (String) -> Unit) {
    BasicTextField(
        value,
        onChange,
        Modifier.fillMaxWidth().heightIn(min = 120.dp).padding(horizontal = 16.dp, vertical = 12.dp),
        textStyle = theme.type.body.text.copy(color = theme.colors.textPrimary),
        cursorBrush = SolidColor(theme.colors.textPrimary),
    )
}

// Identify

/** Searches the providers for what an item is and identifies it as a result, as the web UI's Identify. */
@Composable
fun IdentifyScreen(model: AppModel, route: Route.Identify) {
    WithItem(model, route.serverId, route.itemId) { loaded -> IdentifyPage(model, route.serverId, loaded.item) }
}

@Composable
private fun IdentifyPage(model: AppModel, serverId: String, item: Item) {
    val navigator = LocalNavigator.current
    val scope = rememberCoroutineScope()
    var name by remember { mutableStateOf(item.name) }
    var year by remember { mutableStateOf(num(item.productionYear)) }
    var query by remember { mutableStateOf(item.name to item.productionYear) }
    var results by remember { mutableStateOf<List<RemoteSearchResult>?>(null) }
    var searchFailure by remember { mutableStateOf<String?>(null) }
    var busy by remember { mutableStateOf(false) }
    var failure by remember { mutableStateOf<String?>(null) }
    LaunchedEffect(query) {
        results = null
        runCatching {
            metadata(
                model,
                serverId,
                "SearchRemote",
                searchRemoteRequest {
                    itemId = item.id
                    this.name = query.first
                    this.year = query.second
                },
                SearchRemoteResponse::parseFrom,
            ).resultsList
        }.onSuccess { results = it; searchFailure = null }
            .onFailure { results = emptyList(); searchFailure = it.reason }
    }
    SettingsList(stringResource(R.string.metadata_identify_title, item.name)) {
        item {
            Column(verticalArrangement = Arrangement.spacedBy(MavioSpacing.s12)) {
                Group {
                    TextRow(stringResource(R.string.metadata_name), name, { name = it }, fieldWidth = 190)
                    RowDivider()
                    TextRow(
                        stringResource(R.string.metadata_year),
                        year,
                        { year = it },
                        keyboard = KeyboardType.Number,
                        error = year.isNotBlank() && year.trim().toIntOrNull()?.let { it in 1800..3000 } != true,
                        fieldWidth = 80,
                    )
                }
                MavioButton(
                    stringResource(R.string.metadata_search),
                    { query = name.trim() to (year.trim().toIntOrNull() ?: 0) },
                    kind = ButtonKind.Secondary,
                    enabled = name.isNotBlank(),
                )
            }
        }
        item {
            val list = results
            Column(verticalArrangement = Arrangement.spacedBy(MavioSpacing.s8)) {
                failure?.let { Text(it, style = theme.type.footnote.text, color = theme.colors.danger) }
                Group {
                    when {
                        list == null -> Note(stringResource(R.string.metadata_searching))
                        searchFailure != null -> Note(searchFailure!!, failed = true)
                        list.isEmpty() -> Note(stringResource(R.string.metadata_no_results))
                        else -> list.forEachIndexed { i, r ->
                            if (i > 0) RowDivider()
                            ResultRow(r, busy) {
                                busy = true
                                scope.launch {
                                    runCatching {
                                        metadata(
                                            model,
                                            serverId,
                                            "IdentifyItem",
                                            identifyItemRequest {
                                                itemId = item.id
                                                externalIds.putAll(r.externalIdsMap)
                                            },
                                            IdentifyItemResponse::parseFrom,
                                        )
                                    }.onSuccess {
                                        model.bump(serverId)
                                        navigator.back()
                                    }.onFailure { failure = it.reason }
                                    busy = false
                                }
                            }
                        }
                    }
                }
            }
        }
    }
}

@Composable
private fun ResultRow(r: RemoteSearchResult, busy: Boolean, onIdentify: () -> Unit) {
    val colors = theme.colors
    Row(
        Modifier.fillMaxWidth().padding(horizontal = 16.dp, vertical = 12.dp),
        horizontalArrangement = Arrangement.spacedBy(MavioSpacing.s12),
        verticalAlignment = Alignment.Top,
    ) {
        Artwork(
            r.imageUrl.ifEmpty { null },
            Modifier.width(56.dp).aspectRatio(2f / 3f).clip(RoundedCornerShape(MavioRadius.md)),
        )
        Column(Modifier.weight(1f), verticalArrangement = Arrangement.spacedBy(2.dp)) {
            Text(
                if (r.year > 0) "${r.name} (${r.year})" else r.name,
                style = theme.type.headline.text,
            )
            Text(
                (listOf(r.provider) + r.externalIdsMap.map { (k, v) -> "$k $v" }).joinToString(" · "),
                style = theme.type.footnote.text,
                color = colors.textSecondary,
            )
            if (r.overview.isNotEmpty()) {
                Text(r.overview, style = theme.type.footnote.text, color = colors.textSecondary, maxLines = 2, overflow = TextOverflow.Ellipsis)
            }
        }
        MavioButton(stringResource(R.string.metadata_identify), onIdentify, kind = ButtonKind.Secondary, enabled = !busy)
    }
}

// Images

private val imageKinds = listOf(
    ImageKind.IMAGE_KIND_PRIMARY to R.string.metadata_primary,
    ImageKind.IMAGE_KIND_BACKDROP to R.string.metadata_backdrop,
    ImageKind.IMAGE_KIND_LOGO to R.string.metadata_logo,
    ImageKind.IMAGE_KIND_THUMB to R.string.metadata_thumb,
    ImageKind.IMAGE_KIND_BANNER to R.string.metadata_banner,
    ImageKind.IMAGE_KIND_ART to R.string.metadata_art,
    ImageKind.IMAGE_KIND_DISC to R.string.metadata_disc,
)

/** An item's images of a kind, deleted, replaced from the providers' or by an upload, as the web UI's Images. */
@Composable
fun ItemImagesScreen(model: AppModel, route: Route.ItemImages) {
    var attempt by remember { mutableIntStateOf(0) }
    WithItem(model, route.serverId, route.itemId, attempt) { loaded ->
        ImagesPage(model, route.serverId, loaded) { attempt++ }
    }
}

@Composable
private fun ImagesPage(model: AppModel, serverId: String, loaded: Loaded, reload: () -> Unit) {
    val item = loaded.item
    val context = LocalContext.current
    val scope = rememberCoroutineScope()
    var kind by remember { mutableStateOf(ImageKind.IMAGE_KIND_PRIMARY) }
    var remote by remember { mutableStateOf<List<RemoteImage>?>(null) }
    var remoteFailure by remember { mutableStateOf<String?>(null) }
    var busy by remember { mutableStateOf(false) }
    var failure by remember { mutableStateOf<String?>(null) }
    var deleting by remember { mutableStateOf<Image?>(null) }
    LaunchedEffect(kind) {
        remote = null
        runCatching {
            metadata(
                model,
                serverId,
                "ListRemoteImages",
                listRemoteImagesRequest {
                    itemId = item.id
                    this.kind = kind
                },
                ListRemoteImagesResponse::parseFrom,
            ).imagesList
        }.onSuccess { remote = it; remoteFailure = null }
            .onFailure { remote = emptyList(); remoteFailure = it.reason }
    }
    fun change(action: suspend () -> Unit) {
        busy = true
        scope.launch {
            runCatching { action() }
                .onSuccess {
                    failure = null
                    model.bump(serverId)
                    reload()
                }.onFailure { failure = it.reason }
            busy = false
        }
    }
    val pick = rememberLauncherForActivityResult(ActivityResultContracts.PickVisualMedia()) { uri ->
        if (uri == null) return@rememberLauncherForActivityResult
        change {
            val bytes = withContext(Dispatchers.IO) { context.contentResolver.openInputStream(uri)!!.use { it.readBytes() } }
            metadata(
                model,
                serverId,
                "SetItemImage",
                setItemImageRequest {
                    itemId = item.id
                    this.kind = kind
                    data = com.google.protobuf.ByteString.copyFrom(bytes)
                },
                SetItemImageResponse::parseFrom,
            )
        }
    }
    val wide = kind != ImageKind.IMAGE_KIND_PRIMARY && kind != ImageKind.IMAGE_KIND_DISC
    val current = item.imagesList.filter { it.kind == kind && it.url.isNotEmpty() }
    SettingsList(stringResource(R.string.metadata_images_title, item.name)) {
        item {
            Column(verticalArrangement = Arrangement.spacedBy(MavioSpacing.s12)) {
                Group {
                    ChoiceRow(stringResource(R.string.metadata_image_kind), imageKinds.map { it.first to stringResource(it.second) }, kind) { kind = it }
                }
                Row(horizontalArrangement = Arrangement.spacedBy(MavioSpacing.s12), verticalAlignment = Alignment.CenterVertically) {
                    MavioButton(
                        stringResource(R.string.metadata_upload_image),
                        { pick.launch(PickVisualMediaRequest(ActivityResultContracts.PickVisualMedia.ImageOnly)) },
                        kind = ButtonKind.Secondary,
                        enabled = !busy,
                    )
                    if (busy) Text("…", style = theme.type.footnote.text, color = theme.colors.textSecondary)
                }
                failure?.let { Text(it, style = theme.type.footnote.text, color = theme.colors.danger) }
            }
        }
        item {
            ImageSection(stringResource(R.string.metadata_current)) {
                if (current.isEmpty()) {
                    Text(stringResource(R.string.metadata_no_image), style = theme.type.callout.text, color = theme.colors.textSecondary)
                }
                ImageGrid(current, wide) { image, modifier ->
                    Column(modifier, verticalArrangement = Arrangement.spacedBy(MavioSpacing.s4)) {
                        Artwork(loaded.url(image.url), Modifier.fillMaxWidth().aspectRatio(if (wide) 16f / 9f else 2f / 3f).clip(RoundedCornerShape(MavioRadius.md)))
                        Row(verticalAlignment = Alignment.CenterVertically) {
                            Text(
                                if (image.width > 0 && image.height > 0) "${image.width}×${image.height}" else "",
                                Modifier.weight(1f),
                                style = theme.type.caption.text,
                                color = theme.colors.textSecondary,
                            )
                            Text(
                                stringResource(R.string.metadata_delete_image),
                                Modifier.clickable(enabled = !busy) { deleting = image },
                                style = theme.type.footnote.text,
                                fontWeight = FontWeight.SemiBold,
                                color = theme.colors.danger,
                            )
                        }
                    }
                }
            }
        }
        item {
            ImageSection(stringResource(R.string.metadata_from_providers)) {
                val list = remote
                when {
                    list == null -> Text(stringResource(R.string.metadata_searching), style = theme.type.callout.text, color = theme.colors.textSecondary)
                    remoteFailure != null -> Text(remoteFailure!!, style = theme.type.callout.text, color = theme.colors.danger)
                    list.isEmpty() -> Text(stringResource(R.string.metadata_no_remote_images), style = theme.type.callout.text, color = theme.colors.textSecondary)
                }
                ImageGrid(list.orEmpty(), wide) { r, modifier ->
                    Column(
                        modifier.clip(RoundedCornerShape(MavioRadius.md)).clickable(enabled = !busy) {
                            change {
                                metadata(
                                    model,
                                    serverId,
                                    "SetItemImage",
                                    setItemImageRequest {
                                        itemId = item.id
                                        this.kind = kind
                                        url = r.url
                                    },
                                    SetItemImageResponse::parseFrom,
                                )
                            }
                        },
                        verticalArrangement = Arrangement.spacedBy(MavioSpacing.s4),
                    ) {
                        Artwork(r.url, Modifier.fillMaxWidth().aspectRatio(if (wide) 16f / 9f else 2f / 3f).clip(RoundedCornerShape(MavioRadius.md)))
                        Text(
                            listOf(r.provider, if (r.width > 0 && r.height > 0) "${r.width}×${r.height}" else "", r.language)
                                .filter { it.isNotEmpty() }.joinToString(" · "),
                            style = theme.type.caption.text,
                            color = theme.colors.textSecondary,
                            maxLines = 1,
                            overflow = TextOverflow.Ellipsis,
                        )
                    }
                }
            }
        }
    }
    deleting?.let { image ->
        ConfirmDialog(
            stringResource(R.string.metadata_delete_image),
            null,
            stringResource(R.string.metadata_confirm_delete),
            onConfirm = {
                deleting = null
                change {
                    metadata(
                        model,
                        serverId,
                        "DeleteItemImage",
                        deleteItemImageRequest {
                            itemId = item.id
                            imageId = image.id
                        },
                        DeleteItemImageResponse::parseFrom,
                    )
                }
            },
            onDismiss = { deleting = null },
        )
    }
}

@Composable
private fun ImageSection(title: String, content: @Composable () -> Unit) {
    Column(verticalArrangement = Arrangement.spacedBy(MavioSpacing.s8)) {
        Text(title, style = theme.type.headline.text)
        content()
    }
}

/** Images in rows, two wide ones or three posters to a row. */
@Composable
private fun <T> ImageGrid(images: List<T>, wide: Boolean, cell: @Composable (T, Modifier) -> Unit) {
    val columns = if (wide) 2 else 3
    Column(verticalArrangement = Arrangement.spacedBy(MavioSpacing.s12)) {
        images.chunked(columns).forEach { row ->
            Row(horizontalArrangement = Arrangement.spacedBy(MavioSpacing.s12)) {
                row.forEach { cell(it, Modifier.weight(1f)) }
                repeat(columns - row.size) { Spacer(Modifier.weight(1f)) }
            }
        }
    }
}
