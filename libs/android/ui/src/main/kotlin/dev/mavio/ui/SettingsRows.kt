package dev.mavio.ui

import androidx.annotation.DrawableRes
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.clickable
import androidx.compose.foundation.focusGroup
import androidx.compose.foundation.focusable
import androidx.compose.foundation.gestures.animateScrollBy
import androidx.compose.foundation.interaction.MutableInteractionSource
import androidx.compose.foundation.interaction.collectIsFocusedAsState
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.ColumnScope
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.WindowInsets
import androidx.compose.foundation.layout.asPaddingValues
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.navigationBars
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.widthIn
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.LazyListScope
import androidx.compose.foundation.lazy.rememberLazyListState
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.text.BasicTextField
import androidx.compose.foundation.text.KeyboardOptions
import androidx.compose.material3.DropdownMenu
import androidx.compose.material3.HorizontalDivider
import androidx.compose.material3.LocalContentColor
import androidx.compose.material3.Switch
import androidx.compose.material3.SwitchDefaults
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.CompositionLocalProvider
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableIntStateOf
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.runtime.setValue
import androidx.compose.runtime.snapshotFlow
import androidx.compose.runtime.staticCompositionLocalOf
import androidx.compose.runtime.withFrameNanos
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.alpha
import androidx.compose.ui.draw.clip
import androidx.compose.ui.draw.shadow
import androidx.compose.ui.focus.FocusDirection
import androidx.compose.ui.focus.FocusRequester
import androidx.compose.ui.focus.focusRequester
import androidx.compose.ui.focus.onFocusChanged
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.SolidColor
import androidx.compose.ui.input.key.Key
import androidx.compose.ui.input.key.KeyEventType
import androidx.compose.ui.input.key.key
import androidx.compose.ui.input.key.onKeyEvent
import androidx.compose.ui.input.key.type
import androidx.compose.ui.layout.boundsInWindow
import androidx.compose.ui.layout.layout
import androidx.compose.ui.layout.onGloballyPositioned
import androidx.compose.ui.layout.onSizeChanged
import androidx.compose.ui.platform.LocalDensity
import androidx.compose.ui.platform.LocalFocusManager
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.input.KeyboardType
import androidx.compose.ui.text.input.PasswordVisualTransformation
import androidx.compose.ui.text.input.VisualTransformation
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.Dp
import androidx.compose.ui.unit.dp
import dev.mavio.design.LightColors
import dev.mavio.design.MavioRadius
import dev.mavio.design.MavioSpacing
import kotlin.time.Duration.Companion.seconds
import kotlinx.coroutines.flow.first
import kotlinx.coroutines.launch
import kotlinx.coroutines.withTimeoutOrNull

// The rows the settings are made of, on a phone, a tablet and a television
// alike: on a television each row takes focus and turns white with it, as
// the web UI's do in its television mode (docs/tv.md §6.6), a choice opens
// a menu of choices and a field opens the keyboard (docs/tv.md §4, §5).

/** The horizontal padding of a row. */
val rowPadding: Dp
    @Composable get() = if (LocalTelevision.current) 24.dp else 16.dp

/** The least height of a row, [phone]'s on a phone. */
@Composable
private fun rowHeight(phone: Dp): Dp = if (LocalTelevision.current) phone + 20.dp else phone

/**
 * [content] run when selected through [onClick]: tapped on a phone, and on
 * a television focusable, white with black content while it has focus,
 * rounded as its group is and raised by a shadow.
 */
@Composable
fun Pressable(onClick: () -> Unit, modifier: Modifier = Modifier, content: @Composable () -> Unit) {
    if (!LocalTelevision.current) {
        Box(modifier.clickable(onClick = onClick)) { content() }
        return
    }
    val state = remember { Focusable() }
    val focused = state.focused()
    val shape = RoundedCornerShape(14.dp)
    Box(
        modifier
            .shakes()
            .then(if (focused) Modifier.shadow(12.dp, shape, ambientColor = theme.colors.tvFocusShadow, spotColor = theme.colors.tvFocusShadow) else Modifier)
            .background(if (focused) Color.White else Color.Transparent, shape)
            .recallsFocus()
            .selectable(state, onClick),
    ) {
        if (focused) OnWhite(content) else content()
    }
}

