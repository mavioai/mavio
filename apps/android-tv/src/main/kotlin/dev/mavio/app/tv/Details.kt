package dev.mavio.app.tv

import android.text.format.DateFormat
import androidx.compose.foundation.ExperimentalFoundationApi
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.gestures.LocalBringIntoViewSpec
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.ExperimentalLayoutApi
import androidx.compose.foundation.layout.FlowRow
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
import androidx.compose.foundation.lazy.LazyRow
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.lazy.rememberLazyListState
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.CircularProgressIndicator
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
import androidx.compose.ui.draw.shadow
import androidx.compose.ui.focus.FocusRequester
import androidx.compose.ui.focus.onFocusChanged
import androidx.compose.ui.graphics.Brush
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.platform.LocalDensity
import androidx.compose.ui.platform.LocalResources
import androidx.compose.ui.platform.LocalWindowInfo
import androidx.compose.ui.res.pluralStringResource
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import dev.mavio.core.AppModel
import dev.mavio.core.Card
import dev.mavio.core.CardKind
import dev.mavio.core.CreditEntry
import dev.mavio.core.CreditJob
import dev.mavio.core.Details
import dev.mavio.core.Episode
import dev.mavio.core.ExtraKind
import dev.mavio.core.SeriesStatus
import dev.mavio.core.Source
import dev.mavio.design.MavioRadius
import dev.mavio.design.MavioSpacing
import dev.mavio.ui.Artwork
import dev.mavio.ui.CardShape
import dev.mavio.ui.FactList
import dev.mavio.ui.ItemRatings
import dev.mavio.ui.LocalDiscover
import dev.mavio.ui.Focusable
import dev.mavio.ui.Logo
import dev.mavio.ui.MavioIcon
import dev.mavio.ui.ProgressBar
import dev.mavio.ui.R
import dev.mavio.ui.StatusView
import dev.mavio.ui.TitleRatings
import dev.mavio.ui.WatchProviders
import dev.mavio.ui.TvButton
import dev.mavio.ui.TvScrolling
import dev.mavio.ui.aboutFacts
import dev.mavio.ui.crewOrder
import dev.mavio.ui.focusArtwork
import dev.mavio.ui.focusCard
import dev.mavio.ui.focused
import dev.mavio.ui.isCast
import dev.mavio.ui.label
import dev.mavio.ui.mediaFacts
import dev.mavio.ui.minutesText
import dev.mavio.ui.range
import dev.mavio.ui.resolution
import dev.mavio.ui.selectable
import dev.mavio.ui.shakes
import dev.mavio.ui.discoverLanguage
import dev.mavio.ui.discovered
import dev.mavio.ui.shownRatings
import dev.mavio.ui.shownRole
import dev.mavio.ui.surround
import dev.mavio.ui.text
import dev.mavio.ui.theme
import dev.mavio.ui.titleRatings
import java.time.Duration
import java.time.Instant
import java.time.ZoneId
import java.time.format.DateTimeFormatter
import java.time.format.FormatStyle
import java.util.Date
import kotlinx.coroutines.CancellationException
import kotlinx.coroutines.launch
import mavio.acquisition.v1.Discover.GetTitleResponse
import mavio.acquisition.v1.Discover.Title
import mavio.acquisition.v1.Profile.MediaKind
import mavio.acquisition.v1.getTitleRequest

/** What the primary action plays: the item itself, or a series' or season's episode to watch next. */
private data class NextUp(
    val itemId: String,
    val name: String,
    val position: Duration?,
    val progress: Float?,
    val runtime: Duration?,
    val played: Boolean,
    /** Whether it is the item shown. */
    val own: Boolean,
    /** The episode's numbers, as "S1 E2", when it is not the item shown. */
    val episode: String?,
) {
    /** Where it resumes, when it is partly watched. */
    val resumable: Boolean get() = (position?.seconds ?: 0) > 0 && !played

    /** How far it was watched, from 0 to 1. */
    val fraction: Float?
        get() {
            progress?.takeIf { it > 0f }?.let { return it }
            val p = position ?: return null
            val r = runtime?.takeIf { !it.isZero } ?: return null
            return (p.seconds.toFloat() / r.seconds).coerceAtMost(1f)
        }

    /** The time left, a minute at least. */
    val left: Duration?
        get() {
            val r = runtime ?: return null
            val p = position ?: return null
            return maxOf(Duration.ofMinutes(1), r - p)
        }

    /** When it would end if played now. */
    val endsAt: Instant?
        get() {
            val r = runtime ?: return null
            val left = r - (if (resumable) position ?: Duration.ZERO else Duration.ZERO)
            return if (left.isNegative || left.isZero) null else Instant.now() + left
        }
}

