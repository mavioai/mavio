package dev.mavio.app

import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.ExperimentalLayoutApi
import androidx.compose.foundation.layout.FlowRow
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.navigationBarsPadding
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.ModalBottomSheet
import androidx.compose.material3.Text
import androidx.compose.material3.rememberModalBottomSheetState
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.produceState
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.input.KeyboardType
import androidx.compose.ui.unit.dp
import dev.mavio.core.AppModel
import dev.mavio.design.MavioSpacing
import dev.mavio.ui.Artwork
import dev.mavio.ui.ButtonKind
import dev.mavio.ui.CatalogFilters
import dev.mavio.ui.MavioButton
import dev.mavio.ui.MavioIcon
import dev.mavio.ui.NamedValue
import dev.mavio.ui.R
import dev.mavio.ui.discoverLanguage
import dev.mavio.ui.discovered
import dev.mavio.ui.text
import dev.mavio.ui.theme
import kotlinx.coroutines.delay
import mavio.acquisition.v1.Discover.Catalog
import mavio.acquisition.v1.Discover.ListFilterValuesResponse
import mavio.acquisition.v1.Profile.MediaKind
import mavio.acquisition.v1.listFilterValuesRequest
import mavio.plugin.v1.Discovery.DiscoverFilter
import mavio.plugin.v1.Discovery.DiscoverSort
import mavio.plugin.v1.Discovery.FilterValue
import java.time.Year

/** The country of the streaming services until another is chosen, the one with the most of them. */
private const val DEFAULT_REGION = "US"

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

/** A value to pick, filled with the accent once picked, after its logo if it has one. */
@Composable
private fun ToggleChip(label: String, on: Boolean, image: String? = null, onChange: (Boolean) -> Unit) {
    val colors = theme.colors
    Row(
        Modifier
            .heightIn(min = 32.dp)
            .clip(CircleShape)
            .background(if (on) colors.accent else colors.fill)
            .clickable { onChange(!on) }
            .padding(start = if (image != null) 4.dp else 12.dp, end = 12.dp),
        horizontalArrangement = Arrangement.spacedBy(6.dp),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        image?.let { Artwork(it, Modifier.size(24.dp).clip(CircleShape)) }
        Text(
            label,
            style = theme.type.footnote.text,
            fontWeight = if (on) FontWeight.SemiBold else null,
            color = if (on) colors.onAccent else colors.textPrimary,
        )
        if (on && image == null) MavioIcon(R.drawable.ic_check, size = 14.dp, tint = colors.onAccent)
    }
}

/** A section of the sheet: its heading, then what it chooses. */
@Composable
private fun Section(title: String, content: @Composable () -> Unit) {
    Column(Modifier.fillMaxWidth().padding(vertical = MavioSpacing.s8), verticalArrangement = Arrangement.spacedBy(MavioSpacing.s8)) {
        Text(title, style = theme.type.headline.text)
        content()
    }
}

/** Values listed whole, as genres and streaming services are, to pick from. */
@OptIn(ExperimentalLayoutApi::class)
@Composable
private fun ListedValues(values: List<FilterValue>?, chosen: List<String>, images: Boolean = false, onChange: (List<String>) -> Unit) {
    // A long list, as streaming services are, scrolls in a box of its own.
    val long = values.orEmpty().size > 30
    FlowRow(
        if (long) Modifier.heightIn(max = 160.dp).verticalScroll(rememberScrollState()) else Modifier,
        horizontalArrangement = Arrangement.spacedBy(MavioSpacing.s8),
        verticalArrangement = Arrangement.spacedBy(MavioSpacing.s8),
    ) {
        values.orEmpty().forEach { v ->
            ToggleChip(v.name, v.id in chosen, if (images) v.imageUrl.ifEmpty { null } else null) { on ->
                onChange(if (on) chosen + v.id else chosen - v.id)
            }
        }
    }
}

