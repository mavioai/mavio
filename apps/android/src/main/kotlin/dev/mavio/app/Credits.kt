package dev.mavio.app

import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.BoxWithConstraints
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.ExperimentalLayoutApi
import androidx.compose.foundation.layout.FlowRow
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.aspectRatio
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.layout.widthIn
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.LazyListScope
import androidx.compose.foundation.lazy.grid.GridCells
import androidx.compose.foundation.lazy.grid.LazyVerticalGrid
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableIntStateOf
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.platform.LocalUriHandler
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.Dp
import androidx.compose.ui.unit.dp
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import dev.mavio.core.AppModel
import dev.mavio.core.Card
import dev.mavio.core.CardKind
import dev.mavio.core.CreditEntry
import dev.mavio.core.CreditJob
import dev.mavio.core.PersonDetails
import dev.mavio.design.MavioRadius
import dev.mavio.design.MavioSpacing
import dev.mavio.ui.Artwork
import dev.mavio.ui.ItemCard
import dev.mavio.ui.MavioIcon
import dev.mavio.ui.PersonCard
import dev.mavio.ui.R
import dev.mavio.ui.StatusView
import dev.mavio.ui.crewOrder
import dev.mavio.ui.isCast
import dev.mavio.ui.label
import dev.mavio.ui.text
import dev.mavio.ui.theme
import java.time.Instant
import java.time.Period
import java.time.ZoneId
import java.time.format.DateTimeFormatter
import java.time.format.FormatStyle
import kotlin.coroutines.cancellation.CancellationException

/**
 * Loads something of a server again as the server changes, keeping what
 * was shown when a later load fails.
 */
@Composable
private fun <T : Any> loaded(model: AppModel, serverId: String, key: Any, attempt: Int, load: suspend () -> T): Pair<T?, String?> {
    val revisions by model.revisions.collectAsStateWithLifecycle()
    var value by remember(serverId, key) { mutableStateOf<T?>(null) }
    var failure by remember(serverId, key) { mutableStateOf<String?>(null) }
    LaunchedEffect(serverId, key, revisions[serverId], attempt) {
        try {
            value = load()
            failure = null
        } catch (e: CancellationException) {
            throw e
        } catch (e: Exception) {
            if (value == null) failure = e.reason
        }
    }
    return value to failure
}

/** The spinner while loading, or the error with Retry. */
@Composable
private fun Pending(failure: String?, onRetry: () -> Unit) {
    if (failure != null) {
        StatusView(
            stringResource(R.string.state_error),
            Modifier.padding(top = LocalTopInset.current),
            icon = R.drawable.ic_error,
            message = failure,
            action = stringResource(R.string.action_retry) to onRetry,
        )
    } else {
        Box(Modifier.fillMaxSize(), Alignment.Center) { CircularProgressIndicator() }
    }
}

/**
 * An item's whole cast and crew (libs/design/components.md, Credits
 * Screen), as the web UI's: the cast in billing order, the guest stars,
 * then the crew by job; or the crew alone, which View Full Crew opens.
 */
