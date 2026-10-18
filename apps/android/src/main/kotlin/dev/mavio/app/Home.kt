package dev.mavio.app

import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.clickable
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
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.pager.HorizontalPager
import androidx.compose.foundation.pager.rememberPagerState
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.Text
import androidx.compose.material3.pulltorefresh.PullToRefreshBox
import androidx.compose.material3.pulltorefresh.PullToRefreshDefaults
import androidx.compose.material3.pulltorefresh.rememberPullToRefreshState
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Brush
import androidx.compose.ui.graphics.Color
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
import dev.mavio.ui.ButtonKind
import dev.mavio.ui.CardPlaceholder
import dev.mavio.ui.CardShape
import dev.mavio.ui.ItemCard
import dev.mavio.ui.Logo
import dev.mavio.ui.MavioButton
import dev.mavio.ui.R
import dev.mavio.ui.Shelf
import dev.mavio.ui.StatusView
import dev.mavio.ui.minutesText
import dev.mavio.ui.rowTitle
import dev.mavio.ui.shapeOf
import dev.mavio.ui.text
import dev.mavio.ui.theme
import kotlinx.coroutines.delay

/** A title the hero shows, and the line above it. */
private data class HeroEntry(val card: Card, val eyebrow: String)

private val heroKinds = setOf(CardKind.MOVIE, CardKind.SERIES, CardKind.VIDEO, CardKind.MUSIC_VIDEO)

/** How long each of the hero's titles shows before the next, in milliseconds. */
private const val HERO_INTERVAL = 9000L

/**
 * Home, as the web UI's (docs/clients.md §6.3): the hero of the latest
 * titles, then a shelf for each row the servers return, pulled down to
 * load again.
 */
