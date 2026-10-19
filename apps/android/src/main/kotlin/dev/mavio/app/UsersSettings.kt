package dev.mavio.app

import android.content.ClipData
import android.content.ClipboardManager
import android.net.Uri
import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.lazy.LazyListScope
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.text.selection.SelectionContainer
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableIntStateOf
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.platform.LocalFocusManager
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.text.input.KeyboardType
import androidx.compose.ui.unit.dp
import com.google.protobuf.Timestamp
import com.google.protobuf.duration
import dev.mavio.core.AppModel
import dev.mavio.core.call
import dev.mavio.core.instant
import dev.mavio.design.MavioSpacing
import dev.mavio.ui.Avatar
import dev.mavio.ui.ButtonKind
import dev.mavio.ui.Chevron
import dev.mavio.ui.ChoiceRow
import dev.mavio.ui.Group
import dev.mavio.ui.LinkRow
import dev.mavio.ui.MavioButton
import dev.mavio.ui.MavioIcon
import dev.mavio.ui.R
import dev.mavio.ui.RowDivider
import dev.mavio.ui.RowLabel
import dev.mavio.ui.SettingsField
import dev.mavio.ui.SettingsList
import dev.mavio.ui.StatusView
import dev.mavio.ui.SwitchRow
import dev.mavio.ui.ValueRow
import dev.mavio.ui.text
import dev.mavio.ui.theme
import java.text.Collator
import java.time.ZoneId
import java.time.format.DateTimeFormatter
import java.time.format.FormatStyle
import kotlinx.coroutines.async
import kotlinx.coroutines.coroutineScope
import kotlinx.coroutines.launch
import mavio.auth.v1.Auth.AccountMode
import mavio.auth.v1.Auth.GetAuthInfoResponse
import mavio.auth.v1.getAuthInfoRequest
import mavio.library.v1.LibraryOuterClass.Library
import mavio.library.v1.LibraryOuterClass.LibraryKind
import mavio.library.v1.LibraryOuterClass.ListLibrariesResponse
import mavio.library.v1.listLibrariesRequest
import mavio.system.v1.Localization.ListRatingsResponse
import mavio.system.v1.Localization.Rating
import mavio.system.v1.listRatingsRequest
import mavio.user.v1.UserOuterClass.CreateInvitationResponse
import mavio.user.v1.UserOuterClass.CreateUserResponse
import mavio.user.v1.UserOuterClass.DeleteInvitationResponse
import mavio.user.v1.UserOuterClass.DeletePasskeyResponse
import mavio.user.v1.UserOuterClass.DeleteTotpResponse
import mavio.user.v1.UserOuterClass.DeleteUserResponse
import mavio.user.v1.UserOuterClass.GetUserResponse
import mavio.user.v1.UserOuterClass.Invitation
import mavio.user.v1.UserOuterClass.ListInvitationsResponse
import mavio.user.v1.UserOuterClass.ListUsersResponse
import mavio.user.v1.UserOuterClass.RestrictionProfile
import mavio.user.v1.UserOuterClass.SetPasswordResponse
import mavio.user.v1.UserOuterClass.SetPinResponse
import mavio.user.v1.UserOuterClass.UpdateUserResponse
import mavio.user.v1.UserOuterClass.User
import mavio.user.v1.UserOuterClass.UserPolicy
import mavio.user.v1.createInvitationRequest
import mavio.user.v1.createUserRequest
import mavio.user.v1.deleteInvitationRequest
import mavio.user.v1.deletePasskeyRequest
import mavio.user.v1.deleteTotpRequest
import mavio.user.v1.deleteUserRequest
import mavio.user.v1.getUserRequest
import mavio.user.v1.listInvitationsRequest
import mavio.user.v1.listUsersRequest
import mavio.user.v1.setPasswordRequest
import mavio.user.v1.setPinRequest
import mavio.user.v1.updateUserRequest
import mavio.user.v1.userPolicy

// Users (docs/clients.md §6.3.5), as the web UI's on a phone: the users of
// the server and its invitation links, and a user's page: its name, whether
// it is disabled, its restriction profile, the libraries and ratings it may
// see, how it may play, and its sign-ins. What a user may request in
// Discover is left out (docs/clients.md §4.2). In mavio.dev mode a user's
// name, access and state are mavio.dev's, and the server's own credentials
// and invitations are not used.

/** The users of a server, its administrators first, then by name. */
private suspend fun users(model: AppModel, serverId: String): List<User> {
    val collator = Collator.getInstance()
    return model.client.call(serverId, Procedure.users("ListUsers"), listUsersRequest {}, ListUsersResponse::parseFrom).usersList
        .sortedWith { a, b -> if (a.admin != b.admin) (if (a.admin) -1 else 1) else collator.compare(a.name, b.name) }
}

/** Whether a server keeps its own accounts rather than mavio.dev's. */
private suspend fun standalone(model: AppModel, serverId: String): Boolean =
    runCatching { model.client.call(serverId, Procedure.auth("GetAuthInfo"), getAuthInfoRequest {}, GetAuthInfoResponse::parseFrom) }
        .getOrNull()?.mode != AccountMode.ACCOUNT_MODE_MAVIO_DEV

