package dev.mavio.app

import android.text.format.DateFormat
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.BoxWithConstraints
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.FlowRow
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.aspectRatio
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.offset
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
import androidx.compose.material3.DropdownMenu
import androidx.compose.material3.DropdownMenuItem
import androidx.compose.material3.HorizontalDivider
import androidx.compose.material3.LocalContentColor
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.CompositionLocalProvider
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableIntStateOf
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.alpha
import androidx.compose.ui.draw.blur
import androidx.compose.ui.draw.clip
import androidx.compose.ui.draw.clipToBounds
import androidx.compose.ui.draw.shadow
import androidx.compose.ui.graphics.Brush
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.graphicsLayer
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.platform.LocalDensity
import androidx.compose.ui.platform.LocalLocale
import androidx.compose.ui.platform.LocalResources
import androidx.compose.ui.platform.LocalUriHandler
import androidx.compose.ui.platform.LocalWindowInfo
import androidx.compose.ui.res.pluralStringResource
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.text.LinkAnnotation
import androidx.compose.ui.text.SpanStyle
import androidx.compose.ui.text.TextLinkStyles
import androidx.compose.ui.text.buildAnnotatedString
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.text.withLink
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import dev.mavio.core.AppModel
import dev.mavio.core.Card
import dev.mavio.core.CardKind
import dev.mavio.core.CollectionLink
import dev.mavio.core.CreditJob
import dev.mavio.core.Details
import dev.mavio.core.Episode
import dev.mavio.core.SeriesStatus
import dev.mavio.core.Source
import dev.mavio.design.DarkColors
import dev.mavio.design.MavioRadius
import dev.mavio.design.MavioSpacing
import dev.mavio.design.TabletLayout
import dev.mavio.ui.Artwork
import dev.mavio.ui.ButtonKind
import dev.mavio.ui.CircleButton
import dev.mavio.ui.ItemCard
import dev.mavio.ui.LocalTheme
import dev.mavio.ui.Logo
import dev.mavio.ui.MavioButton
import dev.mavio.ui.MavioIcon
import dev.mavio.ui.PersonCard
import dev.mavio.ui.ProgressBar
import dev.mavio.ui.R
import dev.mavio.ui.Shelf
import dev.mavio.ui.StatusView
import dev.mavio.ui.blurAvailable
import dev.mavio.ui.crewOrder
import dev.mavio.ui.format
import dev.mavio.ui.isCast
import dev.mavio.ui.itemIcon
import dev.mavio.ui.label
import dev.mavio.ui.languageName
import dev.mavio.ui.minutesText
import dev.mavio.ui.range
import dev.mavio.ui.resolution
import dev.mavio.ui.shownRole
import dev.mavio.ui.sizeText
import dev.mavio.ui.surround
import dev.mavio.ui.text
import dev.mavio.ui.theme
import dev.mavio.ui.versionName
import java.time.Duration
import java.time.Instant
import java.time.ZoneId
import java.time.format.DateTimeFormatter
import java.time.format.FormatStyle
import java.util.Date
import kotlinx.coroutines.CancellationException
import kotlinx.coroutines.launch

/**
 * An item's details, as the web UI's (libs/design/components.md, Details
 * Screen): the header with what to watch and how, a series' episodes by
 * season, an episode's season, the cast, the extras, the collection, items
 * like it, and its details and media; loaded again as the server changes.
 */
@Composable
fun DetailsScreen(model: AppModel, serverId: String, itemId: String) {
    val revisions by model.revisions.collectAsStateWithLifecycle()
    var details by remember(serverId, itemId) { mutableStateOf<Details?>(null) }
    var failure by remember(serverId, itemId) { mutableStateOf<String?>(null) }
    var attempt by remember { mutableIntStateOf(0) }
    LaunchedEffect(serverId, itemId, revisions[serverId], attempt) {
        try {
            details = model.client.details(serverId, itemId)
            failure = null
        } catch (e: CancellationException) {
            throw e
        } catch (e: Exception) {
            if (details == null) failure = e.message ?: e.toString()
        }
    }
    val shown = details
    when {
        shown != null -> DetailsContent(model, shown)
        failure != null -> StatusView(
            stringResource(R.string.state_error),
            Modifier.padding(top = LocalTopInset.current),
            icon = R.drawable.ic_error,
            message = failure,
            action = stringResource(R.string.action_retry) to { attempt++ },
        )
        else -> Box(Modifier.fillMaxSize(), Alignment.Center) { CircularProgressIndicator() }
    }
}

/** What the main action plays: the item itself, or a series' or season's episode to watch next. */
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

