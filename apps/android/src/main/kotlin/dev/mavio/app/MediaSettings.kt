package dev.mavio.app

import androidx.annotation.DrawableRes
import androidx.annotation.StringRes
import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxHeight
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.ModalBottomSheet
import androidx.compose.material3.Text
import androidx.compose.material3.rememberModalBottomSheetState
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateMapOf
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.platform.LocalFocusManager
import androidx.compose.ui.res.pluralStringResource
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.Dp
import androidx.compose.ui.unit.dp
import com.google.protobuf.Duration
import dev.mavio.core.AppModel
import dev.mavio.core.call
import dev.mavio.core.choices
import dev.mavio.core.localization
import dev.mavio.design.MavioSpacing
import dev.mavio.ui.ButtonKind
import dev.mavio.ui.ChoiceRow
import dev.mavio.ui.Group
import dev.mavio.ui.LinkRow
import dev.mavio.ui.MavioButton
import dev.mavio.ui.MavioIcon
import dev.mavio.ui.R
import dev.mavio.ui.RowDivider
import dev.mavio.ui.SettingsField
import dev.mavio.ui.SettingsList
import dev.mavio.ui.StatusView
import dev.mavio.ui.SwitchRow
import dev.mavio.ui.text
import dev.mavio.ui.theme
import java.text.Collator
import kotlinx.coroutines.launch
import mavio.library.v1.LibraryOuterClass.CreateLibraryResponse
import mavio.library.v1.LibraryOuterClass.DeleteLibraryResponse
import mavio.library.v1.LibraryOuterClass.GetLibraryResponse
import mavio.library.v1.LibraryOuterClass.Library
import mavio.library.v1.LibraryOuterClass.LibraryKind
import mavio.library.v1.LibraryOuterClass.LibrarySpec
import mavio.library.v1.LibraryOuterClass.ListLibrariesResponse
import mavio.library.v1.LibraryOuterClass.ScanLibraryResponse
import mavio.library.v1.LibraryOuterClass.StorageClass
import mavio.library.v1.LibraryOuterClass.UpdateLibraryResponse
import mavio.library.v1.createLibraryRequest
import mavio.library.v1.deleteLibraryRequest
import mavio.library.v1.getLibraryRequest
import mavio.library.v1.librarySpec
import mavio.library.v1.listLibrariesRequest
import mavio.library.v1.scanLibraryRequest
import mavio.library.v1.updateLibraryRequest
import mavio.library.v1.virtualEpisodes
import mavio.storage.v1.Storage.GetStatusResponse
import mavio.storage.v1.Storage.ListFoldersResponse
import mavio.storage.v1.Storage.ListRemotesResponse
import mavio.storage.v1.getStatusRequest
import mavio.storage.v1.listFoldersRequest
import mavio.storage.v1.listRemotesRequest
import mavio.system.v1.System.ListDirectoryResponse
import mavio.system.v1.listDirectoryRequest

// The Media menu's Libraries (docs/clients.md §6.3.4), as the web UI's on a
// phone: the server's libraries, to scan, and a library's page to add,
// change and delete it.

/** A kind of library as the settings show it: its name, icon and color. */
private enum class LibraryKindStyle(val kind: LibraryKind, @StringRes val title: Int, @DrawableRes val icon: Int, val color: Color) {
    Movies(LibraryKind.LIBRARY_KIND_MOVIES, R.string.libraries_kind_movies, R.drawable.ic_movies_fill, Color(0xFFFF9500)),
    Shows(LibraryKind.LIBRARY_KIND_SHOWS, R.string.libraries_kind_shows, R.drawable.ic_shows_fill, Color(0xFF007AFF)),
    Music(LibraryKind.LIBRARY_KIND_MUSIC, R.string.libraries_kind_music, R.drawable.ic_music_fill, Color(0xFFFF2D55)),
    MusicVideos(LibraryKind.LIBRARY_KIND_MUSIC_VIDEOS, R.string.libraries_kind_music_videos, R.drawable.ic_music_fill, Color(0xFFAF52DE)),
    HomeVideos(LibraryKind.LIBRARY_KIND_HOME_VIDEOS, R.string.libraries_kind_home_videos, R.drawable.ic_home_videos_fill, Color(0xFF34C759)),
    Books(LibraryKind.LIBRARY_KIND_BOOKS, R.string.libraries_kind_books, R.drawable.ic_books_fill, Color(0xFFA2845E)),
    Photos(LibraryKind.LIBRARY_KIND_PHOTOS, R.string.libraries_kind_photos, R.drawable.ic_photos_fill, Color(0xFF32ADE6)),
    Mixed(LibraryKind.LIBRARY_KIND_MIXED, R.string.libraries_kind_mixed, R.drawable.ic_folder_fill, Color(0xFF8E8E93)),
    ;

