package dev.mavio.app

import android.content.Context
import android.content.Intent
import android.util.Log
import androidx.activity.compose.BackHandler
import androidx.activity.compose.ManagedActivityResultLauncher
import androidx.activity.compose.rememberLauncherForActivityResult
import androidx.browser.auth.AuthTabIntent
import androidx.browser.customtabs.CustomTabsIntent
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.imePadding
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.safeDrawingPadding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.widthIn
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.text.KeyboardActions
import androidx.compose.foundation.text.KeyboardOptions
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.HorizontalDivider
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.OutlinedTextFieldDefaults
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableIntStateOf
import androidx.compose.runtime.mutableStateListOf
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.input.ImeAction
import androidx.compose.ui.text.input.KeyboardType
import androidx.compose.ui.text.input.PasswordVisualTransformation
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.unit.dp
import androidx.core.net.toUri
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import androidx.compose.ui.unit.sp
import dev.mavio.core.AccountMode
import dev.mavio.core.AppModel
import dev.mavio.core.BrowserSignIn
import dev.mavio.core.CoreException
import dev.mavio.core.FoundServer
import dev.mavio.core.LinkCode
import dev.mavio.core.QuickConnectRequest
import dev.mavio.core.ServerInfo
import dev.mavio.core.isUnauthenticated
import dev.mavio.design.MavioRadius
import dev.mavio.design.MavioSpacing
import dev.mavio.ui.ButtonKind
import dev.mavio.ui.MavioButton
import dev.mavio.ui.MavioIcon
import dev.mavio.ui.PosterWall
import dev.mavio.ui.R
import dev.mavio.ui.Wordmark
import dev.mavio.ui.text
import dev.mavio.ui.theme
import java.time.Duration
import java.time.Instant
import kotlinx.coroutines.CancellationException
import kotlinx.coroutines.delay
import kotlinx.coroutines.flow.first
import kotlinx.coroutines.launch

/** The client ID of the Android apps at mavio.dev. */
const val CLIENT_ID = "dev.mavio.app"

/** Where mavio.dev sends the browser back to, in the app's own scheme (RFC 8252 §7.1). */
private const val REDIRECT_URI = "$CLIENT_ID:/callback"

/** The mavio.dev the app signs a Mavio account in at, unless a server names its own. */
private const val MAVIO_DEV_API = BuildConfig.MAVIO_DEV_API

/** The steps of signing in, as the web UI's and the desktop app's. */
private sealed interface Step {
    /** The way in: a Mavio account, or a server's address. */
    data object Welcome : Step

    /** The browser signing a Mavio account in. */
    data class Browser(val signIn: BrowserSignIn) : Step

    /** A link code approved on another device. */
    data object Link : Step

    /** A server's address, typed or found on the network. */
    data object Address : Step

    /** The name and password of a server's user. */
    data class Credentials(val info: ServerInfo) : Step

    /** A Quick Connect code entered on a device signed in. */
    data class QuickConnect(val info: ServerInfo, val request: QuickConnectRequest) : Step

    /** A server not set up yet, which its setup in a browser does. */
    data class SetUp(val info: ServerInfo) : Step
}

/**
 * Sign-in (docs/accounts.md §6): a Mavio account in the browser or with a
 * link code, or a server's address, then its users' credentials or Quick
 * Connect; a server in mavio.dev mode goes back to the Mavio account, at
 * its mavio.dev. Over the wall of posters, as the web UI's.
 */