@Composable
private fun DetailsContent(model: AppModel, details: Details) {
    val card = details.card
    val navigator = LocalNavigator.current
    val signedIn by model.signedIn.collectAsStateWithLifecycle()
    var sourceIndex by rememberSaveable(card.itemId) { mutableIntStateOf(0) }
    // The files of this server, then the same title on the user's other servers.
    val own = details.sources.filter { it.serverId == card.serverId }
    val copies = details.sources.filter { it.serverId != card.serverId }
    val source = own.getOrNull(sourceIndex) ?: own.firstOrNull()
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
            navigator.play(source.serverId, source.itemId, card.title, source.mediaSourceId, if (here) start else null)
        } else {
            navigator.play(card.serverId, n.itemId, n.name, null, start)
        }
    }
    // The wide layout's hero reaches the content's top and holds the actions.
    val wide = theme.layout == TabletLayout
    val actions = @Composable { Actions(model, details, next, own, copies, source, onSource = { sourceIndex = own.indexOf(it) }, play = play, wide = wide) }
    LazyColumn(Modifier.fillMaxSize(), contentPadding = PaddingValues(top = if (wide) 0.dp else LocalTopInset.current, bottom = LocalBottomInset.current + MavioSpacing.s40)) {
        if (wide) {
            item(key = "header") { WideHeader(details, source, several = signedIn.size > 1, actions) }
        } else {
            item(key = "header") { Header(details, source, several = signedIn.size > 1) }
        }
        if (!wide) item(key = "actions") {
            Column(
                Modifier.padding(start = theme.layout.margin, end = theme.layout.margin, top = MavioSpacing.s16),
                verticalArrangement = Arrangement.spacedBy(MavioSpacing.s16),
            ) {
                actions()
                details.overview?.let { InlineOverview(it) }
            }
        }
        item(key = "gap") { Spacer(Modifier.height(MavioSpacing.s24)) }
        when (card.kind) {
            CardKind.SERIES -> item(key = "seasons") {
                SeasonsSection(details.children, details.episodes, details.next?.card?.itemId)
            }
            CardKind.SEASON -> if (details.episodes.isNotEmpty()) {
                item(key = "episodes-title") {
                    Text(
                        stringResource(R.string.item_episodes),
                        Modifier.padding(start = theme.layout.margin, end = theme.layout.margin, top = MavioSpacing.s8, bottom = MavioSpacing.s8),
                        style = theme.type.title3.text,
                    )
                }
                items(details.episodes, key = { "episode/${it.card.itemId}" }) { e ->
                    EpisodeRow(e, current = e.card.itemId == details.next?.card?.itemId)
                }
            }
            CardKind.EPISODE -> if (details.episodes.size > 1) item(key = "season") {
                Column(Modifier.padding(vertical = MavioSpacing.s8), verticalArrangement = Arrangement.spacedBy(MavioSpacing.s12)) {
                    Text(
                        stringResource(R.string.item_more_from, details.seasonName ?: (details.seasonNumber ?: 0).toString()),
                        Modifier.padding(horizontal = theme.layout.margin),
                        style = theme.type.title3.text,
                    )
                    EpisodeCards(details.episodes, current = card.itemId, scrollTo = card.itemId)
                }
            }
            else -> {}
        }
        when (card.kind) {
            CardKind.PLAYLIST -> item(key = "entries") { PlaylistEntries(model, details) }
            CardKind.COLLECTION -> item(key = "members") { CollectionMembers(model, details) }
            else -> {}
        }
        if (details.children.isNotEmpty() && card.kind !in setOf(CardKind.SERIES, CardKind.SEASON, CardKind.EPISODE, CardKind.PLAYLIST, CardKind.COLLECTION)) {
            item(key = "children") { Children(card.kind, details.children) }
        }
        item(key = "cast") { Cast(details) }
        if (details.extras.isNotEmpty()) item(key = "extras") { Extras(details) }
        details.collection?.let { c -> item(key = "collection") { CollectionBanner(c, details) } }
        if (details.similar.isNotEmpty()) item(key = "similar") {
            Shelf(stringResource(R.string.item_similar)) {
                items(details.similar, key = { "${it.serverId}/${it.itemId}" }) { c ->
                    ItemCard(c, Modifier.clickable { navigator.open(c) }, width = theme.layout.poster)
                }
            }
        }
        item(key = "about") { AboutSection(details, source) }
        if (!details.offline) item(key = "issues") { ItemIssues(model, card.serverId, card.itemId) }
    }
}

// Header

/**
 * The artwork in a square as wide as the screen right below the header,
 * its colors spreading up into the header, and over its lower
 * left the series an episode or season belongs to, the logo or title, the
 * facts and the badges.
 */
@Composable
private fun Header(details: Details, source: Source?, several: Boolean) {
    val card = details.card
    val image = details.backdrop ?: if (card.kind == CardKind.EPISODE) card.thumb else null
    Box(Modifier.fillMaxWidth().aspectRatio(1f)) {
        HeroBleed(1f) { HeaderArt(image?.url, card.poster?.url, card.kind) }
        Box(
            Modifier
                .matchParentSize()
                .clipToBounds()
                .background(theme.colors.surface),
        ) {
            HeaderArt(image?.url, card.poster?.url, card.kind)
            Box(
                Modifier
                    .fillMaxSize()
                    .background(
                        Brush.verticalGradient(
                            0f to Color.Transparent,
                            0.5f to Color.Black.copy(alpha = 0.5f),
                            1f to Color.Black,
                        ),
                    ),
            )
            HeaderText(
                details,
                source,
                several,
                Modifier
                    .align(Alignment.BottomStart)
                    .padding(start = theme.layout.margin, end = theme.layout.margin, bottom = MavioSpacing.s24),
            )
        }
    }
}

/**
 * The wide layout's hero, as the web UI's: the artwork across the content
 * at 70% of the window's height, 520 to 700 dp but no taller than the
 * window, shaded from the left and the bottom, and at its lower left the
 * poster when there is no backdrop, the text, the overview and the actions.
 */
@Composable
private fun WideHeader(details: Details, source: Source?, several: Boolean, actions: @Composable () -> Unit) {
    val card = details.card
    val image = details.backdrop ?: if (card.kind == CardKind.EPISODE) card.thumb else null
    val window = with(LocalDensity.current) { LocalWindowInfo.current.containerSize.height.toDp() }
    val height = (window * 0.7f).coerceIn(520.dp, 700.dp).coerceAtMost(window)
    Box(
        Modifier
            .fillMaxWidth()
            .height(height)
            .clipToBounds()
            .background(theme.colors.surface),
    ) {
        HeaderArt(image?.url, card.poster?.url, card.kind)
        Box(
            Modifier
                .fillMaxSize()
                .background(Brush.horizontalGradient(0f to Color.Black.copy(alpha = 0.82f), 0.45f to Color.Black.copy(alpha = 0.45f), 0.75f to Color.Transparent)),
        )
        Box(Modifier.fillMaxSize().background(Brush.verticalGradient(0.45f to Color.Transparent, 1f to Color.Black.copy(alpha = 0.75f))))
        // Over the artwork the colors are the dark appearance's.
        CompositionLocalProvider(LocalTheme provides theme.copy(colors = DarkColors), LocalContentColor provides DarkColors.textPrimary) {
            Row(
                Modifier
                    .align(Alignment.BottomStart)
                    .padding(horizontal = MavioSpacing.s40, vertical = MavioSpacing.s32),
                horizontalArrangement = Arrangement.spacedBy(MavioSpacing.s40),
                verticalAlignment = Alignment.Bottom,
            ) {
                val poster = card.poster
                if (image == null && poster != null) {
                    Artwork(poster.url, Modifier.width(200.dp).aspectRatio(2f / 3f).shadow(12.dp, RoundedCornerShape(8.dp)).clip(RoundedCornerShape(8.dp)))
                }
                Column(Modifier.widthIn(max = 640.dp), verticalArrangement = Arrangement.spacedBy(MavioSpacing.s16)) {
                    HeaderText(details, source, several, Modifier, wide = true, short = height < 520.dp)
                    details.overview?.let { InlineOverview(it) }
                    actions()
                }
            }
        }
    }
}

