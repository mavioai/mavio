package dev.mavio.ui

import android.provider.Settings
import androidx.activity.compose.BackHandler
import androidx.compose.animation.core.MutableTransitionState
import androidx.compose.animation.core.animateFloatAsState
import androidx.compose.animation.core.tween
import androidx.compose.animation.AnimatedVisibility
import androidx.compose.animation.fadeIn
import androidx.compose.animation.fadeOut
import androidx.compose.animation.slideInVertically
import androidx.compose.foundation.Canvas
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.clickable
import androidx.compose.foundation.focusable
import androidx.compose.foundation.gestures.awaitEachGesture
import androidx.compose.foundation.gestures.awaitFirstDown
import androidx.compose.foundation.gestures.waitForUpOrCancellation
import androidx.compose.foundation.interaction.MutableInteractionSource
import androidx.compose.foundation.interaction.collectIsFocusedAsState
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.BoxWithConstraints
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.navigationBarsPadding
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.requiredSize
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.statusBarsPadding
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.DisposableEffect
import androidx.compose.runtime.staticCompositionLocalOf
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.key
import androidx.compose.runtime.mutableDoubleStateOf
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.runtime.withFrameNanos
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.draw.scale
import androidx.compose.ui.draw.shadow
import androidx.compose.ui.focus.FocusRequester
import androidx.compose.ui.focus.focusRequester
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.CompositingStrategy
import androidx.compose.ui.graphics.graphicsLayer
import androidx.compose.ui.graphics.luminance
import androidx.compose.ui.input.key.onKeyEvent
import androidx.compose.ui.input.pointer.PointerEventPass
import androidx.compose.ui.input.pointer.pointerInput
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.semantics.Role
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.onClick
import androidx.compose.ui.semantics.role
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.SpanStyle
import androidx.compose.ui.text.buildAnnotatedString
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.text.withStyle
import androidx.compose.ui.unit.Dp
import androidx.compose.ui.unit.dp
import dev.mavio.design.MavioDuration
import dev.mavio.design.MavioElevation
import dev.mavio.design.MavioRadius
import dev.mavio.design.MavioSpacing

/** The app's assistant; null where there is none, as in previews. */
val LocalAssistant = staticCompositionLocalOf<Assistant?> { null }

/** Makes [player] the player the assistant controls while it is in the composition. */
@Composable
fun AssistantPlayerEffect(player: AssistantPlayer?) {
    val assistant = LocalAssistant.current ?: return
    DisposableEffect(assistant, player) {
        if (player != null) {
            assistant.player = player
            assistant.contextChanged()
        }
        onDispose {
            if (player != null && assistant.player === player) {
                assistant.player = null
                assistant.contextChanged()
            }
        }
    }
}

/**
 * The assistant over the app (docs/assistant.md §9.1, §9.4;
 * libs/design/components.md, Assistant), as the web UI's: on a phone the
 * orb at the bottom's center, above the tab bar where it shows ([lift]),
 * and the cards above it, the first nearest; on a tablet and a television
 * the orb at the top right, its status pill beside it, and the cards below
 * it. A touch outside them, or Back, dismisses them.
 */
@Composable
fun AssistantOverlay(assistant: Assistant, lift: Dp = 0.dp) {
    val tv = LocalTelevision.current
    // Opened, it takes Back before the screens under it.
    key(assistant.open) {
        BackHandler(assistant.open && !assistant.leaving) { assistant.close() }
    }
    if (!assistant.open) return
    val alpha by animateFloatAsState(if (assistant.leaving) 0f else 1f, if (assistant.leaving) tween(MavioDuration.slow) else tween(0), label = "leave")
    BoxWithConstraints(Modifier.fillMaxSize().graphicsLayer { this.alpha = alpha }) {
        val wide = tv || maxWidth >= 600.dp
        if (!tv) {
            // A touch only dismisses: it does not reach the page.
            Box(
                Modifier.fillMaxSize().pointerInput(assistant) {
                    awaitEachGesture {
                        awaitFirstDown().consume()
                        if (!assistant.leaving) assistant.close()
                    }
                },
            )
        }
        if (wide) {
            Row(
                Modifier.align(Alignment.TopEnd).statusBarsPadding().padding(top = 12.dp, end = 12.dp),
                horizontalArrangement = Arrangement.spacedBy(MavioSpacing.s8),
                verticalAlignment = Alignment.CenterVertically,
            ) {
                assistant.status?.let { Pill(it) }
                AssistantOrb(assistant)
            }
            // Just under the orb's light, which fills only the middle of its box.
            Cards(
                assistant,
                wide = true,
                Modifier
                    .align(Alignment.TopEnd)
                    .statusBarsPadding()
                    .padding(top = 80.dp, end = MavioSpacing.s16 - MavioSpacing.s4, bottom = MavioSpacing.s16)
                    .width(CARD_WIDTH),
            )
        } else {
            AssistantOrb(assistant, Modifier.align(Alignment.BottomCenter).navigationBarsPadding().padding(bottom = lift + MavioSpacing.s16))
            // No status pill on a phone; the cards start just over the orb's light.
            Cards(
                assistant,
                wide = false,
                Modifier
                    .align(Alignment.BottomCenter)
                    .statusBarsPadding()
                    .navigationBarsPadding()
                    .padding(start = MavioSpacing.s4, end = MavioSpacing.s4, top = 56.dp, bottom = lift + 80.dp),
            )
        }
    }
}