/**
 * An item's details on a television (docs/tv.md §6.2), over the whole
 * screen: the backdrop 86 % of its height with the title, the facts, two
 * lines of overview and the actions, on which focus starts; then a
 * series' seasons and their episodes, a season's episodes, an episode's
 * season, the cast, the extras and items like it. Loaded again as the
 * server changes; back returns focus to where it was in [page].
 */
@OptIn(ExperimentalFoundationApi::class)
@Composable
fun DetailsPage(model: AppModel, page: Page.Details) {
    val revisions by model.revisions.collectAsStateWithLifecycle()
    var details by remember(page) { mutableStateOf<Details?>(null) }
    var failure by remember(page) { mutableStateOf<String?>(null) }
    var attempt by remember { mutableIntStateOf(0) }
    LaunchedEffect(page, revisions[page.serverId], attempt) {
        try {
            details = model.client.details(page.serverId, page.itemId)
            failure = null
        } catch (e: CancellationException) {
            throw e
        } catch (e: Exception) {
            if (details == null) failure = e.message ?: e.toString()
        }
    }
    val shown = details
    Box(Modifier.fillMaxSize().background(theme.colors.tvBackground)) {
        when {
            shown != null -> DetailsContent(model, shown, page)
            failure != null -> StatusView(
                stringResource(R.string.state_error),
                icon = R.drawable.ic_error,
                message = failure,
                action = stringResource(R.string.action_retry) to { attempt++ },
            )
            else -> CircularProgressIndicator(Modifier.align(Alignment.Center), color = Color.White)
        }
    }
}

@OptIn(ExperimentalFoundationApi::class)
@Composable
private fun DetailsContent(model: AppModel, details: Details, page: Page.Details) {
    val card = details.card
    val pages = LocalPages.current
    val signedIn by model.signedIn.collectAsStateWithLifecycle()
    // The files of this server, then the same title on the user's other servers.
    val own = details.sources.filter { it.serverId == card.serverId }
    val copies = details.sources.filter { it.serverId != card.serverId }
    val source = own.getOrNull(page.sourceIndex) ?: own.firstOrNull()
    val short = episodeShort()
    val next = when (card.kind) {
        CardKind.SERIES, CardKind.SEASON -> details.next?.let { e ->
            NextUp(e.card.itemId, e.card.title, e.position, e.card.progress, e.card.runtime, e.card.played, false, short(e))
        }
        else -> source?.let {
            NextUp(card.itemId, card.title, details.position, card.progress, card.runtime, card.played, true, null)
        }
    }
    val play = { n: NextUp, start: Duration? ->
        if (n.own && source != null) {
            val here = source.itemId == card.itemId
            pages.play(source.serverId, source.itemId, card.title, source.mediaSourceId, if (here) start else null)
        } else {
            pages.play(card.serverId, n.itemId, n.name, null, start)
        }
    }
    val focus = page.focus
    val primary = focus.of("primary")
    // Focus starts on the primary action, and returns to where it was.
    LaunchedEffect(Unit) { focus.restore(primary) }
    val density = LocalDensity.current
    val scrolling = remember(density) { with(density) { TvScrolling(120.dp.toPx(), 120.dp.toPx()) } }
    val scope = rememberCoroutineScope()
    CompositionLocalProvider(LocalBringIntoViewSpec provides scrolling) {
        LazyColumn(Modifier.fillMaxSize(), state = page.list, contentPadding = PaddingValues(bottom = MavioSpacing.s40)) {
            item(key = "header") {
                Header(
                    details,
                    source,
                    several = signedIn.size > 1,
                    // Focus on the actions shows the page from its top.
                    Modifier.onFocusChanged { if (it.hasFocus) scope.launch { page.list.animateScrollToItem(0) } },
                ) {
                    Actions(model, details, next, own, copies, source, { page.sourceIndex = own.indexOf(it) }, focus, play)
                }
            }
            when (card.kind) {
                CardKind.SERIES -> item(key = "seasons") {
                    Seasons(details.children, details.episodes, details.next?.card?.itemId, page)
                }
                CardKind.SEASON -> if (details.episodes.isNotEmpty()) item(key = "episodes") {
                    Episodes(stringResource(R.string.item_episodes), details.episodes, details.next?.card?.itemId, page, "episodes")
                }
                CardKind.EPISODE -> if (details.episodes.size > 1) item(key = "season") {
                    val season = details.seasonName ?: (details.seasonNumber ?: 0).toString()
                    Episodes(stringResource(R.string.item_more_from, season), details.episodes, card.itemId, page, "season")
                }
                else -> {}
            }
            if (details.children.isNotEmpty() && card.kind !in setOf(CardKind.SERIES, CardKind.SEASON, CardKind.EPISODE)) {
                item(key = "children") {
                    TvShelf(stringResource(R.string.item_tracks), page.rows.of("children")) {
                        items(details.children, key = { it.itemId }) { c ->
                            TvItemCard(c, Modifier.focusKey(focus, "children/${c.itemId}")) { pages.open(c) }
                        }
                    }
                }
            }
            // The cast in billing order, then the creators, directors and writers.
            val cast = details.credits.filter { it.job.isCast } +
                details.credits.filter { it.job == CreditJob.CREATOR || it.job == CreditJob.DIRECTOR || it.job == CreditJob.WRITER }
            if (cast.isNotEmpty()) item(key = "cast") {
                val context = LocalContext.current
                TvShelf(stringResource(R.string.item_cast), page.rows.of("cast")) {
                    items(cast.size, key = { "cast/$it" }) { i ->
                        val c = cast[i]
                        PersonCard(c.copy(role = c.shownRole(context)), Modifier.focusKey(focus, "cast/$i"))
                    }
                    item(key = "all") {
                        CastAllCard(Modifier.focusKey(focus, "cast/all")) { pages.openCredits(card.serverId, card.itemId, crew = false) }
                    }
                }
            }
            if (details.extras.isNotEmpty()) item(key = "extras") {
                TvShelf(stringResource(R.string.item_extras), page.rows.of("extras")) {
                    items(details.extras, key = { it.card.itemId }) { e ->
                        TvItemCard(e.card, Modifier.focusKey(focus, "extras/${e.card.itemId}"), shape = CardShape.Thumb) {
                            pages.play(e.card.serverId, e.card.itemId, e.card.title)
                        }
                    }
                }
            }
            if (details.similar.isNotEmpty()) item(key = "similar") {
                TvShelf(stringResource(R.string.item_similar), page.rows.of("similar")) {
                    items(details.similar, key = { "${it.serverId}/${it.itemId}" }) { c ->
                        TvItemCard(c, Modifier.focusKey(focus, "similar/${c.serverId}/${c.itemId}")) { pages.open(c) }
                    }
                }
            }
            item(key = "about") { About(details, source, focus) }
        }
    }
}