@Composable
private fun HeaderText(details: Details, source: Source?, several: Boolean, modifier: Modifier, wide: Boolean = false, short: Boolean = false) {
    val card = details.card
    val navigator = LocalNavigator.current
    val inSeries = card.kind == CardKind.EPISODE || card.kind == CardKind.SEASON
    val headline = theme.type.headline.text
    Column(modifier.widthIn(max = 640.dp), verticalArrangement = Arrangement.spacedBy(MavioSpacing.s12)) {
        details.series?.let { series ->
            val open = Modifier.clickable { navigator.open(Route.Item(card.serverId, series.itemId)) }
            val name = @Composable { Text(series.name, open, style = headline, color = Color.White.copy(alpha = 0.85f)) }
            if (series.logo != null) {
                Logo(series.logo?.url, series.name, open.heightIn(max = 48.dp).widthIn(max = 240.dp), fallback = name)
            } else {
                name()
            }
        }
        val season = details.seasonNumber
        val episode = details.episodeNumber
        if (season != null && episode != null) {
            Text(
                stringResource(R.string.item_episode_number, season.toString(), episode.toString()),
                style = headline,
                color = Color.White.copy(alpha = 0.85f),
            )
        }
        val title = @Composable {
            // A hero shorter than a window's 70% keeps the title as a phone's, clear of Back.
            val large = wide && !short
            Text(card.title, fontSize = if (large) 52.sp else 34.sp, lineHeight = if (large) 55.sp else 36.sp, fontWeight = FontWeight.Bold, color = Color.White)
        }
        if (!inSeries && details.logo != null) {
            Logo(details.logo?.url, card.title, Modifier.heightIn(max = if (short) 72.dp else 112.dp).fillMaxWidth(0.8f), fallback = title)
        } else {
            title()
        }
        val facts = facts(details)
        if (facts.isNotEmpty()) {
            // Each separator keeps to the fact before it, so that no line starts with one.
            Text(
                facts.joinToString(" · ") { it.replace(' ', ' ') },
                style = theme.type.callout.text,
                color = Color.White.copy(alpha = 0.8f),
            )
        }
        val badges = listOfNotNull(
            details.officialRating,
            source?.resolution,
            source?.range,
            source?.surround,
            if (source?.subtitles?.isNotEmpty() == true) "CC" else null,
        )
        if (badges.isNotEmpty() || several) {
            FlowRow(
                horizontalArrangement = Arrangement.spacedBy(MavioSpacing.s12),
                verticalArrangement = Arrangement.spacedBy(6.dp),
            ) {
                val caption = theme.type.caption.text
                badges.forEach {
                    Text(
                        it,
                        Modifier
                            .border(1.dp, Color.White.copy(alpha = 0.5f), RoundedCornerShape(4.dp))
                            .padding(horizontal = 6.dp),
                        style = caption,
                        fontWeight = FontWeight.SemiBold,
                        color = Color.White.copy(alpha = 0.9f),
                    )
                }
                if (several) {
                    Text(
                        details.serverName,
                        Modifier
                            .clip(RoundedCornerShape(4.dp))
                            .background(Color.White.copy(alpha = 0.2f))
                            .padding(horizontal = 6.dp),
                        style = caption,
                        color = Color.White.copy(alpha = 0.8f),
                    )
                }
            }
        }
    }
}

/**
 * The year, or a series' years and seasons, a season's episodes, an
 * episode's air date, the runtime and up to three genres.
 */
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

// Actions