@Composable
fun CreditsScreen(model: AppModel, route: Route.Credits) {
    var attempt by remember { mutableIntStateOf(0) }
    val (details, failure) = loaded(model, route.serverId, route.itemId, attempt) { model.client.details(route.serverId, route.itemId) }
    if (details == null) return Pending(failure) { attempt++ }
    val credits = if (route.crew) details.credits.filter { !it.job.isCast } else details.credits
    val groups = (listOf(CreditJob.ACTOR, CreditJob.GUEST_STAR) + crewOrder)
        .map { job -> job to credits.filter { it.job == job } }
        .filter { it.second.isNotEmpty() }
    LazyVerticalGrid(
        GridCells.Adaptive(112.dp),
        Modifier.fillMaxSize(),
        contentPadding = PaddingValues(
            start = theme.layout.margin,
            end = theme.layout.margin,
            top = LocalTopInset.current,
            bottom = LocalBottomInset.current + MavioSpacing.s24,
        ),
        horizontalArrangement = Arrangement.spacedBy(MavioSpacing.s16),
        verticalArrangement = Arrangement.spacedBy(MavioSpacing.s24),
    ) {
        line("title") { ScreenTitle(stringResource(if (route.crew) R.string.credits_full_crew else R.string.item_cast)) }
        if (credits.isEmpty()) {
            line("none") { Text(stringResource(R.string.credits_none), style = theme.type.body.text, color = theme.colors.textSecondary) }
        }
        groups.forEach { (job, people) ->
            line("group/$job") { Text(stringResource(job.label), Modifier.padding(top = MavioSpacing.s8), style = theme.type.title3.text) }
            items(people.size, key = { "$job/$it" }) { i ->
                val credit = people[i]
                // The cast's characters, and what the crew's Other did.
                val role = if (job == CreditJob.OTHER || job.isCast) credit.role else null
                Box(Modifier.fillMaxWidth(), Alignment.TopCenter) { PersonLink(route.serverId, credit.copy(role = role)) }
            }
        }
    }
}

/** A credited person's card, opening their page when the server names them. */
@Composable
internal fun PersonLink(serverId: String, credit: CreditEntry, modifier: Modifier = Modifier) {
    val navigator = LocalNavigator.current
    PersonCard(
        credit,
        modifier.then(
            if (credit.personId.isEmpty()) Modifier else Modifier.clickable { navigator.open(Route.Person(serverId, credit.personId)) },
        ),
    )
}

/**
 * A person's page (docs/clients.md §6.3.19), as the web UI's on a phone,
 * opened from the cast and crew of an item's details: their photo, dates,
 * biography and links, and the movies, series and episodes of the server
 * crediting them.
 */
@Composable
fun PersonScreen(model: AppModel, route: Route.Person) {
    var attempt by remember { mutableIntStateOf(0) }
    val (person, failure) = loaded(model, route.serverId, route.personId, attempt) { model.client.person(route.serverId, route.personId) }
    if (person == null) return Pending(failure) { attempt++ }
    LazyColumn(
        Modifier.fillMaxSize(),
        contentPadding = PaddingValues(top = LocalTopInset.current, bottom = LocalBottomInset.current + MavioSpacing.s40),
    ) {
        item(key = "header") { PersonHeader(person) }
        titles("movies", R.string.person_movies, person.items.filter { it.kind == CardKind.MOVIE })
        titles("series", R.string.person_series, person.items.filter { it.kind == CardKind.SERIES })
        titles("episodes", R.string.person_episodes, person.items.filter { it.kind == CardKind.EPISODE }, wide = true)
        if (person.items.isEmpty()) {
            item(key = "none") {
                Text(
                    stringResource(R.string.person_none),
                    Modifier.padding(start = theme.layout.margin, end = theme.layout.margin, top = MavioSpacing.s32),
                    style = theme.type.body.text,
                    color = theme.colors.textSecondary,
                )
            }
        }
    }
}

/** A person's photo, then name, dates, biography and links: one above the other, or beside the photo from 640 dp. */
@Composable
private fun PersonHeader(person: PersonDetails) {
    val format = DateTimeFormatter.ofLocalizedDate(FormatStyle.LONG)
    val day = { i: Instant -> i.atZone(ZoneId.systemDefault()).toLocalDate() }
    val facts = listOfNotNull(
        person.born?.let {
            val age = Period.between(day(it), day(person.died ?: Instant.now())).years
            stringResource(R.string.person_born, day(it).format(format), age.toString())
        },
        person.birthPlace,
        person.died?.let { stringResource(R.string.person_died, day(it).format(format)) },
    )
    // From 640 dp the photo stands beside the rest, as the web UI's.
    val row = windowWidth() >= 640.dp
    val photo = @Composable {
        Artwork(
            person.photo?.url,
            // The photo keeps twice the gap from the name that the rest keep.
            Modifier
                .padding(bottom = if (row) 0.dp else MavioSpacing.s12)
                .width(if (row) 208.dp else 160.dp)
                .aspectRatio(2f / 3f)
                .clip(RoundedCornerShape(14.dp)),
            icon = R.drawable.ic_profile,
            contentDescription = person.name,
        )
    }
    val padding = Modifier.padding(start = theme.layout.margin, end = theme.layout.margin, top = MavioSpacing.s24)
    if (row) {
        Row(padding, horizontalArrangement = Arrangement.spacedBy(MavioSpacing.s24)) {
            photo()
            Box(Modifier.widthIn(max = 768.dp)) { PersonInfo(person, facts) }
        }
    } else {
        Column(padding, verticalArrangement = Arrangement.spacedBy(MavioSpacing.s12)) {
            photo()
            PersonInfo(person, facts)
        }
    }
}