/** Values found by name, as studios and keywords are: the chosen ones, a field, and what it finds. */
@OptIn(ExperimentalLayoutApi::class)
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
            FlowRow(horizontalArrangement = Arrangement.spacedBy(MavioSpacing.s8), verticalArrangement = Arrangement.spacedBy(MavioSpacing.s8)) {
                chosen.forEach { v -> ToggleChip(v.name, on = true) { onChange(chosen - v) } }
            }
        }
        if (searchable) {
            SettingsField(text, { text = it }, stringResource(R.string.discover_search_values, title))
        }
        if (query.isNotEmpty()) {
            val found = filterValues(serverId, catalog, filter, mediaKind, query = query)
            val ids = chosen.map { it.id }.toSet()
            FlowRow(horizontalArrangement = Arrangement.spacedBy(MavioSpacing.s8), verticalArrangement = Arrangement.spacedBy(MavioSpacing.s8)) {
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
 * The sheet of a catalog's filters and sorts (docs/clients.md §6.3.28),
 * as the web UI's dialog, offering those the catalog takes; its values are
 * those of the catalog's kind of title, movies when it mixes them.
 */
@OptIn(ExperimentalMaterial3Api::class, ExperimentalLayoutApi::class)
@Composable
internal fun FiltersSheet(
    model: AppModel,
    serverId: String,
    catalog: Catalog,
    kind: MediaKind,
    initial: CatalogFilters,
    onDismiss: () -> Unit,
    onApply: (CatalogFilters) -> Unit,
) {
    var f by remember { mutableStateOf(initial) }
    val takes = { x: DiscoverFilter -> x in catalog.filtersList }
    val mediaKind = if (kind == MediaKind.MEDIA_KIND_UNSPECIFIED) MediaKind.MEDIA_KIND_MOVIE else kind
    val localization by produceState<Pair<List<Pair<String, String>>, List<Pair<String, String>>>?>(null, serverId) {
        value = localization(model, serverId)
    }
    val region = f.watchRegion.ifEmpty { DEFAULT_REGION }
    val sheet = rememberModalBottomSheetState(skipPartiallyExpanded = true)
    ModalBottomSheet(onDismiss, sheetState = sheet, containerColor = theme.colors.surfaceRaised) {
        LazyColumn(
            Modifier.fillMaxWidth(),
            contentPadding = PaddingValues(horizontal = theme.layout.margin),
        ) {
            item {
                Text(stringResource(R.string.discover_filters), Modifier.padding(bottom = MavioSpacing.s8), style = theme.type.title2.text)
            }
            if (catalog.sortsCount > 0) {
                item {
                    Section(stringResource(R.string.discover_sort_by)) {
                        Column(Modifier.clip(RoundedCornerShape(10.dp)).background(theme.colors.fill)) {
                            ChoiceRow(
                                stringResource(R.string.discover_sort_by),
                                (listOf(DiscoverSort.DISCOVER_SORT_UNSPECIFIED) + catalog.sortsList).map { it.number to stringResource(it.label) },
                                f.sort,
                            ) { f = f.copy(sort = it, ascending = if (it == 0) false else f.ascending) }
                            if (f.sort != 0) {
                                ChoiceRow(
                                    stringResource(R.string.discover_sort_by),
                                    listOf(false to stringResource(R.string.discover_descending), true to stringResource(R.string.discover_ascending)),
                                    f.ascending,
                                ) { f = f.copy(ascending = it) }
                            }
                        }
                    }
                }
            }
            if (takes(DiscoverFilter.DISCOVER_FILTER_GENRE)) {
                item {
                    val values = filterValues(serverId, catalog.id, DiscoverFilter.DISCOVER_FILTER_GENRE, mediaKind)
                    if (values == null || values.isNotEmpty()) {
                        Section(stringResource(R.string.discover_genres)) { ListedValues(values, f.genres) { f = f.copy(genres = it) } }
                    }
                }
            }
            if (takes(DiscoverFilter.DISCOVER_FILTER_STUDIO) && mediaKind == MediaKind.MEDIA_KIND_MOVIE) {
                item {
                    SearchedValues(serverId, catalog.id, DiscoverFilter.DISCOVER_FILTER_STUDIO, mediaKind, stringResource(R.string.discover_studios), f.studios) {
                        f = f.copy(studios = it)
                    }
                }
            }
            if (mediaKind == MediaKind.MEDIA_KIND_SERIES && f.networks.isNotEmpty()) {
                item {
                    SearchedValues(serverId, catalog.id, DiscoverFilter.DISCOVER_FILTER_NETWORK, mediaKind, stringResource(R.string.discover_networks), f.networks, searchable = false) {
                        f = f.copy(networks = it)
                    }
                }
            }
            if (takes(DiscoverFilter.DISCOVER_FILTER_KEYWORD)) {
                item {
                    SearchedValues(serverId, catalog.id, DiscoverFilter.DISCOVER_FILTER_KEYWORD, mediaKind, stringResource(R.string.discover_keywords), f.keywords) {
                        f = f.copy(keywords = it)
                    }
                }
            }
            if (takes(DiscoverFilter.DISCOVER_FILTER_WATCH_PROVIDER)) {
                item {
                    Section(stringResource(R.string.discover_watch_providers)) {
                        Column(Modifier.clip(RoundedCornerShape(10.dp)).background(theme.colors.fill)) {
                            ChoiceRow(
                                stringResource(R.string.discover_region),
                                choices(localization?.second.orEmpty(), region),
                                region,
                            ) { f = f.copy(watchRegion = it, watchProviders = emptyList()) }
                        }
                        val values = filterValues(serverId, catalog.id, DiscoverFilter.DISCOVER_FILTER_WATCH_PROVIDER, mediaKind, region = region)
                        ListedValues(values, f.watchProviders, images = true) { f = f.copy(watchProviders = it, watchRegion = region) }
                    }
                }
            }
            if (takes(DiscoverFilter.DISCOVER_FILTER_ORIGINAL_LANGUAGE)) {
                item {
                    Section(stringResource(R.string.discover_original_language)) {
                        Column(Modifier.clip(RoundedCornerShape(10.dp)).background(theme.colors.fill)) {
                            ChoiceRow(
                                stringResource(R.string.discover_original_language),
                                listOf("" to stringResource(R.string.discover_any)) + choices(localization?.first.orEmpty(), f.originalLanguage),
                                f.originalLanguage,
                            ) { f = f.copy(originalLanguage = it) }
                        }
                    }
                }
            }
            if (takes(DiscoverFilter.DISCOVER_FILTER_RELEASE_DATE)) {
                item {
                    Section(stringResource(R.string.discover_released)) {
                        val last = Year.now().value + 5
                        RangeFields(f.yearFrom, f.yearTo) { a, b -> f = f.copy(yearFrom = a.coerceAtMost(last), yearTo = b.coerceAtMost(last)) }
                    }
                }
            }
            if (takes(DiscoverFilter.DISCOVER_FILTER_RUNTIME)) {
                item {
                    Section(stringResource(R.string.discover_runtime)) {
                        RangeFields(f.runtimeMin, f.runtimeMax) { a, b -> f = f.copy(runtimeMin = a.coerceAtMost(1000), runtimeMax = b.coerceAtMost(1000)) }
                    }
                }
            }
            if (takes(DiscoverFilter.DISCOVER_FILTER_RATING)) {
                item {
                    Section(stringResource(R.string.discover_rating_min)) {
                        FlowRow(horizontalArrangement = Arrangement.spacedBy(MavioSpacing.s8), verticalArrangement = Arrangement.spacedBy(MavioSpacing.s8)) {
                            (0..9).forEach { n ->
                                ToggleChip(if (n == 0) stringResource(R.string.discover_any) else n.toString(), f.ratingMin == n) { f = f.copy(ratingMin = n) }
                            }
                        }
                    }
                }
            }
            if (takes(DiscoverFilter.DISCOVER_FILTER_VOTE_COUNT)) {
                item {
                    Section(stringResource(R.string.discover_votes_min)) {
                        SettingsField(
                            if (f.votesMin > 0) f.votesMin.toString() else "",
                            { f = f.copy(votesMin = (it.filter(Char::isDigit).take(7).toIntOrNull() ?: 0).coerceAtMost(1_000_000)) },
                            stringResource(R.string.discover_votes_min),
                            placeholder = stringResource(R.string.discover_any),
                            keyboard = KeyboardType.Number,
                        )
                    }
                }
            }
            item {
                Row(
                    Modifier.fillMaxWidth().padding(vertical = MavioSpacing.s16).navigationBarsPadding(),
                    horizontalArrangement = Arrangement.spacedBy(MavioSpacing.s8),
                ) {
                    MavioButton(stringResource(R.string.discover_reset), { f = CatalogFilters() }, Modifier.weight(1f), kind = ButtonKind.Secondary, large = true)
                    MavioButton(stringResource(R.string.discover_apply), { onApply(f) }, Modifier.weight(1f), large = true)
                }
            }
            item { androidx.compose.foundation.layout.Spacer(Modifier.height(MavioSpacing.s8)) }
        }
    }
}