/**
 * Play, or what plays next, then as circles Play from Beginning, the
 * trailer, favorite, played and the version menu; under them the progress,
 * the episode's name, the time left and when it would end.
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
    play: (NextUp, Duration?) -> Unit,
    wide: Boolean = false,
) {
    val card = details.card
    val navigator = LocalNavigator.current
    val uri = LocalUriHandler.current
    val scope = rememberCoroutineScope()
    // What the buttons show until the details load again.
    var favorite by remember(card) { mutableStateOf(card.favorite) }
    var played by remember(card) { mutableStateOf(card.played) }
    Column(verticalArrangement = Arrangement.spacedBy(MavioSpacing.s12)) {
        val primary = @Composable { modifier: Modifier ->
            if (next != null) {
                val label = when {
                    next.episode != null && next.resumable -> stringResource(R.string.item_resume_episode, next.episode)
                    next.episode != null -> stringResource(R.string.item_play_episode, next.episode)
                    next.resumable -> stringResource(R.string.item_resume)
                    else -> stringResource(R.string.item_play)
                }
                MavioButton(
                    label,
                    { play(next, if (next.resumable) next.position else null) },
                    modifier,
                    icon = R.drawable.ic_play_fill,
                    large = true,
                )
            }
        }
        // Wide, Play leads the row of buttons; on a phone it spans the screen above them.
        if (!wide) primary(Modifier.fillMaxWidth().height(48.dp))
        FlowRow(
            horizontalArrangement = Arrangement.spacedBy(MavioSpacing.s12),
            verticalArrangement = Arrangement.spacedBy(MavioSpacing.s12),
        ) {
            if (wide) primary(Modifier.height(44.dp))
            if (next != null && next.resumable) {
                CircleButton(R.drawable.ic_previous, stringResource(R.string.item_from_start), { play(next, Duration.ZERO) })
            }
            val trailer = details.extras.firstOrNull { it.kind == dev.mavio.core.ExtraKind.TRAILER }
            val remote = details.remoteTrailer
            if (trailer != null) {
                CircleButton(R.drawable.ic_movies, stringResource(R.string.item_trailer), {
                    navigator.play(trailer.card.serverId, trailer.card.itemId, trailer.card.title, null, null)
                })
            } else if (remote != null) {
                CircleButton(R.drawable.ic_movies, stringResource(R.string.item_trailer), { runCatching { uri.openUri(remote) } })
            }
            CircleButton(
                if (favorite) R.drawable.ic_favorite_fill else R.drawable.ic_favorite,
                stringResource(R.string.item_favorite),
                {
                    favorite = !favorite
                    scope.launch { runCatching { model.setFavorite(card, favorite) }.onFailure { favorite = card.favorite } }
                },
                tint = if (favorite) Color(0xFFFF453A) else theme.colors.textPrimary,
            )
            CircleButton(
                if (played) R.drawable.ic_played_fill else R.drawable.ic_played,
                stringResource(if (played) R.string.item_mark_unplayed else R.string.item_mark_played),
                {
                    played = !played
                    scope.launch { runCatching { model.setPlayed(card, played) }.onFailure { played = card.played } }
                },
            )
            if (own.size > 1 || copies.isNotEmpty()) VersionMenu(details, own, copies, source, onSource)
            MoreMenu(model, details)
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
    Row(horizontalArrangement = Arrangement.spacedBy(MavioSpacing.s12), verticalAlignment = Alignment.CenterVertically) {
        if (next.resumable && fraction != null) {
            ProgressBar(
                fraction,
                Modifier
                    .width(160.dp)
                    .clip(CircleShape)
                    .background(theme.colors.fillSelected),
                color = theme.colors.progress,
            )
        }
        if (parts.isNotEmpty()) {
            Text(parts.joinToString(" · "), style = theme.type.footnote.text, color = theme.colors.textSecondary)
        }
    }
}

/** The version to play, then the same title on the user's other servers. */
@Composable
private fun VersionMenu(details: Details, own: List<Source>, copies: List<Source>, source: Source?, onSource: (Source) -> Unit) {
    val navigator = LocalNavigator.current
    val context = LocalContext.current
    val colors = theme.colors
    var open by remember { mutableStateOf(false) }
    Box {
        Row(
            Modifier
                .height(44.dp)
                .clip(CircleShape)
                .background(colors.fill)
                .clickable { open = true }
                .padding(horizontal = MavioSpacing.s16),
            horizontalArrangement = Arrangement.spacedBy(MavioSpacing.s8),
            verticalAlignment = Alignment.CenterVertically,
        ) {
            val style = theme.type.callout.text
            Text(stringResource(R.string.item_version), style = style, fontWeight = FontWeight.SemiBold, color = colors.textSecondary)
            Text(source?.versionName ?: details.serverName, style = style, fontWeight = FontWeight.SemiBold, maxLines = 1)
            MavioIcon(R.drawable.ic_expand, size = 14.dp)
        }
        DropdownMenu(open, onDismissRequest = { open = false }) {
            if (own.size > 1) {
                Text(
                    stringResource(R.string.item_versions),
                    Modifier.padding(horizontal = MavioSpacing.s16, vertical = MavioSpacing.s8),
                    style = theme.type.footnote.text,
                    fontWeight = FontWeight.SemiBold,
                    color = colors.textSecondary,
                )
                own.forEach { s ->
                    DropdownMenuItem(
                        text = {
                            Column {
                                Text(s.versionName, style = theme.type.callout.text, fontWeight = FontWeight.SemiBold)
                                Text(
                                    listOfNotNull(s.resolution, s.range, s.surround, s.format, s.sizeText(context)).joinToString(" · "),
                                    style = theme.type.caption.text,
                                    color = colors.textSecondary,
                                )
                            }
                        },
                        onClick = {
                            onSource(s)
                            open = false
                        },
                        trailingIcon = { if (s == source) MavioIcon(R.drawable.ic_check, size = 18.dp) },
                    )
                }
                if (copies.isNotEmpty()) HorizontalDivider()
            }
            copies.forEach { copy ->
                DropdownMenuItem(
                    text = { Text(stringResource(R.string.item_on_server, copy.serverName), style = theme.type.callout.text) },
                    onClick = {
                        open = false
                        navigator.open(Route.Item(copy.serverId, copy.itemId))
                    },
                    leadingIcon = { MavioIcon(R.drawable.ic_server, size = 18.dp) },
                )
            }
        }
    }
}

/** The overview, three lines long, with More opening it whole in place. */
@Composable
private fun InlineOverview(text: String) {
    var open by rememberSaveable { mutableStateOf(false) }
    var cut by remember { mutableStateOf(false) }
    Column(verticalArrangement = Arrangement.spacedBy(MavioSpacing.s4)) {
        Text(
            text,
            style = theme.type.body.text,
            color = theme.colors.textSecondary,
            maxLines = if (open) Int.MAX_VALUE else 3,
            overflow = TextOverflow.Ellipsis,
            onTextLayout = { if (!open) cut = it.hasVisualOverflow },
        )
        if (cut && !open) {
            Text(
                stringResource(R.string.item_more),
                Modifier.clickable { open = true },
                style = theme.type.callout.text,
                fontWeight = FontWeight.SemiBold,
            )
        }
    }
}

// Episodes

/**
 * A series' episodes by season: the seasons as pills, starting with the
 * season of the episode to watch next, and the chosen one's episodes in a
 * row, with See All opening the season.
 */
@Composable
private fun SeasonsSection(seasons: List<Card>, episodes: List<Episode>, current: String?) {
    val navigator = LocalNavigator.current
    val colors = theme.colors
    var chosen by rememberSaveable { mutableStateOf<String?>(null) }
    val currentSeason = episodes.firstOrNull { it.card.itemId == current }?.seasonId
    val season = seasons.firstOrNull { it.itemId == chosen }
        ?: seasons.firstOrNull { it.itemId == currentSeason }
        ?: seasons.firstOrNull()
        ?: return
    Column(Modifier.padding(vertical = MavioSpacing.s8), verticalArrangement = Arrangement.spacedBy(MavioSpacing.s12)) {
        Row(
            Modifier.padding(horizontal = theme.layout.margin),
            horizontalArrangement = Arrangement.spacedBy(MavioSpacing.s16),
            verticalAlignment = Alignment.CenterVertically,
        ) {
            Text(stringResource(R.string.item_episodes), style = theme.type.title3.text)
            if (seasons.size > 1) {
                LazyRow(Modifier.weight(1f), horizontalArrangement = Arrangement.spacedBy(MavioSpacing.s4)) {
                    items(seasons, key = { it.itemId }) { s ->
                        val on = s.itemId == season.itemId
                        Text(
                            s.title,
                            Modifier
                                .clip(CircleShape)
                                .background(if (on) colors.fillSelected else Color.Transparent)
                                .clickable { chosen = s.itemId }
                                .padding(horizontal = MavioSpacing.s16, vertical = 6.dp),
                            style = theme.type.callout.text,
                            fontWeight = FontWeight.SemiBold,
                            color = if (on) colors.textPrimary else colors.textSecondary,
                            maxLines = 1,
                        )
                    }
                }
            } else {
                Text(season.title, Modifier.weight(1f), style = theme.type.callout.text, color = colors.textSecondary, maxLines = 1)
            }
            Text(
                stringResource(R.string.home_more),
                Modifier.clickable { navigator.open(Route.Item(season.serverId, season.itemId)) },
                style = theme.type.callout.text,
                color = colors.accentText,
            )
        }
        androidx.compose.runtime.key(season.itemId) {
            EpisodeCards(episodes.filter { it.seasonId == season.itemId }, current, scrollTo = current)
        }
    }
}