/**
 * The top of the page, 86 % of the screen's height: the backdrop, and over
 * its lower left the series an episode or season belongs to, the logo or
 * name at 72 points, the facts and badges, two lines of overview and the
 * [actions], no wider than 960 points.
 */
@Composable
private fun Header(details: Details, source: Source?, several: Boolean, modifier: Modifier, actions: @Composable () -> Unit) {
    val card = details.card
    val screen = with(LocalDensity.current) { LocalWindowInfo.current.containerSize.height.toDp() }
    Box(modifier.fillMaxWidth().height(screen * 0.86f).background(theme.colors.surface)) {
        // An episode shows its still; without either, the poster, faint, colors the header.
        val backdrop = details.backdrop ?: card.backdrop ?: card.thumb
        if (backdrop != null) {
            Artwork(backdrop.url, Modifier.fillMaxSize())
        } else {
            Artwork(card.poster?.url, Modifier.fillMaxSize().alpha(0.5f))
        }
        Box(
            Modifier
                .fillMaxSize()
                .background(Brush.horizontalGradient(0f to Color.Black.copy(alpha = 0.82f), 0.45f to Color.Black.copy(alpha = 0.45f), 0.75f to Color.Transparent)),
        )
        Box(Modifier.fillMaxSize().background(Brush.verticalGradient(0.45f to Color.Transparent, 1f to Color.Black.copy(alpha = 0.75f))))
        Column(
            Modifier
                .align(Alignment.BottomStart)
                .padding(start = theme.layout.margin, end = theme.layout.margin, bottom = screen * 0.1f)
                .widthIn(max = 960.dp),
            verticalArrangement = Arrangement.spacedBy(MavioSpacing.s12),
        ) {
            details.series?.let { s ->
                Logo(s.logo?.url, s.name, Modifier.heightIn(max = 80.dp)) {
                    Text(s.name, style = theme.type.headline.text, color = Color.White.copy(alpha = 0.85f))
                }
            }
            val season = details.seasonNumber
            val episode = details.episodeNumber
            if (card.kind == CardKind.EPISODE && season != null && episode != null) {
                Text(
                    stringResource(R.string.item_episode_number, season, episode),
                    style = theme.type.headline.text,
                    color = Color.White.copy(alpha = 0.85f),
                )
            }
            val logo = if (details.series == null) details.logo ?: card.logo else null
            Logo(logo?.url, card.title, Modifier.heightIn(max = 160.dp).fillMaxWidth(0.8f)) {
                Text(card.title, fontSize = 72.sp, lineHeight = 76.sp, fontWeight = FontWeight.Bold, color = Color.White, maxLines = 2, overflow = TextOverflow.Ellipsis)
            }
            val facts = facts(details)
            if (facts.isNotEmpty()) {
                Text(facts.joinToString("  ·  "), style = theme.type.callout.text, color = Color.White.copy(alpha = 0.8f))
            }
            val subtitles = source?.subtitles?.isNotEmpty() == true
            val badges = listOfNotNull(details.officialRating, source?.resolution, source?.range, source?.surround, "CC".takeIf { subtitles })
            if (badges.isNotEmpty() || several) {
                Row(horizontalArrangement = Arrangement.spacedBy(MavioSpacing.s12), verticalAlignment = Alignment.CenterVertically) {
                    badges.forEach {
                        Text(
                            it,
                            Modifier
                                .border(1.dp, Color.White.copy(alpha = 0.5f), RoundedCornerShape(4.dp))
                                .padding(horizontal = 6.dp),
                            style = theme.type.caption.text,
                            fontWeight = FontWeight.SemiBold,
                            color = Color.White.copy(alpha = 0.9f),
                        )
                    }
                    if (several) {
                        Text(
                            details.serverName,
                            Modifier.background(Color.White.copy(alpha = 0.2f), RoundedCornerShape(4.dp)).padding(horizontal = 6.dp),
                            style = theme.type.caption.text,
                            color = Color.White.copy(alpha = 0.8f),
                        )
                    }
                }
            }
            details.overview?.let {
                Text(it, style = theme.type.body.text, color = Color.White.copy(alpha = 0.85f), maxLines = 2, overflow = TextOverflow.Ellipsis)
            }
            Spacer(Modifier.height(MavioSpacing.s8))
            actions()
        }
    }
}

