package dev.mavio.app

import androidx.annotation.DrawableRes
import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.ColumnScope
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxHeight
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.navigationBarsPadding
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.statusBarsPadding
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateMapOf
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.draw.rotate
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import dev.mavio.admin.AdminPage
import dev.mavio.admin.Procedure
import dev.mavio.admin.ServerMenu
import dev.mavio.admin.ServerSection
import dev.mavio.admin.serverEntries
import dev.mavio.core.AppModel
import dev.mavio.core.call
import dev.mavio.design.MavioSpacing
import dev.mavio.ui.AccountCard
import dev.mavio.ui.ChoiceRow
import dev.mavio.ui.MavioIcon
import dev.mavio.ui.R
import dev.mavio.ui.signedInServers
import dev.mavio.ui.text
import dev.mavio.ui.theme
import mavio.user.v1.UserOuterClass.GetCurrentUserResponse
import mavio.user.v1.getCurrentUserRequest

// The settings' own sidebar in the wide layout (docs/clients.md §6.3), as
// the web UI's: Home, the title, the account's card, This Device and the
// Server group, whose menus open in place while they hold the section
// shown. The section shows beside it.

/** The section the wide layout shows when the settings open. */
internal val FIRST_SETTING: Route = Route.Setting(SettingsPage.Appearance)

/**
 * The settings' sidebar: [section] is the section shown, chosen with
 * [onSelect]; [onHome] leaves the settings.
 */
@Composable
internal fun SettingsSidebar(model: AppModel, section: Route?, modifier: Modifier = Modifier, onHome: () -> Unit, onSelect: (Route) -> Unit) {
    val colors = theme.colors
    val signedIn by model.signedIn.collectAsStateWithLifecycle()
    val servers = signedInServers(model)
    Column(
        modifier
            .fillMaxHeight()
            .background(colors.sidebar)
            .statusBarsPadding()
            .navigationBarsPadding()
            .verticalScroll(rememberScrollState())
            .padding(horizontal = MavioSpacing.s16, vertical = MavioSpacing.s20),
        verticalArrangement = Arrangement.spacedBy(MavioSpacing.s16),
    ) {
        Row(
            Modifier.clip(RoundedCornerShape(8.dp)).clickable(onClick = onHome).padding(vertical = 4.dp, horizontal = 2.dp),
            horizontalArrangement = Arrangement.spacedBy(4.dp),
            verticalAlignment = Alignment.CenterVertically,
        ) {
            MavioIcon(R.drawable.ic_back, size = 16.dp, tint = colors.accentText)
            Text(stringResource(R.string.nav_home), style = theme.type.callout.text, color = colors.accentText)
        }
        Text(stringResource(R.string.nav_settings), style = theme.type.title1.text, fontWeight = FontWeight.Bold)
        val account = Route.Setting(SettingsPage.Account)
        AccountCard(
            model,
            Modifier
                .clip(RoundedCornerShape(14.dp))
                .background(if (section == account) colors.fillSelected else colors.surface)
                .clickable { onSelect(account) },
            chevron = false,
        )
        Heading(stringResource(R.string.settings_this_device)) {
            Link(R.string.settings_appearance, R.drawable.ic_appearance_fill, Color(0xFF5E5CE6), Route.Setting(SettingsPage.Appearance), section, onSelect)
            Link(R.string.settings_playback, R.drawable.ic_play_fill, Color(0xFFFF3B30), Route.Setting(SettingsPage.Playback), section, onSelect)
            if (signedIn.isNotEmpty()) {
                Link(R.string.settings_subtitles_audio, R.drawable.ic_subtitles_fill, Color(0xFF34C759), Route.Setting(SettingsPage.SubtitlesAudio), section, onSelect)
            }
        }
        if (servers.isNotEmpty()) {
            // The server whose sections show: the section's, else the first.
            val shownId = ((section as? Route.Admin)?.page as? AdminPage.ServerSetting)?.serverId
            var serverId by rememberSaveable { mutableStateOf(shownId ?: servers.first().id) }
            LaunchedEffect(shownId) { shownId?.let { serverId = it } }
            val server = servers.firstOrNull { it.id == serverId } ?: servers.first()
            // Whether the user administers each server, once known, by
            // server and user, as switching users changes it.
            val admin = remember { mutableStateMapOf<String, Boolean>() }
            val adminKey = "${server.id}/${server.userId}"
            LaunchedEffect(adminKey) {
                runCatching {
                    model.client.call(server.id, Procedure.users("GetCurrentUser"), getCurrentUserRequest {}, GetCurrentUserResponse::parseFrom)
                }.onSuccess { admin[adminKey] = it.user.admin }
            }
            Heading(stringResource(R.string.settings_server_group)) {
                if (servers.size > 1) {
                    Box(Modifier.padding(bottom = 4.dp).clip(RoundedCornerShape(10.dp)).background(colors.surface)) {
                        ChoiceRow(stringResource(R.string.auth_server), servers.map { it.id to it.name }, server.id) { serverId = it }
                    }
                }
                serverEntries(admin[adminKey] == true).forEach { entry ->
                    when (entry) {
                        is ServerSection -> Link(entry.title, entry.icon, entry.color, Route.Admin(AdminPage.ServerSetting(server.id, entry)), section, onSelect)
                        is ServerMenu -> Menu(entry, server.id, section, onSelect)
                    }
                }
            }
        }
    }
}

