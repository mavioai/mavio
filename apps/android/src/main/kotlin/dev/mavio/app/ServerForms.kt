package dev.mavio.app

import android.text.format.DateUtils
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.layout.widthIn
import androidx.compose.foundation.text.BasicTextField
import androidx.compose.foundation.text.KeyboardOptions
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.SolidColor
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.text.input.KeyboardType
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.unit.dp
import com.google.protobuf.Timestamp
import dev.mavio.core.AppModel
import dev.mavio.core.call
import dev.mavio.core.instant
import dev.mavio.design.MavioSpacing
import dev.mavio.ui.MavioButton
import dev.mavio.ui.R
import dev.mavio.ui.text
import dev.mavio.ui.theme
import kotlinx.coroutines.CancellationException
import kotlinx.coroutines.async
import kotlinx.coroutines.coroutineScope
import mavio.system.v1.Localization.ListCountriesResponse
import mavio.system.v1.Localization.ListLanguagesResponse
import mavio.system.v1.System.GetServerSettingsResponse
import mavio.system.v1.System.ServerSettings
import mavio.system.v1.System.UpdateServerSettingsResponse
import mavio.system.v1.getServerSettingsRequest
import mavio.system.v1.listCountriesRequest
import mavio.system.v1.listLanguagesRequest
import mavio.system.v1.updateServerSettingsRequest
import java.text.Collator
import java.util.Locale

/** A text field's row, its value right of its label. */
@Composable
internal fun TextRow(
    label: String,
    value: String,
    onChange: (String) -> Unit,
    detail: String? = null,
    placeholder: String? = null,
    keyboard: KeyboardType = KeyboardType.Text,
    error: Boolean = false,
    fieldWidth: Int = 170,
) {
    Row(
        Modifier.fillMaxWidth().heightIn(min = 52.dp).padding(horizontal = 16.dp, vertical = 8.dp),
        horizontalArrangement = Arrangement.spacedBy(MavioSpacing.s12),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        RowLabel(label, detail, Modifier.weight(1f))
        val style = theme.type.body.text.copy(
            color = if (error) theme.colors.danger else theme.colors.textSecondary,
            textAlign = TextAlign.End,
        )
        BasicTextField(
            value,
            onChange,
            Modifier.widthIn(max = fieldWidth.dp).width(fieldWidth.dp),
            textStyle = style,
            singleLine = true,
            cursorBrush = SolidColor(theme.colors.textPrimary),
            keyboardOptions = KeyboardOptions(keyboardType = keyboard, autoCorrectEnabled = false),
            decorationBox = { field ->
                Box(contentAlignment = Alignment.CenterEnd) {
                    if (value.isEmpty() && placeholder != null) {
                        Text(placeholder, style = style.copy(color = theme.colors.textTertiary), maxLines = 1)
                    }
                    field()
                }
            },
        )
    }
}

/** A field of a number, red while it is outside its range. */
@Composable
internal fun NumberRow(
    label: String,
    value: Double,
    range: ClosedFloatingPointRange<Double>,
    onChange: (Double) -> Unit,
    detail: String? = null,
    decimals: Boolean = false,
) {
    fun shown(v: Double) = if (decimals) v.toBigDecimal().stripTrailingZeros().toPlainString() else v.toLong().toString()
    var text by remember { mutableStateOf(shown(value)) }
    // A value changed elsewhere, as by saving, replaces what was typed.
    LaunchedEffect(value) { if (text.toDoubleOrNull() != value) text = shown(value) }
    val parsed = text.toDoubleOrNull()
    TextRow(
        label,
        text,
        { t ->
            text = t
            t.toDoubleOrNull()?.let { if (decimals || it == Math.rint(it)) onChange(it) }
        },
        detail = detail,
        keyboard = if (decimals) KeyboardType.Decimal else KeyboardType.Number,
        error = parsed == null || parsed !in range,
        fieldWidth = 80,
    )
}

/** Save, enabled while there are changes, and what saving said. */
@Composable
internal fun SaveRow(enabled: Boolean, busy: Boolean, notice: String?, failure: String?, onSave: () -> Unit) {
    Row(horizontalArrangement = Arrangement.spacedBy(MavioSpacing.s12), verticalAlignment = Alignment.CenterVertically) {
        MavioButton(if (busy) "…" else stringResource(R.string.action_save), onSave, enabled = enabled && !busy)
        when {
            failure != null -> Text(failure, style = theme.type.callout.text, color = theme.colors.danger)
            notice != null && !enabled -> Text(notice, style = theme.type.callout.text, color = theme.colors.textSecondary)
        }
    }
}