/** The cards' width on a tablet and a television. */
private val CARD_WIDTH = 380.dp

/** What the pill beside the orb says. */
@Composable
private fun Pill(text: String) {
    Text(
        text,
        Modifier.cardFill(CircleShape).padding(horizontal = MavioSpacing.s12, vertical = MavioSpacing.s4),
        style = theme.type.footnote.text,
        color = theme.colors.textPrimary,
        maxLines = 1,
    )
}

/** The cards, scrolling where they are taller than the room for them: on a phone the first at the bottom. */
@Composable
private fun Cards(assistant: Assistant, wide: Boolean, modifier: Modifier) {
    val cards = buildList<Pair<String, @Composable () -> Unit>> {
        if (assistant.heard.isNotEmpty() || assistant.answer.isNotEmpty()) add("talk" to { Card { Talk(assistant) } })
        if (assistant.activity.isNotEmpty()) {
            add("tools" to { Card(vertical = 12.dp, horizontal = 14.dp) { Column(verticalArrangement = Arrangement.spacedBy(MavioSpacing.s8)) { assistant.activity.forEach { ToolRow(it) } } } })
        }
        if (assistant.items.isNotEmpty()) {
            add(
                "items" to {
                    Card(vertical = 10.dp, horizontal = 10.dp) {
                        Column(verticalArrangement = Arrangement.spacedBy(2.dp)) {
                            assistant.items.forEachIndexed { i, item -> ResultRow(assistant, item, if (assistant.choose) i + 1 else null, first = i == 0) }
                        }
                    }
                },
            )
        }
        assistant.confirmation?.let { c -> add("confirm" + c.id to { Card { ConfirmView(assistant, c) } }) }
        assistant.error?.let { e -> add("error" to { Card { Text(e, color = theme.colors.danger) } }) }
    }
    Column(
        modifier
            .verticalScroll(rememberScrollState(), reverseScrolling = !wide)
            // A touched card keeps the assistant.
            .pointerInput(assistant) {
                awaitEachGesture {
                    awaitFirstDown(requireUnconsumed = false, pass = PointerEventPass.Initial)
                    assistant.hold()
                    waitForUpOrCancellation(PointerEventPass.Initial)
                    assistant.letGo()
                }
            }
            .padding(MavioSpacing.s4),
        verticalArrangement = Arrangement.spacedBy(MavioSpacing.s8),
    ) {
        (if (wide) cards else cards.reversed()).forEach { (id, card) ->
            key(id) {
                val shown = remember { MutableTransitionState(false).apply { targetState = true } }
                AnimatedVisibility(shown, enter = fadeIn(tween(MavioDuration.slow)) + slideInVertically(tween(MavioDuration.slow)) { if (wide) -it / 6 else it / 6 }, exit = fadeOut()) {
                    card()
                }
            }
        }
    }
}

/** A card of the assistant: the cards' material, as the web UI's. */
@Composable
private fun Card(vertical: Dp = 14.dp, horizontal: Dp = 16.dp, content: @Composable () -> Unit) {
    val shape = RoundedCornerShape(MavioRadius.xl)
    Box(
        Modifier
            .fillMaxWidth()
            .shadow(MavioElevation.level3.blur / 2, shape, clip = false, ambientColor = MavioElevation.level3.color, spotColor = MavioElevation.level3.color)
            .cardFill(shape)
            .padding(vertical = vertical, horizontal = horizontal),
    ) {
        androidx.compose.runtime.CompositionLocalProvider(
            androidx.compose.material3.LocalTextStyle provides theme.type.body.text,
            androidx.compose.material3.LocalContentColor provides theme.colors.textPrimary,
        ) { content() }
    }
}