/** A group of the sidebar's links under its heading. */
@Composable
private fun Heading(title: String, content: @Composable ColumnScope.() -> Unit) {
    Column(verticalArrangement = Arrangement.spacedBy(2.dp)) {
        Text(
            title,
            Modifier.padding(start = 8.dp, bottom = 4.dp),
            style = theme.type.caption.text,
            fontWeight = FontWeight.SemiBold,
            color = theme.colors.textTertiary,
        )
        content()
    }
}

/** A link to a section: its icon on a colored tile and its title, chosen while it shows. */
@Composable
private fun Link(title: Int, @DrawableRes icon: Int, color: Color, route: Route, section: Route?, onSelect: (Route) -> Unit) {
    Row(
        Modifier
            .fillMaxWidth()
            .heightIn(min = 40.dp)
            .clip(RoundedCornerShape(10.dp))
            .background(if (route == section) theme.colors.fillSelected else theme.colors.sidebar)
            .clickable { onSelect(route) }
            .padding(horizontal = 6.dp),
        horizontalArrangement = Arrangement.spacedBy(10.dp),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        Box(Modifier.size(28.dp).clip(RoundedCornerShape(7.dp)).background(color), Alignment.Center) {
            MavioIcon(icon, size = 17.dp, tint = Color.White)
        }
        Text(stringResource(title), style = theme.type.body.text, maxLines = 1, overflow = TextOverflow.Ellipsis)
    }
}

/** A menu of sections, opening in place; open while it holds the section shown. */
@Composable
private fun Menu(menu: ServerMenu, serverId: String, section: Route?, onSelect: (Route) -> Unit) {
    val shown = (section as? Route.Admin)?.page as? AdminPage.ServerSetting
    val holds = shown != null && shown.serverId == serverId && shown.section in menu.sections
    var open by rememberSaveable(menu) { mutableStateOf(holds) }
    LaunchedEffect(holds) { if (holds) open = true }
    Row(
        Modifier
            .fillMaxWidth()
            .heightIn(min = 40.dp)
            .clip(RoundedCornerShape(10.dp))
            .clickable { open = !open }
            .padding(horizontal = 6.dp),
        horizontalArrangement = Arrangement.spacedBy(10.dp),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        Box(Modifier.size(28.dp).clip(RoundedCornerShape(7.dp)).background(menu.color), Alignment.Center) {
            MavioIcon(menu.icon, size = 17.dp, tint = Color.White)
        }
        Text(stringResource(menu.title), Modifier.weight(1f), style = theme.type.body.text, maxLines = 1, overflow = TextOverflow.Ellipsis)
        MavioIcon(R.drawable.ic_forward, Modifier.rotate(if (open) 90f else 0f), size = 14.dp, tint = theme.colors.textTertiary)
    }
    if (open) {
        Column(Modifier.padding(start = 24.dp), verticalArrangement = Arrangement.spacedBy(2.dp)) {
            menu.sections.forEach { s -> Link(s.title, s.icon, s.color, Route.Admin(AdminPage.ServerSetting(serverId, s)), section, onSelect) }
        }
    }
}
