package dev.mavio.app.tv

import android.util.Log
import androidx.activity.compose.BackHandler
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.widthIn
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.key
import androidx.compose.runtime.mutableIntStateOf
import androidx.compose.runtime.mutableStateListOf
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.setValue
import androidx.compose.runtime.withFrameNanos
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.alpha
import androidx.compose.ui.draw.shadow
import androidx.compose.ui.focus.FocusRequester
import androidx.compose.ui.focus.focusRequester
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.luminance
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.input.KeyboardType
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import dev.mavio.core.AccountMode
import dev.mavio.core.AppModel
import dev.mavio.core.CoreException
import dev.mavio.core.FoundServer
import dev.mavio.core.LinkCode
import dev.mavio.core.QuickConnectRequest
import dev.mavio.core.ServerInfo
import dev.mavio.core.isUnauthenticated
import dev.mavio.design.MavioSpacing
import dev.mavio.ui.Group
import dev.mavio.ui.PosterWall
import dev.mavio.ui.Pressable
import dev.mavio.ui.QRCodeOf
import dev.mavio.ui.R
import dev.mavio.ui.RowDivider
import dev.mavio.ui.SettingsField
import dev.mavio.ui.TvButton
import dev.mavio.ui.ValueRow
import dev.mavio.ui.Wordmark
import dev.mavio.ui.theme
import java.time.Duration
import java.time.Instant
import kotlin.coroutines.cancellation.CancellationException
import kotlinx.coroutines.delay
import kotlinx.coroutines.launch

/** The client ID of the Android apps at mavio.dev. */
private const val CLIENT_ID = "dev.mavio.app"

/** A step of signing in. */
private sealed interface Step {
    /** A link code approved at mavio.dev/link, or at the mavio.dev of the server connected to. */
    data object Link : Step

    /** A server's address, typed or found on the network. */
    data object Address : Step

    /** A standalone server's user: Quick Connect, or a name and password. */
    data class Credentials(val info: ServerInfo) : Step

    /** A Quick Connect code entered on a device signed in. */
    data class QuickConnect(val info: ServerInfo, val request: QuickConnectRequest) : Step

    /** A server not set up yet, which a computer or phone sets up. */
    data class SetUp(val info: ServerInfo) : Step
}

/**
 * Signing in on a television (docs/tv.md §6.5): a Mavio account, with a
 * code to enter at mavio.dev/link on another device, polled until it is
 * approved and replaced as it expires; or a server's address, which signs
 * a standalone server's user in with Quick Connect or a password, sends a
 * server in mavio.dev mode back to the code, at its mavio.dev, and shows a
 * new server's address to set it up elsewhere. Over the wall of posters,
 * in a card of glass, as the web UI's television layout. [adding] starts
 * at the address, to add a server from the settings, [onDone] leaving.
 */