/** A person's name, facts, overview and links. */
@OptIn(ExperimentalLayoutApi::class)
@Composable
private fun PersonInfo(person: PersonDetails, facts: List<String>) {
    val colors = theme.colors
    val uri = LocalUriHandler.current
    var more by rememberSaveable { mutableStateOf(false) }
    Column(verticalArrangement = Arrangement.spacedBy(MavioSpacing.s12)) {
        Text(person.name, style = theme.type.title1.text, fontWeight = FontWeight.Bold)
        facts.forEach { Text(it, style = theme.type.callout.text, color = colors.textSecondary) }
        person.overview?.let { overview ->
            Column(verticalArrangement = Arrangement.spacedBy(MavioSpacing.s4)) {
                Text(overview, style = theme.type.body.text, maxLines = if (more) Int.MAX_VALUE else 5)
                Text(
                    stringResource(if (more) R.string.person_less else R.string.person_more),
                    Modifier.clip(RoundedCornerShape(MavioRadius.xs)).clickable { more = !more },
                    style = theme.type.callout.text,
                    fontWeight = FontWeight.SemiBold,
                    color = colors.accentText,
                )
            }
        }
        if (person.links.isNotEmpty()) {
            FlowRow(
                Modifier.padding(top = MavioSpacing.s4),
                horizontalArrangement = Arrangement.spacedBy(MavioSpacing.s8),
                verticalArrangement = Arrangement.spacedBy(MavioSpacing.s8),
            ) {
                person.links.forEach { link ->
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
    }
}

/** A heading with its count over the cards of one kind, in rows as many as fit. */
private fun LazyListScope.titles(key: String, title: Int, cards: List<Card>, wide: Boolean = false) {
    if (cards.isEmpty()) return
    item(key = "$key/title") {
        Row(
            Modifier.padding(start = theme.layout.margin, end = theme.layout.margin, top = MavioSpacing.s32, bottom = MavioSpacing.s12),
            horizontalArrangement = Arrangement.spacedBy(MavioSpacing.s8),
            verticalAlignment = Alignment.Bottom,
        ) {
            Text(stringResource(title), style = theme.type.title3.text)
            Text(cards.size.toString(), style = theme.type.callout.text, color = theme.colors.textSecondary)
        }
    }
    item(key = "$key/cards") {
        CardRows(cards, if (wide) theme.layout.thumbnail else theme.layout.poster)
    }
}

/** Cards in rows, as many to a row as fit at their width at least, stretched to fill it. */
@Composable
private fun CardRows(cards: List<Card>, min: Dp) {
    val navigator = LocalNavigator.current
    val gap = theme.layout.gutter * 1.5f
    BoxWithConstraints(Modifier.padding(horizontal = theme.layout.margin)) {
        val columns = maxOf(1, ((maxWidth + gap) / (min + gap)).toInt())
        Column(verticalArrangement = Arrangement.spacedBy(MavioSpacing.s24)) {
            cards.chunked(columns).forEach { row ->
                Row(horizontalArrangement = Arrangement.spacedBy(gap)) {
                    row.forEach { c -> ItemCard(c, Modifier.weight(1f).clickable { navigator.open(c) }, width = null) }
                    repeat(columns - row.size) { Spacer(Modifier.weight(1f)) }
                }
            }
        }
    }
}