/** The facts under the title: the years or day, the seasons or episodes, the runtime and the genres. */
@Composable
private fun facts(details: Details): List<String> {
    val card = details.card
    val facts = mutableListOf<String>()
    when (card.kind) {
        CardKind.SERIES -> {
            years(details)?.let { facts += it }
            val n = details.children.size
            if (n > 0) facts += pluralStringResource(R.plurals.item_season_count, n, n)
        }
        CardKind.SEASON -> {
            card.year?.let { facts += it.toString() }
            val n = details.episodes.size
            if (n > 0) facts += pluralStringResource(R.plurals.item_episode_count, n, n)
        }
        CardKind.EPISODE -> details.premiere?.let { facts += day(it) }
        else -> card.year?.let { facts += it.toString() }
    }
    if (card.kind != CardKind.SERIES) card.runtime?.let { facts += minutesText(it.toMinutes()) }
    if (card.kind != CardKind.EPISODE && details.genres.isNotEmpty()) facts += details.genres.take(3).joinToString(" / ")
    return facts
}

/** The years a series ran, as "2023–" while it continues. */
private fun years(details: Details): String? {
    val from = details.card.year ?: return null
    val to = details.endDate?.atZone(ZoneId.systemDefault())?.year
    if (to != null && to != from) return "$from–$to"
    return if (details.seriesStatus == SeriesStatus.CONTINUING) "$from–" else from.toString()
}

/** A day, as the user's locale writes it in medium length. */
private fun day(instant: Instant): String =
    DateTimeFormatter.ofLocalizedDate(FormatStyle.MEDIUM).withZone(ZoneId.systemDefault()).format(instant)

/** An episode's numbers, as "S1 E2". */
@Composable
private fun episodeShort(): (Episode) -> String? {
    val resources = LocalResources.current
    return { e -> e.number?.let { resources.getString(R.string.item_episode_short, (e.season ?: 0).toString(), it.toString()) } }
}

/**
 * Play, or what plays next, which takes focus first; then as circles Play
 * from Beginning, the trailer, favorite and played; under them the
 * progress, the episode's name, the time left and when it would end.
 */