    val video get() = this in listOf(Movies, Shows, MusicVideos, HomeVideos, Mixed)
    val music get() = this == Music || this == Mixed
    val movies get() = this == Movies || this == Mixed
    val shows get() = this == Shows || this == Mixed

    companion object {
        fun of(kind: LibraryKind) = entries.firstOrNull { it.kind == kind } ?: Mixed
    }
}

/** A tile of a color with an icon, as the settings' entries have. */
@Composable
private fun ColorTile(@DrawableRes icon: Int, color: Color, size: Dp = 28.dp) {
    Box(Modifier.size(size).background(color, RoundedCornerShape(size / 4)), Alignment.Center) {
        MavioIcon(icon, size = size * 0.6f, tint = Color.White)
    }
}

/** Asks a server to scan a library, telling what came of it. */
private suspend fun scan(model: AppModel, serverId: String, id: String): Pair<Int, String?> = try {
    val r = model.client.call(serverId, Procedure.libraries("ScanLibrary"), scanLibraryRequest { this.id = id }, ScanLibraryResponse::parseFrom)
    (if (r.enqueued) R.string.libraries_scan_queued else R.string.libraries_scan_pending) to null
} catch (e: kotlinx.coroutines.CancellationException) {
    throw e
} catch (e: Exception) {
    0 to e.reason
}

/** Libraries: the server's libraries with their kinds and folders, each to scan and to open. */
@Composable
internal fun LibrariesPage(model: AppModel, serverId: String) {
    var libraries by remember { mutableStateOf<List<Library>?>(null) }
    var failure by remember { mutableStateOf<String?>(null) }
    // What scanning a library told, by its ID: a string's resource, or why it failed.
    val scanned = remember { mutableStateMapOf<String, Pair<Int, String?>>() }
    val scanning = remember { mutableStateMapOf<String, Boolean>() }
    val scope = rememberCoroutineScope()
    val navigator = LocalNavigator.current
    LaunchedEffect(serverId) {
        attempt({ failure = it }) {
            val collator = Collator.getInstance()
            libraries = model.client.call(serverId, Procedure.libraries("ListLibraries"), listLibrariesRequest {}, ListLibrariesResponse::parseFrom)
                .librariesList
                .filter { it.kind != LibraryKind.LIBRARY_KIND_COLLECTIONS && it.kind != LibraryKind.LIBRARY_KIND_PLAYLISTS }
                .sortedWith { a, b -> collator.compare(a.name, b.name) }
            failure = null
        }
    }
    SettingsList(stringResource(R.string.settings_libraries), stringResource(R.string.libraries_message)) {
        val list = libraries
        if (list == null) {
            item { if (failure == null) StatusView(stringResource(R.string.state_loading)) else StatusView(failure!!, icon = R.drawable.ic_error) }
            return@SettingsList
        }
        item {
            Group {
                list.forEach { l ->
                    val k = LibraryKindStyle.of(l.kind)
                    Row(
                        Modifier.fillMaxWidth().clickable { navigator.open(Route.ManagedLibrary(serverId, l.id)) }
                            .padding(start = 16.dp, end = 12.dp, top = 8.dp, bottom = 8.dp),
                        horizontalArrangement = Arrangement.spacedBy(MavioSpacing.s16),
                        verticalAlignment = Alignment.CenterVertically,
                    ) {
                        ColorTile(k.icon, k.color)
                        val told = scanned[l.id]
                        Lines(
                            l.name,
                            told?.let { it.second ?: stringResource(it.first) }
                                ?: "${stringResource(k.title)} · ${pluralStringResource(R.plurals.libraries_folder_count, l.pathsCount, l.pathsCount)}",
                            Modifier.weight(1f),
                            maxLines = 1,
                        )
                        val busy = scanning[l.id] == true
                        MavioButton(
                            if (busy) "…" else stringResource(R.string.libraries_scan),
                            {
                                scanning[l.id] = true
                                scope.launch {
                                    scanned[l.id] = scan(model, serverId, l.id)
                                    scanning.remove(l.id)
                                }
                            },
                            kind = ButtonKind.Secondary,
                            enabled = !busy,
                        )
                    }
                    RowDivider(tiled = true)
                }
                if (list.isEmpty()) {
                    Note(stringResource(R.string.libraries_none))
                    RowDivider()
                }
                LinkRow(stringResource(R.string.libraries_add), R.drawable.ic_add) { navigator.open(Route.ManagedLibrary(serverId, null)) }
            }
        }
    }
}

