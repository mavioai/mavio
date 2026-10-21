package dev.mavio.admin

import android.content.ClipData
import android.content.ClipboardManager
import android.content.Context
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.lazy.LazyListScope
import androidx.compose.foundation.text.selection.SelectionContainer
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateMapOf
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.unit.dp
import dev.mavio.core.AppModel
import dev.mavio.core.call
import dev.mavio.core.choices
import dev.mavio.core.localization
import dev.mavio.design.MavioSpacing
import dev.mavio.ui.ConfirmDialog
import dev.mavio.ui.ButtonKind
import dev.mavio.ui.ChoiceMenuBox
import dev.mavio.ui.ChoiceRow
import dev.mavio.ui.Group
import dev.mavio.ui.LocalTelevision
import dev.mavio.ui.Pressable
import dev.mavio.ui.R
import dev.mavio.ui.RowDivider
import dev.mavio.ui.SettingsButton
import dev.mavio.ui.SettingsField
import dev.mavio.ui.SettingsList
import dev.mavio.ui.StatusView
import dev.mavio.ui.SwitchRow
import dev.mavio.ui.PressableBox
import dev.mavio.ui.rowPadding
import dev.mavio.ui.text
import dev.mavio.ui.theme
import kotlinx.coroutines.CancellationException
import kotlinx.coroutines.async
import kotlinx.coroutines.launch
import mavio.auth.v1.Auth.AccountMode
import mavio.auth.v1.Auth.ApiKey
import mavio.auth.v1.Auth.CreateApiKeyResponse
import mavio.auth.v1.Auth.GetAuthInfoResponse
import mavio.auth.v1.Auth.ListApiKeysResponse
import mavio.auth.v1.Auth.RevokeApiKeyResponse
import mavio.auth.v1.createApiKeyRequest
import mavio.auth.v1.getAuthInfoRequest
import mavio.auth.v1.listApiKeysRequest
import mavio.auth.v1.revokeApiKeyRequest
import mavio.plugin.v1.Plugin.Capability
import mavio.system.v1.PluginOuterClass.ListPluginsResponse
import mavio.system.v1.System.HardwareAcceleration
import mavio.system.v1.System.ServerSettings
import mavio.system.v1.System.TranscodingSettings
import mavio.system.v1.listPluginsRequest
import mavio.user.v1.UserOuterClass.ListUsersResponse
import mavio.user.v1.listUsersRequest

/** A page of the server's settings while they load or failed to. */
fun LazyListScope.loading(editor: SettingsEditor) {
    if (editor.saved == null) {
        val failure = editor.failure
        item { if (failure == null) StatusView(stringResource(R.string.state_loading)) else StatusView(failure, icon = R.drawable.ic_error) }
    }
}

/** The server's general settings as edited. */
private data class GeneralDraft(
    val name: String = "",
    val language: String = "",
    val country: String = "",
    val idleDays: Int = 1,
    val resetPlugin: String = "",
    val baseUrl: String = "",
    val httpsPort: Int = 0,
    val certificate: String = "",
    val key: String = "",
    val discovery: Boolean = false,
    val relay: Boolean = false,
) {
    constructor(s: ServerSettings) : this(
        s.network.serverName,
        s.metadata.preferredLanguage,
        s.metadata.country,
        s.sessionIdleDays,
        s.passwordResetPlugin,
        s.network.baseUrl,
        s.network.httpsPort,
        s.network.certificatePath,
        s.network.keyPath,
        s.network.localDiscovery,
        s.network.relay,
    )

    val valid get() = idleDays in 1..3650 && httpsPort in 0..65535

    fun apply(s: ServerSettings.Builder) {
        s.network = s.network.toBuilder()
            .setServerName(name.trim())
            .setBaseUrl(baseUrl.trim())
            .setHttpsPort(httpsPort)
            .setCertificatePath(certificate.trim())
            .setKeyPath(key.trim())
            .setLocalDiscovery(discovery)
            .setRelay(relay)
            .build()
        s.metadata = s.metadata.toBuilder().setPreferredLanguage(language).setCountry(country).build()
        s.sessionIdleDays = idleDays
        s.passwordResetPlugin = resetPlugin
    }
}