/** [content] drawn on white, its text black. */
@Composable
fun OnWhite(content: @Composable () -> Unit) {
    val colors = theme.colors.copy(
        textPrimary = Color.Black,
        textSecondary = LightColors.textSecondary,
        textTertiary = LightColors.textTertiary,
        accentText = Color.Black,
        border = LightColors.border,
        fill = LightColors.fill,
    )
    CompositionLocalProvider(LocalTheme provides theme.copy(colors = colors), LocalContentColor provides Color.Black, content = content)
}

/**
 * A page of settings: its title, an optional message, then its groups;
 * read again by pulling it down when it has [refresh]. On a television it
 * is a centered column, as the web UI's is, and opens with focus on its
 * first element unless something in it has taken focus; with nothing to
 * focus, the page takes focus itself and up and down scroll it.
 */
@Composable
fun SettingsList(
    title: String,
    message: String? = null,
    large: Boolean = false,
    refresh: (suspend () -> Unit)? = null,
    content: LazyListScope.() -> Unit,
) {
    val tv = LocalTelevision.current
    val requester = remember { FocusRequester() }
    var within by remember { mutableStateOf(false) }
    // Whether the page itself has focus, holding nothing to focus.
    var alone by remember { mutableStateOf(false) }
    var fallback by remember { mutableStateOf(false) }
    val list = rememberLazyListState()
    val recall = LocalFocusRecall.current
    if (recall != null) {
        // Where the list was scrolled, for its return: its rows load again.
        var spot by rememberSaveable { mutableStateOf(intArrayOf(0, 0)) }
        LaunchedEffect(list) {
            val (index, offset) = spot
            if (index > 0 || offset > 0) {
                withTimeoutOrNull(3.seconds) { snapshotFlow { list.layoutInfo.totalItemsCount }.first { it > index } }
                list.scrollToItem(index, offset)
            }
            snapshotFlow { list.firstVisibleItemIndex to list.firstVisibleItemScrollOffset }.collect { spot = intArrayOf(it.first, it.second) }
        }
    }
    val scope = rememberCoroutineScope()
    val focusManager = LocalFocusManager.current
    val step = with(LocalDensity.current) { 240.dp.toPx() }
    if (tv) {
        LaunchedEffect(Unit) {
            // Its rows are laid out a frame or two after it.
            repeat(5) {
                withFrameNanos {}
                if (within) return@LaunchedEffect
                runCatching { requester.requestFocus(FocusDirection.Enter) }
            }
            fallback = true
            withFrameNanos {}
            if (!within) runCatching { requester.requestFocus() }
        }
    }
    // Without refresh, pulling does nothing.
    PullToRefresh(refresh = refresh ?: {}, enabled = refresh != null) {
        Box(Modifier.fillMaxSize(), Alignment.TopCenter) {
            LazyColumn(
                Modifier
                    .then(if (tv) Modifier.widthIn(max = 1000.dp) else Modifier)
                    .fillMaxSize()
                    .focusRequester(requester)
                    .onFocusChanged {
                        within = it.hasFocus
                        alone = it.isFocused
                        // Once a row has focus the page no longer takes it, so
                        // that back, which leaves the focused element, leaves the page.
                        if (it.hasFocus && !it.isFocused) fallback = false
                    }
                    .then(
                        if (fallback) {
                            Modifier
                                .onKeyEvent { e ->
                                    val by = when (e.key) {
                                        Key.DirectionDown -> step
                                        Key.DirectionUp -> -step
                                        else -> return@onKeyEvent false
                                    }
                                    if (!alone) return@onKeyEvent false
                                    if (e.type == KeyEventType.KeyDown && !focusManager.moveFocus(FocusDirection.Enter)) {
                                        scope.launch { list.animateScrollBy(by) }
                                    }
                                    true
                                }
                                .focusable()
                        } else {
                            Modifier
                        },
                    )
                    .focusGroup(),
                state = list,
                contentPadding = if (tv) {
                    PaddingValues(start = MavioSpacing.s24, end = MavioSpacing.s24, top = MavioSpacing.s64, bottom = MavioSpacing.s64)
                } else {
                    PaddingValues(
                        start = theme.layout.margin,
                        end = theme.layout.margin,
                        top = MavioSpacing.s16,
                        // No tab bar below the settings.
                        bottom = MavioSpacing.s24 + WindowInsets.navigationBars.asPaddingValues().calculateBottomPadding(),
                    )
                },
                verticalArrangement = Arrangement.spacedBy(listGap),
            ) {
                item {
                    Column(
                        Modifier.pageTitle(title),
                        verticalArrangement = Arrangement.spacedBy(MavioSpacing.s4),
                    ) {
                        val heading = @Composable { modifier: Modifier ->
                            Text(title, modifier, style = (if (large) theme.type.title1 else theme.type.title2).text, fontWeight = FontWeight.Bold)
                        }
                        val accessory = LocalTitleAccessory.current
                        if (accessory == null) {
                            heading(Modifier)
                        } else {
                            Row(horizontalArrangement = Arrangement.spacedBy(MavioSpacing.s16), verticalAlignment = Alignment.CenterVertically) {
                                heading(Modifier.weight(1f))
                                accessory()
                            }
                        }
                        if (message != null) Text(message, style = theme.type.callout.text, color = theme.colors.textSecondary)
                    }
                }
                content()
            }
        }
    }
}

