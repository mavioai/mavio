package dev.mavio.app.tv

import androidx.compose.animation.Crossfade
import androidx.compose.animation.core.tween
import androidx.compose.foundation.ExperimentalFoundationApi
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.gestures.LocalBringIntoViewSpec
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.aspectRatio
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.layout.widthIn
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.LazyListScope
import androidx.compose.foundation.lazy.LazyListState
import androidx.compose.foundation.lazy.LazyRow
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.lazy.itemsIndexed
import androidx.compose.foundation.lazy.rememberLazyListState
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.CompositionLocalProvider
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableIntStateOf
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.alpha
import androidx.compose.ui.draw.clip
import androidx.compose.ui.focus.FocusRequester
import androidx.compose.ui.focus.focusRequester
import androidx.compose.ui.focus.onFocusChanged
import androidx.compose.ui.graphics.Brush
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.platform.LocalDensity
import androidx.compose.ui.platform.LocalWindowInfo
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import dev.mavio.core.AppModel
import dev.mavio.core.Card
import dev.mavio.core.CardKind
import dev.mavio.core.Row as HomeRow
import dev.mavio.core.RowContent
import dev.mavio.core.RowKind
import dev.mavio.core.TitleCard
import dev.mavio.design.MavioRadius
import dev.mavio.design.MavioSpacing
import dev.mavio.ui.Artwork
import dev.mavio.ui.CardPlaceholder
import dev.mavio.ui.CardShape
import dev.mavio.ui.Focusable
import dev.mavio.ui.Logo
import dev.mavio.ui.MavioIcon
import dev.mavio.ui.ProgressBar
import dev.mavio.ui.R
import dev.mavio.ui.StatusView
import dev.mavio.ui.TvButton
import dev.mavio.ui.TvScrolling
import dev.mavio.ui.focusArtwork
import dev.mavio.ui.focusCard
import dev.mavio.ui.focused
import dev.mavio.ui.heroKinds
import dev.mavio.ui.itemIcon
import dev.mavio.ui.minutesText
import dev.mavio.ui.rowTitle
import dev.mavio.ui.selectable
import dev.mavio.ui.shakes
import dev.mavio.ui.shapeOf
import dev.mavio.ui.subtitleOf
import dev.mavio.ui.text
import dev.mavio.ui.theme
import kotlinx.coroutines.delay
import kotlinx.coroutines.launch
import mavio.acquisition.v1.Profile.MediaKind

/** A title the hero shows, and the line above it. */
private data class HeroEntry(val card: Card, val eyebrow: String)

/** How long each of the hero's titles shows before the next, in milliseconds. */
private const val HERO_INTERVAL = 9000L

/** What Home's cards and buttons open: an item's details, or its playback. */
class HomeActions(val open: (Card) -> Unit, val play: (Card) -> Unit)

/**
 * Home on a television (docs/tv.md §6.1): a hero 78 % of the screen's
 * height behind the navigation bar, then a row for each the servers
 * return. Focus starts on the hero's first button, else the first card;
 * [entry] takes it there. While [claim], as when the app starts or a page
 * closes, the page takes focus back to where it was.
 */