@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun HomeScreen(model: AppModel) {
    val rows by model.home.collectAsStateWithLifecycle()
    val signedIn by model.signedIn.collectAsStateWithLifecycle()
    var refreshing by remember { mutableStateOf(false) }
    LaunchedEffect(rows) { refreshing = false }
    val titles = rows.associate { it.id to rowTitle(it) }
    val hero = remember(rows, titles) { heroEntries(rows, titles) }
    val top = LocalTopInset.current
    val pull = rememberPullToRefreshState()
    PullToRefreshBox(
        refreshing,
        onRefresh = {
            refreshing = true
            model.refreshHome()
        },
        Modifier.fillMaxSize(),
        state = pull,
        indicator = {
            PullToRefreshDefaults.Indicator(
                state = pull,
                isRefreshing = refreshing,
                modifier = Modifier.align(Alignment.TopCenter).padding(top = top),
            )
        },
    ) {
        if (rows.isEmpty()) {
            StatusView(stringResource(R.string.state_loading))
            return@PullToRefreshBox
        }
        LazyColumn(Modifier.fillMaxSize(), contentPadding = PaddingValues(top = top, bottom = LocalBottomInset.current)) {
            if (hero.isNotEmpty()) {
                item(key = "hero") {
                    Hero(hero)
                    Spacer(Modifier.height(MavioSpacing.s24))
                }
            }
            items(rows, key = { it.id }) { row ->
                HomeShelf(row, titles.getValue(row.id), several = signedIn.size > 1)
            }
        }
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

/** The hero: the titles one after another, each with its facts and the buttons to play it and open its details. */
@Composable
private fun Hero(entries: List<HeroEntry>) {
    val navigator = LocalNavigator.current
    val pager = rememberPagerState { entries.size }
    LaunchedEffect(pager, entries.size) {
        while (entries.size > 1) {
            delay(HERO_INTERVAL)
            pager.animateScrollToPage((pager.currentPage + 1) % entries.size)
        }
    }
    // A 4:5 frame as wide as the screen, right below the header, its
    // backdrop cropped to it and its colors spreading up into the header.
    Box(Modifier.fillMaxWidth().aspectRatio(4f / 5f)) {
        HeroBleed(4f / 5f) {
            Artwork(entries[pager.currentPage].card.backdrop?.url, Modifier.fillMaxSize(), contentDescription = null)
        }
        HorizontalPager(pager, Modifier.fillMaxSize()) { page ->
            val (card, eyebrow) = entries[page]
            Box(Modifier.fillMaxSize().background(theme.colors.surface)) {
                Artwork(card.backdrop?.url, Modifier.fillMaxSize(), contentDescription = null)
                Box(
                    Modifier
                        .fillMaxSize()
                        .background(
                            Brush.verticalGradient(
                                0f to Color.Transparent,
                                0.5f to Color.Black.copy(alpha = 0.4f),
                                1f to Color.Black.copy(alpha = 0.9f),
                            ),
                        ),
                )
                Column(
                    Modifier
                        .align(Alignment.BottomStart)
                        .padding(start = theme.layout.margin, end = theme.layout.margin, bottom = MavioSpacing.s32),
                    verticalArrangement = Arrangement.spacedBy(MavioSpacing.s12),
                ) {
                    Text(
                        eyebrow.uppercase(),
                        style = theme.type.footnote.text,
                        fontWeight = FontWeight.SemiBold,
                        letterSpacing = 0.5.sp,
                        color = Color.White.copy(alpha = 0.75f),
                    )
                    Logo(card.logo?.url, card.title, Modifier.heightIn(max = HERO_LOGO_HEIGHT).fillMaxWidth(0.8f)) {
                        Text(
                            card.title,
                            fontSize = 34.sp,
                            lineHeight = 36.sp,
                            fontWeight = FontWeight.Bold,
                            color = Color.White,
                            maxLines = 2,
                            overflow = TextOverflow.Ellipsis,
                        )
                    }
                    HeroFacts(card)
                    Row(
                        Modifier.padding(top = MavioSpacing.s4),
                        horizontalArrangement = Arrangement.spacedBy(MavioSpacing.s12),
                    ) {
                        if (card.kind != CardKind.SERIES) {
                            HeroButton(
                                stringResource(R.string.item_play),
                                Modifier.weight(1f),
                                white = true,
                            ) { navigator.play(card.serverId, card.itemId, card.title, null, null) }
                        }
                        HeroButton(stringResource(R.string.item_details), Modifier.weight(1f), white = false) {
                            navigator.open(card)
                        }
                    }
                }
            }
        }
        if (entries.size > 1) {
            Row(
                Modifier
                    .align(Alignment.TopEnd)
                    .padding(top = MavioSpacing.s16, end = theme.layout.margin),
                horizontalArrangement = Arrangement.spacedBy(6.dp),
            ) {
                repeat(entries.size) {
                    Box(
                        Modifier
                            .size(width = if (it == pager.currentPage) 20.dp else 6.dp, height = 6.dp)
                            .clip(CircleShape)
                            .background(Color.White.copy(alpha = if (it == pager.currentPage) 1f else 0.45f)),
                    )
                }
            }
        }
    }
}

/** The height of the hero's logo, or its title where there is none. */
private val HERO_LOGO_HEIGHT = 96.dp

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

/** A button of the hero: white for Play, glass for Details. */
@Composable
private fun HeroButton(label: String, modifier: Modifier, white: Boolean, onClick: () -> Unit) {
    if (white) {
        Row(
            modifier
                .height(44.dp)
                .clip(RoundedCornerShape(10.dp))
                .background(Color.White)
                .clickable(onClick = onClick),
            horizontalArrangement = Arrangement.spacedBy(MavioSpacing.s8, Alignment.CenterHorizontally),
            verticalAlignment = Alignment.CenterVertically,
        ) {
            dev.mavio.ui.MavioIcon(R.drawable.ic_play_fill, tint = Color.Black)
            Text(label, style = theme.type.headline.text, fontWeight = FontWeight.SemiBold, color = Color.Black)
        }
    } else {
        MavioButton(label, onClick, modifier, kind = ButtonKind.Glass, large = true)
    }
}

/** A row of Home as a shelf: its cards, titles not on the servers, placeholders while it loads, or why it can't. */
@Composable
internal fun HomeShelf(row: HomeRow, title: String, several: Boolean) {
    val navigator = LocalNavigator.current
    val badge: @Composable () -> Unit = {
        val server = row.serverName
        if (several && server != null && row.kind != RowKind.SERVER) ServerBadge(server)
    }
    when (val content = row.content) {
        is RowContent.Items -> {
            if (content.v1.isEmpty()) return
            val shape = if (row.kind == RowKind.CONTINUE_WATCHING) CardShape.Thumb else null
            Shelf(title, badge = badge) {
                items(content.v1, key = { "${it.serverId}/${it.itemId}" }) { card ->
                    ItemCard(
                        card,
                        Modifier.clickable { navigator.open(card) },
                        shape = shape ?: shapeOf(card.kind),
                    )
                }
            }
        }
        is RowContent.Titles -> {
            if (content.v1.isEmpty()) return
            Shelf(title, badge = badge) {
                items(content.v1, key = { it.title + it.year }) { TitlePoster(it) }
            }
        }
        RowContent.Loading -> Shelf(title, badge = badge) {
            items(6) { CardPlaceholder(if (row.kind == RowKind.CONTINUE_WATCHING) CardShape.Thumb else CardShape.Poster) }
        }
        is RowContent.Unavailable -> Column(
            Modifier.padding(horizontal = theme.layout.margin, vertical = MavioSpacing.s12),
            verticalArrangement = Arrangement.spacedBy(MavioSpacing.s4),
        ) {
            Text(title, style = theme.type.title3.text)
            Text(
                if (row.kind == RowKind.SERVER) stringResource(R.string.home_server_unreachable, row.serverName.orEmpty()) else content.v1,
                style = theme.type.callout.text,
                color = theme.colors.textSecondary,
            )
        }
    }
}

/** The name of the server a row comes from, when several are signed in. */
@Composable
private fun ServerBadge(name: String) {
    Text(
        name,
        Modifier
            .background(theme.colors.fill, RoundedCornerShape(MavioRadius.xs))
            .padding(horizontal = 6.dp, vertical = 2.dp),
        style = theme.type.caption.text,
        color = theme.colors.textSecondary,
        maxLines = 1,
    )
}

/** A title not on the servers yet: its poster, title and year. */
@Composable
private fun TitlePoster(title: TitleCard) {
    Column(Modifier.width(theme.layout.poster)) {
        Artwork(
            title.posterUrl,
            Modifier
                .fillMaxWidth()
                .aspectRatio(2f / 3f)
                .clip(RoundedCornerShape(MavioRadius.md)),
            icon = if (title.movie) R.drawable.ic_movies else R.drawable.ic_shows,
            contentDescription = title.title,
        )
        Box(Modifier.height(MavioSpacing.s8))
        Text(title.title, style = theme.type.headline.text, maxLines = 1, overflow = TextOverflow.Ellipsis)
        title.year?.let { Text(it.toString(), style = theme.type.footnote.text, color = theme.colors.textSecondary) }
    }
}