/** When something happened or will, relative to now, as "5 minutes ago". */
internal fun fromNow(t: Timestamp): String =
    t.instant?.let {
        // A server's clock a little ahead of the device's still reads as the past.
        val now = System.currentTimeMillis()
        DateUtils.getRelativeTimeSpanString(minOf(it.toEpochMilli(), now), now, DateUtils.MINUTE_IN_MILLIS).toString()
    }.orEmpty()

/**
 * The server's settings as a page edits them: read, changed and saved, each
 * change applied to the settings the server has when saving so that what
 * other pages changed in the meantime is kept.
 */
internal class SettingsEditor(private val model: AppModel, private val serverId: String) {
    var saved by mutableStateOf<GetServerSettingsResponse?>(null)
        private set
    var failure by mutableStateOf<String?>(null)
    var saving by mutableStateOf(false)
        private set
    var notice by mutableStateOf<String?>(null)
        private set

    private suspend fun get() =
        model.client.call(serverId, Procedure.system("GetServerSettings"), getServerSettingsRequest {}, GetServerSettingsResponse::parseFrom)

    suspend fun load() {
        try {
            saved = get()
        } catch (e: CancellationException) {
            throw e
        } catch (e: Exception) {
            failure = e.reason
        }
    }

    /** Saves a change, saying so with the notice given; the settings saved, or null. */
    suspend fun save(notice: String, change: (ServerSettings.Builder) -> Unit): ServerSettings? {
        saving = true
        failure = null
        try {
            val current = get()
            val settings = current.settings.toBuilder().also(change).build()
            val response = model.client.call(
                serverId,
                Procedure.system("UpdateServerSettings"),
                updateServerSettingsRequest { this.settings = settings },
                UpdateServerSettingsResponse::parseFrom,
            )
            saved = current.toBuilder().setSettings(response.settings).build()
            this.notice = notice
            return response.settings
        } catch (e: CancellationException) {
            throw e
        } catch (e: Exception) {
            failure = e.reason
            return null
        } finally {
            saving = false
        }
    }
}

/** The choices of a list, with the value chosen though the list lacks it. */
internal fun choices(list: List<Pair<String, String>>, chosen: String): List<Pair<String, String>> =
    if (chosen.isEmpty() || list.any { it.first == chosen }) list else list + (chosen to chosen)

/**
 * The languages and countries of a server's metadata, named in the user's
 * language where it can, by name, without repeats.
 */
internal suspend fun localization(model: AppModel, serverId: String): Pair<List<Pair<String, String>>, List<Pair<String, String>>> = coroutineScope {
    val langs = async {
        runCatching {
            model.client.call(serverId, Procedure.localization("ListLanguages"), listLanguagesRequest {}, ListLanguagesResponse::parseFrom)
        }.getOrNull()
    }
    val ctrs = async {
        runCatching {
            model.client.call(serverId, Procedure.localization("ListCountries"), listCountriesRequest {}, ListCountriesResponse::parseFrom)
        }.getOrNull()
    }
    val locale = Locale.getDefault()
    val collator = Collator.getInstance(locale)
    fun named(list: List<Pair<String, String>>, of: (String) -> String) = list
        .filter { it.first.isNotEmpty() }
        .distinctBy { it.first }
        .map { (code, name) -> code to of(code).takeIf { it.isNotEmpty() && it != code }.orEmptyOr(name) }
        .sortedWith { a, b -> collator.compare(a.second, b.second) }
    val languages = named(langs.await()?.languagesList?.map { it.code to it.name }.orEmpty()) { Locale.forLanguageTag(it).getDisplayLanguage(locale) }
    val countries = named(ctrs.await()?.countriesList?.map { it.code to it.name }.orEmpty()) { Locale("", it).getDisplayCountry(locale) }
    languages to countries
}

private fun String?.orEmptyOr(fallback: String) = this ?: fallback

/** A question before doing what cannot be undone, its action in red. */
@Composable
internal fun ConfirmDialog(title: String, message: String?, action: String, onConfirm: () -> Unit, onDismiss: () -> Unit) {
    androidx.compose.material3.AlertDialog(
        onDismissRequest = onDismiss,
        confirmButton = {
            androidx.compose.material3.TextButton({
                onDismiss()
                onConfirm()
            }) { Text(action, color = theme.colors.danger) }
        },
        dismissButton = {
            androidx.compose.material3.TextButton(onDismiss) { Text(stringResource(R.string.action_cancel), color = theme.colors.textPrimary) }
        },
        title = { Text(title, style = theme.type.headline.text) },
        text = message?.let { { Text(it, style = theme.type.callout.text, color = theme.colors.textSecondary) } },
        containerColor = theme.colors.surfaceRaised,
    )
}
