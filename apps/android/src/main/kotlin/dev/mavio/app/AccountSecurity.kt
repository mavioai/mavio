package dev.mavio.app

import android.content.Intent
import androidx.compose.foundation.BorderStroke
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.clickable
import androidx.compose.foundation.horizontalScroll
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.navigationBarsPadding
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.text.selection.SelectionContainer
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.ModalBottomSheet
import androidx.compose.material3.Text
import androidx.compose.material3.rememberModalBottomSheetState
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.alpha
import androidx.compose.ui.draw.clip
import androidx.compose.ui.hapticfeedback.HapticFeedbackType
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.platform.LocalFocusManager
import androidx.compose.ui.platform.LocalHapticFeedback
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.semantics.stateDescription
import androidx.compose.ui.text.input.KeyboardType
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import androidx.core.net.toUri
import dev.mavio.admin.Note
import dev.mavio.admin.reason
import dev.mavio.core.AccountSecurity
import dev.mavio.core.AppModel
import dev.mavio.core.ServerRecord
import dev.mavio.core.SwitchableUser
import dev.mavio.core.TotpSetup
import dev.mavio.design.MavioSpacing
import dev.mavio.ui.Avatar
import dev.mavio.ui.ButtonKind
import dev.mavio.ui.Group
import dev.mavio.ui.MavioButton
import dev.mavio.ui.MavioIcon
import dev.mavio.ui.R
import dev.mavio.ui.RowDivider
import dev.mavio.ui.RowLabel
import dev.mavio.ui.SettingsField
import dev.mavio.ui.text
import dev.mavio.ui.theme
import java.time.ZoneId
import java.time.format.DateTimeFormatter
import java.time.format.FormatStyle
import kotlinx.coroutines.CancellationException
import kotlinx.coroutines.launch

// Switching users and Sign-In and Security on a standalone server, as the
// web UI's account settings (docs/accounts.md): the server's users as
// avatars to switch to with their PIN, and the user's password, passkeys,
// authenticator app and PIN.

/**
 * The users of a standalone server as avatars, to switch to with their
 * PIN; shown when there is someone else to switch to.
 */
@OptIn(ExperimentalMaterial3Api::class)
@Composable
internal fun SwitchUserGroup(model: AppModel, server: ServerRecord) {
    var users by remember { mutableStateOf<List<SwitchableUser>>(emptyList()) }
    var asking by remember { mutableStateOf<SwitchableUser?>(null) }
    var busy by remember { mutableStateOf(false) }
    var failure by remember { mutableStateOf<String?>(null) }
    val scope = rememberCoroutineScope()
    LaunchedEffect(server.id, server.userId) {
        users = runCatching { model.client.switchableUsers(server.id) }.getOrDefault(emptyList())
    }
    fun switchTo(user: SwitchableUser, pin: String?) {
        busy = true
        failure = null
        scope.launch {
            try {
                model.switchUser(server.id, user.id, pin)
                asking = null
            } catch (e: CancellationException) {
                throw e
            } catch (e: Exception) {
                failure = e.reason
            }
            busy = false
        }
    }
    if (users.size <= 1) return
    Group(stringResource(R.string.auth_switch_user), failure?.takeIf { asking == null }) {
        Row(
            Modifier.fillMaxWidth().horizontalScroll(rememberScrollState()).padding(horizontal = 16.dp, vertical = 12.dp),
            horizontalArrangement = Arrangement.spacedBy(MavioSpacing.s20),
        ) {
            users.forEach { user ->
                Member(user, Modifier.clickable(enabled = !user.current && !busy) {
                    if (user.hasPin) asking = user else switchTo(user, null)
                })
            }
        }
    }
    asking?.let { user ->
        ModalBottomSheet(
            {
                asking = null
                failure = null
            },
            sheetState = rememberModalBottomSheetState(skipPartiallyExpanded = true),
            containerColor = theme.colors.background,
        ) {
            PinPad(user.name, busy, failure, { switchTo(user, it) }) {
                asking = null
                failure = null
            }
        }
    }
}