/** Episodes side by side, each with its still, number, name, length and overview, starting at scrollTo. */
@Composable
private fun EpisodeCards(episodes: List<Episode>, current: String?, scrollTo: String?) {
    val navigator = LocalNavigator.current
    val start = episodes.indexOfFirst { it.card.itemId == scrollTo }.coerceAtLeast(0)
    val state = rememberLazyListState(initialFirstVisibleItemIndex = start)
    LazyRow(
        state = state,
        contentPadding = PaddingValues(horizontal = theme.layout.margin),
        horizontalArrangement = Arrangement.spacedBy(theme.layout.gutter * 1.5f),
    ) {
        items(episodes, key = { it.card.itemId }) { e ->
            EpisodeCard(e, current = e.card.itemId == current, Modifier.clickable { navigator.open(e.card) })
        }
    }
}

/** An episode in a row (libs/design/components.md, Episode Card). */
@Composable
private fun EpisodeCard(episode: Episode, current: Boolean, modifier: Modifier) {
    val colors = theme.colors
    val width = theme.layout.thumbnail
    Column(modifier.width(width), verticalArrangement = Arrangement.spacedBy(2.dp)) {
        EpisodeStill(episode.card, Modifier.fillMaxWidth().padding(bottom = MavioSpacing.s4))
        Row(horizontalArrangement = Arrangement.spacedBy(MavioSpacing.s8)) {
            episodeShort()(episode)?.let {
                Text(
                    it,
                    style = theme.type.footnote.text,
                    fontWeight = FontWeight.SemiBold,
                    color = if (current) colors.accentText else colors.textSecondary,
                )
            }
            Text(episodeFacts(episode), style = theme.type.footnote.text, color = colors.textTertiary, maxLines = 1)
        }
        Text(episode.card.title, style = theme.type.headline.text, maxLines = 1, overflow = TextOverflow.Ellipsis)
        episode.overview?.let {
            Text(it, style = theme.type.footnote.text, color = colors.textSecondary, maxLines = 3, overflow = TextOverflow.Ellipsis)
        }
    }
}

/** A season's episode, one under another (libs/design/components.md, Episode Row). */
@Composable
private fun EpisodeRow(episode: Episode, current: Boolean) {
    val navigator = LocalNavigator.current
    val colors = theme.colors
    BoxWithConstraints(Modifier.fillMaxWidth()) {
        val still = (maxWidth * 0.32f).coerceIn(140.dp, theme.layout.thumbnail)
        Row(
            Modifier
                .fillMaxWidth()
                .clickable { navigator.open(episode.card) }
                .padding(horizontal = theme.layout.margin, vertical = MavioSpacing.s12),
            horizontalArrangement = Arrangement.spacedBy(MavioSpacing.s20),
        ) {
            EpisodeStill(episode.card, Modifier.width(still))
            Column(Modifier.padding(vertical = MavioSpacing.s4), verticalArrangement = Arrangement.spacedBy(2.dp)) {
                episodeShort()(episode)?.let {
                    Text(
                        it,
                        style = theme.type.footnote.text,
                        fontWeight = FontWeight.SemiBold,
                        color = if (current) colors.accentText else colors.textSecondary,
                    )
                }
                Text(episode.card.title, style = theme.type.headline.text)
                Text(episodeFacts(episode), style = theme.type.footnote.text, color = colors.textTertiary)
                episode.overview?.let {
                    Text(
                        it,
                        Modifier.padding(top = 6.dp),
                        style = theme.type.callout.text,
                        color = colors.textSecondary,
                        maxLines = 3,
                        overflow = TextOverflow.Ellipsis,
                    )
                }
            }
        }
    }
}

/** An episode's runtime and air date. */
private fun episodeFacts(e: Episode): String =
    listOfNotNull(e.card.runtime?.let { minutesText(it.toMinutes()) }, e.premiere?.let(::day)).joinToString(" · ")

/** An episode's still, with its progress and the played check. */
@Composable
private fun EpisodeStill(card: Card, modifier: Modifier) {
    Box(
        modifier
            .aspectRatio(16f / 9f)
            .clip(RoundedCornerShape(MavioRadius.md)),
    ) {
        Artwork((card.thumb ?: card.poster)?.url, Modifier.fillMaxSize(), icon = R.drawable.ic_shows, contentDescription = card.title)
        val progress = card.progress
        if (progress != null && progress > 0f && progress < 1f && !card.played) {
            ProgressBar(
                progress,
                Modifier
                    .align(Alignment.BottomCenter)
                    .padding(10.dp)
                    .clip(CircleShape)
                    .background(Color.White.copy(alpha = 0.35f)),
            )
        }
        if (card.played) {
            MavioIcon(
                R.drawable.ic_played_fill,
                Modifier
                    .align(Alignment.TopEnd)
                    .padding(6.dp)
                    .clip(CircleShape)
                    .background(theme.colors.scrim)
                    .padding(2.dp),
                size = 18.dp,
                tint = Color.White,
            )
        }
    }
}

// Sections

/** An album's tracks, or a folder's items as a grid. */
@Composable
private fun Children(kind: CardKind, children: List<Card>) {
    val navigator = LocalNavigator.current
    val colors = theme.colors
    when (kind) {
        CardKind.ALBUM, CardKind.AUDIOBOOK -> Column(
            Modifier.padding(horizontal = theme.layout.margin),
            verticalArrangement = Arrangement.spacedBy(MavioSpacing.s12),
        ) {
            Text(stringResource(R.string.item_tracks), style = theme.type.title3.text)
            children.forEach { c ->
                Row(
                    Modifier
                        .fillMaxWidth()
                        .clickable { navigator.open(c) }
                        .padding(vertical = MavioSpacing.s8),
                    horizontalArrangement = Arrangement.spacedBy(MavioSpacing.s12),
                ) {
                    Text(c.title, Modifier.weight(1f), style = theme.type.body.text, maxLines = 1, overflow = TextOverflow.Ellipsis)
                    c.runtime?.let { Text(minutesText(it.toMinutes()), style = theme.type.footnote.text, color = colors.textSecondary) }
                }
            }
        }
        else -> FlowRow(
            Modifier.padding(horizontal = theme.layout.margin, vertical = MavioSpacing.s12),
            horizontalArrangement = Arrangement.spacedBy(theme.layout.gutter),
            verticalArrangement = Arrangement.spacedBy(MavioSpacing.s16),
        ) {
            children.forEach { c -> ItemCard(c, Modifier.clickable { navigator.open(c) }) }
        }
    }
}

