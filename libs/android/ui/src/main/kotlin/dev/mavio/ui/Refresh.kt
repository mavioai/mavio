package dev.mavio.ui

import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.BoxScope
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.padding
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.pulltorefresh.PullToRefreshBox
import androidx.compose.material3.pulltorefresh.PullToRefreshDefaults
import androidx.compose.material3.pulltorefresh.rememberPullToRefreshState
import androidx.compose.runtime.Composable
import androidx.compose.runtime.Stable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableIntStateOf
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.rememberUpdatedState
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.unit.Dp
import androidx.compose.ui.unit.dp
import kotlinx.coroutines.launch

/**
 * How a page that can be read again is read again: its loads are keyed on
 * [attempt], which a pull or Retry bumps, and each calls [done] as it ends,
 * which takes the pull's spinner away.
 */
@Stable
class Reload {
    /** Bumped by each pull and Retry, for the page's loads to key on. */
    var attempt by mutableIntStateOf(0)
        private set

    /** Whether a pull waits for the page to be read again. */
    var refreshing by mutableStateOf(false)
        private set

    /** Reads the page again, the spinner showing until [done]. */
    fun pull() {
        refreshing = true
        attempt++
    }

    /** Reads the page again, without the spinner. */
    fun retry() {
        attempt++
    }

    /** Called by a load as it ends, though it failed; not when it was cancelled. */
    fun done() {
        refreshing = false
    }
}

/** A page's [Reload]. */
@Composable
fun rememberReload(): Reload = remember { Reload() }

/**
 * Pull to refresh: [content], which scrolls, read again by pulling it down,
 * the spinner showing below [top] while [refreshing]. Only on a touch
 * screen and while [enabled]; otherwise it is the content alone.
 */
@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun PullToRefresh(
    refreshing: Boolean,
    onRefresh: () -> Unit,
    modifier: Modifier = Modifier,
    top: Dp = 0.dp,
    enabled: Boolean = true,
    content: @Composable BoxScope.() -> Unit,
) {
    if (!enabled || LocalTelevision.current) {
        Box(modifier.fillMaxSize(), content = content)
        return
    }
    val state = rememberPullToRefreshState()
    PullToRefreshBox(
        refreshing,
        onRefresh,
        modifier.fillMaxSize(),
        state = state,
        indicator = {
            PullToRefreshDefaults.Indicator(
                state = state,
                isRefreshing = refreshing,
                modifier = Modifier.align(Alignment.TopCenter).padding(top = top),
            )
        },
        content = content,
    )
}

/** Pull to refresh of a page read again by [reload]. */
@Composable
fun PullToRefresh(reload: Reload, modifier: Modifier = Modifier, top: Dp = 0.dp, content: @Composable BoxScope.() -> Unit) =
    PullToRefresh(reload.refreshing, reload::pull, modifier, top, content = content)

/** Pull to refresh running [refresh], the spinner showing until it returns. */
@Composable
fun PullToRefresh(
    refresh: suspend () -> Unit,
    modifier: Modifier = Modifier,
    top: Dp = 0.dp,
    enabled: Boolean = true,
    content: @Composable BoxScope.() -> Unit,
) {
    val scope = rememberCoroutineScope()
    val action by rememberUpdatedState(refresh)
    var refreshing by remember { mutableStateOf(false) }
    PullToRefresh(
        refreshing,
        onRefresh = {
            refreshing = true
            scope.launch {
                try {
                    action()
                } finally {
                    refreshing = false
                }
            }
        },
        modifier,
        top,
        enabled,
        content,
    )
}
