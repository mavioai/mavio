package dev.mavio.ui

import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.tooling.preview.Preview
import dev.mavio.design.DarkColors

/** The Mavio wordmark in the accent color. */
@Composable
fun Wordmark(modifier: Modifier = Modifier) {
    Text(
        text = "Mavio",
        modifier = modifier,
        color = DarkColors.accent,
        style = MaterialTheme.typography.displaySmall,
        fontWeight = FontWeight.Bold,
    )
}

@Preview
@Composable
private fun WordmarkPreview() {
    Wordmark()
}
