package dev.mavio.admin

import androidx.compose.runtime.Composable
import androidx.compose.runtime.staticCompositionLocalOf
import dev.mavio.core.AppModel

/** A page of the server's administration, opened from the Server group or within it. */
sealed interface AdminPage {
    /** A section of a server's settings. */
    data class ServerSetting(val serverId: String, val section: ServerSection) : AdminPage

    /** A menu of a server's settings. */
    data class ServerMenuPage(val serverId: String, val menu: ServerMenu) : AdminPage

    /** An issue reported on a server. */
    data class IssuePage(val serverId: String, val id: String) : AdminPage

    /** A library of a server's settings, or a new one's when id is null. */
    data class ManagedLibrary(val serverId: String, val id: String?) : AdminPage

    /** A user of a server, or a new one's page when id is null; standalone when the server keeps its own accounts. */
    data class ManagedUser(val serverId: String, val id: String?, val standalone: Boolean) : AdminPage

    /** Adding an rclone remote to a server. */
    data class NewRemote(val serverId: String) : AdminPage

    /** An rclone remote of a server. */
    data class StorageRemote(val serverId: String, val name: String) : AdminPage
}

/** How an app shows the administration's pages. */
interface AdminHost {
    /** Opens a page over the one shown. */
    fun open(page: AdminPage)

    /** Leaves the page shown for the one under it. */
    fun back()

    /**
     * Controls another device's playback, closing with onDismiss, where the
     * app can; null where it cannot.
     */
    val remote: (@Composable (model: AppModel, serverId: String, sessionId: String, onDismiss: () -> Unit) -> Unit)?
}

/** The app's host of the administration's pages. */
val LocalAdmin = staticCompositionLocalOf<AdminHost> { error("no admin host") }

/** A page of the server's administration. */
@Composable
fun AdminScreen(model: AppModel, page: AdminPage) {
    when (page) {
        is AdminPage.ServerSetting -> ServerSettingScreen(model, page)
        is AdminPage.ServerMenuPage -> ServerMenuScreen(page)
        is AdminPage.IssuePage -> IssueScreen(model, page)
        is AdminPage.ManagedLibrary -> ManagedLibraryScreen(model, page)
        is AdminPage.ManagedUser -> ManagedUserScreen(model, page)
        is AdminPage.NewRemote -> NewRemoteScreen(model, page)
        is AdminPage.StorageRemote -> StorageRemoteScreen(model, page)
    }
}
