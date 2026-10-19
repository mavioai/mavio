package dev.mavio.ui

import android.util.Base64
import android.util.LruCache
import androidx.compose.foundation.layout.BoxScope
import androidx.compose.runtime.Composable
import androidx.compose.runtime.Stable
import androidx.compose.runtime.State
import androidx.compose.runtime.compositionLocalOf
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.produceState
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.unit.Dp
import androidx.compose.ui.unit.dp
import com.google.protobuf.MessageLite
import dev.mavio.core.AppModel
import dev.mavio.core.call
import java.io.Serializable
import java.util.Locale
import kotlinx.coroutines.CancellationException
import kotlinx.coroutines.async
import kotlinx.coroutines.awaitAll
import kotlinx.coroutines.coroutineScope
import mavio.acquisition.v1.Discover.Catalog
import mavio.acquisition.v1.Discover.DiscoveredTitle
import mavio.acquisition.v1.Discover.ListCatalogsResponse
import mavio.acquisition.v1.Discover.MediaStatus
import mavio.acquisition.v1.Profile.MediaKind
import mavio.acquisition.v1.discoverRequest
import mavio.acquisition.v1.listCatalogsRequest
import mavio.plugin.v1.Discovery.CatalogKind

/** The server Discover asks, the first signed-in server whose discovery providers offer catalogs, with its catalogs. */
data class Discovery(val serverId: String, val catalogs: List<Catalog>)

/**
 * Discover's calls (docs/clients.md §6.3.2), their answers kept a while so
 * that a shelf scrolled back to or a page gone back to shows at once, and
 * the server that discovers once the servers have answered.
 */
@Stable
class DiscoverState(private val model: AppModel) {
    /** The server that discovers, or null while none does or the servers have not answered. */
    var discovery by mutableStateOf<Discovery?>(null)
        private set

    private val cache = LruCache<String, MessageLite>(CACHE_SIZE)

    private fun key(serverId: String, method: String, request: MessageLite) =
        "$serverId/$method/" + Base64.encodeToString(request.toByteArray(), Base64.NO_WRAP)

    /** A DiscoverService call's answer kept from before, if any. */
    @Suppress("UNCHECKED_CAST")
    fun <T : MessageLite> peek(serverId: String, method: String, request: MessageLite): T? = cache.get(key(serverId, method, request)) as T?

    /** Forgets a call's answer kept from before, so that it is asked again. */
    fun forget(serverId: String, method: String, request: MessageLite) {
        cache.remove(key(serverId, method, request))
    }

    /** Forgets every answer of a server kept from before, so that a page pulled to refresh asks again. */
    fun forget(serverId: String) {
        cache.snapshot().keys.filter { it.startsWith("$serverId/") }.forEach(cache::remove)
    }

    /** Calls a method of DiscoverService, answering from what was kept when it can. */
    suspend fun <T : MessageLite> call(serverId: String, method: String, request: MessageLite, parse: (ByteArray) -> T): T {
        peek<T>(serverId, method, request)?.let { return it }
        return model.client.call(serverId, "/mavio.acquisition.v1.DiscoverService/$method", request, parse)
            .also { cache.put(key(serverId, method, request), it) }
    }

    /** Asks the servers for their catalogs, the first with any discovering. */
    suspend fun load(serverIds: List<String>) {
        val found = coroutineScope {
            serverIds.map { id ->
                async {
                    runCatching {
                        call(id, "ListCatalogs", listCatalogsRequest { language = discoverLanguage }, ListCatalogsResponse::parseFrom)
                    }.getOrNull()?.catalogsList.orEmpty()
                }
            }.awaitAll()
        }
        discovery = serverIds.zip(found).firstOrNull { it.second.isNotEmpty() }?.let { Discovery(it.first, it.second) }
    }

    private companion object {
        const val CACHE_SIZE = 200
    }
}

/** Discover's calls and its server. */
val LocalDiscover = compositionLocalOf<DiscoverState> { error("no discover") }

/** The language Discover asks in, the UI's (ISO 639-1). */
val discoverLanguage: String get() = Locale.getDefault().language

/**
 * A DiscoverService call's answer, null while it is on its way, at once
 * when it was kept from before; asked again as [attempt] changes, keeping
 * the answer shown until the new one comes. Each answer ends [reload]'s
 * pull, whose attempt it then asks by.
 */
