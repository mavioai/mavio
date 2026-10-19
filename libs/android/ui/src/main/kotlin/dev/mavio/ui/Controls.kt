package dev.mavio.ui

import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.lazy.grid.GridItemSpan
import androidx.compose.foundation.lazy.grid.LazyGridScope
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.DropdownMenu
import androidx.compose.material3.DropdownMenuItem
import androidx.compose.material3.LocalContentColor
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.CompositionLocalProvider
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableIntStateOf
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.draw.shadow
import androidx.compose.ui.focus.FocusRequester
import androidx.compose.ui.focus.focusRequester
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.Shape
import androidx.compose.ui.layout.onSizeChanged
import androidx.compose.ui.platform.LocalDensity
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.Dp
import androidx.compose.ui.unit.dp
import dev.mavio.design.MavioSpacing

/** An item of a grid across its whole width. */
fun LazyGridScope.line(key: String, content: @Composable () -> Unit) =
    item(key = key, span = { GridItemSpan(maxLineSpan) }) { content() }

/** A choice of a few, side by side in a well, the chosen one raised, as the web UI's. */
@Composable
fun Segmented(labels: List<String>, chosen: Int, onChoose: (Int) -> Unit) {
    // The television's are pills chosen as focus moves to them.
    if (LocalTelevision.current) {
        val current = remember { FocusRequester() }
        TvChoices(current) {
            labels.forEachIndexed { i, label ->
                TvChoice(label, i == chosen, if (i == chosen) Modifier.focusRequester(current) else Modifier) { onChoose(i) }
            }
        }
        return
    }
    val colors = theme.colors
    Row(
        Modifier
            .clip(RoundedCornerShape(9.dp))
            .background(colors.fill)
            .padding(2.dp),
    ) {
        labels.forEachIndexed { i, label ->
            val active = i == chosen
            Text(
                label,
                Modifier
                    .then(if (active) Modifier.shadow(1.dp, RoundedCornerShape(7.dp)) else Modifier)
                    .clip(RoundedCornerShape(7.dp))
                    .background(if (active) colors.surfaceOverlay else Color.Transparent)
                    .clickable { onChoose(i) }
                    .padding(horizontal = MavioSpacing.s16, vertical = MavioSpacing.s4),
                style = theme.type.callout.text,
                fontWeight = FontWeight.SemiBold,
                color = if (active) colors.textPrimary else colors.textSecondary,
            )
        }
    }
}

/**
 * What is pressed beside a row's content, a link's text or an icon, in
 * [shape]: on a television it takes focus and turns white with black
 * content, as every focused element does (docs/tv.md §4).
 */
@Composable
fun PressableBox(
    onClick: () -> Unit,
    modifier: Modifier = Modifier,
    enabled: Boolean = true,
    onClickLabel: String? = null,
    shape: Shape = RoundedCornerShape(10.dp),
    content: @Composable () -> Unit,
) {
    if (!LocalTelevision.current) {
        Box(modifier.clip(shape).clickable(enabled = enabled, onClickLabel = onClickLabel, onClick = onClick), Alignment.Center) { content() }
        return
    }
    val state = remember { Focusable() }
    val focused = state.focused()
    Box(
        modifier
            .shakes()
            .then(if (focused) Modifier.shadow(12.dp, shape, ambientColor = theme.colors.tvFocusShadow, spotColor = theme.colors.tvFocusShadow) else Modifier)
            .clip(shape)
            .background(if (focused) Color.White else Color.Transparent)
            .recallsFocus()
            .clickable(state.interactions, null, enabled = enabled, onClickLabel = onClickLabel, onClick = onClick),
        Alignment.Center,
    ) {
        if (focused) OnWhite(content) else content()
    }
}

/**
 * [anchor], which opens a menu of [choices] below it when pressed, the
 * [chosen] one checked: the phone's drops down at it, the television's
 * opens [width] wide at its right with focus on the chosen one, else the
 * first.
 */
@Composable
fun <T> ChoiceMenuBox(
    choices: List<Pair<T, String>>,
    chosen: T?,
    onChoose: (T) -> Unit,
    modifier: Modifier = Modifier,
    width: Dp = 480.dp,
    anchor: @Composable (open: () -> Unit) -> Unit,
) {
    var open by remember { mutableStateOf(false) }
    val tv = LocalTelevision.current
    var height by remember { mutableIntStateOf(0) }
    val page = theme
    val ink = LocalContentColor.current
    Box(modifier.onSizeChanged { height = it.height }) {
        anchor { open = true }
        if (!tv) {
            DropdownMenu(open, onDismissRequest = { open = false }) {
                choices.forEach { (value, text) ->
                    DropdownMenuItem(
                        text = { Text(text, style = theme.type.callout.text) },
                        onClick = {
                            open = false
                            onChoose(value)
                        },
                        trailingIcon = { if (value == chosen) MavioIcon(R.drawable.ic_check, size = 18.dp) },
                    )
                }
            }
        } else if (open) {
            val first = remember { FocusRequester() }
            val initial = choices.indexOfFirst { it.first == chosen }.coerceAtLeast(0)
            CompositionLocalProvider(LocalTheme provides page, LocalContentColor provides ink) {
                TvMenu(first, width, Alignment.TopEnd, with(LocalDensity.current) { height.toDp() } + MavioSpacing.s8, { open = false }) {
                    choices.forEachIndexed { i, (value, text) ->
                        TvMenuItem(
                            {
                                open = false
                                onChoose(value)
                            },
                            if (i == initial) Modifier.focusRequester(first) else Modifier,
                        ) {
                            Text(text, Modifier.weight(1f), style = theme.type.headline.text, maxLines = 1, overflow = TextOverflow.Ellipsis)
                            if (value == chosen) MavioIcon(R.drawable.ic_check, size = 24.dp, tint = theme.colors.accentText)
                        }
                    }
                }
            }
        }
    }
}
