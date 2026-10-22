package dev.mavio.admin

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
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import com.google.protobuf.Timestamp
import dev.mavio.core.AppModel
import dev.mavio.core.call
import dev.mavio.core.instant
import dev.mavio.design.MavioSpacing
import dev.mavio.ui.ButtonKind
import dev.mavio.ui.LocalTelevision
import dev.mavio.ui.Pressable
import dev.mavio.ui.R
import dev.mavio.ui.RowLabel
import dev.mavio.ui.SettingsButton
import dev.mavio.ui.TvButton
import dev.mavio.ui.TvDialog
import dev.mavio.ui.TvKeyboard
import dev.mavio.ui.fieldKind
import dev.mavio.ui.rowPadding
import dev.mavio.ui.text
import dev.mavio.ui.theme
import kotlinx.coroutines.CancellationException
import mavio.playback.v1.Playback.GetTranscodingSettingsResponse
import mavio.playback.v1.Playback.UpdateTranscodingSettingsResponse
import mavio.playback.v1.getTranscodingSettingsRequest
import mavio.playback.v1.updateTranscodingSettingsRequest
import mavio.system.v1.System.GetGeneralSettingsResponse
import mavio.system.v1.System.UpdateGeneralSettingsResponse
import mavio.system.v1.getGeneralSettingsRequest
import mavio.system.v1.updateGeneralSettingsRequest

/** A text field's row, its value right of its label. */
@Composable
fun TextRow(
    label: String,
    value: String,
    onChange: (String) -> Unit,
    detail: String? = null,
    placeholder: String? = null,
    keyboard: KeyboardType = KeyboardType.Text,
    error: Boolean = false,
    fieldWidth: Int = 170,
) {
    // On a television the row opens the keyboard, its value shown at its end.
    if (LocalTelevision.current) {
        var typing by remember { mutableStateOf(false) }
        Pressable({ typing = true }) {
            Row(
                Modifier.fillMaxWidth().heightIn(min = 72.dp).padding(horizontal = rowPadding, vertical = 8.dp),
                horizontalArrangement = Arrangement.spacedBy(MavioSpacing.s12),
                verticalAlignment = Alignment.CenterVertically,
            ) {
                RowLabel(label, detail, Modifier.weight(1f))
                Text(
                    value.ifEmpty { placeholder.orEmpty() },
                    Modifier.widthIn(max = (fieldWidth * 2).dp),
                    style = theme.type.body.text,
                    color = when {
                        error -> theme.colors.danger
                        value.isEmpty() -> theme.colors.textTertiary
                        else -> theme.colors.textSecondary
                    },
                    textAlign = TextAlign.End,
                    maxLines = 1,
                    overflow = TextOverflow.Ellipsis,
                )
            }
        }
        if (typing) TvKeyboard(label, value, fieldKind(keyboard), onChange) { typing = false }
        return
    }
    Row(
        Modifier.fillMaxWidth().heightIn(min = 52.dp).padding(horizontal = rowPadding, vertical = 8.dp),
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
fun NumberRow(
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
fun SaveRow(enabled: Boolean, busy: Boolean, notice: String?, failure: String?, onSave: () -> Unit) {
    Row(horizontalArrangement = Arrangement.spacedBy(MavioSpacing.s12), verticalAlignment = Alignment.CenterVertically) {
        SettingsButton(if (busy) "…" else stringResource(R.string.action_save), onSave, enabled = enabled && !busy)
        when {
            failure != null -> Text(failure, style = theme.type.callout.text, color = theme.colors.danger)
            notice != null && !enabled -> Text(notice, style = theme.type.callout.text, color = theme.colors.textSecondary)
        }
    }
}

/** When something happened or will, relative to now, as "5 minutes ago". */
fun fromNow(t: Timestamp): String =
    t.instant?.let {
        // A server's clock a little ahead of the device's still reads as the past.
        val now = System.currentTimeMillis()
        DateUtils.getRelativeTimeSpanString(minOf(it.toEpochMilli(), now), now, DateUtils.MINUTE_IN_MILLIS).toString()
    }.orEmpty()

/**
 * A group of the server's settings as a page edits them, on the service of
 * their domain: [read] gives the response holding them ([settingsOf]),
 * [write] replaces them and gives them back, [replace] puts them back in
 * the response. Each change is applied to the settings the server has when
 * saving, so that what another administrator changed in the meantime is
 * kept.
 */
class SettingsEditor<R, S>(
    private val read: suspend () -> R,
    private val settingsOf: (R) -> S,
    private val write: suspend (S) -> S,
    private val replace: (R, S) -> R,
) {
    var saved by mutableStateOf<R?>(null)
        private set
    var failure by mutableStateOf<String?>(null)
    var saving by mutableStateOf(false)
        private set
    var notice by mutableStateOf<String?>(null)
        private set

    suspend fun load() {
        try {
            saved = read()
        } catch (e: CancellationException) {
            throw e
        } catch (e: Exception) {
            failure = e.reason
        }
    }

    /** Saves a change, saying so with the notice given; the settings saved, or null. */
    suspend fun save(notice: String, change: (S) -> S): S? {
        saving = true
        failure = null
        try {
            val current = read()
            val settings = write(change(settingsOf(current)))
            saved = replace(current, settings)
            this.notice = notice
            return settings
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

/** The server's general settings as a page edits them. */
fun generalEditor(model: AppModel, serverId: String) = SettingsEditor(
    read = { model.client.call(serverId, Procedure.system("GetGeneralSettings"), getGeneralSettingsRequest {}, GetGeneralSettingsResponse::parseFrom) },
    settingsOf = { it.settings },
    write = { settings ->
        model.client.call(
            serverId,
            Procedure.system("UpdateGeneralSettings"),
            updateGeneralSettingsRequest { this.settings = settings },
            UpdateGeneralSettingsResponse::parseFrom,
        ).settings
    },
    replace = { response, settings -> response.toBuilder().setSettings(settings).build() },
)

/** The server's transcoding settings as a page edits them. */
fun transcodingEditor(model: AppModel, serverId: String) = SettingsEditor(
    read = { model.client.call(serverId, Procedure.playback("GetTranscodingSettings"), getTranscodingSettingsRequest {}, GetTranscodingSettingsResponse::parseFrom) },
    settingsOf = { it.settings },
    write = { settings ->
        model.client.call(
            serverId,
            Procedure.playback("UpdateTranscodingSettings"),
            updateTranscodingSettingsRequest { this.settings = settings },
            UpdateTranscodingSettingsResponse::parseFrom,
        ).settings
    },
    replace = { response, settings -> response.toBuilder().setSettings(settings).build() },
)