@Composable
fun SignInScreen(model: AppModel, adding: Boolean = false, onDone: () -> Unit = {}) {
    val history = remember { mutableStateListOf<Step>() }
    var step by remember { mutableStateOf<Step>(if (adding) Step.Address else Step.Link) }
    // The server connected to, whose mavio.dev a Mavio account signs in at.
    var server by remember { mutableStateOf<ServerInfo?>(null) }
    var error by remember { mutableStateOf<String?>(null) }
    var busy by remember { mutableStateOf(false) }
    val scope = rememberCoroutineScope()
    val notMavio = stringResource(R.string.auth_not_mavio)
    val wrongPassword = stringResource(R.string.auth_wrong_password)
    val unreachable = stringResource(R.string.auth_unreachable)

    fun goTo(next: Step) {
        history.add(step)
        step = next
        error = null
    }

    fun back() {
        step = history.removeLastOrNull() ?: return
        if (step == Step.Link || step == Step.Address) server = null
        error = null
    }

    fun run(body: suspend () -> Unit) {
        busy = true
        error = null
        scope.launch {
            try {
                body()
            } catch (e: CancellationException) {
                throw e
            } catch (e: Exception) {
                Log.w("Mavio", "sign-in", e)
                error = when {
                    e.isUnauthenticated -> wrongPassword
                    e is CoreException.Unreachable -> unreachable
                    e is CoreException && "not a Mavio" in (e.message ?: "") -> notMavio
                    else -> e.message ?: e.toString()
                }
            } finally {
                busy = false
            }
        }
    }

    fun probe(address: String) = run {
        val info = model.client.probe(address.trim())
        server = info
        goTo(
            when {
                info.mode == AccountMode.MAVIO_DEV -> Step.Link
                info.setupRequired -> Step.SetUp(info)
                else -> Step.Credentials(info)
            },
        )
    }

    BackHandler(history.isNotEmpty() || adding) { if (history.isNotEmpty()) back() else onDone() }

    val dark = theme.colors.background.luminance() < 0.5f
    Box(Modifier.fillMaxSize().background(theme.colors.background)) {
        PosterWall(tv = true)
        Wordmark(Modifier.padding(start = 120.dp, top = 90.dp), height = 60.dp)
        Box(Modifier.fillMaxSize().padding(horizontal = 120.dp, vertical = 90.dp), Alignment.Center) {
            val card = RoundedCornerShape(44.dp)
            Column(
                Modifier
                    .widthIn(max = 1040.dp)
                    .fillMaxWidth()
                    .shadow(80.dp, card, ambientColor = Color.Black.copy(alpha = 0.5f), spotColor = Color.Black.copy(alpha = 0.5f))
                    .background(if (dark) Color(0xD11C1C1E) else Color(0xD6FFFFFF), card)
                    .border(1.dp, if (dark) Color(0x80545458) else Color(0x243C3C43), card)
                    .padding(horizontal = 90.dp, vertical = 80.dp),
                verticalArrangement = Arrangement.spacedBy(MavioSpacing.s32),
            ) {
                // Each step's first element takes focus as it shows.
                key(step) {
                    when (val s = step) {
                        Step.Link -> LinkStep(model, server, onDone) { goTo(Step.Address) }
                        Step.Address -> AddressStep(model, busy, error, ::probe)
                        is Step.Credentials -> CredentialsStep(
                            s.info,
                            busy,
                            error,
                            onSignIn = { name, password ->
                                run {
                                    model.signIn(s.info.address, name, password)
                                    onDone()
                                }
                            },
                            onQuickConnect = {
                                run { goTo(Step.QuickConnect(s.info, model.client.startQuickConnect(s.info.address))) }
                            },
                        )
                        is Step.QuickConnect -> QuickConnectStep(model, s.request, onDone) {
                            back()
                            error = it
                        }
                        is Step.SetUp -> SetUpStep(s.info)
                    }
                }
            }
        }
    }
}

/** The title of a step, as the card's. */
@Composable
private fun Heading(text: String, eyebrow: String? = null) {
    Column(verticalArrangement = Arrangement.spacedBy(MavioSpacing.s8)) {
        eyebrow?.let { Text(it.uppercase(), fontSize = 22.sp, fontWeight = FontWeight.SemiBold, color = theme.colors.textTertiary) }
        Text(text, fontSize = 54.sp, lineHeight = 62.sp, fontWeight = FontWeight.Bold)
    }
}

/** The text of a step. */
@Composable
private fun Body(text: String, color: Color = theme.colors.textSecondary) {
    Text(text, fontSize = 26.sp, lineHeight = 36.sp, color = color)
}

/** A button of the card, as large as the card's type, [primary] filled with `color.button`. */
@Composable
private fun CardButton(label: String, onClick: () -> Unit, modifier: Modifier = Modifier, primary: Boolean = false, enabled: Boolean = true) {
    TvButton(
        label,
        { if (enabled) onClick() },
        modifier.fillMaxWidth().alpha(if (enabled) 1f else 0.4f),
        fill = if (primary) theme.colors.button else theme.colors.fill,
        content = if (primary) theme.colors.onButton else theme.colors.textPrimary,
        height = 84.dp,
        shape = RoundedCornerShape(22.dp),
    )
}

