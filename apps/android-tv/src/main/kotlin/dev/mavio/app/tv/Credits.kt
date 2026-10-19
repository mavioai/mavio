package dev.mavio.app.tv

import androidx.compose.foundation.ExperimentalFoundationApi
import androidx.compose.foundation.background
import androidx.compose.foundation.gestures.LocalBringIntoViewSpec
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.lazy.grid.GridCells
import androidx.compose.foundation.lazy.grid.LazyVerticalGrid
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.CompositionLocalProvider
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableIntStateOf
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.platform.LocalDensity
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import dev.mavio.core.AppModel
import dev.mavio.core.CreditJob
import dev.mavio.core.Details
import dev.mavio.design.MavioSpacing
import dev.mavio.ui.R
import dev.mavio.ui.StatusView
import dev.mavio.ui.TvScrolling
import dev.mavio.ui.crewOrder
import dev.mavio.ui.isCast
import dev.mavio.ui.label
import dev.mavio.ui.text
import dev.mavio.ui.theme
import kotlinx.coroutines.CancellationException

/**
 * An item's whole cast and crew (docs/tv.md §6.2), as a catalog of
 * Discover: under its name the cast in billing order, the guest stars,
 * then the crew by job, each in a grid; or the crew alone, which View Full
 * Crew opens. Focus starts on the first person.
 */
@OptIn(ExperimentalFoundationApi::class)
@Composable
fun CreditsPage(model: AppModel, page: Page.Credits) {
    var details by remember(page) { mutableStateOf<Details?>(null) }
    var failure by remember(page) { mutableStateOf<String?>(null) }
    var attempt by remember { mutableIntStateOf(0) }
    LaunchedEffect(page, attempt) {
        try {
            details = model.client.details(page.serverId, page.itemId)
            failure = null
        } catch (e: CancellationException) {
            throw e
        } catch (e: Exception) {
            failure = e.message ?: e.toString()
        }
    }
    val shown = details
    Box(Modifier.fillMaxSize().background(theme.colors.tvBackground)) {
        when {
            shown != null -> {
                val credits = if (page.crew) shown.credits.filter { !it.job.isCast } else shown.credits
                val groups = (listOf(CreditJob.ACTOR, CreditJob.GUEST_STAR) + crewOrder)
                    .map { job -> job to credits.filter { it.job == job } }
                    .filter { it.second.isNotEmpty() }
                val focus = page.focus
                val first = focus.of("first")
                LaunchedEffect(Unit) { focus.restore(first) }
                val density = LocalDensity.current
                val scrolling = remember(density) { with(density) { TvScrolling(200.dp.toPx(), 120.dp.toPx()) } }
                CompositionLocalProvider(LocalBringIntoViewSpec provides scrolling) {
                    LazyVerticalGrid(
                        GridCells.Adaptive(180.dp),
                        Modifier.fillMaxSize(),
                        state = page.grid,
                        contentPadding = PaddingValues(start = theme.layout.margin, end = theme.layout.margin, bottom = MavioSpacing.s48),
                        horizontalArrangement = Arrangement.spacedBy(theme.layout.gutter),
                        verticalArrangement = Arrangement.spacedBy(MavioSpacing.s24),
                    ) {
                        gridLine("header") {
                            Text(
                                stringResource(if (page.crew) R.string.credits_full_crew else R.string.item_cast),
                                Modifier.padding(top = MavioSpacing.s48),
                                style = theme.type.title1.text,
                                fontWeight = FontWeight.Bold,
                                maxLines = 1,
                            )
                        }
                        if (credits.isEmpty()) {
                            gridLine("none") {
                                Text(stringResource(R.string.credits_none), style = theme.type.body.text, color = theme.colors.textSecondary)
                            }
                        }
                        groups.forEachIndexed { g, (job, people) ->
                            gridLine("group/$job") {
                                Text(stringResource(job.label), Modifier.padding(top = MavioSpacing.s16), style = theme.type.title3.text)
                            }
                            items(people.size, key = { "$job/$it" }) { i ->
                                val credit = people[i]
                                val key = if (g == 0 && i == 0) "first" else "$job/$i"
                                // The cast's characters, and what the crew's Other did.
                                val role = if (job == CreditJob.OTHER || job.isCast) credit.role else null
                                Box(Modifier.fillMaxWidth(), Alignment.TopCenter) {
                                    PersonCard(credit.copy(role = role), Modifier.focusKey(focus, key))
                                }
                            }
                        }
                    }
                }
            }
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
