package dev.mavio.app.tv

import androidx.compose.foundation.lazy.LazyListState
import androidx.compose.foundation.lazy.grid.LazyGridState
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.setValue
import androidx.compose.runtime.staticCompositionLocalOf
import androidx.compose.runtime.mutableStateListOf
import androidx.compose.runtime.withFrameNanos
import androidx.compose.ui.Modifier
import androidx.compose.ui.focus.FocusDirection
import androidx.compose.ui.focus.FocusRequester
import androidx.compose.ui.focus.focusRequester
import androidx.compose.ui.focus.onFocusChanged
import dev.mavio.core.Card
import dev.mavio.core.Row
import dev.mavio.core.SearchResults
import dev.mavio.core.Sort
import java.time.Duration

/**
 * A page over the tabs, which hides the navigation bar (docs/tv.md §6.2,
 * §6.3, §6.4, §7): an item's details, a library, Search, or an item's
 * playback. Each keeps where it was, so that back returns to it as it was
 * left.
 */
sealed class Page {
    /** An item's details. */
    class Details(val serverId: String, val itemId: String) : Page() {
        /** The page's scroll. */
        val list = LazyListState()

        /** Its rows' scrolls. */
        val rows = RowStates()

        /** The element that had focus. */
        val focus = FocusMemory()
    }

    /**
     * A library (docs/tv.md §6.3): its Recommended or All, All's filter
     * and order, and where each was.
     */
    class Library(val serverId: String, val libraryId: String, val name: String) : Page() {
        /** Whether All shows rather than Recommended. */
        var all by mutableStateOf(false)

        /** The items All shows. */
        var shown by mutableStateOf(Shown.ALL)

        /** The order of All's items. */
        var sort by mutableStateOf(Sort.NAME)

        /** Recommended's rows. */
        var recommended by mutableStateOf<Result<List<Row>>?>(null)

        /** Recommended's scroll. */
        val list = LazyListState()

        /** Recommended's rows' scrolls. */
        val rows = RowStates()

        /** All's scroll. */
        val grid = LazyGridState()

        /** All's items. */
        internal var items: LibraryItems? = null

        /** The element that had focus. */
        val focus = FocusMemory()
    }

    /** Search (docs/tv.md §6.4): what is typed, the query searched, and its results. */
    class Search : Page() {
        /** What the keyboard typed. */
        var text by mutableStateOf("")

        /** What is searched, [text] after a pause in typing. */
        var query by mutableStateOf("")

        /** The results of a query. */
        var results by mutableStateOf<Pair<String, Result<List<SearchResults>>>?>(null)

        /** The suggestions' scroll. */
        val list = LazyListState()

        /** The suggestions' rows' scrolls. */
        val rows = RowStates()

        /** The results' scroll. */
        val grid = LazyGridState()

        /** The element that had focus. */
        val focus = FocusMemory()
    }

    /** An item's playback, from [start] when set. */
    class Player(val serverId: String, val itemId: String, val title: String, val mediaSourceId: String?, val start: Duration?) : Page()
}

/** The pages opened over the tabs, the last on top. */
class Pages {
    private val stack = mutableStateListOf<Page>()

    /** The page on top, if any. */
    val top: Page? get() = stack.lastOrNull()

    /** The pages, the last on top. */
    val all: List<Page> get() = stack

    /** Opens [card]'s details. */
    fun open(card: Card) {
        stack += Page.Details(card.serverId, card.itemId)
    }

    /** Opens the library [libraryId] of [serverId], named [name]. */
    fun openLibrary(serverId: String, libraryId: String, name: String) {
        stack += Page.Library(serverId, libraryId, name)
    }

    /** Opens Search. */
    fun openSearch() {
        stack += Page.Search()
    }

    /** Plays an item, from [start] when set. */
    fun play(serverId: String, itemId: String, title: String, mediaSourceId: String? = null, start: Duration? = null) {
        stack += Page.Player(serverId, itemId, title, mediaSourceId, start)
    }

    /** Replaces the page on top with [page], as the next episode does its player. */
    fun replace(page: Page) {
        if (stack.isNotEmpty()) stack.removeAt(stack.lastIndex)
        stack += page
    }

    /** Closes the page on top. */
    fun back() {
        if (stack.isNotEmpty()) stack.removeAt(stack.lastIndex)
    }
}

/** The pages of the app, which its screens open. */
val LocalPages = staticCompositionLocalOf<Pages> { error("no pages") }

/**
 * The element of a page that had focus, for back to return to it: each
 * focusable registers under a key with [focusKey].
 */
class FocusMemory {
    private val requesters = mutableMapOf<String, FocusRequester>()

    /** The key of the element that had focus last. */
    var last: String? = null

    /** The requester of the element [key]. */
    fun of(key: String): FocusRequester = requesters.getOrPut(key) { FocusRequester() }

    /**
     * Focuses the element that had focus last, once it is laid out, else
     * [fallback]; returns whether either took focus.
     */
    suspend fun restore(fallback: FocusRequester? = null): Boolean {
        val target = last?.let { of(it) } ?: fallback ?: return false
        // Rows lay their cards out a frame or two after the page.
        repeat(5) {
            if (runCatching { target.requestFocus(FocusDirection.Enter) }.getOrDefault(false)) return true
            withFrameNanos {}
        }
        return fallback != null && target != fallback && runCatching { fallback.requestFocus(FocusDirection.Enter) }.getOrDefault(false)
    }
}

/** Registers the focusable this modifies, put before it, under [key] in [memory]. */
fun Modifier.focusKey(memory: FocusMemory, key: String): Modifier =
    focusRequester(memory.of(key)).onFocusChanged { if (it.isFocused) memory.last = key }