@Composable
fun SignInScreen(model: AppModel, adding: Boolean = false, onDone: () -> Unit = {}) {
    val context = LocalContext.current
    val scope = rememberCoroutineScope()
    val history = remember { mutableStateListOf<Step>() }
    var step by remember { mutableStateOf<Step>(if (adding) Step.Address else Step.Welcome) }
    // The server connected to, whose mavio.dev a Mavio account signs in at.
    var server by remember { mutableStateOf<ServerInfo?>(null) }
    var error by remember { mutableStateOf<String?>(null) }
    var busy by remember { mutableStateOf(false) }
    val notMavio = stringResource(R.string.auth_not_mavio)
    val wrongPassword = stringResource(R.string.auth_wrong_password)
    val unreachable = stringResource(R.string.auth_unreachable)
    val redirects = (context.applicationContext as MavioApplication).signInRedirect
    // The Auth Tab hands back where mavio.dev sent it; closed, it leaves the
    // sign-in waiting, to open it again or go back.
    val authTab = rememberLauncherForActivityResult(AuthTabIntent.AuthenticateUserResultContract()) { result ->
        if (result.resultCode == AuthTabIntent.RESULT_OK) redirects.value = result.resultUri?.toString()
    }

    fun api() = server?.mavioDevUrl?.takeIf { it.isNotEmpty() } ?: MAVIO_DEV_API

    fun goTo(next: Step) {
        history.add(step)
        step = next
        error = null
    }

    fun back() {
        step = history.removeLastOrNull() ?: return
        if (step == Step.Welcome || step == Step.Address) server = null
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

    fun signInInBrowser() = run {
        val signIn = model.client.startAppSignIn(api(), CLIENT_ID, REDIRECT_URI)
        redirects.value = null
        goTo(Step.Browser(signIn))
        openAuthTab(context, authTab, signIn.url())
    }

    fun probe(address: String) = run {
        val info = model.client.probe(address.trim())
        server = info
        goTo(
            when {
                info.mode == AccountMode.MAVIO_DEV -> Step.Welcome
                info.setupRequired -> Step.SetUp(info)
                else -> Step.Credentials(info)
            },
        )
    }

    BackHandler(history.isNotEmpty()) { back() }

    Box(Modifier.fillMaxSize().background(theme.colors.background)) {
        PosterWall()
        Column(
            Modifier
                .fillMaxSize()
                .safeDrawingPadding()
                .imePadding()
                .verticalScroll(rememberScrollState())
                .padding(horizontal = 24.dp, vertical = MavioSpacing.s32),
            verticalArrangement = Arrangement.Center,
            horizontalAlignment = Alignment.CenterHorizontally,
        ) {
            Column(
                Modifier.widthIn(max = 420.dp).fillMaxWidth(),
                verticalArrangement = Arrangement.spacedBy(MavioSpacing.s16),
            ) {
                if (history.isNotEmpty()) BackLink(::back)
                when (val s = step) {
                    Step.Welcome -> Welcome(
                        server = server,
                        busy = busy,
                        error = error,
                        onBrowser = ::signInInBrowser,
                        onLink = { goTo(Step.Link) },
                        onServer = { goTo(Step.Address) },
                    )
                    is Step.Browser -> BrowserStep(model, s.signIn, onAgain = { openAuthTab(context, authTab, s.signIn.url()) }) {
                        if (it == null) {
                            onDone()
                        } else {
                            back()
                            error = it
                        }
                    }
                    Step.Link -> LinkStep(model, api(), server, onDone)
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
                    is Step.SetUp -> SetUpStep(s.info) { openInBrowser(context, s.info.address) }
                }
            }
        }
    }
}

/** Opens a page in a Custom Tab, or the browser where there are none. */
private fun openInBrowser(context: Context, url: String) {
    runCatching { CustomTabsIntent.Builder().build().launchUrl(context, url.toUri()) }
        .onFailure { context.startActivity(Intent(Intent.ACTION_VIEW, url.toUri())) }
}

/**
 * Opens a page of mavio.dev in an Auth Tab, which comes back with the URI
 * of the app's scheme mavio.dev sends it to; a browser without Auth Tabs
 * opens a Custom Tab, whose redirect reaches the app as a link; the browser
 * itself where there are none.
 */
private fun openAuthTab(context: Context, launcher: ManagedActivityResultLauncher<Intent, AuthTabIntent.AuthResult>, url: String) {
    runCatching { AuthTabIntent.Builder().build().launch(launcher, url.toUri(), CLIENT_ID) }
        .onFailure { openInBrowser(context, url) }
}

@Composable
private fun BackLink(onClick: () -> Unit) {
    Row(
        Modifier.clickable(onClick = onClick).padding(vertical = MavioSpacing.s4),
        verticalAlignment = Alignment.CenterVertically,
        horizontalArrangement = Arrangement.spacedBy(2.dp),
    ) {
        MavioIcon(R.drawable.ic_back, size = 18.dp, tint = theme.colors.accentText)
        Text(stringResource(R.string.action_back), style = theme.type.body.text, color = theme.colors.accentText)
    }
}

/** A step's eyebrow, title and the line under it. */
@Composable
private fun Heading(title: String, eyebrow: String? = null, subtitle: String? = null) {
    Column(verticalArrangement = Arrangement.spacedBy(MavioSpacing.s4)) {
        eyebrow?.let {
            Text(
                it.uppercase(),
                style = theme.type.footnote.text,
                fontWeight = FontWeight.SemiBold,
                color = theme.colors.textTertiary,
            )
        }
        Text(title, style = theme.type.title1.text, fontWeight = FontWeight.Bold)
        subtitle?.let { Text(it, style = theme.type.body.text, color = theme.colors.textSecondary) }
    }
}

@Composable
private fun Body(text: String) {
    Text(text, style = theme.type.body.text, color = theme.colors.textSecondary)
}

@Composable
private fun ErrorLine(error: String?) {
    error?.let { Text(it, style = theme.type.callout.text, color = theme.colors.danger) }
}

@Composable
private fun Waiting() {
    Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(MavioSpacing.s8)) {
        CircularProgressIndicator(Modifier.size(18.dp), color = theme.colors.textSecondary, strokeWidth = 2.dp)
        Text(stringResource(R.string.auth_waiting), style = theme.type.callout.text, color = theme.colors.textSecondary)
    }
}