/** Gives [requester] focus once it is laid out. */
@Composable
private fun FocusFirst(requester: FocusRequester) {
    LaunchedEffect(Unit) {
        withFrameNanos {}
        runCatching { requester.requestFocus() }
    }
}

/**
 * A link code approved at mavio.dev/link on another device, or at the
 * mavio.dev of [server], polled until it is approved and replaced as it
 * expires; or connecting to a server instead.
 */
@Composable
private fun LinkStep(model: AppModel, server: ServerInfo?, onDone: () -> Unit, onServer: () -> Unit) {
    var link by remember { mutableStateOf<LinkCode?>(null) }
    var error by remember { mutableStateOf<String?>(null) }
    var attempt by remember { mutableIntStateOf(0) }
    val api = server?.mavioDevUrl?.takeIf { it.isNotEmpty() } ?: BuildConfig.MAVIO_DEV_API
    LaunchedEffect(attempt) {
        error = null
        try {
            while (true) {
                val code = model.client.startLink(api, CLIENT_ID)
                link = code
                val every = maxOf(code.interval(), Duration.ofSeconds(1)).toMillis()
                while (Instant.now() < code.expires()) {
                    delay(every)
                    // Signed in, the app leaves for Home.
                    if (model.pollLink(code) != null) {
                        onDone()
                        return@LaunchedEffect
                    }
                }
            }
        } catch (e: CancellationException) {
            throw e
        } catch (e: Exception) {
            Log.w("Mavio", "link code", e)
            link = null
            error = e.message ?: e.toString()
        }
    }
    Heading(stringResource(R.string.auth_sign_in_with_mavio), eyebrow = server?.name)
    link?.let {
        val where = it.verificationUri().removePrefix("https://").removePrefix("http://")
        Body(stringResource(R.string.auth_link_message, where))
        Code(it.userCode())
        Scan(it.verificationUriComplete())
    }
    val failed = error
    val first = remember { FocusRequester() }
    if (failed != null) {
        Body(failed, theme.colors.danger)
        CardButton(stringResource(R.string.auth_try_again), { attempt++ }, Modifier.focusRequester(first), primary = true)
        CardButton(stringResource(R.string.auth_connect_to_server), onServer)
    } else {
        Waiting()
        if (server == null) CardButton(stringResource(R.string.auth_connect_to_server), onServer, Modifier.focusRequester(first))
    }
    if (failed != null || server == null) FocusFirst(first)
}

/** A server's address, typed or chosen from those found on the network. */
@Composable
private fun AddressStep(model: AppModel, busy: Boolean, error: String?, onProbe: (String) -> Unit) {
    var address by remember { mutableStateOf("") }
    var found by remember { mutableStateOf<List<FoundServer>>(emptyList()) }
    LaunchedEffect(Unit) {
        found = runCatching { model.client.discover(Duration.ofSeconds(2)) }.getOrDefault(emptyList())
    }
    val servers by model.servers.collectAsStateWithLifecycle()
    val known = servers.map { it.id }.toSet()
    val nearby = found.filter { it.id !in known }
    val first = remember { FocusRequester() }
    FocusFirst(first)
    Heading(stringResource(R.string.auth_connect_to_server))
    Body(stringResource(R.string.auth_server_address_message))
    Box(Modifier.focusRequester(first)) {
        SettingsField(address, { address = it }, null, placeholder = "http://192.168.1.5:8686", keyboard = KeyboardType.Uri)
    }
    error?.let { Body(it, theme.colors.danger) }
    CardButton(stringResource(R.string.action_continue), { onProbe(address) }, primary = true, enabled = !busy && address.isNotBlank())
    if (nearby.isNotEmpty()) {
        Group(stringResource(R.string.auth_nearby)) {
            nearby.forEachIndexed { i, f ->
                if (i > 0) RowDivider()
                Pressable({ if (!busy) onProbe(f.address) }, Modifier.fillMaxWidth()) {
                    ValueRow(f.name, f.address)
                }
            }
        }
    }
}

