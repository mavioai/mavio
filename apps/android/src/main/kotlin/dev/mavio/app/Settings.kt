package dev.mavio.app

import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.HorizontalDivider
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import dev.mavio.core.AppModel
import dev.mavio.core.SignInState
import dev.mavio.core.StreamState
import dev.mavio.design.MavioSpacing
import dev.mavio.ui.ButtonKind
import dev.mavio.ui.MavioButton
import dev.mavio.ui.MavioIcon
import dev.mavio.ui.R
import dev.mavio.ui.text
import dev.mavio.ui.theme
import kotlinx.coroutines.launch

/** The settings: who is signed in, the servers and signing out. */
@Composable
fun SettingsScreen(model: AppModel) {
    val state by model.signInState.collectAsStateWithLifecycle()
    val servers by model.servers.collectAsStateWithLifecycle()
    val states by model.states.collectAsStateWithLifecycle()
    val scope = rememberCoroutineScope()
    val account = (state as? SignInState.Account)?.v1
    val name = account?.name ?: servers.firstOrNull()?.userName.orEmpty()
    LazyColumn(
        Modifier.fillMaxSize(),
        contentPadding = PaddingValues(
            start = theme.layout.margin,
            end = theme.layout.margin,
            top = MavioSpacing.s16,
            bottom = LocalBottomInset.current,
        ),
        verticalArrangement = Arrangement.spacedBy(MavioSpacing.s24),
    ) {
        item {
            Text(stringResource(R.string.nav_settings), style = theme.type.title1.text, fontWeight = FontWeight.Bold)
        }
        item {
            Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(MavioSpacing.s12)) {
                Avatar(name, size = 56.dp)
                Column {
                    Text(name, style = theme.type.title3.text)
                    Text(
                        stringResource(if (account != null) R.string.auth_mavio_account else R.string.auth_standalone_server),
                        style = theme.type.callout.text,
                        color = theme.colors.textSecondary,
                    )
                }
            }
        }
        item {
            Column(verticalArrangement = Arrangement.spacedBy(MavioSpacing.s8)) {
                Text(stringResource(R.string.auth_servers), style = theme.type.headline.text)
                Column(Modifier.fillMaxWidth().background(theme.colors.surface, RoundedCornerShape(14.dp))) {
                    servers.forEachIndexed { i, server ->
                        if (i > 0) HorizontalDivider(Modifier.padding(start = 48.dp), color = theme.colors.border)
                        Row(
                            Modifier.padding(horizontal = 14.dp, vertical = 10.dp),
                            verticalAlignment = Alignment.CenterVertically,
                            horizontalArrangement = Arrangement.spacedBy(12.dp),
                        ) {
                            MavioIcon(R.drawable.ic_server, size = 22.dp, tint = theme.colors.textSecondary)
                            Column(Modifier.weight(1f)) {
                                Text(server.name, style = theme.type.headline.text, maxLines = 1)
                                val line = when (states[server.id]) {
                                    is StreamState.Offline -> stringResource(R.string.auth_unreachable)
                                    is StreamState.SignedOut -> stringResource(R.string.auth_signed_out)
                                    else -> stringResource(R.string.auth_signed_in_as, server.userName)
                                }
                                Text(line, style = theme.type.footnote.text, color = theme.colors.textSecondary, maxLines = 1)
                            }
                        }
                    }
                }
            }
        }
        item {
            MavioButton(
                stringResource(R.string.auth_sign_out),
                { scope.launch { runCatching { model.signOut() } } },
                Modifier.fillMaxWidth(),
                kind = ButtonKind.Secondary,
                icon = R.drawable.ic_sign_out,
                large = true,
            )
        }
    }
}
