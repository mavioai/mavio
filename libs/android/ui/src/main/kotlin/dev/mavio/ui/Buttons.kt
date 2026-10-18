package dev.mavio.ui

import androidx.annotation.DrawableRes
import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.interaction.MutableInteractionSource
import androidx.compose.foundation.interaction.collectIsPressedAsState
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.defaultMinSize
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.LocalContentColor
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.CompositionLocalProvider
import androidx.compose.runtime.getValue
import androidx.compose.runtime.remember
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.alpha
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.semantics.Role
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import dev.mavio.design.MavioRadius
import dev.mavio.design.MavioSpacing

/** The kinds of the web UI's buttons; the accent is left to progress, selection, links and switches. */
enum class ButtonKind {
    /** The screen's main action, filled with `color.button`. */
    Primary,

    /** Another action, filled with `color.fill`. */
    Secondary,

    /** Over artwork: a translucent white. */
    Glass,
}

/**
 * A button of a kind with a label and an optional icon before it; large is
 * the 44 dp of a screen's main actions on a phone.
 */
@Composable
fun MavioButton(
    label: String,
    onClick: () -> Unit,
    modifier: Modifier = Modifier,
    kind: ButtonKind = ButtonKind.Primary,
    @DrawableRes icon: Int? = null,
    large: Boolean = false,
    enabled: Boolean = true,
) {
    val colors = theme.colors
    val source = remember { MutableInteractionSource() }
    val pressed by source.collectIsPressedAsState()
    val background = when (kind) {
        ButtonKind.Primary -> colors.button
        ButtonKind.Secondary -> colors.fill
        ButtonKind.Glass -> Color.White.copy(alpha = 0.2f)
    }
    val foreground = when {
        !enabled -> colors.textDisabled
        kind == ButtonKind.Primary -> colors.onButton
        kind == ButtonKind.Glass -> Color.White
        else -> colors.textPrimary
    }
    val shape = RoundedCornerShape(if (large) 10.dp else MavioRadius.md)
    Row(
        modifier
            .defaultMinSize(minHeight = if (large) 44.dp else 36.dp)
            .alpha(if (!enabled) 0.6f else if (pressed) 0.8f else 1f)
            .clip(shape)
            .background(background)
            .clickable(source, null, enabled = enabled, role = Role.Button, onClick = onClick)
            .padding(horizontal = if (large) MavioSpacing.s20 else MavioSpacing.s16),
        horizontalArrangement = Arrangement.spacedBy(MavioSpacing.s8, Alignment.CenterHorizontally),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        CompositionLocalProvider(LocalContentColor provides foreground) {
            icon?.let { MavioIcon(it, size = if (large) 22.dp else 18.dp) }
            Text(
                label,
                style = theme.type.headline.text,
                fontWeight = FontWeight.SemiBold,
                maxLines = 1,
                overflow = TextOverflow.Ellipsis,
            )
        }
    }
}

/** A round button of an icon in a circle filled with `color.fill`, 44 dp wide. */
@Composable
fun CircleButton(
    @DrawableRes icon: Int,
    contentDescription: String,
    onClick: () -> Unit,
    modifier: Modifier = Modifier,
    tint: Color = theme.colors.textPrimary,
    background: Color = theme.colors.fill,
    enabled: Boolean = true,
) {
    Box(
        modifier
            .size(44.dp)
            .clip(CircleShape)
            .background(background)
            .clickable(enabled = enabled, role = Role.Button, onClick = onClick),
        Alignment.Center,
    ) {
        MavioIcon(icon, size = 22.dp, tint = tint, contentDescription = contentDescription)
    }
}