/** A library as its page edits it: the fields of LibrarySpec the page shows. */
private data class LibraryDraft(
    val name: String = "",
    val kind: LibraryKind = LibraryKind.LIBRARY_KIND_MOVIES,
    val paths: List<String> = emptyList(),
    val storageClasses: Map<String, StorageClass> = emptyMap(),
    val scanSeconds: Long = 12 * 3600,
    val language: String = "",
    val country: String = "",
    val saveLocalMetadata: Boolean = false,
    val autoCollections: Boolean = false,
    val extractTrickplay: Boolean = false,
    val extractChapterImages: Boolean = false,
    val analyzeLoudness: Boolean = false,
    val downloadLyrics: Boolean = false,
    val missing: Boolean = false,
    val unaired: Boolean = false,
    val specials: Boolean = false,
    val graceDays: Int = 0,
) {
    constructor(l: Library) : this(
        name = l.name,
        kind = l.kind,
        paths = l.pathsList,
        storageClasses = l.storageClassesMap,
        scanSeconds = if (l.hasScanInterval()) l.scanInterval.seconds else 0,
        language = l.preferredLanguage,
        country = l.metadataCountry,
        saveLocalMetadata = l.saveLocalMetadata,
        autoCollections = l.autoCollections,
        extractTrickplay = l.extractTrickplay,
        extractChapterImages = l.extractChapterImages,
        analyzeLoudness = l.analyzeLoudness,
        downloadLyrics = l.downloadLyrics,
        missing = l.virtualEpisodes.missing,
        unaired = l.virtualEpisodes.unaired,
        specials = l.virtualEpisodes.specials,
        graceDays = l.virtualEpisodes.graceDays,
    )

    val valid get() = name.isNotBlank() && paths.isNotEmpty() && graceDays in 0..3650

    /** The spec of the draft, keeping what the page does not show of the library. */
    fun spec(keeping: Library?): LibrarySpec = librarySpec {
        name = this@LibraryDraft.name.trim()
        kind = this@LibraryDraft.kind
        paths += this@LibraryDraft.paths
        if (scanSeconds > 0) scanInterval = Duration.newBuilder().setSeconds(scanSeconds).build()
        preferredLanguage = language
        metadataCountry = country
        saveLocalMetadata = this@LibraryDraft.saveLocalMetadata
        autoCollections = this@LibraryDraft.autoCollections
        extractTrickplay = this@LibraryDraft.extractTrickplay
        extractChapterImages = this@LibraryDraft.extractChapterImages
        analyzeLoudness = this@LibraryDraft.analyzeLoudness
        downloadLyrics = this@LibraryDraft.downloadLyrics
        if (keeping != null) {
            if (keeping.hasProviderOrder()) providerOrder = keeping.providerOrder
            if (keeping.hasHubs()) hubs = keeping.hubs
        }
        virtualEpisodes = virtualEpisodes {
            missing = this@LibraryDraft.missing
            unaired = this@LibraryDraft.unaired
            specials = this@LibraryDraft.specials
            graceDays = this@LibraryDraft.graceDays
        }
        storageClasses.putAll(this@LibraryDraft.storageClasses.filter { it.key in this@LibraryDraft.paths && it.value != StorageClass.STORAGE_CLASS_UNSPECIFIED })
    }
}

/**
 * A library's page, or a new library's: its name and kind, its folders with
 * their storage, its metadata, how often it is scanned and what scans make,
 * and the missing episodes of its series.
 */