/** A user to switch to: its avatar, ringed when it is the current one, with a lock when it has a PIN, and its name. */
@Composable
private fun Member(user: SwitchableUser, modifier: Modifier) {
    Column(modifier.width(68.dp), horizontalAlignment = Alignment.CenterHorizontally, verticalArrangement = Arrangement.spacedBy(6.dp)) {
        Box {
            Box(
                Modifier.border(if (user.current) BorderStroke(2.dp, theme.colors.accent) else BorderStroke(0.dp, theme.colors.background.copy(alpha = 0f)), CircleShape)
                    .padding(3.dp),
            ) {
                Avatar(user.name, size = 56.dp)
            }
            if (user.hasPin) {
                Box(Modifier.align(Alignment.BottomEnd).background(theme.colors.surfaceRaised, CircleShape).padding(4.dp)) {
                    MavioIcon(R.drawable.ic_lock_fill, size = 12.dp)
                }
            }
        }
        Text(user.name, style = theme.type.footnote.text, maxLines = 1)
    }
}

/**
 * Asks for a user's four-digit PIN on a phone's keypad, as the web UI's;
 * the fourth digit sends it, and a wrong one clears the dots.
 */
@Composable
internal fun PinPad(name: String, busy: Boolean, error: String?, enter: (String) -> Unit, cancel: () -> Unit) {
    var pin by remember { mutableStateOf("") }
    val haptics = LocalHapticFeedback.current
    LaunchedEffect(error) {
        if (error != null) {
            pin = ""
            haptics.performHapticFeedback(HapticFeedbackType.Reject)
        }
    }
    fun press(k: String) {
        when (k) {
            "" -> cancel()
            "⌫" -> pin = pin.dropLast(1)
            else -> if (pin.length < 4) {
                pin += k
                if (pin.length == 4) enter(pin)
            }
        }
    }
    val pinLabel = stringResource(R.string.auth_pin)
    Column(
        Modifier.fillMaxWidth().padding(MavioSpacing.s24).navigationBarsPadding(),
        horizontalAlignment = Alignment.CenterHorizontally,
        verticalArrangement = Arrangement.spacedBy(MavioSpacing.s20),
    ) {
        Avatar(name, size = 56.dp)
        Text(stringResource(R.string.auth_enter_pin, name), style = theme.type.title3.text, textAlign = TextAlign.Center)
        Row(
            Modifier.semantics {
                contentDescription = pinLabel
                stateDescription = "${pin.length}/4"
            },
            horizontalArrangement = Arrangement.spacedBy(MavioSpacing.s16),
        ) {
            repeat(4) { i ->
                Box(
                    Modifier.size(12.dp).border(2.dp, theme.colors.textPrimary, CircleShape)
                        .background(if (i < pin.length) theme.colors.textPrimary else theme.colors.background.copy(alpha = 0f), CircleShape),
                )
            }
        }
        Text(error ?: " ", Modifier.alpha(if (error == null) 0f else 1f), style = theme.type.footnote.text, color = theme.colors.danger)
        Column(verticalArrangement = Arrangement.spacedBy(MavioSpacing.s12)) {
            listOf(listOf("1", "2", "3"), listOf("4", "5", "6"), listOf("7", "8", "9"), listOf("", "0", "⌫")).forEach { row ->
                Row(horizontalArrangement = Arrangement.spacedBy(MavioSpacing.s20)) {
                    row.forEach { k ->
                        Box(
                            Modifier.size(72.dp).clip(CircleShape).background(if (k == "⌫") theme.colors.background.copy(alpha = 0f) else theme.colors.fill)
                                .clickable(enabled = !busy || k == "") { press(k) },
                            Alignment.Center,
                        ) {
                            when (k) {
                                // Cancel takes the keypad's empty corner, as on a phone's.
                                "" -> Text(stringResource(R.string.action_cancel), style = theme.type.callout.text)
                                "⌫" -> MavioIcon(R.drawable.ic_backspace, size = 24.dp, contentDescription = stringResource(R.string.search_backspace))
                                else -> Text(k, fontSize = 28.sp)
                            }
                        }
                    }
                }
            }
        }
    }
}