@Composable
fun <T : MessageLite> discovered(
    serverId: String,
    method: String,
    request: MessageLite,
    parse: (ByteArray) -> T,
    attempt: Int = 0,
    reload: Reload? = null,
): State<Result<T>?> {
    val discover = LocalDiscover.current
    val asked = reload?.attempt ?: attempt
    val initial = remember(serverId, method, request, asked) { discover.peek<T>(serverId, method, request)?.let { Result.success(it) } }
    // The call last asked, whose answer stays shown while it is asked again.
    val last = remember { arrayOfNulls<Any>(1) }
    return produceState(initial, serverId, method, request, asked) {
        val call = Triple(serverId, method, request)
        // The state outlives its keys: a new request starts from its own answer, not the last one's,
        // and a failure gives way to the spinner as it is asked again.
        if (initial != null || last[0] != call || value?.isSuccess != true) value = initial
        last[0] = call
        if (initial?.isSuccess != true) {
            value = try {
                Result.success(discover.call(serverId, method, request, parse))
            } catch (e: CancellationException) {
                throw e
            } catch (e: Exception) {
                Result.failure(e)
            }
        }
        reload?.done()
    }
}

/**
 * Pull to refresh of a Discover page read again by [reload], which first
 * forgets the server's answers kept from before.
 */
@Composable
fun DiscoverPullToRefresh(serverId: String, reload: Reload, modifier: Modifier = Modifier, top: Dp = 0.dp, content: @Composable BoxScope.() -> Unit) {
    val discover = LocalDiscover.current
    PullToRefresh(
        reload.refreshing,
        onRefresh = {
            discover.forget(serverId)
            reload.pull()
        },
        modifier,
        top,
        content = content,
    )
}

/** A filter value chosen by its provider's ID, with its name to show. */
data class NamedValue(val id: String, val name: String) : Serializable

/** How a catalog's titles are filtered and sorted; unset fields do not filter. */
data class CatalogFilters(
    val sort: Int = 0,
    val ascending: Boolean = false,
    val genres: List<String> = emptyList(),
    /** Production companies, of movies. */
    val studios: List<NamedValue> = emptyList(),
    /** Television networks, of series. */
    val networks: List<NamedValue> = emptyList(),
    val keywords: List<NamedValue> = emptyList(),
    val watchProviders: List<String> = emptyList(),
    /** ISO 3166-1 country of the watch providers. */
    val watchRegion: String = "",
    val originalLanguage: String = "",
    /** Years of release, inclusive; 0 when unset. */
    val yearFrom: Int = 0,
    val yearTo: Int = 0,
    /** Minutes; 0 when unset. */
    val runtimeMin: Int = 0,
    val runtimeMax: Int = 0,
    /** 0–10. */
    val ratingMin: Int = 0,
    val votesMin: Int = 0,
) : Serializable {
    /** How many filters are set, the sort aside. */
    val count: Int
        get() = listOf(
            genres.isNotEmpty(),
            studios.isNotEmpty(),
            networks.isNotEmpty(),
            keywords.isNotEmpty(),
            watchProviders.isNotEmpty(),
            originalLanguage.isNotEmpty(),
            yearFrom > 0 || yearTo > 0,
            runtimeMin > 0 || runtimeMax > 0,
            ratingMin > 0,
            votesMin > 0,
        ).count { it }
}

/** A catalog's page of titles, filtered and sorted; tmdbId names the title of recommendations and similar titles. */
fun catalogRequest(catalog: String, mediaKind: Int, tmdbId: Int, f: CatalogFilters, page: Int) = discoverRequest {
    this.catalog = catalog
    mediaKindValue = mediaKind
    this.tmdbId = tmdbId
    language = discoverLanguage
    this.page = page
    sortValue = f.sort
    ascending = f.ascending
    genres += f.genres
    studios += f.studios.map { it.id }
    networks += f.networks.map { it.id }
    keywords += f.keywords.map { it.id }
    watchProviders += f.watchProviders
    if (f.watchProviders.isNotEmpty()) watchRegion = f.watchRegion
    // Only the filters chosen are sent, as the web UI's: a field set is a filter even at zero.
    if (f.originalLanguage.isNotEmpty()) originalLanguage = f.originalLanguage
    if (f.yearFrom > 0) releasedFrom = "${f.yearFrom}-01-01"
    if (f.yearTo > 0) releasedTo = "${f.yearTo}-12-31"
    if (f.runtimeMin > 0) runtimeMin = f.runtimeMin
    if (f.runtimeMax > 0) runtimeMax = f.runtimeMax
    if (f.ratingMin > 0) ratingMin = f.ratingMin.toDouble()
    if (f.votesMin > 0) votesMin = f.votesMin
}

/** A row of Discover: a catalog, of one kind of title unless it mixes them. */
data class CatalogRow(val catalog: Catalog, val mediaKind: MediaKind)