@Composable
fun ManagedLibraryScreen(model: AppModel, route: Route.ManagedLibrary) {
    val serverId = route.serverId
    // The library as saved; null until a new one is added.
    var library by remember { mutableStateOf<Library?>(null) }
    var loaded by remember { mutableStateOf(route.id == null) }
    var draft by remember { mutableStateOf(LibraryDraft()) }
    var languages by remember { mutableStateOf<List<Pair<String, String>>>(emptyList()) }
    var countries by remember { mutableStateOf<List<Pair<String, String>>>(emptyList()) }
    var typed by rememberSaveable { mutableStateOf("") }
    var saving by remember { mutableStateOf(false) }
    var scanning by remember { mutableStateOf(false) }
    var failure by remember { mutableStateOf<String?>(null) }
    var notice by remember { mutableStateOf<String?>(null) }
    var confirming by remember { mutableStateOf(false) }
    var browsing by remember { mutableStateOf(false) }
    val scope = rememberCoroutineScope()
    val navigator = LocalNavigator.current
    val focus = LocalFocusManager.current
    val told = mapOf(
        R.string.libraries_scan_queued to stringResource(R.string.libraries_scan_queued),
        R.string.libraries_scan_pending to stringResource(R.string.libraries_scan_pending),
    )
    LaunchedEffect(serverId, route.id) {
        if (route.id != null) {
            attempt({ failure = it }) {
                val l = model.client.call(serverId, Procedure.libraries("GetLibrary"), getLibraryRequest { id = route.id }, GetLibraryResponse::parseFrom).library
                library = l
                draft = LibraryDraft(l)
                loaded = true
                failure = null
            }
        }
        model.localization(serverId).let { (l, c) ->
            languages = l
            countries = c
        }
    }
    val saved = library?.let(::LibraryDraft) ?: LibraryDraft()
    val k = LibraryKindStyle.of(draft.kind)
    fun set(change: LibraryDraft.() -> LibraryDraft) {
        draft = draft.change()
    }
    fun add(path: String) {
        var p = path.trim()
        while (p.length > 1 && p.endsWith("/")) p = p.dropLast(1)
        if (p.isNotEmpty() && p !in draft.paths) set { copy(paths = paths + p) }
    }
    SettingsList(library?.name ?: stringResource(R.string.libraries_add)) {
        if (!loaded) {
            item { if (failure == null) StatusView(stringResource(R.string.state_loading)) else StatusView(failure!!, icon = R.drawable.ic_error) }
            return@SettingsList
        }
        item {
            Row(horizontalArrangement = Arrangement.spacedBy(MavioSpacing.s12), verticalAlignment = Alignment.CenterVertically) {
                ColorTile(k.icon, k.color, 40.dp)
                Text(stringResource(k.title), style = theme.type.title3.text, color = theme.colors.textSecondary)
            }
        }
        item {
            Group {
                TextRow(stringResource(R.string.libraries_name), draft.name, { set { copy(name = it) } })
                RowDivider()
                ChoiceRow(stringResource(R.string.libraries_kind), LibraryKindStyle.entries.map { it.kind to stringResource(it.title) }, draft.kind) {
                    set { copy(kind = it) }
                }
            }
        }
        item {
            Group(stringResource(R.string.libraries_folders), stringResource(R.string.libraries_folders_footer)) {
                draft.paths.forEach { path ->
                    Column(Modifier.fillMaxWidth().padding(start = 16.dp, end = 4.dp, top = 6.dp)) {
                        Row(verticalAlignment = Alignment.CenterVertically) {
                            Text(path, Modifier.weight(1f), style = theme.type.body.text)
                            Box(
                                Modifier.size(40.dp).clickable(onClickLabel = stringResource(R.string.libraries_remove_folder)) {
                                    set { copy(paths = paths - path) }
                                },
                                Alignment.Center,
                            ) {
                                MavioIcon(R.drawable.ic_close, size = 16.dp, tint = theme.colors.textSecondary)
                            }
                        }
                    }
                    Box(Modifier.padding(start = 0.dp)) {
                        ChoiceRow(
                            stringResource(R.string.libraries_storage_class),
                            listOf(
                                StorageClass.STORAGE_CLASS_UNSPECIFIED to autoStorage(library?.effectiveStorageClassesMap?.get(path)),
                                StorageClass.STORAGE_CLASS_LOCAL to stringResource(R.string.libraries_storage_local),
                                StorageClass.STORAGE_CLASS_NETWORK to stringResource(R.string.libraries_storage_network),
                                StorageClass.STORAGE_CLASS_CLOUD to stringResource(R.string.libraries_storage_cloud),
                            ),
                            draft.storageClasses[path] ?: StorageClass.STORAGE_CLASS_UNSPECIFIED,
                        ) { c -> set { copy(storageClasses = storageClasses + (path to c)) } }
                    }
                    RowDivider()
                }
                Row(
                    Modifier.fillMaxWidth().padding(horizontal = 16.dp, vertical = 10.dp),
                    horizontalArrangement = Arrangement.spacedBy(MavioSpacing.s8),
                    verticalAlignment = Alignment.CenterVertically,
                ) {
                    SettingsField(typed, { typed = it }, stringResource(R.string.libraries_folder_path), Modifier.weight(1f))
                    MavioButton(
                        stringResource(R.string.libraries_add_folder),
                        {
                            add(typed)
                            typed = ""
                        },
                        kind = ButtonKind.Secondary,
                        enabled = typed.isNotBlank(),
                    )
                }
                RowDivider()
                LinkRow(stringResource(R.string.libraries_browse), R.drawable.ic_folder) { browsing = true }
            }
        }
        item {
            Group(stringResource(R.string.libraries_metadata)) {
                ChoiceRow(
                    stringResource(R.string.libraries_language),
                    listOf("" to stringResource(R.string.libraries_server_default)) + choices(languages, draft.language),
                    draft.language,
                ) { set { copy(language = it) } }
                RowDivider()
                ChoiceRow(
                    stringResource(R.string.libraries_country),
                    listOf("" to stringResource(R.string.libraries_server_default)) + choices(countries, draft.country),
                    draft.country,
                ) { set { copy(country = it) } }
                RowDivider()
                SwitchRow(
                    stringResource(R.string.libraries_save_local_metadata),
                    draft.saveLocalMetadata,
                    stringResource(R.string.libraries_save_local_metadata_detail),
                ) { set { copy(saveLocalMetadata = it) } }
                if (k.movies) {
                    RowDivider()
                    SwitchRow(
                        stringResource(R.string.libraries_auto_collections),
                        draft.autoCollections,
                        stringResource(R.string.libraries_auto_collections_detail),
                    ) { set { copy(autoCollections = it) } }
                }
                if (k.music) {
                    RowDivider()
                    SwitchRow(stringResource(R.string.libraries_download_lyrics), draft.downloadLyrics) { set { copy(downloadLyrics = it) } }
                }
            }
        }
        item {
            Group(stringResource(R.string.libraries_scanning)) {
                val intervals = listOf(
                    0L to stringResource(R.string.libraries_scan_never),
                    3600L to stringResource(R.string.libraries_scan_hourly),
                    6 * 3600L to stringResource(R.string.libraries_scan_every_hours, "6"),
                    12 * 3600L to stringResource(R.string.libraries_scan_every_hours, "12"),
                    24 * 3600L to stringResource(R.string.libraries_scan_daily),
                    7 * 24 * 3600L to stringResource(R.string.libraries_scan_weekly),
                )
                ChoiceRow(
                    stringResource(R.string.libraries_scan_interval),
                    if (intervals.any { it.first == draft.scanSeconds }) {
                        intervals
                    } else {
                        intervals + (draft.scanSeconds to stringResource(R.string.libraries_scan_every_hours, (draft.scanSeconds / 3600).toString()))
                    },
                    draft.scanSeconds,
                ) { set { copy(scanSeconds = it) } }
                if (k.video) {
                    RowDivider()
                    SwitchRow(stringResource(R.string.libraries_trickplay), draft.extractTrickplay, stringResource(R.string.libraries_trickplay_detail)) {
                        set { copy(extractTrickplay = it) }
                    }
                    RowDivider()
                    SwitchRow(stringResource(R.string.libraries_chapter_images), draft.extractChapterImages) { set { copy(extractChapterImages = it) } }
                }
                if (k.music) {
                    RowDivider()
                    SwitchRow(stringResource(R.string.libraries_loudness), draft.analyzeLoudness, stringResource(R.string.libraries_loudness_detail)) {
                        set { copy(analyzeLoudness = it) }
                    }
                }
            }
        }
        if (k.shows) {
            item {
                Group(stringResource(R.string.libraries_missing_episodes), stringResource(R.string.libraries_missing_episodes_footer)) {
                    SwitchRow(stringResource(R.string.libraries_missing_aired), draft.missing) { set { copy(missing = it) } }
                    RowDivider()
                    SwitchRow(stringResource(R.string.libraries_missing_unaired), draft.unaired) { set { copy(unaired = it) } }
                    RowDivider()
                    SwitchRow(stringResource(R.string.libraries_missing_specials), draft.specials) { set { copy(specials = it) } }
                    if (!draft.missing) {
                        RowDivider()
                        NumberRow(
                            stringResource(R.string.libraries_grace_days),
                            draft.graceDays.toDouble(),
                            0.0..3650.0,
                            { set { copy(graceDays = it.toInt()) } },
                            stringResource(R.string.libraries_grace_days_detail),
                        )
                    }
                }
            }
        }
        item {
            val savedNotice = stringResource(R.string.libraries_saved)
            Row(horizontalArrangement = Arrangement.spacedBy(MavioSpacing.s12), verticalAlignment = Alignment.CenterVertically) {
                MavioButton(
                    if (saving) "…" else stringResource(if (library == null) R.string.libraries_create else R.string.action_save),
                    {
                        focus.clearFocus()
                        saving = true
                        failure = null
                        scope.launch {
                            attempt({ failure = it }) {
                                val current = library
                                library = if (current != null) {
                                    model.client.call(
                                        serverId,
                                        Procedure.libraries("UpdateLibrary"),
                                        updateLibraryRequest {
                                            id = current.id
                                            spec = draft.spec(current)
                                        },
                                        UpdateLibraryResponse::parseFrom,
                                    ).library
                                } else {
                                    model.client.call(
                                        serverId,
                                        Procedure.libraries("CreateLibrary"),
                                        createLibraryRequest { spec = draft.spec(null) },
                                        CreateLibraryResponse::parseFrom,
                                    ).library
                                }
                                draft = LibraryDraft(library!!)
                                notice = savedNotice
                            }
                            saving = false
                        }
                    },
                    enabled = draft != saved && draft.valid && !saving,
                )
                when {
                    failure != null -> Text(failure!!, style = theme.type.callout.text, color = theme.colors.danger)
                    notice != null && draft == saved -> Text(notice!!, style = theme.type.callout.text, color = theme.colors.textSecondary)
                }
            }
        }
        library?.let { l ->
            item {
                Group {
                    ActionRow(if (scanning) "…" else stringResource(R.string.libraries_scan), theme.colors.accentText, !scanning) {
                        scanning = true
                        scope.launch {
                            val (res, reason) = scan(model, serverId, l.id)
                            notice = reason ?: told[res]
                            scanning = false
                        }
                    }
                    RowDivider()
                    ActionRow(stringResource(R.string.action_delete), theme.colors.danger, true) { confirming = true }
                }
            }
        }
    }
    if (confirming) {
        library?.let { l ->
            ConfirmDialog(
                stringResource(R.string.libraries_delete_title, l.name),
                stringResource(R.string.libraries_delete_message),
                stringResource(R.string.action_delete),
                onConfirm = {
                    scope.launch {
                        attempt({ failure = it }) {
                            model.client.call(serverId, Procedure.libraries("DeleteLibrary"), deleteLibraryRequest { id = l.id }, DeleteLibraryResponse::parseFrom)
                            navigator.back()
                        }
                    }
                },
                onDismiss = { confirming = false },
            )
        }
    }
    if (browsing) {
        FolderSheet(model, serverId, draft.paths, { add(it) }) { browsing = false }
    }
}