/**
 * Where a page's large title is, for a header that shows it once the page
 * has scrolled it under: the title and where it ends in the window, or the
 * page gone with null.
 */
fun interface PageTitles {
    /** Reports the page's [title] ending [bottom] pixels down the window, or none. */
    fun report(title: String?, bottom: Float)
}

/** What a [SettingsList] shows at the right of its title, as the web UI's choice of server on a phone. */
val LocalTitleAccessory = staticCompositionLocalOf<(@Composable () -> Unit)?> { null }

/** Where the page shown reports its large title, if anywhere. */
val LocalPageTitles = staticCompositionLocalOf<PageTitles?> { null }

/** Marks this as the page's large [title], which a header shows once the page has scrolled it under. */
@Composable
fun Modifier.pageTitle(title: String): Modifier {
    val titles = LocalPageTitles.current ?: return this
    return onGloballyPositioned { titles.report(title, it.boundsInWindow().bottom) }
}

/** The spacing of a [SettingsList]'s items. */
private val listGap: Dp
    @Composable get() = if (LocalTelevision.current) MavioSpacing.s32 else MavioSpacing.s24

/**
 * Reported shorter by the list's spacing, less [after], so that the item
 * following it in a [SettingsList] starts [after] below it.
 */
@Composable
private fun Modifier.followedAt(after: Dp): Modifier {
    val cut = listGap - after
    return layout { measurable, constraints ->
        val placeable = measurable.measure(constraints)
        layout(placeable.width, (placeable.height - cut.roundToPx()).coerceAtLeast(0)) { placeable.place(0, 0) }
    }
}

/**
 * A group's title as an item of a [SettingsList] of its own, for a group
 * drawn as [GroupPart]s: as far above the card as [Group]'s.
 */
@Composable
fun GroupTitle(title: String) {
    Text(title, Modifier.followedAt(MavioSpacing.s8).padding(horizontal = 4.dp), style = theme.type.headline.text)
}

/**
 * A run of a group's rows as an item of a [SettingsList] of its own, so that
 * a long group composes only the rows scrolled into view: the card rounded
 * at its [top] and its [bottom], and the next part joined to it, or below the
 * [bottom] what follows [after] it, a footer [MavioSpacing.s8] as in [Group].
 */
@Composable
fun GroupPart(top: Boolean, bottom: Boolean, after: Dp = listGap, content: @Composable ColumnScope.() -> Unit) {
    val corner = 14.dp
    val shape = RoundedCornerShape(
        topStart = if (top) corner else 0.dp,
        topEnd = if (top) corner else 0.dp,
        bottomStart = if (bottom) corner else 0.dp,
        bottomEnd = if (bottom) corner else 0.dp,
    )
    Column(
        Modifier
            .followedAt(if (bottom) after else 0.dp)
            .fillMaxWidth()
            // On a television a focused row's shadow reaches past the card.
            .then(if (LocalTelevision.current) Modifier else Modifier.clip(shape))
            .background(theme.colors.surface, shape),
        content = content,
    )
}

/** A group's footer as an item of a [SettingsList] of its own, below the last [GroupPart]. */
@Composable
fun GroupFooter(footer: String) {
    Text(footer, Modifier.padding(horizontal = 4.dp), style = theme.type.footnote.text, color = theme.colors.textSecondary)
}

/** A group of rows on a card, with an optional title above, an action beside it, and a footer below. */
@Composable
fun Group(
    title: String? = null,
    footer: String? = null,
    action: (@Composable () -> Unit)? = null,
    content: @Composable ColumnScope.() -> Unit,
) {
    Column(verticalArrangement = Arrangement.spacedBy(MavioSpacing.s8)) {
        if (title != null || action != null) {
            Row(Modifier.padding(horizontal = 4.dp), verticalAlignment = Alignment.CenterVertically) {
                Text(title.orEmpty(), Modifier.weight(1f), style = theme.type.headline.text)
                action?.invoke()
            }
        }
        // On a television a focused row's shadow reaches past the card.
        val shape = RoundedCornerShape(14.dp)
        Column(
            Modifier
                .fillMaxWidth()
                .then(if (LocalTelevision.current) Modifier else Modifier.clip(shape))
                .background(theme.colors.surface, shape),
            content = content,
        )
        if (footer != null) {
            Text(footer, Modifier.padding(horizontal = 4.dp), style = theme.type.footnote.text, color = theme.colors.textSecondary)
        }
    }
}