/** The cast in billing order, then the creators, directors and writers. */
@Composable
private fun Cast(details: Details) {
    val context = LocalContext.current
    val shown = details.credits.filter {
        it.job.isCast || it.job == CreditJob.CREATOR || it.job == CreditJob.DIRECTOR || it.job == CreditJob.WRITER
    }
    if (shown.isEmpty()) return
    val navigator = LocalNavigator.current
    val card = details.card
    Shelf(
        stringResource(R.string.credits_cast),
        more = { MoreLink(stringResource(R.string.credits_all)) { navigator.open(Route.Credits(card.serverId, card.itemId, crew = false)) } },
    ) {
        items(shown.size) { i ->
            val credit = shown[i]
            PersonLink(card.serverId, credit.copy(role = credit.shownRole(context)), Modifier.width(112.dp))
        }
    }
}

/** The trailers, featurettes and other extras, each played by itself. */
@Composable
private fun Extras(details: Details) {
    val navigator = LocalNavigator.current
    val colors = theme.colors
    val width = theme.layout.thumbnail
    Shelf(stringResource(R.string.item_extras)) {
        items(details.extras, key = { it.card.itemId }) { extra ->
            Column(
                Modifier
                    .width(width)
                    .clickable { navigator.play(extra.card.serverId, extra.card.itemId, extra.card.title, null, null) },
                verticalArrangement = Arrangement.spacedBy(MavioSpacing.s4),
            ) {
                Box(
                    Modifier
                        .fillMaxWidth()
                        .aspectRatio(16f / 9f)
                        .clip(RoundedCornerShape(MavioRadius.md)),
                ) {
                    Artwork((extra.card.thumb ?: details.backdrop)?.url, Modifier.fillMaxSize(), icon = R.drawable.ic_shows)
                    MavioIcon(
                        R.drawable.ic_play_fill,
                        Modifier
                            .align(Alignment.BottomStart)
                            .padding(MavioSpacing.s8)
                            .clip(CircleShape)
                            .background(colors.scrim)
                            .padding(6.dp),
                        size = 14.dp,
                        tint = Color.White,
                    )
                }
                Text(extra.card.title, style = theme.type.headline.text, maxLines = 1, overflow = TextOverflow.Ellipsis)
                Text(
                    listOfNotNull(stringResource(extra.kind.label), extra.card.runtime?.let { minutesText(it.toMinutes()) })
                        .joinToString(" · "),
                    style = theme.type.footnote.text,
                    color = colors.textSecondary,
                    maxLines = 1,
                )
            }
        }
    }
}

/** The collection a movie belongs to, over its backdrop, else the movie's. */
@Composable
private fun CollectionBanner(collection: CollectionLink, details: Details) {
    val navigator = LocalNavigator.current
    Box(
        Modifier
            .padding(horizontal = theme.layout.margin, vertical = MavioSpacing.s12)
            .fillMaxWidth()
            .heightIn(min = 112.dp)
            .clip(RoundedCornerShape(14.dp))
            .clickable { navigator.open(Route.Item(details.card.serverId, collection.itemId)) },
    ) {
        Artwork((collection.backdrop ?: details.backdrop)?.url, Modifier.matchParentSize(), icon = R.drawable.ic_collections)
        Box(
            Modifier
                .matchParentSize()
                .background(
                    Brush.horizontalGradient(
                        listOf(Color.Black.copy(alpha = 0.8f), Color.Black.copy(alpha = 0.55f), Color.Black.copy(alpha = 0.3f)),
                    ),
                ),
        )
        Row(
            Modifier
                .fillMaxWidth()
                .heightIn(min = 112.dp)
                .padding(horizontal = MavioSpacing.s24, vertical = MavioSpacing.s20),
            horizontalArrangement = Arrangement.spacedBy(MavioSpacing.s16),
            verticalAlignment = Alignment.CenterVertically,
        ) {
            Column(Modifier.weight(1f), verticalArrangement = Arrangement.spacedBy(2.dp)) {
                Text(
                    stringResource(R.string.item_collection),
                    style = theme.type.footnote.text,
                    fontWeight = FontWeight.SemiBold,
                    color = Color.White.copy(alpha = 0.7f),
                )
                Text(collection.name, style = theme.type.title3.text, color = Color.White, maxLines = 1, overflow = TextOverflow.Ellipsis)
            }
            Row(
                Modifier
                    .height(36.dp)
                    .clip(CircleShape)
                    .background(Color.White.copy(alpha = 0.2f))
                    .padding(horizontal = MavioSpacing.s16),
                horizontalArrangement = Arrangement.spacedBy(MavioSpacing.s4),
                verticalAlignment = Alignment.CenterVertically,
            ) {
                Text(
                    stringResource(R.string.item_view_collection),
                    style = theme.type.callout.text,
                    fontWeight = FontWeight.SemiBold,
                    color = Color.White,
                    maxLines = 1,
                )
                MavioIcon(R.drawable.ic_forward, size = 14.dp, tint = Color.White)
            }
        }
    }
}

// About

/**
 * The Details section (libs/design/components.md, Details Screen): the
 * leading crew, the tags and links, then the ratings, the facts and the
 * chosen version's media.
 */
