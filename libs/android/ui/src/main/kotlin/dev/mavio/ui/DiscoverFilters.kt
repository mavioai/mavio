package dev.mavio.ui

import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.ColumnScope
import androidx.compose.foundation.layout.ExperimentalLayoutApi
import androidx.compose.foundation.layout.FlowRow
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.lazy.LazyListScope
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.draw.shadow
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.input.KeyboardType
import androidx.compose.ui.unit.dp
import dev.mavio.core.choices
import dev.mavio.design.MavioSpacing
import java.time.Year
import kotlinx.coroutines.delay
import mavio.acquisition.v1.Discover.Catalog
import mavio.acquisition.v1.Discover.ListFilterValuesResponse
import mavio.acquisition.v1.Profile.MediaKind
import mavio.acquisition.v1.listFilterValuesRequest
import mavio.plugin.v1.Discovery.DiscoverFilter
import mavio.plugin.v1.Discovery.DiscoverSort
import mavio.plugin.v1.Discovery.FilterValue

/** The country of the streaming services until another is chosen, the one with the most of them. */
private const val DEFAULT_REGION = "US"

/** How many values make a list long. */
private const val LONG_LIST = 30

/** The name of a catalog's order. */
private val DiscoverSort.label: Int
    get() = when (this) {
        DiscoverSort.DISCOVER_SORT_POPULARITY -> R.string.discover_sort_popularity
        DiscoverSort.DISCOVER_SORT_RELEASE_DATE -> R.string.discover_sort_release_date
        DiscoverSort.DISCOVER_SORT_RATING -> R.string.discover_sort_rating
        DiscoverSort.DISCOVER_SORT_VOTE_COUNT -> R.string.discover_sort_votes
        DiscoverSort.DISCOVER_SORT_TITLE -> R.string.discover_sort_title
        else -> R.string.discover_sort_default
    }

/** Whether a catalog takes filters or sorts: recommendations and similar titles take none. */
fun Catalog.filterable(tmdbId: Int): Boolean = tmdbId == 0 && (filtersCount > 0 || sortsCount > 0)

/** The values a catalog's filter takes, those matching a query for those found by name; null while they load. */
@Composable
private fun filterValues(serverId: String, catalog: String, filter: DiscoverFilter, mediaKind: MediaKind, query: String = "", region: String = ""): List<FilterValue>? {
    val request = remember(catalog, filter, mediaKind, query, region) {
        listFilterValuesRequest {
            this.catalog = catalog
            this.filter = filter
            this.mediaKind = mediaKind
            this.query = query
            language = discoverLanguage
            this.region = region
        }
    }
    val result by discovered(serverId, "ListFilterValues", request, ListFilterValuesResponse::parseFrom)
    return result?.let { r -> r.getOrNull()?.valuesList.orEmpty() }
}

/**
 * A value to pick, filled with the accent once picked, after its logo if
 * it has one; on a television white with black text while it has focus.
 */
@Composable
private fun ToggleChip(label: String, on: Boolean, image: String? = null, onChange: (Boolean) -> Unit) {
    val colors = theme.colors
    val tv = LocalTelevision.current
    val state = remember { Focusable() }
    val focused = tv && state.focused()
    val ink = when {
        focused -> Color.Black
        on -> colors.onAccent
        else -> colors.textPrimary
    }
    Row(
        Modifier
            .then(if (tv) Modifier.shakes() else Modifier)
            .heightIn(min = if (tv) 48.dp else 32.dp)
            .then(if (focused) Modifier.shadow(12.dp, CircleShape, ambientColor = colors.tvFocusShadow, spotColor = colors.tvFocusShadow) else Modifier)
            .clip(CircleShape)
            .background(
                when {
                    focused -> Color.White
                    on -> colors.accent
                    else -> colors.fill
                },
            )
            .then(if (tv) Modifier.selectable(state) { onChange(!on) } else Modifier.clickable { onChange(!on) })
            .padding(start = if (image != null) 4.dp else if (tv) 18.dp else 12.dp, end = if (tv) 18.dp else 12.dp),
        horizontalArrangement = Arrangement.spacedBy(6.dp),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        image?.let { Artwork(it, Modifier.size(if (tv) 40.dp else 24.dp).clip(CircleShape)) }
        Text(
            label,
            style = (if (tv) theme.type.callout else theme.type.footnote).text,
            fontWeight = if (on) FontWeight.SemiBold else null,
            color = ink,
        )
        if (on && image == null) MavioIcon(R.drawable.ic_check, size = if (tv) 18.dp else 14.dp, tint = ink)
    }
}