/** The libraries access is given to: not those of collections and playlists. */
private suspend fun accessibleLibraries(model: AppModel, serverId: String): List<Library> = runCatching {
    model.client.call(serverId, Procedure.libraries("ListLibraries"), listLibrariesRequest {}, ListLibrariesResponse::parseFrom).librariesList
}.getOrDefault(emptyList()).filter { it.kind != LibraryKind.LIBRARY_KIND_COLLECTIONS && it.kind != LibraryKind.LIBRARY_KIND_PLAYLISTS }

/** The link of an invitation's token, which opens the server's page accepting it. */
private fun invitationLink(address: String, token: String): String = "${address.trimEnd('/')}/invite#${Uri.encode(token, "-_.!~*'()")}"

/** A date as a day, in the user's language. */
private fun day(t: Timestamp): String =
    t.instant?.atZone(ZoneId.systemDefault())?.format(DateTimeFormatter.ofLocalizedDate(FormatStyle.MEDIUM)).orEmpty()

/** The badges of a user: administrator, managed, disabled. */
@Composable
private fun Badges(user: User) {
    listOfNotNull(
        R.string.users_admin.takeIf { user.admin },
        R.string.users_managed.takeIf { user.managed },
        R.string.users_disabled.takeIf { user.disabled },
    ).forEach {
        Text(
            stringResource(it),
            Modifier.background(theme.colors.fill, RoundedCornerShape(4.dp)).padding(horizontal = 6.dp),
            style = theme.type.caption.text,
            color = theme.colors.textSecondary,
            maxLines = 1,
        )
    }
}

/** The restriction profiles a managed user may have. */
@Composable
private fun restrictions(): List<Pair<RestrictionProfile, String>> = listOf(
    RestrictionProfile.RESTRICTION_PROFILE_UNSPECIFIED to stringResource(R.string.users_restriction_none),
    RestrictionProfile.RESTRICTION_PROFILE_YOUNGER_CHILD to stringResource(R.string.users_restriction_younger_child),
    RestrictionProfile.RESTRICTION_PROFILE_OLDER_CHILD to stringResource(R.string.users_restriction_older_child),
    RestrictionProfile.RESTRICTION_PROFILE_TEEN to stringResource(R.string.users_restriction_teen),
)

/** Users: the users of the server, adding one, and its invitation links. */
@Composable
internal fun UsersPage(model: AppModel, serverId: String) {
    var users by remember { mutableStateOf<List<User>?>(null) }
    var failure by remember { mutableStateOf<String?>(null) }
    var standalone by remember { mutableStateOf(false) }
    val navigator = LocalNavigator.current
    LaunchedEffect(serverId) {
        coroutineScope {
            val s = async { standalone(model, serverId) }
            attempt({ failure = it }) {
                users = users(model, serverId)
                failure = null
            }
            standalone = s.await()
        }
    }
    SettingsList(
        stringResource(R.string.settings_users),
        users?.let { stringResource(if (standalone) R.string.users_message else R.string.users_message_mavio_dev) },
    ) {
        item {
            Group {
                users.orEmpty().forEachIndexed { i, u ->
                    if (i > 0) RowDivider(tiled = true)
                    Row(
                        Modifier.fillMaxWidth().clickable { navigator.open(Route.ManagedUser(serverId, u.id, standalone)) }
                            .padding(horizontal = 16.dp, vertical = 10.dp),
                        horizontalArrangement = Arrangement.spacedBy(MavioSpacing.s12),
                        verticalAlignment = Alignment.CenterVertically,
                    ) {
                        Avatar(u.name)
                        Column(Modifier.weight(1f), verticalArrangement = Arrangement.spacedBy(2.dp)) {
                            Row(horizontalArrangement = Arrangement.spacedBy(MavioSpacing.s8), verticalAlignment = Alignment.CenterVertically) {
                                Text(u.name, Modifier.weight(1f, fill = false), style = theme.type.body.text, maxLines = 1)
                                Badges(u)
                            }
                            Text(
                                if (u.hasLastLoginTime()) stringResource(R.string.users_last_sign_in, day(u.lastLoginTime)) else stringResource(R.string.users_never_signed_in),
                                style = theme.type.footnote.text,
                                color = theme.colors.textSecondary,
                                maxLines = 1,
                            )
                        }
                        Chevron()
                    }
                }
                when {
                    failure != null -> Note(failure!!, failed = true)
                    users == null -> Note(stringResource(R.string.state_loading))
                }
                if (standalone && users != null) {
                    RowDivider()
                    LinkRow(stringResource(R.string.users_add), R.drawable.ic_add) { navigator.open(Route.ManagedUser(serverId, null, true)) }
                }
            }
        }
        if (standalone && users != null) invitations(model, serverId, users.orEmpty())
    }
}

/** A row of a four-digit PIN. */
@Composable
private fun PinRow(label: String, pin: String, detail: String?, onChange: (String) -> Unit) {
    TextRow(label, pin, { onChange(it.filter(Char::isDigit).take(4)) }, detail = detail, placeholder = "0000", keyboard = KeyboardType.NumberPassword, fieldWidth = 80)
}

