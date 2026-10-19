package dev.mavio.ui

import android.content.Context
import android.view.animation.AccelerateInterpolator
import androidx.activity.ComponentActivity
import androidx.core.splashscreen.SplashScreen.Companion.installSplashScreen
import androidx.lifecycle.lifecycleScope
import coil3.SingletonImageLoader
import dev.mavio.core.AppModel
import dev.mavio.core.CardKind
import dev.mavio.core.Row
import dev.mavio.core.RowContent
import dev.mavio.core.RowKind
import dev.mavio.design.MavioDuration
import kotlin.time.Duration
import kotlin.time.Duration.Companion.seconds
import kotlinx.coroutines.coroutineScope
import kotlinx.coroutines.flow.first
import kotlinx.coroutines.launch
import kotlinx.coroutines.withTimeoutOrNull

/** The kinds of item Home's hero shows. */
val heroKinds = setOf(CardKind.MOVIE, CardKind.SERIES, CardKind.VIDEO, CardKind.MUSIC_VIDEO)

/**
 * Keeps the system's launch screen, the mark, until Home can be drawn
 * whole — its rows and the first screen's images — or for at most [limit],
 * then grows the mark as it fades into Home, so that no placeholder flashes
 * by (docs/clients.md §4.2). Called before the activity's `super.onCreate`.
 */
fun ComponentActivity.installLaunch(model: AppModel, limit: Duration = 2.seconds) {
    val splash = installSplashScreen()
    var ready = false
    splash.setKeepOnScreenCondition { !ready }
    lifecycleScope.launch {
        awaitHome(this@installLaunch, model, limit)
        ready = true
    }
    splash.setOnExitAnimationListener { screen ->
        val duration = MavioDuration.slow.toLong()
        // The icon's view is missing where the system drew the launch screen
        // without one, as when the app opens from a notification.
        runCatching { screen.iconView }.getOrNull()?.animate()
            ?.scaleX(1.6f)?.scaleY(1.6f)
            ?.setDuration(duration)?.setInterpolator(AccelerateInterpolator())
            ?.start()
        screen.view.animate()
            .alpha(0f)
            .setDuration(duration).setInterpolator(AccelerateInterpolator())
            .withEndAction { screen.remove() }
            .start()
    }
}

/**
 * Waits until Home's rows have their servers' answers and the first
 * screen's images are on hand, or [limit] passes, whichever comes first.
 * Signed out, the sign-in needs nothing.
 */
private suspend fun awaitHome(context: Context, model: AppModel, limit: Duration) {
    if (model.signedIn.value.isEmpty()) return
    withTimeoutOrNull(limit) {
        val rows = model.home.first { rows -> rows.isNotEmpty() && rows.none { it.content is RowContent.Loading } }
        val loader = SingletonImageLoader.get(context)
        coroutineScope {
            for (url in firstImages(rows)) launch { loader.execute(imageRequest(context, url)) }
        }
    }
}

/**
 * The images Home's first screen shows: the hero's backdrop and logo, and
 * the artwork of the first rows' first cards.
 */
private fun firstImages(rows: List<Row>): List<String> {
    val urls = mutableListOf<String>()
    val hero = rows.asSequence()
        .filter { it.kind != RowKind.CONTINUE_WATCHING }
        .flatMap { (it.content as? RowContent.Items)?.v1.orEmpty() }
        .firstOrNull { it.kind in heroKinds && it.backdrop != null }
    if (hero != null) urls += listOfNotNull(hero.backdrop?.url, hero.logo?.url)
    for (row in rows.take(2)) {
        when (val content = row.content) {
            is RowContent.Items -> for (card in content.v1.take(6)) {
                val thumb = row.kind == RowKind.CONTINUE_WATCHING || shapeOf(card.kind) == CardShape.Thumb
                val art = if (thumb) card.thumb ?: card.poster else card.poster ?: card.thumb
                art?.url?.let { urls += it }
            }
            is RowContent.Titles -> urls += content.v1.take(6).mapNotNull { it.posterUrl }
            else -> {}
        }
    }
    return urls
}