@OptIn(ExperimentalFoundationApi::class)
@Composable
fun HomePage(model: AppModel, list: LazyListState, rows: RowStates, focus: FocusMemory, entry: FocusRequester, claim: Boolean, actions: HomeActions) {
    val home by model.home.collectAsStateWithLifecycle()
    val signedIn by model.signedIn.collectAsStateWithLifecycle()
    val titles = home.associate { it.id to rowTitle(it) }
    val hero = remember(home, titles) { heroEntries(home, titles) }
    if (home.isEmpty() || home.all { it.content == RowContent.Loading }) {
        StatusView(stringResource(R.string.state_loading))
        return
    }
    val density = LocalDensity.current
    val scrolling = remember(density) { with(density) { TvScrolling(200.dp.toPx(), 120.dp.toPx()) } }
    val scope = rememberCoroutineScope()
    // The first focusable: the hero's first button, else the first row's first card.
    val firstRow = home.indexOfFirst { (it.content as? RowContent.Items)?.v1?.isNotEmpty() == true || (it.content as? RowContent.Titles)?.v1?.isNotEmpty() == true }
    // Back from a page returns focus to what opened it.
    LaunchedEffect(hero.isNotEmpty(), firstRow) { if (claim) focus.restore(entry) }
    CompositionLocalProvider(LocalBringIntoViewSpec provides scrolling) {
        LazyColumn(Modifier.fillMaxSize(), state = list, contentPadding = PaddingValues(top = if (hero.isEmpty()) 128.dp else 0.dp, bottom = MavioSpacing.s24)) {
            if (hero.isNotEmpty()) {
                item(key = "hero") {
                    // Focus on the hero shows the page from its top.
                    Hero(
                        hero,
                        Modifier.onFocusChanged { if (it.hasFocus) scope.launch { list.animateScrollToItem(0) } },
                        entry,
                        focus,
                        actions,
                    )
                    Spacer(Modifier.height(MavioSpacing.s16))
                }
            }
            itemsIndexed(home, key = { _, row -> row.id }) { i, row ->
                HomeShelf(row, titles.getValue(row.id), signedIn.size > 1, rows.of(row.id), if (hero.isEmpty() && i == firstRow) entry else null, focus, actions)
            }
        }
    }
}

/** The scroll states of a page's rows, which back returns to their starts. */
class RowStates {
    private val states = mutableMapOf<String, LazyListState>()

    /** The state of the row [id]. */
    fun of(id: String): LazyListState = states.getOrPut(id) { LazyListState() }

    /** Scrolls every row back to its start. */
    suspend fun reset() {
        states.values.forEach { it.scrollToItem(0) }
    }
}

/** Up to five titles with backdrops from Home's rows, Continue Watching aside. */
private fun heroEntries(rows: List<HomeRow>, titles: Map<String, String>): List<HeroEntry> {
    val out = mutableListOf<HeroEntry>()
    val seen = mutableSetOf<String>()
    for (row in rows) {
        if (row.kind == RowKind.CONTINUE_WATCHING) continue
        val cards = (row.content as? RowContent.Items)?.v1 ?: continue
        for (card in cards) {
            val key = "${card.serverId}/${card.itemId}"
            if (key in seen || card.kind !in heroKinds || card.backdrop == null) continue
            seen += key
            out += HeroEntry(card, titles.getValue(row.id))
            if (out.size == 5) return out
        }
    }
    return out
}

/**
 * The hero: the titles one after another, paused while it has focus, each
 * with its facts, overview and the buttons to play it and open its
 * details, the first of which [entry] focuses.
 */