/** A section of the filters: its heading, then what it chooses. */
@Composable
private fun Section(title: String, content: @Composable () -> Unit) {
    Column(Modifier.fillMaxWidth().padding(vertical = MavioSpacing.s8), verticalArrangement = Arrangement.spacedBy(MavioSpacing.s8)) {
        Text(title, style = theme.type.headline.text)
        content()
    }
}

/** The card of a section's menus; unclipped on a television, where a focused row's shadow reaches past it. */
@Composable
private fun Choices(content: @Composable ColumnScope.() -> Unit) {
    val shape = RoundedCornerShape(10.dp)
    Column(
        Modifier.then(if (LocalTelevision.current) Modifier else Modifier.clip(shape)).background(theme.colors.fill, shape),
        content = content,
    )
}

/** The chips of values, wrapping. */
@OptIn(ExperimentalLayoutApi::class)
@Composable
private fun Chips(modifier: Modifier = Modifier, content: @Composable () -> Unit) {
    FlowRow(
        modifier,
        horizontalArrangement = Arrangement.spacedBy(MavioSpacing.s8),
        verticalArrangement = Arrangement.spacedBy(MavioSpacing.s8),
    ) { content() }
}

/**
 * Values listed whole, as genres and streaming services are, to pick
 * from. A long list scrolls in a box of its own on a phone; on a
 * television, where focus would cross it all, it shows its first values
 * and those picked until Show More.
 */
@Composable
private fun ListedValues(values: List<FilterValue>?, chosen: List<String>, images: Boolean = false, onChange: (List<String>) -> Unit) {
    val all = values.orEmpty()
    val long = all.size > LONG_LIST
    val tv = LocalTelevision.current
    var more by remember { mutableStateOf(false) }
    val shown = if (long && tv && !more) all.filterIndexed { i, v -> i < LONG_LIST || v.id in chosen } else all
    Chips(if (long && !tv) Modifier.heightIn(max = 160.dp).verticalScroll(rememberScrollState()) else Modifier) {
        shown.forEach { v ->
            ToggleChip(v.name, v.id in chosen, if (images) v.imageUrl.ifEmpty { null } else null) { on ->
                onChange(if (on) chosen + v.id else chosen - v.id)
            }
        }
        if (long && tv) {
            ToggleChip(stringResource(if (more) R.string.action_show_less else R.string.discover_show_more), on = false) { more = !more }
        }
    }
}

/** Values found by name, as studios and keywords are: the chosen ones, a field, and what it finds. */
@Composable
private fun SearchedValues(
    serverId: String,
    catalog: String,
    filter: DiscoverFilter,
    mediaKind: MediaKind,
    title: String,
    chosen: List<NamedValue>,
    searchable: Boolean = true,
    onChange: (List<NamedValue>) -> Unit,
) {
    var text by remember { mutableStateOf("") }
    var query by remember { mutableStateOf("") }
    LaunchedEffect(text) {
        delay(300)
        query = text.trim()
    }
    Section(title) {
        if (chosen.isNotEmpty()) {
            Chips { chosen.forEach { v -> ToggleChip(v.name, on = true) { onChange(chosen - v) } } }
        }
        if (searchable) {
            SettingsField(text, { text = it }, stringResource(R.string.discover_search_values, title))
        }
        if (query.isNotEmpty()) {
            val found = filterValues(serverId, catalog, filter, mediaKind, query = query)
            val ids = chosen.map { it.id }.toSet()
            Chips {
                found.orEmpty().filter { it.id !in ids }.take(20).forEach { v ->
                    ToggleChip(v.name, on = false) {
                        onChange(chosen + NamedValue(v.id, v.name))
                        text = ""
                    }
                }
            }
        }
    }
}