/** A row of a library, checked when chosen. */
@Composable
private fun LibraryCheckRow(name: String, on: Boolean, enabled: Boolean, onClick: () -> Unit) {
    Row(
        Modifier.fillMaxWidth().clickable(enabled = enabled, onClick = onClick).heightIn(min = 48.dp).padding(horizontal = 16.dp, vertical = 12.dp),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        Text(name, Modifier.weight(1f), style = theme.type.body.text)
        if (on) MavioIcon(R.drawable.ic_check, size = 18.dp, tint = theme.colors.accentText)
    }
}

/**
 * The page adding a user: a member with a password, or a managed user with
 * a PIN and a restriction profile; then the user's page. Showing a user's
 * page when id is set.
 */
@Composable
fun ManagedUserScreen(model: AppModel, route: Route.ManagedUser) {
    var created by remember { mutableStateOf<User?>(null) }
    created?.let {
        UserPage(model, route.serverId, it, standalone = true)
        return
    }
    if (route.id != null) {
        var user by remember { mutableStateOf<User?>(null) }
        var failure by remember { mutableStateOf<String?>(null) }
        LaunchedEffect(route.id) {
            attempt({ failure = it }) {
                user = model.client.call(route.serverId, Procedure.users("GetUser"), getUserRequest { id = route.id }, GetUserResponse::parseFrom).user
            }
        }
        val u = user
        when {
            u != null -> UserPage(model, route.serverId, u, route.standalone)
            failure != null -> StatusView(failure!!, icon = R.drawable.ic_error)
            else -> StatusView(stringResource(R.string.state_loading))
        }
        return
    }
    var managed by remember { mutableStateOf(false) }
    var name by remember { mutableStateOf("") }
    var password by remember { mutableStateOf("") }
    var pin by remember { mutableStateOf("") }
    var restriction by remember { mutableStateOf(RestrictionProfile.RESTRICTION_PROFILE_UNSPECIFIED) }
    var saving by remember { mutableStateOf(false) }
    var failure by remember { mutableStateOf<String?>(null) }
    val scope = rememberCoroutineScope()
    val focus = LocalFocusManager.current
    val valid = name.isNotBlank() && if (managed) pin.isEmpty() || pin.length == 4 else password.isNotEmpty()
    SettingsList(stringResource(R.string.users_add)) {
        item {
            Group(footer = stringResource(if (managed) R.string.users_managed_footer else R.string.users_member_footer)) {
                ChoiceRow(
                    stringResource(R.string.users_kind),
                    listOf(false to stringResource(R.string.users_member), true to stringResource(R.string.users_managed)),
                    managed,
                ) { managed = it }
                RowDivider()
                TextRow(stringResource(R.string.users_name), name, { name = it })
                RowDivider()
                if (managed) {
                    PinRow(stringResource(R.string.users_pin), pin, stringResource(R.string.users_pin_detail)) { pin = it }
                    RowDivider()
                    ChoiceRow(stringResource(R.string.users_restriction), restrictions(), restriction, stringResource(R.string.users_restriction_detail)) { restriction = it }
                } else {
                    Column(Modifier.padding(horizontal = 16.dp, vertical = 10.dp)) {
                        SettingsField(password, { password = it }, stringResource(R.string.auth_password), secret = true)
                    }
                }
            }
        }
        item {
            Row(horizontalArrangement = Arrangement.spacedBy(MavioSpacing.s12), verticalAlignment = Alignment.CenterVertically) {
                MavioButton(
                    if (saving) "…" else stringResource(R.string.users_create),
                    {
                        focus.clearFocus()
                        saving = true
                        failure = null
                        scope.launch {
                            attempt({ failure = it }) {
                                created = model.client.call(
                                    route.serverId,
                                    Procedure.users("CreateUser"),
                                    createUserRequest {
                                        this.name = name.trim()
                                        if (managed) {
                                            this.managed = true
                                            this.restriction = restriction
                                            if (pin.isNotEmpty()) this.pin = pin
                                        } else {
                                            this.password = password
                                        }
                                    },
                                    CreateUserResponse::parseFrom,
                                ).user
                            }
                            saving = false
                        }
                    },
                    enabled = valid && !saving,
                )
                failure?.let { Text(it, style = theme.type.callout.text, color = theme.colors.danger) }
            }
        }
    }
}

