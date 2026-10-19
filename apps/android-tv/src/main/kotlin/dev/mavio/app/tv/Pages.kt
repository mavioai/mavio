package dev.mavio.app.tv

import androidx.compose.foundation.lazy.LazyListState
import androidx.compose.foundation.lazy.grid.LazyGridState
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableIntStateOf
import androidx.compose.runtime.mutableStateListOf
import androidx.compose.runtime.mutableStateMapOf
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.setValue
import androidx.compose.runtime.staticCompositionLocalOf
import androidx.compose.runtime.withFrameNanos
import androidx.compose.ui.Modifier
import androidx.compose.ui.focus.FocusDirection
import androidx.compose.ui.focus.FocusRequester
import androidx.compose.ui.focus.focusRequester
import androidx.compose.ui.focus.onFocusChanged
import dev.mavio.admin.AdminPage
import dev.mavio.core.Card
import dev.mavio.core.Row
import dev.mavio.core.SearchResults
import dev.mavio.core.Sort
import dev.mavio.ui.CatalogFilters
import dev.mavio.ui.FocusRecall
import java.time.Duration

/**
 * A page over the tabs, which hides the navigation bar (docs/tv.md §6.2,
 * §6.3, §6.4, §7): an item's details, a library, a catalog of Discover,
 * a discovered title, Search, the settings, or an item's playback. Each keeps where
 * it was, so that back returns to it as it was left.
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

        /** Which of the server's versions plays. */
        var sourceIndex by mutableIntStateOf(0)
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

    /**
     * A catalog of Discover (docs/tv.md §6.3): its titles as they load,
     * of [mediaKind], like the title [tmdbId] when set, narrowed by
     * [filters], as a discovered title's genre or keyword opens it.
     */
    class Catalog(val serverId: String, val catalogId: String, val mediaKind: Int, val tmdbId: Int, val filters: CatalogFilters) : Page() {
        /** Its titles loaded so far. */
        internal val titles = TitlePages()

        /** Its scroll. */
        val grid = LazyGridState()

        /** The element that had focus. */
        val focus = FocusMemory()
    }

    /**
     * An item's whole cast and crew, which the cast's See All opens, or its
     * crew alone, which View Full Crew opens (docs/tv.md §6.2).
     */
    class Credits(val serverId: String, val itemId: String, val crew: Boolean) : Page() {
        /** Its scroll. */
        val grid = LazyGridState()

        /** The element that had focus. */
        val focus = FocusMemory()
    }

    /**
     * A discovered person (docs/tv.md §6.3) by their TMDB ID: the kinds
     * of titles shown, whether the whole biography is, and where it was.
     */
    class Person(val serverId: String, val tmdbId: Int) : Page() {
        /** All titles, movies or series: 0, 1 or 2. */
        var kinds by mutableIntStateOf(0)

        /** Whether the whole biography shows. */
        var more by mutableStateOf(false)

        /** Its scroll. */
        val grid = LazyGridState()

        /** The element that had focus. */
        val focus = FocusMemory()
    }

    /**
     * A discovered title's whole cast and crew, which its cast's See All
     * opens, or its crew alone, which View Full Crew opens (docs/tv.md §6.3).
     */
    class TitleCredits(val serverId: String, val mediaKind: Int, val tmdbId: Int, val crew: Boolean) : Page() {
        /** Its scroll. */
        val grid = LazyGridState()

        /** The element that had focus. */
        val focus = FocusMemory()
    }

    /** A discovered collection (docs/tv.md §6.3) by its TMDB ID. */
    class Collection(val serverId: String, val tmdbId: Int) : Page() {
        /** Its scroll. */
        val grid = LazyGridState()

        /** The element that had focus. */
        val focus = FocusMemory()
    }

    /** A discovered title (docs/tv.md §6.3), a movie or series by its TMDB ID. */
    class Title(val serverId: String, val mediaKind: Int, val tmdbId: Int) : Page() {
        /** The page's scroll. */
        val list = LazyListState()

        /** Its rows' scrolls. */
        val rows = RowStates()

        /** The seasons showing their episodes. */
        val open = mutableStateListOf<Int>()

        /** The element that had focus. */
        val focus = FocusMemory()
    }

    /**
     * The settings (docs/tv.md §6.6): their list, or the [section]
     * selected from it in its place.
     */
    class Settings : Page() {
        /** The section shown, or null for the list. */
        var section by mutableStateOf<SettingsSection?>(null)

        /** The element of the list that had focus. */
        val focus = FocusMemory()

        /** Whether the user administers each server, by server and user, for the Server group. */
        val admins = mutableStateMapOf<String, Boolean>()
    }

    /** Connecting to a server and signing in to it, from the account (docs/tv.md §6.5). */
    class AddServer : Page()

    /**
     * A page of a server's administration, from the settings' Server group,
     * its state kept under [id] while pages open over it.
     */
    class Admin(val page: AdminPage, val id: Int) : Page() {
        /** Where focus was, for its return. */
        val recall = FocusRecall()
    }

    /** An item's playback, from [start] when set. */
    class Player(val serverId: String, val itemId: String, val title: String, val mediaSourceId: String?, val start: Duration?) : Page()
}

/** The pages opened over the tabs, the last on top. */
class Pages {
    private val stack = mutableStateListOf<Page>()
    private var admins = 0

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

    /** Opens the item [itemId] of [serverId]. */
    fun openItem(serverId: String, itemId: String) {
        stack += Page.Details(serverId, itemId)
    }

    /** Opens an item's whole cast and crew, or its [crew] alone. */
    fun openCredits(serverId: String, itemId: String, crew: Boolean) {
        stack += Page.Credits(serverId, itemId, crew)
    }

    /** Opens a catalog of Discover. */
    fun openCatalog(serverId: String, catalogId: String, mediaKind: Int, tmdbId: Int = 0, filters: CatalogFilters = CatalogFilters()) {
        stack += Page.Catalog(serverId, catalogId, mediaKind, tmdbId, filters)
    }

    /** Opens a discovered title. */
    fun openTitle(serverId: String, mediaKind: Int, tmdbId: Int) {
        stack += Page.Title(serverId, mediaKind, tmdbId)
    }

    /** Opens a discovered person. */
    fun openPerson(serverId: String, tmdbId: Int) {
        stack += Page.Person(serverId, tmdbId)
    }

    /** Opens a discovered title's whole cast and crew, or its [crew] alone. */
    fun openTitleCredits(serverId: String, mediaKind: Int, tmdbId: Int, crew: Boolean) {
        stack += Page.TitleCredits(serverId, mediaKind, tmdbId, crew)
    }

    /** Opens a discovered collection. */
    fun openCollection(serverId: String, tmdbId: Int) {
        stack += Page.Collection(serverId, tmdbId)
    }

    /** Opens Search. */
    fun openSearch() {
        stack += Page.Search()
    }

    /** Opens connecting to a server. */
    fun openAddServer() {
        stack += Page.AddServer()
    }

    /** Opens a page of a server's administration. */
    fun openAdmin(page: AdminPage) {
        stack += Page.Admin(page, ++admins)
    }

    /** Opens the settings. */
    fun openSettings() {
        stack += Page.Settings()
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
