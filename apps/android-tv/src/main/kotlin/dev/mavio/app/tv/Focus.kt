package dev.mavio.app.tv

import androidx.annotation.DrawableRes
import androidx.compose.animation.core.animateFloatAsState
import androidx.compose.animation.core.tween
import androidx.compose.foundation.ExperimentalFoundationApi
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.clickable
import androidx.compose.foundation.focusGroup
import androidx.compose.foundation.gestures.BringIntoViewSpec
import androidx.compose.foundation.interaction.MutableInteractionSource
import androidx.compose.foundation.interaction.collectIsFocusedAsState
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.BoxWithConstraints
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.RowScope
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.CompositionLocalProvider
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.remember
import androidx.compose.runtime.withFrameNanos
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.draw.drawWithContent
import androidx.compose.ui.draw.shadow
import androidx.compose.ui.focus.FocusRequester
import androidx.compose.ui.focus.focusProperties
import androidx.compose.ui.focus.focusRequester
import androidx.compose.ui.focus.onFocusChanged
import androidx.compose.ui.geometry.CornerRadius
import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.geometry.Size
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.Shape
import androidx.compose.ui.graphics.drawscope.Stroke
import androidx.compose.ui.graphics.graphicsLayer
import androidx.compose.ui.platform.LocalDensity
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.Density
import androidx.compose.ui.unit.Dp
import androidx.compose.ui.unit.IntOffset
import androidx.compose.ui.unit.dp
import androidx.compose.ui.window.Popup
import androidx.compose.ui.window.PopupProperties
import androidx.compose.ui.zIndex
import dev.mavio.design.MavioRadius
import dev.mavio.design.MavioSpacing
import dev.mavio.ui.MavioIcon
import dev.mavio.ui.R
import dev.mavio.ui.text
import dev.mavio.ui.theme

// The television's canvas and focus (docs/tv.md §2, §4): the screen is
// 1920 points wide, as on Apple TV and in the web UI's television mode, and
// focus shows by scale and white rather than a ring.

/** The width of the canvas the television's tokens are given in, in points. */
private const val CANVAS_WIDTH = 1920f

/**
 * Draws [content] on a canvas 1920 points wide, whatever the screen's
 * density, so that the `tv` tokens lay out as on Apple TV and in the web
 * UI's television mode.
 */
@Composable
fun TvCanvas(content: @Composable () -> Unit) {
    BoxWithConstraints {
        val font = LocalDensity.current.fontScale
        val density = constraints.maxWidth / CANVAS_WIDTH
        CompositionLocalProvider(LocalDensity provides Density(density, font), content = content)
    }
}

/**
 * Scrolling that keeps the focused element in view (docs/tv.md §4):
 * [before] points past the start, as below the navigation bar, and
 * [after] before the end, with room for a card to grow.
 */
@OptIn(ExperimentalFoundationApi::class)
class TvScrolling(private val before: Float, private val after: Float) : BringIntoViewSpec {
    override fun calculateScrollDistance(offset: Float, size: Float, containerSize: Float): Float {
        val end = offset + size
        return when {
            // Taller than the room between, it shows from its start.
            size > containerSize - before - after -> offset - before
            offset < before -> offset - before
            end > containerSize - after -> end - (containerSize - after)
            else -> 0f
        }
    }
}

/** A focusable element's state: its interactions and whether it has focus. */
class Focusable {
    /** The element's interactions. */
    val interactions = MutableInteractionSource()
}

/** Whether the element has focus. */
@Composable
fun Focusable.focused(): Boolean {
    val focused by interactions.collectIsFocusedAsState()
    return focused
}

/**
 * The frame of a card's artwork (docs/tv.md §4): with focus, the card
 * scales by `layout.focusScale` above its neighbours, and its artwork gets
 * an accent border 1.5 points apart and a shadow.
 */
@Composable
fun Modifier.focusCard(focused: Boolean): Modifier {
    val scale by animateFloatAsState(if (focused) theme.layout.focusScale else 1f, tween(160), label = "scale")
    return this
        .zIndex(if (focused) 1f else 0f)
        .graphicsLayer {
            scaleX = scale
            scaleY = scale
        }
}

/** The border and shadow of a focused card's artwork, its corners rounded [radius]. */
@Composable
fun Modifier.focusArtwork(focused: Boolean, radius: Dp): Modifier {
    val accent = theme.colors.accent
    val shadow = theme.colors.tvFocusShadow
    val shape = RoundedCornerShape(radius)
    return this
        .drawWithContent {
            drawContent()
            if (!focused) return@drawWithContent
            // A 3-point stroke whose inner edge is 1.5 points out.
            val inset = 1.5.dp.toPx() + 1.5.dp.toPx()
            val r = radius.toPx() + inset
            drawRoundRect(
                accent,
                topLeft = Offset(-inset, -inset),
                size = Size(size.width + inset * 2, size.height + inset * 2),
                cornerRadius = CornerRadius(r, r),
                style = Stroke(3.dp.toPx()),
            )
        }
        // Unclipped, so that the border drawn around it shows.
        .then(if (focused) Modifier.shadow(32.dp, shape, clip = false, ambientColor = shadow, spotColor = shadow) else Modifier)
}

/**
 * A focusable that runs [onClick] when selected, showing focus as
 * [modifier] draws it from [state].
 */
fun Modifier.selectable(state: Focusable, onClick: () -> Unit): Modifier =
    clickable(state.interactions, indication = null, onClick = onClick)