/**
 * The way in: the wordmark, then a Mavio account, in the browser or with a
 * link code, or a server's address; for a server in mavio.dev mode, its
 * Mavio account alone.
 */
@Composable
private fun Welcome(
    server: ServerInfo?,
    busy: Boolean,
    error: String?,
    onBrowser: () -> Unit,
    onLink: () -> Unit,
    onServer: () -> Unit,
) {
    Column(
        Modifier.fillMaxWidth(),
        horizontalAlignment = Alignment.CenterHorizontally,
        verticalArrangement = Arrangement.spacedBy(MavioSpacing.s12),
    ) {
        Wordmark(height = 40.dp)
        Spacer(Modifier.height(MavioSpacing.s8))
        Text(
            if (server != null) stringResource(R.string.auth_sign_in_to, server.name) else stringResource(R.string.auth_welcome),
            style = theme.type.title1.text,
            fontWeight = FontWeight.Bold,
            textAlign = TextAlign.Center,
        )
        if (server != null) {
            Text(
                stringResource(R.string.auth_mode_mavio_dev_message),
                style = theme.type.body.text,
                color = theme.colors.textSecondary,
                textAlign = TextAlign.Center,
            )
        }
    }
    Spacer(Modifier.height(MavioSpacing.s8))
    ErrorLine(error)
    MavioButton(
        stringResource(R.string.auth_sign_in_with_mavio),
        onBrowser,
        Modifier.fillMaxWidth(),
        large = true,
        enabled = !busy,
    )
    MavioButton(
        stringResource(R.string.auth_use_link_code),
        onLink,
        Modifier.fillMaxWidth(),
        kind = ButtonKind.Secondary,
        icon = R.drawable.ic_devices,
        large = true,
        enabled = !busy,
    )
    if (server == null) {
        Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(MavioSpacing.s12)) {
            HorizontalDivider(Modifier.weight(1f), color = theme.colors.border)
            Text(stringResource(R.string.auth_or), style = theme.type.footnote.text, color = theme.colors.textTertiary)
            HorizontalDivider(Modifier.weight(1f), color = theme.colors.border)
        }
        MavioButton(
            stringResource(R.string.auth_connect_to_server),
            onServer,
            Modifier.fillMaxWidth(),
            kind = ButtonKind.Secondary,
            icon = R.drawable.ic_server,
            large = true,
            enabled = !busy,
        )
    }
}