/** Automatic storage, with what the server finds it is. */
@Composable
private fun autoStorage(effective: StorageClass?): String = when (effective) {
    StorageClass.STORAGE_CLASS_LOCAL -> stringResource(R.string.libraries_storage_auto_is, stringResource(R.string.libraries_storage_local))
    StorageClass.STORAGE_CLASS_NETWORK -> stringResource(R.string.libraries_storage_auto_is, stringResource(R.string.libraries_storage_network))
    StorageClass.STORAGE_CLASS_CLOUD -> stringResource(R.string.libraries_storage_auto_is, stringResource(R.string.libraries_storage_cloud))
    else -> stringResource(R.string.libraries_storage_auto)
}

/** The folder above a remote's: the remote's root, or the roots above it. */
private fun remoteParent(path: String): String {
    val rest = path.removePrefix("rclone:")
    val colon = rest.indexOf(':')
    if (colon < 0 || colon == rest.length - 1) return ""
    val slash = path.lastIndexOf('/')
    return if (slash >= 0) path.take(slash) else "rclone:" + rest.take(colon + 1)
}

/** The folders of the server's file system and of its rclone remotes, to walk and choose one from. */
@OptIn(ExperimentalMaterial3Api::class)
@Composable
private fun FolderSheet(model: AppModel, serverId: String, chosen: List<String>, choose: (String) -> Unit, onDismiss: () -> Unit) {
    var path by remember { mutableStateOf("") }
    var here by remember { mutableStateOf("") }
    var parent by remember { mutableStateOf("") }
    var folders by remember { mutableStateOf<List<Pair<String, String>>?>(null) }
    var failure by remember { mutableStateOf<String?>(null) }
    LaunchedEffect(path) {
        folders = null
        failure = null
        attempt({ failure = it }) {
            if (path.startsWith("rclone:")) {
                val r = model.client.call(serverId, Procedure.storage("ListFolders"), listFoldersRequest { this.path = path }, ListFoldersResponse::parseFrom)
                here = path
                parent = remoteParent(path)
                folders = r.foldersList.map { f -> f to f.substring(maxOf(f.lastIndexOf('/'), f.lastIndexOf(':')) + 1) }
            } else {
                val r = model.client.call(serverId, Procedure.system("ListDirectory"), listDirectoryRequest { this.path = path }, ListDirectoryResponse::parseFrom)
                here = r.path
                parent = r.parent
                var list = r.entriesList.filter { it.isDir }.map { it.path to it.name }
                if (path.isEmpty()) list = list + remotes(model, serverId)
                folders = list
            }
        }
    }
    val sheet = rememberModalBottomSheetState(skipPartiallyExpanded = true)
    ModalBottomSheet(onDismiss, sheetState = sheet, containerColor = theme.colors.surfaceRaised) {
        // As tall at every folder, rather than growing and shrinking with it.
        LazyColumn(Modifier.fillMaxWidth().fillMaxHeight(0.9f), contentPadding = PaddingValues(horizontal = theme.layout.margin, vertical = MavioSpacing.s8)) {
            item {
                Row(Modifier.padding(bottom = MavioSpacing.s12), verticalAlignment = Alignment.CenterVertically) {
                    Column(Modifier.weight(1f)) {
                        Text(stringResource(R.string.libraries_browse), style = theme.type.title2.text)
                        Text(
                            here.ifEmpty { stringResource(R.string.libraries_roots) },
                            style = theme.type.callout.text,
                            color = theme.colors.textSecondary,
                            maxLines = 2,
                            overflow = TextOverflow.Ellipsis,
                        )
                    }
                    MavioButton(
                        stringResource(R.string.libraries_choose_folder),
                        {
                            choose(here)
                            onDismiss()
                        },
                        enabled = here.isNotEmpty() && here !in chosen,
                    )
                }
            }
            if (here.isNotEmpty()) {
                item { FolderRow(stringResource(R.string.libraries_parent_folder), R.drawable.ic_back) { path = parent } }
            }
            folders?.let { list ->
                items(list.size) { i -> FolderRow(list[i].second, R.drawable.ic_folder) { path = list[i].first } }
            }
            item {
                when {
                    failure != null -> Note(failure!!, failed = true)
                    folders?.isEmpty() == true -> Note(stringResource(R.string.libraries_no_folders))
                    folders == null -> Note(stringResource(R.string.state_loading))
                }
            }
        }
    }
}

@Composable
private fun FolderRow(name: String, @DrawableRes icon: Int, onClick: () -> Unit) {
    Row(
        Modifier.fillMaxWidth().clickable(onClick = onClick).heightIn(min = 48.dp).padding(horizontal = 4.dp),
        horizontalArrangement = Arrangement.spacedBy(MavioSpacing.s12),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        MavioIcon(icon, size = 20.dp, tint = theme.colors.textSecondary)
        Text(name, style = theme.type.body.text, maxLines = 1, overflow = TextOverflow.Ellipsis)
    }
}

/** The roots of the rclone remotes, when the server reaches rclone. */
private suspend fun remotes(model: AppModel, serverId: String): List<Pair<String, String>> = runCatching {
    val status = model.client.call(serverId, Procedure.storage("GetStatus"), getStatusRequest {}, GetStatusResponse::parseFrom)
    if (!status.available) return@runCatching emptyList()
    model.client.call(serverId, Procedure.storage("ListRemotes"), listRemotesRequest {}, ListRemotesResponse::parseFrom)
        .remotesList.map { "rclone:${it.name}:" to "rclone:${it.name}:" }
}.getOrDefault(emptyList())