/** A user as its page edits it. */
private data class UserDraft(
    val name: String,
    val disabled: Boolean,
    val restriction: RestrictionProfile,
    val allLibraries: Boolean,
    val libraryIds: List<String>,
    /** The highest rating score allowed; null when unrestricted. */
    val maxRating: Int?,
    val blockUnrated: Boolean,
    val allowedLabels: String,
    val blockedLabels: String,
    val allowDownload: Boolean,
    val allowTranscoding: Boolean,
    val maxBitrate: Long,
    val maxSessions: Int,
    val reportIssues: Boolean,
) {
    constructor(u: User) : this(
        name = u.name,
        disabled = u.disabled,
        restriction = u.restriction,
        allLibraries = u.policy.allLibraries,
        libraryIds = u.policy.libraryIdsList,
        maxRating = if (u.policy.hasMaxParentalRating()) u.policy.maxParentalRating else null,
        blockUnrated = u.policy.blockUnrated,
        allowedLabels = u.policy.allowedLabelsList.joinToString(", "),
        blockedLabels = u.policy.blockedLabelsList.joinToString(", "),
        allowDownload = u.policy.allowDownload,
        allowTranscoding = u.policy.allowTranscoding,
        maxBitrate = u.policy.maxStreamingBitrate,
        maxSessions = u.policy.maxSessions,
        reportIssues = u.policy.reportIssues,
    )

    /** The policy of the draft, keeping what the page does not show of the user's. */
    fun policy(keeping: UserPolicy): UserPolicy = keeping.toBuilder().also { q ->
        q.allLibraries = allLibraries
        q.clearLibraryIds()
        if (!allLibraries) q.addAllLibraryIds(libraryIds)
        if (maxRating != null) q.maxParentalRating = maxRating else q.clearMaxParentalRating()
        q.blockUnrated = blockUnrated
        q.clearAllowedLabels().addAllAllowedLabels(labels(allowedLabels))
        q.clearBlockedLabels().addAllBlockedLabels(labels(blockedLabels))
        q.allowDownload = allowDownload
        q.allowTranscoding = allowTranscoding
        q.maxStreamingBitrate = maxBitrate
        q.maxSessions = maxSessions
        q.reportIssues = reportIssues
    }.build()

    companion object {
        fun labels(s: String): List<String> = s.split(',').map { it.trim() }.filter { it.isNotEmpty() }.distinct()
    }
}

private const val MBPS = 1_000_000L
private val BITRATES = listOf(0L, 120, 80, 40, 20, 10, 8, 4, 2, 1).map { it * MBPS }

/** The rating score of adult content. */
private const val ADULT = 1000

/** A setting the user's page shows: a switch, or its value when mavio.dev keeps it. */
@Composable
private fun Toggle(label: String, on: Boolean, detail: String? = null, own: Boolean = true, onChange: (Boolean) -> Unit) {
    if (own) {
        SwitchRow(label, on, detail, onChange)
    } else {
        Row(
            Modifier.fillMaxWidth().heightIn(min = 52.dp).padding(horizontal = 16.dp, vertical = 8.dp),
            horizontalArrangement = Arrangement.spacedBy(MavioSpacing.s12),
            verticalAlignment = Alignment.CenterVertically,
        ) {
            RowLabel(label, detail, Modifier.weight(1f))
            Text(stringResource(if (on) R.string.users_on else R.string.users_off), style = theme.type.body.text, color = theme.colors.textSecondary)
        }
    }
}

/** A field of labels separated by commas. */
@Composable
private fun LabelsRow(label: String, detail: String, value: String, enabled: Boolean, onChange: (String) -> Unit) {
    Column(Modifier.fillMaxWidth().padding(horizontal = 16.dp, vertical = 10.dp), verticalArrangement = Arrangement.spacedBy(MavioSpacing.s8)) {
        RowLabel(label, detail)
        if (enabled) {
            SettingsField(value, onChange, null)
        } else {
            Text(value.ifEmpty { "—" }, style = theme.type.body.text, color = theme.colors.textSecondary)
        }
    }
}

/**
 * A user's page: its name, state and restriction profile, the libraries
 * and ratings it may see, how it may play, its sign-ins, and deleting it.
 */