/** The catalogs Discover shows, in its order; those that need a title to start from are left out. */
private val rowKinds = listOf(
    CatalogKind.CATALOG_KIND_TRENDING,
    CatalogKind.CATALOG_KIND_POPULAR,
    CatalogKind.CATALOG_KIND_UPCOMING,
    CatalogKind.CATALOG_KIND_TOP_RATED,
    CatalogKind.CATALOG_KIND_LIST,
)

/** The catalogs' rows: one, or one per kind of title a catalog lists apart. */
fun rowsOf(catalogs: List<Catalog>): List<CatalogRow> = rowKinds.flatMap { kind ->
    catalogs.filter { it.kind == kind }.flatMap { c ->
        if (c.mixed || c.mediaKindsCount < 2) {
            listOf(CatalogRow(c, if (c.mixed) MediaKind.MEDIA_KIND_UNSPECIFIED else c.mediaKindsList.firstOrNull() ?: MediaKind.MEDIA_KIND_UNSPECIFIED))
        } else {
            listOf(MediaKind.MEDIA_KIND_MOVIE, MediaKind.MEDIA_KIND_SERIES).filter { it in c.mediaKindsList }.map { CatalogRow(c, it) }
        }
    }
}

/** The name of a row of Discover, by its catalog's kind and its kind of title. */
@Composable
fun rowTitle(row: CatalogRow): String {
    val series = row.mediaKind == MediaKind.MEDIA_KIND_SERIES
    return when (row.catalog.kind) {
        CatalogKind.CATALOG_KIND_TRENDING ->
            if (row.mediaKind == MediaKind.MEDIA_KIND_UNSPECIFIED) stringResource(R.string.discover_trending) else row.catalog.name
        CatalogKind.CATALOG_KIND_POPULAR ->
            stringResource(if (series) R.string.discover_popular_series else R.string.discover_popular_movies)
        CatalogKind.CATALOG_KIND_UPCOMING ->
            stringResource(if (series) R.string.discover_upcoming_series else R.string.discover_upcoming_movies)
        CatalogKind.CATALOG_KIND_TOP_RATED ->
            stringResource(if (series) R.string.discover_top_rated_series else R.string.discover_top_rated_movies)
        else -> row.catalog.name
    }
}

/** The same TMDB image at a width, rather than its original size. */
fun sized(url: String, width: Int): String? =
    url.takeIf { it.isNotEmpty() }?.replace("image.tmdb.org/t/p/original/", "image.tmdb.org/t/p/w$width/")

/** A title's TMDB ID. */
val DiscoveredTitle.tmdbId: Int get() = externalIdsMap["tmdb"]?.toIntOrNull() ?: 0

/** The status a title's card and header show, if any. */
fun statusLabel(status: MediaStatus): Int? = when (status) {
    MediaStatus.MEDIA_STATUS_AVAILABLE -> R.string.discover_available
    MediaStatus.MEDIA_STATUS_PARTIALLY_AVAILABLE -> R.string.discover_partially_available
    MediaStatus.MEDIA_STATUS_REQUESTED -> R.string.discover_requested
    MediaStatus.MEDIA_STATUS_WANTED -> R.string.discover_processing
    MediaStatus.MEDIA_STATUS_BLOCKLISTED -> R.string.discover_blocklisted
    else -> null
}

/** The key of a discovered title in a list. */
fun titleKey(t: DiscoveredTitle) = "${t.mediaKindValue}/${t.externalIdsMap["tmdb"] ?: t.name}"

/** Whether a status is of a title the server has, at least in part. */
fun MediaStatus.available(): Boolean =
    this == MediaStatus.MEDIA_STATUS_AVAILABLE || this == MediaStatus.MEDIA_STATUS_PARTIALLY_AVAILABLE

/**
 * What the server has of a collection, as Seerr marks it: available when
 * it has every movie, partially available when some, requested when every
 * other movie is requested, blocklisted when every movie is.
 */
fun collectionStatus(parts: List<DiscoveredTitle>): MediaStatus = when {
    parts.isEmpty() -> MediaStatus.MEDIA_STATUS_UNSPECIFIED
    parts.all { it.status == MediaStatus.MEDIA_STATUS_BLOCKLISTED } -> MediaStatus.MEDIA_STATUS_BLOCKLISTED
    parts.all { it.status == MediaStatus.MEDIA_STATUS_AVAILABLE } -> MediaStatus.MEDIA_STATUS_AVAILABLE
    parts.any { it.status.available() } -> MediaStatus.MEDIA_STATUS_PARTIALLY_AVAILABLE
    parts.all { it.status == MediaStatus.MEDIA_STATUS_REQUESTED || it.status == MediaStatus.MEDIA_STATUS_WANTED } -> MediaStatus.MEDIA_STATUS_REQUESTED
    else -> MediaStatus.MEDIA_STATUS_UNSPECIFIED
}