/** The fill of the cards and the pill: the card's color, and a hairline border. */
@Composable
private fun Modifier.cardFill(shape: androidx.compose.ui.graphics.Shape): Modifier {
    val colors = theme.colors
    val light = colors.background.luminance() > 0.5f
    return clip(shape)
        .background(if (light) Color.White.copy(alpha = 0.92f) else colors.surfaceOverlay.copy(alpha = 0.92f))
        .border(1.dp, colors.border, shape)
}

/** What the user said, and the answer, its words lit as they are spoken. */
@Composable
private fun Talk(assistant: Assistant) {
    val colors = theme.colors
    Column(verticalArrangement = Arrangement.spacedBy(10.dp)) {
        if (assistant.heard.isNotEmpty()) {
            Text(
                buildAnnotatedString {
                    withStyle(SpanStyle(color = colors.textSecondary)) { append(assistant.heard) }
                    if (!assistant.userSpeaking && assistant.heardFinal) withStyle(SpanStyle(color = colors.textTertiary)) { append(" ›") }
                },
            )
        }
        if (assistant.answer.isNotEmpty()) {
            val live = assistant.answerLive
            // How much of the speech has played, which goes only forward for the same answer.
            val clock = remember { SpokenClock() }
            var frame by remember { mutableDoubleStateOf(0.0) }
            LaunchedEffect(live) {
                while (live) withFrameNanos { frame = it / 1e9 }
            }
            frame
            val played = clock.advance(if (live) assistant.played() else Double.POSITIVE_INFINITY, assistant.answer)
            Text(
                buildAnnotatedString {
                    SpokenText.segments(assistant.answer).forEach { (text, at) ->
                        withStyle(SpanStyle(color = if (!live || at <= played + 120) colors.textPrimary else colors.textTertiary)) { append(text) }
                    }
                },
            )
        }
    }
}

/** How much of the answer's speech has played, which goes only forward for the same answer. */
private class SpokenClock {
    private var played = 0.0
    private var answer = ""

    fun advance(to: Double, answer: String): Double {
        // A new answer starts again.
        if (!answer.startsWith(this.answer) || this.answer.isEmpty()) played = 0.0
        this.answer = answer
        played = maxOf(played, to)
        return played
    }
}

/** A tool run or running. */
@Composable
private fun ToolRow(activity: Assistant.Activity) {
    val colors = theme.colors
    Row(horizontalArrangement = Arrangement.spacedBy(10.dp), verticalAlignment = Alignment.CenterVertically) {
        Box(Modifier.size(26.dp).background(colors.accent.copy(alpha = 0.16f), RoundedCornerShape(MavioRadius.sm)), Alignment.Center) {
            MavioIcon(R.drawable.ic_search, size = 14.dp, tint = colors.accent)
        }
        Text(activity.summary, Modifier.weight(1f), style = theme.type.footnote.text, color = colors.textSecondary)
        Box(Modifier.size(16.dp), Alignment.Center) {
            when {
                activity.failed -> MavioIcon(R.drawable.ic_close, size = 16.dp, tint = colors.danger)
                activity.done -> MavioIcon(R.drawable.ic_check, size = 16.dp, tint = colors.success)
                else -> CircularProgressIndicator(Modifier.size(12.dp), color = colors.textSecondary, strokeWidth = 1.5.dp)
            }
        }
    }
}

/** A result, chosen or opened when tapped; on a television the first takes focus. */
@Composable
private fun ResultRow(assistant: Assistant, item: Assistant.Result, number: Int?, first: Boolean) {
    val colors = theme.colors
    val interactions = remember { MutableInteractionSource() }
    val focused by interactions.collectIsFocusedAsState()
    val focus = remember { FocusRequester() }
    if (first && LocalTelevision.current) LaunchedEffect(item.id) { runCatching { focus.requestFocus() } }
    Row(
        Modifier
            .fillMaxWidth()
            .clip(RoundedCornerShape(MavioRadius.md))
            .background(if (focused) colors.fillSelected else Color.Transparent)
            .focusRequester(focus)
            .clickable(interactions, null) { assistant.choose(item) }
            .padding(6.dp),
        horizontalArrangement = Arrangement.spacedBy(10.dp),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        number?.let { Text("$it", Modifier.width(16.dp), style = theme.type.footnote.text, fontWeight = FontWeight.SemiBold, color = colors.textTertiary) }
        Artwork(item.poster, Modifier.size(34.dp, 51.dp).clip(RoundedCornerShape(5.dp)), icon = itemIcon(item.kind), contentDescription = item.name)
        Column(Modifier.weight(1f), verticalArrangement = Arrangement.spacedBy(1.dp)) {
            Text(item.name, style = theme.type.callout.text, fontWeight = FontWeight.Medium, color = colors.textPrimary, maxLines = 1, overflow = TextOverflow.Ellipsis)
            item.subtitle?.let { Text(it, style = theme.type.caption.text, color = colors.textSecondary, maxLines = 1, overflow = TextOverflow.Ellipsis) }
        }
    }
}