/**
 * General (docs/clients.md §6.3.8): the server's name, the language and
 * country of its metadata, how long devices stay signed in, the plugin that
 * resets forgotten passwords, and how it is reached.
 */
@Composable
fun GeneralPage(model: AppModel, serverId: String) {
    val editor = remember(serverId) { SettingsEditor(model, serverId) }
    var draft by remember { mutableStateOf(GeneralDraft()) }
    var languages by remember { mutableStateOf<List<Pair<String, String>>>(emptyList()) }
    var countries by remember { mutableStateOf<List<Pair<String, String>>>(emptyList()) }
    var resetters by remember { mutableStateOf<List<Pair<String, String>>>(emptyList()) }
    var mavioDev by remember { mutableStateOf(false) }
    LaunchedEffect(editor) {
        val info = async {
            runCatching { model.client.call(serverId, Procedure.auth("GetAuthInfo"), getAuthInfoRequest {}, GetAuthInfoResponse::parseFrom) }.getOrNull()
        }
        val plugins = async {
            runCatching { model.client.call(serverId, Procedure.plugins("ListPlugins"), listPluginsRequest {}, ListPluginsResponse::parseFrom) }.getOrNull()
        }
        editor.load()
        editor.saved?.let { draft = GeneralDraft(it.settings) }
        model.localization(serverId).let { (l, c) ->
            languages = l
            countries = c
        }
        mavioDev = info.await()?.mode == AccountMode.ACCOUNT_MODE_MAVIO_DEV
        resetters = plugins.await()?.pluginsList.orEmpty()
            .filter { Capability.CAPABILITY_PASSWORD_RESET in it.manifest.capabilitiesList }
            .map { it.manifest.id to it.manifest.name.ifEmpty { it.manifest.id } }
    }
    val scope = rememberCoroutineScope()
    val saved = stringResource(R.string.general_saved)
    SettingsList(stringResource(R.string.settings_general)) {
        loading(editor)
        val settings = editor.saved?.settings ?: return@SettingsList
        fun set(change: GeneralDraft.() -> GeneralDraft) {
            draft = draft.change()
        }
        item {
            Group {
                TextRow(stringResource(R.string.general_name), draft.name, { set { copy(name = it) } }, stringResource(R.string.general_name_detail))
                RowDivider()
                ChoiceRow(
                    stringResource(R.string.general_language),
                    listOf("" to stringResource(R.string.general_language_default)) + choices(languages, draft.language),
                    draft.language,
                    stringResource(R.string.general_language_detail),
                ) { set { copy(language = it) } }
                RowDivider()
                ChoiceRow(
                    stringResource(R.string.general_country),
                    listOf("" to stringResource(R.string.general_country_default)) + choices(countries, draft.country),
                    draft.country,
                    stringResource(R.string.general_country_detail),
                ) { set { copy(country = it) } }
            }
        }
        item {
            Group(stringResource(R.string.general_sign_in)) {
                NumberRow(
                    stringResource(R.string.general_idle_days),
                    draft.idleDays.toDouble(),
                    1.0..3650.0,
                    { set { copy(idleDays = it.toInt()) } },
                    stringResource(R.string.general_idle_days_detail),
                )
                if (!mavioDev) {
                    RowDivider()
                    ChoiceRow(
                        stringResource(R.string.general_reset_plugin),
                        listOf("" to stringResource(R.string.general_reset_off)) + choices(resetters, draft.resetPlugin),
                        draft.resetPlugin,
                        stringResource(if (resetters.isEmpty()) R.string.general_reset_plugin_none else R.string.general_reset_plugin_detail),
                    ) { set { copy(resetPlugin = it) } }
                }
            }
        }
        item {
            Group(stringResource(R.string.general_network), stringResource(R.string.general_network_footer)) {
                TextRow(
                    stringResource(R.string.general_base_url),
                    draft.baseUrl,
                    { set { copy(baseUrl = it) } },
                    stringResource(R.string.general_base_url_detail),
                    placeholder = "/mavio",
                )
                RowDivider()
                NumberRow(
                    stringResource(R.string.general_https_port),
                    draft.httpsPort.toDouble(),
                    0.0..65535.0,
                    { set { copy(httpsPort = it.toInt()) } },
                    stringResource(if (mavioDev) R.string.general_https_port_mavio_dev else R.string.general_https_port_detail),
                )
                if (draft.httpsPort > 0) {
                    RowDivider()
                    TextRow(
                        stringResource(R.string.general_certificate),
                        draft.certificate,
                        { set { copy(certificate = it) } },
                        stringResource(R.string.general_certificate_detail),
                        placeholder = "/etc/mavio/cert.pem",
                    )
                    RowDivider()
                    TextRow(stringResource(R.string.general_key), draft.key, { set { copy(key = it) } }, placeholder = "/etc/mavio/key.pem")
                }
                RowDivider()
                SwitchRow(stringResource(R.string.general_discovery), draft.discovery, stringResource(R.string.general_discovery_detail)) {
                    set { copy(discovery = it) }
                }
                if (mavioDev) {
                    RowDivider()
                    SwitchRow(stringResource(R.string.general_relay), draft.relay, stringResource(R.string.general_relay_detail)) {
                        set { copy(relay = it) }
                    }
                }
            }
        }
        item {
            SaveRow(draft != GeneralDraft(settings) && draft.valid, editor.saving, editor.notice, editor.failure) {
                val d = draft
                scope.launch { editor.save(saved) { d.apply(it) }?.let { draft = GeneralDraft(it) } }
            }
        }
    }
}