@Composable
private fun AboutSection(details: Details, source: Source?) {
    val context = LocalContext.current
    val navigator = LocalNavigator.current
    val card = details.card
    val colors = theme.colors
    val uri = LocalUriHandler.current
    val inSeries = card.kind == CardKind.SERIES || card.kind == CardKind.SEASON || card.kind == CardKind.EPISODE
    val crew = details.credits.withIndex()
        .filter { !it.value.job.isCast }
        .sortedWith(compareBy({ crewOrder.indexOf(it.value.job).let { i -> if (i < 0) Int.MAX_VALUE else i } }, { it.index }))
        .map { it.value }
        .take(6)
    val tags = if (card.kind == CardKind.EPISODE || card.kind == CardKind.SEASON) emptyList() else details.tags
    var allStudios by rememberSaveable { mutableStateOf(false) }
    val studiosKey = stringResource(if (inSeries) R.string.item_networks else R.string.item_studio)
    val facts = aboutFacts(details, studiosKey, allStudios)
    val media = source?.let { mediaFacts(it, context) }.orEmpty()
    val community = details.communityRating?.takeIf { it > 0f }
    val critic = details.criticRating?.takeIf { it > 0f }
    if (crew.isEmpty() && tags.isEmpty() && details.links.isEmpty() && facts.isEmpty() && media.isEmpty() && community == null && critic == null) return
    Column(
        Modifier.padding(start = theme.layout.margin, end = theme.layout.margin, top = MavioSpacing.s32, bottom = MavioSpacing.s16),
        verticalArrangement = Arrangement.spacedBy(MavioSpacing.s32),
    ) {
        Text(stringResource(R.string.item_details), style = theme.type.title3.text)
        if (crew.isNotEmpty()) {
            FlowRow(maxItemsInEachRow = 2, verticalArrangement = Arrangement.spacedBy(MavioSpacing.s16)) {
                crew.forEach { c ->
                    Column(Modifier.weight(1f).padding(end = MavioSpacing.s24)) {
                        Text(
                            if (c.job == CreditJob.OTHER) c.role ?: stringResource(c.job.label) else stringResource(c.job.label),
                            style = theme.type.headline.text,
                        )
                        Text(
                            c.name,
                            if (c.personId.isEmpty()) Modifier else Modifier.clickable { navigator.open(Route.Person(card.serverId, c.personId)) },
                            style = theme.type.callout.text,
                            color = colors.textSecondary,
                            maxLines = 1,
                        )
                    }
                }
            }
            Row(
                Modifier.offset(y = -MavioSpacing.s12).clickable { navigator.open(Route.Credits(card.serverId, card.itemId, crew = true)) },
                horizontalArrangement = Arrangement.spacedBy(MavioSpacing.s4),
                verticalAlignment = Alignment.CenterVertically,
            ) {
                Text(stringResource(R.string.discover_full_crew), style = theme.type.callout.text, fontWeight = FontWeight.SemiBold)
                MavioIcon(R.drawable.ic_forward, size = 16.dp)
            }
        }
        if (tags.isNotEmpty()) {
            Column(verticalArrangement = Arrangement.spacedBy(MavioSpacing.s8)) {
                Text(stringResource(R.string.item_tags), style = theme.type.footnote.text, fontWeight = FontWeight.SemiBold, color = colors.textSecondary)
                FlowRow(horizontalArrangement = Arrangement.spacedBy(MavioSpacing.s8), verticalArrangement = Arrangement.spacedBy(MavioSpacing.s8)) {
                    tags.forEach {
                        Text(
                            it,
                            Modifier
                                .clip(CircleShape)
                                .then(narrowed(details) { tag = it }?.let { open -> Modifier.clickable(onClick = open) } ?: Modifier)
                                .border(1.dp, colors.border, CircleShape)
                                .padding(horizontal = MavioSpacing.s12, vertical = MavioSpacing.s4),
                            style = theme.type.footnote.text,
                        )
                    }
                }
            }
        }
        if (details.links.isNotEmpty()) {
            FlowRow(horizontalArrangement = Arrangement.spacedBy(MavioSpacing.s8), verticalArrangement = Arrangement.spacedBy(MavioSpacing.s8)) {
                details.links.forEach { link ->
                    Row(
                        Modifier
                            .clip(CircleShape)
                            .background(colors.fill)
                            .clickable { runCatching { uri.openUri(link.url) } }
                            .padding(horizontal = 14.dp, vertical = 6.dp),
                        horizontalArrangement = Arrangement.spacedBy(6.dp),
                        verticalAlignment = Alignment.CenterVertically,
                    ) {
                        Text(link.name, style = theme.type.footnote.text, fontWeight = FontWeight.SemiBold)
                        MavioIcon(R.drawable.ic_external, size = 14.dp, tint = colors.textSecondary)
                    }
                }
            }
        }
        if (community != null || critic != null) {
            FlowRow(horizontalArrangement = Arrangement.spacedBy(MavioSpacing.s12), verticalArrangement = Arrangement.spacedBy(MavioSpacing.s12)) {
                community?.let {
                    RatingCard(stringResource(R.string.item_community_rating), "/ 10") {
                        MavioIcon(R.drawable.ic_rating_fill, size = 22.dp, tint = Color(0xFFFFD60A))
                        Text(String.format(LocalLocale.current.platformLocale, "%.1f", it), style = theme.type.title2.text, fontWeight = FontWeight.Bold)
                    }
                }
                critic?.let {
                    RatingCard(stringResource(R.string.item_critic_rating), null) {
                        Text("${Math.round(it)}%", style = theme.type.title2.text, fontWeight = FontWeight.Bold)
                    }
                }
            }
        }
        val links = mapOf(
            stringResource(R.string.item_genres) to details.genres.map { g -> g to narrowed(details) { genre = g } },
            studiosKey to (if (allStudios) details.studios else details.studios.take(MIN_STUDIOS)).map { st -> st to narrowed(details) { studio = st } },
        )
        FactList(facts, links) { key ->
            if (key == studiosKey && details.studios.size > MIN_STUDIOS) {
                Text(
                    stringResource(if (allStudios) R.string.action_show_less else R.string.discover_show_more),
                    Modifier.clickable { allStudios = !allStudios },
                    style = theme.type.callout.text,
                    color = colors.textSecondary,
                )
            }
        }
        FactList(media, emptyMap()) {}
    }
}

/**
 * Opens the items of the item's library with a genre, keyword or studio,
 * when the server is reachable.
 */
@Composable
private fun narrowed(details: Details, narrow: LibraryNarrowing.() -> Unit): (() -> Unit)? {
    val navigator = LocalNavigator.current
    val libraryId = details.libraryId ?: return null
    if (details.offline) return null
    val n = LibraryNarrowing().apply(narrow)
    return { navigator.open(Route.Library(details.card.serverId, libraryId, null, genre = n.genre, tag = n.tag, studio = n.studio)) }
}

/** What a library opened from the details narrows its items to. */
private class LibraryNarrowing {
    var genre: String? = null
    var tag: String? = null
    var studio: String? = null
}

/** How many studios show until Show More. */
private const val MIN_STUDIOS = 3