/** The icon of a result's kind, while its poster loads or where it has none. */
private fun itemIcon(kind: mavio.library.v1.ItemOuterClass.ItemKind): Int = when (kind) {
    mavio.library.v1.ItemOuterClass.ItemKind.ITEM_KIND_SERIES,
    mavio.library.v1.ItemOuterClass.ItemKind.ITEM_KIND_SEASON,
    mavio.library.v1.ItemOuterClass.ItemKind.ITEM_KIND_EPISODE,
    -> R.drawable.ic_shows
    mavio.library.v1.ItemOuterClass.ItemKind.ITEM_KIND_MUSIC_ALBUM,
    mavio.library.v1.ItemOuterClass.ItemKind.ITEM_KIND_TRACK,
    mavio.library.v1.ItemOuterClass.ItemKind.ITEM_KIND_MUSIC_ARTIST,
    -> R.drawable.ic_music
    else -> R.drawable.ic_movies
}

/** The confirmation and its buttons, as the web UI's: Cancel on the fill, Confirm on the accent; on a television Cancel takes focus. */
@Composable
private fun ConfirmView(assistant: Assistant, c: Assistant.Confirmation) {
    val colors = theme.colors
    val cancel = remember { FocusRequester() }
    if (LocalTelevision.current) LaunchedEffect(c.id) { runCatching { cancel.requestFocus() } }
    Column(verticalArrangement = Arrangement.spacedBy(MavioSpacing.s12)) {
        Text(c.text)
        Row(horizontalArrangement = Arrangement.spacedBy(MavioSpacing.s8)) {
            ConfirmButton(stringResource(R.string.action_cancel), primary = false, Modifier.weight(1f).focusRequester(cancel)) { assistant.confirm(false) }
            ConfirmButton(stringResource(R.string.assistant_confirm), primary = true, Modifier.weight(1f)) { assistant.confirm(true) }
        }
    }
}

@Composable
private fun ConfirmButton(label: String, primary: Boolean, modifier: Modifier, onClick: () -> Unit) {
    val colors = theme.colors
    val interactions = remember { MutableInteractionSource() }
    val focused by interactions.collectIsFocusedAsState()
    val shape = RoundedCornerShape(MavioRadius.md)
    Box(
        modifier
            .heightIn(min = 36.dp)
            .clip(shape)
            .background(
                when {
                    focused -> Color.White
                    primary -> colors.accent
                    else -> colors.fill
                },
            )
            .clickable(interactions, null, onClick = onClick),
        Alignment.Center,
    ) {
        Text(
            label,
            style = theme.type.callout.text,
            color = when {
                focused -> Color.Black
                primary -> colors.onAccent
                else -> colors.textPrimary
            },
        )
    }
}

/** The orb, which pressed starts a voice session and pressed again ends it, or held talks while held. */
@Composable
private fun AssistantOrb(assistant: Assistant, modifier: Modifier = Modifier) {
    val renderer = remember { OrbRenderer() }
    val context = LocalContext.current
    val reduceMotion = remember { Settings.Global.getFloat(context.contentResolver, Settings.Global.ANIMATOR_DURATION_SCALE, 1f) == 0f }
    val light = theme.colors.background.luminance() > 0.5f
    var now by remember { mutableDoubleStateOf(0.0) }
    LaunchedEffect(Unit) {
        while (true) withFrameNanos { now = it / 1e9 }
    }
    val scale by animateFloatAsState(if (assistant.leaving) 0.4f else 1f, if (assistant.leaving) tween(MavioDuration.slow) else tween(0), label = "orb")
    val talk = stringResource(if (assistant.listening) R.string.assistant_stop_listening else R.string.assistant_talk)
    Box(
        modifier
            .size(OrbRenderer.SIZE)
            .scale(scale)
            .semantics {
                role = Role.Button
                contentDescription = talk
                onClick {
                    assistant.tap()
                    true
                }
            }
            .pressAndHold(assistant),
        Alignment.Center,
    ) {
        Canvas(
            Modifier
                .requiredSize(OrbRenderer.SIZE * OrbRenderer.BLEED)
                .graphicsLayer {
                    // Drawn apart from the page, so that its lights add up only with each other.
                    compositingStrategy = CompositingStrategy.Offscreen
                    translationX = renderer.offset * density
                },
        ) {
            renderer.step(now, assistant.mode, assistant.level(), assistant.interrupts, reduceMotion)
            renderer.draw(this, now, light, reduceMotion)
        }
    }
}

