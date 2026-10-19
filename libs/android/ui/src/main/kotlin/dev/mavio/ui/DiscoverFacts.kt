package dev.mavio.ui

import androidx.compose.runtime.Composable
import androidx.compose.ui.res.stringResource
import java.text.NumberFormat
import java.time.LocalDate
import java.time.Period
import java.time.format.DateTimeFormatter
import java.time.format.FormatStyle
import java.util.Currency
import java.util.Locale
import mavio.acquisition.v1.Discover.SeasonStatus
import mavio.acquisition.v1.Discover.TitleCredit
import mavio.library.v1.ItemOuterClass.CreditKind
import mavio.plugin.v1.MetadataOuterClass.ProductionStatus

/** A date of the calendar as TMDB gives it, in the UI's language. */
fun dayText(iso: String): String? = runCatching {
    LocalDate.parse(iso.take(10)).format(DateTimeFormatter.ofLocalizedDate(FormatStyle.MEDIUM))
}.getOrNull()

/** A date of the calendar as TMDB gives it, written out. */
fun longDayText(iso: String): String? = runCatching {
    LocalDate.parse(iso.take(10)).format(DateTimeFormatter.ofLocalizedDate(FormatStyle.LONG))
}.getOrNull()

/** Whole years between two calendar dates, to today when the second is empty. */
fun yearsBetween(from: String, to: String): Int? = runCatching {
    Period.between(LocalDate.parse(from.take(10)), if (to.isEmpty()) LocalDate.now() else LocalDate.parse(to.take(10))).years
}.getOrNull()

/** An amount of US dollars, as TMDB gives budgets and revenues, or null when unknown. */
fun moneyText(dollars: Long): String? = dollars.takeIf { it > 0 }?.let {
    NumberFormat.getCurrencyInstance(Locale.getDefault()).apply {
        currency = Currency.getInstance("USD")
        maximumFractionDigits = 0
    }.format(it)
}

/** A credit of a discovered title. */
data class Credit(val name: String, val photo: String?, val kind: CreditKind, val role: String, val order: Int, val tmdbId: Int) {
    val isCast: Boolean get() = kind == CreditKind.CREDIT_KIND_ACTOR || kind == CreditKind.CREDIT_KIND_GUEST_STAR
}

/** A title's credits, each of the cast once. */
fun titleCredits(credits: List<TitleCredit>): List<Credit> {
    val seen = HashSet<String>()
    return credits
        .filter { c ->
            val cast = c.kind == CreditKind.CREDIT_KIND_ACTOR || c.kind == CreditKind.CREDIT_KIND_GUEST_STAR
            !cast || seen.add(c.externalIdsMap["tmdb"] ?: c.name)
        }
        .map { c ->
            Credit(c.name, sized(c.imageUrl, 185), c.kind, c.role, c.order, c.externalIdsMap["tmdb"]?.toIntOrNull() ?: 0)
        }
}

/** The crew's jobs, in the order the pages list them. */
val crewLabels = listOf(
    CreditKind.CREDIT_KIND_CREATOR to R.string.item_creators,
    CreditKind.CREDIT_KIND_DIRECTOR to R.string.item_director,
    CreditKind.CREDIT_KIND_WRITER to R.string.item_writers,
    CreditKind.CREDIT_KIND_AUTHOR to R.string.credits_authors,
    CreditKind.CREDIT_KIND_PRODUCER to R.string.item_producers,
    CreditKind.CREDIT_KIND_COMPOSER to R.string.item_music,
    CreditKind.CREDIT_KIND_CONDUCTOR to R.string.credits_conductors,
    CreditKind.CREDIT_KIND_LYRICIST to R.string.credits_lyricists,
    CreditKind.CREDIT_KIND_ARTIST to R.string.credits_artists,
    CreditKind.CREDIT_KIND_NARRATOR to R.string.credits_narrators,
    CreditKind.CREDIT_KIND_OTHER to R.string.credits_other,
)

/** The jobs Seerr shows first of a title's crew, in its order, after a series' creators. */
private val crewJobs = listOf(
    "Director", "Creator", "Screenplay", "Writer", "Original Music Composer", "Composer", "Editor", "Producer", "Co-Producer",
    "Executive Producer", "Animation",
)

/** The job of a credit without a more specific one. */
private val kindJobs = mapOf(
    CreditKind.CREDIT_KIND_DIRECTOR to "Director",
    CreditKind.CREDIT_KIND_CREATOR to "Creator",
    CreditKind.CREDIT_KIND_WRITER to "Writer",
    CreditKind.CREDIT_KIND_COMPOSER to "Composer",
    CreditKind.CREDIT_KIND_PRODUCER to "Producer",
)

/** Where a credit of the crew comes among the leading crew, lower first. */
fun crewRank(c: Credit): Int {
    if (c.kind == CreditKind.CREDIT_KIND_CREATOR) return -1
    val i = crewJobs.indexOf(c.role.ifEmpty { kindJobs[c.kind].orEmpty() })
    return if (i < 0) crewJobs.size else i
}

/** The name of an ISO 3166-1 country in the UI's language, after its flag. */
fun countryText(code: String): String {
    if (!Regex("^[A-Z]{2}$").matches(code)) return code
    val flag = code.map { Character.toChars(0x1F1A5 + it.code).concatToString() }.joinToString("")
    return "$flag ${Locale.Builder().setRegion(code).build().getDisplayCountry(Locale.getDefault()).ifEmpty { code }}"
}

/** The name of a title's production status, if it has one. */
@Composable
fun productionStatus(s: ProductionStatus): String? = when (s) {
    ProductionStatus.PRODUCTION_STATUS_RUMORED -> R.string.discover_status_rumored
    ProductionStatus.PRODUCTION_STATUS_PLANNED -> R.string.discover_status_planned
    ProductionStatus.PRODUCTION_STATUS_IN_PRODUCTION -> R.string.discover_status_in_production
    ProductionStatus.PRODUCTION_STATUS_POST_PRODUCTION -> R.string.discover_status_post_production
    ProductionStatus.PRODUCTION_STATUS_RELEASED -> R.string.discover_status_released
    ProductionStatus.PRODUCTION_STATUS_CANCELED -> R.string.discover_status_canceled
    ProductionStatus.PRODUCTION_STATUS_PILOT -> R.string.discover_status_pilot
    ProductionStatus.PRODUCTION_STATUS_RETURNING -> R.string.discover_status_returning
    ProductionStatus.PRODUCTION_STATUS_ENDED -> R.string.discover_status_ended
    else -> null
}?.let { stringResource(it) }

/** A season's name, Specials for the season 0. */
@Composable
fun seasonName(s: SeasonStatus): String =
    if (s.seasonNumber == 0) stringResource(R.string.discover_specials) else stringResource(R.string.discover_season, s.seasonNumber.toString())