/** The release, status, runtime, rating, genres, studios, countries, languages and when it was added. */
@Composable
private fun aboutFacts(details: Details, studiosKey: String, allStudios: Boolean): List<Pair<String, String>> {
    val card = details.card
    val rows = mutableListOf<Pair<String, String?>>()
    if (card.kind == CardKind.MOVIE) {
        val releases = listOfNotNull(
            details.premiere?.let { "${stringResource(R.string.discover_theatrical_release)}  ${day(it)}" },
            details.digitalRelease?.let { "${stringResource(R.string.item_digital_release)}  ${day(it)}" },
            details.physicalRelease?.let { "${stringResource(R.string.item_physical_release)}  ${day(it)}" },
        )
        rows += if (releases.size > 1) {
            stringResource(R.string.discover_release_dates) to releases.joinToString("\n")
        } else {
            stringResource(R.string.discover_release_date) to details.premiere?.let(::day)
        }
    } else {
        val key = when (card.kind) {
            CardKind.SERIES -> R.string.item_first_aired
            CardKind.EPISODE -> R.string.item_aired
            else -> R.string.item_released
        }
        rows += stringResource(key) to details.premiere?.let(::day)
    }
    if (card.kind == CardKind.SERIES) {
        rows += stringResource(R.string.item_status) to when (details.seriesStatus) {
            SeriesStatus.CONTINUING -> stringResource(R.string.item_continuing)
            SeriesStatus.ENDED -> stringResource(R.string.item_ended)
            else -> null
        }
        rows += stringResource(R.string.item_last_aired) to
            if (details.seriesStatus == SeriesStatus.ENDED) details.endDate?.let(::day) else null
    }
    rows += stringResource(R.string.item_runtime) to
        if (card.kind == CardKind.SERIES) null else card.runtime?.let { minutesText(it.toMinutes()) }
    rows += stringResource(R.string.item_rated) to details.officialRating
    rows += stringResource(R.string.item_genres) to details.genres.takeIf { it.isNotEmpty() }?.joinToString(", ")
    rows += studiosKey to details.studios.takeIf { it.isNotEmpty() }
        ?.let { if (allStudios) it else it.take(MIN_STUDIOS) }?.joinToString(", ")
    rows += stringResource(R.string.item_country) to details.countries.takeIf { it.isNotEmpty() }?.joinToString("\n")
    rows += stringResource(R.string.item_original_language) to details.originalLanguage?.let { languageName(it) ?: it }
    rows += stringResource(R.string.item_original_title) to details.originalTitle?.takeIf { it != card.title }
    rows += stringResource(R.string.item_date_added) to details.dateAdded?.let(::day)
    return rows.mapNotNull { (k, v) -> v?.let { k to it } }
}

/** The chosen version's video, audio, subtitles and file. */
@Composable
private fun mediaFacts(source: Source, context: android.content.Context): List<Pair<String, String>> {
    val video = source.video?.let { v -> listOfNotNull(source.resolution, v.codec.uppercase(), source.range).joinToString(" · ") }
    val audio = source.audio.takeIf { it.isNotEmpty() }?.joinToString("\n") { a ->
        val channels = when {
            a.channels == 6 -> "5.1"
            a.channels == 8 -> "7.1"
            a.channels > 0 -> "${a.channels} ch"
            else -> null
        }
        listOfNotNull(a.title ?: languageName(a.language), a.codec.uppercase(), channels).joinToString(" · ")
    }
    val subtitles = source.subtitles.takeIf { it.isNotEmpty() }?.joinToString("\n") { s ->
        listOfNotNull(s.title ?: languageName(s.language), s.codec.uppercase()).joinToString(" · ")
    }
    val file = listOfNotNull(source.fileName, source.sizeText(context)).joinToString("\n").ifEmpty { null }
    return listOf(
        stringResource(R.string.item_video) to video,
        stringResource(R.string.item_audio_tracks) to audio,
        stringResource(R.string.item_subtitle_tracks) to subtitles,
        stringResource(R.string.item_file) to file,
    ).mapNotNull { (k, v) -> v?.let { k to it } }
}

/** A rating, large, with its caption and scale. */
@Composable
private fun RatingCard(label: String, scale: String?, value: @Composable () -> Unit) {
    val colors = theme.colors
    Column(
        Modifier
            .widthIn(min = 144.dp)
            .clip(RoundedCornerShape(12.dp))
            .background(colors.fill)
            .padding(horizontal = MavioSpacing.s16, vertical = MavioSpacing.s12),
        verticalArrangement = Arrangement.spacedBy(MavioSpacing.s4),
    ) {
        Text(label, style = theme.type.footnote.text, fontWeight = FontWeight.SemiBold, color = colors.textSecondary)
        Row(horizontalArrangement = Arrangement.spacedBy(6.dp), verticalAlignment = Alignment.CenterVertically) {
            value()
            scale?.let { Text(it, style = theme.type.callout.text, color = colors.textTertiary) }
        }
    }
}

/** Facts, each a caption over its value, two to a line. */
@Composable
private fun FactList(
    rows: List<Pair<String, String>>,
    links: Map<String, List<Pair<String, (() -> Unit)?>>>,
    extra: @Composable (String) -> Unit,
) {
    if (rows.isEmpty()) return
    val colors = theme.colors
    FlowRow(maxItemsInEachRow = 2, verticalArrangement = Arrangement.spacedBy(MavioSpacing.s20)) {
        rows.forEach { (key, value) ->
            Column(Modifier.weight(1f).padding(end = MavioSpacing.s32), verticalArrangement = Arrangement.spacedBy(MavioSpacing.s4)) {
                Text(key, style = theme.type.footnote.text, fontWeight = FontWeight.SemiBold, color = colors.textSecondary)
                val values = links[key]
                if (values != null && values.any { it.second != null }) {
                    // Each value opening what it links to.
                    Text(
                        buildAnnotatedString {
                            values.forEachIndexed { i, (text, open) ->
                                if (i > 0) append(", ")
                                if (open == null) {
                                    append(text)
                                } else {
                                    withLink(LinkAnnotation.Clickable(text, TextLinkStyles(SpanStyle(color = colors.textPrimary))) { open() }) { append(text) }
                                }
                            }
                        },
                        style = theme.type.callout.text,
                        color = colors.textPrimary,
                    )
                } else {
                    Text(value, style = theme.type.callout.text)
                }
                extra(key)
            }
        }
    }
}

/** A details hero's artwork: the backdrop or still, else the poster blurred. */
@Composable
private fun HeaderArt(image: String?, poster: String?, kind: CardKind) {
    when {
        image != null -> Artwork(image, Modifier.fillMaxSize(), icon = itemIcon(kind))
        poster != null -> Artwork(
            poster,
            Modifier
                .fillMaxSize()
                .graphicsLayer {
                    scaleX = 1.25f
                    scaleY = 1.25f
                }
                .then(if (blurAvailable) Modifier.blur(48.dp) else Modifier)
                .alpha(0.7f),
        )
    }
}