/** The line between rows, after the tiles when tiled. */
@Composable
fun RowDivider(tiled: Boolean = false) {
    val start = if (tiled) rowPadding + 44.dp else rowPadding
    HorizontalDivider(Modifier.padding(start = start), color = theme.colors.border)
}

/** A row opening a section: its icon on a colored tile, and its title. */
@Composable
fun EntryRow(title: String, @DrawableRes icon: Int, color: Color, modifier: Modifier = Modifier, onClick: () -> Unit) {
    Pressable(onClick, modifier.fillMaxWidth()) {
        Row(
            Modifier.fillMaxWidth().heightIn(min = rowHeight(52.dp)).padding(horizontal = rowPadding),
            horizontalArrangement = Arrangement.spacedBy(MavioSpacing.s16),
            verticalAlignment = Alignment.CenterVertically,
        ) {
            Box(Modifier.size(28.dp).background(color, RoundedCornerShape(7.dp)), Alignment.Center) {
                MavioIcon(icon, size = 17.dp, tint = Color.White)
            }
            Text(title, Modifier.weight(1f), style = theme.type.body.text)
            Chevron()
        }
    }
}

/** The arrow of a row opening a page. */
@Composable
fun Chevron() {
    MavioIcon(R.drawable.ic_forward, size = 18.dp, tint = theme.colors.textTertiary)
}

/** A label and its value. */
@Composable
fun ValueRow(label: String, value: String) {
    Row(
        Modifier.fillMaxWidth().heightIn(min = rowHeight(48.dp)).padding(horizontal = rowPadding),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        Text(label, Modifier.weight(1f), style = theme.type.body.text)
        Text(value, style = theme.type.body.text, color = theme.colors.textSecondary)
    }
}

/** A person's initial and name. */
@Composable
fun NameRow(name: String) {
    Row(
        Modifier.fillMaxWidth().padding(horizontal = rowPadding, vertical = 12.dp),
        horizontalArrangement = Arrangement.spacedBy(MavioSpacing.s12),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        Avatar(name, size = 44.dp)
        Text(name, style = theme.type.headline.text)
    }
}

/** An action in the accent's color, with its icon. */
@Composable
fun LinkRow(label: String, @DrawableRes icon: Int, modifier: Modifier = Modifier, onClick: () -> Unit) {
    Pressable(onClick, modifier.fillMaxWidth()) {
        Row(
            Modifier.fillMaxWidth().heightIn(min = rowHeight(48.dp)).padding(horizontal = rowPadding),
            horizontalArrangement = Arrangement.spacedBy(MavioSpacing.s12),
            verticalAlignment = Alignment.CenterVertically,
        ) {
            MavioIcon(icon, size = 18.dp, tint = theme.colors.accentText)
            Text(label, style = theme.type.body.text, color = theme.colors.accentText)
        }
    }
}

/** A row's label, with what it means below it. */
@Composable
fun RowLabel(label: String, detail: String? = null, modifier: Modifier = Modifier) {
    Column(modifier, verticalArrangement = Arrangement.spacedBy(2.dp)) {
        Text(label, style = theme.type.body.text)
        if (detail != null) Text(detail, style = theme.type.footnote.text, color = theme.colors.textSecondary)
    }
}

/** A setting turned on and off; on a television selecting the row turns it. */
@Composable
fun SwitchRow(label: String, checked: Boolean, detail: String? = null, onChange: (Boolean) -> Unit) {
    val tv = LocalTelevision.current
    Pressable({ onChange(!checked) }, Modifier.fillMaxWidth()) {
        Row(
            Modifier.fillMaxWidth().heightIn(min = rowHeight(52.dp)).padding(horizontal = rowPadding, vertical = 8.dp),
            horizontalArrangement = Arrangement.spacedBy(MavioSpacing.s12),
            verticalAlignment = Alignment.CenterVertically,
        ) {
            RowLabel(label, detail, Modifier.weight(1f))
            MavioSwitch(checked, if (tv) null else onChange)
        }
    }
}

