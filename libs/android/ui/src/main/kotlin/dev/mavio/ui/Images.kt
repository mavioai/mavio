package dev.mavio.ui

import android.content.Context
import androidx.compose.runtime.Composable
import androidx.compose.runtime.remember
import coil3.ImageLoader
import coil3.SingletonImageLoader
import coil3.compose.LocalPlatformContext
import coil3.disk.DiskCache
import coil3.request.ImageRequest
import coil3.request.crossfade
import dev.mavio.design.MavioDuration
import okio.Path.Companion.toOkioPath

/**
 * Makes the apps' image loader keep the images it loads on disk across
 * launches, so that a screen drawn again shows them at once instead of
 * their placeholders, and fade each in as it arrives. Called as the app's
 * process starts.
 */
fun installImages(context: Context) {
    SingletonImageLoader.setSafe { imageLoader(context) }
}

private fun imageLoader(context: Context): ImageLoader = ImageLoader.Builder(context)
    .diskCache {
        DiskCache.Builder()
            .directory(context.cacheDir.resolve("images").toOkioPath())
            .maxSizeBytes(500L shl 20)
            .build()
    }
    .crossfade(MavioDuration.normal)
    .build()

/**
 * The request of an image from a server, kept on disk under its URL
 * without the server's signature, which changes with the session and every
 * period while the image it names stays the same.
 */
fun imageRequest(context: Context, url: String): ImageRequest =
    ImageRequest.Builder(context).data(url).diskCacheKey(unsignedKey(url)).build()

/** [imageRequest], remembered for [url]. */
@Composable
internal fun rememberImageRequest(url: String): ImageRequest {
    val context = LocalPlatformContext.current
    return remember(url, context) { imageRequest(context, url) }
}

/** An image's URL with its signed token cut out: `…/signed/<token>/images/<id>` is kept as `…/images/<id>`. */
internal fun unsignedKey(url: String): String {
    val start = url.indexOf("/signed/")
    if (start < 0) return url
    val end = url.indexOf('/', start + "/signed/".length)
    if (end < 0) return url
    return url.removeRange(start, end)
}