@Composable
private fun Hero(entries: List<HeroEntry>, modifier: Modifier, entry: FocusRequester, focus: FocusMemory, actions: HomeActions) {
    var at by remember { mutableIntStateOf(0) }
    var paused by remember { mutableStateOf(false) }
    LaunchedEffect(paused, entries.size, at) {
        if (paused || entries.size < 2) return@LaunchedEffect
        delay(HERO_INTERVAL)
        at = (at + 1) % entries.size
    }
    val screen = with(LocalDensity.current) { LocalWindowInfo.current.containerSize.height.toDp() }
    val (card, eyebrow) = entries[at % entries.size]
    Box(
        modifier
            .fillMaxWidth()
            .height(screen * 0.78f)
            .background(theme.colors.surface)
            .onFocusChanged { paused = it.hasFocus },
    ) {
        Crossfade(card, Modifier.fillMaxSize(), animationSpec = tween(600), label = "backdrop") {
            Artwork(it.backdrop?.url, Modifier.fillMaxSize(), contentDescription = null)
        }
        Box(
            Modifier
                .fillMaxSize()
                .background(Brush.horizontalGradient(0f to Color.Black.copy(alpha = 0.78f), 0.45f to Color.Black.copy(alpha = 0.35f), 0.75f to Color.Transparent)),
        )
        Box(Modifier.fillMaxSize().background(Brush.verticalGradient(0.55f to Color.Transparent, 1f to Color.Black.copy(alpha = 0.55f))))
        Column(
            Modifier
                .align(Alignment.BottomStart)
                .padding(start = theme.layout.margin, bottom = screen * 0.14f)
                .widthIn(max = 860.dp),
            verticalArrangement = Arrangement.spacedBy(MavioSpacing.s12),
        ) {
            Text(
                eyebrow.uppercase(),
                style = theme.type.footnote.text,
                fontWeight = FontWeight.SemiBold,
                letterSpacing = 0.5.sp,
                color = Color.White.copy(alpha = 0.75f),
            )
            Logo(card.logo?.url, card.title, Modifier.heightIn(max = 160.dp).fillMaxWidth(0.8f)) {
                Text(
                    card.title,
                    fontSize = 72.sp,
                    lineHeight = 76.sp,
                    fontWeight = FontWeight.Bold,
                    color = Color.White,
                    maxLines = 2,
                    overflow = TextOverflow.Ellipsis,
                )
            }
            HeroFacts(card)
            card.overview?.let {
                Text(it, style = theme.type.body.text, color = Color.White.copy(alpha = 0.85f), maxLines = 3, overflow = TextOverflow.Ellipsis)
            }
            Row(Modifier.padding(top = MavioSpacing.s8), horizontalArrangement = Arrangement.spacedBy(MavioSpacing.s12)) {
                val playable = card.kind != CardKind.SERIES
                if (playable) {
                    TvButton(stringResource(R.string.item_play), { actions.play(card) }, Modifier.focusRequester(entry).focusKey(focus, "hero/play"), icon = R.drawable.ic_play_fill)
                }
                TvButton(
                    stringResource(R.string.item_details),
                    { actions.open(card) },
                    (if (playable) Modifier else Modifier.focusRequester(entry)).focusKey(focus, "hero/details"),
                    icon = R.drawable.ic_info,
                )
            }
        }
        if (entries.size > 1) {
            Row(
                Modifier
                    .align(Alignment.BottomEnd)
                    .padding(end = theme.layout.margin, bottom = screen * 0.14f),
                horizontalArrangement = Arrangement.spacedBy(6.dp),
            ) {
                repeat(entries.size) {
                    Box(
                        Modifier
                            .size(width = if (it == at) 20.dp else 6.dp, height = 6.dp)
                            .clip(CircleShape)
                            .background(Color.White.copy(alpha = if (it == at) 1f else 0.45f)),
                    )
                }
            }
        }
    }
}

/** The year, the runtime and two genres, then the rating in a frame, as the web UI's. */
@Composable
private fun HeroFacts(card: Card) {
    val facts = listOfNotNull(card.year?.toString(), card.runtime?.let { minutesText(it.toMinutes()) }) + card.genres.take(2)
    val rating = card.officialRating
    if (facts.isEmpty() && rating == null) return
    val style = theme.type.callout.text
    Row(horizontalArrangement = Arrangement.spacedBy(MavioSpacing.s8), verticalAlignment = Alignment.CenterVertically) {
        facts.forEachIndexed { i, fact ->
            if (i > 0) Text("·", style = style, color = Color.White.copy(alpha = 0.8f))
            Text(fact, style = style, color = Color.White.copy(alpha = 0.8f), maxLines = 1)
        }
        if (rating != null) {
            Text(
                rating,
                Modifier
                    .border(1.dp, Color.White.copy(alpha = 0.5f), RoundedCornerShape(4.dp))
                    .padding(horizontal = 6.dp),
                style = theme.type.caption.text,
                color = Color.White.copy(alpha = 0.9f),
                maxLines = 1,
            )
        }
    }
}

/**
 * A row of Home: its cards, titles not on the servers, placeholders while
 * it loads, or that its server can't be reached; [entry] focuses its first
 * card.
 */