/** A switch in the design system's colors; only showing its state without [onChange]. */
@Composable
fun MavioSwitch(checked: Boolean, onChange: ((Boolean) -> Unit)?) {
    Switch(
        checked,
        onChange,
        colors = SwitchDefaults.colors(
            checkedTrackColor = theme.colors.success,
            checkedThumbColor = Color.White,
            uncheckedTrackColor = theme.colors.fill,
            uncheckedThumbColor = Color.White,
            uncheckedBorderColor = Color.Transparent,
        ),
    )
}

/**
 * A setting chosen from a menu, showing its choice: on a television the
 * menu of choices opens below it with focus on the chosen one. Without a
 * [label], the choice takes its place.
 */
@Composable
fun <T> ChoiceRow(label: String, choices: List<Pair<T, String>>, chosen: T, detail: String? = null, onChoose: (T) -> Unit) {
    var open by remember { mutableStateOf(false) }
    val tv = LocalTelevision.current
    // The menu keeps the page's colors, whatever the focused row's.
    val page = theme
    val ink = LocalContentColor.current
    val row = remember { FocusRequester() }
    var height by remember { mutableIntStateOf(0) }
    fun close() {
        open = false
        if (tv) runCatching { row.requestFocus() }
    }
    Box(Modifier.fillMaxWidth().onSizeChanged { height = it.height }) {
        Pressable({ open = true }, Modifier.fillMaxWidth().focusRequester(row)) {
            Row(
                Modifier.fillMaxWidth().heightIn(min = rowHeight(52.dp)).padding(horizontal = rowPadding, vertical = 8.dp),
                horizontalArrangement = Arrangement.spacedBy(MavioSpacing.s8),
                verticalAlignment = Alignment.CenterVertically,
            ) {
                // Without a label, the choice in its place, as a group's title names it.
                if (label.isNotEmpty()) RowLabel(label, detail, Modifier.weight(1f))
                // The phone's menu opens at the choice.
                Box(if (label.isEmpty()) Modifier.weight(1f) else Modifier) {
                    Row(horizontalArrangement = Arrangement.spacedBy(MavioSpacing.s8), verticalAlignment = Alignment.CenterVertically) {
                        Text(
                            choices.firstOrNull { it.first == chosen }?.second.orEmpty(),
                            if (label.isEmpty()) Modifier.weight(1f) else Modifier.widthIn(max = if (tv) 360.dp else if (detail == null) 200.dp else 150.dp),
                            style = theme.type.body.text,
                            color = if (label.isEmpty()) LocalContentColor.current else theme.colors.textSecondary,
                            maxLines = 1,
                            overflow = TextOverflow.Ellipsis,
                        )
                        MavioIcon(R.drawable.ic_selector, size = 16.dp, tint = theme.colors.textSecondary)
                    }
                    if (!tv) {
                        DropdownMenu(open, onDismissRequest = { open = false }) {
                            choices.forEach { (value, text) ->
                                ChoiceItem(text, value == chosen) {
                                    open = false
                                    onChoose(value)
                                }
                            }
                        }
                    }
                }
            }
        }
        // The television's opens below the row, at its right.
        if (open && tv) {
            CompositionLocalProvider(LocalTheme provides page, LocalContentColor provides ink) {
                TvChoiceMenu(
                    choices.map { it.first },
                    chosen,
                    below = with(LocalDensity.current) { height.toDp() },
                    width = 400.dp,
                    label = { value -> choices.first { it.first == value }.second },
                    onDismiss = ::close,
                    alignment = Alignment.TopEnd,
                ) {
                    close()
                    onChoose(it)
                }
            }
        }
    }
}

/**
 * A text field of the settings. On a television it is a box that takes
 * focus, ringed in white with it, and opens the keyboard named by [label]
 * when selected (docs/tv.md §4).
 */