@Composable
private fun Actions(
    model: AppModel,
    details: Details,
    next: NextUp?,
    own: List<Source>,
    copies: List<Source>,
    source: Source?,
    onSource: (Source) -> Unit,
    focus: FocusMemory,
    play: (NextUp, Duration?) -> Unit,
) {
    val card = details.card
    val pages = LocalPages.current
    val scope = rememberCoroutineScope()
    // What the buttons show until the details load again.
    var favorite by remember(card) { mutableStateOf(card.favorite) }
    var played by remember(card) { mutableStateOf(card.played) }
    Column(verticalArrangement = Arrangement.spacedBy(MavioSpacing.s16)) {
        Row(horizontalArrangement = Arrangement.spacedBy(MavioSpacing.s16), verticalAlignment = Alignment.CenterVertically) {
            if (next != null) {
                val label = when {
                    next.episode != null && next.resumable -> stringResource(R.string.item_resume_episode, next.episode)
                    next.episode != null -> stringResource(R.string.item_play_episode, next.episode)
                    next.resumable -> stringResource(R.string.item_resume)
                    else -> stringResource(R.string.item_play)
                }
                TvButton(
                    label,
                    { play(next, if (next.resumable) next.position else null) },
                    Modifier.focusKey(focus, "primary"),
                    icon = R.drawable.ic_play_fill,
                )
                if (next.resumable) {
                    TvCircleButton(R.drawable.ic_previous, stringResource(R.string.item_from_start), Modifier.focusKey(focus, "start")) {
                        play(next, Duration.ZERO)
                    }
                }
            }
            val trailer = details.extras.firstOrNull { it.kind == ExtraKind.TRAILER }
            if (trailer != null) {
                TvCircleButton(R.drawable.ic_movies, stringResource(R.string.item_trailer), Modifier.focusKey(focus, "trailer")) {
                    pages.play(trailer.card.serverId, trailer.card.itemId, trailer.card.title)
                }
            }
            TvCircleButton(
                if (favorite) R.drawable.ic_favorite_fill else R.drawable.ic_favorite,
                stringResource(R.string.item_favorite),
                // Without Play, the first circle takes focus first.
                Modifier.focusKey(focus, if (next == null && trailer == null) "primary" else "favorite"),
                tint = if (favorite) Color(0xFFFF453A) else null,
            ) {
                favorite = !favorite
                scope.launch { runCatching { model.setFavorite(card, favorite) }.onFailure { favorite = card.favorite } }
            }
            TvCircleButton(
                if (played) R.drawable.ic_played_fill else R.drawable.ic_played,
                stringResource(if (played) R.string.item_mark_unplayed else R.string.item_mark_played),
                Modifier.focusKey(focus, "played"),
            ) {
                played = !played
                scope.launch { runCatching { model.setPlayed(card, played) }.onFailure { played = card.played } }
            }
            if (!details.offline && (own.size > 1 || copies.isNotEmpty())) {
                TvVersionMenu(details, own, copies, source, Modifier.focusKey(focus, "version"), onSource)
            }
            TvMoreMenu(model, details, Modifier.focusKey(focus, "more"))
        }
        if (next != null) Status(next)
    }
}

/** The progress, the episode's name, the time left and when it would end. */
@Composable
private fun Status(next: NextUp) {
    val context = LocalContext.current
    val parts = listOfNotNull(
        if (next.episode == null) null else next.name,
        if (next.resumable) next.left?.let { stringResource(R.string.item_time_left, minutesText(it.toMinutes())) } else null,
        next.endsAt?.let { stringResource(R.string.item_ends_at, DateFormat.getTimeFormat(context).format(Date.from(it))) },
    )
    val fraction = next.fraction
    Row(horizontalArrangement = Arrangement.spacedBy(MavioSpacing.s16), verticalAlignment = Alignment.CenterVertically) {
        if (next.resumable && fraction != null) {
            ProgressBar(fraction, Modifier.width(240.dp), color = Color.White)
        }
        if (parts.isNotEmpty()) {
            Text(parts.joinToString(" · "), style = theme.type.callout.text, color = Color.White.copy(alpha = 0.8f))
        }
    }
}

/**
 * A round button of the television, 64 points, glass and white with
 * focus; its label shows under it while it has focus.
 */
@Composable
fun TvCircleButton(icon: Int, label: String, modifier: Modifier = Modifier, tint: Color? = null, onClick: () -> Unit) {
    val state = remember { Focusable() }
    val focused = state.focused()
    Box(contentAlignment = Alignment.Center) {
        Box(
            modifier
                .shakes()
                .size(64.dp)
                .then(if (focused) Modifier.shadow(12.dp, CircleShape, ambientColor = theme.colors.tvFocusShadow, spotColor = theme.colors.tvFocusShadow) else Modifier)
                .clip(CircleShape)
                .background(if (focused) Color.White else Color.White.copy(alpha = 0.2f))
                .selectable(state, onClick),
            Alignment.Center,
        ) {
            MavioIcon(icon, size = 28.dp, tint = tint ?: if (focused) Color.Black else Color.White, contentDescription = label)
        }
    }
}