@Composable
internal fun HomeShelf(row: HomeRow, title: String, several: Boolean, state: LazyListState, entry: FocusRequester?, focus: FocusMemory, actions: HomeActions) {
    val badge: @Composable () -> Unit = {
        val server = row.serverName
        if (several && server != null && row.kind != RowKind.SERVER) ServerBadge(server)
    }
    when (val content = row.content) {
        is RowContent.Items -> {
            if (content.v1.isEmpty()) return
            val shape = if (row.kind == RowKind.CONTINUE_WATCHING) CardShape.Thumb else null
            TvShelf(title, state, badge) {
                itemsIndexed(content.v1, key = { _, c -> "${c.serverId}/${c.itemId}" }) { i, card ->
                    TvItemCard(
                        card,
                        (if (i == 0 && entry != null) Modifier.focusRequester(entry) else Modifier).focusKey(focus, "${row.id}/${card.serverId}/${card.itemId}"),
                        shape = shape ?: shapeOf(card.kind),
                    ) { actions.open(card) }
                }
            }
        }
        is RowContent.Titles -> {
            if (content.v1.isEmpty()) return
            val notOnServers: @Composable () -> Unit = {
                Text(stringResource(R.string.home_not_on_servers), style = theme.type.footnote.text, color = theme.colors.textTertiary, maxLines = 1)
            }
            TvShelf(title, state, notOnServers) {
                itemsIndexed(content.v1, key = { _, t -> t.title + t.year }) { i, t ->
                    TitlePoster(
                        row.serverId,
                        t,
                        (if (i == 0 && entry != null) Modifier.focusRequester(entry) else Modifier).focusKey(focus, "${row.id}/${t.title}/${t.year}"),
                    )
                }
            }
        }
        RowContent.Loading -> TvShelf(title, state, badge) {
            items(8) { CardPlaceholder(if (row.kind == RowKind.CONTINUE_WATCHING) CardShape.Thumb else CardShape.Poster) }
        }
        is RowContent.Unavailable -> Row(
            Modifier.padding(horizontal = theme.layout.margin, vertical = MavioSpacing.s12),
            horizontalArrangement = Arrangement.spacedBy(MavioSpacing.s8),
            verticalAlignment = Alignment.CenterVertically,
        ) {
            MavioIcon(R.drawable.ic_offline, size = 24.dp, tint = theme.colors.textTertiary)
            Text(
                stringResource(R.string.home_server_unreachable, row.serverName.orEmpty()),
                style = theme.type.callout.text,
                color = theme.colors.textTertiary,
            )
        }
    }
}

/**
 * A titled row of cards that scrolls sideways, keeping the focused card
 * inside the margins, with room above and below for it to grow.
 */
@OptIn(ExperimentalFoundationApi::class)
@Composable
fun TvShelf(title: String, state: LazyListState = rememberLazyListState(), badge: @Composable () -> Unit = {}, content: LazyListScope.() -> Unit) {
    val layout = theme.layout
    val density = LocalDensity.current
    val scrolling = remember(density, layout) { with(density) { TvScrolling(layout.margin.toPx(), layout.margin.toPx()) } }
    Column(Modifier.padding(top = MavioSpacing.s16)) {
        Row(
            Modifier.padding(horizontal = layout.margin),
            verticalAlignment = Alignment.CenterVertically,
            horizontalArrangement = Arrangement.spacedBy(MavioSpacing.s12),
        ) {
            Text(title, Modifier.weight(1f, fill = false), style = theme.type.title3.text, maxLines = 1, overflow = TextOverflow.Ellipsis)
            badge()
        }
        CompositionLocalProvider(LocalBringIntoViewSpec provides scrolling) {
            LazyRow(
                state = state,
                contentPadding = PaddingValues(horizontal = layout.margin, vertical = MavioSpacing.s24),
                horizontalArrangement = Arrangement.spacedBy(layout.gutter),
                content = content,
            )
        }
    }
}

/**
 * The card of an item on a television: its artwork with watched progress
 * and the played check, its title and subtitle; with focus it grows and
 * its artwork is framed (docs/tv.md §4). It is a poster or thumbnail
 * wide, or fills its cell when [fill], as in a grid.
 */