/**
 * A button of the television (docs/tv.md §4): glass, or [fill] when given,
 * and white with black content once it has focus.
 */
@Composable
fun TvButton(
    label: String,
    onClick: () -> Unit,
    modifier: Modifier = Modifier,
    @DrawableRes icon: Int? = null,
    fill: Color = Color.White.copy(alpha = 0.2f),
    content: Color = Color.White,
    height: Dp = 64.dp,
    shape: Shape = RoundedCornerShape(10.dp),
) {
    val state = remember { Focusable() }
    val focused = state.focused()
    val color = if (focused) Color.Black else content
    Row(
        modifier
            .height(height)
            .then(if (focused) Modifier.shadow(12.dp, shape, ambientColor = theme.colors.tvFocusShadow, spotColor = theme.colors.tvFocusShadow) else Modifier)
            .clip(shape)
            .background(if (focused) Color.White else fill)
            .selectable(state, onClick)
            .padding(horizontal = MavioSpacing.s40),
        horizontalArrangement = Arrangement.spacedBy(MavioSpacing.s12, Alignment.CenterHorizontally),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        icon?.let { MavioIcon(it, size = 28.dp, tint = color) }
        Text(label, style = theme.type.headline.text, fontWeight = FontWeight.SemiBold, color = color, maxLines = 1)
    }
}

/**
 * A pill of a choice that focus makes, as a tab, a library's Recommended
 * and All or its filters (docs/tv.md §3, §6.3): filled with
 * `color.fillSelected` while current, white with focus, and chosen as
 * focus moves to it.
 */
@Composable
fun TvChoice(label: String, current: Boolean, modifier: Modifier = Modifier, horizontal: Dp = 28.dp, onChoose: () -> Unit) {
    val state = remember { Focusable() }
    val focused = state.focused()
    Text(
        label,
        modifier
            .clip(CircleShape)
            .background(
                when {
                    focused -> Color.White
                    current -> theme.colors.fillSelected
                    else -> Color.Transparent
                },
            )
            .onFocusChanged { if (it.isFocused && !current) onChoose() }
            .selectable(state, onChoose)
            .padding(horizontal = horizontal, vertical = 10.dp),
        style = theme.type.headline.text,
        color = when {
            focused -> Color.Black
            current -> theme.colors.textPrimary
            else -> theme.colors.textSecondary
        },
        maxLines = 1,
    )
}

/**
 * The pill of the tabs' glass holding a choice's [TvChoice]s; focus
 * entering it, from wherever, goes to [current] (docs/tv.md §4).
 */
@Composable
fun TvChoices(current: FocusRequester, modifier: Modifier = Modifier, content: @Composable RowScope.() -> Unit) {
    Row(
        modifier
            .clip(CircleShape)
            .background(theme.colors.surfaceRaised.copy(alpha = 0.6f))
            .focusProperties { onEnter = { current.requestFocus() } }
            .focusGroup()
            .padding(6.dp),
        horizontalArrangement = Arrangement.spacedBy(4.dp),
        content = content,
    )
}

/**
 * A menu of choices over the page (docs/tv.md §5), [width] wide below what
 * it is placed in, [below] its top: frosted glass whose items fill with
 * `color.fill` with focus, the current one checked. It opens with focus on
 * the current choice; back closes it.
 */
@Composable
fun <T> TvChoiceMenu(
    choices: List<T>,
    current: T,
    below: Dp,
    width: Dp,
    label: @Composable (T) -> String,
    onDismiss: () -> Unit,
    onChoose: (T) -> Unit,
) {
    val density = LocalDensity.current
    val chosen = remember { FocusRequester() }
    LaunchedEffect(Unit) {
        withFrameNanos {}
        runCatching { chosen.requestFocus() }
    }
    Popup(
        offset = with(density) { IntOffset(0, (below + MavioSpacing.s8).roundToPx()) },
        onDismissRequest = onDismiss,
        properties = PopupProperties(focusable = true),
    ) {
        Column(
            Modifier
                .width(width)
                .shadow(24.dp, RoundedCornerShape(MavioRadius.lg), ambientColor = theme.colors.tvFocusShadow, spotColor = theme.colors.tvFocusShadow)
                .clip(RoundedCornerShape(MavioRadius.lg))
                // Without a blur behind it, the glass is all but opaque.
                .background(theme.colors.surfaceOverlay.copy(alpha = 0.95f))
                .border(1.dp, theme.colors.border, RoundedCornerShape(MavioRadius.lg))
                .padding(MavioSpacing.s8),
            verticalArrangement = Arrangement.spacedBy(2.dp),
        ) {
            choices.forEach { c ->
                val state = remember { Focusable() }
                val focused = state.focused()
                Row(
                    (if (c == current) Modifier.focusRequester(chosen) else Modifier)
                        .fillMaxWidth()
                        .clip(RoundedCornerShape(MavioRadius.md))
                        .background(if (focused) theme.colors.fill else Color.Transparent)
                        .selectable(state) { onChoose(c) }
                        .padding(horizontal = MavioSpacing.s16, vertical = MavioSpacing.s12),
                    verticalAlignment = Alignment.CenterVertically,
                ) {
                    Text(label(c), Modifier.weight(1f), style = theme.type.headline.text, maxLines = 1, overflow = TextOverflow.Ellipsis)
                    if (c == current) MavioIcon(R.drawable.ic_check, size = 24.dp, tint = theme.colors.accentText)
                }
            }
        }
    }
}
