package dev.mavio.ui

import androidx.annotation.DrawableRes
import androidx.compose.foundation.isSystemInDarkTheme
import androidx.compose.foundation.layout.size
import androidx.compose.material3.LocalContentColor
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.darkColorScheme
import androidx.compose.material3.lightColorScheme
import androidx.compose.runtime.Composable
import androidx.compose.runtime.CompositionLocalProvider
import androidx.compose.runtime.Immutable
import androidx.compose.runtime.staticCompositionLocalOf
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.res.painterResource
import androidx.compose.ui.text.TextStyle
import androidx.compose.ui.unit.Dp
import androidx.compose.ui.unit.dp
import dev.mavio.design.DarkColors
import dev.mavio.design.LightColors
import dev.mavio.design.MavioColors
import dev.mavio.design.MavioLayout
import dev.mavio.design.MavioTypeStyle
import dev.mavio.design.MavioTypography
import dev.mavio.design.PhoneLayout
import dev.mavio.design.StandardTypography

/** The tokens a view draws with: the theme's colors, the form factor's layout and the type scale. */
@Immutable
data class Theme(
    val colors: MavioColors = DarkColors,
    val layout: MavioLayout = PhoneLayout,
    val type: MavioTypography = StandardTypography,
)

/** The theme of the composition. */
val LocalTheme = staticCompositionLocalOf { Theme() }

/** The theme of the composition, read where it is drawn. */
val theme: Theme
    @Composable get() = LocalTheme.current

/** The style of the system font in a style of the type scale. */
val MavioTypeStyle.text: TextStyle
    get() = TextStyle(fontSize = size, lineHeight = lineHeight, fontWeight = weight, letterSpacing = tracking)

/**
 * Provides the theme of a form factor in the system's appearance, dark
 * unless the system is light, and maps it onto Material's colors and type
 * for the controls Compose draws.
 */
@Composable
fun MavioTheme(
    dark: Boolean = isSystemInDarkTheme(),
    layout: MavioLayout = PhoneLayout,
    type: MavioTypography = StandardTypography,
    content: @Composable () -> Unit,
) {
    val colors = if (dark) DarkColors else LightColors
    val scheme = (if (dark) darkColorScheme() else lightColorScheme()).copy(
        // The accent marks only progress, selection, links and switches.
        primary = colors.accent,
        onPrimary = colors.onAccent,
        background = colors.background,
        onBackground = colors.textPrimary,
        surface = colors.background,
        onSurface = colors.textPrimary,
        surfaceVariant = colors.surface,
        onSurfaceVariant = colors.textSecondary,
        surfaceContainer = colors.surface,
        surfaceContainerLow = colors.surface,
        surfaceContainerHigh = colors.surfaceRaised,
        surfaceContainerHighest = colors.surfaceOverlay,
        outline = colors.border,
        outlineVariant = colors.border,
        error = colors.danger,
        scrim = colors.scrim,
    )
    val typography = MaterialTheme.typography.let {
        it.copy(
            displaySmall = type.display.text,
            headlineMedium = type.title1.text,
            headlineSmall = type.title2.text,
            titleLarge = type.title3.text,
            titleMedium = type.headline.text,
            bodyLarge = type.body.text,
            bodyMedium = type.callout.text,
            bodySmall = type.footnote.text,
            labelLarge = type.headline.text,
            labelMedium = type.caption.text,
        )
    }
    MaterialTheme(colorScheme = scheme, typography = typography) {
        CompositionLocalProvider(
            LocalTheme provides Theme(colors, layout, type),
            LocalContentColor provides colors.textPrimary,
            content = content,
        )
    }
}

/** An icon of the design system, drawn in the content color unless tinted. */
@Composable
fun MavioIcon(
    @DrawableRes icon: Int,
    modifier: Modifier = Modifier,
    size: Dp = 20.dp,
    tint: Color = LocalContentColor.current,
    contentDescription: String? = null,
) {
    androidx.compose.material3.Icon(
        painterResource(icon),
        contentDescription,
        modifier.size(size),
        tint = tint,
    )
}