private val PRESETS = listOf("ultrafast", "superfast", "veryfast", "faster", "fast", "medium", "slow", "slower", "veryslow")
private val TONEMAPS = listOf("bt2390", "hable", "mobius", "reinhard", "clip", "linear", "gamma")
private val RANGES = listOf("auto", "tv", "pc")
private val DEINTERLACERS = listOf("yadif", "bwdif")

/**
 * Transcoding (docs/clients.md §6.3.9): how the server re-encodes what
 * devices cannot play — its hardware acceleration, the encoder's preset and
 * quality, tone mapping, deinterlacing, the downmix, cropping and the folder
 * of running transcodes.
 */
@Composable
fun TranscodingPage(model: AppModel, serverId: String) {
    val editor = remember(serverId) { SettingsEditor(model, serverId) }
    var draft by remember { mutableStateOf(TranscodingSettings.getDefaultInstance()) }
    LaunchedEffect(editor) {
        editor.load()
        editor.saved?.let { draft = it.settings.transcoding }
    }
    val scope = rememberCoroutineScope()
    val savedNotice = stringResource(R.string.transcoding_saved)
    SettingsList(stringResource(R.string.settings_transcoding), stringResource(R.string.transcoding_message)) {
        loading(editor)
        val saved = editor.saved ?: return@SettingsList
        fun set(change: TranscodingSettings.Builder.() -> Unit) {
            draft = draft.toBuilder().apply(change).build()
        }
        fun pick(list: List<String>, chosen: String) = (if (chosen in list) list else list + chosen).map { it to it }
        val valid = draft.h264Crf in 0..51 && draft.h265Crf in 0..51 && draft.threads in 0..256 &&
            draft.tonemapDesat in 0.0..10.0 && draft.tonemapPeak in 0.0..10000.0 && draft.downmixBoost in 0.5..3.0
        item {
            Group(stringResource(R.string.transcoding_hardware)) {
                val available = saved.availableHardwareAccelerationsList.ifEmpty {
                    listOf(HardwareAcceleration.HARDWARE_ACCELERATION_AUTO, HardwareAcceleration.HARDWARE_ACCELERATION_NONE)
                }
                val list = (if (draft.hardwareAcceleration in available) available else available + draft.hardwareAcceleration)
                    .filter { it != HardwareAcceleration.HARDWARE_ACCELERATION_UNSPECIFIED && it != HardwareAcceleration.UNRECOGNIZED }
                ChoiceRow(
                    stringResource(R.string.transcoding_acceleration),
                    list.map { it to accelerationName(it) },
                    draft.hardwareAcceleration,
                    stringResource(R.string.transcoding_acceleration_detail),
                ) { set { hardwareAcceleration = it } }
                if (draft.hardwareAcceleration != HardwareAcceleration.HARDWARE_ACCELERATION_NONE) {
                    RowDivider()
                    SwitchRow(
                        stringResource(R.string.transcoding_hardware_encoding),
                        draft.hardwareEncoding,
                        stringResource(R.string.transcoding_hardware_encoding_detail),
                    ) { set { hardwareEncoding = it } }
                }
            }
        }
        item {
            Group(stringResource(R.string.transcoding_encoding)) {
                ChoiceRow(
                    stringResource(R.string.transcoding_preset),
                    listOf("" to stringResource(R.string.transcoding_preset_auto)) + pick(PRESETS, draft.encoderPreset).filter { it.first.isNotEmpty() },
                    draft.encoderPreset,
                    stringResource(R.string.transcoding_preset_detail),
                ) { set { encoderPreset = it } }
                RowDivider()
                NumberRow(
                    stringResource(R.string.transcoding_avc_crf),
                    draft.h264Crf.toDouble(),
                    0.0..51.0,
                    { set { h264Crf = it.toInt() } },
                    stringResource(R.string.transcoding_crf_detail),
                )
                RowDivider()
                NumberRow(stringResource(R.string.transcoding_hevc_crf), draft.h265Crf.toDouble(), 0.0..51.0, { set { h265Crf = it.toInt() } })
                RowDivider()
                NumberRow(
                    stringResource(R.string.transcoding_threads),
                    draft.threads.toDouble(),
                    0.0..256.0,
                    { set { threads = it.toInt() } },
                    stringResource(R.string.transcoding_threads_detail),
                )
            }
        }
        item {
            Group(stringResource(R.string.transcoding_video)) {
                ChoiceRow(
                    stringResource(R.string.transcoding_tonemap),
                    pick(TONEMAPS, draft.tonemapAlgorithm),
                    draft.tonemapAlgorithm,
                    stringResource(R.string.transcoding_tonemap_detail),
                ) { set { tonemapAlgorithm = it } }
                RowDivider()
                ChoiceRow(stringResource(R.string.transcoding_tonemap_range), pick(RANGES, draft.tonemapRange), draft.tonemapRange) {
                    set { tonemapRange = it }
                }
                RowDivider()
                NumberRow(
                    stringResource(R.string.transcoding_tonemap_desat),
                    draft.tonemapDesat,
                    0.0..10.0,
                    { set { tonemapDesat = it } },
                    decimals = true,
                )
                RowDivider()
                NumberRow(
                    stringResource(R.string.transcoding_tonemap_peak),
                    draft.tonemapPeak,
                    0.0..10000.0,
                    { set { tonemapPeak = it } },
                    stringResource(R.string.transcoding_tonemap_peak_detail),
                )
                RowDivider()
                ChoiceRow(stringResource(R.string.transcoding_deinterlace), pick(DEINTERLACERS, draft.deinterlaceMethod), draft.deinterlaceMethod) {
                    set { deinterlaceMethod = it }
                }
                RowDivider()
                SwitchRow(
                    stringResource(R.string.transcoding_double_rate),
                    draft.deinterlaceDoubleRate,
                    stringResource(R.string.transcoding_double_rate_detail),
                ) { set { deinterlaceDoubleRate = it } }
                RowDivider()
                SwitchRow(stringResource(R.string.transcoding_crop), draft.cropBlackBorders, stringResource(R.string.transcoding_crop_detail)) {
                    set { cropBlackBorders = it }
                }
            }
        }
        item {
            Group(stringResource(R.string.transcoding_audio)) {
                NumberRow(
                    stringResource(R.string.transcoding_downmix_boost),
                    draft.downmixBoost,
                    0.5..3.0,
                    { set { downmixBoost = it } },
                    stringResource(R.string.transcoding_downmix_boost_detail),
                    decimals = true,
                )
            }
        }
        item {
            Group(stringResource(R.string.transcoding_folder), stringResource(R.string.transcoding_folder_footer)) {
                TextRow(
                    stringResource(R.string.transcoding_folder),
                    draft.transcodeDir,
                    { set { transcodeDir = it } },
                    placeholder = stringResource(R.string.transcoding_folder_default),
                )
            }
        }
        item {
            SaveRow(draft != saved.settings.transcoding && valid, editor.saving, editor.notice, editor.failure) {
                val d = draft.toBuilder().setTranscodeDir(draft.transcodeDir.trim()).build()
                scope.launch { editor.save(savedNotice) { it.transcoding = d }?.let { draft = it.transcoding } }
            }
        }
    }
}