@Composable
fun TvItemCard(card: Card, modifier: Modifier = Modifier, shape: CardShape = shapeOf(card.kind), fill: Boolean = false, onClick: () -> Unit) {
    val state = remember { Focusable() }
    val focused = state.focused()
    val width = if (shape == CardShape.Thumb) theme.layout.thumbnail else theme.layout.poster
    Column(modifier.shakes().then(if (fill) Modifier.fillMaxWidth() else Modifier.width(width)).focusCard(focused).selectable(state, onClick)) {
        Box(Modifier.focusArtwork(focused, MavioRadius.md)) {
            val art = if (shape == CardShape.Thumb) card.thumb ?: card.poster else card.poster ?: card.thumb
            Artwork(
                art?.url,
                Modifier
                    .fillMaxWidth()
                    .aspectRatio(shape.aspect)
                    .clip(RoundedCornerShape(MavioRadius.md)),
                icon = itemIcon(card.kind),
                contentDescription = card.title,
            )
            val progress = card.progress
            if (progress != null && progress > 0f && !card.played) {
                ProgressBar(progress, Modifier.align(Alignment.BottomCenter).padding(16.dp))
            }
            if (card.played) {
                MavioIcon(
                    R.drawable.ic_played_fill,
                    Modifier
                        .align(Alignment.TopEnd)
                        .padding(10.dp)
                        .background(theme.colors.scrim, CircleShape)
                        .padding(3.dp),
                    size = 28.dp,
                    tint = Color.White,
                )
            }
        }
        Spacer(Modifier.height(MavioSpacing.s12))
        Text(card.title, style = theme.type.headline.text, maxLines = 1, overflow = TextOverflow.Ellipsis)
        subtitleOf(card)?.let {
            Text(it, style = theme.type.footnote.text, color = theme.colors.textSecondary, maxLines = 1, overflow = TextOverflow.Ellipsis)
        }
    }
}

/**
 * A title of a row from outside the servers: its poster, title and year,
 * opening its details where a server has it, or else on Discover of the
 * row's server, to be requested, as the web UI's.
 */
@Composable
private fun TitlePoster(serverId: String?, title: TitleCard, modifier: Modifier) {
    val pages = LocalPages.current
    val state = remember { Focusable() }
    val focused = state.focused()
    val at = title.available.firstOrNull()
    val tmdb = title.externalIds["tmdb"]?.toIntOrNull()
    Column(
        modifier.shakes().width(theme.layout.poster).focusCard(focused).alpha(if (at != null) 1f else 0.8f).selectable(state) {
            when {
                at != null -> pages.openItem(at.serverId, at.itemId)
                serverId != null && tmdb != null ->
                    pages.openTitle(serverId, if (title.movie) MediaKind.MEDIA_KIND_MOVIE_VALUE else MediaKind.MEDIA_KIND_SERIES_VALUE, tmdb)
            }
        },
    ) {
        Artwork(
            title.posterUrl,
            Modifier
                .focusArtwork(focused, MavioRadius.md)
                .fillMaxWidth()
                .aspectRatio(2f / 3f)
                .clip(RoundedCornerShape(MavioRadius.md)),
            contentDescription = title.title,
        )
        Spacer(Modifier.height(MavioSpacing.s12))
        Text(title.title, style = theme.type.headline.text, maxLines = 1, overflow = TextOverflow.Ellipsis)
        title.year?.let { Text(it.toString(), style = theme.type.footnote.text, color = theme.colors.textSecondary) }
    }
}

/** The name of the server a row comes from, when several are signed in. */
@Composable
private fun ServerBadge(name: String) {
    Text(
        name,
        Modifier
            .background(theme.colors.surfaceRaised, RoundedCornerShape(MavioRadius.xs))
            .padding(horizontal = 6.dp, vertical = 2.dp),
        style = theme.type.caption.text,
        color = theme.colors.textSecondary,
        maxLines = 1,
    )
}