/** Waits for the browser to come back from mavio.dev; done with null, or with the error. */
@Composable
private fun BrowserStep(model: AppModel, signIn: BrowserSignIn, onAgain: () -> Unit, onDone: (String?) -> Unit) {
    val context = LocalContext.current
    val redirects = (context.applicationContext as MavioApplication).signInRedirect
    LaunchedEffect(signIn) {
        try {
            val callback = redirects.first { it != null }!!
            redirects.value = null
            model.finishAppSignIn(signIn, callback)
            onDone(null)
        } catch (e: CancellationException) {
            throw e
        } catch (e: Exception) {
            onDone(e.message ?: e.toString())
        }
    }
    val host = runCatching { signIn.url().toUri().host }.getOrNull().orEmpty()
    Heading(stringResource(R.string.auth_continue_in_browser))
    Body(stringResource(R.string.auth_continue_in_browser_message, host))
    Waiting()
    MavioButton(
        stringResource(R.string.auth_open_browser_again),
        onAgain,
        Modifier.fillMaxWidth(),
        kind = ButtonKind.Secondary,
        icon = R.drawable.ic_external,
        large = true,
    )
}

/**
 * A link code to approve at mavio.dev/link, polled until it is approved,
 * and replaced as it expires.
 */
@Composable
private fun LinkStep(model: AppModel, api: String, server: ServerInfo?, onDone: () -> Unit) {
    var link by remember { mutableStateOf<LinkCode?>(null) }
    var error by remember { mutableStateOf<String?>(null) }
    var attempt by remember { mutableIntStateOf(0) }
    LaunchedEffect(attempt) {
        error = null
        try {
            while (true) {
                val code = model.client.startLink(api, CLIENT_ID)
                link = code
                val every = maxOf(code.interval(), Duration.ofSeconds(1)).toMillis()
                while (Instant.now() < code.expires()) {
                    delay(every)
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
    Heading(stringResource(R.string.auth_sign_in_with_mavio), subtitle = server?.name)
    link?.let {
        val where = it.verificationUri().removePrefix("https://").removePrefix("http://")
        Body(stringResource(R.string.auth_link_message, where))
        CodeBoxes(it.userCode())
    }
    if (error != null) {
        ErrorLine(error)
        MavioButton(
            stringResource(R.string.auth_try_again),
            { attempt++ },
            Modifier.fillMaxWidth(),
            kind = ButtonKind.Secondary,
            large = true,
        )
    } else {
        Waiting()
    }
}

/** A code in a box for each character, as the web UI shows a link code. */
@Composable
private fun CodeBoxes(code: String) {
    Row(Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.spacedBy(6.dp, Alignment.CenterHorizontally)) {
        code.forEach { c ->
            if (c == '-') {
                Text("–", Modifier.align(Alignment.CenterVertically), style = theme.type.title2.text, color = theme.colors.textTertiary)
            } else {
                Box(
                    Modifier
                        .weight(1f, fill = false)
                        .size(width = 36.dp, height = 48.dp)
                        .background(theme.colors.fill, RoundedCornerShape(MavioRadius.md))
                        .border(1.dp, theme.colors.border, RoundedCornerShape(MavioRadius.md)),
                    Alignment.Center,
                ) {
                    Text(c.toString(), fontFamily = FontFamily.Monospace, fontSize = 24.sp, fontWeight = FontWeight.SemiBold)
                }
            }
        }
    }
}

/** A server's address, typed or chosen from those found on the network. */
@Composable
private fun AddressStep(model: AppModel, busy: Boolean, error: String?, onProbe: (String) -> Unit) {
    var address by remember { mutableStateOf("") }
    var found by remember { mutableStateOf<List<FoundServer>>(emptyList()) }
    LaunchedEffect(Unit) {
        found = runCatching { model.client.discover(Duration.ofSeconds(2)) }.getOrDefault(emptyList())
    }
    val ok = !busy && address.isNotBlank()
    Heading(stringResource(R.string.auth_connect_to_server))
    Body(stringResource(R.string.auth_server_address_message))
    Field(
        address,
        { address = it },
        stringResource(R.string.auth_server),
        placeholder = "http://192.168.1.5:8686",
        keyboard = KeyboardType.Uri,
        onSubmit = { if (ok) onProbe(address) },
    )
    ErrorLine(error)
    MavioButton(stringResource(R.string.action_continue), { onProbe(address) }, Modifier.fillMaxWidth(), large = true, enabled = ok)
    val servers by model.servers.collectAsStateWithLifecycle()
    val known = servers.map { it.id }.toSet()
    val nearby = found.filter { it.id !in known }
    if (nearby.isNotEmpty()) {
        Text(
            stringResource(R.string.auth_nearby),
            style = theme.type.footnote.text,
            fontWeight = FontWeight.SemiBold,
            color = theme.colors.textTertiary,
        )
        Column(Modifier.background(theme.colors.fill, RoundedCornerShape(12.dp))) {
            nearby.forEachIndexed { i, f ->
                if (i > 0) HorizontalDivider(Modifier.padding(start = 48.dp), color = theme.colors.border)
                Row(
                    Modifier
                        .fillMaxWidth()
                        .clickable(enabled = !busy) {
                            address = f.address
                            onProbe(f.address)
                        }
                        .padding(horizontal = 14.dp, vertical = 10.dp),
                    verticalAlignment = Alignment.CenterVertically,
                    horizontalArrangement = Arrangement.spacedBy(12.dp),
                ) {
                    MavioIcon(R.drawable.ic_server, size = 22.dp, tint = theme.colors.accent)
                    Column {
                        Text(f.name, style = theme.type.headline.text, maxLines = 1)
                        Text(f.address, style = theme.type.footnote.text, color = theme.colors.textSecondary, maxLines = 1)
                    }
                }
            }
        }
    }
}

/** The name and password of a server's user, or Quick Connect. */
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
    Heading(stringResource(R.string.auth_sign_in_to, info.name), subtitle = info.address)
    Field(name, { name = it }, stringResource(R.string.auth_username), imeAction = ImeAction.Next)
    Field(
        password,
        { password = it },
        stringResource(R.string.auth_password),
        keyboard = KeyboardType.Password,
        secret = true,
        onSubmit = { if (name.isNotEmpty() && !busy) onSignIn(name, password) },
    )
    ErrorLine(error)
    MavioButton(
        stringResource(R.string.auth_sign_in),
        { onSignIn(name, password) },
        Modifier.fillMaxWidth(),
        large = true,
        enabled = !busy && name.isNotEmpty(),
    )
    MavioButton(
        stringResource(R.string.auth_quick_connect),
        onQuickConnect,
        Modifier.fillMaxWidth(),
        kind = ButtonKind.Secondary,
        large = true,
        enabled = !busy,
    )
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
    CodeBoxes(request.code())
    Waiting()
}

/** A server not set up yet: its address, to set it up in a browser. */
@Composable
private fun SetUpStep(info: ServerInfo, onOpen: () -> Unit) {
    Heading(stringResource(R.string.auth_set_up_elsewhere, info.name), eyebrow = stringResource(R.string.auth_new_server_setup))
    Body(stringResource(R.string.auth_set_up_elsewhere_message))
    Text(info.address, fontFamily = FontFamily.Monospace, style = theme.type.title3.text)
    MavioButton(
        stringResource(R.string.auth_open_in_browser),
        onOpen,
        Modifier.fillMaxWidth(),
        icon = R.drawable.ic_external,
        large = true,
    )
}

/** A text field on the sign-in's card. */
@Composable
private fun Field(
    value: String,
    onChange: (String) -> Unit,
    label: String,
    placeholder: String? = null,
    keyboard: KeyboardType = KeyboardType.Text,
    secret: Boolean = false,
    imeAction: ImeAction = ImeAction.Done,
    onSubmit: () -> Unit = {},
) {
    OutlinedTextField(
        value,
        onChange,
        Modifier.fillMaxWidth(),
        label = { Text(label) },
        placeholder = placeholder?.let { { Text(it) } },
        singleLine = true,
        visualTransformation = if (secret) PasswordVisualTransformation() else androidx.compose.ui.text.input.VisualTransformation.None,
        keyboardOptions = KeyboardOptions(keyboardType = keyboard, imeAction = imeAction, autoCorrectEnabled = false),
        keyboardActions = KeyboardActions(onDone = { onSubmit() }),
        shape = RoundedCornerShape(10.dp),
        colors = OutlinedTextFieldDefaults.colors(
            focusedContainerColor = theme.colors.fill,
            unfocusedContainerColor = theme.colors.fill,
            focusedBorderColor = theme.colors.textSecondary,
            unfocusedBorderColor = theme.colors.border,
            focusedLabelColor = theme.colors.textSecondary,
            cursorColor = theme.colors.textPrimary,
        ),
    )
}
