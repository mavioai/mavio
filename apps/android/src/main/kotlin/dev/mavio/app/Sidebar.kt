package dev.mavio.app

import androidx.annotation.DrawableRes
import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxHeight
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.navigationBarsPadding
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.statusBarsPadding
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateMapOf
import androidx.compose.runtime.remember
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import dev.mavio.core.AppModel
import dev.mavio.core.LibraryEntry
import dev.mavio.core.LibraryType
import dev.mavio.core.SignInState
import dev.mavio.design.MavioSpacing
import dev.mavio.ui.MavioIcon
import dev.mavio.ui.R
import dev.mavio.ui.Wordmark
import dev.mavio.ui.text
import dev.mavio.ui.theme

// The sidebar of the wide layout (docs/clients.md §6.3), as the web UI's:
// the wordmark, Search, Home, Discover while a server discovers, the
// libraries of the servers signed in to, and the user at its foot, opening
// the settings. Collapsed, it is a rail of icons.

/** The icon of a library of a kind. */
@DrawableRes
internal fun libraryIcon(kind: LibraryType) = when (kind) {
    LibraryType.MOVIES -> R.drawable.ic_movies
    LibraryType.SHOWS -> R.drawable.ic_shows
    LibraryType.MUSIC -> R.drawable.ic_music
    LibraryType.MUSIC_VIDEOS -> R.drawable.ic_music
    LibraryType.HOME_VIDEOS -> R.drawable.ic_home_videos
    LibraryType.BOOKS -> R.drawable.ic_books
    LibraryType.PHOTOS -> R.drawable.ic_photos
    LibraryType.COLLECTIONS -> R.drawable.ic_collections
    LibraryType.PLAYLISTS -> R.drawable.ic_playlists
    LibraryType.MIXED -> R.drawable.ic_folder
}

/** A library's place in the sidebar. */
internal fun libraryPlace(library: LibraryEntry) = Route.Library(library.serverId, library.id, library.name)

/**
 * The sidebar's panel, expanded or as a rail; [place] is the place shown,
 * chosen with [onSelect], and [onToggle] expands or collapses it.
 */