@Composable
private fun UserPage(model: AppModel, serverId: String, initial: User, standalone: Boolean) {
    var user by remember(initial.id) { mutableStateOf(initial) }
    var draft by remember(initial.id) { mutableStateOf(UserDraft(initial)) }
    var libraries by remember { mutableStateOf<List<Library>>(emptyList()) }
    var ratings by remember { mutableStateOf<List<Rating>>(emptyList()) }
    var saving by remember { mutableStateOf(false) }
    var notice by remember { mutableStateOf<String?>(null) }
    var failure by remember { mutableStateOf<String?>(null) }
    var confirming by remember { mutableStateOf(false) }
    val scope = rememberCoroutineScope()
    val navigator = LocalNavigator.current
    val focus = LocalFocusManager.current
    val savedNotice = stringResource(R.string.users_saved)
    LaunchedEffect(serverId) {
        coroutineScope {
            val l = async { accessibleLibraries(model, serverId) }
            ratings = runCatching {
                model.client.call(serverId, Procedure.localization("ListRatings"), listRatingsRequest {}, ListRatingsResponse::parseFrom).ratingsList
            }.getOrDefault(emptyList())
            libraries = l.await()
        }
    }
    val saved = UserDraft(user)
    val valid = draft.name.isNotBlank()
    val deletable = !user.admin && (standalone || user.disabled)
    val ratingAll = stringResource(R.string.users_rating_all)
    val ratingAdult = stringResource(R.string.users_rating_adult)
    val ratingAge = stringResource(R.string.users_rating_age, "%s")
    // Ratings by the age they stand for; those above adult content are left to no limit.
    val scores = (ratings.map { it.score }.filter { it <= ADULT } + listOfNotNull(draft.maxRating)).distinct().sorted()
    fun ratingTitle(s: Int): String {
        val names = ratings.filter { it.score == s }.map { it.name }
        val age = when {
            s == 0 -> ratingAll
            s >= ADULT -> ratingAdult
            else -> ratingAge.replace("%s", s.toString())
        }
        return if (names.isNotEmpty() && names.size <= 3) "$age (${names.joinToString(", ")})" else age
    }
    val unlimited = stringResource(R.string.users_unlimited)
    val mbps = stringResource(R.string.users_mbps, "%s")
    val bitrates = if (draft.maxBitrate in BITRATES) BITRATES else BITRATES + draft.maxBitrate
    SettingsList(user.name) {
        item {
            Group(
                footer = if (!standalone) stringResource(R.string.users_mavio_dev_note) else null,
            ) {
                Row(
                    Modifier.fillMaxWidth().padding(horizontal = 16.dp, vertical = 12.dp),
                    horizontalArrangement = Arrangement.spacedBy(MavioSpacing.s12),
                    verticalAlignment = Alignment.CenterVertically,
                ) {
                    Avatar(user.name, size = 40.dp)
                    Text(user.name, Modifier.weight(1f, fill = false), style = theme.type.title3.text, maxLines = 1)
                    Badges(user)
                }
                RowDivider()
                // The account's name, state and access are mavio.dev's in its mode.
                if (standalone) {
                    TextRow(stringResource(R.string.users_name), draft.name, { draft = draft.copy(name = it) })
                } else {
                    ValueRow(stringResource(R.string.users_name), user.name)
                }
                if (!user.admin) {
                    RowDivider()
                    Toggle(stringResource(R.string.users_disable), draft.disabled, stringResource(R.string.users_disable_detail), standalone) {
                        draft = draft.copy(disabled = it)
                    }
                }
                if (user.managed) {
                    RowDivider()
                    ChoiceRow(stringResource(R.string.users_restriction), restrictions(), draft.restriction, stringResource(R.string.users_restriction_detail)) {
                        draft = draft.copy(restriction = it)
                    }
                }
            }
        }
        if (!user.admin) {
            item {
                Group(stringResource(R.string.users_access), if (!standalone) stringResource(R.string.users_access_mavio_dev) else null) {
                    Toggle(stringResource(R.string.users_all_libraries), draft.allLibraries, own = standalone) { draft = draft.copy(allLibraries = it) }
                    if (!draft.allLibraries) {
                        libraries.forEach { l ->
                            val on = l.id in draft.libraryIds
                            RowDivider()
                            LibraryCheckRow(l.name, on, standalone) {
                                draft = draft.copy(libraryIds = if (on) draft.libraryIds - l.id else draft.libraryIds + l.id)
                            }
                        }
                    }
                    RowDivider()
                    if (standalone) {
                        ChoiceRow(
                            stringResource(R.string.users_max_rating),
                            listOf<Pair<Int?, String>>(null to stringResource(R.string.users_no_limit)) + scores.map { it to ratingTitle(it) },
                            draft.maxRating,
                            stringResource(R.string.users_max_rating_detail),
                        ) { draft = draft.copy(maxRating = it) }
                    } else {
                        ValueRow(stringResource(R.string.users_max_rating), draft.maxRating?.let(::ratingTitle) ?: stringResource(R.string.users_no_limit))
                    }
                    RowDivider()
                    Toggle(stringResource(R.string.users_block_unrated), draft.blockUnrated, own = standalone) { draft = draft.copy(blockUnrated = it) }
                    RowDivider()
                    LabelsRow(stringResource(R.string.users_allowed_labels), stringResource(R.string.users_allowed_labels_detail), draft.allowedLabels, standalone) {
                        draft = draft.copy(allowedLabels = it)
                    }
                    RowDivider()
                    LabelsRow(stringResource(R.string.users_blocked_labels), stringResource(R.string.users_blocked_labels_detail), draft.blockedLabels, standalone) {
                        draft = draft.copy(blockedLabels = it)
                    }
                    RowDivider()
                    Toggle(stringResource(R.string.users_allow_download), draft.allowDownload, own = standalone) { draft = draft.copy(allowDownload = it) }
                }
            }
        }
        item {
            Group(stringResource(R.string.users_playback)) {
                SwitchRow(stringResource(R.string.users_allow_transcoding), draft.allowTranscoding, stringResource(R.string.users_allow_transcoding_detail)) {
                    draft = draft.copy(allowTranscoding = it)
                }
                RowDivider()
                ChoiceRow(
                    stringResource(R.string.users_max_bitrate),
                    bitrates.map { it to if (it == 0L) unlimited else mbps.replace("%s", (it / MBPS).toString()) },
                    draft.maxBitrate,
                ) { draft = draft.copy(maxBitrate = it) }
                RowDivider()
                NumberRow(
                    stringResource(R.string.users_max_sessions),
                    draft.maxSessions.toDouble(),
                    0.0..100.0,
                    { draft = draft.copy(maxSessions = it.toInt()) },
                    detail = stringResource(R.string.users_max_sessions_detail),
                )
            }
        }
        if (!user.admin) {
            item {
                Group {
                    SwitchRow(stringResource(R.string.users_report_issues), draft.reportIssues) { draft = draft.copy(reportIssues = it) }
                }
            }
        }
        item {
            SaveRow(draft != saved && valid, saving, notice, failure) {
                focus.clearFocus()
                saving = true
                failure = null
                scope.launch {
                    attempt({ failure = it }) {
                        user = model.client.call(
                            serverId,
                            Procedure.users("UpdateUser"),
                            updateUserRequest {
                                id = user.id
                                name = draft.name.trim()
                                disabled = draft.disabled
                                restriction = draft.restriction
                                policy = draft.policy(user.policy)
                            },
                            UpdateUserResponse::parseFrom,
                        ).user
                        draft = UserDraft(user)
                        notice = savedNotice
                    }
                    saving = false
                }
            }
        }
        if (standalone) {
            item { SignIns(model, serverId, user) { user = it } }
        }
        if (deletable) {
            item {
                Group {
                    ActionRow(stringResource(R.string.action_delete), theme.colors.danger, true) { confirming = true }
                }
            }
        }
    }
    if (confirming) {
        ConfirmDialog(
            stringResource(R.string.users_delete_title, user.name),
            stringResource(R.string.users_delete_message),
            stringResource(R.string.action_delete),
            onConfirm = {
                confirming = false
                scope.launch {
                    attempt({ failure = it }) {
                        model.client.call(serverId, Procedure.users("DeleteUser"), deleteUserRequest { id = user.id }, DeleteUserResponse::parseFrom)
                        navigator.back()
                    }
                }
            },
            onDismiss = { confirming = false },
        )
    }
}

