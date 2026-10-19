package dev.mavio.ui

import androidx.compose.animation.core.Animatable
import androidx.compose.animation.core.EaseOut
import androidx.compose.animation.core.keyframes
import androidx.compose.runtime.Stable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.setValue
import androidx.compose.runtime.staticCompositionLocalOf
import androidx.compose.ui.Modifier
import androidx.compose.ui.focus.FocusEventModifierNode
import androidx.compose.ui.focus.FocusState
import androidx.compose.ui.layout.Measurable
import androidx.compose.ui.layout.MeasureResult
import androidx.compose.ui.layout.MeasureScope
import androidx.compose.ui.node.CompositionLocalConsumerModifierNode
import androidx.compose.ui.node.LayoutModifierNode
import androidx.compose.ui.node.ModifierNodeElement
import androidx.compose.ui.node.currentValueOf
import androidx.compose.ui.unit.Constraints
import androidx.compose.ui.unit.dp
import kotlin.math.roundToInt

/**
 * The shake of the focused element where focus cannot move (docs/tv.md
 * §4): sideways for 320 ms, as the web UI's, by translation so that a card
 * keeps its scale. The app's root starts it; the focusables that [shakes]
 * follow it.
 */
@Stable
class FocusShake {
    /** The focused element's translation, in dp. */
    internal val offset = Animatable(0f)

    /** Shakes the focused element. */
    suspend fun shake() {
        offset.snapTo(0f)
        offset.animateTo(
            0f,
            keyframes {
                durationMillis = 320
                0f at 0 using EaseOut
                -10f at 64 using EaseOut
                8f at 128 using EaseOut
                -5f at 192 using EaseOut
                3f at 256 using EaseOut
            },
        )
    }
}

/** The television's [FocusShake]; none on a phone. */
val LocalFocusShake = staticCompositionLocalOf<FocusShake?> { null }

/**
 * Shakes this focusable with [LocalFocusShake] while it has focus; put
 * first among its modifiers, so that all it draws moves.
 */
fun Modifier.shakes(): Modifier = this then ShakesElement

private data object ShakesElement : ModifierNodeElement<ShakesNode>() {
    override fun create() = ShakesNode()

    override fun update(node: ShakesNode) = Unit
}

private class ShakesNode :
    Modifier.Node(),
    FocusEventModifierNode,
    LayoutModifierNode,
    CompositionLocalConsumerModifierNode {
    private var focused by mutableStateOf(false)

    override fun onFocusEvent(focusState: FocusState) {
        focused = focusState.isFocused
    }

    override fun MeasureScope.measure(measurable: Measurable, constraints: Constraints): MeasureResult {
        val placeable = measurable.measure(constraints)
        val shake = currentValueOf(LocalFocusShake)
        return layout(placeable.width, placeable.height) {
            // Read as it places, so that the shake places it again alone.
            val x = if (focused && shake != null) shake.offset.value.dp.toPx().roundToInt() else 0
            placeable.place(x, 0)
        }
    }
}