/** A range of two whole numbers, each empty when unset. */
@Composable
private fun RangeFields(from: Int, to: Int, onChange: (Int, Int) -> Unit) {
    Row(horizontalArrangement = Arrangement.spacedBy(MavioSpacing.s8), verticalAlignment = Alignment.CenterVertically) {
        SettingsField(
            if (from > 0) from.toString() else "",
            { onChange(it.filter(Char::isDigit).take(7).toIntOrNull() ?: 0, to) },
            stringResource(R.string.discover_from),
            Modifier.weight(1f),
            keyboard = KeyboardType.Number,
        )
        Text("–", color = theme.colors.textSecondary)
        SettingsField(
            if (to > 0) to.toString() else "",
            { onChange(from, it.filter(Char::isDigit).take(7).toIntOrNull() ?: 0) },
            stringResource(R.string.discover_to),
            Modifier.weight(1f),
            keyboard = KeyboardType.Number,
        )
    }
}

/**
 * The sections of a catalog's filters and sorts (docs/clients.md §6.3.28),
 * as the web UI's dialog, offering those the catalog takes; its values are
 * those of the catalog's kind of title, movies when it mixes them.
 * [localization] holds the server's languages and countries, null while
 * they load.
 */
fun LazyListScope.filterSections(
    serverId: String,
    catalog: Catalog,
    kind: MediaKind,
    f: CatalogFilters,
    localization: Pair<List<Pair<String, String>>, List<Pair<String, String>>>?,
    onChange: (CatalogFilters) -> Unit,
) {
    val takes = { x: DiscoverFilter -> x in catalog.filtersList }
    val mediaKind = if (kind == MediaKind.MEDIA_KIND_UNSPECIFIED) MediaKind.MEDIA_KIND_MOVIE else kind
    val region = f.watchRegion.ifEmpty { DEFAULT_REGION }
    if (catalog.sortsCount > 0) {
        item("sort") {
            Section(stringResource(R.string.discover_sort_by)) {
                Choices {
                    ChoiceRow(
                        stringResource(R.string.discover_sort_by),
                        (listOf(DiscoverSort.DISCOVER_SORT_UNSPECIFIED) + catalog.sortsList).map { it.number to stringResource(it.label) },
                        f.sort,
                    ) { onChange(f.copy(sort = it, ascending = if (it == 0) false else f.ascending)) }
                    if (f.sort != 0) {
                        ChoiceRow(
                            stringResource(R.string.discover_sort_by),
                            listOf(false to stringResource(R.string.discover_descending), true to stringResource(R.string.discover_ascending)),
                            f.ascending,
                        ) { onChange(f.copy(ascending = it)) }
                    }
                }
            }
        }
    }
    if (takes(DiscoverFilter.DISCOVER_FILTER_GENRE)) {
        item("genres") {
            val values = filterValues(serverId, catalog.id, DiscoverFilter.DISCOVER_FILTER_GENRE, mediaKind)
            if (values == null || values.isNotEmpty()) {
                Section(stringResource(R.string.discover_genres)) { ListedValues(values, f.genres) { onChange(f.copy(genres = it)) } }
            }
        }
    }
    if (takes(DiscoverFilter.DISCOVER_FILTER_STUDIO) && mediaKind == MediaKind.MEDIA_KIND_MOVIE) {
        item("studios") {
            SearchedValues(serverId, catalog.id, DiscoverFilter.DISCOVER_FILTER_STUDIO, mediaKind, stringResource(R.string.discover_studios), f.studios) {
                onChange(f.copy(studios = it))
            }
        }
    }
    if (mediaKind == MediaKind.MEDIA_KIND_SERIES && f.networks.isNotEmpty()) {
        item("networks") {
            SearchedValues(serverId, catalog.id, DiscoverFilter.DISCOVER_FILTER_NETWORK, mediaKind, stringResource(R.string.discover_networks), f.networks, searchable = false) {
                onChange(f.copy(networks = it))
            }
        }
    }
    if (takes(DiscoverFilter.DISCOVER_FILTER_KEYWORD)) {
        item("keywords") {
            SearchedValues(serverId, catalog.id, DiscoverFilter.DISCOVER_FILTER_KEYWORD, mediaKind, stringResource(R.string.discover_keywords), f.keywords) {
                onChange(f.copy(keywords = it))
            }
        }
    }
    if (takes(DiscoverFilter.DISCOVER_FILTER_WATCH_PROVIDER)) {
        item("providers") {
            Section(stringResource(R.string.discover_watch_providers)) {
                Choices {
                    ChoiceRow(
                        stringResource(R.string.discover_region),
                        choices(localization?.second.orEmpty(), region),
                        region,
                    ) { onChange(f.copy(watchRegion = it, watchProviders = emptyList())) }
                }
                val values = filterValues(serverId, catalog.id, DiscoverFilter.DISCOVER_FILTER_WATCH_PROVIDER, mediaKind, region = region)
                ListedValues(values, f.watchProviders, images = true) { onChange(f.copy(watchProviders = it, watchRegion = region)) }
            }
        }
    }
    if (takes(DiscoverFilter.DISCOVER_FILTER_ORIGINAL_LANGUAGE)) {
        item("language") {
            Section(stringResource(R.string.discover_original_language)) {
                Choices {
                    ChoiceRow(
                        stringResource(R.string.discover_original_language),
                        listOf("" to stringResource(R.string.discover_any)) + choices(localization?.first.orEmpty(), f.originalLanguage),
                        f.originalLanguage,
                    ) { onChange(f.copy(originalLanguage = it)) }
                }
            }
        }
    }
    if (takes(DiscoverFilter.DISCOVER_FILTER_RELEASE_DATE)) {
        item("released") {
            Section(stringResource(R.string.discover_released)) {
                val last = Year.now().value + 5
                RangeFields(f.yearFrom, f.yearTo) { a, b -> onChange(f.copy(yearFrom = a.coerceAtMost(last), yearTo = b.coerceAtMost(last))) }
            }
        }
    }
    if (takes(DiscoverFilter.DISCOVER_FILTER_RUNTIME)) {
        item("runtime") {
            Section(stringResource(R.string.discover_runtime)) {
                RangeFields(f.runtimeMin, f.runtimeMax) { a, b -> onChange(f.copy(runtimeMin = a.coerceAtMost(1000), runtimeMax = b.coerceAtMost(1000))) }
            }
        }
    }
    if (takes(DiscoverFilter.DISCOVER_FILTER_RATING)) {
        item("rating") {
            Section(stringResource(R.string.discover_rating_min)) {
                Chips {
                    (0..9).forEach { n ->
                        ToggleChip(if (n == 0) stringResource(R.string.discover_any) else n.toString(), f.ratingMin == n) { onChange(f.copy(ratingMin = n)) }
                    }
                }
            }
        }
    }
    if (takes(DiscoverFilter.DISCOVER_FILTER_VOTE_COUNT)) {
        item("votes") {
            Section(stringResource(R.string.discover_votes_min)) {
                SettingsField(
                    if (f.votesMin > 0) f.votesMin.toString() else "",
                    { onChange(f.copy(votesMin = (it.filter(Char::isDigit).take(7).toIntOrNull() ?: 0).coerceAtMost(1_000_000))) },
                    stringResource(R.string.discover_votes_min),
                    placeholder = stringResource(R.string.discover_any),
                    keyboard = KeyboardType.Number,
                )
            }
        }
    }
}