/** A button copying a link, telling once it is copied. */
@Composable
private fun CopyLink(link: String) {
    val context = LocalContext.current
    var copied by remember(link) { mutableStateOf(false) }
    MavioButton(
        stringResource(if (copied) R.string.users_copied else R.string.users_copy_link),
        {
            context.getSystemService(ClipboardManager::class.java).setPrimaryClip(ClipData.newPlainText(link, link))
            copied = true
        },
        kind = ButtonKind.Secondary,
        icon = R.drawable.ic_copy,
    )
}

/** A row with a label, its detail and a button at its end. */
@Composable
private fun ButtonRow(label: String, detail: String?, action: String, enabled: Boolean = true, onClick: () -> Unit) {
    Row(
        Modifier.fillMaxWidth().padding(horizontal = 16.dp, vertical = 10.dp),
        horizontalArrangement = Arrangement.spacedBy(MavioSpacing.s12),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        RowLabel(label, detail, Modifier.weight(1f))
        MavioButton(action, onClick, kind = ButtonKind.Secondary, enabled = enabled)
    }
}

/**
 * A user's sign-ins: its password, PIN, authenticator and passkeys, and an
 * invitation that converts a managed user into a member.
 */
@Composable
private fun SignIns(model: AppModel, serverId: String, user: User, changed: (User) -> Unit) {
    var password by remember { mutableStateOf("") }
    var pin by remember { mutableStateOf("") }
    var link by remember { mutableStateOf<String?>(null) }
    var busy by remember { mutableStateOf(false) }
    var notice by remember { mutableStateOf<Int?>(null) }
    var failure by remember { mutableStateOf<String?>(null) }
    val scope = rememberCoroutineScope()
    val focus = LocalFocusManager.current
    val server = model.servers.collectAsState().value.firstOrNull { it.id == serverId }

    /** Runs a change of the user's sign-ins, then reloads the user. */
    fun run(done: Int?, action: suspend () -> Unit) {
        focus.clearFocus()
        busy = true
        failure = null
        scope.launch {
            attempt({ failure = it }) {
                action()
                notice = done
                runCatching {
                    model.client.call(serverId, Procedure.users("GetUser"), getUserRequest { id = user.id }, GetUserResponse::parseFrom).user
                }.getOrNull()?.let(changed)
            }
            busy = false
        }
    }
    Group(
        stringResource(R.string.users_sign_ins),
        failure ?: notice?.let { stringResource(it) },
    ) {
        var first = true
        @Composable
        fun divider() {
            if (!first) RowDivider()
            first = false
        }
        // Users change their own password, with the current one, in their account's settings.
        if (user.authProvider.isEmpty() && user.id != server?.userId) {
            divider()
            Column(Modifier.fillMaxWidth().padding(horizontal = 16.dp, vertical = 10.dp), verticalArrangement = Arrangement.spacedBy(MavioSpacing.s8)) {
                RowLabel(
                    stringResource(if (user.managed) R.string.users_give_password else R.string.users_set_password),
                    stringResource(if (user.managed) R.string.users_give_password_detail else R.string.users_set_password_detail),
                )
                Row(horizontalArrangement = Arrangement.spacedBy(MavioSpacing.s8), verticalAlignment = Alignment.CenterVertically) {
                    SettingsField(password, { password = it }, null, Modifier.weight(1f), stringResource(R.string.auth_password), secret = true)
                    MavioButton(
                        stringResource(R.string.users_set),
                        {
                            run(if (user.managed) R.string.users_converted else R.string.users_password_set) {
                                model.client.call(
                                    serverId,
                                    Procedure.users("SetPassword"),
                                    setPasswordRequest {
                                        userId = user.id
                                        newPassword = password
                                    },
                                    SetPasswordResponse::parseFrom,
                                )
                                password = ""
                            }
                        },
                        kind = ButtonKind.Secondary,
                        enabled = password.isNotEmpty() && !busy,
                    )
                }
            }
        }
        if (user.managed) {
            fun setPin(value: String) = run(if (value.isEmpty()) R.string.users_pin_cleared else R.string.users_pin_set) {
                model.client.call(
                    serverId,
                    Procedure.users("SetPin"),
                    setPinRequest {
                        userId = user.id
                        this.pin = value
                    },
                    SetPinResponse::parseFrom,
                )
                pin = ""
            }
            divider()
            Row(verticalAlignment = Alignment.CenterVertically) {
                Column(Modifier.weight(1f)) {
                    PinRow(stringResource(R.string.users_pin), pin, stringResource(if (user.hasPin) R.string.users_pin_on else R.string.users_pin_off)) { pin = it }
                }
                MavioButton(stringResource(R.string.users_set), { setPin(pin) }, kind = ButtonKind.Secondary, enabled = pin.length == 4 && !busy)
                androidx.compose.foundation.layout.Spacer(Modifier.width(16.dp))
            }
            if (user.hasPin) {
                RowDivider()
                ActionRow(stringResource(R.string.users_clear), theme.colors.accentText, !busy) { setPin("") }
            }
            RowDivider()
            Column(Modifier.fillMaxWidth().padding(horizontal = 16.dp, vertical = 10.dp), verticalArrangement = Arrangement.spacedBy(MavioSpacing.s8)) {
                SelectionContainer {
                    Column { RowLabel(stringResource(R.string.users_invite_managed), link ?: stringResource(R.string.users_invite_managed_detail)) }
                }
                val l = link
                if (l != null) {
                    CopyLink(l)
                } else {
                    MavioButton(
                        stringResource(R.string.users_create_link),
                        {
                            busy = true
                            failure = null
                            scope.launch {
                                attempt({ failure = it }) {
                                    val r = model.client.call(
                                        serverId,
                                        Procedure.users("CreateInvitation"),
                                        createInvitationRequest {
                                            userId = user.id
                                            maxUses = 1
                                            validFor = duration { seconds = 7 * 86400L }
                                        },
                                        CreateInvitationResponse::parseFrom,
                                    )
                                    link = invitationLink(server?.address.orEmpty(), r.token)
                                }
                                busy = false
                            }
                        },
                        kind = ButtonKind.Secondary,
                        enabled = !busy,
                    )
                }
            }
        }
        if (user.hasTotp) {
            divider()
            ButtonRow(stringResource(R.string.users_totp), stringResource(R.string.users_totp_detail), stringResource(R.string.users_turn_off), enabled = !busy) {
                run(null) {
                    model.client.call(serverId, Procedure.users("DeleteTotp"), deleteTotpRequest { userId = user.id }, DeleteTotpResponse::parseFrom)
                }
            }
        }
        user.passkeysList.forEach { k ->
            divider()
            ButtonRow(
                k.name.ifEmpty { stringResource(R.string.users_passkey) },
                listOf(k.rpId, if (k.hasLastUseTime()) stringResource(R.string.users_last_used, day(k.lastUseTime)) else "").filter { it.isNotEmpty() }.joinToString(" · "),
                stringResource(R.string.action_delete),
                                enabled = !busy,
            ) {
                run(null) {
                    model.client.call(
                        serverId,
                        Procedure.users("DeletePasskey"),
                        deletePasskeyRequest {
                            userId = user.id
                            id = k.id
                        },
                        DeletePasskeyResponse::parseFrom,
                    )
                }
            }
        }
    }
}