/**
 * A series' seasons as pills, focus on one choosing it, then the chosen
 * season's episodes; it starts on the season of the episode up next.
 */
@Composable
private fun Seasons(seasons: List<Card>, episodes: List<Episode>, current: String?, page: Page.Details) {
    var chosen by remember { mutableStateOf<String?>(null) }
    val currentSeason = episodes.firstOrNull { it.card.itemId == current }?.seasonId
    val season = seasons.firstOrNull { it.itemId == chosen }
        ?: seasons.firstOrNull { it.itemId == currentSeason }
        ?: seasons.firstOrNull()
        ?: return
    val focus = page.focus
    Column(Modifier.padding(top = MavioSpacing.s24)) {
        if (seasons.size > 1) {
            LazyRow(
                state = page.rows.of("seasons"),
                contentPadding = PaddingValues(horizontal = theme.layout.margin, vertical = MavioSpacing.s8),
                horizontalArrangement = Arrangement.spacedBy(MavioSpacing.s8),
            ) {
                items(seasons, key = { it.itemId }) { s ->
                    SeasonPill(s.title, s.itemId == season.itemId, Modifier.focusKey(focus, "seasons/${s.itemId}")) { chosen = s.itemId }
                }
            }
        }
        androidx.compose.runtime.key(season.itemId) {
            Episodes(
                if (seasons.size > 1) null else season.title,
                episodes.filter { it.seasonId == season.itemId },
                current,
                page,
                "episodes/${season.itemId}",
            )
        }
    }
}

/** A season's pill: filled while chosen, white with focus, which chooses it. */
@Composable
private fun SeasonPill(title: String, chosen: Boolean, modifier: Modifier, onChoose: () -> Unit) {
    val state = remember { Focusable() }
    val focused = state.focused()
    Text(
        title,
        modifier
            .shakes()
            .onFocusChanged { if (it.isFocused) onChoose() }
            .clip(CircleShape)
            .background(
                when {
                    focused -> Color.White
                    chosen -> theme.colors.fillSelected
                    else -> Color.Transparent
                },
            )
            .selectable(state, onChoose)
            .padding(horizontal = 28.dp, vertical = 10.dp),
        style = theme.type.headline.text,
        color = when {
            focused -> Color.Black
            chosen -> theme.colors.textPrimary
            else -> theme.colors.textSecondary
        },
        maxLines = 1,
    )
}

/** Episodes in a row under [title], starting at [current], each opening its details. */
@Composable
private fun Episodes(title: String?, episodes: List<Episode>, current: String?, page: Page.Details, row: String) {
    if (episodes.isEmpty()) return
    val pages = LocalPages.current
    val short = episodeShort()
    val start = episodes.indexOfFirst { it.card.itemId == current }.coerceAtLeast(0)
    val state = page.rows.of(row)
    LaunchedEffect(row) { if (state.firstVisibleItemIndex == 0 && state.firstVisibleItemScrollOffset == 0) state.scrollToItem(start) }
    TvShelf(title.orEmpty(), state) {
        items(episodes, key = { it.card.itemId }) { e ->
            EpisodeCard(e, short(e), e.card.itemId == current, Modifier.focusKey(page.focus, "$row/${e.card.itemId}")) { pages.open(e.card) }
        }
    }
}

