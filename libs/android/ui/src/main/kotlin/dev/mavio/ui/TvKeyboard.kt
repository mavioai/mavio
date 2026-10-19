package dev.mavio.ui

import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.RowScope
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.runtime.withFrameNanos
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.alpha
import androidx.compose.ui.draw.clip
import androidx.compose.ui.focus.FocusRequester
import androidx.compose.ui.focus.focusRequester
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.platform.LocalDensity
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.compose.ui.window.Dialog
import androidx.compose.ui.window.DialogProperties
import dev.mavio.design.MavioRadius
import dev.mavio.design.MavioSpacing
import kotlinx.coroutines.delay

/** What a field takes, which chooses the television keyboard's keys. */
enum class FieldKind {
    /** A line of text. */
    Text,

    /** Text of several lines, with a key for a new line. */
    Lines,

    /** A secret, shown as dots. */
    Secret,

    /** A whole number, on a keypad. */
    Integer,

    /** A number that may have a fraction, on a keypad with a point. */
    Decimal,
}

private val LETTERS = "abcdefghijklmnopqrstuvwxyz".map(Char::toString)
private val DIGITS = "1234567890".map(Char::toString)
private val SYMBOLS = "-_.@/:;,?!#\$%&*+=()[]{}<>'\"~|\\".map(Char::toString)

/**
 * The television's keyboard for a field (docs/tv.md §4), over the page and
 * named by [label]: keys reached with the D-pad, as Search's are, typing
 * into the field as they are pressed through [onChange]. Done keeps what was
 * typed; back deletes a character, and once there is none puts back
 * [value] as it was and closes through [onClose].
 */
@Composable
fun TvKeyboard(label: String, value: String, kind: FieldKind, onChange: (String) -> Unit, onClose: () -> Unit) {
    val original = remember { value }
    var text by remember { mutableStateOf(value) }
    var upper by remember { mutableStateOf(false) }
    var more by remember { mutableStateOf(false) }
    val first = remember { FocusRequester() }
    LaunchedEffect(Unit) {
        withFrameNanos {}
        runCatching { first.requestFocus() }
    }
    fun change(next: String) {
        text = next
        onChange(next)
    }
    fun type(c: String) = change(text + c)
    fun erase() = change(text.dropLast(1))
    val number = kind == FieldKind.Integer || kind == FieldKind.Decimal
    val chars = when {
        number -> "123456789".map(Char::toString) + listOf("-", "0") + if (kind == FieldKind.Decimal) listOf(".") else emptyList()
        more -> SYMBOLS
        else -> DIGITS + LETTERS + listOf("-", "_", ".", "@")
    }
    val canvas = LocalDensity.current
    Dialog(
        onDismissRequest = {
            if (text.isNotEmpty()) {
                erase()
            } else {
                onChange(original)
                onClose()
            }
        },
        properties = DialogProperties(usePlatformDefaultWidth = false),
    ) {
        OnCanvas(canvas) {
            Column(
                Modifier
                    .width(if (number) 440.dp else 820.dp)
                    .clip(RoundedCornerShape(MavioRadius.xl))
                    .background(theme.colors.surfaceOverlay)
                    .padding(MavioSpacing.s24),
                verticalArrangement = Arrangement.spacedBy(MavioSpacing.s16),
            ) {
                Text(label, style = theme.type.title3.text, color = theme.colors.textSecondary)
                Typed(if (kind == FieldKind.Secret) "•".repeat(text.length) else text, kind == FieldKind.Lines)
                chars.chunked(if (number) 3 else 10).forEachIndexed { i, line ->
                    Row(horizontalArrangement = Arrangement.spacedBy(MavioSpacing.s8)) {
                        line.forEachIndexed { j, c ->
                            val shown = if (upper) c.uppercase() else c
                            val key = Modifier.weight(1f)
                            Key(shown, if (i == 0 && j == 0) key.focusRequester(first) else key) { type(shown) }
                        }
                        // The last row of a keypad keeps its columns.
                        repeat((if (number) 3 else 10) - line.size) { Box(Modifier.weight(1f)) }
                    }
                }
                Row(horizontalArrangement = Arrangement.spacedBy(MavioSpacing.s8)) {
                    if (!number) {
                        Key(if (upper) "abc" else "ABC", Modifier.width(96.dp), enabled = !more) { upper = !upper }
                        Key(if (more) "abc" else "#+=", Modifier.width(96.dp)) { more = !more }
                        Key(stringResource(R.string.search_space), Modifier.weight(1f)) { type(" ") }
                        if (kind == FieldKind.Lines) Key("↵", Modifier.width(80.dp)) { type("\n") }
                    }
                    Key(stringResource(R.string.search_backspace), Modifier.weight(1f)) { erase() }
                    Key(stringResource(R.string.search_clear), Modifier.weight(1f)) { change("") }
                    Key(stringResource(R.string.action_done), Modifier.weight(1f), done = true, onClick = onClose)
                }
            }
        }
    }
}

/** What the keyboard has typed, with a blinking accent caret where it types. */
@Composable
private fun Typed(text: String, lines: Boolean) {
    var shown by remember { mutableStateOf(true) }
    LaunchedEffect(text) {
        shown = true
        while (true) {
            delay(500)
            shown = !shown
        }
    }
    Row(
        Modifier
            .fillMaxWidth()
            .heightIn(min = 64.dp)
            .background(theme.colors.surfaceRaised, RoundedCornerShape(MavioRadius.lg))
            .padding(horizontal = 20.dp, vertical = MavioSpacing.s12),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        Text(
            text,
            Modifier.weight(1f, fill = false).heightIn(max = 160.dp),
            style = theme.type.title3.text,
            maxLines = if (lines) 4 else 1,
            overflow = if (lines) TextOverflow.Clip else TextOverflow.StartEllipsis,
        )
        Box(
            Modifier
                .padding(horizontal = 1.dp)
                .width(2.dp)
                .height(30.dp)
                .alpha(if (shown) 1f else 0f)
                .background(theme.colors.accent, RoundedCornerShape(1.dp)),
        )
    }
}

/** A key: `color.fill`, or `color.button` for Done, white with black text with focus. */
@Composable
private fun RowScope.Key(label: String, modifier: Modifier, enabled: Boolean = true, done: Boolean = false, onClick: () -> Unit) {
    val state = remember { Focusable() }
    val focused = state.focused()
    val fill = when {
        focused -> Color.White
        done -> theme.colors.button
        else -> theme.colors.fill
    }
    val ink = when {
        focused -> Color.Black
        done -> theme.colors.onButton
        else -> theme.colors.textPrimary
    }
    Box(
        modifier
            .height(56.dp)
            .alpha(if (enabled) 1f else 0.4f)
            .clip(RoundedCornerShape(MavioRadius.lg))
            .background(fill)
            .selectable(state) { if (enabled) onClick() },
        Alignment.Center,
    ) {
        Text(label, style = theme.type.headline.text, color = ink, maxLines = 1)
    }
}