/**
 * The server's invitation links — making one, which shows its link once,
 * and revoking them — and the links made for managed users, each
 * converting one into a member.
 */
private fun LazyListScope.invitations(model: AppModel, serverId: String, users: List<User>) {
    item { Invitations(model, serverId, users) }
}

@Composable
private fun Invitations(model: AppModel, serverId: String, users: List<User>) {
    var invitations by remember { mutableStateOf<List<Invitation>>(emptyList()) }
    var loaded by remember { mutableStateOf(false) }
    var libraries by remember { mutableStateOf<List<Library>>(emptyList()) }
    var making by remember { mutableStateOf(false) }
    var uses by remember { mutableIntStateOf(1) }
    var days by remember { mutableIntStateOf(7) }
    var all by remember { mutableStateOf(true) }
    var ids by remember { mutableStateOf<List<String>>(emptyList()) }
    var download by remember { mutableStateOf(true) }
    var made by remember { mutableStateOf<Pair<String, String>?>(null) }
    var busy by remember { mutableStateOf(false) }
    var failure by remember { mutableStateOf<String?>(null) }
    var reload by remember { mutableIntStateOf(0) }
    val scope = rememberCoroutineScope()
    val server = model.servers.collectAsState().value.firstOrNull { it.id == serverId }
    LaunchedEffect(serverId, reload) {
        attempt({ failure = it }) {
            invitations = model.client.call(serverId, Procedure.users("ListInvitations"), listInvitationsRequest {}, ListInvitationsResponse::parseFrom).invitationsList
            loaded = true
        }
    }
    LaunchedEffect(serverId) { libraries = accessibleLibraries(model, serverId) }
    // Links used up can no longer be accepted.
    val open = invitations.filter { it.uses < it.maxUses }
    val list = open.filter { it.userId.isEmpty() }
    // The link made is shown until its invitation is revoked or used up.
    val link = made?.takeIf { m -> list.any { it.id == m.first } }?.second

    fun revoke(id: String) {
        busy = true
        failure = null
        scope.launch {
            attempt({ failure = it }) {
                model.client.call(serverId, Procedure.users("DeleteInvitation"), deleteInvitationRequest { this.id = id }, DeleteInvitationResponse::parseFrom)
                reload++
            }
            busy = false
        }
    }

    @Composable
    fun InvitationRow(i: Invitation, label: String) {
        ButtonRow(
            label,
            listOf(
                stringResource(R.string.users_expires, day(i.expireTime)),
                if (i.hasCreateTime()) stringResource(R.string.users_created, day(i.createTime)) else "",
            ).filter { it.isNotEmpty() }.joinToString(" · "),
            stringResource(R.string.users_revoke),
                        enabled = !busy,
        ) { revoke(i.id) }
    }
    Column(verticalArrangement = Arrangement.spacedBy(24.dp)) {
        Group(stringResource(R.string.users_invitations), failure ?: stringResource(R.string.users_invitations_footer)) {
            if (link != null) {
                Column(Modifier.fillMaxWidth().padding(horizontal = 16.dp, vertical = 10.dp), verticalArrangement = Arrangement.spacedBy(MavioSpacing.s8)) {
                    SelectionContainer { Column { RowLabel(stringResource(R.string.users_link_made), link) } }
                    CopyLink(link)
                }
                RowDivider()
            }
            if (making) {
                NumberRow(stringResource(R.string.users_max_uses), uses.toDouble(), 1.0..100.0, { uses = it.toInt() })
                RowDivider()
                NumberRow(stringResource(R.string.users_valid_days), days.toDouble(), 1.0..30.0, { days = it.toInt() })
                RowDivider()
                SwitchRow(stringResource(R.string.users_all_libraries), all) { all = it }
                if (!all) {
                    libraries.forEach { l ->
                        val on = l.id in ids
                        RowDivider()
                        LibraryCheckRow(l.name, on, true) { ids = if (on) ids - l.id else ids + l.id }
                    }
                }
                RowDivider()
                SwitchRow(stringResource(R.string.users_allow_download), download) { download = it }
                RowDivider()
                Row(Modifier.padding(horizontal = 16.dp, vertical = 10.dp), horizontalArrangement = Arrangement.spacedBy(MavioSpacing.s12)) {
                    MavioButton(
                        stringResource(R.string.users_create_link),
                        {
                            busy = true
                            failure = null
                            scope.launch {
                                attempt({ failure = it }) {
                                    val r = model.client.call(
                                        serverId,
                                        Procedure.users("CreateInvitation"),
                                        createInvitationRequest {
                                            maxUses = uses.coerceIn(1, 100)
                                            validFor = duration { seconds = days.coerceIn(1, 30) * 86400L }
                                            access = userPolicy {
                                                allLibraries = all
                                                if (!all) libraryIds.addAll(ids)
                                                allowDownload = download
                                            }
                                        },
                                        CreateInvitationResponse::parseFrom,
                                    )
                                    made = r.invitation.id to invitationLink(server?.address.orEmpty(), r.token)
                                    making = false
                                    reload++
                                }
                                busy = false
                            }
                        },
                        enabled = !busy && uses in 1..100 && days in 1..30,
                    )
                    MavioButton(stringResource(R.string.action_cancel), { making = false }, kind = ButtonKind.Secondary)
                }
            } else {
                LinkRow(stringResource(R.string.users_invite), R.drawable.ic_add) {
                    making = true
                    made = null
                }
            }
            list.forEach {
                RowDivider()
                InvitationRow(it, stringResource(R.string.users_invitation_uses, it.uses.toString(), it.maxUses.toString()))
            }
            if (loaded && list.isEmpty() && !making && link == null) {
                RowDivider()
                Note(stringResource(R.string.users_no_invitations))
            }
        }
        val conversions = open.filter { it.userId.isNotEmpty() }
        if (conversions.isNotEmpty()) {
            Group(stringResource(R.string.users_conversion_links)) {
                conversions.forEachIndexed { n, i ->
                    if (n > 0) RowDivider()
                    InvitationRow(i, users.firstOrNull { it.id == i.userId }?.name ?: i.userId)
                }
            }
        }
    }
}