/** An episode in a row: its still with progress, its numbers and runtime, its name and overview. */
@Composable
private fun EpisodeCard(episode: Episode, short: String?, current: Boolean, modifier: Modifier, onClick: () -> Unit) {
    val state = remember { Focusable() }
    val focused = state.focused()
    val card = episode.card
    Column(modifier.shakes().width(theme.layout.thumbnail).focusCard(focused).selectable(state, onClick)) {
        Box(Modifier.focusArtwork(focused, MavioRadius.md)) {
            Artwork(
                (card.thumb ?: card.backdrop ?: card.poster)?.url,
                Modifier
                    .fillMaxWidth()
                    .aspectRatio(CardShape.Thumb.aspect)
                    .clip(RoundedCornerShape(MavioRadius.md)),
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
        Row(horizontalArrangement = Arrangement.spacedBy(MavioSpacing.s8)) {
            short?.let {
                Text(
                    it,
                    style = theme.type.footnote.text,
                    fontWeight = FontWeight.SemiBold,
                    color = if (current) theme.colors.accentText else theme.colors.textSecondary,
                )
            }
            card.runtime?.let { Text(minutesText(it.toMinutes()), style = theme.type.footnote.text, color = theme.colors.textTertiary) }
        }
        Text(card.title, style = theme.type.headline.text, maxLines = 1, overflow = TextOverflow.Ellipsis)
        episode.overview?.let {
            Text(it, style = theme.type.footnote.text, color = theme.colors.textSecondary, maxLines = 2, overflow = TextOverflow.Ellipsis)
        }
    }
}

/** A person of the cast or crew: the photo in a circle, which grows with focus, the name and the role. */
@Composable
internal fun PersonCard(credit: CreditEntry, modifier: Modifier) {
    val state = remember { Focusable() }
    val focused = state.focused()
    Column(modifier.shakes().width(180.dp).focusCard(focused).selectable(state) {}, horizontalAlignment = Alignment.CenterHorizontally) {
        Artwork(
            credit.photo?.url,
            Modifier.focusArtwork(focused, 80.dp).size(160.dp).clip(CircleShape),
            icon = R.drawable.ic_profile,
            contentDescription = credit.name,
        )
        Spacer(Modifier.height(MavioSpacing.s12))
        Text(credit.name, style = theme.type.headline.text, maxLines = 1, overflow = TextOverflow.Ellipsis, textAlign = TextAlign.Center)
        credit.role?.let {
            Text(it, style = theme.type.footnote.text, color = theme.colors.textSecondary, maxLines = 1, overflow = TextOverflow.Ellipsis, textAlign = TextAlign.Center)
        }
    }
}


/** The cast's last card, opening the whole cast and crew: an arrow in a circle over See All. */
@Composable
internal fun CastAllCard(modifier: Modifier, onClick: () -> Unit) {
    val state = remember { Focusable() }
    val focused = state.focused()
    Column(modifier.shakes().width(180.dp).focusCard(focused).selectable(state, onClick), horizontalAlignment = Alignment.CenterHorizontally) {
        Box(
            Modifier
                .focusArtwork(focused, 80.dp)
                .size(160.dp)
                .clip(CircleShape)
                .background(theme.colors.fill),
            Alignment.Center,
        ) {
            MavioIcon(R.drawable.ic_forward, size = 40.dp)
        }
        Spacer(Modifier.height(MavioSpacing.s12))
        Text(stringResource(R.string.credits_all), style = theme.type.headline.text, maxLines = 1, textAlign = TextAlign.Center)
    }
}

/**
 * The item's details (docs/tv.md §6.2), a region in the page's flow: the
 * tagline and overview, the leading crew with View Full Crew, and the
 * tags beside the ratings, the facts and the chosen version's media. Only
 * View Full Crew takes focus; a television opens no web pages.
 */
@OptIn(ExperimentalLayoutApi::class)
@Composable
private fun About(details: Details, source: Source?, focus: FocusMemory) {
    val pages = LocalPages.current
    val card = details.card
    val colors = theme.colors
    val context = LocalContext.current
    val inSeries = card.kind == CardKind.SERIES || card.kind == CardKind.SEASON || card.kind == CardKind.EPISODE
    val crew = details.credits.withIndex()
        .filter { !it.value.job.isCast }
        .sortedWith(compareBy({ crewOrder.indexOf(it.value.job).let { i -> if (i < 0) Int.MAX_VALUE else i } }, { it.index }))
        .map { it.value }
        .take(6)
    val tags = if (card.kind == CardKind.EPISODE || card.kind == CardKind.SEASON) emptyList() else details.tags
    val studiosKey = stringResource(if (inSeries) R.string.item_networks else R.string.item_studio)
    val media = source?.let { mediaFacts(it, context) }.orEmpty()
    val community = details.communityRating?.takeIf { it > 0f }
    val critic = details.criticRating?.takeIf { it > 0f }
    // A movie or series tells what its discovered title does.
    val kind = when (card.kind) {
        CardKind.MOVIE -> MediaKind.MEDIA_KIND_MOVIE
        CardKind.SERIES -> MediaKind.MEDIA_KIND_SERIES
        else -> null
    }
    val tmdbId = kind?.let { details.externalIds["tmdb"]?.toIntOrNull() } ?: 0
    val serverId = LocalDiscover.current.discovery?.serverId
    val titled = kind != null && tmdbId > 0 && serverId != null && !details.offline
    val title = if (titled) titleOf(serverId, kind, tmdbId) else null
    val ratings = if (titled) titleRatings(serverId, kind, tmdbId) else null
    val score = title?.takeIf { it.voteCount > 0 }?.communityRating
    val facts = aboutFacts(details, studiosKey, allStudios = true, title = title)
    val streaming = title?.watchProvidersList.orEmpty()
    val overview = details.overview
    val left = overview != null || crew.isNotEmpty() || tags.isNotEmpty()
    if (!left && !titled && facts.isEmpty() && media.isEmpty() && community == null && critic == null && streaming.isEmpty()) return
    Column(
        Modifier.padding(start = theme.layout.margin, end = theme.layout.margin, top = MavioSpacing.s32, bottom = MavioSpacing.s16),
        verticalArrangement = Arrangement.spacedBy(MavioSpacing.s16),
    ) {
        Text(stringResource(R.string.item_details), style = theme.type.title3.text)
        Row(horizontalArrangement = Arrangement.spacedBy(MavioSpacing.s64)) {
            if (left) {
                Column(Modifier.weight(0.44f), verticalArrangement = Arrangement.spacedBy(MavioSpacing.s20)) {
                    if (overview != null) {
                        Column(verticalArrangement = Arrangement.spacedBy(MavioSpacing.s8)) {
                            details.tagline?.let { Text(it, style = theme.type.headline.text) }
                            Text(overview, style = theme.type.body.text, color = colors.textSecondary)
                        }
                    }
                    if (crew.isNotEmpty()) {
                        FlowRow(maxItemsInEachRow = 2, verticalArrangement = Arrangement.spacedBy(MavioSpacing.s16)) {
                            crew.forEach { c ->
                                Column(Modifier.weight(1f).padding(end = MavioSpacing.s24)) {
                                    Text(
                                        if (c.job == CreditJob.OTHER) c.role ?: stringResource(c.job.label) else stringResource(c.job.label),
                                        style = theme.type.headline.text,
                                    )
                                    Text(c.name, style = theme.type.callout.text, color = colors.textSecondary, maxLines = 1, overflow = TextOverflow.Ellipsis)
                                }
                            }
                        }
                        FullCrew(Modifier.focusKey(focus, "fullCrew")) { pages.openCredits(card.serverId, card.itemId, crew = true) }
                    }
                    if (tags.isNotEmpty()) {
                        Column(verticalArrangement = Arrangement.spacedBy(MavioSpacing.s8)) {
                            Text(stringResource(R.string.item_tags), style = theme.type.footnote.text, fontWeight = FontWeight.SemiBold, color = colors.textSecondary)
                            FlowRow(horizontalArrangement = Arrangement.spacedBy(MavioSpacing.s8), verticalArrangement = Arrangement.spacedBy(MavioSpacing.s8)) {
                                tags.forEach {
                                    Text(
                                        it,
                                        Modifier
                                            .border(1.dp, colors.border, CircleShape)
                                            .padding(horizontal = MavioSpacing.s12, vertical = MavioSpacing.s4),
                                        style = theme.type.footnote.text,
                                    )
                                }
                            }
                        }
                    }
                }
            }
            Column(Modifier.weight(0.56f), verticalArrangement = Arrangement.spacedBy(MavioSpacing.s32)) {
                // A title's ratings once asked, else the library's.
                if (ratings != null && (shownRatings(ratings).isNotEmpty() || score != null)) {
                    TitleRatings(kind!!, tmdbId, ratings, score, start = true)
                } else if ((!titled || ratings != null) && (community != null || critic != null)) {
                    ItemRatings(community, critic)
                }
                FactList(facts, emptyMap()) {}
                WatchProviders(streaming)
                FactList(media, emptyMap()) {}
            }
        }
    }
}

/** A movie's or series' discovered title, null while asked or when not found. */
@Composable
private fun titleOf(serverId: String, kind: MediaKind, tmdbId: Int): Title? {
    val result by discovered(
        serverId,
        "GetTitle",
        remember(kind, tmdbId) {
            getTitleRequest {
                mediaKind = kind
                this.tmdbId = tmdbId
                language = discoverLanguage
            }
        },
        GetTitleResponse::parseFrom,
    )
    return result?.getOrNull()?.takeIf { it.hasTitle() }?.title
}

/** View Full Crew: a link under the leading crew, white with black text once it has focus. */
@Composable
internal fun FullCrew(modifier: Modifier, onClick: () -> Unit) {
    val state = remember { Focusable() }
    val focused = state.focused()
    Row(
        modifier
            .shakes()
            .clip(RoundedCornerShape(MavioRadius.md))
            .background(if (focused) Color.White else Color.Transparent)
            .selectable(state, onClick)
            .padding(horizontal = MavioSpacing.s12, vertical = MavioSpacing.s8),
        horizontalArrangement = Arrangement.spacedBy(MavioSpacing.s4),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        val ink = if (focused) Color.Black else theme.colors.textPrimary
        Text(stringResource(R.string.discover_full_crew), style = theme.type.callout.text, fontWeight = FontWeight.SemiBold, color = ink)
        MavioIcon(R.drawable.ic_forward, size = 16.dp, tint = ink)
    }
}