/** A setting's title and state, with its buttons. */
@Composable
private fun SecurityRow(title: String, detail: String, buttons: @Composable () -> Unit) {
    Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(MavioSpacing.s12)) {
        RowLabel(title, detail, Modifier.weight(1f))
        buttons()
    }
}

/** A text button of the security settings. */
@Composable
private fun TextAction(label: String, enabled: Boolean, danger: Boolean = false, quiet: Boolean = false, onClick: () -> Unit) {
    Text(
        label,
        Modifier.clip(CircleShape).clickable(enabled = enabled, onClick = onClick).padding(horizontal = 4.dp, vertical = 6.dp),
        style = theme.type.callout.text,
        color = when {
            !enabled -> theme.colors.textTertiary
            danger -> theme.colors.danger
            quiet -> theme.colors.textSecondary
            else -> theme.colors.accentText
        },
    )
}

/**
 * Sign-In and Security on a standalone server, as the web UI's: the
 * password, the passkeys, the authenticator app and the PIN; a managed
 * user has none of them.
 */
@Composable
internal fun SecurityGroups(model: AppModel, server: ServerRecord) {
    var security by remember { mutableStateOf<AccountSecurity?>(null) }
    var busy by remember { mutableStateOf(false) }
    var failure by remember { mutableStateOf<String?>(null) }
    // What is being changed: "password" or "pin".
    var editing by remember { mutableStateOf<String?>(null) }
    var current by remember { mutableStateOf("") }
    var new by remember { mutableStateOf("") }
    var pin by remember { mutableStateOf("") }
    var code by remember { mutableStateOf("") }
    var totp by remember { mutableStateOf<TotpSetup?>(null) }
    var passwordChanged by remember { mutableStateOf(false) }
    var reload by remember { mutableStateOf(0) }
    val scope = rememberCoroutineScope()
    val focus = LocalFocusManager.current
    val context = LocalContext.current
    LaunchedEffect(server.id, server.userId, reload) {
        try {
            security = model.client.security(server.id)
        } catch (e: CancellationException) {
            throw e
        } catch (e: Exception) {
            failure = e.reason
        }
    }

    /** Runs a change, then reads the user's security again. */
    fun change(f: suspend () -> Unit) {
        focus.clearFocus()
        busy = true
        failure = null
        scope.launch {
            try {
                f()
            } catch (e: CancellationException) {
                throw e
            } catch (e: Exception) {
                failure = e.reason
            }
            busy = false
            reload++
        }
    }
    fun close() {
        editing = null
        current = ""
        new = ""
        pin = ""
    }

    /** Save, which runs f and closes the form when it succeeds, and Cancel. */
    @Composable
    fun Actions(disabled: Boolean, f: suspend () -> Unit) {
        Row(horizontalArrangement = Arrangement.spacedBy(MavioSpacing.s12)) {
            MavioButton(stringResource(R.string.action_save), {
                change {
                    f()
                    close()
                }
            }, enabled = !busy && !disabled)
            MavioButton(stringResource(R.string.action_cancel), ::close, kind = ButtonKind.Secondary)
        }
    }
    val s = security ?: return
    if (s.managed) return
    Column(verticalArrangement = Arrangement.spacedBy(24.dp)) {
        Group(stringResource(R.string.auth_security), failure) {
            val rows = Modifier.fillMaxWidth().padding(horizontal = 16.dp, vertical = 10.dp)
            if (s.hasPassword) {
                Column(rows, verticalArrangement = Arrangement.spacedBy(MavioSpacing.s8)) {
                    SecurityRow(
                        stringResource(R.string.auth_password),
                        stringResource(if (passwordChanged) R.string.auth_password_changed else R.string.auth_password_message),
                    ) {
                        if (editing != "password") {
                            TextAction(stringResource(R.string.auth_change_password), !busy) {
                                passwordChanged = false
                                editing = "password"
                            }
                        }
                    }
                    if (editing == "password") {
                        SettingsField(current, { current = it }, stringResource(R.string.auth_current_password), secret = true)
                        SettingsField(new, { new = it }, stringResource(R.string.auth_new_password), secret = true)
                        Actions(current.isEmpty() || new.isEmpty()) {
                            model.client.setPassword(server.id, current, new)
                            passwordChanged = true
                        }
                    }
                }
                RowDivider()
            }
            Column(rows, verticalArrangement = Arrangement.spacedBy(MavioSpacing.s8)) {
                SecurityRow(stringResource(R.string.auth_totp), stringResource(if (s.hasTotp) R.string.auth_totp_on else R.string.auth_totp_off)) {
                    if (s.hasTotp) {
                        TextAction(stringResource(R.string.auth_turn_off), !busy, danger = true) { change { model.client.deleteTotp(server.id) } }
                    } else if (totp == null) {
                        TextAction(stringResource(R.string.auth_set_up_totp), !busy) { change { totp = model.client.startTotp(server.id) } }
                    }
                }
                val t = totp
                if (t != null && !s.hasTotp) {
                    SelectionContainer {
                        Text(stringResource(R.string.auth_totp_secret, t.secret), style = theme.type.callout.text, color = theme.colors.textSecondary)
                    }
                    TextAction(stringResource(R.string.auth_open_authenticator), true) {
                        runCatching { context.startActivity(Intent(Intent.ACTION_VIEW, t.uri.toUri())) }
                    }
                    SettingsField(
                        code,
                        { code = it.filter(Char::isDigit).take(6) },
                        stringResource(R.string.auth_totp_code),
                        placeholder = "000000",
                        keyboard = KeyboardType.Number,
                    )
                    Row(horizontalArrangement = Arrangement.spacedBy(MavioSpacing.s12)) {
                        MavioButton(stringResource(R.string.action_save), {
                            change {
                                model.client.confirmTotp(server.id, code)
                                totp = null
                                code = ""
                            }
                        }, enabled = !busy && code.length == 6)
                        MavioButton(stringResource(R.string.action_cancel), {
                            totp = null
                            code = ""
                        }, kind = ButtonKind.Secondary)
                    }
                }
            }
            RowDivider()
            Column(rows, verticalArrangement = Arrangement.spacedBy(MavioSpacing.s8)) {
                SecurityRow(
                    stringResource(R.string.auth_pin),
                    "${stringResource(if (s.hasPin) R.string.auth_pin_on else R.string.auth_totp_off)} · ${stringResource(R.string.auth_pin_message)}",
                ) {
                    if (s.hasPin) {
                        TextAction(stringResource(R.string.auth_remove_pin), !busy, danger = true) { change { model.client.setPin(server.id, "") } }
                    }
                    if (editing != "pin") TextAction(stringResource(R.string.auth_set_pin), !busy) { editing = "pin" }
                }
                if (editing == "pin") {
                    SettingsField(
                        pin,
                        { pin = it.filter(Char::isDigit).take(4) },
                        stringResource(R.string.auth_pin),
                        keyboard = KeyboardType.NumberPassword,
                        secret = true,
                    )
                    Actions(pin.length != 4) { model.client.setPin(server.id, pin) }
                }
            }
        }
        Group(stringResource(R.string.auth_passkeys)) {
            if (s.passkeys.isEmpty()) Note(stringResource(R.string.auth_no_passkeys))
            s.passkeys.forEachIndexed { i, k ->
                if (i > 0) RowDivider()
                Row(
                    Modifier.fillMaxWidth().padding(horizontal = 16.dp, vertical = 10.dp),
                    horizontalArrangement = Arrangement.spacedBy(MavioSpacing.s12),
                    verticalAlignment = Alignment.CenterVertically,
                ) {
                    RowLabel(
                        k.name,
                        listOfNotNull(
                            stringResource(R.string.auth_passkey_host, k.host),
                            k.created?.atZone(ZoneId.systemDefault())?.format(DateTimeFormatter.ofLocalizedDate(FormatStyle.MEDIUM)),
                        ).joinToString(" · "),
                        Modifier.weight(1f),
                    )
                    TextAction(stringResource(R.string.action_delete), !busy, danger = true) {
                        change { model.client.deletePasskey(server.id, k.id) }
                    }
                }
            }
        }
    }
}