/** A standalone server's user: Quick Connect, or a name and password. */
@Composable
private fun CredentialsStep(
    info: ServerInfo,
    busy: Boolean,
    error: String?,
    onSignIn: (String, String) -> Unit,
    onQuickConnect: () -> Unit,
) {
    var name by remember { mutableStateOf("") }
    var password by remember { mutableStateOf("") }
    val first = remember { FocusRequester() }
    FocusFirst(first)
    Heading(stringResource(R.string.auth_sign_in_to, info.name), eyebrow = info.address)
    CardButton(stringResource(R.string.auth_quick_connect), onQuickConnect, Modifier.focusRequester(first), primary = true, enabled = !busy)
    Row(horizontalArrangement = Arrangement.spacedBy(MavioSpacing.s16)) {
        SettingsField(name, { name = it }, stringResource(R.string.auth_username), Modifier.weight(1f))
        SettingsField(password, { password = it }, stringResource(R.string.auth_password), Modifier.weight(1f), secret = true)
    }
    error?.let { Body(it, theme.colors.danger) }
    CardButton(stringResource(R.string.auth_sign_in), { onSignIn(name, password) }, enabled = !busy && name.isNotEmpty())
}

/** A Quick Connect code, asked about every three seconds until it is entered or expires. */
@Composable
private fun QuickConnectStep(model: AppModel, request: QuickConnectRequest, onDone: () -> Unit, onFailed: (String?) -> Unit) {
    LaunchedEffect(request) {
        try {
            while (Instant.now() < request.expires()) {
                delay(3000)
                if (model.pollQuickConnect(request) != null) {
                    onDone()
                    return@LaunchedEffect
                }
            }
            onFailed(null)
        } catch (e: CancellationException) {
            throw e
        } catch (e: Exception) {
            onFailed(e.message ?: e.toString())
        }
    }
    Heading(stringResource(R.string.auth_quick_connect_title))
    Body(stringResource(R.string.auth_quick_connect_code, request.code()))
    Code(request.code())
    Waiting()
}

/** A server not set up yet: its address, to set it up on a computer or phone. */
@Composable
private fun SetUpStep(info: ServerInfo) {
    Heading(stringResource(R.string.auth_set_up_elsewhere, info.name), eyebrow = stringResource(R.string.auth_new_server_setup))
    Body(stringResource(R.string.auth_set_up_elsewhere_message))
    Text(info.address, fontFamily = FontFamily.Monospace, fontSize = 40.sp, fontWeight = FontWeight.SemiBold)
    Scan(info.address)
}

/**
 * The QR code of an address for a phone to scan, the line beside it: the
 * link page with the code filled in, or a new server's address.
 */
@Composable
private fun Scan(text: String) {
    Row(horizontalArrangement = Arrangement.spacedBy(24.dp), verticalAlignment = Alignment.CenterVertically) {
        QRCodeOf(text, Modifier.size(220.dp), quiet = 16.dp, radius = 18.dp)
        Body(stringResource(R.string.auth_scan_link_code))
    }
}

/** A code to enter on another device, a character to a box. */
@Composable
private fun Code(code: String) {
    Row(Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.spacedBy(10.dp, Alignment.CenterHorizontally)) {
        code.forEach { c ->
            if (c == '-') {
                Text("–", Modifier.align(Alignment.CenterVertically), fontSize = 56.sp, color = theme.colors.textTertiary)
            } else {
                Box(Modifier.size(width = 88.dp, height = 110.dp).background(theme.colors.fill, RoundedCornerShape(22.dp)), Alignment.Center) {
                    Text(c.toString(), fontFamily = FontFamily.Monospace, fontSize = 56.sp, fontWeight = FontWeight.SemiBold)
                }
            }
        }
    }
}

/** Waiting for the code's approval. */
@Composable
private fun Waiting() {
    Row(Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.spacedBy(MavioSpacing.s12, Alignment.CenterHorizontally), verticalAlignment = Alignment.CenterVertically) {
        CircularProgressIndicator(Modifier.size(24.dp), color = theme.colors.textTertiary, strokeWidth = 3.dp)
        Text(stringResource(R.string.auth_waiting), fontSize = 26.sp, color = theme.colors.textSecondary)
    }
}