@Composable
private fun accelerationName(a: HardwareAcceleration): String = when (a) {
    HardwareAcceleration.HARDWARE_ACCELERATION_AUTO -> stringResource(R.string.transcoding_acceleration_auto)
    HardwareAcceleration.HARDWARE_ACCELERATION_NONE -> stringResource(R.string.transcoding_acceleration_none)
    HardwareAcceleration.HARDWARE_ACCELERATION_VIDEOTOOLBOX -> "VideoToolbox"
    else -> a.number.toString()
}

/** A key just made: its token is shown this once. */
private data class MadeKey(val id: String, val name: String, val token: String)

/**
 * API keys (docs/clients.md §6.3.15): the keys scripts call the server
 * with, each acting as the administrator who made it; a key's token is
 * shown once, when it is made.
 */
@Composable
fun ApiKeysPage(model: AppModel, serverId: String) {
    var keys by remember { mutableStateOf<List<ApiKey>?>(null) }
    val users = remember { mutableStateMapOf<String, String>() }
    var failure by remember { mutableStateOf<String?>(null) }
    var name by remember { mutableStateOf("") }
    var creating by remember { mutableStateOf(false) }
    var createFailure by remember { mutableStateOf<String?>(null) }
    var made by remember { mutableStateOf<MadeKey?>(null) }
    var copied by remember { mutableStateOf(false) }
    var revoking by remember { mutableStateOf<ApiKey?>(null) }
    val scope = rememberCoroutineScope()
    suspend fun load() {
        try {
            keys = model.client.call(serverId, Procedure.auth("ListApiKeys"), listApiKeysRequest {}, ListApiKeysResponse::parseFrom).apiKeysList
            failure = null
        } catch (e: CancellationException) {
            throw e
        } catch (e: Exception) {
            failure = e.reason
        }
        if (users.isEmpty()) {
            runCatching { model.client.call(serverId, Procedure.users("ListUsers"), listUsersRequest {}, ListUsersResponse::parseFrom) }
                .getOrNull()?.usersList?.forEach { users.putIfAbsent(it.id, it.name) }
        }
    }
    LaunchedEffect(serverId) { load() }
    fun create() {
        val trimmed = name.trim()
        if (trimmed.isEmpty() || creating) return
        creating = true
        createFailure = null
        scope.launch {
            try {
                val r = model.client.call(serverId, Procedure.auth("CreateApiKey"), createApiKeyRequest { this.name = trimmed }, CreateApiKeyResponse::parseFrom)
                made = MadeKey(r.apiKey.id, r.apiKey.name.ifEmpty { trimmed }, r.token)
                copied = false
                name = ""
                load()
            } catch (e: CancellationException) {
                throw e
            } catch (e: Exception) {
                createFailure = e.reason
            } finally {
                creating = false
            }
        }
    }
    val context = LocalContext.current
    SettingsList(stringResource(R.string.settings_api_keys), stringResource(R.string.keys_message), refresh = { load() }) {
        item {
            Group(footer = stringResource(R.string.keys_create_footer)) {
                Row(
                    Modifier.fillMaxWidth().padding(16.dp),
                    horizontalArrangement = Arrangement.spacedBy(MavioSpacing.s12),
                    verticalAlignment = Alignment.CenterVertically,
                ) {
                    SettingsField(name, { name = it }, null, Modifier.weight(1f), placeholder = stringResource(R.string.keys_name))
                    SettingsButton(if (creating) "…" else stringResource(R.string.keys_create), ::create, enabled = name.isNotBlank() && !creating)
                }
                createFailure?.let { Text(it, Modifier.padding(start = 16.dp, end = 16.dp, bottom = 16.dp), style = theme.type.callout.text, color = theme.colors.danger) }
            }
        }
        val key = made
        if (key != null && keys?.any { it.id == key.id } == true) {
            item {
                Group(stringResource(R.string.keys_made_title, key.name), stringResource(R.string.keys_made_footer)) {
                    Column(Modifier.padding(16.dp), verticalArrangement = Arrangement.spacedBy(MavioSpacing.s12)) {
                        SelectionContainer {
                            Text(key.token, style = theme.type.callout.text.copy(fontFamily = FontFamily.Monospace))
                        }
                        Row(verticalAlignment = Alignment.CenterVertically) {
                            SettingsButton(
                                stringResource(if (copied) R.string.users_copied else R.string.keys_copy),
                                {
                                    context.getSystemService(ClipboardManager::class.java)
                                        .setPrimaryClip(ClipData.newPlainText(key.name, key.token))
                                    copied = true
                                },
                                kind = ButtonKind.Secondary,
                            )
                            Column(Modifier.weight(1f)) {}
                            PressableBox({ made = null }) {
                                Text(
                                    stringResource(R.string.keys_done),
                                    Modifier.padding(8.dp),
                                    style = theme.type.callout.text,
                                    color = theme.colors.accentText,
                                )
                            }
                        }
                    }
                }
            }
        }
        item {
            Group(stringResource(R.string.keys_list)) {
                keys.orEmpty().forEachIndexed { i, k ->
                    if (i > 0) RowDivider()
                    Row(
                        Modifier.fillMaxWidth().padding(horizontal = rowPadding, vertical = 10.dp),
                        horizontalArrangement = Arrangement.spacedBy(MavioSpacing.s12),
                        verticalAlignment = Alignment.CenterVertically,
                    ) {
                        Column(Modifier.weight(1f)) {
                            Text(k.name, style = theme.type.body.text)
                            Text(keyDetail(context, k, users), style = theme.type.footnote.text, color = theme.colors.textSecondary)
                        }
                        PressableBox({ revoking = k }) {
                            Text(
                                stringResource(R.string.keys_revoke),
                                Modifier.padding(horizontal = if (LocalTelevision.current) 12.dp else 0.dp, vertical = 6.dp),
                                style = theme.type.callout.text,
                                color = theme.colors.danger,
                            )
                        }
                    }
                }
                val error = failure
                when {
                    error != null -> Text(error, Modifier.padding(16.dp), style = theme.type.callout.text, color = theme.colors.danger)
                    keys == null -> StatusView(stringResource(R.string.state_loading))
                    keys?.isEmpty() == true -> Text(
                        stringResource(R.string.keys_none),
                        Modifier.padding(16.dp),
                        style = theme.type.body.text,
                        color = theme.colors.textSecondary,
                    )
                }
            }
        }
    }
    revoking?.let { k ->
        ConfirmDialog(
            stringResource(R.string.keys_revoke_title, k.name),
            stringResource(R.string.keys_revoke_message),
            stringResource(R.string.keys_revoke),
            onConfirm = {
                scope.launch {
                    try {
                        model.client.call(serverId, Procedure.auth("RevokeApiKey"), revokeApiKeyRequest { id = k.id }, RevokeApiKeyResponse::parseFrom)
                        load()
                    } catch (e: CancellationException) {
                        throw e
                    } catch (e: Exception) {
                        failure = e.reason
                    }
                }
            },
            onDismiss = { revoking = null },
        )
    }
}

private fun keyDetail(context: Context, key: ApiKey, users: Map<String, String>): String = listOfNotNull(
    users[key.userId]?.let { context.getString(R.string.keys_acts_as, it) },
    if (key.hasCreateTime()) context.getString(R.string.keys_created, fromNow(key.createTime)) else null,
    if (key.hasLastUseTime()) context.getString(R.string.keys_used, fromNow(key.lastUseTime)) else context.getString(R.string.keys_never_used),
).joinToString(" · ")