@Composable
fun SettingsField(
    value: String,
    onChange: (String) -> Unit,
    label: String?,
    modifier: Modifier = Modifier.fillMaxWidth(),
    placeholder: String? = null,
    keyboard: KeyboardType = KeyboardType.Text,
    error: Boolean = false,
    secret: Boolean = false,
) {
    if (LocalTelevision.current) {
        TvField(value, onChange, label, modifier, placeholder, keyboard, error, secret)
        return
    }
    // As the web UI's settings fields: as tall as its buttons, on the card's
    // color with a border, which turns the focus color while typing.
    val interactions = remember { MutableInteractionSource() }
    val focused by interactions.collectIsFocusedAsState()
    val shape = RoundedCornerShape(MavioRadius.md)
    val edge = when {
        error -> theme.colors.danger
        focused -> theme.colors.focus
        else -> theme.colors.border
    }
    Column(modifier, verticalArrangement = Arrangement.spacedBy(MavioSpacing.s4)) {
        if (label != null) Text(label, style = theme.type.footnote.text, color = theme.colors.textSecondary)
        BasicTextField(
            value,
            onChange,
            Modifier.fillMaxWidth(),
            singleLine = true,
            textStyle = theme.type.body.text.copy(color = theme.colors.textPrimary),
            cursorBrush = SolidColor(theme.colors.textPrimary),
            visualTransformation = if (secret) PasswordVisualTransformation() else VisualTransformation.None,
            keyboardOptions = KeyboardOptions(keyboardType = keyboard, autoCorrectEnabled = false),
            interactionSource = interactions,
        ) { inner ->
            Box(
                Modifier
                    .fillMaxWidth()
                    .heightIn(min = 36.dp)
                    .background(theme.colors.surface, shape)
                    .border(1.dp, edge, shape)
                    .padding(horizontal = MavioSpacing.s12),
                Alignment.CenterStart,
            ) {
                if (value.isEmpty() && placeholder != null) {
                    Text(placeholder, style = theme.type.body.text, color = theme.colors.textTertiary, maxLines = 1, overflow = TextOverflow.Ellipsis)
                }
                inner()
            }
        }
    }
}

/** [SettingsField] on a television. */
@Composable
private fun TvField(
    value: String,
    onChange: (String) -> Unit,
    label: String?,
    modifier: Modifier,
    placeholder: String?,
    keyboard: KeyboardType,
    error: Boolean,
    secret: Boolean,
) {
    var typing by remember { mutableStateOf(false) }
    val state = remember { Focusable() }
    val focused = state.focused()
    val shape = RoundedCornerShape(10.dp)
    Column(modifier, verticalArrangement = Arrangement.spacedBy(MavioSpacing.s8)) {
        if (label != null) Text(label, style = theme.type.callout.text, color = theme.colors.textSecondary)
        Box(
            Modifier
                .shakes()
                .fillMaxWidth()
                .heightIn(min = 64.dp)
                .clip(shape)
                .background(theme.colors.fill)
                .border(
                    if (focused) 3.dp else 1.dp,
                    when {
                        focused -> Color.White
                        error -> theme.colors.danger
                        else -> theme.colors.border
                    },
                    shape,
                )
                .recallsFocus()
                .selectable(state) { typing = true }
                .padding(horizontal = 20.dp),
            Alignment.CenterStart,
        ) {
            val shown = if (secret) "•".repeat(value.length) else value
            Text(
                shown.ifEmpty { placeholder.orEmpty() },
                style = theme.type.body.text,
                color = if (shown.isEmpty()) theme.colors.textTertiary else theme.colors.textPrimary,
                maxLines = 1,
                overflow = TextOverflow.Ellipsis,
            )
        }
    }
    if (typing) TvKeyboard(label ?: placeholder.orEmpty(), value, fieldKind(keyboard, secret), onChange) { typing = false }
}

/** The television keyboard's keys for a field of [keyboard], or a secret. */
fun fieldKind(keyboard: KeyboardType, secret: Boolean = false): FieldKind = when {
    secret -> FieldKind.Secret
    keyboard == KeyboardType.Number -> FieldKind.Integer
    keyboard == KeyboardType.Decimal -> FieldKind.Decimal
    else -> FieldKind.Text
}

/**
 * A button of the settings: the design system's, [kind] choosing its fill,
 * or on a television one in `color.fill` that turns white with focus.
 */
@Composable
fun SettingsButton(
    label: String,
    onClick: () -> Unit,
    modifier: Modifier = Modifier,
    kind: ButtonKind = ButtonKind.Primary,
    @DrawableRes icon: Int? = null,
    large: Boolean = false,
    enabled: Boolean = true,
) {
    if (!LocalTelevision.current) {
        MavioButton(label, onClick, modifier, kind, icon, large, enabled)
        return
    }
    // Every kind is in `color.fill` until focus turns it white, as a
    // filled one would look focused; disabled, it is dimmed.
    TvButton(
        label,
        { if (enabled) onClick() },
        modifier.alpha(if (enabled) 1f else 0.5f),
        icon,
        fill = theme.colors.fill,
        content = theme.colors.textPrimary,
    )
}