@Composable
internal fun SidebarPanel(
    model: AppModel,
    rail: Boolean,
    place: Route,
    discovers: Boolean,
    modifier: Modifier = Modifier,
    onToggle: () -> Unit,
    onSelect: (Route) -> Unit,
) {
    val colors = theme.colors
    val servers by model.servers.collectAsStateWithLifecycle()
    val signedIn by model.signedIn.collectAsStateWithLifecycle()
    val revisions by model.revisions.collectAsStateWithLifecycle()
    val state by model.signInState.collectAsStateWithLifecycle()
    val shown = servers.filter { it.id in signedIn }
    // Each server's libraries, as they come.
    val libraries = remember { mutableStateMapOf<String, List<LibraryEntry>>() }
    shown.forEach { server ->
        LaunchedEffect(server.id, revisions[server.id]) {
            runCatching { model.client.libraries(server.id) }.onSuccess { libraries[server.id] = it.libraries }
        }
    }
    val several = shown.size > 1
    val name = (state as? SignInState.Account)?.v1?.name ?: shown.firstOrNull()?.userName.orEmpty()
    Column(
        modifier
            .fillMaxHeight()
            .background(colors.sidebar)
            .statusBarsPadding()
            .navigationBarsPadding(),
    ) {
        Row(
            Modifier.fillMaxWidth().height(56.dp).padding(start = if (rail) 0.dp else 16.dp, end = if (rail) 0.dp else 10.dp),
            horizontalArrangement = if (rail) Arrangement.Center else Arrangement.SpaceBetween,
            verticalAlignment = Alignment.CenterVertically,
        ) {
            if (!rail) Wordmark(height = 22.dp)
            val toggle = stringResource(if (rail) R.string.nav_expand_sidebar else R.string.nav_collapse_sidebar)
            Box(
                Modifier.size(36.dp).clip(RoundedCornerShape(10.dp)).clickable(onClick = onToggle).semantics { contentDescription = toggle },
                Alignment.Center,
            ) {
                MavioIcon(R.drawable.ic_sidebar, size = 18.dp, tint = colors.textSecondary)
            }
        }
        Column(
            Modifier
                .weight(1f)
                .fillMaxWidth()
                .verticalScroll(rememberScrollState())
                .padding(horizontal = if (rail) 8.dp else 10.dp)
                .padding(bottom = MavioSpacing.s8),
            verticalArrangement = Arrangement.spacedBy(if (rail) 8.dp else 2.dp),
            horizontalAlignment = if (rail) Alignment.CenterHorizontally else Alignment.Start,
        ) {
            NavItem(rail, R.drawable.ic_search, R.drawable.ic_search_fill, stringResource(R.string.nav_search), place == Route.Search) { onSelect(Route.Search) }
            NavItem(rail, R.drawable.ic_home, R.drawable.ic_home_fill, stringResource(R.string.nav_home), place == Route.Home) { onSelect(Route.Home) }
            if (discovers) {
                NavItem(rail, R.drawable.ic_discover, R.drawable.ic_discover_fill, stringResource(R.string.nav_discover), place == Route.Discover) {
                    onSelect(Route.Discover)
                }
            }
            Caption(rail, stringResource(R.string.nav_libraries))
            shown.forEach { server ->
                val list = libraries[server.id].orEmpty()
                if (several && list.isNotEmpty() && !rail) {
                    Text(
                        server.name,
                        Modifier.padding(start = 10.dp, top = 8.dp, bottom = 2.dp),
                        style = theme.type.caption.text,
                        color = colors.textTertiary,
                        maxLines = 1,
                        overflow = TextOverflow.Ellipsis,
                    )
                }
                list.forEach { library ->
                    val p = libraryPlace(library)
                    NavItem(rail, libraryIcon(library.kind), null, library.name, place == p, iconTint = colors.accentText) { onSelect(p) }
                }
            }
        }
        Box(Modifier.fillMaxWidth().height(1.dp).background(colors.border))
        val user = stringResource(R.string.nav_user_menu, name)
        Row(
            Modifier
                .fillMaxWidth()
                .padding(if (rail) 8.dp else 6.dp)
                .clip(RoundedCornerShape(10.dp))
                .background(if (place == Route.Settings) colors.fillSelected else colors.sidebar)
                .clickable { onSelect(Route.Settings) }
                .semantics { contentDescription = user }
                .padding(6.dp),
            horizontalArrangement = if (rail) Arrangement.Center else Arrangement.spacedBy(10.dp),
            verticalAlignment = Alignment.CenterVertically,
        ) {
            Avatar(name)
            if (!rail) {
                Text(name, Modifier.weight(1f), style = theme.type.callout.text, fontWeight = FontWeight.SemiBold, maxLines = 1, overflow = TextOverflow.Ellipsis)
            }
        }
    }
}

/** A heading of the sidebar's places, or a rule between them on the rail. */
@Composable
private fun Caption(rail: Boolean, text: String) {
    if (rail) {
        Box(Modifier.padding(vertical = 4.dp).width(32.dp).height(1.dp).background(theme.colors.border))
    } else {
        Text(
            text,
            Modifier.padding(start = 10.dp, top = 16.dp, bottom = 4.dp),
            style = theme.type.caption.text,
            fontWeight = FontWeight.SemiBold,
            color = theme.colors.textTertiary,
        )
    }
}

/** A place of the sidebar: its icon, filled while chosen, and on the expanded sidebar its name. */
@Composable
private fun NavItem(
    rail: Boolean,
    @DrawableRes icon: Int,
    @DrawableRes filled: Int?,
    label: String,
    selected: Boolean,
    iconTint: androidx.compose.ui.graphics.Color? = null,
    onClick: () -> Unit,
) {
    val colors = theme.colors
    val shape = RoundedCornerShape(10.dp)
    Row(
        Modifier
            .then(if (rail) Modifier.size(44.dp) else Modifier.fillMaxWidth().heightIn(min = 40.dp))
            .clip(shape)
            .background(if (selected) colors.fillSelected else colors.sidebar)
            .clickable(onClick = onClick)
            .then(if (rail) Modifier.semantics { contentDescription = label } else Modifier.padding(horizontal = 10.dp)),
        horizontalArrangement = if (rail) Arrangement.Center else Arrangement.spacedBy(10.dp),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        MavioIcon(if (selected && filled != null) filled else icon, size = 20.dp, tint = iconTint ?: colors.textPrimary)
        if (!rail) {
            Text(
                label,
                style = theme.type.callout.text,
                fontWeight = if (selected) FontWeight.SemiBold else null,
                maxLines = 1,
                overflow = TextOverflow.Ellipsis,
            )
            Spacer(Modifier.weight(1f))
        }
    }
}
