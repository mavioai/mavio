package dev.mavio.app.tv

import android.util.Log
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.widthIn
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableIntStateOf
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.shadow
import androidx.compose.ui.focus.FocusRequester
import androidx.compose.ui.focus.focusRequester
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.luminance
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import dev.mavio.core.AppModel
import dev.mavio.core.LinkCode
import dev.mavio.design.MavioSpacing
import dev.mavio.ui.PosterWall
import dev.mavio.ui.R
import dev.mavio.ui.Wordmark
import dev.mavio.ui.theme
import java.time.Duration
import java.time.Instant
import kotlin.coroutines.cancellation.CancellationException
import kotlinx.coroutines.delay

/** The client ID of the Android apps at mavio.dev. */
private const val CLIENT_ID = "dev.mavio.app"

/**
 * Signing in on a television (docs/tv.md §6.5): a Mavio account, with a
 * code to enter at mavio.dev/link on another device, polled until it is
 * approved and replaced as it expires. Over the wall of posters, in a card
 * of glass, as the web UI's television layout.
 */
@Composable
fun SignInScreen(model: AppModel) {
    var link by remember { mutableStateOf<LinkCode?>(null) }
    var error by remember { mutableStateOf<String?>(null) }
    var attempt by remember { mutableIntStateOf(0) }
    LaunchedEffect(attempt) {
        error = null
        try {
            while (true) {
                val code = model.client.startLink(BuildConfig.MAVIO_DEV_API, CLIENT_ID)
                link = code
                val every = maxOf(code.interval(), Duration.ofSeconds(1)).toMillis()
                while (Instant.now() < code.expires()) {
                    delay(every)
                    // Signed in, the app leaves for Home.
                    if (model.pollLink(code) != null) return@LaunchedEffect
                }
            }
        } catch (e: CancellationException) {
            throw e
        } catch (e: Exception) {
            Log.w("Mavio", "link code", e)
            link = null
            error = e.message ?: e.toString()
        }
    }
    val dark = theme.colors.background.luminance() < 0.5f
    Box(Modifier.fillMaxSize().background(theme.colors.background)) {
        PosterWall(tv = true)
        Wordmark(Modifier.padding(start = 120.dp, top = 90.dp), height = 60.dp)
        Box(Modifier.fillMaxSize().padding(horizontal = 120.dp, vertical = 90.dp), Alignment.Center) {
            val card = RoundedCornerShape(44.dp)
            Column(
                Modifier
                    .widthIn(max = 1040.dp)
                    .fillMaxWidth()
                    .shadow(80.dp, card, ambientColor = Color.Black.copy(alpha = 0.5f), spotColor = Color.Black.copy(alpha = 0.5f))
                    .background(if (dark) Color(0xD11C1C1E) else Color(0xD6FFFFFF), card)
                    .border(1.dp, if (dark) Color(0x80545458) else Color(0x243C3C43), card)
                    .padding(horizontal = 90.dp, vertical = 80.dp),
                verticalArrangement = Arrangement.spacedBy(MavioSpacing.s32),
            ) {
                Text(stringResource(R.string.auth_sign_in_with_mavio), fontSize = 54.sp, lineHeight = 62.sp, fontWeight = FontWeight.Bold)
                link?.let {
                    val where = it.verificationUri().removePrefix("https://").removePrefix("http://")
                    Text(stringResource(R.string.auth_link_message, where), fontSize = 26.sp, lineHeight = 36.sp, color = theme.colors.textSecondary)
                    Code(it.userCode())
                }
                val failed = error
                if (failed != null) {
                    Text(failed, fontSize = 26.sp, color = theme.colors.danger)
                    val retry = remember { FocusRequester() }
                    TvButton(
                        stringResource(R.string.auth_try_again),
                        { attempt++ },
                        Modifier.focusRequester(retry),
                        fill = theme.colors.fill,
                        content = theme.colors.textPrimary,
                        height = 84.dp,
                        shape = RoundedCornerShape(22.dp),
                    )
                    LaunchedEffect(Unit) { retry.requestFocus() }
                } else {
                    Waiting()
                }
            }
        }
    }
}

/** A code to enter on another device, a character to a box. */
@Composable
private fun Code(code: String) {
    Row(Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.spacedBy(10.dp, Alignment.CenterHorizontally)) {
        code.forEach { c ->
            if (c == '-') {
                Text("–", Modifier.align(Alignment.CenterVertically), fontSize = 56.sp, color = theme.colors.textTertiary)
            } else {
                Box(Modifier.size(width = 88.dp, height = 110.dp).background(theme.colors.fill, RoundedCornerShape(22.dp)), Alignment.Center) {
                    Text(c.toString(), fontFamily = FontFamily.Monospace, fontSize = 56.sp, fontWeight = FontWeight.SemiBold)
                }
            }
        }
    }
}

/** Waiting for the code's approval. */
@Composable
private fun Waiting() {
    Row(Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.spacedBy(MavioSpacing.s12, Alignment.CenterHorizontally), verticalAlignment = Alignment.CenterVertically) {
        CircularProgressIndicator(Modifier.size(24.dp), color = theme.colors.textTertiary, strokeWidth = 3.dp)
        Text(stringResource(R.string.auth_waiting), fontSize = 26.sp, color = theme.colors.textSecondary)
    }
}