/** Presses the assistant as a finger goes down, and releases it as it goes up. */
private fun Modifier.pressAndHold(assistant: Assistant): Modifier = pointerInput(assistant) {
    awaitEachGesture {
        awaitFirstDown().consume()
        assistant.press()
        waitForUpOrCancellation()?.consume()
        assistant.release()
    }
}

/**
 * The assistant button, a microphone, which tapped starts a voice session
 * and tapped again ends it, or held talks while held; hidden while no
 * server's assistant is on.
 */
@Composable
fun AssistantButton(assistant: Assistant, tint: Color = theme.colors.textPrimary, size: Dp = 22.dp, modifier: Modifier = Modifier) {
    if (assistant.server == null) return
    var pressed by remember { mutableStateOf(false) }
    val talk = stringResource(if (assistant.listening) R.string.assistant_stop_listening else R.string.assistant_talk)
    Box(
        modifier
            .size(36.dp)
            .clip(CircleShape)
            .graphicsLayer { alpha = if (pressed) 0.6f else 1f }
            .semantics {
                role = Role.Button
                contentDescription = talk
                onClick {
                    assistant.tap()
                    true
                }
            }
            .pointerInput(assistant) {
                awaitEachGesture {
                    awaitFirstDown().consume()
                    pressed = true
                    assistant.press()
                    waitForUpOrCancellation()?.consume()
                    pressed = false
                    assistant.release()
                }
            },
        Alignment.Center,
    ) {
        MavioIcon(if (assistant.listening) R.drawable.ic_microphone_fill else R.drawable.ic_microphone, size = size, tint = tint)
    }
}

/**
 * The television's assistant button (docs/tv.md §2): a microphone in a
 * round button like the menu button, or on [fill] as the player's, white
 * with focus; selected it starts or ends a conversation, held it talks
 * while held.
 */
@Composable
fun TvAssistantButton(
    assistant: Assistant,
    size: Dp,
    modifier: Modifier = Modifier,
    fill: Color = Color.Transparent,
    tint: Color = theme.colors.textSecondary,
) {
    if (assistant.server == null) return
    val interactions = remember { MutableInteractionSource() }
    val focused by interactions.collectIsFocusedAsState()
    val talk = stringResource(if (assistant.listening) R.string.assistant_stop_listening else R.string.assistant_talk)
    Box(
        modifier
            .shakes()
            .size(size)
            .clip(CircleShape)
            .background(if (focused) Color.White else fill)
            .onCenterHeld(assistant)
            .focusable(interactionSource = interactions)
            .semantics { contentDescription = talk },
        Alignment.Center,
    ) {
        MavioIcon(
            if (assistant.listening) R.drawable.ic_microphone_fill else R.drawable.ic_microphone,
            size = size / 2,
            tint = if (focused) Color.Black else tint,
        )
    }
}

/** The remote's select key pressing and releasing the assistant. */
private fun Modifier.onCenterHeld(assistant: Assistant): Modifier = onKeyEvent { e ->
    val code = e.nativeKeyEvent.keyCode
    if (code != android.view.KeyEvent.KEYCODE_DPAD_CENTER && code != android.view.KeyEvent.KEYCODE_ENTER && code != android.view.KeyEvent.KEYCODE_NUMPAD_ENTER) return@onKeyEvent false
    when (e.nativeKeyEvent.action) {
        android.view.KeyEvent.ACTION_DOWN -> if (e.nativeKeyEvent.repeatCount == 0) assistant.press()
        android.view.KeyEvent.ACTION_UP -> assistant.release()
    }
    true
}
